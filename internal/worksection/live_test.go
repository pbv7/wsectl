package worksection

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveSmoke(t *testing.T) {
	if os.Getenv("WSECTL_LIVE_TESTS") != "1" {
		t.Skip("set WSECTL_LIVE_TESTS=1 to run live Worksection smoke tests")
	}
	accountURL := os.Getenv("WSECTL_TEST_ACCOUNT_URL")
	token := os.Getenv("WSECTL_TEST_ACCESS_TOKEN")
	if accountURL == "" || token == "" {
		t.Fatal("WSECTL_TEST_ACCOUNT_URL and WSECTL_TEST_ACCESS_TOKEN are required")
	}
	limiter, err := NewLimiter("1/s")
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(nil, Credentials{Mode: AuthOAuth2, Token: token, AccountURL: accountURL}, 30*time.Second, limiter)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, tc := range []struct {
		action string
		params map[string]string
	}{
		{action: "me"},
		{action: "get_projects", params: map[string]string{"filter": "active"}},
		{action: "search_tasks", params: map[string]string{"filter": "name has 'invoice'"}},
		{action: "get_users"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			if _, err := client.Call(ctx, tc.action, tc.params); err != nil {
				t.Fatal(err)
			}
		})
	}
}
