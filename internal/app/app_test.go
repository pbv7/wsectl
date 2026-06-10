package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbv7/wsectl/internal/testutil"
	"github.com/pbv7/wsectl/internal/worksection"
)

// runCapture drives the real entry point with separated streams and returns
// the process exit code, so tests can assert the stdout/stderr split and the
// exit-code contract that the merged commands-package harness cannot see.
func runCapture(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	var out, errb bytes.Buffer
	err := RunWithIO(t.Context(), args, &out, &errb)
	return out.String(), errb.String(), ExitCode(err)
}

func TestMain(m *testing.M) {
	testutil.UnsetWsectlEnv()
	os.Exit(m.Run())
}

func TestAppMachineErrorAndExitCodes(t *testing.T) {
	plain := errors.New("bad flags")
	rendered := appMachineError(plain)
	if !rendered.SuppressPrint() || rendered.ExitCode() != 2 || !strings.Contains(rendered.Error(), "bad flags") {
		t.Fatalf("unexpected rendered plain error: %#v", rendered)
	}
	if !strings.Contains(rendered.Unwrap().Error(), "bad flags") {
		t.Fatalf("unwrap lost message: %v", rendered.Unwrap())
	}

	apiErr := &worksection.Error{Code: worksection.CodeNetwork, Message: "network down"}
	rendered = appMachineError(apiErr)
	if rendered.ExitCode() != 5 {
		t.Fatalf("rendered exit code = %d, want 5", rendered.ExitCode())
	}
	if ExitCode(nil) != 0 || ExitCode(errors.New("plain")) != 1 || ExitCode(apiErr) != 5 {
		t.Fatalf("unexpected ExitCode mapping")
	}
}

func TestRunInvalidCommandMachineError(t *testing.T) {
	err := Run(t.Context(), []string{"definitely-not-a-command", "--json"})
	if err == nil {
		t.Fatal("expected invalid command to fail")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit code = %d, want 2 (%v)", ExitCode(err), err)
	}
}

// B1: a usage error (unknown flag, wrong arg count) is exit 2 per the
// documented contract, and that must not depend on how output format was
// selected. Today the human-mode cases return exit 1.
func TestUsageErrorExitCodeIsFormatIndependent(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_OUTPUT", "")
	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag, human", []string{"projects", "list", "--badflag"}},
		{"unknown flag, json", []string{"projects", "list", "--badflag", "--json"}},
		{"missing required arg, human", []string{"projects", "get"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, exit := runCapture(t, tc.args...)
			if exit != 2 {
				t.Fatalf("exit = %d, want 2 (usage); stderr: %s", exit, strings.TrimSpace(stderr))
			}
		})
	}
}

// B2: when the machine output format comes from config defaults (not an
// explicit flag/env), a usage error must still render as a JSON envelope on
// stderr, not plain text. Today it prints plain text.
func TestUsageErrorHonorsConfigDefaultFormat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("[defaults]\noutput = \"json\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", cfgPath)
	t.Setenv("WSECTL_OUTPUT", "")

	stdout, stderr, exit := runCapture(t, "projects", "list", "--badflag")
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if stdout != "" {
		t.Fatalf("usage error leaked to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, `"status": "error"`) || !strings.Contains(stderr, `"code": "usage"`) {
		t.Fatalf("config default json format not honored for usage error; got:\n%s", stderr)
	}
}

// WSECTL_OUTPUT must still select the error format when the config file is
// unreadable (config.Load returns before applying env), matching the
// command-body path which falls back to the env setting. Otherwise top-level
// usage errors and command-body errors disagree under a malformed config.
func TestUsageErrorHonorsEnvWhenConfigUnreadable(t *testing.T) {
	dir := t.TempDir()
	badCfg := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(badCfg, []byte("not = [valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", badCfg)
	t.Setenv("WSECTL_OUTPUT", "json")

	_, stderr, exit := runCapture(t, "projects", "list", "--badflag")
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if !strings.Contains(stderr, `"code": "usage"`) {
		t.Fatalf("WSECTL_OUTPUT=json must select an envelope even when config is unreadable; got:\n%s", stderr)
	}
}

// An explicit human output selector (--table/--raw/--output table) must give
// a plain-text error even when the config default is a machine format, just as
// it gives human output for normal command results. Guards the precedence:
// explicit flag > config default.
func TestExplicitHumanFormatOverridesConfigDefaultForErrors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("[defaults]\noutput = \"json\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", cfgPath)
	t.Setenv("WSECTL_OUTPUT", "")

	for _, args := range [][]string{
		{"projects", "list", "--badflag", "--table"},
		{"projects", "list", "--badflag", "--raw"},
		{"projects", "list", "--badflag", "--output", "table"},
	} {
		_, stderr, exit := runCapture(t, args...)
		if exit != 2 {
			t.Fatalf("args %v: exit = %d, want 2", args, exit)
		}
		if strings.Contains(stderr, `"status"`) {
			t.Fatalf("args %v: explicit human format must render plain text, got an envelope:\n%s", args, stderr)
		}
	}
}

// A usage error must render in the same format the same flags would give
// successful output. Because shortcuts override --output in a fixed order
// (not by argument position), `--json --output table` resolves to json for
// both, so the error is a JSON envelope — not plain text.
func TestUsageErrorFormatMatchesShortcutPrecedence(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_OUTPUT", "")
	_, stderr, exit := runCapture(t, "projects", "list", "--badflag", "--json", "--output", "table")
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if !strings.Contains(stderr, `"code": "usage"`) {
		t.Fatalf("--json must win over --output table (matching successful output); got:\n%s", stderr)
	}
}

// A bare internal error escaping a command body (here a malformed 200 that
// fails JSON parsing) is a general failure (exit 1), not a usage error, and
// the exit code must not depend on output format. Guards against the
// top-level classifier over-broadly tagging non-usage errors as usage.
func TestInternalErrorStaysGeneralAcrossFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "tok")
	t.Setenv("WSECTL_OUTPUT", "")

	for _, args := range [][]string{{"me"}, {"me", "--json"}} {
		_, _, exit := runCapture(t, args...)
		if exit != 1 {
			t.Fatalf("args %v: exit = %d, want 1 (general, not usage)", args, exit)
		}
	}
}

// A --config after the -- terminator is positional and must not load a config
// file that changes error rendering. With WSECTL_CONFIG pointing at a missing
// file, the usage error stays plain text even though the positional config
// would have set a machine default.
func TestConfigAfterTerminatorDoesNotAffectErrorFormat(t *testing.T) {
	dir := t.TempDir()
	jsonCfg := filepath.Join(dir, "json.toml")
	if err := os.WriteFile(jsonCfg, []byte("[defaults]\noutput = \"json\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", filepath.Join(dir, "missing.toml"))
	t.Setenv("WSECTL_OUTPUT", "")

	_, stderr, exit := runCapture(t, "projects", "list", "--badflag", "--", "--config", jsonCfg)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2", exit)
	}
	if strings.Contains(stderr, `"status"`) {
		t.Fatalf("positional --config after -- changed error format to an envelope:\n%s", stderr)
	}
}

// A value-taking flag consumes the next token, so a shortcut-looking value is
// not a selector; the resulting error must render as human output, not a JSON
// envelope. Covers both a global value flag (--profile) and a subcommand value
// flag (--extra), since the resolver parses against the target command's full
// flag set. End-to-end check of the pflag-backed resolution.
func TestValueFlagConsumesShortcutToken(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_OUTPUT", "")
	for _, args := range [][]string{
		{"me", "--profile", "--json"},
		{"projects", "get", "--extra", "--json"},
	} {
		_, stderr, _ := runCapture(t, args...)
		if strings.Contains(stderr, `"status"`) {
			t.Fatalf("args %v: shortcut-looking value consumed by a value flag, output must be human, got an envelope:\n%s", args, stderr)
		}
	}
}

func TestClassifyTopLevelError(t *testing.T) {
	usage := errors.New("unknown flag: --badflag")              // bare cobra parse error
	apiErr := &worksection.Error{Code: worksection.CodeNetwork} // in-body, exit 5
	bare := errors.New("malformed response")                    // in-body internal error

	t.Run("pre-body bare error becomes usage exit 2", func(t *testing.T) {
		if got := ExitCode(classifyTopLevelError(usage, false)); got != 2 {
			t.Fatalf("exit = %d, want 2", got)
		}
	})
	t.Run("pre-body context cancellation stays general exit 1", func(t *testing.T) {
		if got := ExitCode(classifyTopLevelError(context.Canceled, false)); got != 1 {
			t.Fatalf("exit = %d, want 1 (cancellation is not a usage error)", got)
		}
	})
	t.Run("pre-body deadline stays general exit 1", func(t *testing.T) {
		if got := ExitCode(classifyTopLevelError(context.DeadlineExceeded, false)); got != 1 {
			t.Fatalf("exit = %d, want 1", got)
		}
	})
	t.Run("in-body error keeps its own code", func(t *testing.T) {
		if got := ExitCode(classifyTopLevelError(apiErr, true)); got != 5 {
			t.Fatalf("exit = %d, want 5 (in-body errors keep their classification)", got)
		}
	})
	t.Run("in-body bare error stays general exit 1", func(t *testing.T) {
		if got := ExitCode(classifyTopLevelError(bare, true)); got != 1 {
			t.Fatalf("exit = %d, want 1", got)
		}
	})
}

// secretToken is sent as the OAuth bearer credential in the matrix below; no
// case may echo it on either stream.
const secretToken = "super-secret-token-value-do-not-leak"

// TestExitCodeAndStreamMatrix drives each documented exit code through the real
// entry point against an httptest stub, asserting the exit code, the
// stdout/stderr split (success → stdout, error → stderr), and that the bearer
// token never leaks onto either stream.
func TestExitCodeAndStreamMatrix(t *testing.T) {
	okHandler := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","data":{"id":"1","name":"Ada"}}`))
	}
	statusHandler := func(code int) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"status":"error","message":"upstream said no"}`))
		}
	}
	errorBodyHandler := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"error","message":"Field is required: period"}`))
	}
	bigArrayHandler := func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","data":[`))
		for i := range 10000 {
			if i > 0 {
				_, _ = w.Write([]byte(","))
			}
			_, _ = w.Write([]byte(`{"id":"x"}`))
		}
		_, _ = w.Write([]byte(`]}`))
	}

	cases := []struct {
		name     string
		handler  http.HandlerFunc
		args     []string
		wantExit int
		wantCode string // error code in the envelope; "" for success
	}{
		{"success exit 0", okHandler, []string{"me", "--json"}, 0, ""},
		{"auth 401 exit 3", statusHandler(401), []string{"me", "--json"}, 3, "authentication"},
		{"authz 403 exit 4", statusHandler(403), []string{"me", "--json"}, 4, "authorization"},
		{"api 200-error exit 6", errorBodyHandler, []string{"me", "--json"}, 6, "worksection_api"},
		{"rate 429 exit 7", statusHandler(429), []string{"me", "--json"}, 7, "rate_limited"},
		{"truncated exit 8", bigArrayHandler, []string{"tasks", "all", "--json", "--fail-on-truncated"}, 8, "truncated"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()
			t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
			t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
			t.Setenv("WSECTL_ACCESS_TOKEN", secretToken)
			t.Setenv("WSECTL_OUTPUT", "")

			stdout, stderr, exit := runCapture(t, tc.args...)

			if exit != tc.wantExit {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", exit, tc.wantExit, stdout, stderr)
			}
			if tc.wantCode == "" {
				// Success: data on stdout, stderr clean.
				if !strings.Contains(stdout, `"status": "ok"`) {
					t.Fatalf("success envelope not on stdout:\n%s", stdout)
				}
				if strings.TrimSpace(stderr) != "" {
					t.Fatalf("stderr should be empty on success, got:\n%s", stderr)
				}
			} else {
				// Error: envelope on stderr, stdout clean.
				if strings.TrimSpace(stdout) != "" {
					t.Fatalf("error leaked to stdout:\n%s", stdout)
				}
				if !strings.Contains(stderr, `"status": "error"`) || !strings.Contains(stderr, `"code": "`+tc.wantCode+`"`) {
					t.Fatalf("error envelope missing/wrong code on stderr (want %q):\n%s", tc.wantCode, stderr)
				}
			}
			if strings.Contains(stdout, secretToken) || strings.Contains(stderr, secretToken) {
				t.Fatalf("bearer token leaked onto output\nstdout: %s\nstderr: %s", stdout, stderr)
			}
		})
	}
}

// Network failures (exit 5) are exercised by pointing at a closed listener so
// the connection is refused, rather than a live handler.
func TestNetworkErrorExit5(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close() // close immediately so the address refuses connections

	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", url)
	t.Setenv("WSECTL_ACCESS_TOKEN", secretToken)
	t.Setenv("WSECTL_OUTPUT", "")

	stdout, stderr, exit := runCapture(t, "me", "--json")
	if exit != 5 {
		t.Fatalf("exit = %d, want 5 (network)\nstderr: %s", exit, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("network error leaked to stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, `"code": "network"`) {
		t.Fatalf("network error code missing on stderr:\n%s", stderr)
	}
}
