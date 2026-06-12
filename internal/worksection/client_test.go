package worksection

import (
	"context"
	"encoding/json"
	"errors"
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

func TestResponseErrorMessageVariants(t *testing.T) {
	tests := []struct {
		name string
		resp *Response
		want string
	}{
		{
			name: "error string",
			resp: &Response{Error: json.RawMessage(`"plain error"`)},
			want: "plain error",
		},
		{
			name: "error object message",
			resp: &Response{Error: json.RawMessage(`{"message":"object error"}`)},
			want: "object error",
		},
		{
			name: "raw message details",
			resp: &Response{Raw: json.RawMessage(`{"message":"outer","message_details":{"message":"inner"}}`)},
			want: "outer: inner",
		},
		{
			name: "raw nested error",
			resp: &Response{Raw: json.RawMessage(`{"error":{"message":"nested error"}}`)},
			want: "nested error",
		},
		{
			name: "raw fallback",
			resp: &Response{Raw: json.RawMessage(`not-json`)},
			want: "not-json",
		},
		{
			name: "empty fallback",
			resp: &Response{},
			want: "Worksection API error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResponseErrorMessage(tt.resp); got != tt.want {
				t.Fatalf("ResponseErrorMessage = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorksectionErrorExitCodeContract(t *testing.T) {
	tests := map[ErrorCode]int{
		CodeUsage:         2,
		CodeAuth:          3,
		CodeAuthorization: 4,
		CodeNetwork:       5,
		CodeAPI:           6,
		CodeRateLimited:   7,
		CodeTruncated:     8,
		CodeGeneral:       1,
	}
	for code, want := range tests {
		if got := (&Error{Code: code, Message: string(code)}).ExitCode(); got != want {
			t.Fatalf("%s exit code = %d, want %d", code, got, want)
		}
	}
	var nilErr *Error
	if nilErr.Error() != "" || nilErr.ExitCode() != 0 {
		t.Fatalf("nil error contract changed: message=%q exit=%d", nilErr.Error(), nilErr.ExitCode())
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

func TestNormalizeHTTPSHost(t *testing.T) {
	tests := map[string]string{
		"https://Example.Test.":        "example.test",
		"https://example.test:443":     "example.test",
		"https://example.test:8443":    "example.test:8443",
		"https://[::1]:443/callback":   "::1",
		"https://[::1]:33443/callback": "[::1]:33443",
	}
	for raw, want := range tests {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := normalizeHTTPSHost(u); got != want {
			t.Fatalf("normalizeHTTPSHost(%s) = %q, want %q", raw, got, want)
		}
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

func TestResponseOutputDataPreservesCompositeAndObjectBodies(t *testing.T) {
	composite, err := ParseResponse([]byte(`{"status":"ok","data":[{"id":"1"}],"total":{"money":"10"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := composite.OutputData("get_costs"); !strings.Contains(string(got), `"total"`) || strings.Contains(string(got), `"status"`) {
		t.Fatalf("composite output data = %s", got)
	}
	object, err := ParseResponse([]byte(`{"status":"ok","url":"https://example.test/file.bin","name":"file.bin"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := object.OutputData("download"); !strings.Contains(string(got), `"url"`) || strings.Contains(string(got), `"status"`) {
		t.Fatalf("object output data = %s", got)
	}
	empty, err := ParseResponse([]byte(`{"status":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := empty.OutputData("get_users"); string(got) != "[]" {
		t.Fatalf("empty output data = %s", got)
	}
}

func TestSchemaNewLimiterAndEncodeParams(t *testing.T) {
	if spec, ok := Schema("get_users"); !ok || spec.Name != "get_users" {
		t.Fatalf("schema lookup failed: %#v ok=%t", spec, ok)
	}
	limiter, err := NewLimiter("2/s")
	if err != nil {
		t.Fatal(err)
	}
	if limiter == nil {
		t.Fatal("expected limiter")
	}
	for _, spec := range []string{"bad", "0/s", "2/m"} {
		if _, err := NewLimiter(spec); err == nil {
			t.Fatalf("expected invalid limiter %q to fail", spec)
		}
	}
	encoded := EncodeParams("get_users", map[string]string{"email": "a@example.com", "empty": ""})
	if !strings.Contains(encoded, "action=get_users") || !strings.Contains(encoded, "email=a%40example.com") || strings.Contains(encoded, "empty") {
		t.Fatalf("unexpected encoded params %q", encoded)
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

// refreshStub is a sequence-driven transport for reactive-refresh tests: it
// returns the queued statuses in order and records the Authorization header
// of every request it serves.
func refreshStub(statuses []int) (rt roundTripFunc, gotAuth *[]string) {
	var auths []string
	gotAuth = &auths
	rt = func(r *http.Request) (*http.Response, error) {
		auths = append(auths, r.Header.Get("Authorization"))
		status := statuses[len(auths)-1]
		body := `{"status":"error","status_code":0,"message":"Invalid token"}`
		if status == http.StatusOK {
			body = `{"status":"ok","data":[{"id":"1"}]}`
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	return rt, gotAuth
}

func newRefreshTestClient(rt roundTripFunc) *Client {
	return NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthOAuth2, Token: "stale-token", AccountURL: "https://example.test"}, time.Second, nil)
}

// A 401 with a refresher present is recovered by refreshing once and replaying
// the request with the NEW bearer token. The header assertion is the load-
// bearing one: requestForAttempt clones the base request, so mutating only
// c.creds.Token would replay the stale header. The replay must also be
// immediate — it is a credential fix, not a server-fault retry, so no backoff
// sleep applies (a naive loop re-entry would sleep 1s).
func TestClientReactiveRefreshRetriesOnceWithNewToken(t *testing.T) {
	rt, gotAuth := refreshStub([]int{http.StatusUnauthorized, http.StatusOK})
	client := newRefreshTestClient(rt)
	refreshCalls := 0
	client.SetTokenRefresher(func(context.Context) (string, error) {
		refreshCalls++
		return "new-token", nil
	})

	started := time.Now()
	resp, err := client.Call(context.Background(), "get_users", nil)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Data), `"id"`) {
		t.Fatalf("unexpected data %s", resp.Data)
	}
	if refreshCalls != 1 {
		t.Fatalf("refresher calls = %d, want 1", refreshCalls)
	}
	if len(*gotAuth) != 2 {
		t.Fatalf("requests = %d, want 2", len(*gotAuth))
	}
	if (*gotAuth)[1] != "Bearer new-token" {
		t.Fatalf("replayed Authorization = %q, want \"Bearer new-token\"", (*gotAuth)[1])
	}
	if elapsed >= 900*time.Millisecond {
		t.Fatalf("refresh replay took %v; it must not inherit the 1s server-fault backoff", elapsed)
	}
}

// A second 401 after a successful refresh is terminal: exactly two requests,
// one refresher call, and the authentication error surfaces.
func TestClientReactiveRefreshSecond401IsTerminal(t *testing.T) {
	rt, gotAuth := refreshStub([]int{http.StatusUnauthorized, http.StatusUnauthorized})
	client := newRefreshTestClient(rt)
	refreshCalls := 0
	client.SetTokenRefresher(func(context.Context) (string, error) {
		refreshCalls++
		return "new-token", nil
	})

	_, err := client.Call(context.Background(), "get_users", nil)
	var wsErr *Error
	if !errors.As(err, &wsErr) || wsErr.Code != CodeAuth {
		t.Fatalf("err = %v, want CodeAuth", err)
	}
	if refreshCalls != 1 {
		t.Fatalf("refresher calls = %d, want 1", refreshCalls)
	}
	if len(*gotAuth) != 2 {
		t.Fatalf("requests = %d, want exactly 2 (no third attempt)", len(*gotAuth))
	}
}

// A failing refresher must surface the ORIGINAL HTTP 401 authentication
// error, not the refresh implementation detail, and must not replay.
func TestClientReactiveRefreshHookFailurePreservesOriginal401(t *testing.T) {
	rt, gotAuth := refreshStub([]int{http.StatusUnauthorized})
	client := newRefreshTestClient(rt)
	client.SetTokenRefresher(func(context.Context) (string, error) {
		return "", errors.New("oauth refresh exploded")
	})

	_, err := client.Call(context.Background(), "get_users", nil)
	var wsErr *Error
	if !errors.As(err, &wsErr) || wsErr.Code != CodeAuth {
		t.Fatalf("err = %v, want CodeAuth", err)
	}
	if !strings.Contains(err.Error(), "Worksection HTTP 401") || strings.Contains(err.Error(), "exploded") {
		t.Fatalf("error must be the original 401, not the refresh detail: %v", err)
	}
	if len(*gotAuth) != 1 {
		t.Fatalf("requests = %d, want 1 (no replay after failed refresh)", len(*gotAuth))
	}
}

// A hook that reports success but returns an empty token is a broken hook:
// the original 401 surfaces and no replay is spent on a guaranteed failure.
func TestClientReactiveRefreshEmptyTokenIsRefreshFailure(t *testing.T) {
	rt, gotAuth := refreshStub([]int{http.StatusUnauthorized})
	client := newRefreshTestClient(rt)
	client.SetTokenRefresher(func(context.Context) (string, error) {
		return "", nil
	})

	_, err := client.Call(context.Background(), "get_users", nil)
	var wsErr *Error
	if !errors.As(err, &wsErr) || wsErr.Code != CodeAuth {
		t.Fatalf("err = %v, want CodeAuth", err)
	}
	if !strings.Contains(err.Error(), "Worksection HTTP 401") {
		t.Fatalf("error must be the original 401: %v", err)
	}
	if len(*gotAuth) != 1 {
		t.Fatalf("requests = %d, want 1 (no replay with an empty bearer)", len(*gotAuth))
	}
}

// The refresh budget is once per Client (one command invocation), not once
// per request: after a refresh recovered call one, a 401 on call two of the
// same client is terminal without consulting the refresher again.
func TestClientReactiveRefreshOncePerClient(t *testing.T) {
	rt, gotAuth := refreshStub([]int{http.StatusUnauthorized, http.StatusOK, http.StatusUnauthorized})
	client := newRefreshTestClient(rt)
	refreshCalls := 0
	client.SetTokenRefresher(func(context.Context) (string, error) {
		refreshCalls++
		return "new-token", nil
	})

	if _, err := client.Call(context.Background(), "get_users", nil); err != nil {
		t.Fatalf("first call should recover via refresh: %v", err)
	}
	_, err := client.Call(context.Background(), "get_users", nil)
	var wsErr *Error
	if !errors.As(err, &wsErr) || wsErr.Code != CodeAuth {
		t.Fatalf("second call err = %v, want CodeAuth", err)
	}
	if refreshCalls != 1 {
		t.Fatalf("refresher calls = %d, want 1 across the client lifetime", refreshCalls)
	}
	if len(*gotAuth) != 3 {
		t.Fatalf("requests = %d, want 3", len(*gotAuth))
	}
}

// Admin-token mode has no refresh concept: a present refresher is never
// consulted and the 401 stays terminal.
func TestClientAdminModeNeverConsultsRefresher(t *testing.T) {
	rt, gotAuth := refreshStub([]int{http.StatusUnauthorized})
	client := NewClient(&http.Client{Transport: rt}, Credentials{Mode: AuthAdmin, AdminKey: "k", AccountURL: "https://example.test"}, time.Second, nil)
	refreshCalls := 0
	client.SetTokenRefresher(func(context.Context) (string, error) {
		refreshCalls++
		return "new-token", nil
	})

	_, err := client.Call(context.Background(), "get_users", nil)
	var wsErr *Error
	if !errors.As(err, &wsErr) || wsErr.Code != CodeAuth {
		t.Fatalf("err = %v, want CodeAuth", err)
	}
	if refreshCalls != 0 {
		t.Fatalf("refresher calls = %d, want 0 in admin mode", refreshCalls)
	}
	if len(*gotAuth) != 1 {
		t.Fatalf("requests = %d, want 1", len(*gotAuth))
	}
}
