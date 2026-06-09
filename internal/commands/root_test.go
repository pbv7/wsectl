package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbv7/wsectl/internal/auth"
	"github.com/pbv7/wsectl/internal/doctor"
	"github.com/pbv7/wsectl/internal/history"
	"github.com/pbv7/wsectl/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestMain(m *testing.M) {
	testutil.UnsetWsectlEnv()
	os.Exit(m.Run())
}

func execute(args ...string) (string, error) {
	return executeWithInput("", args...)
}

func executeWithInput(input string, args ...string) (string, error) {
	cmd := NewRoot("test", "commit", "date")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRootEntryPointsUseSameGuide(t *testing.T) {
	noArgs, err := execute()
	if err != nil {
		t.Fatal(err)
	}
	flagHelp, err := execute("--help")
	if err != nil {
		t.Fatal(err)
	}
	helpCommand, err := execute("help")
	if err != nil {
		t.Fatal(err)
	}
	if noArgs != flagHelp || noArgs != helpCommand {
		t.Fatal("wsectl, wsectl --help, and wsectl help must render identical output")
	}
	assertGolden(t, "no-args.txt", noArgs)
	if !strings.HasPrefix(noArgs, "wsectl - Unofficial command-line client for Worksection.") {
		t.Fatalf("unexpected title: %q", strings.SplitN(noArgs, "\n", 2)[0])
	}
	if strings.Index(noArgs, "Start here (for AI agents)") > strings.Index(noArgs, "Human quick start") {
		t.Fatal("agent entry point must appear before human setup")
	}
}

func TestAgentHelpCompactAndFull(t *testing.T) {
	compact, err := execute("help", "agent")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "agent-help.txt", compact)
	if strings.Contains(compact, "Command catalog") {
		t.Fatal("compact agent help unexpectedly contains the full command catalog")
	}

	full, err := execute("help", "agent", "--full")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "agent-help-full.txt", full)
	for _, want := range []string{"Setup and authentication", "Output and errors", "Safety model", "Worksection limits", "Command catalog"} {
		if !strings.Contains(full, want) {
			t.Fatalf("full agent help missing %q", want)
		}
	}
}

func TestHelpAgentJSON(t *testing.T) {
	out, err := execute("help", "agent", "--full", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Topic              string         `json:"topic"`
			Content            string         `json:"content"`
			GuideFormatVersion string         `json:"guide_format_version"`
			Full               bool           `json:"full"`
			Sections           []guideSection `json:"sections"`
			Commands           []commandInfo  `json:"commands"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if env.Status != "ok" || env.Data.Topic != "agent" || env.Data.GuideFormatVersion == "" || !env.Data.Full {
		t.Fatalf("unexpected guide envelope %#v", env.Data)
	}
	if env.Data.Content == "" || len(env.Data.Sections) == 0 || len(env.Data.Commands) == 0 {
		t.Fatalf("incomplete structured guide %#v", env.Data)
	}
}

func TestCommandsMetadataCoverage(t *testing.T) {
	root := NewRoot("test", "commit", "date")
	commands := collectCommands(root)
	if len(commands) < 50 {
		t.Fatalf("unexpectedly small command catalog: %d", len(commands))
	}
	byPath := map[string]commandInfo{}
	for _, info := range commands {
		byPath[info.Path] = info
		if info.Category == "" {
			t.Errorf("%s has no category", info.Path)
		}
		if info.Examples == "" {
			t.Errorf("%s has no examples", info.Path)
		}
		if info.AuthRequired && strings.HasPrefix(info.Path, "wsectl ") &&
			!strings.HasPrefix(info.Path, "wsectl auth ") && len(info.Actions) == 0 {
			t.Errorf("%s requires auth but has no action mapping", info.Path)
		}
	}
	for _, path := range []string{
		"wsectl help",
		"wsectl completion",
		"wsectl docs generate",
		"wsectl profiles list",
		"wsectl api actions",
		"wsectl api schema",
		"wsectl auth login",
		"wsectl auth status",
		"wsectl version",
	} {
		if info, ok := byPath[path]; !ok {
			t.Errorf("missing %s", path)
		} else if info.AuthRequired {
			t.Errorf("%s incorrectly requires API authentication", path)
		}
	}
	for path, action := range map[string]string{
		"wsectl me":            "me",
		"wsectl projects list": "get_projects",
		"wsectl tasks search":  "search_tasks",
		"wsectl webhooks list": "get_webhooks",
	} {
		if info := byPath[path]; len(info.Actions) != 1 || info.Actions[0] != action {
			t.Errorf("%s action mapping = %v, want %s", path, info.Actions, action)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	text, err := execute("version")
	if err != nil {
		t.Fatal(err)
	}
	if text != "wsectl test\n" {
		t.Fatalf("version text = %q", text)
	}
	out, err := execute("version", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Version   string `json:"version"`
			Commit    string `json:"commit"`
			Date      string `json:"date"`
			GoVersion string `json:"go_version"`
			OS        string `json:"os"`
			Arch      string `json:"arch"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid version json: %v\n%s", err, out)
	}
	if env.Status != "ok" || env.Data.Version != "test" || env.Data.Commit != "commit" || env.Data.Date != "date" || env.Data.GoVersion == "" || env.Data.OS == "" || env.Data.Arch == "" {
		t.Fatalf("unexpected version envelope %#v", env)
	}
}

func TestHistoryDisabledByDefault(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)
	out, err := execute("version")
	if err != nil {
		t.Fatal(err)
	}
	if out != "wsectl test\n" {
		t.Fatalf("history should not affect stdout: %q", out)
	}
	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Fatalf("history should be disabled by default, stat err=%v", err)
	}
}

func TestHistoryRecordsCommandAndCommandsWork(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY", "1")
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)

	out, err := execute("version")
	if err != nil {
		t.Fatal(err)
	}
	if out != "wsectl test\n" {
		t.Fatalf("history should not affect stdout: %q", out)
	}
	result, err := history.ReadWithStats(historyPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	events := result.Events
	if len(events) != 1 || events[0].Command != "wsectl version" || events[0].Status != "ok" || events[0].ExitCode != 0 {
		t.Fatalf("unexpected history event: %#v", events)
	}

	pathOut, err := execute("history", "path", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var pathEnv struct {
		Status string `json:"status"`
		Data   struct {
			Enabled bool   `json:"enabled"`
			Path    string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(pathOut), &pathEnv); err != nil {
		t.Fatalf("invalid history path json: %v\n%s", err, pathOut)
	}
	if pathEnv.Status != "ok" || !pathEnv.Data.Enabled || pathEnv.Data.Path != historyPath {
		t.Fatalf("unexpected history path output: %#v", pathEnv)
	}

	listOut, err := execute("history", "list", "--json", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	var listEnv struct {
		Status string          `json:"status"`
		Data   []history.Event `json:"data"`
	}
	if err := json.Unmarshal([]byte(listOut), &listEnv); err != nil {
		t.Fatalf("invalid history list json: %v\n%s", err, listOut)
	}
	if listEnv.Status != "ok" || len(listEnv.Data) != 1 || listEnv.Data[0].Command != "wsectl version" {
		t.Fatalf("unexpected history list output: %#v", listEnv)
	}

	if _, err := execute("history", "clear", "--json"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Fatalf("history clear should remove the file without rewriting it, stat err=%v", err)
	}
}

func TestHistorySkipsHelpCompletionAndHistoryCommands(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY", "1")
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)

	for _, args := range [][]string{
		{},
		{"--help"},
		{"help"},
		{"help", "agent"},
		{"auth", "login", "--help"},
		{"completion", "bash"},
		{"history", "path", "--json"},
		{"history", "list", "--json"},
	} {
		if _, err := execute(args...); err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
	}
	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Fatalf("noisy commands should not create history, stat err=%v", err)
	}
}

func TestHistoryListSkipsMalformedLinesAndSupportsFieldsAndJQ(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY", "1")
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)
	raw := strings.Join([]string{
		`{"schema_version":"history.1","timestamp":"2026-06-07T12:00:00Z","command":"wsectl me","status":"ok","exit_code":0,"duration_ms":10}`,
		`not-json`,
		`{"schema_version":"history.1","timestamp":"2026-06-07T12:01:00Z","command":"wsectl version","status":"ok","exit_code":0,"duration_ms":1}`,
		"",
	}, "\n")
	if err := os.WriteFile(historyPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	fieldsOut, err := execute("history", "list", "--json", "--fields", "command,status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fieldsOut, "Skipped 1 malformed history line") || strings.Contains(fieldsOut, "duration_ms") {
		t.Fatalf("history fields output did not project rows or warn:\n%s", fieldsOut)
	}

	jqOut, err := execute("history", "list", "--json", "--jq", ".data[-1].command")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(jqOut) != `"wsectl version"` {
		t.Fatalf("unexpected history jq output: %q", jqOut)
	}
}

func TestHistoryClearKeepRetainsLatestEntries(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY", "1")
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)
	raw := strings.Join([]string{
		`{"schema_version":"history.1","timestamp":"2026-06-07T12:00:00Z","command":"wsectl me","status":"ok","exit_code":0,"duration_ms":10}`,
		`{"schema_version":"history.1","timestamp":"2026-06-07T12:01:00Z","command":"wsectl version","status":"ok","exit_code":0,"duration_ms":1}`,
		"",
	}, "\n")
	if err := os.WriteFile(historyPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute("history", "clear", "--keep", "1", "--json"); err != nil {
		t.Fatal(err)
	}
	result, err := history.ReadWithStats(historyPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].Command != "wsectl version" {
		t.Fatalf("history clear --keep retained wrong entries: %#v", result.Events)
	}
}

func TestHistoryParamsNoneOmitsParamsFromNormalizedArgs(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY", "1")
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)
	t.Setenv("WSECTL_HISTORY_PARAMS", "none")
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")

	out, err := execute("api", "call", "get_users", "--allow-unknown", "--param", "token=secret-token", "--param", "filter=name has 'invoice'", "--json")
	if err == nil {
		t.Fatal("expected auth error")
	}
	if strings.Contains(out, "secret-token") {
		t.Fatalf("command output leaked token:\n%s", out)
	}
	result, readErr := history.ReadWithStats(historyPath, 0)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected one history event, got %#v", result.Events)
	}
	event := result.Events[0]
	if len(event.Params) != 0 {
		t.Fatalf("none policy should omit params: %#v", event.Params)
	}
	joined := strings.Join(event.NormalizedArgs, " ")
	for _, leaked := range []string{"--param", "secret-token", "invoice", "filter="} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("none policy leaked %q in normalized args: %#v", leaked, event.NormalizedArgs)
		}
	}
}

func TestHistoryRecordsFailuresWithoutSecretLeaks(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history.jsonl")
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_HISTORY", "1")
	t.Setenv("WSECTL_HISTORY_FILE", historyPath)
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")

	out, err := execute("api", "call", "get_users", "--allow-unknown", "--param", "token=secret-token", "--param", "filter=name has 'invoice'", "--json")
	if err == nil {
		t.Fatal("expected auth error")
	}
	if strings.Contains(out, "secret-token") {
		t.Fatalf("command output leaked token:\n%s", out)
	}
	result, readErr := history.ReadWithStats(historyPath, 0)
	if readErr != nil {
		t.Fatal(readErr)
	}
	events := result.Events
	if len(events) != 1 {
		t.Fatalf("expected one history event, got %#v", events)
	}
	event := events[0]
	if event.Status != "error" || event.ExitCode != 3 || event.ErrorCode != "authentication" || event.Action != "get_users" {
		t.Fatalf("unexpected failure event: %#v", event)
	}
	if event.Params["token"] != history.Redacted || event.Params["filter"] != "name has 'invoice'" {
		t.Fatalf("unexpected params redaction: %#v", event.Params)
	}
	if strings.Contains(strings.Join(event.NormalizedArgs, " "), "secret-token") {
		t.Fatalf("history normalized args leaked token: %#v", event.NormalizedArgs)
	}
	paramArgs := 0
	for _, arg := range event.NormalizedArgs {
		if arg == "--param" {
			t.Fatalf("history normalized args should not emit bare --param without value: %#v", event.NormalizedArgs)
		}
		if strings.HasPrefix(arg, "--param=") {
			paramArgs++
		}
	}
	if paramArgs != 2 || !containsString(event.NormalizedArgs, "--param=token="+history.Redacted) || !containsString(event.NormalizedArgs, "--param=filter=name has 'invoice'") {
		t.Fatalf("history normalized args did not preserve repeated params safely: %#v", event.NormalizedArgs)
	}
	if containsString(event.NormalizedArgs, "api") || containsString(event.NormalizedArgs, "call") {
		t.Fatalf("history normalized args should not duplicate command path: %#v", event.NormalizedArgs)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestSecretShapedFlagsAreRedactedOrAllowlisted(t *testing.T) {
	allow := map[string]bool{
		"auth-type":   true,
		"author":      true,
		"manual-code": true,
	}
	root := NewRoot("test", "commit", "date")
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if !secretShapedFlag(flag.Name) || allow[flag.Name] {
				return
			}
			if !history.IsSensitiveName(flag.Name) {
				t.Errorf("%s flag --%s looks sensitive but is not redacted", cmd.CommandPath(), flag.Name)
			}
		})
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
}

func secretShapedFlag(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	for _, marker := range []string{"token", "secret", "password", "passphrase", "key", "hash", "code", "bearer", "auth"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func TestCommandsJSONPreservesAndExtendsContract(t *testing.T) {
	out, err := execute("commands", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data []commandInfo `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid commands JSON: %v", err)
	}
	for _, info := range env.Data {
		if info.Path == "wsectl tasks search" {
			if info.Category != "tasks" || !info.AuthRequired || !info.ReadOnly || !info.Output {
				t.Fatalf("incorrect tasks search metadata: %#v", info)
			}
			return
		}
	}
	t.Fatal("commands output missing wsectl tasks search")
}

func TestAPICallRejectsDuplicateParams(t *testing.T) {
	out, err := execute("api", "call", "get_users", "--param", "id=1", "--param", "id=2", "--json")
	if err == nil {
		t.Fatal("expected duplicate param usage error")
	}
	if !strings.Contains(out, `"status": "error"`) || !strings.Contains(out, "provided more than once") {
		t.Fatalf("unexpected duplicate-param output:\n%s", out)
	}
}

func TestCommandSchemaDoesNotRequireCredentials(t *testing.T) {
	out, err := execute("tasks", "search", "--schema", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Name     string `json:"name"`
			Response struct {
				Shape           string `json:"response_shape"`
				ContractVersion string `json:"contract_version"`
			} `json:"response"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid schema json: %v\n%s", err, out)
	}
	if env.Status != "ok" || env.Data.Name != "search_tasks" || env.Data.Response.Shape != "array" || env.Data.Response.ContractVersion == "" {
		t.Fatalf("unexpected schema output: %#v", env)
	}
}

func TestCommandSchemaHonorsYAMLFormat(t *testing.T) {
	out, err := execute("tasks", "search", "--schema", "--yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "status: ok") || !strings.Contains(out, "name: search_tasks") {
		t.Fatalf("expected YAML schema output, got:\n%s", out)
	}
	if strings.Contains(out, `"status": "ok"`) {
		t.Fatalf("schema rendered as JSON despite --yaml:\n%s", out)
	}
}

func TestCommandValidationBeforeAuth(t *testing.T) {
	tests := [][]string{
		{"tasks", "all", "--status", "done", "--json"},
		{"files", "list", "--json"},
	}
	for _, args := range tests {
		out, err := execute(args...)
		if err == nil {
			t.Fatalf("%v should fail validation", args)
		}
		if !strings.Contains(out, `"status": "error"`) {
			t.Fatalf("%v should emit json error before auth, got:\n%s", args, out)
		}
	}
}

func TestMissingCredentialsJSONErrorIsParseable(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")
	t.Setenv("WSECTL_ACCESS_TOKEN", "")
	t.Setenv("WSECTL_REFRESH_TOKEN", "")
	t.Setenv("WSECTL_ADMIN_TOKEN", "")
	t.Setenv("WSECTL_CLIENT_ID", "")
	out, err := execute("me", "--json")
	if err == nil {
		t.Fatal("expected auth error")
	}
	var env struct {
		Status string `json:"status"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(out), &env); jsonErr != nil {
		t.Fatalf("error output must be a single JSON envelope: %v\n%s", jsonErr, out)
	}
	if env.Status != "error" || env.Error.Code != "authentication" {
		t.Fatalf("unexpected error envelope %#v", env)
	}
	if strings.Contains(out, "\nexit status") {
		t.Fatalf("unexpected plaintext process error in command output:\n%s", out)
	}
}

func TestRawOutputHonorsOutFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Query().Get("action") != "get_users" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":[{"id":"1"}]}`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")

	outPath := filepath.Join(t.TempDir(), "raw.json")
	out, err := execute("api", "call", "get_users", "--raw", "--out", outPath)
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Fatalf("raw --out should not write stdout/stderr, got %q", out)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"status":"ok","data":[{"id":"1"}]}` {
		t.Fatalf("raw file was not exact: %q", raw)
	}
}

func TestRawWarnsWhenTransformsAreIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","data":[{"id":"1"},{"id":"2"}]}`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")

	out, err := execute("api", "call", "get_users", "--raw", "--fields", "id", "--limit", "1", "--jq", ".data")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"warning: --fields is ignored with --raw",
		"warning: --limit is ignored with --raw",
		"warning: --jq is ignored with --raw",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected warning %q in output:\n%s", want, out)
		}
	}
	if !strings.Contains(out, `{"status":"ok","data":[{"id":"1"},{"id":"2"}]}`) {
		t.Fatalf("raw body was not preserved verbatim:\n%s", out)
	}
}

func TestRawQuietSuppressesWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","data":[{"id":"1"}]}`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")

	out, err := execute("api", "call", "get_users", "--raw", "--fields", "id", "--quiet")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "warning:") {
		t.Fatalf("--quiet should suppress raw warnings:\n%s", out)
	}
}

func TestTableUsesActionCuratedColumns(t *testing.T) {
	// get_users declares cols("id","name","email","role") at the action layer.
	// Without --fields the table renderer must honor those curated columns,
	// not fall back to the alphabetical preferredKeys heuristic. Returning
	// extra fields (department, group) lets us prove the curated set wins
	// rather than coincidentally matching the default order.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","data":[
			{"id":"1","name":"Ada","email":"ada@example.com","role":"admin","department":"Eng","group":{"id":"7","name":"Core"}}
		]}`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")

	out, err := execute("users", "list", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ID", "NAME", "EMAIL", "ROLE"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected curated column %q in header:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"DEPARTMENT", "GROUP"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("non-curated column %q rendered; action TableColumns not honored:\n%s", unwanted, out)
		}
	}
	// Order: id, name, email, role — exactly as cols(...) declares.
	positions := []int{
		strings.Index(out, "ID"),
		strings.Index(out, "NAME"),
		strings.Index(out, "EMAIL"),
		strings.Index(out, "ROLE"),
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Fatalf("curated column order not preserved (positions %v):\n%s", positions, out)
		}
	}
	if strings.Contains(out, "Note: table output shows") {
		t.Fatalf("omitted-columns note should not appear when curated set is used:\n%s", out)
	}
}

func TestVerboseEnvFallbackDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer env-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"status":"ok","data":{"id":"1","name":"Ada"}}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	missingSecretPath := filepath.Join(dir, "missing-secret.json")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(`current_profile = "default"

[defaults]
output = "json"
rate_limit = "1/s"
timeout = "30s"

[profiles.default]
account_url = "https://company.worksection.com"
auth_type = "oauth2"
secret_ref = %q
`, "plaintext:"+missingSecretPath)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", configPath)
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "env-token")

	out, err := execute("me", "--verbose", "--output", "table")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "using environment credentials") || !strings.Contains(out, "plaintext:") {
		t.Fatalf("verbose env fallback diagnostic missing:\n%s", out)
	}
	if strings.Contains(out, "env-token") {
		t.Fatalf("verbose output exposed env token:\n%s", out)
	}
}

func TestFilesDownloadVerboseEnvFallbackDiagnostic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("action") != "download" || r.URL.Query().Get("id_file") != "file-1" {
			t.Fatalf("unexpected request %s", r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer env-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("download bytes"))
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	missingSecretPath := filepath.Join(dir, "missing-secret.json")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(`current_profile = "default"

[defaults]
output = "table"
rate_limit = "1/s"
timeout = "30s"

[profiles.default]
account_url = "https://company.worksection.com"
auth_type = "oauth2"
secret_ref = %q
`, "plaintext:"+missingSecretPath)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", configPath)
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "env-token")

	outPath := filepath.Join(dir, "download.bin")
	out, err := execute("files", "download", "file-1", "--out", outPath, "--verbose")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "using environment credentials") || !strings.Contains(out, "plaintext:") {
		t.Fatalf("verbose env fallback diagnostic missing from download:\n%s", out)
	}
	if strings.Contains(out, "env-token") {
		t.Fatalf("verbose output exposed env token:\n%s", out)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "download bytes" {
		t.Fatalf("download output = %q", raw)
	}
}

func TestDoctorJSONErrorIsSingleEnvelope(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")
	t.Setenv("WSECTL_ACCESS_TOKEN", "")
	t.Setenv("WSECTL_ADMIN_TOKEN", "")
	out, err := execute("doctor", "--json")
	if err == nil {
		t.Fatal("expected doctor error")
	}
	var env struct {
		Status string `json:"status"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(out), &env); jsonErr != nil {
		t.Fatalf("doctor output must be one JSON envelope: %v\n%s", jsonErr, out)
	}
	if env.Status != "error" || env.Error.Code == "" {
		t.Fatalf("unexpected doctor envelope %#v", env)
	}
}

func TestDoctorTextOutputIncludesChecksAndRemediation(t *testing.T) {
	report := doctor.Report{
		Healthy:     false,
		ConfigPath:  "/tmp/config.toml",
		Profile:     "default",
		AccountURL:  "https://company.worksection.com",
		Remediation: []string{"Run `wsectl auth login`."},
		Checks: []doctor.Check{
			{Name: "config_file", Status: doctor.StatusOK, Message: "config is readable"},
			{Name: "credentials", Status: doctor.StatusFail, Message: "OAuth access token is missing", Remediation: "Run `wsectl auth login`."},
		},
	}
	var out bytes.Buffer
	if err := writeDoctorText(&out, report); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"wsectl doctor", "[ok] config_file", "[fail] credentials", "Remediation:", "Overall: unhealthy"} {
		if !strings.Contains(text, want) {
			t.Fatalf("doctor text missing %q:\n%s", want, text)
		}
	}
}

func TestDoctorAPIActionMatchesAuthMode(t *testing.T) {
	if got := doctorAPIAction("admin_token"); got != "get_users" {
		t.Fatalf("admin-token doctor action = %q, want get_users", got)
	}
	for _, authType := range []string{"", "oauth2", "unknown"} {
		if got := doctorAPIAction(authType); got != "me" {
			t.Fatalf("%q doctor action = %q, want me", authType, got)
		}
	}
}

func TestTaskQueryFilterEscapesQuotes(t *testing.T) {
	got := taskNameFilter("Bob's invoice")
	want := "name has 'Bob\\'s invoice'"
	if got != want {
		t.Fatalf("taskNameFilter = %q, want %q", got, want)
	}
}

func TestFilesSelectorValidationBeforeAuth(t *testing.T) {
	out, err := execute("files", "list", "--project", "123", "--task", "456", "--json")
	if err == nil {
		t.Fatal("expected selector validation error")
	}
	if !strings.Contains(out, "requires exactly one") {
		t.Fatalf("unexpected validation output:\n%s", out)
	}
}

func TestCostTimerFlagSendsIsTimer(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"status":"ok","data":[]}`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")
	if _, err := execute("costs", "list", "--project", "123", "--timer", "true", "--json"); err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("action") != "get_costs" || values.Get("id_project") != "123" || values.Get("is_timer") != "true" {
		t.Fatalf("unexpected query %s", query)
	}
	if _, ok := values["timer"]; ok {
		t.Fatalf("public timer flag leaked as raw timer param: %s", query)
	}
}

func TestCostsListCompositeJSONCountsPrimaryRows(t *testing.T) {
	setupCompositeCostServer(t)
	out, err := execute("costs", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Data  []map[string]any `json:"data"`
			Total map[string]any   `json:"total"`
		} `json:"data"`
		Meta struct {
			Count int `json:"count"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("invalid costs json: %v\n%s", err, out)
	}
	if env.Status != "ok" || env.Meta.Count != 2 || len(env.Data.Data) != 2 || env.Data.Total["money"] != "30" {
		t.Fatalf("unexpected costs envelope: %#v", env)
	}
}

func TestCostsListCompositeNDJSONUsesPrimaryRows(t *testing.T) {
	setupCompositeCostServer(t)
	out, err := execute("costs", "list", "--ndjson")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || strings.Contains(out, `"total"`) || !strings.Contains(out, `"id":"1"`) || !strings.Contains(out, `"id":"2"`) {
		t.Fatalf("unexpected costs ndjson:\n%s", out)
	}
}

func TestCostsListCompositeLimitAndFieldsUsePrimaryRows(t *testing.T) {
	setupCompositeCostServer(t)
	limited, err := execute("costs", "list", "--limit", "1", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(limited, `"id": "2"`) || !strings.Contains(limited, `"total"`) || !strings.Contains(limited, "Client-side --limit") {
		t.Fatalf("unexpected limited costs output:\n%s", limited)
	}

	setupCompositeCostServer(t)
	selected, err := execute("costs", "list", "--fields", "id,task.name", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(selected, `"task"`) || !strings.Contains(selected, `"Task A"`) || !strings.Contains(selected, `"total"`) {
		t.Fatalf("selected costs output missing expected fields:\n%s", selected)
	}
	if strings.Contains(selected, "Design") || strings.Contains(selected, `"money": "10"`) {
		t.Fatalf("selected costs output kept unselected row fields:\n%s", selected)
	}
}

func TestTaskSearchTaskFlagSendsIDTask(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"status":"ok","data":[]}`))
	}))
	defer server.Close()
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")
	if _, err := execute("tasks", "search", "--task", "456", "--json"); err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("action") != "search_tasks" || values.Get("id_task") != "456" {
		t.Fatalf("unexpected query %s", query)
	}
}

func TestImageFiltering(t *testing.T) {
	raw, warnings, err := filterImageFiles(json.RawMessage(`[
		{"id":"1","name":"photo.jpg"},
		{"id":"2","name":"document.pdf"},
		{"id":"3","type":"image/png","name":"inline"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected client-side filtering warning")
	}
	text := string(raw)
	if !strings.Contains(text, `"id":"1"`) || !strings.Contains(text, `"id":"3"`) || strings.Contains(text, `"id":"2"`) {
		t.Fatalf("unexpected filtered images: %s", text)
	}
}

func TestCommandSpecificHelpRemainsFocused(t *testing.T) {
	out, err := execute("help", "tasks", "search")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Search tasks", "--query", "--filter", "--project", "--task", "--json", "--out"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task search help missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Human quick start") {
		t.Fatal("subcommand help rendered the root start screen")
	}
}

func TestCompletionCommandsAreAvailable(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out, err := execute("completion", shell)
		if err != nil {
			t.Fatalf("completion %s failed: %v", shell, err)
		}
		if !strings.Contains(out, "wsectl") {
			t.Fatalf("completion %s output does not mention wsectl", shell)
		}
	}
}

func TestHelpCompletion(t *testing.T) {
	out, err := execute("help", "completion")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wsectl completion bash") || !strings.Contains(out, "powershell") {
		t.Fatalf("unexpected completion help: %s", out)
	}
}

func TestDoctorDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")
	t.Setenv("WSECTL_ACCESS_TOKEN", "access-token-must-not-appear")
	t.Setenv("WSECTL_CLIENT_SECRET", "client-secret-must-not-appear")
	out, err := execute("doctor", "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"access-token-must-not-appear", "client-secret-must-not-appear"} {
		if strings.Contains(out, secret) {
			t.Fatalf("doctor output exposed %q", secret)
		}
	}
}

func TestAuthLoginAdminTokenStdinStoresSecretWithoutPrintingIt(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "admin_token")
	out, err := executeWithInput("admin-secret-from-stdin\n", "auth", "login", "--admin-token-stdin", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "admin-secret-from-stdin") {
		t.Fatalf("auth login output exposed stdin secret:\n%s", out)
	}
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "admin-secret-from-stdin") {
		t.Fatalf("stdin secret was not stored: %s", raw)
	}
}

func TestAuthStatusAndLogoutPlaintextProfile(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "oauth2")
	if _, err := execute("auth", "login", "--access-token", "access-token-must-not-print", "--json"); err != nil {
		t.Fatal(err)
	}

	status, err := execute("auth", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(status, "access-token-must-not-print") {
		t.Fatalf("auth status exposed access token:\n%s", status)
	}
	var statusEnv struct {
		Status string `json:"status"`
		Data   struct {
			Authenticated bool   `json:"authenticated"`
			SecretRef     string `json:"secret_ref"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(status), &statusEnv); err != nil {
		t.Fatalf("invalid auth status json: %v\n%s", err, status)
	}
	if statusEnv.Status != "ok" || !statusEnv.Data.Authenticated || !strings.HasPrefix(statusEnv.Data.SecretRef, "plaintext:") {
		t.Fatalf("unexpected auth status envelope: %#v", statusEnv)
	}

	logout, err := execute("auth", "logout", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logout, "access-token-must-not-print") {
		t.Fatalf("auth logout exposed access token:\n%s", logout)
	}
	if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
		t.Fatalf("logout should delete plaintext secret, stat err=%v", err)
	}

	status, err = execute("auth", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(status), &statusEnv); err != nil {
		t.Fatalf("invalid post-logout auth status json: %v\n%s", err, status)
	}
	if statusEnv.Data.Authenticated {
		t.Fatalf("post-logout auth status still authenticated: %#v", statusEnv)
	}
}

func TestAuthLoginPlaintextWarnsOnlyInHumanOutput(t *testing.T) {
	writePlaintextProfileConfig(t, "oauth2")
	out, err := execute("auth", "login", "--access-token", "access")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "warning: plaintext secret storage is enabled") {
		t.Fatalf("expected plaintext warning in human output:\n%s", out)
	}

	writePlaintextProfileConfig(t, "oauth2")
	out, err = execute("auth", "login", "--access-token", "access", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "plaintext secret storage") {
		t.Fatalf("plaintext warning contaminated machine output:\n%s", out)
	}
	var env struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || env.Status != "ok" {
		t.Fatalf("invalid machine output err=%v out=%s", err, out)
	}
}

func TestAuthLoginClientSecretStdinStoresSecretWithoutPrintingIt(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "oauth2")
	out, err := executeWithInput("client-secret-from-stdin\n", "auth", "login", "--client-id", "client", "--client-secret-stdin", "--access-token", "access", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "client-secret-from-stdin") {
		t.Fatalf("auth login output exposed stdin secret:\n%s", out)
	}
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "client-secret-from-stdin") {
		t.Fatalf("stdin client secret was not stored: %s", raw)
	}
}

func TestAuthLoginClientSecretStdinRequiresInput(t *testing.T) {
	writePlaintextProfileConfig(t, "oauth2")
	out, err := executeWithInput("", "auth", "login", "--client-id", "client", "--client-secret-stdin", "--access-token", "access", "--json")
	if err == nil {
		t.Fatal("expected empty stdin to fail")
	}
	if !strings.Contains(out, `"status": "error"`) || !strings.Contains(out, "expected one secret value on stdin") {
		t.Fatalf("unexpected empty stdin output:\n%s", out)
	}
}

func TestAuthLoginAdminTokenCanComeFromEnv(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "admin_token")
	t.Setenv("WSECTL_ADMIN_TOKEN", "admin-secret-from-env")
	out, err := execute("auth", "login", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "admin-secret-from-env") {
		t.Fatalf("auth login output exposed env secret:\n%s", out)
	}
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "admin-secret-from-env") {
		t.Fatalf("env admin token was not stored: %s", raw)
	}
}

func TestAuthLoginOAuthIgnoresAdminTokenEnvForManualCode(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "oauth2")
	t.Setenv("WSECTL_CLIENT_SECRET", "oauth-secret-from-env")
	t.Setenv("WSECTL_ADMIN_TOKEN", "admin-secret-must-be-ignored")
	out, err := execute("auth", "login", "--client-id", "client", "--manual-code")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"oauth-secret-from-env", "admin-secret-must-be-ignored"} {
		if strings.Contains(out, secret) {
			t.Fatalf("manual code output exposed %q:\n%s", secret, out)
		}
	}
	if strings.Contains(out, "--client-secret ") || strings.Contains(out, "[REDACTED]") {
		t.Fatalf("manual code output still recommends client-secret flag:\n%s", out)
	}
	if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
		t.Fatalf("manual code should not store credentials before code exchange; stat error: %v", err)
	}
}

func TestAuthLoginOAuthDirectTokenIgnoresAdminTokenEnv(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "oauth2")
	t.Setenv("WSECTL_ADMIN_TOKEN", "admin-secret-must-be-ignored")
	out, err := execute("auth", "login", "--access-token", "access-token-from-flag", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "admin-secret-must-be-ignored") || strings.Contains(out, "access-token-from-flag") {
		t.Fatalf("auth login output exposed secret:\n%s", out)
	}
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "access-token-from-flag") {
		t.Fatalf("access token was not stored: %s", raw)
	}
	if strings.Contains(text, "admin-secret-must-be-ignored") {
		t.Fatalf("oauth login stored admin token env value: %s", raw)
	}
}

func TestAuthLoginOAuthRejectsExplicitAdminToken(t *testing.T) {
	writePlaintextProfileConfig(t, "oauth2")
	out, err := execute("auth", "login", "--admin-token", "admin-secret-from-flag", "--json")
	if err == nil {
		t.Fatal("expected explicit admin token on oauth2 profile to fail")
	}
	if !strings.Contains(out, `"status": "error"`) || !strings.Contains(out, "--admin-token cannot be used with an oauth2 profile") {
		t.Fatalf("unexpected wrong-mode output:\n%s", out)
	}
	if strings.Contains(out, "admin-secret-from-flag") {
		t.Fatalf("wrong-mode output exposed admin token:\n%s", out)
	}
}

func TestAuthLoginAdminTokenIgnoresOAuthEnv(t *testing.T) {
	secretPath := writePlaintextProfileConfig(t, "admin_token")
	t.Setenv("WSECTL_CLIENT_ID", "client-id-must-be-ignored")
	t.Setenv("WSECTL_CLIENT_SECRET", "client-secret-must-be-ignored")
	t.Setenv("WSECTL_ADMIN_TOKEN", "admin-secret-from-env")
	out, err := execute("auth", "login", "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"client-id-must-be-ignored", "client-secret-must-be-ignored", "admin-secret-from-env"} {
		if strings.Contains(out, secret) {
			t.Fatalf("auth login output exposed %q:\n%s", secret, out)
		}
	}
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "admin-secret-from-env") {
		t.Fatalf("admin token was not stored: %s", raw)
	}
	for _, forbidden := range []string{"client-id-must-be-ignored", "client-secret-must-be-ignored"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("admin-token login stored oauth env value %q: %s", forbidden, raw)
		}
	}
}

func TestAuthLoginAdminTokenRejectsExplicitOAuthFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		in   string
		want string
	}{
		{name: "client-id", args: []string{"--client-id", "client-id-from-flag"}, want: "--client-id cannot be used with an admin_token profile"},
		{name: "client-secret", args: []string{"--client-secret", "client-secret-from-flag"}, want: "--client-secret cannot be used with an admin_token profile"},
		{name: "client-secret-stdin", args: []string{"--client-secret-stdin"}, in: "client-secret-from-stdin\n", want: "--client-secret-stdin cannot be used with an admin_token profile"},
		{name: "access-token", args: []string{"--access-token", "access-token-from-flag"}, want: "--access-token cannot be used with an admin_token profile"},
		{name: "refresh-token", args: []string{"--refresh-token", "refresh-token-from-flag"}, want: "--refresh-token cannot be used with an admin_token profile"},
		{name: "code", args: []string{"--code", "code-from-flag"}, want: "--code cannot be used with an admin_token profile"},
		{name: "manual-code", args: []string{"--manual-code"}, want: "--manual-code cannot be used with an admin_token profile"},
		{name: "scope", args: []string{"--scope", "projects_read"}, want: "--scope cannot be used with an admin_token profile"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writePlaintextProfileConfig(t, "admin_token")
			t.Setenv("WSECTL_ADMIN_TOKEN", "admin-secret-from-env")
			args := append([]string{"auth", "login", "--json"}, tc.args...)
			out, err := executeWithInput(tc.in, args...)
			if err == nil {
				t.Fatal("expected explicit oauth flag on admin_token profile to fail")
			}
			if !strings.Contains(out, `"status": "error"`) || !strings.Contains(out, tc.want) {
				t.Fatalf("unexpected wrong-mode output:\n%s", out)
			}
			for _, secret := range []string{"client-id-from-flag", "client-secret-from-flag", "client-secret-from-stdin", "access-token-from-flag", "refresh-token-from-flag", "code-from-flag", "admin-secret-from-env"} {
				if strings.Contains(out, secret) {
					t.Fatalf("wrong-mode output exposed %q:\n%s", secret, out)
				}
			}
		})
	}
}

func TestAuthLoginOAuthRejectsExplicitAdminTokenStdin(t *testing.T) {
	writePlaintextProfileConfig(t, "oauth2")
	out, err := executeWithInput("admin-secret-from-stdin\n", "auth", "login", "--admin-token-stdin", "--json")
	if err == nil {
		t.Fatal("expected explicit admin token stdin on oauth2 profile to fail")
	}
	if !strings.Contains(out, `"status": "error"`) || !strings.Contains(out, "--admin-token-stdin cannot be used with an oauth2 profile") {
		t.Fatalf("unexpected wrong-mode output:\n%s", out)
	}
	if strings.Contains(out, "admin-secret-from-stdin") {
		t.Fatalf("wrong-mode output exposed admin token:\n%s", out)
	}
}

func TestAuthLoginStdinSecretConflictsWithValueFlag(t *testing.T) {
	writePlaintextProfileConfig(t, "oauth2")
	out, err := executeWithInput("stdin-secret\n", "auth", "login", "--client-id", "client", "--client-secret", "flag-secret", "--client-secret-stdin", "--json")
	if err == nil {
		t.Fatal("expected conflicting secret flags to fail")
	}
	if !strings.Contains(out, `"status": "error"`) || strings.Contains(out, "stdin-secret") || strings.Contains(out, "flag-secret") {
		t.Fatalf("unexpected conflict output:\n%s", out)
	}
}

func TestStoreLoginSecretRequiresReadableSecret(t *testing.T) {
	store := unreadableSecretStore{}
	err := storeLoginSecret(context.Background(), store, auth.SecretRef{Scheme: "keyring", Name: "wsectl/default"}, "oauth2", auth.SecretBundle{AccessToken: "access"})
	if err == nil {
		t.Fatal("expected unreadable stored secret to fail")
	}
	if !strings.Contains(err.Error(), "stored credentials could not be read back") || strings.Contains(err.Error(), "access") {
		t.Fatalf("unexpected store verification error: %v", err)
	}
}

func TestProfilesCRUDRoundTrip(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("WSECTL_CONFIG", configPath)

	if _, err := execute("profiles", "add", "default", "--account-url", "https://company.worksection.com", "--auth-type", "oauth2", "--secret-ref", "env:"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute("profiles", "add", "admin", "--account-url", "https://admin.worksection.com", "--auth-type", "admin_token", "--secret-ref", "plaintext:/tmp/wsectl-admin.json"); err != nil {
		t.Fatal(err)
	}

	list, err := execute("profiles", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(list, `"default"`) || !strings.Contains(list, `"admin"`) {
		t.Fatalf("profiles list missing expected profiles:\n%s", list)
	}

	show, err := execute("profiles", "show", "admin", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(show, `"AuthType": "admin_token"`) || !strings.Contains(show, "https://admin.worksection.com") {
		t.Fatalf("profiles show returned unexpected output:\n%s", show)
	}

	if _, err := execute("profiles", "use", "admin"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `current_profile = "admin"`) {
		t.Fatalf("profiles use did not persist active profile:\n%s", raw)
	}

	if _, err := execute("profiles", "remove", "default"); err != nil {
		t.Fatal(err)
	}
	removed, err := execute("profiles", "show", "default", "--json")
	if err == nil || (!strings.Contains(removed, `profile \"default\" not found`) && !strings.Contains(err.Error(), `profile "default" not found`)) {
		t.Fatalf("profiles show for removed profile should fail, err=%v out=%s", err, removed)
	}
}

func TestGeneratedCommandReferenceIsCurrent(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "docs", "command-reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := renderCommandReference(collectCommands(NewRoot("test", "commit", "date")))
	if normalizeText(got) != normalizeText(string(want)) {
		t.Fatal("docs/command-reference.md is stale; run `wsectl docs generate --out docs/command-reference.md`")
	}
}

func setupCompositeCostServer(t *testing.T) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Query().Get("action") != "get_costs" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{
			"status":"ok",
			"data":[
				{"id":"1","comment":"Design","money":"10","task":{"name":"Task A"}},
				{"id":"2","comment":"Build","money":"20","task":{"name":"Task B"}}
			],
			"total":{"money":"30"}
		}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", server.URL)
	t.Setenv("WSECTL_ACCESS_TOKEN", "token")
}

func writePlaintextProfileConfig(t *testing.T, authType string) string {
	t.Helper()
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.json")
	configPath := filepath.Join(dir, "config.toml")
	text := fmt.Sprintf(`current_profile = "default"

[defaults]
output = "auto"
rate_limit = "1/s"
timeout = "30s"

[profiles.default]
account_url = "https://company.worksection.com"
auth_type = %q
secret_ref = %q
`, authType, "plaintext:"+secretPath)
	if err := os.WriteFile(configPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WSECTL_CONFIG", configPath)
	return secretPath
}

type unreadableSecretStore struct{}

func (unreadableSecretStore) Get(context.Context, auth.SecretRef) (auth.SecretBundle, error) {
	return auth.SecretBundle{}, errors.New("read denied")
}

func (unreadableSecretStore) Set(context.Context, auth.SecretRef, auth.SecretBundle) error {
	return nil
}

func (unreadableSecretStore) Delete(context.Context, auth.SecretRef) error {
	return nil
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	if normalizeText(got) != normalizeText(string(want)) {
		t.Fatalf("%s does not match generated output", name)
	}
}

func normalizeText(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
