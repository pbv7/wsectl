package commands

import "github.com/spf13/cobra"

func newWebhooksCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "webhooks", Short: "Read webhooks"}
	cmd.AddCommand(newSimpleActionCommand(s, "list", "get_webhooks", "List webhooks", nil))
	return cmd
}
