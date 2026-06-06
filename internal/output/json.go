package output

import "encoding/json"

// JSON renders an envelope as indented JSON.
func JSON(env Envelope) ([]byte, error) {
	return json.MarshalIndent(env, "", "  ")
}
