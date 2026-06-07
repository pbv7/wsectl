package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"html"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"time"
)

// CallbackOptions controls the temporary HTTPS server used for OAuth redirects.
type CallbackOptions struct {
	Host     string
	Port     int
	CertFile string
	KeyFile  string
}

// CallbackResult contains the authorization code returned by Worksection.
type CallbackResult struct {
	Code string
}

// CallbackServer owns a short-lived HTTPS listener for one OAuth login.
type CallbackServer struct {
	RedirectURI string
	server      *http.Server
	resultCh    chan string
	errCh       chan error
}

type callbackDecision struct {
	Status  int
	Message string
	Code    string
	Err     error
}

// ValidateState rejects missing or mismatched OAuth state values.
func ValidateState(expected, got string) error {
	if expected == "" || got == "" || expected != got {
		return fmt.Errorf("invalid oauth state")
	}
	return nil
}

// ListenForOAuthCode runs a single-use HTTPS callback server and waits until
// Worksection redirects the browser back with a valid state and code.
func ListenForOAuthCode(ctx context.Context, opts CallbackOptions, expectedState string) (string, string, error) {
	callback, err := StartOAuthCallback(opts, expectedState)
	if err != nil {
		return "", "", err
	}
	defer callback.Close()
	code, err := callback.Wait(ctx)
	return callback.RedirectURI, code, err
}

// StartOAuthCallback starts the HTTPS listener and returns immediately so the
// caller can build and open an authorization URL using RedirectURI.
func StartOAuthCallback(opts CallbackOptions, expectedState string) (*CallbackServer, error) {
	if opts.Host == "" {
		opts.Host = "localhost"
	}
	if !isLoopbackHost(opts.Host) {
		return nil, fmt.Errorf("oauth callback host must be localhost or a loopback address")
	}
	if opts.Port == 0 {
		return nil, fmt.Errorf("oauth callback port 0 is not allowed because Worksection redirect URIs must be registered")
	}
	cert, err := callbackCertificate(opts)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port))
	if err != nil {
		return nil, fmt.Errorf("oauth callback failed to listen on %s:%d: %w; choose a free port that is registered in the Worksection OAuth app", opts.Host, opts.Port, err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	redirectURI := fmt.Sprintf("https://%s:%s/callback", opts.Host, port)
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		decision := classifyCallbackRequest(r.Method, r.URL.Query(), expectedState)
		if decision.Status != http.StatusOK {
			w.WriteHeader(decision.Status)
		}
		writeCallbackPage(w, decision.Message)
		if decision.Err != nil {
			sendCallbackError(errCh, decision.Err)
			return
		}
		if decision.Code != "" {
			sendCallbackResult(resultCh, decision.Code)
		}
	})
	server := &http.Server{
		Handler:           mux,
		ErrorLog:          log.New(io.Discard, "", 0),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       30 * time.Second,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
	}
	tlsListener := tls.NewListener(listener, server.TLSConfig)
	go func() {
		if err := server.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
			sendCallbackError(errCh, err)
		}
	}()
	return &CallbackServer{RedirectURI: redirectURI, server: server, resultCh: resultCh, errCh: errCh}, nil
}

func classifyCallbackRequest(method string, query url.Values, expectedState string) callbackDecision {
	if method != http.MethodGet {
		return callbackDecision{
			Status:  http.StatusMethodNotAllowed,
			Message: "Unsupported OAuth callback method. You can close this tab.",
		}
	}
	if remoteErr := query.Get("error"); remoteErr != "" {
		return callbackDecision{
			Status:  http.StatusOK,
			Message: "Authorization failed. You can close this tab.",
			Err:     fmt.Errorf("oauth authorization failed: %s", remoteErr),
		}
	}
	if err := ValidateState(expectedState, query.Get("state")); err != nil {
		return callbackDecision{
			Status:  http.StatusBadRequest,
			Message: "Invalid OAuth state. You can close this tab.",
		}
	}
	code := query.Get("code")
	if code == "" {
		return callbackDecision{
			Status:  http.StatusOK,
			Message: "Missing OAuth code. You can close this tab.",
			Err:     fmt.Errorf("missing oauth code"),
		}
	}
	return callbackDecision{
		Status:  http.StatusOK,
		Message: "Authorization complete. You can close this tab.",
		Code:    code,
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func sendCallbackResult(ch chan string, code string) {
	select {
	case ch <- code:
	default:
	}
}

func sendCallbackError(ch chan error, err error) {
	select {
	case ch <- err:
	default:
	}
}

// Wait blocks until the callback receives a valid code, an error redirect, or
// the caller's context expires.
func (s *CallbackServer) Wait(ctx context.Context) (string, error) {
	select {
	case code := <-s.resultCh:
		return code, nil
	case err := <-s.errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Close shuts down the callback listener.
func (s *CallbackServer) Close() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.server.Shutdown(shutdownCtx)
}

func callbackCertificate(opts CallbackOptions) (tls.Certificate, error) {
	if opts.CertFile != "" || opts.KeyFile != "" {
		if opts.CertFile == "" || opts.KeyFile == "" {
			return tls.Certificate{}, fmt.Errorf("both callback certificate and key are required")
		}
		return tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: opts.Host},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{opts.Host, "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return tls.X509KeyPair(certPEM, keyPEM)
}

func writeCallbackPage(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><title>wsectl</title><p>%s</p>", html.EscapeString(message))
}
