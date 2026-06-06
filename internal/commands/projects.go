package commands

import "github.com/spf13/cobra"

func newProjectsCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "projects", Short: "Read projects"}
	var status, extra string
	list := newSimpleActionCommand(s, "list", "get_projects", "List projects", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"filter": projectStatusFilter(status), "extra": extra}
	})
	list.Example = "wsectl projects list --status active --extra text,options,users --json"
	list.Flags().StringVar(&status, "status", "", "Project status: active, pending, archived")
	list.Flags().StringVar(&extra, "extra", "", "Extra fields: text, options, users")
	_ = list.RegisterFlagCompletionFunc("status", completeValues("active", "pending", "archived"))
	_ = list.RegisterFlagCompletionFunc("extra", commaValueCompletion("text", "options", "users"))
	cmd.AddCommand(list)
	var getExtra string
	get := &cobra.Command{
		Use:     "get PROJECT_ID",
		Short:   "Get a project",
		Example: "wsectl projects get 123 --extra text,users --json",
		Args:    exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "get_project", nil)
			}
			return s.runAction(cmd, "get_project", map[string]string{"id_project": args[0], "extra": getExtra})
		},
	}
	get.Flags().StringVar(&getExtra, "extra", "", "Extra fields: text, options, users")
	_ = get.RegisterFlagCompletionFunc("extra", commaValueCompletion("text", "options", "users"))
	cmd.AddCommand(get)
	cmd.AddCommand(newSimpleActionCommand(s, "groups", "get_project_groups", "List project groups", nil))
	var project, period string
	events := newSimpleActionCommand(s, "events", "get_events", "Get project events", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"id_project": project, "period": period}
	})
	events.Flags().StringVar(&project, "project", "", "Project ID")
	events.Flags().StringVar(&period, "period", "", "Period")
	cmd.AddCommand(events)
	team := &cobra.Command{
		Use:   "team PROJECT_ID",
		Short: "Get project team",
		Args:  exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "get_project", nil)
			}
			return s.runAction(cmd, "get_project", map[string]string{"id_project": args[0], "extra": "users"})
		},
	}
	cmd.AddCommand(team)
	return cmd
}

func projectStatusFilter(status string) string {
	if status == "archived" {
		return "archive"
	}
	return status
}
