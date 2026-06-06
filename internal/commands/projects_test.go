package commands

import "testing"

func TestProjectStatusFilterMapsArchivedToAPIValue(t *testing.T) {
	tests := map[string]string{
		"":         "",
		"active":   "active",
		"pending":  "pending",
		"archived": "archive",
		"archive":  "archive",
	}

	for input, want := range tests {
		if got := projectStatusFilter(input); got != want {
			t.Fatalf("projectStatusFilter(%q) = %q, want %q", input, got, want)
		}
	}
}
