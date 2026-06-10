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
	if format := errorFormat(ctx, args); format != "" {
		_ = output.Write(root.ErrOrStderr(), output.Failure("wsectl", "", renderErr), output.Options{Format: format})
	} else {
		_, _ = fmt.Fprintln(root.ErrOrStderr(), renderErr.Error())
	}
	return classified
}

// errorFormat resolves the format used to render a top-level error using the
// same precedence as normal output: an explicit output flag, then
// WSECTL_OUTPUT, then the config file's default. It returns the machine format
// to render an envelope, or "" for human (plain-text) rendering — including
// when an explicit human selector (--table/--raw/--output table) is chosen
// over a machine config default.
func errorFormat(ctx context.Context, args []string) string {
	format := flagOutput(args)
	if format == "" {
		format = os.Getenv("WSECTL_OUTPUT")
	}
	if format == "" {
		if cfg, err := config.Load(ctx, config.Overrides{ConfigPath: configPathFromArgs(args)}); err == nil {
			format = cfg.Defaults.Output
		}
	}
	if isMachineFormat(format) {
		return format
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

// flagOutput returns the output format selected by CLI flags, mirroring the
// command layer's resolution in PersistentPreRun (internal/commands/root.go):
// --output VALUE is the base, then the --json/--yaml/--table/--ndjson/--raw
// shortcuts override it in that fixed order — so the precedence is by flag,
// not by argument position (e.g. --json always beats --output table, and
// --table always beats --json). Returns "" when no output flag is present.
// errorFormat layers env and config defaults beneath this.
func flagOutput(args []string) string {
	out := ""
	has := map[string]bool{}
	for i, arg := range args {
		if arg == "--output" && i+1 < len(args) {
			out = args[i+1]
		}
		if value, ok := strings.CutPrefix(arg, "--output="); ok {
			out = value
		}
		has[arg] = true
	}
	// Same fixed order as PersistentPreRun: later entries win.
	for _, sc := range []struct{ flag, format string }{
		{"--json", "json"},
		{"--yaml", "yaml"},
		{"--table", "table"},
		{"--ndjson", "ndjson"},
		{"--raw", "raw"},
	} {
		if has[sc.flag] {
			out = sc.format
		}
	}
	return out
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
