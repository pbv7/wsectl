package output

import (
	"encoding/json"

	"github.com/itchyny/gojq"
)

// ApplyJQ evaluates a gojq expression against JSON output.
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
	var out []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		out = append(out, v)
	}
	if len(out) == 1 {
		return json.MarshalIndent(out[0], "", "  ")
	}
	return json.MarshalIndent(out, "", "  ")
}
