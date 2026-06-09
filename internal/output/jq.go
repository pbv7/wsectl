package output

import (
	"bytes"
	"encoding/json"

	"github.com/itchyny/gojq"
)

// ApplyJQ evaluates a gojq expression against JSON output. Each result
// from the gojq iterator is encoded as a separate, pretty-printed JSON
// value joined by a newline, matching standard jq behavior. A single
// result produces a single document; multiple results produce a
// newline-separated stream that downstream tools can iterate.
func ApplyJQ(raw []byte, expr string) ([]byte, error) {
	var input any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, err
	}
	iter := q.Run(input)
	var out bytes.Buffer
	first := true
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		encoded, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		if !first {
			out.WriteByte('\n')
		}
		out.Write(encoded)
		first = false
	}
	return out.Bytes(), nil
}
