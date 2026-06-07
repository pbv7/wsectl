package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/pbv7/wsectl/internal/worksection"
)

func TestMachineFormatDetection(t *testing.T) {
	t.Setenv("WSECTL_OUTPUT", "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "json shortcut", args: []string{"me", "--json"}, want: "json"},
		{name: "yaml shortcut", args: []string{"--yaml", "me"}, want: "yaml"},
		{name: "ndjson shortcut", args: []string{"tasks", "all", "--ndjson"}, want: "ndjson"},
		{name: "output flag", args: []string{"--output", "json", "me"}, want: "json"},
		{name: "output equals", args: []string{"me", "--output=yaml"}, want: "yaml"},
		{name: "human output ignored", args: []string{"me", "--output", "table"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := machineFormat(tt.args); got != tt.want {
				t.Fatalf("machineFormat(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}

	t.Setenv("WSECTL_OUTPUT", "json")
	if got := machineFormat([]string{"me"}); got != "json" {
		t.Fatalf("env machine format = %q, want json", got)
	}
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
