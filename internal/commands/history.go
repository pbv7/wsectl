package commands

import (
	"encoding/json"
	"fmt"

	"github.com/pbv7/wsectl/internal/history"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newHistoryCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Inspect local command history",
		Long:  "Inspect optional local JSONL command history. History is disabled by default and never stores tokens or full Worksection responses.",
	}
	setHistorySkip(cmd)
	cmd.AddCommand(newHistoryListCommand(s), newHistoryPathCommand(s), newHistoryClearCommand(s))
	return cmd
}

func newHistoryListCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local command history",
		Long:  "List optional local command history entries from the configured JSONL history file.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			options, err := s.historyPathInfo(cmd.Context())
			if err != nil {
				return err
			}
			result, err := history.ReadWithStats(options.Path, s.limit)
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(result.Events)
			env := output.Success("history list", "", "", raw)
			if result.Malformed > 0 {
				env.Meta.Warnings = append(env.Meta.Warnings, fmt.Sprintf("Skipped %d malformed history line(s).", result.Malformed))
			}
			return output.Write(cmd.OutOrStdout(), env, s.outputOptions())
		},
	}
	return cmd
}

func newHistoryPathCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show local command history path",
		Long:  "Show whether optional local command history is enabled and which JSONL file path is used.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			options, err := s.historyPathInfo(cmd.Context())
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(map[string]any{
				"enabled":        options.Enabled,
				"path":           options.Path,
				"include_params": options.IncludeParams,
			})
			env := output.Success("history path", "", "", raw)
			return output.Write(cmd.OutOrStdout(), env, s.outputOptions())
		},
	}
	return cmd
}

func newHistoryClearCommand(s *state) *cobra.Command {
	var keep int
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear local command history",
		Long: "Delete the configured local command history JSONL file, or keep only the latest N entries with --keep.\n" +
			"This command uses a local lock to avoid racing concurrent history writes and does not write a replacement history event.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			options, err := s.historyPathInfo(cmd.Context())
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("keep") && keep < 0 {
				return worksection.UsageError("--keep must be zero or positive")
			}
			if keep >= 0 {
				if err := history.Trim(cmd.Context(), options.Path, keep); err != nil {
					return err
				}
			} else if err := history.Clear(cmd.Context(), options.Path); err != nil {
				return err
			}
			raw, _ := json.Marshal(map[string]any{
				"cleared": keep < 0,
				"keep":    keep,
				"path":    options.Path,
			})
			env := output.Success("history clear", "", "", raw)
			return output.Write(cmd.OutOrStdout(), env, s.outputOptions())
		},
	}
	cmd.Flags().IntVar(&keep, "keep", -1, "Keep the latest N history entries instead of deleting the file")
	return cmd
}
