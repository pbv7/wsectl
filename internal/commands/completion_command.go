package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Long:  "Generate a completion script from the current wsectl command tree.",
		Args:  cobra.NoArgs,
	}
	setHistorySkip(cmd)
	cmd.AddCommand(
		completionShellCommand("bash", func(cmd *cobra.Command) error {
			return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
		}),
		completionShellCommand("zsh", func(cmd *cobra.Command) error {
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		}),
		completionShellCommand("fish", func(cmd *cobra.Command) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		}),
		completionShellCommand("powershell", func(cmd *cobra.Command) error {
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}),
	)
	return cmd
}

func completionShellCommand(shell string, generate func(*cobra.Command) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   shell,
		Short:                 fmt.Sprintf("Generate the completion script for %s", shell),
		Args:                  cobra.NoArgs,
		DisableFlagsInUseLine: true,
		ValidArgsFunction:     cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return generate(cmd)
		},
	}
	return cmd
}
