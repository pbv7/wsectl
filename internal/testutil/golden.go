package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func ReadGolden(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
