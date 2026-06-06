package commands

import (
	"encoding/json"
	"fmt"

	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/spf13/cobra"
)

func newProfilesCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "profiles", Short: "Manage Worksection profiles"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := s.loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(cfg.Profiles)
			return output.Write(cmd.OutOrStdout(), output.Success("profiles list", cfg.ActiveProfileName(), "", raw), s.outputOptions())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:               "show [NAME]",
		Short:             "Show a profile",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeConfiguredProfiles(s),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := s.loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			name := cfg.ActiveProfileName()
			if len(args) > 0 {
				name = args[0]
			}
			p, ok := cfg.Profiles[name]
			if !ok {
				return configErr("profile %q not found", name)
			}
			raw, _ := json.Marshal(p)
			return output.Write(cmd.OutOrStdout(), output.Success("profiles show", name, p.AccountURL, raw), s.outputOptions())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:               "use NAME",
		Short:             "Set current profile",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeConfiguredProfiles(s),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := s.loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return configErr("profile %q not found", args[0])
			}
			cfg.CurrentProfile = args[0]
			return config.Save(cfg)
		},
	})
	var accountURL, authType, secretRef string
	add := &cobra.Command{
		Use:   "add NAME",
		Short: "Add or update a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := s.loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			if err := config.AddProfile(&cfg, args[0], config.Profile{AccountURL: accountURL, AuthType: authType, SecretRef: secretRef}); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	}
	add.Flags().StringVar(&accountURL, "account-url", "", "Worksection account URL")
	add.Flags().StringVar(&authType, "auth-type", "oauth2", "Auth type: oauth2 or admin_token")
	add.Flags().StringVar(&secretRef, "secret-ref", "", "Secret ref, for example keyring:wsectl/default")
	_ = add.RegisterFlagCompletionFunc("auth-type", completeValues("oauth2", "admin_token"))
	_ = add.RegisterFlagCompletionFunc("secret-ref", completeValues("keyring:wsectl/default", "env:", "encrypted-file:", "plaintext:"))
	cmd.AddCommand(add)
	cmd.AddCommand(&cobra.Command{
		Use:               "remove NAME",
		Short:             "Remove a profile",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeConfiguredProfiles(s),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := s.loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			if err := config.RemoveProfile(&cfg, args[0]); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	})
	return cmd
}

func configErr(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
