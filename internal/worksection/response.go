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

// OutputData returns the data shape promised by the local action contract.
func (r *Response) OutputData(action string) json.RawMessage {
	spec, _ := LookupAction(action)
	if spec.Response.Shape == "composite" {
		if payload := objectWithoutStatus(r.Raw); len(payload) > 0 {
			return payload
		}
	}
	if len(r.Data) > 0 && string(r.Data) != "null" {
		return r.Data
	}
	if payload := objectWithoutStatus(r.Raw); len(payload) > 0 {
		return payload
	}
	return json.RawMessage(`[]`)
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
