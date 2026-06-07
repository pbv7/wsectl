package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	annotationCategory    = "wsectl.category"
	annotationActions     = "wsectl.actions"
	annotationOutputModes = "wsectl.output_modes"
	annotationAgentNotes  = "wsectl.agent_notes"
	annotationReadOnly    = "wsectl.read_only"
	annotationAuth        = "wsectl.auth_required"
	annotationHistorySkip = "wsectl.history.skip"
)

type commandMetadata struct {
	Category     string
	Actions      []string
	OutputModes  []string
	AgentNotes   []string
	ReadOnly     bool
	AuthRequired bool
}

type commandInfo struct {
	Path         string   `json:"path"`
	Short        string   `json:"short"`
	Long         string   `json:"long,omitempty"`
	Usage        string   `json:"usage"`
	Flags        []string `json:"flags,omitempty"`
	Examples     string   `json:"examples,omitempty"`
	ReadOnly     bool     `json:"read_only"`
	Output       bool     `json:"output_support"`
	AuthRequired bool     `json:"auth_required"`
	Category     string   `json:"category"`
	Actions      []string `json:"actions"`
	OutputModes  []string `json:"output_modes"`
	AgentNotes   []string `json:"agent_notes"`
}

var metadataByPath = map[string]commandMetadata{
	"wsectl":                       localMeta("discovery", "text"),
	"wsectl api":                   groupMeta("api"),
	"wsectl api actions":           localMeta("api", "json"),
	"wsectl api schema":            localMeta("api", "json"),
	"wsectl api call":              apiMeta("api", []string{"dynamic"}, allDataModes()...),
	"wsectl auth":                  groupMeta("auth"),
	"wsectl auth login":            localMeta("auth", "auto", "json", "yaml", "table"),
	"wsectl auth status":           localMeta("auth", "auto", "json", "yaml", "table"),
	"wsectl auth refresh":          authLocalMeta("auth", "auto", "json", "yaml", "table"),
	"wsectl auth logout":           authLocalMeta("auth", "auto", "json", "yaml", "table"),
	"wsectl commands":              localMeta("discovery", "json"),
	"wsectl comments":              groupMeta("comments"),
	"wsectl comments list":         apiMeta("comments", []string{"get_comments"}, allDataModes()...),
	"wsectl completion":            localMeta("completion", "text"),
	"wsectl completion bash":       localMeta("completion", "text"),
	"wsectl completion fish":       localMeta("completion", "text"),
	"wsectl completion powershell": localMeta("completion", "text"),
	"wsectl completion zsh":        localMeta("completion", "text"),
	"wsectl costs":                 groupMeta("costs"),
	"wsectl costs list":            apiMeta("costs", []string{"get_costs"}, allDataModes()...),
	"wsectl costs total":           apiMeta("costs", []string{"get_costs_total"}, allDataModes()...),
	"wsectl docs":                  groupMeta("docs"),
	"wsectl docs generate":         localMeta("docs", "markdown"),
	"wsectl doctor": {
		Category:    "diagnostics",
		Actions:     []string{"me"},
		OutputModes: []string{"text", "json"},
		AgentNotes:  []string{"The me action is called only when --api is set."},
		ReadOnly:    true,
	},
	"wsectl files": groupMeta("files"),
	"wsectl files download": {
		Category:     "files",
		Actions:      []string{"download"},
		OutputModes:  []string{"file", "stdout"},
		AgentNotes:   []string{"Use --out FILE for binary content."},
		ReadOnly:     true,
		AuthRequired: true,
	},
	"wsectl files images":           apiMeta("files", []string{"get_files"}, allDataModes()...),
	"wsectl files list":             apiMeta("files", []string{"get_files"}, allDataModes()...),
	"wsectl files task-attachments": apiMeta("files", []string{"get_task"}, allDataModes()...),
	"wsectl help":                   localMeta("discovery", "text", "json"),
	"wsectl history":                groupMeta("history"),
	"wsectl history clear":          localMeta("history", "auto", "json", "yaml", "table"),
	"wsectl history list":           localMeta("history", "auto", "json", "yaml", "table", "ndjson"),
	"wsectl history path":           localMeta("history", "auto", "json", "yaml", "table"),
	"wsectl me":                     apiMeta("identity", []string{"me"}, allDataModes()...),
	"wsectl profiles":               groupMeta("profiles"),
	"wsectl profiles add":           localMeta("profiles"),
	"wsectl profiles list":          localMeta("profiles", "auto", "json", "yaml", "table", "ndjson"),
	"wsectl profiles remove":        localMeta("profiles"),
	"wsectl profiles show":          localMeta("profiles", "auto", "json", "yaml", "table"),
	"wsectl profiles use":           localMeta("profiles"),
	"wsectl projects":               groupMeta("projects"),
	"wsectl projects events":        apiMeta("projects", []string{"get_events"}, allDataModes()...),
	"wsectl projects get":           apiMeta("projects", []string{"get_project"}, allDataModes()...),
	"wsectl projects groups":        apiMeta("projects", []string{"get_project_groups"}, allDataModes()...),
	"wsectl projects list":          apiMeta("projects", []string{"get_projects"}, allDataModes()...),
	"wsectl projects team":          apiMeta("projects", []string{"get_project"}, allDataModes()...),
	"wsectl tags":                   groupMeta("tags"),
	"wsectl tags project":           groupMeta("tags"),
	"wsectl tags project groups":    apiMeta("tags", []string{"get_project_tag_groups"}, allDataModes()...),
	"wsectl tags project list":      apiMeta("tags", []string{"get_project_tags"}, allDataModes()...),
	"wsectl tags task":              groupMeta("tags"),
	"wsectl tags task groups":       apiMeta("tags", []string{"get_task_tag_groups"}, allDataModes()...),
	"wsectl tags task list":         apiMeta("tags", []string{"get_task_tags"}, allDataModes()...),
	"wsectl tasks":                  groupMeta("tasks"),
	"wsectl tasks all":              apiMeta("tasks", []string{"get_all_tasks"}, allDataModes()...),
	"wsectl tasks discussion":       apiMeta("tasks", []string{"get_task"}, allDataModes()...),
	"wsectl tasks get":              apiMeta("tasks", []string{"get_task"}, allDataModes()...),
	"wsectl tasks list":             apiMeta("tasks", []string{"get_tasks"}, allDataModes()...),
	"wsectl tasks relations":        apiMeta("tasks", []string{"get_task"}, allDataModes()...),
	"wsectl tasks search":           apiMeta("tasks", []string{"search_tasks"}, allDataModes()...),
	"wsectl tasks subscribers":      apiMeta("tasks", []string{"get_task"}, allDataModes()...),
	"wsectl tasks subtasks":         apiMeta("tasks", []string{"get_task"}, allDataModes()...),
	"wsectl timers":                 groupMeta("timers"),
	"wsectl timers list":            apiMeta("timers", []string{"get_timers"}, allDataModes()...),
	"wsectl timers mine":            apiMeta("timers", []string{"get_my_timer"}, allDataModes()...),
	"wsectl users":                  groupMeta("users"),
	"wsectl users contact-groups":   apiMeta("users", []string{"get_contact_groups"}, allDataModes()...),
	"wsectl users contacts":         apiMeta("users", []string{"get_contacts"}, allDataModes()...),
	"wsectl users groups":           apiMeta("users", []string{"get_user_groups"}, allDataModes()...),
	"wsectl users list":             apiMeta("users", []string{"get_users"}, allDataModes()...),
	"wsectl users schedule":         apiMeta("users", []string{"get_users_schedule"}, allDataModes()...),
	"wsectl version":                localMeta("discovery", "text", "json", "yaml", "table", "ndjson", "raw"),
	"wsectl webhooks":               groupMeta("webhooks"),
	"wsectl webhooks list":          apiMeta("webhooks", []string{"get_webhooks"}, allDataModes()...),
}

func localMeta(category string, modes ...string) commandMetadata {
	return commandMetadata{Category: category, ReadOnly: true, OutputModes: modes}
}

func authLocalMeta(category string, modes ...string) commandMetadata {
	meta := localMeta(category, modes...)
	meta.AuthRequired = true
	return meta
}

func groupMeta(category string) commandMetadata {
	return commandMetadata{Category: category, ReadOnly: true}
}

func apiMeta(category string, actions []string, modes ...string) commandMetadata {
	return commandMetadata{
		Category:     category,
		Actions:      actions,
		OutputModes:  modes,
		ReadOnly:     true,
		AuthRequired: true,
		AgentNotes:   []string{"Prefer --json or --ndjson for automation."},
	}
}

func allDataModes() []string {
	return []string{"auto", "json", "yaml", "table", "ndjson", "raw"}
}

func applyCommandMetadata(root *cobra.Command) error {
	var missing []string
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if !isPublicCommand(cmd, root) {
			return
		}
		path := cmd.CommandPath()
		meta, ok := metadataByPath[path]
		if !ok {
			missing = append(missing, path)
		} else {
			if cmd.Example == "" {
				cmd.Example = commandExample(path)
			}
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations[annotationCategory] = meta.Category
			cmd.Annotations[annotationActions] = strings.Join(meta.Actions, "\x1f")
			cmd.Annotations[annotationOutputModes] = strings.Join(meta.OutputModes, "\x1f")
			cmd.Annotations[annotationAgentNotes] = strings.Join(meta.AgentNotes, "\x1f")
			cmd.Annotations[annotationReadOnly] = strconv.FormatBool(meta.ReadOnly)
			cmd.Annotations[annotationAuth] = strconv.FormatBool(meta.AuthRequired)
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing command metadata: %s", strings.Join(missing, ", "))
	}
	return nil
}

func setHistorySkip(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationHistorySkip] = "true"
}

func commandExample(path string) string {
	if example, ok := commandExamples[path]; ok {
		return example
	}
	return path + " --help"
}

var commandExamples = map[string]string{
	"wsectl":                        "wsectl\nwsectl help agent --full",
	"wsectl api actions":            "wsectl api actions --json",
	"wsectl api schema":             "wsectl api schema get_tasks --json",
	"wsectl auth login":             "wsectl auth login --client-id \"$WSECTL_CLIENT_ID\"",
	"wsectl auth status":            "wsectl auth status --json",
	"wsectl auth refresh":           "wsectl auth refresh",
	"wsectl auth logout":            "wsectl auth logout",
	"wsectl commands":               "wsectl commands --json",
	"wsectl comments list":          "wsectl comments list 456 --extra files --json",
	"wsectl completion bash":        "wsectl completion bash > /etc/bash_completion.d/wsectl",
	"wsectl completion fish":        "wsectl completion fish > ~/.config/fish/completions/wsectl.fish",
	"wsectl completion powershell":  "wsectl completion powershell > wsectl.ps1",
	"wsectl completion zsh":         "wsectl completion zsh > \"${fpath[1]}/_wsectl\"",
	"wsectl costs list":             "wsectl costs list --project 123 --json",
	"wsectl costs total":            "wsectl costs total --project 123 --json",
	"wsectl docs generate":          "wsectl docs generate --out docs/command-reference.md",
	"wsectl files download":         "wsectl files download 789 --out ./attachment.bin",
	"wsectl files images":           "wsectl files images --task 456 --json",
	"wsectl files list":             "wsectl files list --project 123 --json",
	"wsectl files task-attachments": "wsectl files task-attachments 456 --json",
	"wsectl help":                   "wsectl help agent --full\nwsectl help agent --json",
	"wsectl history clear":          "wsectl history clear --keep 1000\nwsectl history clear",
	"wsectl history list":           "wsectl history list --json --limit 20",
	"wsectl history path":           "wsectl history path --json",
	"wsectl me":                     "wsectl me --json",
	"wsectl profiles add":           "wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2",
	"wsectl profiles list":          "wsectl profiles list --json",
	"wsectl profiles remove":        "wsectl profiles remove old-account",
	"wsectl profiles show":          "wsectl profiles show default --json",
	"wsectl profiles use":           "wsectl profiles use default",
	"wsectl projects events":        "wsectl projects events --project 123 --period month --json",
	"wsectl projects groups":        "wsectl projects groups --json",
	"wsectl projects list":          "wsectl projects list --status active --json",
	"wsectl projects team":          "wsectl projects team 123 --json",
	"wsectl tags project groups":    "wsectl tags project groups --type status --json",
	"wsectl tags project list":      "wsectl tags project list --type label --json",
	"wsectl tags task groups":       "wsectl tags task groups --type status --json",
	"wsectl tags task list":         "wsectl tags task list --type label --json",
	"wsectl tasks all":              "wsectl tasks all --extra text,files --json --out /tmp/tasks.json",
	"wsectl tasks discussion":       "wsectl tasks discussion 456 --json",
	"wsectl tasks get":              "wsectl tasks get 456 --extra text,files --json",
	"wsectl tasks list":             "wsectl tasks list --project 123 --status active --json",
	"wsectl tasks relations":        "wsectl tasks relations 456 --json",
	"wsectl tasks search":           "wsectl tasks search --query \"invoice\" --json",
	"wsectl tasks subscribers":      "wsectl tasks subscribers 456 --json",
	"wsectl tasks subtasks":         "wsectl tasks subtasks 456 --json",
	"wsectl timers list":            "wsectl timers list --json",
	"wsectl timers mine":            "wsectl timers mine --json",
	"wsectl users contact-groups":   "wsectl users contact-groups --json",
	"wsectl users contacts":         "wsectl users contacts --json",
	"wsectl users groups":           "wsectl users groups --json",
	"wsectl users list":             "wsectl users list --json",
	"wsectl users schedule":         "wsectl users schedule --start 01.05.2026 --end 31.05.2026 --json",
	"wsectl version":                "wsectl version\nwsectl version --json",
	"wsectl webhooks list":          "wsectl webhooks list --json",
}

func collectCommands(root *cobra.Command) []commandInfo {
	var out []commandInfo
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if !isPublicCommand(c, root) {
			return
		}
		flags := []string{}
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name != "help" {
				flags = append(flags, "--"+f.Name)
			}
		})
		annotations := c.Annotations
		outputModes := splitAnnotation(annotations[annotationOutputModes])
		out = append(out, commandInfo{
			Path:         c.CommandPath(),
			Short:        c.Short,
			Long:         c.Long,
			Usage:        strings.TrimSpace(c.UseLine()),
			Flags:        flags,
			Examples:     c.Example,
			ReadOnly:     annotations[annotationReadOnly] == "true",
			Output:       len(outputModes) > 0,
			AuthRequired: annotations[annotationAuth] == "true",
			Category:     annotations[annotationCategory],
			Actions:      splitAnnotation(annotations[annotationActions]),
			OutputModes:  outputModes,
			AgentNotes:   splitAnnotation(annotations[annotationAgentNotes]),
		})
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func isPublicCommand(cmd, root *cobra.Command) bool {
	if cmd.Hidden {
		return false
	}
	return cmd == root || cmd.IsAvailableCommand() || cmd.Name() == "help"
}

func splitAnnotation(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, "\x1f")
}
