package worksection

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClientCallSuccess(t *testing.T) {
	var gotAuth string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAuth = r.Header.Get("Authorization")
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "" {
			t.Fatalf("content-type = %q, want empty", r.Header.Get("Content-Type"))
		}
		if r.URL.Query().Get("action") != "get_users" {
			t.Fatalf("action = %q", r.URL.Query().Get("action"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","data":[{"id":"1","name":"Ada"}]}`)),
		}, nil
	})

	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, Token: "token", AccountURL: "https://example.test"}, time.Second, nil)
	resp, err := client.Call(context.Background(), "get_users", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("authorization header = %q", gotAuth)
	}
	if !strings.Contains(string(resp.Data), "Ada") {
		t.Fatalf("unexpected data %s", resp.Data)
	}
}

func TestClientBuildsOAuthQueryParamPost(t *testing.T) {
	client := NewClient(nil, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "token"}, time.Second, nil)
	req, err := client.apiRequest(context.Background(), "get_projects", map[string]string{
		"filter": "active",
		"empty":  "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", req.Method)
	}
	if req.Body != nil {
		t.Fatal("Worksection API params must remain in the query string, not the request body")
	}
	if req.Header.Get("Content-Type") != "" {
		t.Fatalf("content-type = %q, want empty", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("Authorization") != "Bearer token" {
		t.Fatalf("authorization = %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("User-Agent") != "wsectl" {
		t.Fatalf("user-agent = %q", req.Header.Get("User-Agent"))
	}
	query := req.URL.Query()
	if query.Get("action") != "get_projects" || query.Get("filter") != "active" {
		t.Fatalf("query = %s", req.URL.RawQuery)
	}
	if _, ok := query["empty"]; ok {
		t.Fatalf("empty params must be omitted, query = %s", req.URL.RawQuery)
	}
}

func TestClientBuildsAdminQueryParamPost(t *testing.T) {
	params := map[string]string{"id_project": "26", "empty": ""}
	client := NewClient(nil, Credentials{Mode: AuthAdmin, AccountURL: "https://example.test", AdminKey: "key"}, time.Second, nil)
	req, err := client.apiRequest(context.Background(), "get_tasks", params)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.Body != nil {
		t.Fatalf("unexpected request method/body: %s %#v", req.Method, req.Body)
	}
	if req.URL.Path != "/api/admin/v2/" {
		t.Fatalf("path = %q", req.URL.Path)
	}
	query := req.URL.Query()
	if query.Get("action") != "get_tasks" || query.Get("id_project") != "26" {
		t.Fatalf("query = %s", req.URL.RawQuery)
	}
	if query.Get("hash") != AdminHash("get_tasks", params, "key") {
		t.Fatalf("hash = %q, want %q", query.Get("hash"), AdminHash("get_tasks", params, "key"))
	}
	if _, ok := query["empty"]; ok {
		t.Fatalf("empty params must be omitted, query = %s", req.URL.RawQuery)
	}
	if _, err := url.Parse(req.URL.String()); err != nil {
		t.Fatal(err)
	}
}

func TestClientBlocksWrites(t *testing.T) {
	client := NewClient(nil, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	_, err := client.CallRaw(context.Background(), "post_task", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestClientCallClassifiesHTTP200APIError(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"error","message":"invalid JSON"}`)),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	raw, err := client.CallRaw(context.Background(), "get_users", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "invalid JSON") {
		t.Fatalf("raw body was not preserved: %s", raw)
	}
	_, err = client.Call(context.Background(), "get_users", nil)
	if err == nil {
		t.Fatal("expected API error")
	}
	wsErr, ok := err.(*Error)
	if !ok || wsErr.Code != CodeAPI || !strings.Contains(wsErr.Message, "invalid JSON") {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientClassifiesHTTP500AsAPIError(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("server error")),
			Header:     http.Header{},
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	_, err := client.CallRaw(context.Background(), "get_users", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	wsErr, ok := err.(*Error)
	if !ok || wsErr.Code != CodeAPI {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientDownloadDirectBinary(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type":        []string{"application/octet-stream"},
				"Content-Disposition": []string{`attachment; filename="report.bin"`},
			},
			Body: io.NopCloser(strings.NewReader("bytes")),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	got, err := client.Download(context.Background(), "file-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "report.bin" || got.ContentType != "application/octet-stream" || string(got.Body) != "bytes" {
		t.Fatalf("unexpected download %#v", got)
	}
}

func TestClientDownloadFromJSONURL(t *testing.T) {
	calls := 0
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok","data":{"url":"https://example.test/report.txt","name":"report.txt"}}`)),
			}, nil
		}
		if r.URL.String() != "https://example.test/report.txt" {
			t.Fatalf("download URL = %s", r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer x" {
			t.Fatalf("download authorization = %q", r.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("downloaded")),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	got, err := client.Download(context.Background(), "file-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != "report.txt" || got.ContentType != "text/plain" || string(got.Body) != "downloaded" {
		t.Fatalf("unexpected download %#v", got)
	}
}

func TestClientDownloadBlocksCrossHostURL(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","data":{"url":"https://files.example.test/report.txt","name":"report.txt"}}`)),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	_, err := client.Download(context.Background(), "file-1")
	if err == nil {
		t.Fatal("expected blocked download error")
	}
	wsErr, ok := err.(*Error)
	if !ok || wsErr.Code != CodeUsage || wsErr.Details["reason"] != "download_host_mismatch" {
		t.Fatalf("unexpected error %#v", err)
	}
	if wsErr.Details["expected_host"] != "example.test" || wsErr.Details["actual_host"] != "files.example.test" {
		t.Fatalf("unexpected mismatch details %#v", wsErr.Details)
	}
}

func TestClientDownloadBlocksInsecureURL(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","data":{"url":"http://example.test/report.txt","name":"report.txt"}}`)),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	_, err := client.Download(context.Background(), "file-1")
	if err == nil {
		t.Fatal("expected insecure download error")
	}
	wsErr, ok := err.(*Error)
	if !ok || wsErr.Code != CodeUsage || wsErr.Details["reason"] != "download_insecure_url" {
		t.Fatalf("unexpected error %#v", err)
	}
}

func TestClientDownloadJSONMissingURL(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","data":{"name":"report.txt"}}`)),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	_, err := client.Download(context.Background(), "file-1")
	if err == nil {
		t.Fatal("expected missing URL error")
	}
}

func TestRedactMasksMultipleSecretMarkers(t *testing.T) {
	got := redact("access_token=one&refresh_token=two Authorization: Bearer three\naccess_token=four")
	for _, secret := range []string{"one", "two", "three", "four"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted text still contains %q: %s", secret, got)
		}
	}
	if strings.Count(got, "[REDACTED]") != 4 {
		t.Fatalf("unexpected redaction output: %s", got)
	}
}

func TestClientClassifiesHTTP429(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader("too many")),
			Header:     http.Header{},
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	_, err := client.CallRaw(context.Background(), "get_users", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	wsErr, ok := err.(*Error)
	if !ok || wsErr.Code != CodeRateLimited {
		t.Fatalf("error = %#v", err)
	}
}

func TestRetryAfterParsesSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	seconds, err := parseRetryAfter("2", now)
	if err != nil || seconds != 2*time.Second {
		t.Fatalf("seconds retry-after = %s err=%v", seconds, err)
	}
	date := now.Add(3 * time.Second).Format(http.TimeFormat)
	duration, err := parseRetryAfter(date, now)
	if err != nil || duration != 3*time.Second {
		t.Fatalf("date retry-after = %s err=%v", duration, err)
	}
}

func TestClientResponseSizeLimit(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("too-large")),
		}, nil
	})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, AccountURL: "https://example.test", Token: "x"}, time.Second, nil)
	client.maxResponseBytes = 3
	_, err := client.CallRaw(context.Background(), "get_users", nil)
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestRequestForAttemptRejectsNonReplayableBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.test", strings.NewReader("body"))
	if err != nil {
		t.Fatal(err)
	}
	req.GetBody = nil
	_, err = requestForAttempt(req)
	if err == nil || !strings.Contains(err.Error(), "cannot be replayed") {
		t.Fatalf("expected non-replayable body error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
