package pipeline

import (
	"errors"
	"fmt"
)

// StopError is a special error that stops pipeline execution immediately.
// Unlike regular errors, StopError prevents further action execution.
type StopError struct {
	Cause error
}

// Error implements the error interface.
func (e *StopError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("pipeline stopped: %v", e.Cause)
	}
	return "pipeline stopped"
}

// Unwrap returns the underlying cause for error chain inspection.
func (e *StopError) Unwrap() error {
	return e.Cause
}

// Stop wraps an error as a StopError to halt pipeline execution.
func Stop(err error) error {
	return &StopError{Cause: err}
}

// IsStopError checks whether an error is or wraps a StopError.
func IsStopError(err error) bool {
	var stopErr *StopError
	return errors.As(err, &stopErr)
}
