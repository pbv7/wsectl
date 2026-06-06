package commands

import "github.com/spf13/cobra"

func newCostsCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "costs", Short: "Read costs"}
	cmd.AddCommand(costCommand(s, "list", "get_costs"))
	cmd.AddCommand(costCommand(s, "total", "get_costs_total"))
	return cmd
}

func costCommand(s *state, use, action string) *cobra.Command {
	var project, task, start, end, timer, filter, extra string
	cmd := newSimpleActionCommand(s, use, action, "Read costs", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"id_project": project, "id_task": task, "datestart": start, "dateend": end, "is_timer": timer, "filter": filter, "extra": extra}
	})
	cmd.Flags().StringVar(&project, "project", "", "Project ID")
	cmd.Flags().StringVar(&task, "task", "", "Task ID")
	cmd.Flags().StringVar(&start, "start", "", "Start date DD.MM.YYYY")
	cmd.Flags().StringVar(&end, "end", "", "End date DD.MM.YYYY")
	cmd.Flags().StringVar(&timer, "timer", "", "Filter timer costs: true or false")
	cmd.Flags().StringVar(&filter, "filter", "", "Worksection filter")
	cmd.Flags().StringVar(&extra, "extra", "", "Extra fields")
	_ = cmd.RegisterFlagCompletionFunc("timer", completeValues("true", "false"))
	return cmd
}
