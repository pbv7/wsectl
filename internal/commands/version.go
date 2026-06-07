package commands

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/pbv7/wsectl/internal/output"
	"github.com/spf13/cobra"
)

func newVersionCommand(s *state) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			raw, _ := json.Marshal(map[string]string{
				"version":    s.version,
				"commit":     s.commit,
				"date":       s.date,
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			})
			if s.format == "" || s.format == "auto" {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "wsectl %s\n", s.version)
				return err
			}
			return output.Write(cmd.OutOrStdout(), output.Success("version", "", "", raw), s.outputOptions())
		},
	}
}
