package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/pbv7/wsectl/internal/worksection"
)

type Meta struct {
	Action          string   `json:"action,omitempty" yaml:"action,omitempty"`
	Profile         string   `json:"profile,omitempty" yaml:"profile,omitempty"`
	AccountURL      string   `json:"account_url,omitempty" yaml:"account_url,omitempty"`
	ContractVersion string   `json:"contract_version,omitempty" yaml:"contract_version,omitempty"`
	ResponseShape   string   `json:"response_shape,omitempty" yaml:"response_shape,omitempty"`
	Count           int      `json:"count" yaml:"count"`
	Truncated       bool     `json:"truncated" yaml:"truncated"`
	Warnings        []string `json:"warnings" yaml:"warnings"`
}

// Envelope is the stable top-level shape for machine-readable command output.
type Envelope struct {
	Status string          `json:"status" yaml:"status"`
	Data   json.RawMessage `json:"data,omitempty" yaml:"data,omitempty"`
	Error  *ErrorBody      `json:"error,omitempty" yaml:"error,omitempty"`
	Meta   Meta            `json:"meta" yaml:"meta"`
}

// ErrorBody is the stable error payload embedded in an Envelope.
type ErrorBody struct {
	Code    string         `json:"code" yaml:"code"`
	Message string         `json:"message" yaml:"message"`
	Details map[string]any `json:"details,omitempty" yaml:"details,omitempty"`
}

// Options controls the renderer used for an Envelope.
type Options struct {
	Format          string
	Fields          []string
	JQ              string
	Out             string
	KnownFields     []string
	TableColumns    []string
	Contract        worksection.ResponseContract
	FailOnTruncated bool
}

// Success builds a success envelope with derived count and truncation metadata.
func Success(action, profile, accountURL string, data json.RawMessage) Envelope {
	return SuccessWithContract(action, profile, accountURL, data, worksection.ResponseContract{})
}

// SuccessWithContract builds a success envelope with action contract metadata.
func SuccessWithContract(action, profile, accountURL string, data json.RawMessage, contract worksection.ResponseContract) Envelope {
	if contract.ContractVersion == "" {
		contract.ContractVersion = worksection.ContractVersion
	}
	return Envelope{
		Status: "ok",
		Data:   emptyArrayIfMissing(data),
		Meta: Meta{
			Action:          action,
			Profile:         profile,
			AccountURL:      accountURL,
			ContractVersion: contract.ContractVersion,
			ResponseShape:   contract.Shape,
			Count:           countDataWithContract(data, contract),
			Truncated:       looksTruncatedWithContract(data, contract),
			Warnings:        warningsWithContract(data, contract),
		},
	}
}

// Failure builds an error envelope from a command or API error.
func Failure(action, profile string, err error) Envelope {
	body := &ErrorBody{Code: "general", Message: err.Error()}
	if e, ok := err.(*worksection.Error); ok {
		body.Code = string(e.Code)
		body.Details = e.Details
	}
	return Envelope{Status: "error", Error: body, Meta: Meta{Action: action, Profile: profile, Warnings: []string{}}}
}

// Write renders an envelope to the requested destination.
func Write(w io.Writer, env Envelope, opts Options) error {
	if env.Meta.Truncated && opts.FailOnTruncated {
		return &worksection.Error{Code: worksection.CodeTruncated, Message: "response may be truncated"}
	}
	format := resolveFormat(w, opts.Format)
	env, err := applyWriteFields(env, opts, format)
	if err != nil {
		return err
	}
	out, err := renderEnvelope(env, opts, format)
	if err != nil {
		return err
	}
	out, err = applyWriteJQ(out, opts.JQ)
	if err != nil {
		return err
	}
	return writeRenderedOutput(w, opts.Out, out)
}

func resolveFormat(w io.Writer, requested string) string {
	if requested != "" && requested != "auto" {
		return requested
	}
	if outputIsTerminal(w) {
		return "table"
	}
	return "json"
}

func outputIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && (st.Mode()&os.ModeCharDevice) != 0
}

func applyWriteFields(env Envelope, opts Options, format string) (Envelope, error) {
	if len(opts.Fields) == 0 || format == "raw" {
		return env, nil
	}
	return ApplyFieldSelection(env, opts.Fields, opts.KnownFields, opts.Contract)
}

func renderEnvelope(env Envelope, opts Options, format string) ([]byte, error) {
	switch format {
	case "json":
		return JSON(env)
	case "yaml":
		return YAML(env)
	case "ndjson":
		return NDJSON(env, opts.Contract)
	case "table":
		return Table(env, opts.Contract, tableColumns(opts))
	case "raw":
		return env.Data, nil
	default:
		return nil, worksection.UsageError("unsupported output format %q", opts.Format)
	}
}

func tableColumns(opts Options) []string {
	if len(opts.Fields) > 0 {
		return opts.Fields
	}
	return opts.TableColumns
}

func applyWriteJQ(out []byte, expr string) ([]byte, error) {
	if expr == "" {
		return out, nil
	}
	return ApplyJQ(out, expr)
}

func writeRenderedOutput(w io.Writer, outPath string, out []byte) error {
	if outPath == "" {
		if len(out) == 0 {
			return nil
		}
		_, err := fmt.Fprintln(w, string(out))
		return err
	}
	if outPath == "-" {
		_, err := w.Write(out)
		return err
	}
	return os.WriteFile(outPath, out, 0o600)
}

func emptyArrayIfMissing(data json.RawMessage) json.RawMessage {
	if len(data) == 0 || string(data) == "null" {
		return json.RawMessage(`[]`)
	}
	return data
}

func countData(data json.RawMessage) int {
	var arr []any
	if json.Unmarshal(data, &arr) == nil {
		return len(arr)
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil && obj != nil {
		return 1
	}
	return 0
}

func countDataWithContract(data json.RawMessage, contract worksection.ResponseContract) int {
	return countData(primaryData(data, contract))
}

func looksTruncatedWithContract(data json.RawMessage, contract worksection.ResponseContract) bool {
	return countDataWithContract(data, contract) >= 10000
}

func warningsWithContract(data json.RawMessage, contract worksection.ResponseContract) []string {
	if looksTruncatedWithContract(data, contract) {
		return []string{"Worksection can return at most 10000 records for some endpoints; result completeness is uncertain."}
	}
	return []string{}
}

// LimitData applies client-side slicing to array data.
func LimitData(data json.RawMessage, limit int, contract worksection.ResponseContract) (json.RawMessage, bool, error) {
	if limit <= 0 {
		return data, false, nil
	}
	target := primaryData(data, contract)
	var arr []json.RawMessage
	if err := json.Unmarshal(target, &arr); err != nil {
		return data, false, nil
	}
	if len(arr) <= limit {
		return data, false, nil
	}
	limited, err := json.Marshal(arr[:limit])
	if err != nil {
		return nil, false, err
	}
	if isCompositeContract(contract) {
		out, err := setRawAtPath(data, contractDataPath(contract), limited)
		if err != nil {
			return nil, false, err
		}
		return out, true, nil
	}
	return limited, true, nil
}
