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
	"github.com/spf13/pflag"
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

	history         historyRun
	configLoaded    bool
	cachedConfig    config.Config
	cachedConfigErr error
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
	root.AddCommand(newHistoryCommand(s))
	root.AddCommand(newReadCommands(s)...)
	if err := applyCommandMetadata(root); err != nil {
		panic(err)
	}
	wrapCommandErrors(root, s)
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == root {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), rootGuide().Content)
			s.recordHelpHistory(cmd, args)
			return
		}
		defaultHelp(cmd, args)
		s.recordHelpHistory(cmd, args)
	})
	return root
}

// outputShortcuts maps each boolean output shortcut to the format it selects,
// in the precedence order applied by PersistentPreRun (later entries win).
var outputShortcuts = []struct{ flag, format string }{
	{"json", "json"},
	{"yaml", "yaml"},
	{"table", "table"},
	{"ndjson", "ndjson"},
	{"raw", "raw"},
}

func addGlobalFlags(cmd *cobra.Command, s *state) {
	registerGlobalFlags(cmd.PersistentFlags(), s)
	cmd.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		s.format = resolveOutputFlag(cmd.Flags(), s.format)
	}
}

// registerGlobalFlags binds the global persistent flags onto f. It is shared by
// the live command tree and ResolveOutputAndConfig so both parse exactly the
// same flag set.
func registerGlobalFlags(f *pflag.FlagSet, s *state) {
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
}

// resolveOutputFlag applies the boolean output shortcuts over a base --output
// value, in fixed order so the precedence is by flag, not argument position.
func resolveOutputFlag(f *pflag.FlagSet, base string) string {
	out := base
	for _, sc := range outputShortcuts {
		if v, _ := f.GetBool(sc.flag); v {
			out = sc.format
		}
	}
	return out
}

// ResolveOutputAndConfig parses the output and config flags out of args exactly
// as the CLI would and returns the selected output format (--output plus the
// boolean shortcuts) and --config path. It is used to render a top-level error
// before the command body has parsed anything.
//
// It resolves the target command with the real command tree and parses against
// that command's complete flag set — global persistent flags plus the
// subcommand's local flags. So pflag owns every parsing rule (the -- terminator,
// boolean value forms like --json=false, value consumption for both global and
// subcommand value flags such as --profile --json or --extra --json, and
// last-occurrence wins), and the error path renders in the same format a real
// invocation would. Unknown flags (including the one a usage error is about) are
// ignored via the parse allowlist.
func ResolveOutputAndConfig(args []string) (output, config string) {
	root := NewRoot("", "", "")
	target, remaining, err := root.Find(args)
	if err != nil || target == nil {
		target, remaining = root, args
	}
	fs := pflag.NewFlagSet("resolve", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetOutput(io.Discard)
	fs.AddFlagSet(root.PersistentFlags()) // global flags (--output, --config, shortcuts)
	fs.AddFlagSet(target.LocalFlags())    // subcommand flags so their values are consumed
	_ = fs.Parse(remaining)
	base, _ := fs.GetString("output")
	config, _ = fs.GetString("config")
	return resolveOutputFlag(fs, base), config
}

func (s *state) loadConfig(ctx context.Context) (config.Config, error) {
	if s.configLoaded {
		return s.cachedConfig, s.cachedConfigErr
	}
	cfg, err := config.Load(ctx, s.configOverrides())
	if err != nil && config.IsValidationError(err) {
		// A semantic validation failure is the user's input to fix: exit 2
		// with a "usage" envelope code per the documented contract. The
		// parsed config still carries the user's output default; honor it
		// for error rendering when the default itself is valid.
		if s.format == "" && config.ValidOutput(cfg.Defaults.Output) {
			s.format = cfg.Defaults.Output
		}
		err = worksection.UsageError("%s", err.Error())
	}
	if err == nil && s.format == "" {
		s.format = cfg.Defaults.Output
	}
	s.cachedConfig = cfg
	s.cachedConfigErr = err
	s.configLoaded = true
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

type clientResult struct {
	client          *worksection.Client
	profileName     string
	profile         config.Profile
	usedEnvFallback bool
}

type profileSecret struct {
	secret          auth.SecretBundle
	store           auth.SecretStore
	ref             auth.SecretRef
	usedEnvFallback bool
}

func (s *state) client(ctx context.Context) (clientResult, error) {
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return clientResult{}, err
	}
	profileName, p, err := cfg.ActiveProfile()
	if err != nil {
		return clientResult{profileName: profileName, profile: p}, &worksection.Error{Code: worksection.CodeUsage, Message: err.Error()}
	}
	secretInfo, err := loadProfileSecret(ctx, p)
	if err != nil {
		return clientResult{profileName: profileName, profile: p}, err
	}
	secret, err := refreshProfileSecret(ctx, cfg, p, secretInfo)
	if err != nil {
		return clientResult{profileName: profileName, profile: p}, err
	}
	limiter, err := worksection.NewLimiter(cfg.RateLimit())
	if err != nil {
		return clientResult{profileName: profileName, profile: p}, err
	}
	// Resolve the effective account URL once, then write it back onto the
	// profile so the client, diagnostics, history, and output envelopes all
	// agree on the host actually used.
	p.AccountURL = resolveAccountURL(s.accountURL, os.Getenv("WSECTL_ACCOUNT_URL"), secret.AccountURL, p.AccountURL)
	return clientResult{
		client:          worksection.NewClient(nil, profileCredentials(p, secret), cfg.Timeout(), limiter),
		profileName:     profileName,
		profile:         p,
		usedEnvFallback: secretInfo.usedEnvFallback,
	}, nil
}

// resolveAccountURL applies the account-URL precedence ladder:
//
//	--account-url flag  >  WSECTL_ACCOUNT_URL env  >  credential URL  >  configured profile URL
//
// The flag and env are explicit overrides and outrank everything. With no
// override, the credential's own URL wins: an OAuth exchange may return an
// account_url that differs from the configured profile (Worksection treats
// the response URL as the authorized account for subsequent requests), and
// that stored value must not be discarded by the ordinary configured value.
// The configured profile URL is the final fallback.
//
// configuredURL is passed last and is only consulted when no override and no
// credential URL exist; passing the override-merged profile value here is
// safe because the explicit flag/env arguments already take precedence.
func resolveAccountURL(flagOverride, envOverride, credentialURL, configuredURL string) string {
	return firstNonEmpty(flagOverride, envOverride, credentialURL, configuredURL)
}

func loadProfileSecret(ctx context.Context, p config.Profile) (profileSecret, error) {
	ref, err := auth.ParseRef(p.SecretRef)
	if err != nil {
		return profileSecret{}, err
	}
	store, err := auth.StoreFor(ref)
	if err != nil {
		return profileSecret{}, err
	}
	secret, err := store.Get(ctx, ref)
	if err != nil && ref.Scheme != "env" {
		envSecret, ok := envFallbackSecret(ctx, firstNonEmpty(p.AuthType, "oauth2"))
		if ok {
			return profileSecret{
				secret:          envSecret,
				store:           auth.EnvStore{},
				ref:             auth.SecretRef{Scheme: "env", Name: ""},
				usedEnvFallback: true,
			}, nil
		}
	}
	if err != nil {
		return profileSecret{}, &worksection.Error{Code: worksection.CodeAuth, Message: err.Error()}
	}
	return profileSecret{secret: secret, store: store, ref: ref}, nil
}

func envFallbackSecret(ctx context.Context, authType string) (auth.SecretBundle, bool) {
	envRef := auth.SecretRef{Scheme: "env", Name: ""}
	envStore, err := auth.StoreFor(envRef)
	if err != nil {
		return auth.SecretBundle{}, false
	}
	envSecret, err := envStore.Get(ctx, envRef)
	if err != nil || !envSecretUsable(authType, envSecret) {
		return auth.SecretBundle{}, false
	}
	return envSecret, true
}

func refreshProfileSecret(ctx context.Context, cfg config.Config, p config.Profile, secretInfo profileSecret) (auth.SecretBundle, error) {
	secret := secretInfo.secret
	if !shouldRefreshSecret(p, secret) {
		return secret, nil
	}
	refreshed, err := auth.Refresh(ctx, auth.HTTPClientWithTimeout(cfg.Timeout()), secret)
	if err != nil {
		return secret, &worksection.Error{Code: worksection.CodeAuth, Message: err.Error()}
	}
	if err := secretInfo.store.Set(ctx, secretInfo.ref, refreshed); err != nil {
		return secret, &worksection.Error{Code: worksection.CodeAuth, Message: "refreshed OAuth token could not be persisted: " + err.Error()}
	}
	return refreshed, nil
}

func shouldRefreshSecret(p config.Profile, secret auth.SecretBundle) bool {
	return firstNonEmpty(p.AuthType, "oauth2") == "oauth2" &&
		auth.NeedsRefresh(secret.AccessExpires, time.Now()) &&
		secret.RefreshToken != ""
}

func profileCredentials(p config.Profile, secret auth.SecretBundle) worksection.Credentials {
	// p.AccountURL is already the effective URL resolved by resolveAccountURL
	// in client(); profileCredentials only assembles the credential bundle.
	creds := worksection.Credentials{Mode: worksection.AuthMode(firstNonEmpty(p.AuthType, "oauth2")), AccountURL: p.AccountURL}
	if creds.Mode == worksection.AuthAdmin {
		creds.AdminKey = firstNonEmpty(secret.AdminToken, os.Getenv("WSECTL_ADMIN_TOKEN"))
	} else {
		creds.Token = firstNonEmpty(secret.AccessToken, os.Getenv("WSECTL_ACCESS_TOKEN"))
	}
	return creds
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
	s.noteHistoryAction(action, params)
	if s.schema {
		return s.writeActionSchema(cmd, action)
	}
	if err := worksection.ValidateAction(action, params, allowUnknown); err != nil {
		return writeFailure(cmd.ErrOrStderr(), s, action, "", err)
	}
	clientInfo, err := s.client(cmd.Context())
	if err != nil {
		return writeFailure(cmd.ErrOrStderr(), s, action, clientInfo.profileName, err)
	}
	s.noteHistoryClient(clientInfo)
	raw, err := s.callActionRaw(cmd, clientInfo, action, params)
	if err != nil {
		return writeFailure(cmd.ErrOrStderr(), s, action, clientInfo.profileName, err)
	}
	if s.format == "raw" {
		return s.writeRawActionResult(cmd, action, raw)
	}
	return s.writeParsedActionResult(cmd, action, clientInfo, raw, transform)
}

func (s *state) callActionRaw(cmd *cobra.Command, clientInfo clientResult, action string, params map[string]string) ([]byte, error) {
	s.writeEnvFallbackDiagnostic(cmd, clientInfo)
	s.writeDiagnostic(cmd, "action=%s profile=%s account_url=%s", action, clientInfo.profileName, firstNonEmpty(clientInfo.profile.AccountURL, "[unknown]"))
	return clientInfo.client.CallRaw(cmd.Context(), action, params)
}

func (s *state) writeRawActionResult(cmd *cobra.Command, action string, raw []byte) error {
	s.warnRawIgnoredFlags(cmd)
	if err := writeRawBytes(cmd.OutOrStdout(), s.out, raw); err != nil {
		return err
	}
	s.noteRawHistoryResult(action, raw)
	if s.debug {
		s.writeDiagnostic(cmd, "action=%s response_bytes=%d raw=true", action, len(raw))
	}
	if rawWorksectionError(raw) {
		return &worksection.Error{Code: worksection.CodeAPI, Message: "Worksection API returned an error response"}
	}
	return nil
}

// warnRawIgnoredFlags emits a single stderr warning per ignored transform
// flag. --raw streams the upstream HTTP body verbatim, so --fields/--limit/--jq
// have no effect; silently dropping them is a foot-gun, hard-erroring would
// break users who set these as default flags.
func (s *state) warnRawIgnoredFlags(cmd *cobra.Command) {
	if s.quiet {
		return
	}
	w := cmd.ErrOrStderr()
	if s.fields != "" {
		_, _ = fmt.Fprintln(w, "warning: --fields is ignored with --raw (raw mode emits the upstream response verbatim)")
	}
	if s.limit > 0 {
		_, _ = fmt.Fprintln(w, "warning: --limit is ignored with --raw (raw mode emits the upstream response verbatim)")
	}
	if s.jq != "" {
		_, _ = fmt.Fprintln(w, "warning: --jq is ignored with --raw (raw mode emits the upstream response verbatim)")
	}
	if s.failOnTruncated {
		_, _ = fmt.Fprintln(w, "warning: --fail-on-truncated is ignored with --raw (raw mode does not parse the response to detect truncation)")
	}
}

func (s *state) noteRawHistoryResult(action string, raw []byte) {
	resp, err := worksection.ParseResponse(raw)
	if err != nil || resp.Status != "ok" {
		return
	}
	spec, _ := worksection.LookupAction(action)
	env := output.SuccessWithContract(action, s.history.profile, s.history.accountURL, resp.OutputData(action), spec.Response)
	s.noteHistoryEnvelope(env)
}

func rawWorksectionError(raw []byte) bool {
	resp, err := worksection.ParseResponse(raw)
	return err == nil && resp.Status != "" && resp.Status != "ok"
}

func (s *state) writeParsedActionResult(cmd *cobra.Command, action string, clientInfo clientResult, raw []byte, transform func(json.RawMessage) (json.RawMessage, []string, error)) error {
	resp, err := worksection.ParseResponse(raw)
	if err != nil {
		return err
	}
	if resp.Status != "" && resp.Status != "ok" {
		return writeFailure(cmd.ErrOrStderr(), s, action, clientInfo.profileName, worksectionAPIError(action, resp))
	}
	env, opts, err := s.actionOutput(action, clientInfo, resp, transform)
	if err != nil {
		return err
	}
	if s.debug {
		s.writeDiagnostic(cmd, "action=%s response_bytes=%d raw=false", action, len(raw))
	}
	s.noteHistoryEnvelope(env)
	return output.Write(cmd.OutOrStdout(), env, opts)
}

func worksectionAPIError(action string, resp *worksection.Response) error {
	return &worksection.Error{
		Code:    worksection.CodeAPI,
		Message: worksection.ResponseErrorMessage(resp),
		Details: map[string]any{"action": action},
	}
}

func (s *state) actionOutput(action string, clientInfo clientResult, resp *worksection.Response, transform func(json.RawMessage) (json.RawMessage, []string, error)) (output.Envelope, output.Options, error) {
	spec, _ := worksection.LookupAction(action)
	data, extraWarnings, err := transformedActionData(resp.OutputData(action), transform)
	if err != nil {
		return output.Envelope{}, output.Options{}, err
	}
	fullEnv := output.SuccessWithContract(action, clientInfo.profileName, clientInfo.profile.AccountURL, data, spec.Response)
	data, limited, err := output.LimitData(data, s.limit, spec.Response)
	if err != nil {
		return output.Envelope{}, output.Options{}, err
	}
	env := output.SuccessWithContract(action, clientInfo.profileName, clientInfo.profile.AccountURL, data, spec.Response)
	copyTruncationWarnings(&env, fullEnv)
	if limited {
		env.Meta.Warnings = append(env.Meta.Warnings, "Client-side --limit was applied; output contains only the first requested records.")
	}
	env.Meta.Warnings = append(env.Meta.Warnings, extraWarnings...)
	opts := s.outputOptions()
	opts.KnownFields = spec.KnownFieldNames()
	opts.TableColumns = spec.TableColumns
	opts.Contract = spec.Response
	return env, opts, nil
}

func transformedActionData(data json.RawMessage, transform func(json.RawMessage) (json.RawMessage, []string, error)) (json.RawMessage, []string, error) {
	if transform == nil {
		return data, nil, nil
	}
	return transform(data)
}

func copyTruncationWarnings(env *output.Envelope, fullEnv output.Envelope) {
	if fullEnv.Meta.Truncated && !env.Meta.Truncated {
		env.Meta.Truncated = true
		env.Meta.Warnings = append(env.Meta.Warnings, fullEnv.Meta.Warnings...)
	}
}

func (s *state) writeActionSchema(cmd *cobra.Command, action string) error {
	spec, ok := worksection.Schema(action)
	if !ok {
		return worksection.UsageError("unknown action %q", action)
	}
	raw, _ := json.Marshal(spec)
	env := output.SuccessWithContract("schema "+action, "", "", raw, spec.Response)
	s.noteHistoryEnvelope(env)
	opts := s.outputOptions()
	if opts.Format == "" || opts.Format == "auto" {
		opts.Format = "json"
	}
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
				markBodyEntered(cmd)
				started := time.Now()
				s.beginHistoryRun()
				err := original(cmd, args)
				duration := time.Since(started)
				s.recordCommandHistory(cmd, args, started, duration, err)
				s.endHistoryRun()
				if err == nil || isRendered(err) {
					return err
				}
				return writeFailure(cmd.ErrOrStderr(), s, cmd.CommandPath(), "", err)
			}
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
}

// markBodyEntered records on the root that a command body (RunE) began
// executing, so the top-level error classifier can distinguish a pre-RunE
// cobra usage/parse error from an in-body failure.
func markBodyEntered(cmd *cobra.Command) {
	root := cmd.Root()
	if root.Annotations == nil {
		root.Annotations = map[string]string{}
	}
	root.Annotations[annotationBodyEntered] = "1"
}

// EnteredBody reports whether a command body executed during the most recent
// Execute on root. When false, an error returned by Execute came from cobra's
// flag/argument parsing or command resolution and should be classified as a
// usage error.
func EnteredBody(root *cobra.Command) bool {
	return root.Annotations[annotationBodyEntered] == "1"
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

func (s *state) writeEnvFallbackDiagnostic(cmd *cobra.Command, clientInfo clientResult) {
	if !clientInfo.usedEnvFallback {
		return
	}
	s.writeDiagnostic(cmd, "profile=%s secret_ref=%s unavailable; using environment credentials", clientInfo.profileName, firstNonEmpty(clientInfo.profile.SecretRef, "[none]"))
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
