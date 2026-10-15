package cli

import (
	"errors"
	"fmt"
)

// Exit codes are a stable contract for scripts, hosts, and MCP clients (WP1.1).
// Do not renumber them; add new codes only at the end.
const (
	ExitOK       = 0 // success
	ExitInternal = 1 // unexpected internal error (a bug in aayudha)
	ExitUsage    = 2 // usage or configuration error, including commands this binary does not implement yet
	ExitFloor    = 3 // a deterministic floor missed its threshold
	ExitBlocked  = 4 // blocked or needs a human: refused by policy, a gate, an identity check, or a missing signature
	ExitExternal = 5 // an external system failed: network, GitHub, a model provider, a missing tool
	ExitBudget   = 6 // a [budget] or [autonomy] budget limit tripped the circuit breaker
)

// ExitCodes describes every exit code; the CLI help and docs render it.
var ExitCodes = []struct {
	Code    int
	Meaning string
}{
	{ExitOK, "ok"},
	{ExitInternal, "internal error (a bug; please report it)"},
	{ExitUsage, "usage or configuration error, or a command not implemented in this binary yet"},
	{ExitFloor, "floor miss"},
	{ExitBlocked, "blocked or needs a human"},
	{ExitExternal, "external error (network, GitHub, model provider, missing tool)"},
	{ExitBudget, "budget limit reached"},
}

// Error is an error that carries an exit code, and optionally the exact
// command that fixes it (printed after the message as "next:").
type Error struct {
	Code int
	Err  error
	Next string
}

// WithNext attaches the exact command that fixes err. It keeps err's code (an
// error without one is internal).
func WithNext(err error, next string) error {
	if err == nil {
		return nil
	}
	return &Error{Code: CodeOf(err), Err: err, Next: next}
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Errorf returns an *Error with the given exit code and message.
func Errorf(code int, format string, args ...any) error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

// Wrap attaches an exit code to err. It keeps an inner code if err already has one.
func Wrap(code int, err error) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err
	}
	return &Error{Code: code, Err: err}
}

// CodeOf returns the exit code for err: 0 for nil, the attached code for an
// *Error, and ExitInternal for anything else.
func CodeOf(err error) int {
	if err == nil {
		return ExitOK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ExitInternal
}
