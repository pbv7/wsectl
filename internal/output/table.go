package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pbv7/wsectl/internal/worksection"
)

// Table renders array or object data for human terminal use.
func Table(env Envelope, contract worksection.ResponseContract) ([]byte, error) {
	data := primaryData(env.Data, contract)
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return []byte(string(data)), nil
		}
		arr = []map[string]any{obj}
	}
	if len(arr) == 0 {
		return []byte("No rows"), nil
	}
	keys := preferredKeys(arr)
	widths := map[string]int{}
	for _, k := range keys {
		widths[k] = len(k)
	}
	for _, row := range arr {
		for _, k := range keys {
			if l := len(cell(row[k])); l > widths[k] {
				widths[k] = l
			}
		}
	}
	var b bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&b, "%-*s  ", widths[k], strings.ToUpper(k))
	}
	b.WriteByte('\n')
	for _, k := range keys {
		b.WriteString(strings.Repeat("-", widths[k]))
		b.WriteString("  ")
	}
	b.WriteByte('\n')
	for _, row := range arr {
		for _, k := range keys {
			fmt.Fprintf(&b, "%-*s  ", widths[k], cell(row[k]))
		}
		b.WriteByte('\n')
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func preferredKeys(arr []map[string]any) []string {
	seen := map[string]bool{}
	for _, row := range arr {
		for k := range row {
			seen[k] = true
		}
	}
	prefs := []string{"id", "name", "status", "email", "date_added", "date_start", "date_end"}
	var keys []string
	for _, k := range prefs {
		if seen[k] {
			keys = append(keys, k)
			delete(seen, k)
		}
	}
	var rest []string
	for k := range seen {
		rest = append(rest, k)
	}
	sort.Strings(rest)
	keys = append(keys, rest...)
	if len(keys) > 6 {
		return keys[:6]
	}
	return keys
}

func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		raw, _ := json.Marshal(t)
		return string(raw)
	}
}
