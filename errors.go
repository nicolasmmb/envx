package envx

import (
	"errors"
	"fmt"
)

var (
	ErrRequired        = errors.New("required field is empty")
	ErrValidation      = errors.New("validation failed")
	ErrUnsupportedType = errors.New("unsupported type")
	ErrParse           = errors.New("parse error")
)

type Error struct {
	Field string
	Err   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("envx: %s: %v", e.Field, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }
