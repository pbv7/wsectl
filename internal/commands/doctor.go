package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pbv7/wsectl/internal/doctor"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newDoctorCommand(s *state) *cobra.Command {
	var checkAPI bool
	cmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Diagnose configuration, credentials, and optional API connectivity",
		Long:    "Run local configuration and credential checks. Pass --api to also perform one authenticated Worksection `me` request.",
		Example: "wsectl doctor\nwsectl doctor --json\nwsectl doctor --api --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			deps := doctor.DefaultDependencies()
			deps.APICheck = func(ctx context.Context) error {
				clientInfo, err := s.client(ctx)
				if err != nil {
					return err
				}
				s.writeEnvFallbackDiagnostic(cmd, clientInfo)
				_, err = clientInfo.client.Call(ctx, "me", nil)
				return err
			}
			report, diagnosisErr := doctor.Run(cmd.Context(), doctor.Options{
				Overrides: s.configOverrides(),
				CheckAPI:  checkAPI,
			}, deps)
			jsonOutput, _ := cmd.Flags().GetBool("json")
			if jsonOutput || s.machineFormat() {
				opts := s.outputOptions()
				if jsonOutput {
					opts.Format = "json"
				} else if opts.Format == "" || opts.Format == "auto" {
					opts.Format = s.effectiveFormat()
				}
				if err := writeDoctorMachine(cmd.OutOrStdout(), report, diagnosisErr, opts); err != nil {
					return err
				}
				return markRendered(diagnosisErr)
			} else {
				if err := writeDoctorText(cmd.OutOrStdout(), report); err != nil {
					return err
				}
			}
			return diagnosisErr
		},
	}
	cmd.Flags().BoolVar(&checkAPI, "api", false, "Perform one authenticated Worksection me request")
	return cmd
}

func writeDoctorText(w io.Writer, report doctor.Report) error {
	if _, err := fmt.Fprintf(w, "wsectl doctor\n\nConfig:  %s\nProfile: %s\nAccount: %s\n\n", report.ConfigPath, report.Profile, report.AccountURL); err != nil {
		return err
	}
	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(w, "[%s] %-20s %s\n", check.Status, check.Name, check.Message); err != nil {
			return err
		}
	}
	if len(report.Remediation) > 0 {
		if _, err := fmt.Fprintln(w, "\nRemediation:"); err != nil {
			return err
		}
		for _, item := range report.Remediation {
			if _, err := fmt.Fprintln(w, "  - "+item); err != nil {
				return err
			}
		}
	}
	status := "healthy"
	if !report.Healthy {
		status = "unhealthy"
	}
	_, err := fmt.Fprintln(w, "\nOverall: "+status)
	return err
}

func writeDoctorMachine(w io.Writer, report doctor.Report, diagnosisErr error, opts output.Options) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	env := output.Envelope{
		Status: "ok",
		Data:   raw,
		Meta: output.Meta{
			Action:   "doctor",
			Profile:  report.Profile,
			Warnings: []string{},
		},
	}
	if diagnosisErr != nil {
		env.Status = "error"
		env.Error = &output.ErrorBody{Code: "general", Message: diagnosisErr.Error(), Details: map[string]any{}}
		if wsErr, ok := diagnosisErr.(*worksection.Error); ok {
			env.Error.Code = string(wsErr.Code)
			env.Error.Details = wsErr.Details
		}
	}
	return output.Write(w, env, opts)
}
