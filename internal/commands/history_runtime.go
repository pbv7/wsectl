package commands

import (
	"context"
	"errors"
	"time"

	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/history"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type historyRun struct {
	active     bool
	action     string
	params     map[string]string
	profile    string
	accountURL string
	authType   string
	count      int
	truncated  bool
	warnings   []string
}

func (s *state) beginHistoryRun() {
	s.history = historyRun{active: true}
}

func (s *state) endHistoryRun() {
	s.history.active = false
}

func (s *state) noteHistoryAction(action string, params map[string]string) {
	s.history.action = action
	s.history.params = params
}

func (s *state) noteHistoryClient(clientInfo clientResult) {
	s.history.profile = clientInfo.profileName
	s.history.accountURL = clientInfo.profile.AccountURL
	s.history.authType = firstNonEmpty(clientInfo.profile.AuthType, "oauth2")
}

func (s *state) noteHistoryEnvelope(env output.Envelope) {
	s.history.action = firstNonEmpty(s.history.action, env.Meta.Action)
	s.history.profile = firstNonEmpty(s.history.profile, env.Meta.Profile)
	s.history.accountURL = firstNonEmpty(s.history.accountURL, env.Meta.AccountURL)
	s.history.count = env.Meta.Count
	s.history.truncated = env.Meta.Truncated
	s.history.warnings = env.Meta.Warnings
}

func (s *state) recordCommandHistory(cmd *cobra.Command, args []string, started time.Time, duration time.Duration, err error) {
	if shouldSkipHistory(cmd) {
		return
	}
	options, cfg, cfgErr := s.historyRecordOptions(cmd.Context())
	if cfgErr != nil || !options.Enabled {
		return
	}
	event := s.historyEvent(cmd, args, started, duration, err, options, cfg)
	if recordErr := history.Record(cmd.Context(), options, event); recordErr != nil {
		s.writeDiagnostic(cmd, "history write failed: %s", recordErr)
	}
}

func (s *state) recordHelpHistory(cmd *cobra.Command, args []string) {
	if s.history.active {
		return
	}
	started := time.Now()
	s.beginHistoryRun()
	s.recordCommandHistory(cmd, args, started, 0, nil)
	s.endHistoryRun()
}

func (s *state) historyRecordOptions(ctx context.Context) (history.Options, config.Config, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return history.Options{}, cfg, err
	}
	options := historyOptions(cfg)
	return options, cfg, nil
}

func historyOptions(cfg config.Config) history.Options {
	path := cfg.History.Path
	if path == "" {
		path = config.DefaultHistoryPath()
	}
	includeParams := cfg.History.IncludeParams
	// Preserve the built-in default even when a partial config leaves the field empty.
	if includeParams == "" {
		includeParams = "all"
	}
	return history.Options{
		Enabled:       cfg.History.Enabled,
		Path:          path,
		IncludeParams: includeParams,
	}
}

func (s *state) historyEvent(cmd *cobra.Command, args []string, started time.Time, duration time.Duration, err error, options history.Options, cfg config.Config) history.Event {
	status := "ok"
	if err != nil {
		status = "error"
	}
	profile, accountURL, authType := s.historyProfileContext(cfg)
	return history.Event{
		Timestamp:      started.UTC().Format(time.RFC3339Nano),
		Command:        cmd.CommandPath(),
		NormalizedArgs: normalizedHistoryArgs(cmd, args, options.IncludeParams),
		Action:         s.history.action,
		Profile:        profile,
		AccountURL:     accountURL,
		AuthType:       authType,
		Output:         firstNonEmpty(s.effectiveFormat(), "auto"),
		Params:         history.Params(s.history.params, options.IncludeParams),
		Status:         status,
		ExitCode:       commandExitCode(err),
		ErrorCode:      commandErrorCode(err),
		DurationMS:     duration.Milliseconds(),
		Count:          s.history.count,
		Truncated:      s.history.truncated,
		Warnings:       s.history.warnings,
	}
}

func (s *state) historyProfileContext(cfg config.Config) (string, string, string) {
	profile := s.history.profile
	accountURL := s.history.accountURL
	authType := s.history.authType
	if profile != "" && accountURL != "" && authType != "" {
		return profile, accountURL, authType
	}
	name, p, err := cfg.ActiveProfile()
	if err != nil {
		return profile, accountURL, authType
	}
	return firstNonEmpty(profile, name), firstNonEmpty(accountURL, p.AccountURL), firstNonEmpty(authType, p.AuthType, "oauth2")
}

func normalizedHistoryArgs(cmd *cobra.Command, args []string, includeParams string) []string {
	out := []string{}
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "help" {
			return
		}
		if values, ok := flag.Value.(pflag.SliceValue); ok {
			for _, value := range values.GetSlice() {
				out = append(out, "--"+flag.Name+"="+value)
			}
			return
		}
		out = append(out, "--"+flag.Name)
		if flag.Value.Type() != "bool" {
			out = append(out, flag.Value.String())
		}
	})
	out = append(out, args...)
	return history.Args(out, includeParams)
}

func shouldSkipHistory(cmd *cobra.Command) bool {
	if cmd.Hidden {
		return true
	}
	if cmd == cmd.Root() {
		return true
	}
	if helpRequested(cmd) {
		return true
	}
	for current := cmd; current != nil; current = current.Parent() {
		if current.Annotations[annotationHistorySkip] == "true" {
			return true
		}
	}
	return false
}

func helpRequested(cmd *cobra.Command) bool {
	help, err := cmd.Flags().GetBool("help")
	return err == nil && help
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) {
		return exitCoder.ExitCode()
	}
	return 1
}

func commandErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var wsErr *worksection.Error
	if errors.As(err, &wsErr) {
		return string(wsErr.Code)
	}
	return "general"
}

func (s *state) historyPathInfo(ctx context.Context) (history.Options, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return history.Options{}, err
	}
	return historyOptions(cfg), nil
}
