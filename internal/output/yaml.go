package output

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// YAML renders an envelope as YAML.
func YAML(env Envelope) ([]byte, error) {
	out := struct {
		Status string     `yaml:"status"`
		Data   any        `yaml:"data,omitempty"`
		Error  *ErrorBody `yaml:"error,omitempty"`
		Meta   Meta       `yaml:"meta"`
	}{Status: env.Status, Error: env.Error, Meta: env.Meta}
	if len(env.Data) > 0 {
		var d any
		if err := json.Unmarshal(env.Data, &d); err != nil {
			return nil, err
		}
		out.Data = d
	}
	return yaml.Marshal(out)
}
