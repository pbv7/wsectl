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
// classification. Client is not safe for concurrent use: the reactive token
// refresh mutates creds.Token and the refreshed guard without locking, which
// is sound for the CLI's sequential request pattern.
type Client struct {
	httpClient       *http.Client
	accountURL       string
	creds            Credentials
	limiter          *rate.Limiter
	maxResponseBytes int64
	// refreshToken, when set, is consulted at most once per Client lifetime
	// (the refreshed guard) to recover from an HTTP 401 in OAuth mode: the
	// hook returns a fresh bearer token and the request is replayed once.
	// The client knows nothing about how the token is obtained or persisted.
	// The CLI issues requests sequentially, so no locking guards refreshed.
	refreshToken func(ctx context.Context) (string, error)
	refreshed    bool
}

// SetTokenRefresher installs the hook used to recover from an HTTP 401 in
// OAuth mode. The hook is called at most once for the lifetime of the Client;
// on success the request is replayed once with the returned bearer token, and
// on failure the original authentication error is surfaced unchanged.
func (c *Client) SetTokenRefresher(f func(ctx context.Context) (string, error)) {
	c.refreshToken = f
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
	if err := c.validateDownloadURL(downloadURL); err != nil {
		return nil, err
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

func (c *Client) validateDownloadURL(downloadURL string) error {
	parsed, err := url.Parse(downloadURL)
	if err != nil || parsed.Host == "" {
		return &Error{Code: CodeAPI, Message: "download response included an invalid URL", Details: map[string]any{"reason": "download_invalid_url"}}
	}
	account, err := url.Parse(c.accountURL)
	if err != nil || account.Host == "" {
		return &Error{Code: CodeUsage, Message: "account URL is required"}
	}
	expectedHost := normalizeHTTPSHost(account)
	actualHost := normalizeHTTPSHost(parsed)
	if parsed.Scheme != "https" {
		return &Error{
			Code:    CodeUsage,
			Message: "download URL is not HTTPS; wsectl will not forward credentials to insecure file URLs",
			Details: map[string]any{
				"reason":        "download_insecure_url",
				"expected_host": expectedHost,
				"actual_host":   actualHost,
				"url_scheme":    parsed.Scheme,
			},
		}
	}
	if actualHost != expectedHost {
		return &Error{
			Code:    CodeUsage,
			Message: "download URL host differs from the configured Worksection account; wsectl blocked credential forwarding",
			Details: map[string]any{
				"reason":        "download_host_mismatch",
				"expected_host": expectedHost,
				"actual_host":   actualHost,
				"url_scheme":    parsed.Scheme,
			},
		}
	}
	return nil
}

func normalizeHTTPSHost(u *url.URL) string {
	hostname := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	port := u.Port()
	if port == "" || port == "443" {
		return hostname
	}
	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]:" + port
	}
	return hostname + ":" + port
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
	attempt := 0
	for attempt < 3 {
		attemptReq, err := requestForAttempt(req)
		if err != nil {
			return nil, &Error{Code: CodeNetwork, Message: err.Error()}
		}
		resp, err := c.httpClient.Do(attemptReq)
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
		if resp.StatusCode == http.StatusUnauthorized && c.canReactiveRefresh() {
			// The refresh budget is consumed before calling the hook so a
			// broken hook cannot spin the loop.
			c.refreshed = true
			newToken, refreshErr := c.refreshToken(req.Context())
			if refreshErr != nil || newToken == "" {
				// A failed refresh — including a hook that reports success
				// with an empty token — surfaces the authentication error
				// the API returned, not the refresh implementation detail.
				return nil, lastErr
			}
			c.creds.Token = newToken
			// requestForAttempt clones the base request, so the new bearer
			// must land there or every replay would carry the stale header.
			req.Header.Set("Authorization", "Bearer "+newToken)
			// Immediate replay: a credential fix is not a server fault, so
			// it neither sleeps nor consumes a retry attempt.
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			attempt++
			if attempt < 3 {
				delay := time.Duration(attempt) * time.Second
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if d, err := parseRetryAfter(retryAfter, time.Now()); err == nil {
						delay = d
					}
				}
				if err := sleepContext(req.Context(), delay); err != nil {
					return nil, &Error{Code: CodeNetwork, Message: err.Error()}
				}
			}
			continue
		}
		if resp.StatusCode >= 500 {
			attempt++
			if attempt < 3 {
				if err := sleepContext(req.Context(), time.Duration(attempt)*time.Second); err != nil {
					return nil, &Error{Code: CodeNetwork, Message: err.Error()}
				}
			}
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

// canReactiveRefresh reports whether a 401 may be recovered by the token
// refresh hook: OAuth mode, a hook installed, and the once-per-Client budget
// not yet spent.
func (c *Client) canReactiveRefresh() bool {
	return c.creds.Mode == AuthOAuth2 && c.refreshToken != nil && !c.refreshed
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
		return CodeAPI
	}
}

func requestForAttempt(req *http.Request) (*http.Request, error) {
	cloned := req.Clone(req.Context())
	if req.Body == nil {
		return cloned, nil
	}
	if req.GetBody == nil {
		return nil, fmt.Errorf("request body cannot be replayed for retryable Worksection reads")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	cloned.Body = body
	return cloned, nil
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
		start := 0
		for {
			idxRel := strings.Index(s[start:], marker)
			if idxRel < 0 {
				break
			}
			idx := start + idxRel
			end := strings.IndexAny(s[idx+len(marker):], "& \n\r\t")
			if end < 0 {
				s = s[:idx+len(marker)] + "[REDACTED]"
				break
			}
			pos := idx + len(marker) + end
			s = s[:idx+len(marker)] + "[REDACTED]" + s[pos:]
			start = idx + len(marker) + len("[REDACTED]")
		}
	}
	return s
}
