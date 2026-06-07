package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newDocsCommand(_ *state) *cobra.Command {
	docsCmd := &cobra.Command{Use: "docs", Short: "Documentation helpers"}
	var outPath string
	gen := &cobra.Command{
		Use:   "generate",
		Short: "Generate command reference",
		RunE: func(cmd *cobra.Command, _ []string) error {
			content := renderCommandReference(collectCommands(cmd.Root()))
			if outPath == "" {
				_, err := fmt.Fprint(cmd.OutOrStdout(), content)
				return err
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			return os.WriteFile(outPath, []byte(content), 0o644)
		},
	}
	gen.Flags().StringVar(&outPath, "out", "docs/command-reference.md", "Output markdown file")
	docsCmd.AddCommand(gen)
	return docsCmd
}

func renderCommandReference(commands []commandInfo) string {
	var b strings.Builder
	b.WriteString("# Command Reference\n\n")
	b.WriteString("Generated from the command metadata compiled into `wsectl`. Do not edit by hand.\n\n")
	for _, info := range commands {
		fmt.Fprintf(&b, "## `%s`\n\n%s\n\n", info.Path, info.Short)
		if info.Long != "" && info.Long != info.Short {
			fmt.Fprintf(&b, "%s\n\n", info.Long)
		}
		fmt.Fprintf(&b, "**Usage:** `%s`\n\n", info.Usage)
		b.WriteString("**Command metadata:**\n\n")
		fmt.Fprintf(&b, "- Category: `%s`\n", info.Category)
		fmt.Fprintf(&b, "- Authentication required: `%t`\n", info.AuthRequired)
		fmt.Fprintf(&b, "- Read-only: `%t`\n", info.ReadOnly)
		fmt.Fprintf(&b, "- Output support: `%t`\n\n", info.Output)
		if len(info.Actions) > 0 {
			fmt.Fprintf(&b, "**Worksection actions:** `%s`\n\n", strings.Join(info.Actions, "`, `"))
			wroteDetails := false
			for _, action := range info.Actions {
				spec, ok := worksection.LookupAction(action)
				if !ok || spec.Response.Shape == "" {
					continue
				}
				fmt.Fprintf(&b, "- `%s` response shape: `%s`; count path: `%s`.\n", action, spec.Response.Shape, spec.Response.CountPath)
				wroteDetails = true
				for _, note := range spec.CompatibilityNotes {
					writeMarkdownBullet(&b, fmt.Sprintf("`%s` compatibility: %s", action, note))
					wroteDetails = true
				}
			}
			if wroteDetails {
				b.WriteString("\n")
			}
		}
		if len(info.OutputModes) > 0 {
			fmt.Fprintf(&b, "**Output modes:** `%s`\n\n", strings.Join(info.OutputModes, "`, `"))
		}
		if len(info.Flags) > 0 {
			b.WriteString("**Command flags:**\n\n")
			for _, flag := range info.Flags {
				fmt.Fprintf(&b, "- `%s`\n", flag)
			}
			b.WriteString("\n")
		}
		if info.Examples != "" {
			fmt.Fprintf(&b, "**Examples:**\n\n```bash\n%s\n```\n\n", info.Examples)
		}
		if len(info.AgentNotes) > 0 {
			b.WriteString("**Agent notes:**\n\n")
			for _, note := range info.AgentNotes {
				writeMarkdownBullet(&b, note)
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeMarkdownBullet(b *strings.Builder, text string) {
	writeWrappedMarkdownLine(b, "- ", "  ", text, 150)
}

func writeWrappedMarkdownLine(b *strings.Builder, firstPrefix, nextPrefix, text string, limit int) {
	prefix := firstPrefix
	remaining := strings.TrimSpace(text)
	for {
		width := limit - len(prefix)
		if width <= 0 || len(remaining) <= width {
			fmt.Fprintf(b, "%s%s\n", prefix, remaining)
			return
		}
		cut := strings.LastIndexAny(remaining[:width+1], " \t")
		if cut <= 0 {
			cut = width
		}
		fmt.Fprintf(b, "%s%s\n", prefix, strings.TrimRight(remaining[:cut], " \t"))
		remaining = strings.TrimLeft(remaining[cut:], " \t")
		prefix = nextPrefix
	}
}
