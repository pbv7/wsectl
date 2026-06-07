package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pbv7/wsectl/internal/auth"
	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

type state struct {
	version string
	commit  string
	date    string

	configPath string
	profile    string
	accountURL string
	format     string
	fields     string
	jq         string
	timeout    string
	rateLimit  string
	out        string
	schema     bool
	limit      int

	quiet           bool
	verbose         bool
	debug           bool
	failOnTruncated bool
}

func NewRoot(version, commit, date string) *cobra.Command {
	s := &state{version: version, commit: commit, date: date}
	root := &cobra.Command{
		Use:           "wsectl",
		Short:         "Unofficial command-line client for Worksection",
		Long:          "Unofficial command-line client for Worksection.\n\nThis is an unofficial tool and is not affiliated with Worksection.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), rootGuide().Content)
			return err
		},
	}
	root.SetVersionTemplate("wsectl {{.Version}}\n")
	root.Version = version
	addGlobalFlags(root, s)
	registerGlobalCompletions(root, s)
	helpCmd := newHelpCommand(s)
	root.SetHelpCommand(helpCmd)
	root.AddCommand(helpCmd)
	root.AddCommand(newCommandsCommand(s))
	root.AddCommand(newAuthCommand(s))
	root.AddCommand(newProfilesCommand(s))
	root.AddCommand(newAPICommand(s))
	root.AddCommand(newDocsCommand(s))
	root.AddCommand(newDoctorCommand(s))
	root.AddCommand(newCompletionCommand())
	root.AddCommand(newVersionCommand(s))
	root.AddCommand(newReadCommands(s)...)
	if err := applyCommandMetadata(root); err != nil {
		panic(err)
	}
	wrapCommandErrors(root, s)
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == root {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), rootGuide().Content)
			return
		}
		defaultHelp(cmd, args)
	})
	return root
}

func addGlobalFlags(cmd *cobra.Command, s *state) {
	f := cmd.PersistentFlags()
	f.StringVar(&s.profile, "profile", "", "Profile name")
	f.StringVar(&s.configPath, "config", "", "Config file path")
	f.StringVar(&s.accountURL, "account-url", "", "Worksection account URL")
	f.StringVar(&s.format, "output", "", "Output format: auto, json, yaml, table, ndjson, raw")
	f.Bool("json", false, "Shortcut for --output json")
	f.Bool("yaml", false, "Shortcut for --output yaml")
	f.Bool("table", false, "Shortcut for --output table")
	f.Bool("ndjson", false, "Shortcut for --output ndjson")
	f.Bool("raw", false, "Shortcut for --output raw")
	f.StringVar(&s.fields, "fields", "", "Comma-separated fields to keep")
	f.StringVar(&s.jq, "jq", "", "gojq expression to apply to JSON output")
	f.StringVar(&s.timeout, "timeout", "", "Request timeout")
	f.StringVar(&s.rateLimit, "rate-limit", "", "Client-side rate limit, for example 1/s")
	f.BoolVar(&s.quiet, "quiet", false, "Suppress non-data output")
	f.BoolVar(&s.verbose, "verbose", false, "Verbose output")
	f.BoolVar(&s.debug, "debug", false, "Debug output with secret redaction")
	f.StringVar(&s.out, "out", "", "Write output to file, or - for stdout")
	f.BoolVar(&s.schema, "schema", false, "Print the static action response contract without calling Worksection")
	f.IntVar(&s.limit, "limit", 0, "Client-side maximum number of array records to output")
	f.BoolVar(&s.failOnTruncated, "fail-on-truncated", false, "Exit 8 if metadata indicates possible truncation")
	cmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		if v, _ := cmd.Flags().GetBool("json"); v {
			s.format = "json"
		}
		if v, _ := cmd.Flags().GetBool("yaml"); v {
			s.format = "yaml"
		}
		if v, _ := cmd.Flags().GetBool("table"); v {
			s.format = "table"
		}
		if v, _ := cmd.Flags().GetBool("ndjson"); v {
			s.format = "ndjson"
		}
		if v, _ := cmd.Flags().GetBool("raw"); v {
			s.format = "raw"
		}
	}
}

func (s *state) loadConfig(ctx context.Context) (config.Config, error) {
	cfg, err := config.Load(ctx, s.configOverrides())
	if err == nil && s.format == "" {
		s.format = cfg.Defaults.Output
	}
	return cfg, err
}

func (s *state) configOverrides() config.Overrides {
	return config.Overrides{
		ConfigPath: s.configPath,
		Profile:    s.profile,
		AccountURL: s.accountURL,
		Output:     s.format,
		Timeout:    s.timeout,
		RateLimit:  s.rateLimit,
		Debug:      s.debug,
	}
}

func (s *state) client(ctx context.Context) (*worksection.Client, string, config.Profile, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, "", config.Profile{}, err
	}
	profileName, p, err := cfg.ActiveProfile()
	if err != nil {
		return nil, profileName, p, &worksection.Error{Code: worksection.CodeUsage, Message: err.Error()}
	}
	ref, err := auth.ParseRef(p.SecretRef)
	if err != nil {
		return nil, profileName, p, err
	}
	store, err := auth.StoreFor(ref)
	if err != nil {
		return nil, profileName, p, err
	}
	secret, err := store.Get(ctx, ref)
	if err != nil && ref.Scheme != "env" {
		if envStore, envErr := auth.StoreFor(auth.SecretRef{Scheme: "env", Name: ""}); envErr == nil {
			envRef := auth.SecretRef{Scheme: "env", Name: ""}
			envSecret, envGetErr := envStore.Get(ctx, envRef)
			if envGetErr == nil && envSecretUsable(firstNonEmpty(p.AuthType, "oauth2"), envSecret) {
				secret = envSecret
				store = envStore
				ref = envRef
				err = nil
			}
		}
	}
	if err != nil {
		return nil, profileName, p, &worksection.Error{Code: worksection.CodeAuth, Message: err.Error()}
	}
	if firstNonEmpty(p.AuthType, "oauth2") == "oauth2" && auth.NeedsRefresh(secret.AccessExpires, time.Now()) && secret.RefreshToken != "" {
		oauthClient := auth.HTTPClientWithTimeout(cfg.Timeout())
		refreshed, refreshErr := auth.Refresh(ctx, oauthClient, secret)
		if refreshErr != nil {
			return nil, profileName, p, &worksection.Error{Code: worksection.CodeAuth, Message: refreshErr.Error()}
		}
		secret = refreshed
		if err := store.Set(ctx, ref, secret); err != nil {
			return nil, profileName, p, &worksection.Error{Code: worksection.CodeAuth, Message: "refreshed OAuth token could not be persisted: " + err.Error()}
		}
	}
	accountURL := firstNonEmpty(secret.AccountURL, p.AccountURL)
	creds := worksection.Credentials{Mode: worksection.AuthMode(firstNonEmpty(p.AuthType, "oauth2")), AccountURL: accountURL}
	if creds.Mode == worksection.AuthAdmin {
		creds.AdminKey = firstNonEmpty(secret.AdminToken, os.Getenv("WSECTL_ADMIN_TOKEN"))
	} else {
		creds.Token = firstNonEmpty(secret.AccessToken, os.Getenv("WSECTL_ACCESS_TOKEN"))
	}
	limiter, err := worksection.NewLimiter(cfg.RateLimit())
	if err != nil {
		return nil, profileName, p, err
	}
	return worksection.NewClient(nil, creds, cfg.Timeout(), limiter), profileName, p, nil
}

func envSecretUsable(authType string, secret auth.SecretBundle) bool {
	if authType == "admin_token" {
		return secret.AdminToken != ""
	}
	return secret.AccessToken != ""
}

func (s *state) runAction(cmd *cobra.Command, action string, params map[string]string) error {
	return s.runActionWithOptions(cmd, action, params, false, nil)
}

func (s *state) runActionWithOptions(cmd *cobra.Command, action string, params map[string]string, allowUnknown bool, transform func(json.RawMessage) (json.RawMessage, []string, error)) error {
	if s.schema {
		return s.writeActionSchema(cmd, action)
	}
	if err := worksection.ValidateAction(action, params, allowUnknown); err != nil {
		return writeFailure(cmd.OutOrStderr(), s, action, "", err)
	}
	client, profileName, p, err := s.client(cmd.Context())
	if err != nil {
		return writeFailure(cmd.OutOrStderr(), s, action, profileName, err)
	}
	s.writeDiagnostic(cmd, "action=%s profile=%s account_url=%s", action, profileName, firstNonEmpty(p.AccountURL, "[unknown]"))
	raw, err := client.CallRaw(cmd.Context(), action, params)
	if err != nil {
		return writeFailure(cmd.OutOrStderr(), s, action, profileName, err)
	}
	if s.format == "raw" {
		if err := writeRawBytes(cmd.OutOrStdout(), s.out, raw); err != nil {
			return err
		}
		if s.debug {
			s.writeDiagnostic(cmd, "action=%s response_bytes=%d raw=true", action, len(raw))
		}
		{
			resp, parseErr := worksection.ParseResponse(raw)
			if parseErr == nil && resp.Status != "" && resp.Status != "ok" {
				return &worksection.Error{Code: worksection.CodeAPI, Message: "Worksection API returned an error response"}
			}
		}
		return nil
	}
	resp, err := worksection.ParseResponse(raw)
	if err != nil {
		return err
	}
	if resp.Status != "" && resp.Status != "ok" {
		return writeFailure(cmd.OutOrStderr(), s, action, profileName, &worksection.Error{Code: worksection.CodeAPI, Message: worksection.ResponseErrorMessage(resp), Details: map[string]any{"action": action}})
	}
	data := resp.OutputData(action)
	var extraWarnings []string
	if transform != nil {
		data, extraWarnings, err = transform(data)
		if err != nil {
			return err
		}
	}
	spec, _ := worksection.LookupAction(action)
	fullEnv := output.SuccessWithContract(action, profileName, p.AccountURL, data, spec.Response)
	limited := false
	data, limited, err = output.LimitData(data, s.limit, spec.Response)
	if err != nil {
		return err
	}
	env := output.SuccessWithContract(action, profileName, p.AccountURL, data, spec.Response)
	if fullEnv.Meta.Truncated && !env.Meta.Truncated {
		env.Meta.Truncated = true
		env.Meta.Warnings = append(env.Meta.Warnings, fullEnv.Meta.Warnings...)
	}
	if limited {
		env.Meta.Warnings = append(env.Meta.Warnings, "Client-side --limit was applied; output contains only the first requested records.")
	}
	env.Meta.Warnings = append(env.Meta.Warnings, extraWarnings...)
	opts := s.outputOptions()
	opts.KnownFields = spec.KnownFieldNames()
	opts.Contract = spec.Response
	if s.debug {
		s.writeDiagnostic(cmd, "action=%s response_bytes=%d raw=false", action, len(raw))
	}
	return output.Write(cmd.OutOrStdout(), env, opts)
}

func (s *state) writeActionSchema(cmd *cobra.Command, action string) error {
	spec, ok := worksection.Schema(action)
	if !ok {
		return worksection.UsageError("unknown action %q", action)
	}
	raw, _ := json.Marshal(spec)
	env := output.SuccessWithContract("schema "+action, "", "", raw, spec.Response)
	opts := s.outputOptions()
	opts.Format = "json"
	return output.Write(cmd.OutOrStdout(), env, opts)
}

func (s *state) outputOptions() output.Options {
	fields := []string{}
	if s.fields != "" {
		for _, f := range strings.Split(s.fields, ",") {
			if trimmed := strings.TrimSpace(f); trimmed != "" {
				fields = append(fields, trimmed)
			}
		}
	}
	opts := output.Options{Format: s.format, Fields: fields, JQ: s.jq, Out: s.out, FailOnTruncated: s.failOnTruncated}
	return opts
}

func writeFailure(w io.Writer, s *state, action, profile string, err error) error {
	if s.machineFormat() {
		opts := s.outputOptions()
		if opts.Format == "" || opts.Format == "auto" {
			opts.Format = s.effectiveFormat()
		}
		if writeErr := output.Write(w, output.Failure(action, profile, err), opts); writeErr != nil {
			return writeErr
		}
		return markRendered(err)
	}
	return err
}

func wrapCommandErrors(root *cobra.Command, s *state) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.RunE != nil {
			original := cmd.RunE
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				err := original(cmd, args)
				if err == nil || isRendered(err) {
					return err
				}
				return writeFailure(cmd.OutOrStderr(), s, cmd.CommandPath(), "", err)
			}
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
}

type renderedError struct{ err error }

func (e renderedError) Error() string { return e.err.Error() }
func (e renderedError) Unwrap() error { return e.err }
func (e renderedError) SuppressPrint() bool {
	return true
}
func (e renderedError) ExitCode() int {
	if ec, ok := e.err.(interface{ ExitCode() int }); ok {
		return ec.ExitCode()
	}
	return 1
}

func markRendered(err error) error {
	if err == nil || isRendered(err) {
		return err
	}
	return renderedError{err: err}
}

func isRendered(err error) bool {
	_, ok := err.(interface{ SuppressPrint() bool })
	return ok
}

func (s *state) effectiveFormat() string {
	if s.format != "" {
		return s.format
	}
	return os.Getenv("WSECTL_OUTPUT")
}

func (s *state) machineFormat() bool {
	switch s.effectiveFormat() {
	case "json", "yaml", "ndjson":
		return true
	default:
		return false
	}
}

func (s *state) humanDiagnosticFormat() bool {
	switch s.effectiveFormat() {
	case "json", "yaml", "ndjson", "raw":
		return false
	default:
		return true
	}
}

func (s *state) writeDiagnostic(cmd *cobra.Command, format string, args ...any) {
	if s.quiet || !s.humanDiagnosticFormat() || (!s.verbose && !s.debug) {
		return
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wsectl: "+format+"\n", args...)
}

func writeRawBytes(w io.Writer, outPath string, raw []byte) error {
	if outPath == "" || outPath == "-" {
		_, err := w.Write(raw)
		return err
	}
	return os.WriteFile(outPath, raw, 0o600)
}

func newCommandsCommand(s *state) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List public commands",
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows := collectCommands(cmd.Root())
			raw, _ := json.Marshal(rows)
			env := output.Success("commands", "", "", raw)
			return output.Write(cmd.OutOrStdout(), env, discoveryOutputOptions(s))
		},
	}
}

func paramsFromPairs(pairs []string) (map[string]string, error) {
	params := map[string]string{}
	for _, pair := range pairs {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			return nil, worksection.UsageError("--param must be key=value")
		}
		if _, exists := params[k]; exists {
			return nil, worksection.UsageError("--param %q was provided more than once", k)
		}
		params[k] = v
	}
	return params, nil
}

func exactArgsUnlessSchema(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if schemaRequested(cmd) {
			return nil
		}
		return cobra.ExactArgs(n)(cmd, args)
	}
}

func schemaRequested(cmd *cobra.Command) bool {
	flag := cmd.Flag("schema")
	return flag != nil && flag.Value.String() == "true"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
