package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pbv7/wsectl/internal/worksection"
)

// Table renders array or object data for human terminal use. When
// requested is non-empty it forces both the column set and their order;
// otherwise the action's curated columns (or a fallback heuristic) are
// used.
func Table(env Envelope, contract worksection.ResponseContract, requested []string) ([]byte, error) {
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
	if len(requested) > 0 {
		arr = projectRows(arr, requested)
	}
	keys, omitted := selectKeys(arr, requested)
	widths := map[string]int{}
	for _, k := range keys {
		widths[k] = displayWidth(k)
	}
	for _, row := range arr {
		for _, k := range keys {
			if l := displayWidth(cell(row[k])); l > widths[k] {
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
	if omitted > 0 {
		fmt.Fprintf(&b, "\nNote: table output shows %d of %d columns; use --fields or --json to inspect omitted fields.\n", len(keys), len(keys)+omitted)
	}
	for _, w := range env.Meta.Warnings {
		if strings.HasPrefix(w, "Requested field ") {
			fmt.Fprintf(&b, "\nNote: %s\n", w)
		}
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

func selectKeys(arr []map[string]any, requested []string) ([]string, int) {
	if len(requested) > 0 {
		return append([]string(nil), requested...), 0
	}
	return preferredKeys(arr)
}

func projectRows(arr []map[string]any, fields []string) []map[string]any {
	out := make([]map[string]any, len(arr))
	for i, row := range arr {
		projected := map[string]any{}
		for _, f := range fields {
			path := splitPath(f)
			if len(path) == 0 {
				continue
			}
			if v, ok := getPath(row, path); ok {
				projected[f] = v
			}
		}
		out[i] = projected
	}
	return out
}

func preferredKeys(arr []map[string]any) ([]string, int) {
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
		return keys[:6], len(keys) - 6
	}
	return keys, 0
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

func displayWidth(value string) int {
	return utf8.RuneCountInString(value)
}
