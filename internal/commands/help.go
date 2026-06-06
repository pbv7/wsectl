package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pbv7/wsectl/internal/docs"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/spf13/cobra"
)

func newHelpCommand(_ *state) *cobra.Command {
	var jsonOut, full bool
	cmd := &cobra.Command{
		Use:   "help [topic|command]",
		Short: "Show detailed help",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			topic := "manual"
			if len(args) > 0 {
				topic = args[0]
			}
			if topic == "manual" {
				return writeGuide(cmd, rootGuide(), jsonOut)
			}
			if topic == "agent" {
				return writeGuide(cmd, agentGuide(full, collectCommands(cmd.Root())), jsonOut)
			}
			name := map[string]string{
				"auth":       "auth.md",
				"config":     "configuration.md",
				"output":     "output-contracts.md",
				"completion": "completion.md",
				"doctor":     "doctor.md",
				"examples":   "examples.md",
				"env":        "env.md",
				"limits":     "limits.md",
			}[topic]
			if name != "" && len(args) == 1 {
				text := docs.Read(name)
				if jsonOut {
					doc := guideDocument{
						Topic:              topic,
						Content:            text,
						GuideFormatVersion: guideFormatVersion,
						Sections:           []guideSection{{ID: topic, Title: topic, Lines: strings.Split(strings.TrimSpace(text), "\n")}},
					}
					return writeGuide(cmd, doc, true)
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), text)
				return err
			}
			target, _, err := cmd.Root().Find(args)
			if err != nil || target == cmd.Root() {
				return fmt.Errorf("unknown help topic or command %q", strings.Join(args, " "))
			}
			return target.Help()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Return help as JSON")
	cmd.Flags().BoolVar(&full, "full", false, "Include full setup, contracts, safety, limits, and command catalog")
	return cmd
}

func writeGuide(cmd *cobra.Command, doc guideDocument, jsonOut bool) error {
	if !jsonOut {
		_, err := fmt.Fprint(cmd.OutOrStdout(), doc.Content)
		return err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	env := output.Success("help", "", "", raw)
	return output.Write(cmd.OutOrStdout(), env, output.Options{Format: "json"})
}
