package output

import (
	"bytes"
	"encoding/json"

	"github.com/pbv7/wsectl/internal/worksection"
)

// NDJSON renders array data as one JSON object per line.
func NDJSON(env Envelope, contract worksection.ResponseContract) ([]byte, error) {
	data := primaryData(env.Data, contract)
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		line, err := json.Marshal(env)
		if err != nil {
			return nil, err
		}
		return line, nil
	}
	var b bytes.Buffer
	for _, item := range arr {
		b.Write(item)
		b.WriteByte('\n')
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
