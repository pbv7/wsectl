package commands

import "github.com/spf13/cobra"

func newReadCommands(s *state) []*cobra.Command {
	return []*cobra.Command{
		newSimpleActionCommand(s, "me", "me", "Get authorized user info", nil),
		newUsersCommand(s),
		newProjectsCommand(s),
		newTasksCommand(s),
		newCommentsCommand(s),
		newTagsCommand(s),
		newCostsCommand(s),
		newTimersCommand(s),
		newFilesCommand(s),
		newWebhooksCommand(s),
	}
}

func newSimpleActionCommand(s *state, use, action, short string, build func(*cobra.Command, []string) map[string]string) *cobra.Command {
	var buildE func(*cobra.Command, []string) (map[string]string, error)
	if build != nil {
		buildE = func(cmd *cobra.Command, args []string) (map[string]string, error) {
			return build(cmd, args), nil
		}
	}
	return newSimpleActionCommandE(s, use, action, short, buildE)
}

func newSimpleActionCommandE(s *state, use, action, short string, build func(*cobra.Command, []string) (map[string]string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]string{}
			if build != nil {
				var err error
				params, err = build(cmd, args)
				if err != nil {
					return err
				}
			}
			return s.runAction(cmd, action, params)
		},
	}
}
