package cmd

import "errors"

// Exit codes.
const (
	exitOK    = 0
	exitErr   = 1
	exitUsage = 2
)

// ExitError wraps an error with a specific exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode extracts the exit code from an error, defaulting to 1.
func ExitCode(err error) int {
	if err == nil {
		return exitOK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return exitErr
}

// usage returns a usage error (exit code 2).
func usage(msg string) error {
	return &ExitError{Code: exitUsage, Err: errors.New(msg)}
}
