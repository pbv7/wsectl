package worksection

import "encoding/json"

type Response struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

// DownloadResponse contains bytes and metadata returned by file download.
type DownloadResponse struct {
	FileName    string
	ContentType string
	Body        []byte
}

// ParseResponse decodes Worksection's JSON status envelope.
func ParseResponse(raw []byte) (*Response, error) {
	var body struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return &Response{Status: body.Status, Data: body.Data, Error: body.Error, Raw: raw}, nil
}

// OutputData returns the primary payload promised by the action contract.
// When the upstream body carries a "data" field (array or object responses),
// that is the payload. Otherwise the whole body minus the status envelope is
// returned, which covers object responses that put their fields at the top
// level (e.g. costs total's {total, ...} bundle, or a file download's url).
func (r *Response) OutputData(action string) json.RawMessage {
	if len(r.Data) > 0 && string(r.Data) != "null" {
		return r.Data
	}
	if payload := objectWithoutStatus(r.Raw); len(payload) > 0 {
		return payload
	}
	return json.RawMessage(`[]`)
}

// Aggregate returns the server-side summary an action declares via
// AggregatePath (e.g. costs list's sibling "total" object), lifted out of the
// raw body so callers can surface it in metadata. It returns nil when the
// action declares no aggregate path or the field is absent or null.
func (r *Response) Aggregate(contract ResponseContract) json.RawMessage {
	if contract.AggregatePath == "" {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(r.Raw, &obj); err != nil {
		return nil
	}
	if v, ok := obj[contract.AggregatePath]; ok && len(v) > 0 && string(v) != "null" {
		return v
	}
	return nil
}

func objectWithoutStatus(raw json.RawMessage) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil
	}
	delete(obj, "status")
	delete(obj, "error")
	if len(obj) == 0 {
		return nil
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	return out
}
