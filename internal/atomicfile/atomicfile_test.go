package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteFileCreatesParentAndSetsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "secret.json")
	if err := WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "second" {
		t.Fatalf("file contents = %q", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestMkdirAllTrackedReportsParentOfTopmostCreatedDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")
	syncRoot, err := mkdirAllTracked(nested)
	if err != nil {
		t.Fatal(err)
	}
	if syncRoot != base {
		t.Fatalf("syncRoot = %q, want parent of topmost created dir %q", syncRoot, base)
	}
	if info, err := os.Stat(nested); err != nil || !info.IsDir() {
		t.Fatalf("nested dir not created: info=%v err=%v", info, err)
	}
}

func TestMkdirAllTrackedReturnsEmptyForExistingDir(t *testing.T) {
	base := t.TempDir()
	syncRoot, err := mkdirAllTracked(base)
	if err != nil {
		t.Fatal(err)
	}
	if syncRoot != "" {
		t.Fatalf("syncRoot = %q, want \"\" when the directory already exists", syncRoot)
	}
}
