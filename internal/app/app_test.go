package app

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSuppressesRenderedMachineError(t *testing.T) {
	t.Setenv("WSECTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("WSECTL_ACCOUNT_URL", "https://company.worksection.com")
	t.Setenv("WSECTL_ACCESS_TOKEN", "")
	t.Setenv("WSECTL_REFRESH_TOKEN", "")
	t.Setenv("WSECTL_ADMIN_TOKEN", "")
	t.Setenv("WSECTL_CLIENT_ID", "")

	stdout, stderr, err := captureProcessOutput(func() error {
		return Run(context.Background(), []string{"me", "--json"})
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if stdout != "" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	var env struct {
		Status string `json:"status"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(stderr), &env); jsonErr != nil {
		t.Fatalf("stderr must be one JSON envelope: %v\n%s", jsonErr, stderr)
	}
	if env.Status != "error" || env.Error.Code != "authentication" {
		t.Fatalf("unexpected envelope %#v", env)
	}
}

func captureProcessOutput(fn func() error) (string, string, error) {
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
	}()
	err := fn()
	_ = outW.Close()
	_ = errW.Close()
	out, _ := io.ReadAll(outR)
	stderr, _ := io.ReadAll(errR)
	_ = outR.Close()
	_ = errR.Close()
	return string(out), string(stderr), err
}
