package testutil

import "os"

// WsectlEnvVars lists every WSECTL_* environment variable any wsectl command
// or library may read. Keep in sync with the variables consumed in
// internal/config, internal/commands, and internal/auth.
var WsectlEnvVars = []string{
	"WSECTL_CONFIG",
	"WSECTL_PROFILE",
	"WSECTL_ACCOUNT_URL",
	"WSECTL_OUTPUT",
	"WSECTL_TIMEOUT",
	"WSECTL_RATE_LIMIT",
	"WSECTL_HISTORY",
	"WSECTL_HISTORY_FILE",
	"WSECTL_HISTORY_PARAMS",
	"WSECTL_ACCESS_TOKEN",
	"WSECTL_REFRESH_TOKEN",
	"WSECTL_ADMIN_TOKEN",
	"WSECTL_CLIENT_ID",
	"WSECTL_CLIENT_SECRET",
	"WSECTL_SECRET_PASSPHRASE",
	"WSECTL_LIVE_TESTS",
	"WSECTL_TEST_ACCOUNT_URL",
	"WSECTL_TEST_ACCESS_TOKEN",
}

// UnsetWsectlEnv unsets every WSECTL_* env var. Call from TestMain so tests
// see a clean baseline regardless of what the developer's shell has set.
// Tests that need a specific value should use t.Setenv, which auto-restores.
func UnsetWsectlEnv() {
	for _, name := range WsectlEnvVars {
		_ = os.Unsetenv(name)
	}
}
