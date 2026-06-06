package worksection

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

type AuthMode string

const (
	AuthOAuth2    AuthMode = "oauth2"
	AuthAdmin     AuthMode = "admin_token"
	AuthAnonymous AuthMode = "anonymous"
)

type Credentials struct {
	Mode       AuthMode
	Token      string
	AdminKey   string
	AccountURL string
}

// Client wraps the Worksection HTTP API with auth, rate limiting, and error
// classification.
type Client struct {
	httpClient       *http.Client
	accountURL       string
	creds            Credentials
	limiter          *rate.Limiter
	maxResponseBytes int64
}

// NewClient constructs a Worksection API client.
func NewClient(httpClient *http.Client, creds Credentials, timeout time.Duration, limiter *rate.Limiter) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		httpClient:       httpClient,
		accountURL:       strings.TrimRight(creds.AccountURL, "/"),
		creds:            creds,
		limiter:          limiter,
		maxResponseBytes: 50 * 1024 * 1024,
	}
}

// Call invokes a read-only Worksection action and parses the JSON response.
func (c *Client) Call(ctx context.Context, action string, params map[string]string) (*Response, error) {
	if err := ValidateAction(action, params, false); err != nil {
		return nil, err
	}
	raw, err := c.CallRaw(ctx, action, params)
	if err != nil {
		return nil, err
	}
	resp, err := ParseResponse(raw)
	if err != nil {
		return nil, &Error{Code: CodeAPI, Message: "failed to parse Worksection response", Details: map[string]any{"parse_error": err.Error()}}
	}
	if resp.Status != "" && resp.Status != "ok" {
		return nil, &Error{Code: CodeAPI, Message: ResponseErrorMessage(resp), Details: map[string]any{"action": action}}
	}
	return resp, nil
}

// CallRaw invokes a read-only Worksection action and returns the raw response.
func (c *Client) CallRaw(ctx context.Context, action string, params map[string]string) ([]byte, error) {
	if err := ValidateAction(action, params, true); err != nil {
		return nil, err
	}
	if c.accountURL == "" {
		return nil, &Error{Code: CodeUsage, Message: "account URL is required"}
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, &Error{Code: CodeRateLimited, Message: err.Error()}
		}
	}
	req, err := c.apiRequest(ctx, action, params)
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Message: err.Error()}
	}
	return c.doRead(req)
}

// Download resolves a Worksection file download and returns its bytes.
func (c *Client) Download(ctx context.Context, fileID string) (*DownloadResponse, error) {
	if fileID == "" {
		return nil, &Error{Code: CodeUsage, Message: "file ID is required"}
	}
	params := map[string]string{"id_file": fileID}
	req, err := c.apiRequest(ctx, "download", params)
	if err != nil {
		return nil, err
	}
	apiResp, err := c.doReadResponse(req)
	if err != nil {
		return nil, err
	}
	if parsed, parseErr := ParseResponse(apiResp.Body); parseErr != nil {
		return &DownloadResponse{
			FileName:    filenameFromHeaders(apiResp.Header, fileID),
			ContentType: apiResp.Header.Get("Content-Type"),
			Body:        apiResp.Body,
		}, nil
	} else if parsed.Status != "" && parsed.Status != "ok" {
		return nil, &Error{Code: CodeAPI, Message: ResponseErrorMessage(parsed)}
	} else {
		return c.downloadFromJSON(ctx, parsed, apiResp.Header, fileID)
	}
}

func (c *Client) downloadFromJSON(ctx context.Context, resp *Response, headers http.Header, fallbackName string) (*DownloadResponse, error) {
	var body struct {
		URL  string `json:"url"`
		Page string `json:"page"`
		Name string `json:"name"`
	}
	source := resp.Data
	if len(source) == 0 || string(source) == "null" {
		source = objectWithoutStatus(resp.Raw)
	}
	_ = json.Unmarshal(source, &body)
	downloadURL := body.URL
	if downloadURL == "" && body.Page != "" {
		downloadURL = strings.TrimRight(c.accountURL, "/") + "/" + strings.TrimLeft(body.Page, "/")
	}
	if downloadURL == "" {
		return nil, &Error{Code: CodeAPI, Message: "download response did not include a URL"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Message: err.Error()}
	}
	if c.creds.Mode == AuthOAuth2 && c.creds.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.Token)
	}
	downloaded, err := c.doReadResponse(req)
	if err != nil {
		return nil, err
	}
	name := body.Name
	if name == "" {
		name = filenameFromHeaders(downloaded.Header, filepath.Base(downloadURL))
	}
	contentType := downloaded.Header.Get("Content-Type")
	if contentType == "" {
		contentType = headers.Get("Content-Type")
	}
	return &DownloadResponse{FileName: name, ContentType: contentType, Body: downloaded.Body}, nil
}

func (c *Client) apiRequest(ctx context.Context, action string, params map[string]string) (*http.Request, error) {
	query := url.Values{}
	query.Set("action", action)
	for k, v := range params {
		if v != "" {
			query.Set(k, v)
		}
	}
	endpoint := c.accountURL + "/api/oauth2"
	if c.creds.Mode == AuthAdmin {
		endpoint = c.accountURL + "/api/admin/v2/"
		query.Set("hash", AdminHash(action, params, c.creds.AdminKey))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "wsectl")
	if c.creds.Mode == AuthOAuth2 && c.creds.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.Token)
	}
	return req, nil
}

func (c *Client) doRead(req *http.Request) ([]byte, error) {
	resp, err := c.doReadResponse(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

type rawHTTPResponse struct {
	Body   []byte
	Header http.Header
	URL    string
}

func (c *Client) doReadResponse(req *http.Request) (*rawHTTPResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := sleepContext(req.Context(), time.Duration(attempt)*time.Second); err != nil {
				return nil, &Error{Code: CodeNetwork, Message: err.Error()}
			}
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, &Error{Code: CodeNetwork, Message: redact(err.Error())}
		}
		raw, readErr := c.readLimited(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, &Error{Code: CodeNetwork, Message: readErr.Error()}
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &rawHTTPResponse{Body: raw, Header: resp.Header.Clone(), URL: responseURL(resp)}, nil
		}
		code := statusCode(resp.StatusCode)
		lastErr = &Error{Code: code, Message: fmt.Sprintf("Worksection HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))}
		if resp.StatusCode == http.StatusTooManyRequests {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if d, err := parseRetryAfter(retryAfter, time.Now()); err == nil {
					if err := sleepContext(req.Context(), d); err != nil {
						return nil, &Error{Code: CodeNetwork, Message: err.Error()}
					}
				}
			}
			continue
		}
		if resp.StatusCode >= 500 {
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

func responseURL(resp *http.Response) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return ""
}

func (c *Client) readLimited(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, c.maxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > c.maxResponseBytes {
		return nil, fmt.Errorf("worksection response exceeded %d bytes", c.maxResponseBytes)
	}
	return raw, nil
}

func statusCode(status int) ErrorCode {
	switch status {
	case http.StatusUnauthorized:
		return CodeAuth
	case http.StatusForbidden:
		return CodeAuthorization
	case http.StatusTooManyRequests:
		return CodeRateLimited
	default:
		if status >= 500 {
			return CodeAPI
		}
		return CodeAPI
	}
}

func parseRetryAfter(value string, now time.Time) (time.Duration, error) {
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds, nil
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, err
	}
	d := when.Sub(now)
	if d < 0 {
		return 0, nil
	}
	return d, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func filenameFromHeaders(headers http.Header, fallback string) string {
	if disposition := headers.Get("Content-Disposition"); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if filename := params["filename"]; filename != "" {
				return filepath.Base(filename)
			}
		}
	}
	if fallback == "" || fallback == "." || fallback == "/" {
		return "download"
	}
	return filepath.Base(fallback)
}

func ResponseErrorMessage(resp *Response) string {
	if len(resp.Error) > 0 {
		return strings.TrimSpace(jsonErrorMessage(resp.Error))
	}
	if len(resp.Raw) > 0 {
		var body struct {
			Message        string          `json:"message"`
			MessageDetails json.RawMessage `json:"message_details"`
			Error          json.RawMessage `json:"error"`
			StatusCode     any             `json:"status_code"`
		}
		if err := json.Unmarshal(resp.Raw, &body); err == nil {
			if body.Message != "" {
				if len(body.MessageDetails) > 0 {
					return body.Message + ": " + strings.TrimSpace(jsonErrorMessage(body.MessageDetails))
				}
				return body.Message
			}
			if len(body.Error) > 0 {
				return strings.TrimSpace(jsonErrorMessage(body.Error))
			}
		}
		return strings.TrimSpace(string(resp.Raw))
	}
	return "Worksection API error"
}

func jsonErrorMessage(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if message, ok := obj["message"].(string); ok && message != "" {
			return message
		}
	}
	return string(raw)
}

func redact(s string) string {
	replacers := []string{"access_token=", "refresh_token=", "Authorization: Bearer "}
	for _, marker := range replacers {
		if idx := strings.Index(s, marker); idx >= 0 {
			end := strings.IndexAny(s[idx+len(marker):], "& \n\r\t")
			if end < 0 {
				return s[:idx+len(marker)] + "[REDACTED]"
			}
			pos := idx + len(marker) + end
			return s[:idx+len(marker)] + "[REDACTED]" + s[pos:]
		}
	}
	return s
}
