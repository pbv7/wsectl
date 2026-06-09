package output

import "gopkg.in/yaml.v3"

// YAML renders an envelope as YAML.
//
// env.Data is JSON. We parse it through yaml.v3 directly (YAML 1.2 is a
// JSON superset) rather than json.Unmarshal-then-yaml.Marshal so numeric
// tokens are preserved as their original text. Decoding via `any` would
// route integers through float64 and silently truncate values past
// 2^53 — bare numeric IDs from api call --allow-unknown or future
// response fields would corrupt.
func YAML(env Envelope) ([]byte, error) {
	out := struct {
		Status string     `yaml:"status"`
		Data   *yaml.Node `yaml:"data,omitempty"`
		Error  *ErrorBody `yaml:"error,omitempty"`
		Meta   Meta       `yaml:"meta"`
	}{Status: env.Status, Error: env.Error, Meta: env.Meta}
	if len(env.Data) > 0 {
		var doc yaml.Node
		if err := yaml.Unmarshal(env.Data, &doc); err != nil {
			return nil, err
		}
		// yaml.Unmarshal wraps the root in a DocumentNode; unwrap so we
		// embed the actual mapping/sequence/scalar at the data field.
		if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
			out.Data = doc.Content[0]
		} else {
			out.Data = &doc
		}
		// yaml.Unmarshal of JSON preserves the source flow style on every
		// node; clear it so collections marshal as block style to match
		// the rest of the envelope.
		clearFlowStyle(out.Data)
	}
	return yaml.Marshal(out)
}

func clearFlowStyle(n *yaml.Node) {
	if n == nil {
		return
	}
	n.Style = 0
	for _, child := range n.Content {
		clearFlowStyle(child)
	}
}
