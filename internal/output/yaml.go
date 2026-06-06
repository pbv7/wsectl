package output

import "gopkg.in/yaml.v3"

// YAML renders an envelope as YAML.
func YAML(env Envelope) ([]byte, error) {
	return yaml.Marshal(env)
}
