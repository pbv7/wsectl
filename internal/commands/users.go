package commands

import "github.com/spf13/cobra"

func newUsersCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "users", Short: "Read users, groups, contacts, and schedules"}
	cmd.AddCommand(newSimpleActionCommand(s, "list", "get_users", "List users", nil))
	cmd.AddCommand(newSimpleActionCommand(s, "groups", "get_user_groups", "List user groups", nil))
	cmd.AddCommand(newSimpleActionCommand(s, "contacts", "get_contacts", "List contacts", nil))
	cmd.AddCommand(newSimpleActionCommand(s, "contact-groups", "get_contact_groups", "List contact groups", nil))
	var users, start, end string
	schedule := newSimpleActionCommand(s, "schedule", "get_users_schedule", "List users' non-working days", func(*cobra.Command, []string) map[string]string {
		return map[string]string{"users": users, "datestart": start, "dateend": end}
	})
	schedule.Flags().StringVar(&users, "users", "", "Comma-separated user IDs or emails")
	schedule.Flags().StringVar(&start, "start", "", "Start date DD.MM.YYYY")
	schedule.Flags().StringVar(&end, "end", "", "End date DD.MM.YYYY")
	cmd.AddCommand(schedule)
	return cmd
}
