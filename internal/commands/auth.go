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
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
	"github.com/spf13/cobra"
)

func newAuthCommand(s *state) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authenticate with Worksection"}
	var noBrowser, manualCode bool
	var clientSecretStdin, adminTokenStdin bool
	var callbackHost, callbackCert, callbackKey string
	var loginTimeout string
	var scopes []string
	var code string
	var callbackPort int
	login := &cobra.Command{
		Use:   "login",
		Short: "Start OAuth login or store manually supplied credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := s.loadConfig(cmd.Context())
			if err != nil {
				return err
			}
			name, p, err := cfg.ActiveProfile()
			if err != nil {
				return err
			}
			authType := firstNonEmpty(p.AuthType, "oauth2")
			if err := validateAuthLoginFlags(cmd, authType, clientSecretStdin, adminTokenStdin, manualCode); err != nil {
				return err
			}
			ref, err := auth.ParseRef(p.SecretRef)
			if err != nil {
				return err
			}
			store, err := auth.StoreFor(ref)
			if err != nil {
				return err
			}
			if err := auth.CheckWritable(cmd.Context(), store, ref); err != nil {
				return &worksection.Error{Code: worksection.CodeAuth, Message: "secret store is not writable: " + err.Error()}
			}
			secret, err := authLoginSecret(cmd, p.AccountURL, authType, clientSecretStdin, adminTokenStdin)
			if err != nil {
				return err
			}
			if authType == "admin_token" {
				if secret.AdminToken == "" {
					return worksection.UsageError("admin token is required for admin-token login; pass --admin-token, --admin-token-stdin, or set WSECTL_ADMIN_TOKEN")
				}
				if err := storeLoginSecret(cmd.Context(), store, ref, authType, secret); err != nil {
					return err
				}
				raw, _ := json.Marshal(map[string]any{"profile": name, "account_url": p.AccountURL, "stored": true})
				return output.Write(cmd.OutOrStdout(), output.Success("auth login", name, p.AccountURL, raw), s.outputOptions())
			}
			selectedScopes := scopes
			if len(selectedScopes) == 0 {
				selectedScopes = auth.ReadOnlyScopes
			}
			redirect := fmt.Sprintf("https://%s:%d/callback", callbackHost, callbackPort)
			if code != "" {
				exchanged, err := auth.ExchangeCode(cmd.Context(), oauthHTTPClient(cfg.Timeout()), secret.ClientID, secret.ClientSecret, code, redirect)
				if err != nil {
					return err
				}
				exchanged.AccountURL = firstNonEmpty(exchanged.AccountURL, p.AccountURL)
				secret = exchanged
			}
			if secret.AccessToken == "" && secret.AdminToken == "" {
				if secret.ClientID == "" || secret.ClientSecret == "" {
					return worksection.UsageError("client ID and client secret are required for OAuth login; pass --client-id and set WSECTL_CLIENT_SECRET or use --client-secret-stdin")
				}
				state, err := auth.RandomState()
				if err != nil {
					return err
				}
				if manualCode {
					url := auth.AuthorizationURL(secret.ClientID, redirect, state, selectedScopes)
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "Open this authorization URL and complete OAuth manually:\n%s\n\nThen set WSECTL_CLIENT_SECRET or pipe it with --client-secret-stdin, and run:\nwsectl auth login --code CODE --client-id %q --callback-host %q --callback-port %d\n", url, secret.ClientID, callbackHost, callbackPort)
					return err
				}
				callback, err := auth.StartOAuthCallback(auth.CallbackOptions{
					Host:     callbackHost,
					Port:     callbackPort,
					CertFile: callbackCert,
					KeyFile:  callbackKey,
				}, state)
				if err != nil {
					return err
				}
				defer callback.Close()
				url := auth.AuthorizationURL(secret.ClientID, callback.RedirectURI, state, selectedScopes)
				waitCtx, cancel := loginWaitContext(cmd.Context(), loginTimeout)
				defer cancel()
				if shouldWriteHumanNotice(s) {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "OAuth callback listening at %s\nIf your browser warns about the self-signed localhost certificate, continue only for this localhost callback.\n", callback.RedirectURI)
				}
				if noBrowser {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Open this authorization URL to continue login:\n%s\n", url); err != nil {
						return err
					}
				} else if err := auth.OpenBrowser(url); err != nil {
					if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser automatically: %v\nOpen this authorization URL to continue login:\n%s\n", err, url); writeErr != nil {
						return writeErr
					}
				}
				code, err := callback.Wait(waitCtx)
				if err != nil {
					return err
				}
				exchanged, err := auth.ExchangeCode(waitCtx, oauthHTTPClient(cfg.Timeout()), secret.ClientID, secret.ClientSecret, code, callback.RedirectURI)
				if err != nil {
					return err
				}
				exchanged.AccountURL = firstNonEmpty(exchanged.AccountURL, p.AccountURL)
				secret = exchanged
			}
			if err := storeLoginSecret(cmd.Context(), store, ref, authType, secret); err != nil {
				return err
			}
			raw, _ := json.Marshal(map[string]any{"profile": name, "account_url": p.AccountURL, "stored": true})
			return output.Write(cmd.OutOrStdout(), output.Success("auth login", name, p.AccountURL, raw), s.outputOptions())
		},
	}
	login.Flags().String("client-id", "", "OAuth client ID")
	login.Flags().String("client-secret", "", "OAuth client secret")
	login.Flags().BoolVar(&clientSecretStdin, "client-secret-stdin", false, "Read OAuth client secret from one line on stdin")
	login.Flags().String("access-token", "", "Dangerous: directly store an access token")
	login.Flags().String("refresh-token", "", "Dangerous: directly store a refresh token")
	login.Flags().String("admin-token", "", "Dangerous: directly store an admin API key")
	login.Flags().BoolVar(&adminTokenStdin, "admin-token-stdin", false, "Read admin API key from one line on stdin")
	login.Flags().StringVar(&code, "code", "", "OAuth authorization code to exchange")
	login.Flags().StringVar(&callbackHost, "callback-host", "localhost", "OAuth callback host")
	login.Flags().IntVar(&callbackPort, "callback-port", 33443, "OAuth callback port")
	login.Flags().StringVar(&callbackCert, "callback-cert", "", "OAuth callback TLS certificate")
	login.Flags().StringVar(&callbackKey, "callback-key", "", "OAuth callback TLS key")
	login.Flags().StringVar(&loginTimeout, "login-timeout", "10m", "Maximum time to wait for OAuth browser callback")
	login.Flags().StringArrayVar(&scopes, "scope", nil, "OAuth scope to request; repeat for multiple scopes")
	login.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open browser")
	login.Flags().BoolVar(&manualCode, "manual-code", false, "Use manual code flow")
	_ = login.MarkFlagFilename("callback-cert", "crt", "pem")
	_ = login.MarkFlagFilename("callback-key", "key", "pem")
	cmd.AddCommand(login)
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show auth status",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "refresh",
		Short: "Refresh OAuth token when stored refresh token is available",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Delete stored credentials for the active profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
		},
	})
	return cmd
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
