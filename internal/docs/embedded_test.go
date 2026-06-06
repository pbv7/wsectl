package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedDocsMirrorCanonicalRepoDocs(t *testing.T) {
	for _, name := range []string{
		"agent-usage.md",
		"auth.md",
		"completion.md",
		"configuration.md",
		"doctor.md",
		"manual.md",
		"output-contracts.md",
	} {
		embedded := strings.TrimSpace(Read(name))
		if embedded == "" {
			t.Fatalf("%s is not embedded", name)
		}
		repoRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", name))
		if err != nil {
			t.Fatal(err)
		}
		if embedded != strings.TrimSpace(string(repoRaw)) {
			t.Fatalf("%s embedded doc differs from docs/%s", name, name)
		}
	}
}

func TestDocsDoNotAdvertiseStaleColorOrLiteralSecretExamples(t *testing.T) {
	projectRoot := filepath.Join("..", "..")
	for _, rel := range []string{"README.md", "docs", "internal/docs", "testdata/golden"} {
		path := filepath.Join(projectRoot, rel)
		if err := filepath.WalkDir(path, func(name string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
				return nil
			}
			raw, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			text := string(raw)
			for _, forbidden := range []string{
				"WSECTL_NO_COLOR",
				`color = "auto"`,
				"YOUR_CLIENT_SECRET",
				"ADMIN_API_TOKEN",
				"NEW_CLIENT_SECRET",
				"AGENCY_CLIENT_SECRET",
				"CLIENT_CLIENT_SECRET",
				`--client-secret "$WSECTL_CLIENT_SECRET"`,
				`--client-secret $env:WSECTL_CLIENT_SECRET`,
				`--client-secret %WSECTL_CLIENT_SECRET%`,
				`--admin-token "$WSECTL_ADMIN_TOKEN"`,
				`--admin-token $env:WSECTL_ADMIN_TOKEN`,
				`--admin-token %WSECTL_ADMIN_TOKEN%`,
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s contains forbidden docs text %q", name, forbidden)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
