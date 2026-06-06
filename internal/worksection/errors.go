package worksection

import "fmt"

type ErrorCode string

const (
	CodeGeneral       ErrorCode = "general"
	CodeUsage         ErrorCode = "usage"
	CodeAuth          ErrorCode = "authentication"
	CodeAuthorization ErrorCode = "authorization"
	CodeNetwork       ErrorCode = "network"
	CodeAPI           ErrorCode = "worksection_api"
	CodeRateLimited   ErrorCode = "rate_limited"
	CodeTruncated     ErrorCode = "truncated"
)

type Error struct {
	Code    ErrorCode
	Message string
	Details map[string]any
}

// Error returns a user-facing message.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// ExitCode maps a Worksection error to wsectl's documented exit code contract.
func (e *Error) ExitCode() int {
	if e == nil {
		return 0
	}
	switch e.Code {
	case CodeUsage:
		return 2
	case CodeAuth:
		return 3
	case CodeAuthorization:
		return 4
	case CodeNetwork:
		return 5
	case CodeAPI:
		return 6
	case CodeRateLimited:
		return 7
	case CodeTruncated:
		return 8
	default:
		return 1
	}
}

// UsageError returns a validation error that exits with code 2.
func UsageError(format string, args ...any) *Error {
	return &Error{Code: CodeUsage, Message: fmt.Sprintf(format, args...)}
}
