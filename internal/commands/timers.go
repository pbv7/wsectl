package commands

import "github.com/spf13/cobra"

func newTimersCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "timers", Short: "Read timers"}
	cmd.AddCommand(newSimpleActionCommand(s, "list", "get_timers", "List enabled member timers", nil))
	cmd.AddCommand(newSimpleActionCommand(s, "mine", "get_my_timer", "Get current user timer", nil))
	return cmd
}
