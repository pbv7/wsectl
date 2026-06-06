package docs

import "embed"

//go:embed *.md
var FS embed.FS

// Read returns an embedded Markdown document by filename.
func Read(name string) string {
	raw, err := FS.ReadFile(name)
	if err != nil {
		return ""
	}
	return string(raw)
}
