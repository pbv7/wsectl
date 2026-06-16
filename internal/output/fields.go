package output

import (
	"encoding/json"
	"strings"

	"github.com/pbv7/wsectl/internal/worksection"
)

// ApplyFieldSelection projects object or array data to a fixed field list.
func ApplyFieldSelection(env Envelope, fields []string, knownFields []string, contract worksection.ResponseContract) (Envelope, error) {
	if len(env.Data) == 0 {
		return env, nil
	}
	warnings := fieldContractWarnings(fields, knownFields)
	data, err := projectRaw(env.Data, fields, &warnings)
	if err != nil {
		return env, err
	}
	env.Data = data
	env.Meta.Warnings = appendWarnings(env.Meta.Warnings, warnings)
	return env, nil
}

func projectRaw(raw json.RawMessage, fields []string, warnings *[]string) (json.RawMessage, error) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		for i := range arr {
			arr[i] = project(arr[i], fields, warnings)
		}
		data, err := json.Marshal(arr)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		data, err := json.Marshal(project(obj, fields, warnings))
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return raw, nil
}

// SelectFields preserves the old helper for tests and callers that already
// rendered an envelope.
func SelectFields(raw []byte, fields []string) ([]byte, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Data) == 0 {
		return raw, nil
	}
	selected, err := ApplyFieldSelection(env, fields, nil, worksection.ResponseContract{})
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(selected, "", "  ")
}

func project(row map[string]any, fields []string, warnings *[]string) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		path := splitPath(field)
		if len(path) == 0 {
			continue
		}
		value, ok := getPath(row, path)
		if !ok {
			*warnings = append(*warnings, "Requested field "+field+" was not present in at least one row.")
			continue
		}
		setPath(out, path, value)
	}
	return out
}

func splitPath(field string) []string {
	var out []string
	for _, part := range strings.Split(field, ".") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getPath(row map[string]any, path []string) (any, bool) {
	var current any = row
	for _, part := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setPath(row map[string]any, path []string, value any) {
	if len(path) == 1 {
		row[path[0]] = value
		return
	}
	next, ok := row[path[0]].(map[string]any)
	if !ok {
		next = map[string]any{}
		row[path[0]] = next
	}
	setPath(next, path[1:], value)
}

func fieldContractWarnings(fields, known []string) []string {
	if len(known) == 0 {
		return nil
	}
	knownSet := map[string]bool{}
	for _, field := range known {
		knownSet[field] = true
	}
	var warnings []string
	for _, field := range fields {
		root := field
		if idx := strings.Index(root, "."); idx >= 0 {
			root = root[:idx]
		}
		if root != "" && !knownSet[root] {
			warnings = append(warnings, "Requested field "+field+" is not in the static action response contract.")
		}
	}
	return warnings
}

func appendWarnings(existing, additions []string) []string {
	if len(additions) == 0 {
		return existing
	}
	seen := map[string]bool{}
	for _, warning := range existing {
		seen[warning] = true
	}
	for _, warning := range additions {
		if !seen[warning] {
			existing = append(existing, warning)
			seen[warning] = true
		}
	}
	return existing
}
