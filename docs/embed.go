// Package docs embeds the Markdown documentation shown by runtime help commands.
package docs

import "embed"

//go:embed manual.md agent-usage.md auth.md configuration.md output-contracts.md completion.md doctor.md examples.md env.md limits.md
var FS embed.FS

// Read returns an embedded Markdown document by filename.
func Read(name string) string {
	raw, err := FS.ReadFile(name)
	if err != nil {
		return ""
	}
	return string(raw)
}
