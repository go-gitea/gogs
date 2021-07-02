package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// OsExiter is the function used when the app exits. If not set defaults to os.Exit.
var OsExiter = os.Exit

// ErrWriter is used to write errors to the user. This can be anything
// implementing the io.Writer interface and defaults to os.Stderr.
var ErrWriter io.Writer = os.Stderr

// MultiError is an error that wraps multiple errors.
type MultiError interface {
	error
	Errors() []error
}

// newMultiError creates a new MultiError. Pass in one or more errors.
func newMultiError(err ...error) MultiError {
	ret := multiError(err)
	return &ret
}

type multiError []error

// Error implements the error interface.
func (m *multiError) Error() string {
	errs := make([]string, len(*m))
	for i, err := range *m {
		errs[i] = err.Error()
	}

	return strings.Join(errs, "\n")
}

// Errors returns a copy of the errors slice
func (m *multiError) Errors() []error {
	errs := make([]error, len(*m))
	for _, err := range *m {
		errs = append(errs, err)
	}
	return errs
}

// ErrorFormatter is the interface that will suitably format the error output
type ErrorFormatter interface {
	Format(s fmt.State, verb rune)
}

// ExitCoder is the interface checked by `App` and `Command` for a custom exit
// code
type ExitCoder interface {
	error
	ExitCode() int
}

type exitError struct {
	exitCode int
	message  interface{}
}

// NewExitError calls Exit to create a new ExitCoder.
//
// Deprecated: This function is a duplicate of Exit and will eventually be removed.
func NewExitError(message interface{}, exitCode int) ExitCoder {
	return Exit(message, exitCode)
}

// Exit wraps a message and exit code into an error, which by default is
// handled with a call to os.Exit during default error handling.
//
// This is the simplest way to trigger a non-zero exit code for an App without
// having to call os.Exit manually. During testing, this behavior can be avoided
// by overiding the ExitErrHandler function on an App or the package-global
// OsExiter function.
func Exit(message interface{}, exitCode int) ExitCoder {
	return &exitError{
		message:  message,
		exitCode: exitCode,
	}
}

func (ee *exitError) Error() string {
	return fmt.Sprintf("%v", ee.message)
}

func (ee *exitError) ExitCode() int {
	return ee.exitCode
}

// HandleExitCoder handles errors implementing ExitCoder by printing their
// message and calling OsExiter with the given exit code.
//
// If the given error instead implements MultiError, each error will be checked
// for the ExitCoder interface, and OsExiter will be called with the last exit
// code found, or exit code 1 if no ExitCoder is found.
//
// This function is the default error-handling behavior for an App.
func HandleExitCoder(err error) {
	if err == nil {
		return
	}

	if exitErr, ok := err.(ExitCoder); ok {
		if err.Error() != "" {
			if _, ok := exitErr.(ErrorFormatter); ok {
				_, _ = fmt.Fprintf(ErrWriter, "%+v\n", err)
			} else {
				_, _ = fmt.Fprintln(ErrWriter, err)
			}
		}
		OsExiter(exitErr.ExitCode())
		return
	}

	if multiErr, ok := err.(MultiError); ok {
		code := handleMultiError(multiErr)
		OsExiter(code)
		return
	}
}

func handleMultiError(multiErr MultiError) int {
	code := 1
	for _, merr := range multiErr.Errors() {
		if multiErr2, ok := merr.(MultiError); ok {
			code = handleMultiError(multiErr2)
		} else if merr != nil {
			fmt.Fprintln(ErrWriter, merr)
			if exitErr, ok := merr.(ExitCoder); ok {
				code = exitErr.ExitCode()
			}
		}
	}
	return code
}
