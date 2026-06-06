package output

import (
	"encoding/json"

	"github.com/pbv7/wsectl/internal/worksection"
)

func primaryData(data json.RawMessage, contract worksection.ResponseContract) json.RawMessage {
	if !isCompositeContract(contract) {
		return data
	}
	if raw, ok := rawAtPath(data, contractDataPath(contract)); ok {
		return raw
	}
	return data
}

func isCompositeContract(contract worksection.ResponseContract) bool {
	return contract.Shape == "composite" && contractDataPath(contract) != ""
}

func contractDataPath(contract worksection.ResponseContract) string {
	if contract.CountPath != "" {
		return contract.CountPath
	}
	return contract.DataPath
}

func rawAtPath(raw json.RawMessage, path string) (json.RawMessage, bool) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return raw, true
	}
	current := raw
	for _, part := range parts {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(current, &obj); err != nil || obj == nil {
			return nil, false
		}
		next, ok := obj[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func setRawAtPath(raw json.RawMessage, path string, value json.RawMessage) (json.RawMessage, error) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return value, nil
	}
	return setRawAtParts(raw, parts, value)
}

func setRawAtParts(raw json.RawMessage, parts []string, value json.RawMessage) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if len(parts) == 1 {
		obj[parts[0]] = value
		return json.Marshal(obj)
	}
	child, ok := obj[parts[0]]
	if !ok {
		child = json.RawMessage(`{}`)
	}
	updated, err := setRawAtParts(child, parts[1:], value)
	if err != nil {
		return nil, err
	}
	obj[parts[0]] = updated
	return json.Marshal(obj)
}
