package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pbv7/wsectl/internal/commands"
	"github.com/pbv7/wsectl/internal/config"
	"github.com/pbv7/wsectl/internal/output"
	"github.com/pbv7/wsectl/internal/worksection"
)

type exitCoder interface {
	ExitCode() int
}

type printSuppressor interface {
	SuppressPrint() bool
}

// Run constructs and executes the CLI root command with the supplied arguments,
// writing to the process stdout/stderr.
func Run(ctx context.Context, args []string) error {
	return RunWithIO(ctx, args, os.Stdout, os.Stderr)
}

// RunWithIO is Run with injectable streams, used by tests to assert the
// stdout/stderr split and exit-code mapping at the real entry point.
func RunWithIO(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	root := commands.NewRoot(Version, Commit, Date)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return nil
	}
	if suppressor, ok := err.(printSuppressor); ok && suppressor.SuppressPrint() {
		// Already rendered and classified by the command layer.
		return err
	}
	// Classify the exit code independently of output format. An error that
	// surfaces before any command body ran came from cobra's flag/argument
	// parsing or command resolution: that is a usage error (exit 2). Errors
	// from inside a command body keep their own classification (a
	// worksection.Error keeps its code; a bare internal error stays general
	// exit 1), so output format never changes the exit code.
	classified := err // carries the exit code
	renderErr := err  // displayed; unwrapped so the envelope reports the right code
	if !commands.EnteredBody(root) {
		wrapped := appMachineError(err)
		classified, renderErr = wrapped, wrapped.err
	}
	if format := errorFormat(args); format != "" {
		_ = output.Write(root.ErrOrStderr(), output.Failure("wsectl", "", renderErr), output.Options{Format: format})
	} else {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), renderErr.Error())
	}
	return classified
}

// errorFormat resolves the format used to render a top-level error. It honors
// an explicit --json/--yaml/--ndjson/--output flag or WSECTL_OUTPUT first, then
// falls back to the config file's default output, so a usage error raised
// before the command body loads config still respects a configured machine
// format. Returns "" for human (plain-text) rendering.
func errorFormat(args []string) string {
	if f := machineFormat(args); f != "" {
		return f
	}
	cfg, err := config.Load(context.Background(), config.Overrides{ConfigPath: configPathFromArgs(args)})
	if err == nil && isMachineFormat(cfg.Defaults.Output) {
		return cfg.Defaults.Output
	}
	return ""
}

func configPathFromArgs(args []string) string {
	for i, a := range args {
		if a == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--config="); ok {
			return v
		}
	}
	return ""
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
