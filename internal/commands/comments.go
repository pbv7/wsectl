package commands

import "github.com/spf13/cobra"

func newCommentsCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "comments", Short: "Read comments"}
	var extra string
	list := &cobra.Command{
		Use:   "list TASK_ID",
		Short: "List task comments",
		Args:  exactArgsUnlessSchema(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if s.schema {
				return s.runAction(cmd, "get_comments", nil)
			}
			return s.runAction(cmd, "get_comments", map[string]string{"id_task": args[0], "extra": extra})
		},
	}
	list.Flags().StringVar(&extra, "extra", "", "Extra fields, for example files")
	cmd.AddCommand(list)
	return cmd
}
