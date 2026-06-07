package history

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pbv7/wsectl/internal/atomicfile"
)

const (
	SchemaVersion      = "history.1"
	Redacted           = "[redacted]"
	MaxEventBytes      = 4096
	historyLockTimeout = 2 * time.Second
	historyLockPoll    = 25 * time.Millisecond
	historyLockStale   = 10 * time.Minute
)

type Options struct {
	Enabled       bool
	Path          string
	IncludeParams string
}

type Event struct {
	SchemaVersion  string            `json:"schema_version"`
	Timestamp      string            `json:"timestamp"`
	Command        string            `json:"command"`
	NormalizedArgs []string          `json:"normalized_args,omitempty"`
	Action         string            `json:"action,omitempty"`
	Profile        string            `json:"profile,omitempty"`
	AccountURL     string            `json:"account_url,omitempty"`
	AuthType       string            `json:"auth_type,omitempty"`
	Output         string            `json:"output,omitempty"`
	Params         map[string]string `json:"params,omitempty"`
	Status         string            `json:"status"`
	ExitCode       int               `json:"exit_code"`
	ErrorCode      string            `json:"error_code,omitempty"`
	DurationMS     int64             `json:"duration_ms"`
	Count          int               `json:"count,omitempty"`
	Truncated      bool              `json:"truncated,omitempty"`
	Warnings       []string          `json:"warnings,omitempty"`
}

type ReadResult struct {
	Events    []Event
	Malformed int
}

// Sensitive matching is a conservative local-history redaction heuristic. It is
// not a substitute for command-specific secret handling; commands must still
// avoid passing token material into history params whenever they know better.
var sensitiveExact = map[string]bool{
	"auth-code":  true,
	"code":       true,
	"oauth-code": true,
}

var sensitiveWords = map[string]bool{
	"token":         true,
	"secret":        true,
	"authorization": true,
	"bearer":        true,
	"password":      true,
	"passphrase":    true,
	"key":           true,
	"hash":          true,
}

var freeTextExact = map[string]bool{
	"query":    true,
	"filter":   true,
	"text":     true,
	"comment":  true,
	"name":     true,
	"title":    true,
	"email":    true,
	"file":     true,
	"path":     true,
	"url":      true,
	"filename": true,
}

var freeTextSuffixes = []string{
	"-query",
	"-filter",
	"-text",
	"-comment",
	"-name",
	"-title",
	"-email",
	"-path",
	"-url",
}

func Record(ctx context.Context, options Options, event Event) error {
	if !options.Enabled {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.Path == "" {
		return errors.New("history path is empty")
	}
	if event.SchemaVersion == "" {
		event.SchemaVersion = SchemaVersion
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	event = FitEvent(event, MaxEventBytes)
	return withHistoryLock(ctx, options.Path, func() error {
		return appendEvent(options.Path, event)
	})
}

func CheckWritable(ctx context.Context, options Options) error {
	if !options.Enabled {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.Path == "" {
		return errors.New("history path is empty")
	}
	return withHistoryLock(ctx, options.Path, func() error {
		f, err := openAppendFile(options.Path)
		if err != nil {
			return err
		}
		return f.Close()
	})
}

func appendEvent(path string, event Event) error {
	f, err := openAppendFile(path)
	if err != nil {
		return err
	}
	raw, encErr := json.Marshal(event)
	if encErr == nil {
		raw = append(raw, '\n')
		_, encErr = f.Write(raw)
	}
	closeErr := f.Close()
	if encErr != nil {
		return encErr
	}
	if closeErr != nil {
		return closeErr
	}
	return nil
}

func openAppendFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

func ReadWithStats(path string, limit int) (ReadResult, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ReadResult{}, nil
	}
	if err != nil {
		return ReadResult{}, err
	}
	defer func() { _ = f.Close() }()

	result := ReadResult{}
	var ring []Event
	var validCount int
	if limit > 0 {
		ring = make([]Event, limit)
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			result.Malformed++
			continue
		}
		if limit > 0 {
			ring[validCount%limit] = event
			validCount++
			continue
		}
		result.Events = append(result.Events, event)
	}
	if err := scanner.Err(); err != nil {
		return ReadResult{}, err
	}
	if limit > 0 {
		if validCount <= limit {
			result.Events = ring[:validCount]
		} else {
			result.Events = make([]Event, 0, limit)
			start := validCount % limit
			for i := 0; i < limit; i++ {
				result.Events = append(result.Events, ring[(start+i)%limit])
			}
		}
	}
	return result, nil
}

func Clear(ctx context.Context, path string) error {
	if path == "" {
		return errors.New("history path is empty")
	}
	return withHistoryLock(ctx, path, func() error {
		return clearUnlocked(path)
	})
}

func Trim(ctx context.Context, path string, keep int) error {
	if keep < 0 {
		return errors.New("history keep count must be zero or positive")
	}
	if keep == 0 {
		return Clear(ctx, path)
	}
	if path == "" {
		return errors.New("history path is empty")
	}
	return withHistoryLock(ctx, path, func() error {
		result, err := ReadWithStats(path, 0)
		if err != nil || len(result.Events) <= keep {
			return err
		}
		events := result.Events[len(result.Events)-keep:]
		var b strings.Builder
		for _, event := range events {
			raw, err := json.Marshal(event)
			if err != nil {
				return err
			}
			b.Write(raw)
			b.WriteByte('\n')
		}
		return atomicfile.WriteFile(path, []byte(b.String()), 0o600)
	})
}

func clearUnlocked(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type historyLock struct {
	path string
}

func withHistoryLock(ctx context.Context, historyPath string, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	lock, err := acquireHistoryLock(ctx, historyPath)
	if err != nil {
		return err
	}
	defer lock.release()
	return fn()
}

func acquireHistoryLock(ctx context.Context, historyPath string) (*historyLock, error) {
	lockPath := historyPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o700); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, historyLockTimeout)
	defer cancel()
	payload := []byte(fmt.Sprintf("pid=%d\ncreated=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano)))
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			writeErr := writeLockFile(f, payload)
			if writeErr != nil {
				_ = os.Remove(lockPath)
				return nil, writeErr
			}
			return &historyLock{path: lockPath}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		removed, staleErr := removeStaleHistoryLock(lockPath)
		if staleErr != nil {
			return nil, staleErr
		}
		if removed {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("history lock is busy: %s: %w", lockPath, ctx.Err())
		case <-time.After(historyLockPoll):
		}
	}
}

func removeStaleHistoryLock(lockPath string) (bool, error) {
	st, err := os.Stat(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if time.Since(st.ModTime()) < historyLockStale {
		return false, nil
	}
	if err := os.Remove(lockPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return true, nil
}

func writeLockFile(f *os.File, payload []byte) error {
	_, writeErr := f.Write(payload)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (l historyLock) release() {
	_ = os.Remove(l.path)
}

func Args(args []string, policy string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			out = append(out, arg)
			continue
		}
		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if IsSensitiveName(name) {
			if hasValue {
				out = append(out, "--"+name+"="+Redacted)
			} else {
				out = append(out, "--"+name)
				if i+1 < len(args) {
					out = append(out, Redacted)
					i++
				}
			}
			continue
		}
		if name == "param" {
			if hasValue {
				if redacted, ok := ParamPairForPolicy(value, policy); ok {
					out = append(out, "--"+name+"="+redacted)
				}
			} else {
				if i+1 < len(args) {
					if redacted, ok := ParamPairForPolicy(args[i+1], policy); ok {
						out = append(out, "--"+name)
						out = append(out, redacted)
					}
					i++
				}
			}
			continue
		}
		out = append(out, arg)
	}
	return out
}

func Params(params map[string]string, policy string) map[string]string {
	if len(params) == 0 || policy == "none" {
		return nil
	}
	out := make(map[string]string, len(params))
	for key, value := range params {
		if IsSensitiveName(key) {
			if policy == "all" {
				out[key] = Redacted
			}
			continue
		}
		if policy == "safe" && isFreeTextName(key) {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func RedactParamPair(pair string) string {
	redacted, _ := ParamPairForPolicy(pair, "all")
	return redacted
}

func ParamPairForPolicy(pair, policy string) (string, bool) {
	key, value, ok := strings.Cut(pair, "=")
	if !ok {
		return pair, policy != "none"
	}
	switch policy {
	case "none":
		return "", false
	case "safe":
		if IsSensitiveName(key) || isFreeTextName(key) {
			return "", false
		}
		return key + "=" + value, true
	default:
		if IsSensitiveName(key) {
			value = Redacted
		}
		return key + "=" + value, true
	}
}

func IsSensitiveName(name string) bool {
	normalized := normalizeName(name)
	if sensitiveExact[normalized] {
		return true
	}
	for _, word := range strings.Split(normalized, "-") {
		if sensitiveWords[word] {
			return true
		}
	}
	return false
}

func normalizeName(name string) string {
	var b strings.Builder
	prevDash := false
	prevKind := nameRuneOther
	runes := []rune(name)
	for i, r := range runes {
		kind := classifyNameRune(r)
		if kind != nameRuneOther {
			nextKind := nameRuneOther
			if i+1 < len(runes) {
				nextKind = classifyNameRune(runes[i+1])
			}
			if b.Len() > 0 && !prevDash && shouldSeparateNameRunes(prevKind, kind, nextKind) {
				b.WriteByte('-')
			}
			b.WriteRune(asciiLower(r))
			prevDash = false
			prevKind = kind
			continue
		}
		if b.Len() > 0 && !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
		prevKind = nameRuneOther
	}
	return strings.Trim(b.String(), "-")
}

type nameRuneKind int

const (
	nameRuneOther nameRuneKind = iota
	nameRuneLower
	nameRuneUpper
	nameRuneDigit
)

func classifyNameRune(r rune) nameRuneKind {
	switch {
	case r >= 'a' && r <= 'z':
		return nameRuneLower
	case r >= 'A' && r <= 'Z':
		return nameRuneUpper
	case r >= '0' && r <= '9':
		return nameRuneDigit
	default:
		return nameRuneOther
	}
}

func shouldSeparateNameRunes(previous, current, next nameRuneKind) bool {
	if current != nameRuneUpper {
		return false
	}
	return previous == nameRuneLower ||
		previous == nameRuneDigit ||
		(previous == nameRuneUpper && next == nameRuneLower)
}

func asciiLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

func isFreeTextName(name string) bool {
	normalized := normalizeName(name)
	if freeTextExact[normalized] {
		return true
	}
	for _, suffix := range freeTextSuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

func FitEvent(event Event, maxBytes int) Event {
	if maxBytes <= 0 || eventSize(event) <= maxBytes {
		return event
	}
	event.NormalizedArgs = truncateSlice(event.NormalizedArgs, 16, 128)
	event.Warnings = truncateSlice(event.Warnings, 8, 128)
	event.Params = truncateMap(event.Params, 24, 128)
	if eventSize(event) <= maxBytes {
		event.Warnings = appendHistoryTruncationWarning(event.Warnings)
		return event
	}
	event.NormalizedArgs = nil
	event.Params = nil
	event.Warnings = appendHistoryTruncationWarning(nil)
	event.Command = truncateString(event.Command, 128)
	event.Action = truncateString(event.Action, 128)
	event.Profile = truncateString(event.Profile, 128)
	event.AccountURL = truncateString(event.AccountURL, 256)
	event.AuthType = truncateString(event.AuthType, 64)
	event.Output = truncateString(event.Output, 64)
	return event
}

func eventSize(event Event) int {
	raw, err := json.Marshal(event)
	if err != nil {
		return MaxEventBytes + 1
	}
	return len(raw) + 1
}

func truncateSlice(values []string, maxItems, maxRunes int) []string {
	if len(values) == 0 {
		return nil
	}
	if len(values) > maxItems {
		values = values[:maxItems]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, truncateString(value, maxRunes))
	}
	return out
}

func truncateMap(values map[string]string, maxItems, maxRunes int) map[string]string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > maxItems {
		keys = keys[:maxItems]
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = truncateString(values[key], maxRunes)
	}
	return out
}

func truncateString(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}

func appendHistoryTruncationWarning(warnings []string) []string {
	const message = "History event was truncated to fit the local history size cap."
	for _, warning := range warnings {
		if warning == message {
			return warnings
		}
	}
	return append(warnings, message)
}
