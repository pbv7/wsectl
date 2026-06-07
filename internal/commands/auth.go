package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pbv7/wsectl/internal/auth"
	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

type authLoginOptions struct {
	noBrowser         bool
	manualCode        bool
	clientSecretStdin bool
	adminTokenStdin   bool
	callbackHost      string
	callbackCert      string
	callbackKey       string
	loginTimeout      string
	scopes            []string
	code              string
	callbackPort      int
}

type authLoginTarget struct {
	cfg         config.Config
	profileName string
	profile     config.Profile
	authType    string
	ref         auth.SecretRef
	store       auth.SecretStore
	secret      auth.SecretBundle
}

type oauthLoginResult struct {
	secret auth.SecretBundle
	store  bool
}

func newAuthCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authenticate with Worksection"}
	cmd.AddCommand(newAuthLoginCommand(s))
	cmd.AddCommand(newAuthStatusCommand(s))
	cmd.AddCommand(newAuthRefreshCommand(s))
	cmd.AddCommand(newAuthLogoutCommand(s))
	return cmd
}

func newAuthLoginCommand(s *state) *cobra.Command {
	opts := &authLoginOptions{
		callbackHost: "localhost",
		callbackPort: 33443,
		loginTimeout: "10m",
	}
	login := &cobra.Command{
		Use:   "login",
		Short: "Start OAuth login or store manually supplied credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthLogin(cmd, s, opts)
		},
	}
	login.Flags().String("client-id", "", "OAuth client ID")
	login.Flags().String("client-secret", "", "OAuth client secret")
	login.Flags().BoolVar(&opts.clientSecretStdin, "client-secret-stdin", false, "Read OAuth client secret from one line on stdin")
	login.Flags().String("access-token", "", "Dangerous: directly store an access token")
	login.Flags().String("refresh-token", "", "Dangerous: directly store a refresh token")
	login.Flags().String("admin-token", "", "Dangerous: directly store an admin API key")
	login.Flags().BoolVar(&opts.adminTokenStdin, "admin-token-stdin", false, "Read admin API key from one line on stdin")
	login.Flags().StringVar(&opts.code, "code", "", "OAuth authorization code to exchange")
	login.Flags().StringVar(&opts.callbackHost, "callback-host", opts.callbackHost, "OAuth callback host")
	login.Flags().IntVar(&opts.callbackPort, "callback-port", opts.callbackPort, "OAuth callback port")
	login.Flags().StringVar(&opts.callbackCert, "callback-cert", "", "OAuth callback TLS certificate")
	login.Flags().StringVar(&opts.callbackKey, "callback-key", "", "OAuth callback TLS key")
	login.Flags().StringVar(&opts.loginTimeout, "login-timeout", opts.loginTimeout, "Maximum time to wait for OAuth browser callback")
	login.Flags().StringArrayVar(&opts.scopes, "scope", nil, "OAuth scope to request; repeat for multiple scopes")
	login.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "Do not open browser")
	login.Flags().BoolVar(&opts.manualCode, "manual-code", false, "Use manual code flow")
	_ = login.MarkFlagFilename("callback-cert", "crt", "pem")
	_ = login.MarkFlagFilename("callback-key", "key", "pem")
	return login
}

func newAuthStatusCommand(s *state) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show auth status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthStatus(cmd, s)
		},
	}
}

func newAuthRefreshCommand(s *state) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh OAuth token when stored refresh token is available",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthRefresh(cmd, s)
		},
	}
}

func newAuthLogoutCommand(s *state) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete stored credentials for the active profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAuthLogout(cmd, s)
		},
	}
}

func runAuthLogin(cmd *cobra.Command, s *state, opts *authLoginOptions) error {
	target, err := prepareAuthLogin(cmd, s, opts)
	if err != nil {
		return err
	}
	if target.authType == "admin_token" {
		return runAdminTokenLogin(cmd, s, target)
	}
	result, err := completeOAuthLogin(cmd, s, opts, target)
	if err != nil || !result.store {
		return err
	}
	return writeStoredLogin(cmd, s, target, result.secret)
}

func prepareAuthLogin(cmd *cobra.Command, s *state, opts *authLoginOptions) (authLoginTarget, error) {
	cfg, err := s.loadConfig(cmd.Context())
	if err != nil {
		return authLoginTarget{}, err
	}
	name, p, err := cfg.ActiveProfile()
	if err != nil {
		return authLoginTarget{}, err
	}
	authType := firstNonEmpty(p.AuthType, "oauth2")
	if err := validateAuthLoginFlags(cmd, authType, opts.clientSecretStdin, opts.adminTokenStdin, opts.manualCode); err != nil {
		return authLoginTarget{}, err
	}
	ref, err := auth.ParseRef(p.SecretRef)
	if err != nil {
		return authLoginTarget{}, err
	}
	store, err := auth.StoreFor(ref)
	if err != nil {
		return authLoginTarget{}, err
	}
	if err := auth.CheckWritable(cmd.Context(), store, ref); err != nil {
		return authLoginTarget{}, &worksection.Error{Code: worksection.CodeAuth, Message: "secret store is not writable: " + err.Error()}
	}
	secret, err := authLoginSecret(cmd, p.AccountURL, authType, opts.clientSecretStdin, opts.adminTokenStdin)
	if err != nil {
		return authLoginTarget{}, err
	}
	return authLoginTarget{cfg: cfg, profileName: name, profile: p, authType: authType, ref: ref, store: store, secret: secret}, nil
}

func runAdminTokenLogin(cmd *cobra.Command, s *state, target authLoginTarget) error {
	if target.secret.AdminToken == "" {
		return worksection.UsageError("admin token is required for admin-token login; pass --admin-token, --admin-token-stdin, or set WSECTL_ADMIN_TOKEN")
	}
	return writeStoredLogin(cmd, s, target, target.secret)
}

func completeOAuthLogin(cmd *cobra.Command, s *state, opts *authLoginOptions, target authLoginTarget) (oauthLoginResult, error) {
	secret := target.secret
	redirect := fmt.Sprintf("https://%s:%d/callback", opts.callbackHost, opts.callbackPort)
	if opts.code != "" {
		exchanged, err := exchangeLoginCode(cmd, target, secret, opts.code, redirect)
		return oauthLoginResult{secret: exchanged, store: err == nil}, err
	}
	if hasLoginToken(secret) {
		return oauthLoginResult{secret: secret, store: true}, nil
	}
	if err := requireOAuthClientCredentials(secret); err != nil {
		return oauthLoginResult{}, err
	}
	state, err := auth.RandomState()
	if err != nil {
		return oauthLoginResult{}, err
	}
	if opts.manualCode {
		return oauthLoginResult{}, writeManualOAuthInstructions(cmd, opts, secret, redirect, state)
	}
	exchanged, err := runBrowserOAuthLogin(cmd, s, opts, target, secret, state)
	return oauthLoginResult{secret: exchanged, store: err == nil}, err
}

func exchangeLoginCode(cmd *cobra.Command, target authLoginTarget, secret auth.SecretBundle, code, redirect string) (auth.SecretBundle, error) {
	exchanged, err := auth.ExchangeCode(cmd.Context(), oauthHTTPClient(target.cfg.Timeout()), secret.ClientID, secret.ClientSecret, code, redirect)
	if err != nil {
		return auth.SecretBundle{}, err
	}
	exchanged.AccountURL = firstNonEmpty(exchanged.AccountURL, target.profile.AccountURL)
	return exchanged, nil
}

func hasLoginToken(secret auth.SecretBundle) bool {
	return secret.AccessToken != "" || secret.AdminToken != ""
}

func requireOAuthClientCredentials(secret auth.SecretBundle) error {
	if secret.ClientID != "" && secret.ClientSecret != "" {
		return nil
	}
	return worksection.UsageError("client ID and client secret are required for OAuth login; pass --client-id and set WSECTL_CLIENT_SECRET or use --client-secret-stdin")
}

func writeManualOAuthInstructions(cmd *cobra.Command, opts *authLoginOptions, secret auth.SecretBundle, redirect, state string) error {
	url := auth.AuthorizationURL(secret.ClientID, redirect, state, selectedOAuthScopes(opts.scopes))
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Open this authorization URL and complete OAuth manually:\n%s\n\nThen set WSECTL_CLIENT_SECRET or pipe it with --client-secret-stdin, and run:\nwsectl auth login --code CODE --client-id %q --callback-host %q --callback-port %d\n", url, secret.ClientID, opts.callbackHost, opts.callbackPort)
	return err
}

func runBrowserOAuthLogin(cmd *cobra.Command, s *state, opts *authLoginOptions, target authLoginTarget, secret auth.SecretBundle, state string) (auth.SecretBundle, error) {
	callback, err := auth.StartOAuthCallback(auth.CallbackOptions{
		Host:     opts.callbackHost,
		Port:     opts.callbackPort,
		CertFile: opts.callbackCert,
		KeyFile:  opts.callbackKey,
	}, state)
	if err != nil {
		return auth.SecretBundle{}, err
	}
	defer callback.Close()
	if err := openOAuthAuthorization(cmd, s, opts, secret, callback.RedirectURI, state); err != nil {
		return auth.SecretBundle{}, err
	}
	waitCtx, cancel := loginWaitContext(cmd.Context(), opts.loginTimeout)
	defer cancel()
	code, err := callback.Wait(waitCtx)
	if err != nil {
		return auth.SecretBundle{}, err
	}
	return exchangeLoginCodeWithRedirect(waitCtx, target, secret, code, callback.RedirectURI)
}

func openOAuthAuthorization(cmd *cobra.Command, s *state, opts *authLoginOptions, secret auth.SecretBundle, redirectURI, state string) error {
	url := auth.AuthorizationURL(secret.ClientID, redirectURI, state, selectedOAuthScopes(opts.scopes))
	if shouldWriteHumanNotice(s) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "OAuth callback listening at %s\nIf your browser warns about the self-signed localhost certificate, continue only for this localhost callback.\n", redirectURI)
	}
	if opts.noBrowser {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Open this authorization URL to continue login:\n%s\n", url)
		return err
	}
	if err := auth.OpenBrowser(url); err != nil {
		_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser automatically: %v\nOpen this authorization URL to continue login:\n%s\n", err, url)
		return writeErr
	}
	return nil
}

func exchangeLoginCodeWithRedirect(ctx context.Context, target authLoginTarget, secret auth.SecretBundle, code, redirectURI string) (auth.SecretBundle, error) {
	exchanged, err := auth.ExchangeCode(ctx, oauthHTTPClient(target.cfg.Timeout()), secret.ClientID, secret.ClientSecret, code, redirectURI)
	if err != nil {
		return auth.SecretBundle{}, err
	}
	exchanged.AccountURL = firstNonEmpty(exchanged.AccountURL, target.profile.AccountURL)
	return exchanged, nil
}

func selectedOAuthScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return auth.ReadOnlyScopes
	}
	return scopes
}

func writeStoredLogin(cmd *cobra.Command, s *state, target authLoginTarget, secret auth.SecretBundle) error {
	warnPlaintextSecretWrite(cmd, s, target.ref)
	if err := storeLoginSecret(cmd.Context(), target.store, target.ref, target.authType, secret); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"profile": target.profileName, "account_url": target.profile.AccountURL, "stored": true})
	return output.Write(cmd.OutOrStdout(), output.Success("auth login", target.profileName, target.profile.AccountURL, raw), s.outputOptions())
}

func runAuthStatus(cmd *cobra.Command, s *state) error {
	cfg, err := s.loadConfig(cmd.Context())
	if err != nil {
		return err
	}
	name, p, err := cfg.ActiveProfile()
	if err != nil {
		return err
	}
	ref, _ := auth.ParseRef(p.SecretRef)
	store, _ := auth.StoreFor(ref)
	secret, err := store.Get(cmd.Context(), ref)
	ok := err == nil && (secret.AccessToken != "" || secret.AdminToken != "")
	raw, _ := json.Marshal(map[string]any{"profile": name, "account_url": p.AccountURL, "authenticated": ok, "secret_ref": p.SecretRef})
	return output.Write(cmd.OutOrStdout(), output.Success("auth status", name, p.AccountURL, raw), s.outputOptions())
}

func runAuthRefresh(cmd *cobra.Command, s *state) error {
	cfg, err := s.loadConfig(cmd.Context())
	if err != nil {
		return err
	}
	name, p, err := cfg.ActiveProfile()
	if err != nil {
		return err
	}
	ref, _ := auth.ParseRef(p.SecretRef)
	store, _ := auth.StoreFor(ref)
	secret, err := store.Get(cmd.Context(), ref)
	if err != nil {
		return err
	}
	refreshed, err := auth.Refresh(cmd.Context(), oauthHTTPClient(cfg.Timeout()), secret)
	if err != nil {
		return err
	}
	if err := store.Set(cmd.Context(), ref, refreshed); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"profile": name, "account_url": firstNonEmpty(refreshed.AccountURL, p.AccountURL), "refreshed": true})
	return output.Write(cmd.OutOrStdout(), output.Success("auth refresh", name, p.AccountURL, raw), s.outputOptions())
}

func runAuthLogout(cmd *cobra.Command, s *state) error {
	cfg, err := s.loadConfig(cmd.Context())
	if err != nil {
		return err
	}
	name, p, err := cfg.ActiveProfile()
	if err != nil {
		return err
	}
	ref, _ := auth.ParseRef(p.SecretRef)
	store, _ := auth.StoreFor(ref)
	if err := store.Delete(cmd.Context(), ref); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"profile": name, "logged_out": true})
	return output.Write(cmd.OutOrStdout(), output.Success("auth logout", name, p.AccountURL, raw), s.outputOptions())
}

func warnPlaintextSecretWrite(cmd *cobra.Command, s *state, ref auth.SecretRef) {
	if ref.Scheme != "plaintext" || !shouldWriteHumanNotice(s) {
		return
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "wsectl: warning: plaintext secret storage is enabled; use keyring or encrypted-file for persistent credentials.")
}

func loginWaitContext(parent context.Context, spec string) (context.Context, context.CancelFunc) {
	d, err := time.ParseDuration(spec)
	if err != nil || d <= 0 {
		d = 10 * time.Minute
	}
	return context.WithTimeout(parent, d)
}

func oauthHTTPClient(timeout time.Duration) *http.Client {
	return auth.HTTPClientWithTimeout(timeout)
}

func validateAuthLoginFlags(cmd *cobra.Command, authType string, clientSecretStdin, adminTokenStdin, manualCode bool) error {
	switch authType {
	case "admin_token":
		for _, flagName := range []string{"client-id", "client-secret", "access-token", "refresh-token", "code"} {
			if cmd.Flags().Changed(flagName) {
				return worksection.UsageError("--%s cannot be used with an admin_token profile", flagName)
			}
		}
		if clientSecretStdin {
			return worksection.UsageError("--client-secret-stdin cannot be used with an admin_token profile")
		}
		if manualCode {
			return worksection.UsageError("--manual-code cannot be used with an admin_token profile")
		}
		if cmd.Flags().Changed("scope") {
			return worksection.UsageError("--scope cannot be used with an admin_token profile")
		}
	case "oauth2", "":
		if cmd.Flags().Changed("admin-token") {
			return worksection.UsageError("--admin-token cannot be used with an oauth2 profile")
		}
		if adminTokenStdin {
			return worksection.UsageError("--admin-token-stdin cannot be used with an oauth2 profile")
		}
	default:
		return worksection.UsageError("unsupported auth_type %q", authType)
	}
	return nil
}

func authLoginSecret(cmd *cobra.Command, accountURL, authType string, clientSecretStdin, adminTokenStdin bool) (auth.SecretBundle, error) {
	if authType == "admin_token" {
		adminToken, err := secretFlagValue(cmd, "admin-token", os.Getenv("WSECTL_ADMIN_TOKEN"), adminTokenStdin)
		if err != nil {
			return auth.SecretBundle{}, err
		}
		return auth.SecretBundle{
			AdminToken: adminToken,
			AccountURL: accountURL,
		}, nil
	}
	clientSecret, err := secretFlagValue(cmd, "client-secret", os.Getenv("WSECTL_CLIENT_SECRET"), clientSecretStdin)
	if err != nil {
		return auth.SecretBundle{}, err
	}
	return auth.SecretBundle{
		ClientID:     firstNonEmpty(cmd.Flag("client-id").Value.String(), os.Getenv("WSECTL_CLIENT_ID")),
		ClientSecret: clientSecret,
		AccessToken:  cmd.Flag("access-token").Value.String(),
		RefreshToken: cmd.Flag("refresh-token").Value.String(),
		AccountURL:   accountURL,
	}, nil
}

func storeLoginSecret(ctx context.Context, store auth.SecretStore, ref auth.SecretRef, authType string, secret auth.SecretBundle) error {
	if err := store.Set(ctx, ref, secret); err != nil {
		return err
	}
	stored, err := store.Get(ctx, ref)
	if err != nil {
		return &worksection.Error{Code: worksection.CodeAuth, Message: "stored credentials could not be read back from the secret store: " + err.Error()}
	}
	switch authType {
	case "admin_token":
		if stored.AdminToken == "" {
			return &worksection.Error{Code: worksection.CodeAuth, Message: "stored admin-token credentials are incomplete"}
		}
	default:
		if stored.AccessToken == "" && stored.RefreshToken == "" {
			return &worksection.Error{Code: worksection.CodeAuth, Message: "stored OAuth credentials are incomplete"}
		}
	}
	return nil
}

func secretFlagValue(cmd *cobra.Command, flagName, envValue string, fromStdin bool) (string, error) {
	flagValue := cmd.Flag(flagName).Value.String()
	if fromStdin {
		if cmd.Flags().Changed(flagName) {
			return "", worksection.UsageError("--%s and --%s-stdin are mutually exclusive", flagName, flagName)
		}
		value, err := readSecretLine(cmd.InOrStdin())
		if err != nil {
			return "", err
		}
		return value, nil
	}
	return firstNonEmpty(flagValue, envValue), nil
}

func readSecretLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", worksection.UsageError("expected one secret value on stdin")
	}
	return strings.TrimSpace(scanner.Text()), nil
}

func shouldWriteHumanNotice(s *state) bool {
	if s.quiet {
		return false
	}
	switch s.format {
	case "json", "yaml", "ndjson", "raw":
		return false
	default:
		return true
	}
}
