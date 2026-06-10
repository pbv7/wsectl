package output

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAML renders an envelope as YAML.
//
// env.Data is JSON. We cannot parse it with yaml.Unmarshal even though
// YAML 1.2 is nominally a JSON superset: yaml.v3's libyaml-derived
// tokenizer rejects JSON's optional `\/` escape with "found unknown
// escape character", and Worksection emits `\/` in path-like fields
// (e.g. "page":"\/project\/PROJECT_ID\/TASK_ID\/"). Going through
// json.Unmarshal-into-any would lose integer precision past 2^53
// because the result type is float64.
//
// Decode JSON ourselves with json.Decoder using UseNumber so integers
// keep their full text, then walk the value tree and build yaml.Node
// directly with appropriate kinds and tags. The yaml.v3 emitter uses
// Tag to choose quoting at output time, so stringy "true"/"42" stay
// quoted while !!int values render unquoted.
func YAML(env Envelope) ([]byte, error) {
	out := struct {
		Status string     `yaml:"status"`
		Data   *yaml.Node `yaml:"data,omitempty"`
		Error  *ErrorBody `yaml:"error,omitempty"`
		Meta   Meta       `yaml:"meta"`
	}{Status: env.Status, Error: env.Error, Meta: env.Meta}
	if len(env.Data) > 0 {
		dec := json.NewDecoder(bytes.NewReader(env.Data))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out.Data = jsonToYAML(v)
	}
	return yaml.Marshal(out)
}

// jsonToYAML converts a value produced by json.Decoder (with UseNumber)
// into a yaml.Node tree. Map keys are emitted in alphabetical order to
// keep deterministic output, matching the behavior of the previous
// json.Unmarshal-into-map-then-yaml.Marshal path.
func jsonToYAML(v any) *yaml.Node {
	n := &yaml.Node{}
	switch t := v.(type) {
	case nil:
		n.Kind, n.Tag, n.Value = yaml.ScalarNode, "!!null", "null"
	case bool:
		n.Kind, n.Tag = yaml.ScalarNode, "!!bool"
		if t {
			n.Value = "true"
		} else {
			n.Value = "false"
		}
	case string:
		n.Kind, n.Tag, n.Value = yaml.ScalarNode, "!!str", t
	case json.Number:
		n.Kind, n.Value = yaml.ScalarNode, string(t)
		if strings.ContainsAny(string(t), ".eE") {
			n.Tag = "!!float"
		} else {
			n.Tag = "!!int"
		}
	case []any:
		n.Kind = yaml.SequenceNode
		n.Content = make([]*yaml.Node, 0, len(t))
		for _, item := range t {
			n.Content = append(n.Content, jsonToYAML(item))
		}
	case map[string]any:
		n.Kind = yaml.MappingNode
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		n.Content = make([]*yaml.Node, 0, 2*len(keys))
		for _, k := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
			n.Content = append(n.Content, keyNode, jsonToYAML(t[k]))
		}
	default:
		// json.Decoder produces only the types handled above; this is a
		// belt-and-braces fallback so a future decoder change cannot
		// silently emit a zero-valued node.
		n.Kind, n.Tag, n.Value = yaml.ScalarNode, "!!str", ""
	}
	return n
}
