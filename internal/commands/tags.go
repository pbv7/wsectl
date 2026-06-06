package commands

import "github.com/spf13/cobra"

func newTagsCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "tags", Short: "Read task and project tags"}
	cmd.AddCommand(tagScopeCommand(s, "task", "get_task_tags", "get_task_tag_groups"))
	cmd.AddCommand(tagScopeCommand(s, "project", "get_project_tags", "get_project_tag_groups"))
	return cmd
}

func tagScopeCommand(s *state, use, listAction, groupsAction string) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: "Read " + use + " tags"}
	var group, typ, access string
	list := newSimpleActionCommand(s, "list", listAction, "List tags", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"group": group, "type": typ, "access": access}
	})
	list.Flags().StringVar(&group, "group", "", "Tag group")
	list.Flags().StringVar(&typ, "type", "", "Tag type: status or label")
	list.Flags().StringVar(&access, "access", "", "Access: public or private")
	_ = list.RegisterFlagCompletionFunc("type", completeValues("status", "label"))
	_ = list.RegisterFlagCompletionFunc("access", completeValues("public", "private"))
	cmd.AddCommand(list)
	var gType, gAccess string
	groups := newSimpleActionCommand(s, "groups", groupsAction, "List tag groups", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"type": gType, "access": gAccess}
	})
	groups.Flags().StringVar(&gType, "type", "", "Tag type: status or label")
	groups.Flags().StringVar(&gAccess, "access", "", "Access: public or private")
	_ = groups.RegisterFlagCompletionFunc("type", completeValues("status", "label"))
	_ = groups.RegisterFlagCompletionFunc("access", completeValues("public", "private"))
	cmd.AddCommand(groups)
	return cmd
}
