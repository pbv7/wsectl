package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pbv7/wsectl/internal/worksection"
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
	raw, err := Table(env, worksection.ResponseContract{})
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
	table, err := Table(env, compositeCostContract())
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
