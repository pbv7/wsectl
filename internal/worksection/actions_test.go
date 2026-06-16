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

func TestAdminHashIsStable(t *testing.T) {
	got := AdminHash("get_tasks", map[string]string{"id_project": "26"}, "7776461cd931e7b1c8e9632ff8e979ce")
	if got == "" || len(got) != 32 {
		t.Fatalf("unexpected hash %q", got)
	}
}
