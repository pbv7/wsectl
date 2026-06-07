package history

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRecordReadTrimAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "history.jsonl")
	for _, command := range []string{"one", "two", "three", "four"} {
		if err := Record(context.Background(), Options{Enabled: true, Path: path, IncludeParams: "all"}, Event{Command: command, Status: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ReadWithStats(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 4 {
		t.Fatalf("record should append without auto-trim: %#v", result.Events)
	}
	if err := Trim(context.Background(), path, 2); err != nil {
		t.Fatal(err)
	}
	result, err = ReadWithStats(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 2 || result.Events[0].Command != "three" || result.Events[1].Command != "four" {
		t.Fatalf("unexpected trimmed events: %#v", result.Events)
	}
	if st, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("history mode = %o, want 0600", st.Mode().Perm())
	}
	if err := Clear(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("history file should be removed, err=%v", err)
	}
}

func TestHistoryLockBlocksConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "history.jsonl")
	lock, err := acquireHistoryLock(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), historyLockPoll)
	defer cancel()
	err = Record(ctx, Options{Enabled: true, Path: path}, Event{Command: "blocked", Status: "ok"})
	if err == nil || !strings.Contains(err.Error(), "history lock is busy") {
		t.Fatalf("Record with held lock error = %v, want busy lock error", err)
	}

	lock.release()
	if err := Record(context.Background(), Options{Enabled: true, Path: path}, Event{Command: "allowed", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	result, err := ReadWithStats(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].Command != "allowed" {
		t.Fatalf("unexpected events after lock release: %#v", result.Events)
	}
}

func TestHistoryLockRecoversStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "history.jsonl")
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("stale lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-historyLockStale - time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	if err := Record(context.Background(), Options{Enabled: true, Path: path}, Event{Command: "recovered", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	result, err := ReadWithStats(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].Command != "recovered" {
		t.Fatalf("unexpected events after stale lock recovery: %#v", result.Events)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file should be released after recovery, err=%v", err)
	}
}

func TestRedactsArgsAndParams(t *testing.T) {
	args := Args([]string{
		"auth", "login",
		"--client-secret", "secret-value",
		"--admin-token=admin-value",
		"--param", "token=param-token",
		"--query", "invoice",
	}, "all")
	joined := strings.Join(args, " ")
	for _, leaked := range []string{"secret-value", "admin-value", "param-token"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("redacted args leaked %q: %#v", leaked, args)
		}
	}
	if !strings.Contains(joined, "invoice") {
		t.Fatalf("non-secret arg was not preserved: %#v", args)
	}

	params := Params(map[string]string{
		"token":   "param-token",
		"filter":  "name has 'invoice'",
		"id":      "123",
		"id_file": "456",
	}, "all")
	if params["token"] != Redacted || params["filter"] == "" || params["id"] != "123" || params["id_file"] != "456" {
		t.Fatalf("unexpected all params: %#v", params)
	}
	safeParams := Params(params, "safe")
	if _, ok := safeParams["filter"]; ok {
		t.Fatalf("safe params should omit free-text values: %#v", safeParams)
	}
	if _, ok := safeParams["token"]; ok {
		t.Fatalf("safe params should omit sensitive values: %#v", safeParams)
	}
	if safeParams["id"] != "123" {
		t.Fatalf("safe params should keep stable IDs: %#v", safeParams)
	}
	if safeParams["id_file"] != "456" {
		t.Fatalf("safe params should keep file IDs: %#v", safeParams)
	}
}

func TestArgsHonorsParamPolicy(t *testing.T) {
	input := []string{
		"api", "call",
		"--param=token=secret-token",
		"--param=filter=name has 'invoice'",
		"--param=id_file=999",
		"get_users",
	}
	none := Args(input, "none")
	for _, value := range none {
		if strings.HasPrefix(value, "--param") {
			t.Fatalf("none policy should omit all --param entries: %#v", none)
		}
	}
	safe := Args(input, "safe")
	if !contains(safe, "--param=id_file=999") {
		t.Fatalf("safe policy should keep stable params: %#v", safe)
	}
	for _, leaked := range []string{"secret-token", "invoice", "filter="} {
		if strings.Contains(strings.Join(safe, " "), leaked) {
			t.Fatalf("safe policy leaked %q in args: %#v", leaked, safe)
		}
	}
	all := Args(input, "all")
	if !contains(all, "--param=token="+Redacted) || !contains(all, "--param=filter=name has 'invoice'") {
		t.Fatalf("all policy should preserve non-sensitive params and redact sensitive ones: %#v", all)
	}
}

func TestNameMatchingAvoidsSubstringFalsePositives(t *testing.T) {
	for _, name := range []string{"country-code", "countryCode", "zip-code", "unicode-mode", "keyboard", "api-keystone-region", "apiKeystoneRegion", "rehash", "hashed-output", "id_file"} {
		if IsSensitiveName(name) {
			t.Fatalf("%q should not be treated as sensitive", name)
		}
	}
	for _, name := range []string{"code", "auth-code", "authToken", "client-secret", "clientSecret", "ApiKey", "APIKey", "MyApiKey", "api-key", "oauthAccessKey", "privateKey", "refresh_token", "accessToken", "authorization-header", "request-hash"} {
		if !IsSensitiveName(name) {
			t.Fatalf("%q should be treated as sensitive", name)
		}
	}
	safe := Params(map[string]string{
		"id_file": "123",
		"filter":  "name has 'invoice'",
	}, "safe")
	if safe["id_file"] != "123" {
		t.Fatalf("safe params should retain id_file: %#v", safe)
	}
	if _, ok := safe["filter"]; ok {
		t.Fatalf("safe params should drop free-text filter: %#v", safe)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestReadWithStatsSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	raw := strings.Join([]string{
		`{"schema_version":"history.1","timestamp":"2026-06-07T12:00:00Z","command":"wsectl me","status":"ok","exit_code":0,"duration_ms":10}`,
		`partial-json`,
		`{"schema_version":"history.1","timestamp":"2026-06-07T12:01:00Z","command":"wsectl version","status":"ok","exit_code":0,"duration_ms":1}`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ReadWithStats(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Malformed != 1 || len(result.Events) != 1 || result.Events[0].Command != "wsectl version" {
		t.Fatalf("unexpected read result: %#v", result)
	}
}

func TestReadWithStatsLimitUsesTailRing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	for _, command := range []string{"one", "two", "three", "four"} {
		if err := Record(context.Background(), Options{Enabled: true, Path: path}, Event{Command: command, Status: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ReadWithStats(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 2 || result.Events[0].Command != "three" || result.Events[1].Command != "four" {
		t.Fatalf("unexpected limited events: %#v", result.Events)
	}
}

func TestFitEventCapsLargeEvents(t *testing.T) {
	event := Event{
		Command:        "wsectl tasks search",
		Status:         "ok",
		NormalizedArgs: []string{"tasks", "search", "--query", strings.Repeat("q", 8000)},
		Params: map[string]string{
			"filter": strings.Repeat("f", 8000),
			"id":     "123",
		},
		Warnings: []string{strings.Repeat("w", 8000)},
	}
	fit := FitEvent(event, MaxEventBytes)
	raw, err := json.Marshal(fit)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw)+1 > MaxEventBytes {
		t.Fatalf("event size = %d, want <= %d", len(raw)+1, MaxEventBytes)
	}
	if !strings.Contains(strings.Join(fit.Warnings, " "), "truncated") {
		t.Fatalf("fit event should include truncation warning: %#v", fit.Warnings)
	}
}
