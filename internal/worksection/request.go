package worksection

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// EncodeParams builds a deterministic form body for an action and params.
func EncodeParams(action string, params map[string]string) string {
	values := url.Values{}
	values.Set("action", action)
	for k, v := range params {
		if v != "" {
			values.Set(k, v)
		}
	}
	return values.Encode()
}

// AdminHash implements Worksection's admin API md5 signature calculation.
func AdminHash(action string, params map[string]string, apiKey string) string {
	keys := make([]string, 0, len(params)+1)
	keys = append(keys, "action")
	for k, v := range params {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "action" {
			pairs = append(pairs, "action="+action)
			continue
		}
		pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	sum := md5.Sum([]byte(strings.Join(pairs, "&") + apiKey))
	return hex.EncodeToString(sum[:])
}
