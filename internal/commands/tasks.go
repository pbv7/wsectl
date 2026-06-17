package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newTasksCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "tasks", Short: "Read and search tasks"}
	var status, extra string
	all := newSimpleActionCommand(s, "all", "get_all_tasks", "List all account tasks", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"filter": taskListFilter(status), "extra": extra}
	})
	all.Long = "List all account tasks. Worksection can cap large responses at 10000 records; check meta.truncated and meta.warnings."
	all.Example = "wsectl tasks all --extra text,files --json --out /tmp/tasks.json"
	all.Flags().StringVar(&status, "status", "", "Task status: active or all (for completed tasks use: wsectl tasks search --project ID --status done)")
	all.Flags().StringVar(&extra, "extra", "", "Extra fields: text, files, comments, relations, subtasks, subscribers")
	_ = all.RegisterFlagCompletionFunc("status", completeValues("active", "all"))
	_ = all.RegisterFlagCompletionFunc("extra", commaValueCompletion("text", "files", "comments", "relations", "subtasks", "subscribers"))
	cmd.AddCommand(all)
	var project, listStatus, listExtra string
	list := newSimpleActionCommand(s, "list", "get_tasks", "List project tasks", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"id_project": project, "filter": taskListFilter(listStatus), "extra": listExtra}
	})
	list.Flags().StringVar(&project, "project", "", "Project ID")
	list.Flags().StringVar(&listStatus, "status", "", "Task status: active or all (for completed tasks use: wsectl tasks search --project ID --status done)")
	list.Flags().StringVar(&listExtra, "extra", "", "Extra fields")
	_ = list.RegisterFlagCompletionFunc("status", completeValues("active", "all"))
	_ = list.RegisterFlagCompletionFunc("extra", commaValueCompletion("text", "files", "comments", "relations", "subtasks", "subscribers"))
	cmd.AddCommand(list)
	var getExtra string
	get := &cobra.Command{
		Use:   "get TASK_ID",
		Short: "Get a task",
		Args:  exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "get_task", nil)
			}
			return s.runAction(cmd, "get_task", map[string]string{"id_task": args[0], "extra": getExtra})
		},
	}
	get.Flags().StringVar(&getExtra, "extra", "", "Extra fields")
	_ = get.RegisterFlagCompletionFunc("extra", commaValueCompletion("text", "files", "comments", "relations", "subtasks", "subscribers"))
	cmd.AddCommand(get)
	var query, filter, searchProject, searchTask, assignee, author, searchStatus, searchExtra string
	var topLevelOnly bool
	buildSearch := func(*cobra.Command, []string) (map[string]string, error) {
		if query != "" && filter != "" {
			return nil, worksection.UsageError("--query and --filter cannot be used together")
		}
		// search_tasks requires at least one search dimension upstream; without
		// one it fails with a cryptic "At least one of fields required". Fail
		// fast with a flag-named message instead. Skipped for --schema, which
		// short-circuits before the request and takes no flags.
		if !s.schema && query == "" && filter == "" && searchProject == "" && searchTask == "" && assignee == "" && author == "" {
			return nil, worksection.UsageError("tasks search needs at least one of --query, --filter, --project, --task, --assignee, or --author (--status and --extra are modifiers, not search criteria)")
		}
		params := map[string]string{
			"filter":          filter,
			"id_project":      searchProject,
			"id_task":         searchTask,
			"email_user_to":   assignee,
			"email_user_from": author,
			"status":          searchStatus,
			"extra":           searchExtra,
		}
		if query != "" {
			params["filter"] = taskNameFilter(query)
		}
		return params, nil
	}
	search := newSimpleActionCommandE(s, "search", "search_tasks", "Search tasks", buildSearch)
	search.Long = "Search tasks by simple query or advanced Worksection filter. Prefer --json for scripts and use --out for large results."
	search.Example = "wsectl tasks search --query invoice --json\nwsectl tasks search --filter \"name has 'Report'\" --json\nwsectl tasks search --project 123 --assignee user@example.com --status active --json\nwsectl tasks search --task 456 --json"
	search.Flags().StringVar(&query, "query", "", "Search text")
	search.Flags().StringVar(&filter, "filter", "", "Advanced Worksection filter")
	search.Flags().StringVar(&searchProject, "project", "", "Project ID")
	search.Flags().StringVar(&searchTask, "task", "", "Task ID")
	search.Flags().StringVar(&assignee, "assignee", "", "Assignee email")
	search.Flags().StringVar(&author, "author", "", "Author email")
	search.Flags().StringVar(&searchStatus, "status", "", "Task status")
	search.Flags().StringVar(&searchExtra, "extra", "", "Extra fields")
	_ = search.RegisterFlagCompletionFunc("status", completeValues("active", "done", "all"))
	_ = search.RegisterFlagCompletionFunc("extra", commaValueCompletion("text", "files", "comments", "relations", "subtasks", "subscribers"))
	search.Flags().BoolVar(&topLevelOnly, "top-level-only", false, "Exclude subtasks from results (drop rows with a parent); applied client-side")
	// Override RunE so --top-level-only can filter the parsed result. The filter
	// is client-side, so it is incompatible with raw output (which streams the
	// upstream response untransformed). Reject that combination before the
	// request rather than silently returning unfiltered rows. The raw default
	// can come from --raw, WSECTL_OUTPUT, or the config, so resolve the config
	// (cached) before checking instead of reading the CLI flag alone.
	search.RunE = func(cmd *cobra.Command, args []string) error {
		if s.schema {
			return s.runAction(cmd, "search_tasks", nil)
		}
		if topLevelOnly {
			if _, err := s.loadConfig(cmd.Context()); err != nil {
				return writeFailure(cmd.ErrOrStderr(), s, "search_tasks", "", err)
			}
			if s.format == "raw" {
				return worksection.UsageError("--top-level-only cannot be combined with raw output (--raw, WSECTL_OUTPUT=raw, or a raw config default); raw mode streams the upstream response without client-side filtering")
			}
		}
		params, err := buildSearch(cmd, args)
		if err != nil {
			return err
		}
		if topLevelOnly {
			return s.runActionWithOptions(cmd, "search_tasks", params, false, filterTopLevelTasks)
		}
		return s.runAction(cmd, "search_tasks", params)
	}
	cmd.AddCommand(search)
	cmd.AddCommand(taskExtraCommand(s, "subtasks", "subtasks"))
	cmd.AddCommand(taskExtraCommand(s, "relations", "relations"))
	cmd.AddCommand(taskExtraCommand(s, "subscribers", "subscribers"))
	cmd.AddCommand(taskExtraCommand(s, "discussion", "comments"))
	return cmd
}

func taskExtraCommand(s *state, use, extra string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " TASK_ID",
		Short: "Get task " + extra,
		Args:  exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "get_task", nil)
			}
			return s.runAction(cmd, "get_task", map[string]string{"id_task": args[0], "extra": extra})
		},
	}
}

func taskListFilter(status string) string {
	if status == "active" {
		return "active"
	}
	if status == "done" {
		return "done"
	}
	return ""
}

func taskNameFilter(query string) string {
	escaped := strings.ReplaceAll(query, "'", "\\'")
	return "name has '" + escaped + "'"
}

// filterTopLevelTasks drops subtask rows (those carrying a non-null parent)
// from a search_tasks array, implementing --top-level-only client-side.
func filterTopLevelTasks(data json.RawMessage) (json.RawMessage, []string, error) {
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return data, []string{"--top-level-only was skipped because the response was not a task array."}, nil
	}
	kept := make([]map[string]any, 0, len(rows))
	dropped := 0
	for _, row := range rows {
		if p, ok := row["parent"]; ok && p != nil {
			dropped++
			continue
		}
		kept = append(kept, row)
	}
	raw, err := json.Marshal(kept)
	if err != nil {
		return nil, nil, err
	}
	var warnings []string
	if dropped > 0 {
		warnings = []string{fmt.Sprintf("--top-level-only removed %d subtask row(s) client-side; search_tasks itself returns subtasks.", dropped)}
	}
	return raw, warnings, nil
}
