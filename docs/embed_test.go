package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDocsAreEmbedded(t *testing.T) {
	for _, name := range []string{
		"manual.md",
		"agent-usage.md",
		"auth.md",
		"configuration.md",
		"output-contracts.md",
		"completion.md",
		"doctor.md",
		"examples.md",
		"env.md",
		"limits.md",
	} {
		if strings.TrimSpace(Read(name)) == "" {
			t.Fatalf("%s is not embedded", name)
		}
	}
}

func TestNonRuntimeDocsAreNotEmbedded(t *testing.T) {
	for _, name := range []string{
		"command-reference.md",
		"release.md",
		"security.md",
		"recipes.md",
		"api-coverage.md",
	} {
		if Read(name) != "" {
			t.Fatalf("%s should not be embedded in runtime help", name)
		}
	}
}

func TestDocsDoNotAdvertiseStaleColorOrLiteralSecretExamples(t *testing.T) {
	projectRoot := filepath.Join("..")
	for _, rel := range []string{"README.md", "docs", "testdata/golden"} {
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
