package worksection

import "testing"

func TestKnownWriteActionsAreBlocked(t *testing.T) {
	for _, name := range []string{"post_task", "add_project_group", "add_project_groups"} {
		a, ok := LookupAction(name)
		if !ok {
			t.Fatalf("%s missing from registry", name)
		}
		if a.ReadOnly {
			t.Fatalf("%s must not be read-only", name)
		}
	}
}

func TestKnownReadActionsAreRegistered(t *testing.T) {
	for _, name := range []string{"me", "get_users", "get_projects", "get_all_tasks", "get_comments", "get_files", "get_webhooks"} {
		a, ok := LookupAction(name)
		if !ok {
			t.Fatalf("%s missing from registry", name)
		}
		if !a.ReadOnly {
			t.Fatalf("%s should be read-only", name)
		}
	}
}

func TestActionSpecsHaveResponseContracts(t *testing.T) {
	for _, action := range Actions() {
		if action.Response.ContractVersion == "" {
			t.Fatalf("%s missing response contract version", action.Name)
		}
		if action.ReadOnly && action.Response.Shape == "" {
			t.Fatalf("%s missing response shape", action.Name)
		}
		if action.Response.Shape == "composite" {
			t.Fatalf("%s uses the removed composite response shape", action.Name)
		}
	}
}

func TestCostsContractsAreFlattened(t *testing.T) {
	list, ok := LookupAction("get_costs")
	if !ok {
		t.Fatal("get_costs missing from registry")
	}
	if list.Response.Shape != "array" {
		t.Fatalf("get_costs shape = %q, want array", list.Response.Shape)
	}
	if list.Response.AggregatePath != "total" {
		t.Fatalf("get_costs aggregate_path = %q, want total", list.Response.AggregatePath)
	}
	total, ok := LookupAction("get_costs_total")
	if !ok {
		t.Fatal("get_costs_total missing from registry")
	}
	if total.Response.Shape != "object" {
		t.Fatalf("get_costs_total shape = %q, want object", total.Response.Shape)
	}
	if total.Response.AggregatePath != "" {
		t.Fatalf("get_costs_total should not declare an aggregate_path, got %q", total.Response.AggregatePath)
	}
}

func TestResponseContractsMatchCurrentDocs(t *testing.T) {
	tests := []struct {
		action string
		shape  string
		fields []string
	}{
		{"get_webhooks", "array", []string{"events", "status", "projects"}},
		{"get_costs", "array", []string{"comment", "time", "money", "is_timer", "user_from", "task"}},
		{"get_costs_total", "object", []string{"total"}},
		{"get_timers", "array", []string{"date_started", "user_from", "task"}},
		{"get_my_timer", "object", []string{"date_started", "task"}},
		{"get_users_schedule", "object", []string{"schedule", "group", "department"}},
		{"get_project_groups", "array", []string{"title", "type", "client"}},
		{"get_task_tags", "array", []string{"title", "group"}},
		{"get_task_tag_groups", "array", []string{"title", "type", "access"}},
	}
	for _, tt := range tests {
		spec, ok := LookupAction(tt.action)
		if !ok {
			t.Fatalf("%s missing from registry", tt.action)
		}
		if spec.Response.Shape != tt.shape {
			t.Fatalf("%s shape = %s, want %s", tt.action, spec.Response.Shape, tt.shape)
		}
		known := map[string]bool{}
		for _, field := range spec.KnownFieldNames() {
			known[field] = true
		}
		for _, field := range tt.fields {
			if !known[field] {
				t.Fatalf("%s missing response field %q in %#v", tt.action, field, spec.KnownFieldNames())
			}
		}
	}
}

func TestValidateActionKnownMappings(t *testing.T) {
	if err := ValidateAction("get_projects", map[string]string{"filter": "archive"}, false); err != nil {
		t.Fatalf("archive filter should be valid: %v", err)
	}
	if err := ValidateAction("get_projects", map[string]string{"filter": "archived"}, false); err == nil {
		t.Fatal("archived API filter should be rejected; CLI maps it to archive")
	}
	if err := ValidateAction("get_events", map[string]string{"id_project": "1"}, false); err == nil {
		t.Fatal("get_events without period should fail")
	}
	if err := ValidateAction("get_files", map[string]string{"id_project": "1", "id_task": "2"}, false); err == nil {
		t.Fatal("get_files with both selectors should fail")
	}
	if err := ValidateAction("get_tasks", map[string]string{"id_project": "1", "filter": "done"}, false); err == nil {
		t.Fatal("done task list filter should fail")
	}
	if err := ValidateAction("search_tasks", map[string]string{"id_task": "1"}, false); err != nil {
		t.Fatalf("search_tasks id_task should be valid: %v", err)
	}
	if err := ValidateAction("post_task", nil, true); err == nil {
		t.Fatal("mutation must remain blocked with allowUnknown")
	}
	if err := ValidateAction("add_project_groups", nil, true); err == nil {
		t.Fatal("Postman plural project-folder mutation must remain blocked with allowUnknown")
	}
}

func TestSearchTasksRequiresASearchDimension(t *testing.T) {
	spec, _ := LookupAction("search_tasks")
	if len(spec.AnyOf) == 0 {
		t.Fatal("search_tasks should declare an any_of search-dimension requirement")
	}
	if err := ValidateAction("search_tasks", map[string]string{"status": "all"}, false); err == nil {
		t.Fatal("search_tasks with only status (a modifier) should fail validation")
	}
	if err := ValidateAction("search_tasks", map[string]string{"extra": "text"}, false); err == nil {
		t.Fatal("search_tasks with only extra (a modifier) should fail validation")
	}
	if err := ValidateAction("search_tasks", map[string]string{"id_project": "1"}, false); err != nil {
		t.Fatalf("search_tasks with id_project should pass: %v", err)
	}
}

func TestGetEventsPeriodPatternValidation(t *testing.T) {
	// Valid units are minutes, hours, days; weeks/words/bare numbers are not.
	for _, bad := range []string{"week", "month", "all", "7", "2w", "1y", "0d", "01h"} {
		if err := ValidateAction("get_events", map[string]string{"period": bad}, false); err == nil {
			t.Fatalf("period %q should fail client-side validation", bad)
		}
	}
	for _, good := range []string{"30m", "1h", "24h", "7d"} {
		if err := ValidateAction("get_events", map[string]string{"period": good}, false); err != nil {
			t.Fatalf("period %q should pass: %v", good, err)
		}
	}
}

func TestTaskFieldsIncludePageAndDocumentConditionalFields(t *testing.T) {
	for _, action := range []string{"get_tasks", "search_tasks"} {
		spec, _ := LookupAction(action)
		byName := map[string]FieldSpec{}
		for _, f := range spec.Response.ItemShape {
			byName[f.Name] = f
		}
		for _, f := range []string{"page", "date_closed"} {
			if _, ok := byName[f]; !ok {
				t.Fatalf("%s item shape missing %q", action, f)
			}
		}
		// The conditional fields must carry a schema-visible description so
		// downstream agents do not treat them as guaranteed.
		for _, f := range []string{"date_start", "date_end", "date_closed"} {
			if byName[f].Description == "" {
				t.Fatalf("%s field %q must document its conditional presence in the schema", action, f)
			}
		}
	}
}

func TestSubtaskContract(t *testing.T) {
	// extra=subtasks exposes "child" stubs (not "subtasks") on get_task/list.
	for _, action := range []string{"get_task", "get_tasks", "get_all_tasks"} {
		spec, _ := LookupAction(action)
		cf := spec.Response.ConditionalFields["extra=subtasks"]
		names := map[string]bool{}
		for _, f := range cf {
			names[f.Name] = true
		}
		if !names["child"] {
			t.Fatalf("%s extra=subtasks should expose 'child', got %#v", action, cf)
		}
		if names["subtasks"] {
			t.Fatalf("%s must not advertise a 'subtasks' field", action)
		}
	}
	// parent is in the shared task item shape, with a description.
	for _, action := range []string{"search_tasks", "get_task", "get_tasks"} {
		spec, _ := LookupAction(action)
		var parent *FieldSpec
		for i, f := range spec.Response.ItemShape {
			if f.Name == "parent" {
				parent = &spec.Response.ItemShape[i]
			}
		}
		if parent == nil {
			t.Fatalf("%s item_shape missing parent", action)
		}
		if parent.Description == "" {
			t.Fatalf("%s parent must carry a conditional description", action)
		}
	}
	// search_tasks returns subtasks as flat rows, so it must NOT advertise the
	// child array (extra=subtasks is a no-op there).
	ss, _ := LookupAction("search_tasks")
	if _, ok := ss.Response.ConditionalFields["extra=subtasks"]; ok {
		t.Fatal("search_tasks should not advertise an extra=subtasks child array")
	}
}

func TestAdminHashIsStable(t *testing.T) {
	got := AdminHash("get_tasks", map[string]string{"id_project": "26"}, "7776461cd931e7b1c8e9632ff8e979ce")
	if got == "" || len(got) != 32 {
		t.Fatalf("unexpected hash %q", got)
	}
}
