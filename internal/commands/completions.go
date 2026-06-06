package commands

import (
	"strings"

	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

// registerGlobalCompletions attaches dynamic and enum completions to global
// flags. Cobra's generated shell scripts call these functions at runtime.
func registerGlobalCompletions(root *cobra.Command, s *state) {
	_ = root.RegisterFlagCompletionFunc("output", completeValues("auto", "json", "yaml", "table", "ndjson", "raw"))
	_ = root.RegisterFlagCompletionFunc("profile", completeProfiles(s))
	_ = root.RegisterFlagCompletionFunc("rate-limit", completeValues("1/s", "2/s", "5/s"))
}

func completeValues(values ...string) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeActions(readOnlyOnly bool) cobra.CompletionFunc {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		items := []string{}
		for _, action := range worksection.Actions() {
			if readOnlyOnly && !action.ReadOnly {
				continue
			}
			item := action.Name
			if action.Description != "" {
				item += "\t" + action.Description
			}
			items = append(items, item)
		}
		return items, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeProfiles(s *state) cobra.CompletionFunc {
	return func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg, err := s.loadConfig(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		items := []string{}
		for name, profile := range cfg.Profiles {
			label := name
			if profile.AccountURL != "" {
				label += "\t" + profile.AccountURL
			}
			items = append(items, label)
		}
		return items, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeConfiguredProfiles(s *state) cobra.CompletionFunc {
	return func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg, err := config.Load(cmd.Context(), config.Overrides{ConfigPath: s.configPath})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		items := []string{}
		for name, profile := range cfg.Profiles {
			item := name
			if profile.AccountURL != "" {
				item += "\t" + profile.AccountURL
			}
			items = append(items, item)
		}
		return items, cobra.ShellCompDirectiveNoFileComp
	}
}

func commaValueCompletion(values ...string) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		prefix := ""
		if idx := strings.LastIndex(toComplete, ","); idx >= 0 {
			prefix = toComplete[:idx+1]
		}
		items := make([]string, 0, len(values))
		for _, value := range values {
			items = append(items, prefix+value)
		}
		return items, cobra.ShellCompDirectiveNoFileComp
	}
}
