// Package envx provides type-safe configuration loading from environment variables.
package envx

import (
	"errors"
	"fmt"
)

// Sentinel errors for use with errors.Is.
var (
	ErrRequired        = errors.New("required field is empty")
	ErrValidation      = errors.New("validation failed")
	ErrUnsupportedType = errors.New("unsupported type")
	ErrParse           = errors.New("parse error")
)

// Error wraps configuration errors with context.
type Error struct {
	Field string
	Err   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("envx: %s: %v", e.Field, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }
