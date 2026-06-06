package commands

import (
	"fmt"
	"strings"
)

const guideFormatVersion = "1"

type guideSection struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
}

type guideDocument struct {
	Topic              string         `json:"topic"`
	Content            string         `json:"content"`
	GuideFormatVersion string         `json:"guide_format_version"`
	Full               bool           `json:"full"`
	Sections           []guideSection `json:"sections"`
	Commands           []commandInfo  `json:"commands,omitempty"`
}

func rootGuide() guideDocument {
	sections := []guideSection{
		{
			ID:    "agent-start",
			Title: "Start here (for AI agents)",
			Lines: []string{
				"wsectl help agent --full",
				"",
				"Use the version-matched guide before guessing flags or parsing human output.",
				"Prefer --json or --ndjson, use --out for large results, and inspect",
				"meta.truncated plus meta.warnings before trusting completeness.",
				"Use --schema --json or api schema before selecting --fields or --jq.",
			},
		},
		{
			ID:    "human-start",
			Title: "Human quick start",
			Lines: []string{
				"wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2",
				"Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell.",
				"wsectl auth login --client-id \"$WSECTL_CLIENT_ID\"",
				"wsectl doctor --api",
				"wsectl projects list --json",
			},
		},
		{
			ID:    "persistent-config",
			Title: "Persistent desktop config",
			Lines: []string{
				"profiles add writes non-secret account settings to config.toml.",
				"Without --secret-ref, profiles use keyring:wsectl/PROFILE.",
				"auth login stores tokens and client credentials in the selected secret store.",
				"Use --profile NAME for one command or profiles use NAME to change the default.",
			},
		},
		{
			ID:    "core-reads",
			Title: "Core read commands",
			Lines: []string{
				"wsectl me --json",
				"wsectl users list --json",
				"wsectl projects list --status active --json",
				"wsectl tasks search --query \"invoice\" --json",
				"wsectl tasks all --extra text,files --json --out /tmp/tasks.json",
				"wsectl files download FILE_ID --out ./attachment.bin",
			},
		},
		{
			ID:    "output",
			Title: "Machine-readable output",
			Lines: []string{
				"--json       Stable response envelope for scripts and agents",
				"--ndjson     One data record per line for large arrays",
				"--raw        Exact Worksection response body",
				"--fields     Simple field projection",
				"--schema     Static response contract without a Worksection request",
				"--jq         gojq transform",
				"--out FILE   Write large output without filling agent context",
			},
		},
		{
			ID:    "safety",
			Title: "Safety and limits",
			Lines: []string{
				"This build is read-only. Known mutation actions are blocked locally.",
				"Worksection documents a 1 request/second limit and a 10,000-record cap",
				"for some endpoints. Use --fail-on-truncated when completeness is required.",
				"Tokens are never printed by normal commands.",
			},
		},
		{
			ID:    "discovery",
			Title: "Discovery and diagnostics",
			Lines: []string{
				"wsectl commands --json          Machine-readable command catalog",
				"wsectl api actions --json       Known API actions and read/write status",
				"wsectl api schema ACTION        Parameters and static response contract",
				"wsectl api call ACTION ...      Low-level read-only escape hatch",
				"wsectl doctor [--json] [--api]  Diagnose configuration and credentials",
			},
		},
		{
			ID:    "completion",
			Title: "Shell completion",
			Lines: []string{
				"wsectl completion bash|zsh|fish|powershell",
			},
		},
		{
			ID:    "help",
			Title: "More help",
			Lines: []string{
				"wsectl help agent [--full] [--json]",
				"wsectl help auth|config|doctor|output|examples|env|limits|completion",
				"wsectl COMMAND --help",
			},
		},
	}
	doc := guideDocument{
		Topic:              "manual",
		GuideFormatVersion: guideFormatVersion,
		Sections:           sections,
	}
	doc.Content = renderGuide("wsectl - Unofficial command-line client for Worksection.", sections)
	return doc
}

func agentGuide(full bool, commands []commandInfo) guideDocument {
	sections := []guideSection{
		{
			ID:    "rules",
			Title: "Operating rules",
			Lines: []string{
				"Prefer --json or --ndjson. Never parse table output.",
				"Use --out FILE for large responses.",
				"Check meta.truncated and meta.warnings.",
				"Use --schema --json before selecting fields for unfamiliar commands.",
				"Use --profile NAME when account context matters.",
				"Avoid request loops; Worksection documents a 1 request/second limit.",
				"Never print tokens unless the user explicitly requests token material.",
			},
		},
		{
			ID:    "workflows",
			Title: "Common workflows",
			Lines: []string{
				"wsectl projects list --json",
				"wsectl tasks search --query \"invoice\" --json",
				"wsectl tasks all --extra text,files --json --out /tmp/tasks.json",
				"wsectl api call get_users_schedule --param datestart=01.05.2026 --param dateend=31.05.2026 --json",
			},
		},
		{
			ID:    "discovery",
			Title: "Discover capabilities",
			Lines: []string{
				"wsectl commands --json",
				"wsectl api actions --json",
				"wsectl api schema ACTION --json",
				"wsectl tasks search --schema --json",
				"wsectl doctor --json",
			},
		},
	}
	if full {
		sections = append(sections,
			guideSection{
				ID:    "setup",
				Title: "Setup and authentication",
				Lines: []string{
					"wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2",
					"Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell.",
					"wsectl auth login --client-id \"$WSECTL_CLIENT_ID\"",
					"wsectl auth status --json",
					"wsectl doctor --api --json",
					"",
					"For desktop use: config.toml stores profiles; keyring stores tokens and client secrets.",
					"Use wsectl profiles use NAME to change the default profile.",
					"Use --profile NAME or WSECTL_PROFILE for temporary profile selection.",
					"",
					"Admin token: profiles add admin --auth-type admin_token --secret-ref keyring:wsectl/admin",
					"Encrypted file: profiles add portable --secret-ref encrypted-file:/path/to/secret.json",
					"",
					"For CI: set WSECTL_ACCOUNT_URL and WSECTL_ACCESS_TOKEN.",
				},
			},
			guideSection{
				ID:    "output-contract",
				Title: "Output and errors",
				Lines: []string{
					"Success: {\"status\":\"ok\",\"data\":...,\"meta\":{...}}",
					"Error:   {\"status\":\"error\",\"error\":{\"code\":...,\"message\":...},\"meta\":{...}}",
					"meta.contract_version and meta.response_shape describe the static output contract.",
					"Exit codes: 2 usage, 3 auth, 4 permission, 5 network, 6 API, 7 rate limit, 8 truncation.",
					"Use --fail-on-truncated when incomplete data must fail the run.",
				},
			},
			guideSection{
				ID:    "compatibility",
				Title: "API compatibility notes",
				Lines: []string{
					"First-class commands normalize known Worksection quirks; api call uses raw API params.",
					"projects list --status archived sends filter=archive.",
					"tasks search --query TEXT sends a Worksection filter, not a search parameter.",
					"costs --timer sends is_timer; files images filters client-side.",
					"API calls use query-parameter POST mode; form-encoded API bodies are not supported.",
				},
			},
			guideSection{
				ID:    "safety",
				Title: "Safety model",
				Lines: []string{
					"The MVP is read-only. Known Worksection mutation actions are rejected before a request.",
					"Secrets live in keyring, environment, encrypted-file, or explicit plaintext stores.",
					"Do not expose WSECTL_ACCESS_TOKEN, WSECTL_REFRESH_TOKEN, or WSECTL_CLIENT_SECRET.",
				},
			},
			guideSection{
				ID:    "limits",
				Title: "Worksection limits",
				Lines: []string{
					"Default client rate: 1 request/second.",
					"Some endpoints can return at most 10,000 records.",
					"API params stay in the query string; very long filters can still hit URL limits.",
				},
			},
			commandCatalogSection(commands),
		)
	}
	doc := guideDocument{
		Topic:              "agent",
		GuideFormatVersion: guideFormatVersion,
		Full:               full,
		Sections:           sections,
	}
	if full {
		doc.Commands = commands
	}
	title := "wsectl agent guide"
	if full {
		title += " (full)"
	}
	doc.Content = renderGuide(title, sections)
	return doc
}

func commandCatalogSection(commands []commandInfo) guideSection {
	lines := make([]string, 0, len(commands))
	width := 0
	for _, info := range commands {
		if info.Path == "wsectl" {
			continue
		}
		if len(info.Path) > width {
			width = len(info.Path)
		}
	}
	for _, info := range commands {
		if info.Path == "wsectl" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-*s  %s", width, info.Path, info.Short))
	}
	return guideSection{ID: "command-catalog", Title: "Command catalog", Lines: lines}
}

func renderGuide(title string, sections []guideSection) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString("This is an unofficial tool and is not affiliated with Worksection.\n")
	for _, section := range sections {
		b.WriteString("\n")
		b.WriteString(section.Title)
		b.WriteString(":\n")
		for _, line := range section.Lines {
			if line == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}
