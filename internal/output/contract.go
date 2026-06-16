package output

import (
	"encoding/json"

	"github.com/pbv7/wsectl/internal/worksection"
)

// primaryData returns the records payload used for count, limit, and field
// selection. The composite response shape was removed, so every action's data
// is already its primary payload; this stays as a single seam the renderers
// (count, limit, ndjson, table) share, in case a future shape needs to unwrap.
func primaryData(data json.RawMessage, _ worksection.ResponseContract) json.RawMessage {
	return data
}
