package auth

import "github.com/pbv7/wsectl/internal/worksection"

// AdminHash delegates to the Worksection admin API hash implementation.
func AdminHash(action string, params map[string]string, apiKey string) string {
	return worksection.AdminHash(action, params, apiKey)
}
