package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pbv7/wsectl/internal/commands"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
)

type exitCoder interface {
	ExitCode() int
}

type printSuppressor interface {
	SuppressPrint() bool
}

// Run constructs and executes the CLI root command with the supplied arguments.
func Run(ctx context.Context, args []string) error {
	root := commands.NewRoot(Version, Commit, Date)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		if suppressor, ok := err.(printSuppressor); !ok || !suppressor.SuppressPrint() {
			if format := machineFormat(args); format != "" {
				rendered := appMachineError(err)
				_ = output.Write(root.ErrOrStderr(), output.Failure("wsectl", "", rendered.err), output.Options{Format: format})
				return rendered
			}
			_, _ = fmt.Fprintln(root.ErrOrStderr(), err.Error())
		}
		return err
	}
	return nil
}

type appRenderedError struct {
	err error
}

func (e appRenderedError) Error() string { return e.err.Error() }
func (e appRenderedError) Unwrap() error { return e.err }
func (e appRenderedError) SuppressPrint() bool {
	return true
}
func (e appRenderedError) ExitCode() int {
	if ec, ok := e.err.(exitCoder); ok {
		return ec.ExitCode()
	}
	return 1
}

func appMachineError(err error) appRenderedError {
	if _, ok := err.(exitCoder); ok {
		return appRenderedError{err: err}
	}
	return appRenderedError{err: worksection.UsageError("%s", err.Error())}
}

func machineFormat(args []string) string {
	for i, arg := range args {
		switch arg {
		case "--json":
			return "json"
		case "--yaml":
			return "yaml"
		case "--ndjson":
			return "ndjson"
		case "--output":
			if i+1 < len(args) && isMachineFormat(args[i+1]) {
				return args[i+1]
			}
		}
		if value, ok := strings.CutPrefix(arg, "--output="); ok && isMachineFormat(value) {
			return value
		}
	}
	if env := os.Getenv("WSECTL_OUTPUT"); isMachineFormat(env) {
		return env
	}
	return ""
}

func isMachineFormat(format string) bool {
	switch format {
	case "json", "yaml", "ndjson":
		return true
	default:
		return false
	}
}

// ExitCode maps command errors to the process exit code contract documented by
// the CLI. Unknown errors are treated as general failures.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ec, ok := err.(exitCoder); ok {
		return ec.ExitCode()
	}
	return 1
}
