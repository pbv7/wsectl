package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbv7/wsectl/internal/worksection"
	"gopkg.in/yaml.v3"
)

func TestJSONEnvelope(t *testing.T) {
	env := Success("get_users", "default", "https://company.worksection.com", json.RawMessage(`[{"id":"1","name":"Ada"}]`))
	raw, err := JSON(env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"status": "ok"`) || !strings.Contains(string(raw), `"count": 1`) {
		t.Fatalf("unexpected json %s", raw)
	}
}

func TestFailurePreservesWorksectionErrorContract(t *testing.T) {
	env := Failure("download", "default", &worksection.Error{
		Code:    worksection.CodeUsage,
		Message: "download URL is blocked",
		Details: map[string]any{
			"reason":        "download_host_mismatch",
			"expected_host": "example.test",
			"actual_host":   "files.example.test",
		},
	})
	raw, err := JSON(env)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		`"status": "error"`,
		`"code": "usage"`,
		`"message": "download URL is blocked"`,
		`"reason": "download_host_mismatch"`,
		`"expected_host": "example.test"`,
		`"actual_host": "files.example.test"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("failure envelope missing %q:\n%s", want, text)
		}
	}
}

func TestNDJSON(t *testing.T) {
	env := Success("get_users", "default", "", json.RawMessage(`[{"id":"1"},{"id":"2"}]`))
	raw, err := NDJSON(env, worksection.ResponseContract{})
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(raw, []byte("\n")); got != 1 {
		t.Fatalf("newline count = %d", got)
	}
}

func TestSelectFields(t *testing.T) {
	env := Success("get_users", "default", "", json.RawMessage(`[{"id":"1","name":"Ada"}]`))
	raw, _ := JSON(env)
	selected, err := SelectFields(raw, []string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(selected), "Ada") || !strings.Contains(string(selected), `"id"`) {
		t.Fatalf("unexpected fields %s", selected)
	}
}

func TestApplyFieldSelectionSupportsDottedPathsAndWarnings(t *testing.T) {
	env := Success("get_users", "default", "", json.RawMessage(`[{"id":"1","user":{"email":"a@example.com"},"name":"Ada"}]`))
	selected, err := ApplyFieldSelection(env, []string{"id", "user.email", "missing"}, []string{"id", "user"}, worksection.ResponseContract{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(selected.Data), "a@example.com") || strings.Contains(string(selected.Data), "Ada") {
		t.Fatalf("unexpected selected data %s", selected.Data)
	}
	if len(selected.Meta.Warnings) == 0 {
		t.Fatal("expected warnings for missing/unknown fields")
	}
}

func TestLimitData(t *testing.T) {
	got, limited, err := LimitData(json.RawMessage(`[{"id":1},{"id":2},{"id":3}]`), 2, worksection.ResponseContract{})
	if err != nil {
		t.Fatal(err)
	}
	if !limited || !strings.Contains(string(got), `"id":1`) || strings.Contains(string(got), `"id":3`) {
		t.Fatalf("unexpected limited data %s limited=%t", got, limited)
	}
}

func TestWriteRawToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raw.json")
	env := Success("get_users", "default", "", json.RawMessage(`{"exact":true}`))
	if err := Write(&bytes.Buffer{}, env, Options{Format: "raw", Out: path}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"exact":true}` {
		t.Fatalf("raw output was not exact: %q", raw)
	}
}

func TestWriteFailOnTruncatedBeforeRendering(t *testing.T) {
	env := Success("get_users", "default", "", json.RawMessage(`[{"id":"1"}]`))
	env.Meta.Truncated = true
	err := Write(&bytes.Buffer{}, env, Options{Format: "definitely-not-a-format", FailOnTruncated: true})
	wsErr, ok := err.(*worksection.Error)
	if !ok || wsErr.Code != worksection.CodeTruncated {
		t.Fatalf("error = %#v, want truncated Worksection error", err)
	}
}

func TestWriteYAMLTableAndJQ(t *testing.T) {
	env := Success("get_users", "default", "", json.RawMessage(`[{"id":"1","name":"Ada"}]`))
	var yamlOut bytes.Buffer
	if err := Write(&yamlOut, env, Options{Format: "yaml"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yamlOut.String(), "status: ok") {
		t.Fatalf("unexpected yaml: %s", yamlOut.String())
	}
	if !strings.Contains(yamlOut.String(), "id: \"1\"") || !strings.Contains(yamlOut.String(), "name: Ada") {
		t.Fatalf("yaml data not rendered as a mapping: %s", yamlOut.String())
	}
	if strings.Contains(yamlOut.String(), "- 91") || strings.Contains(yamlOut.String(), "- 123") {
		t.Fatalf("yaml data leaked as raw bytes: %s", yamlOut.String())
	}
	var tableOut bytes.Buffer
	if err := Write(&tableOut, env, Options{Format: "table"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tableOut.String(), "ID") || !strings.Contains(tableOut.String(), "Ada") {
		t.Fatalf("unexpected table: %s", tableOut.String())
	}
	var jqOut bytes.Buffer
	if err := Write(&jqOut, env, Options{Format: "json", JQ: ".data[0].name"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jqOut.String(), "Ada") {
		t.Fatalf("unexpected jq output: %s", jqOut.String())
	}
}

func TestTableWarnsWhenColumnsAreOmitted(t *testing.T) {
	env := Success("wide", "default", "", json.RawMessage(`[{
		"id":"1",
		"name":"Ada",
		"status":"active",
		"email":"ada@example.com",
		"date_added":"2026-06-01",
		"date_start":"2026-06-02",
		"date_end":"2026-06-03",
		"extra":"hidden"
	}]`))
	raw, err := Table(env, worksection.ResponseContract{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "Note: table output shows 6 of 8 columns") {
		t.Fatalf("table did not warn about omitted columns:\n%s", text)
	}
	if strings.Contains(text, "DATE_END") || strings.Contains(text, "EXTRA") {
		t.Fatalf("table rendered columns that should be omitted by the six-column cap:\n%s", text)
	}
}

func TestApplyJQEmitsNewlineSeparatedValues(t *testing.T) {
	raw := []byte(`{"data":[{"action":"post"},{"action":"close"},{"action":"update"}]}`)
	out, err := ApplyJQ(raw, ".data[].action")
	if err != nil {
		t.Fatal(err)
	}
	want := "\"post\"\n\"close\"\n\"update\""
	if string(out) != want {
		t.Fatalf("jq output = %q, want %q", out, want)
	}
}

func TestApplyJQSingleResultHasNoTrailer(t *testing.T) {
	raw := []byte(`{"data":[{"id":1}]}`)
	out, err := ApplyJQ(raw, ".data[0].id")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "1" {
		t.Fatalf("jq output = %q, want \"1\"", out)
	}
}

func TestYAMLAcceptsJSONForwardSlashEscape(t *testing.T) {
	// yaml.v3's tokenizer rejects JSON's optional \/ escape with
	// "found unknown escape character". Worksection emits \/ in
	// path-like fields ("page": "/project/.../task/.../") so the
	// YAML renderer must decode JSON itself rather than route the
	// raw bytes through yaml.Unmarshal. Regression for the bug that
	// shipped in PR #7 and broke every YAML render of non-trivial
	// API data.
	env := Success("tasks", "default", "", json.RawMessage(`{"page":"\/project\/1\/2\/","name":"Task"}`))
	raw, err := YAML(env)
	if err != nil {
		t.Fatalf("YAML render failed on JSON \\/ escape: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `page: /project/1/2/`) {
		t.Fatalf("forward slash not preserved in YAML output:\n%s", text)
	}
}

func TestYAMLHandlesJSONEscapesEndToEnd(t *testing.T) {
	// All standard JSON string escapes must survive decode + encode.
	// `\/`, `\n`, `\t`, `\"`, `\\`, `\uXXXX`. yaml.v3 emits control
	// characters in double-quoted form, so just assert the rendered
	// YAML reparses cleanly and round-trips back to the same Go
	// values rather than asserting a fixed byte pattern.
	src := `{
		"slash":  "a\/b\/c",
		"quote":  "she said \"hi\"",
		"newline":"line1\nline2",
		"tab":    "col1\tcol2",
		"back":   "C:\\path",
		"unicode":"smile ☺ end"
	}`
	env := Success("escapes", "default", "", json.RawMessage(src))
	raw, err := YAML(env)
	if err != nil {
		t.Fatalf("YAML render failed: %v", err)
	}
	var round struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(raw, &round); err != nil {
		t.Fatalf("rendered YAML did not reparse: %v\n%s", err, raw)
	}
	want := map[string]string{
		"slash":   "a/b/c",
		"quote":   `she said "hi"`,
		"newline": "line1\nline2",
		"tab":     "col1\tcol2",
		"back":    `C:\path`,
		"unicode": "smile ☺ end",
	}
	for k, v := range want {
		if round.Data[k] != v {
			t.Fatalf("escape %q: yaml round-trip = %q, want %q", k, round.Data[k], v)
		}
	}
}

func TestYAMLPreservesScalarQuotingAndKeyStyle(t *testing.T) {
	// String values that look like booleans, nulls, or numbers must keep
	// their quoting on YAML output — otherwise a round-trip would reparse
	// them as different types. At the same time, map keys must render
	// unquoted in block style so the envelope reads naturally.
	env := Success("api", "default", "", json.RawMessage(`{"flag":"true","missing":"null","numeric_id":"42"}`))
	raw, err := YAML(env)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`flag: "true"`, `missing: "null"`, `numeric_id: "42"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("scalar value quoting dropped (%s):\n%s", want, text)
		}
	}
	for _, bad := range []string{`"flag":`, `"missing":`, `"numeric_id":`} {
		if strings.Contains(text, bad) {
			t.Fatalf("map key rendered with leftover JSON quotes (%s):\n%s", bad, text)
		}
	}
}

func TestYAMLPreservesLargeIntegerPrecision(t *testing.T) {
	// 9007199254740993 = 2^53 + 1, the canonical value that loses
	// precision when routed through float64. The YAML output must
	// preserve the exact digit string.
	env := Success("call", "default", "", json.RawMessage(`{"big_id":9007199254740993,"small":42}`))
	raw, err := YAML(env)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "big_id: 9007199254740993") {
		t.Fatalf("YAML lost precision on 2^53+1 integer:\n%s", text)
	}
	if !strings.Contains(text, "small: 42") {
		t.Fatalf("YAML lost small-integer rendering:\n%s", text)
	}
}

func TestWriteNormalizesTrailingNewline(t *testing.T) {
	env := Success("get_users", "default", "", json.RawMessage(`[{"id":"1","name":"Ada"}]`))
	// YAML renderer emits its own trailing \n; non-YAML renderers do not.
	// writeRenderedOutput must collapse both cases to exactly one.
	for _, format := range []string{"json", "yaml", "ndjson", "table"} {
		var buf bytes.Buffer
		if err := Write(&buf, env, Options{Format: format}); err != nil {
			t.Fatalf("%s write: %v", format, err)
		}
		b := buf.Bytes()
		if len(b) == 0 {
			t.Fatalf("%s produced no output", format)
		}
		if b[len(b)-1] != '\n' {
			t.Fatalf("%s output missing trailing newline: %q", format, b)
		}
		if len(b) >= 2 && b[len(b)-2] == '\n' {
			t.Fatalf("%s output has double trailing newline: %q", format, b)
		}
	}

	// Raw mode must preserve bytes verbatim — no newline added.
	rawEnv := Success("get_users", "default", "", json.RawMessage(`{"exact":true}`))
	var rawBuf bytes.Buffer
	if err := Write(&rawBuf, rawEnv, Options{Format: "raw"}); err != nil {
		t.Fatal(err)
	}
	if rawBuf.String() != `{"exact":true}` {
		t.Fatalf("raw output should be byte-exact, got %q", rawBuf.String())
	}
}

func TestResolveFormatForcesJSONWhenJQSet(t *testing.T) {
	if got := resolveFormat(os.Stdout, "", ".data[0]"); got != "json" {
		t.Fatalf("auto format + --jq should resolve to json, got %q", got)
	}
	if got := resolveFormat(os.Stdout, "auto", ".data[0]"); got != "json" {
		t.Fatalf("explicit auto + --jq should resolve to json, got %q", got)
	}
	if got := resolveFormat(os.Stdout, "yaml", ".data[0]"); got != "yaml" {
		t.Fatalf("explicit --yaml must not be overridden by --jq, got %q", got)
	}
}

func TestWriteEmptyJQResultTruncatesOutFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(path, []byte("stale prior content"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := Success("users", "default", "", json.RawMessage(`[{"id":"1"},{"id":"2"}]`))
	opts := Options{Format: "json", JQ: ".data[] | select(.id==\"missing\")", Out: path}
	if err := Write(&bytes.Buffer{}, env, opts); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty jq stream should truncate --out file, got %q", got)
	}
}

func TestWriteEmptyJQResultProducesNoOutput(t *testing.T) {
	env := Success("users", "default", "", json.RawMessage(`[{"id":"1","name":"Ada"},{"id":"2","name":"Lin"}]`))
	var buf bytes.Buffer
	if err := Write(&buf, env, Options{Format: "json", JQ: ".data[] | select(.id==\"missing\")"}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty jq stream should produce no stdout, got %q", buf.String())
	}
}

func TestWriteTableSurfacesFieldWarnings(t *testing.T) {
	env := Success("users", "default", "", json.RawMessage(`[{"id":"1","name":"Ada"}]`))
	var buf bytes.Buffer
	opts := Options{
		Format:      "table",
		Fields:      []string{"id", "nmae"},
		KnownFields: []string{"id", "name"},
	}
	if err := Write(&buf, env, opts); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if !strings.Contains(text, "Requested field nmae") {
		t.Fatalf("table should surface field warnings, got:\n%s", text)
	}
	if !strings.Contains(text, "ID") || !strings.Contains(text, "NMAE") {
		t.Fatalf("table missing requested columns:\n%s", text)
	}
}

func TestTableHonorsRequestedColumns(t *testing.T) {
	env := Success("events", "default", "", json.RawMessage(`[
		{"action":"post","date_added":"2026-01-01 09:00","object":{"id":"42"}},
		{"action":"close","date_added":"2026-01-01 09:05","object":{"id":"42"}}
	]`))
	raw, err := Table(env, worksection.ResponseContract{}, []string{"action", "date_added"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "ACTION") || !strings.Contains(text, "DATE_ADDED") {
		t.Fatalf("expected requested columns in header:\n%s", text)
	}
	if strings.Contains(text, "OBJECT") {
		t.Fatalf("non-requested column rendered:\n%s", text)
	}
	if strings.Contains(text, "Note: table output shows") {
		t.Fatalf("did not expect omitted-columns note when columns were explicit:\n%s", text)
	}
	if idx, jdx := strings.Index(text, "ACTION"), strings.Index(text, "DATE_ADDED"); idx < 0 || idx >= jdx {
		t.Fatalf("column order not preserved:\n%s", text)
	}
}

func TestTableHonorsDottedRequestedColumns(t *testing.T) {
	env := Success("events", "default", "", json.RawMessage(`[
		{"action":"post","object":{"id":"42","type":"task"}}
	]`))
	raw, err := Table(env, worksection.ResponseContract{}, []string{"action", "object.id"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "OBJECT.ID") || !strings.Contains(text, "42") {
		t.Fatalf("dotted-path column not rendered:\n%s", text)
	}
}

func TestTableDisplayWidthCountsRunes(t *testing.T) {
	if got := displayWidth("ІТ"); got != 2 {
		t.Fatalf("display width = %d, want 2", got)
	}
}

func TestCompositeContractCountsPrimaryDataPath(t *testing.T) {
	env := SuccessWithContract("get_costs", "default", "", compositeCostData(), compositeCostContract())
	if env.Meta.Count != 2 {
		t.Fatalf("count = %d, want 2", env.Meta.Count)
	}
	raw, err := JSON(env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"total"`) || !strings.Contains(string(raw), `"money": "10"`) {
		t.Fatalf("composite JSON lost aggregate fields: %s", raw)
	}
}

func TestCompositeLimitSlicesPrimaryDataAndPreservesAggregates(t *testing.T) {
	got, limited, err := LimitData(compositeCostData(), 1, compositeCostContract())
	if err != nil {
		t.Fatal(err)
	}
	if !limited {
		t.Fatal("expected limit to apply")
	}
	text := string(got)
	if strings.Contains(text, `"id":"2"`) || !strings.Contains(text, `"id":"1"`) || !strings.Contains(text, `"total"`) {
		t.Fatalf("unexpected limited composite data: %s", text)
	}
}

func TestCompositeNDJSONAndTableUsePrimaryRows(t *testing.T) {
	env := SuccessWithContract("get_costs", "default", "", compositeCostData(), compositeCostContract())
	ndjson, err := NDJSON(env, compositeCostContract())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(ndjson, []byte("\n")) != 1 || strings.Contains(string(ndjson), `"total"`) {
		t.Fatalf("unexpected composite ndjson: %s", ndjson)
	}
	table, err := Table(env, compositeCostContract(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(table), "COMMENT") || !strings.Contains(string(table), "Design") || strings.Contains(string(table), "total") {
		t.Fatalf("unexpected composite table: %s", table)
	}
}

func TestCompositeFieldsProjectPrimaryRowsAndPreserveAggregates(t *testing.T) {
	env := SuccessWithContract("get_costs", "default", "", compositeCostData(), compositeCostContract())
	selected, err := ApplyFieldSelection(env, []string{"id", "task.name"}, []string{"id", "task"}, compositeCostContract())
	if err != nil {
		t.Fatal(err)
	}
	text := string(selected.Data)
	if !strings.Contains(text, `"total"`) || !strings.Contains(text, `"task":{"name":"Task A"}`) {
		t.Fatalf("composite fields lost expected data: %s", text)
	}
	if strings.Contains(text, "Design") || strings.Contains(text, `"money":"10"`) || strings.Contains(text, `"money":"20"`) {
		t.Fatalf("composite fields did not project rows: %s", text)
	}
}

func compositeCostContract() worksection.ResponseContract {
	return worksection.ResponseContract{
		ContractVersion: worksection.ContractVersion,
		Shape:           "composite",
		DataPath:        "data",
		CountPath:       "data",
	}
}

func compositeCostData() json.RawMessage {
	return json.RawMessage(`{
		"data": [
			{"id":"1","comment":"Design","money":"10","task":{"name":"Task A"}},
			{"id":"2","comment":"Build","money":"20","task":{"name":"Task B"}}
		],
		"total": {"money":"30"}
	}`)
}
