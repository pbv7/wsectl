package commands

import (
	"encoding/json"
	"os"

	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newAPICommand(s *state) *cobra.Command {
	api := &cobra.Command{Use: "api", Short: "Low-level Worksection API access"}
	api.AddCommand(&cobra.Command{
		Use:   "actions",
		Short: "List known Worksection API actions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, _ := json.Marshal(worksection.Actions())
			return output.Write(cmd.OutOrStdout(), output.Success("api actions", "", "", raw), discoveryOutputOptions(s))
		},
	})
	schema := &cobra.Command{
		Use:               "schema ACTION",
		Short:             "Show known action parameters",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeActions(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, ok := worksection.Schema(args[0])
			if !ok {
				return worksection.UsageError("unknown action %q", args[0])
			}
			raw, _ := json.Marshal(a)
			return output.Write(cmd.OutOrStdout(), output.Success("api schema", "", "", raw), discoveryOutputOptions(s))
		},
	}
	api.AddCommand(schema)
	var pairs []string
	var paramsJSON string
	var allowUnknown bool
	call := &cobra.Command{
		Use:               "call ACTION",
		Short:             "Call a read-only Worksection API action",
		Example:           "wsectl api call get_users_schedule --param datestart=01.05.2026 --param dateend=31.05.2026 --json",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeActions(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			action := args[0]
			a, known := worksection.LookupAction(action)
			if known && !a.ReadOnly {
				return worksection.UsageError("This action changes Worksection data and is blocked in the read-only build.")
			}
			if !known && !allowUnknown {
				return worksection.UsageError("unknown action %q; pass --allow-unknown to call it", action)
			}
			params, err := paramsFromPairs(pairs)
			if err != nil {
				return err
			}
			if paramsJSON != "" {
				raw, err := os.ReadFile(paramsJSON)
				if err != nil {
					return err
				}
				var extra map[string]string
				if err := json.Unmarshal(raw, &extra); err != nil {
					return err
				}
				for k, v := range extra {
					params[k] = v
				}
			}
			return s.runActionWithOptions(cmd, action, params, allowUnknown, nil)
		},
	}
	call.Flags().StringArrayVar(&pairs, "param", nil, "Parameter as key=value")
	call.Flags().StringVar(&paramsJSON, "params-json", "", "JSON file containing string parameters")
	call.Flags().BoolVar(&allowUnknown, "allow-unknown", false, "Allow actions not in the local registry")
	_ = call.MarkFlagFilename("params-json", "json")
	api.AddCommand(call)
	return api
}

func discoveryOutputOptions(s *state) output.Options {
	opts := s.outputOptions()
	if opts.Format == "" || opts.Format == "auto" {
		opts.Format = "json"
	}
	return opts
}
