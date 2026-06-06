package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

const AuthURL = "https://worksection.com/oauth2/authorize"
const TokenURL = "https://worksection.com/oauth2/token"

// ReadOnlyScopes asks Worksection for all documented read permissions used by
// the MVP command surface.
var ReadOnlyScopes = []string{
	"projects_read",
	"tasks_read",
	"costs_read",
	"tags_read",
	"comments_read",
	"files_read",
	"users_read",
	"contacts_read",
}

func AuthorizationURL(clientID, redirectURI, state string, scopes []string) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("state", state)
	if len(scopes) > 0 {
		v.Set("scope", strings.Join(scopes, " "))
	}
	return AuthURL + "?" + v.Encode()
}

// RandomState returns an unpredictable OAuth state token used to bind the
// browser redirect to the login attempt that initiated it.
func RandomState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// OpenBrowser asks the operating system to open the authorization URL in the
// user's default browser. It intentionally avoids shell evaluation.
func OpenBrowser(authURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", authURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", authURL)
	default:
		cmd = exec.Command("xdg-open", authURL)
	}
	return cmd.Start()
}
