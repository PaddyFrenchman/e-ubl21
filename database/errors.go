package database

import (
	"context"
	"errors"
	"fmt"
)

// ErrorKind identifies the stable class of a database failure.
type ErrorKind string

const (
	ErrorConfiguration ErrorKind = "configuration"
	ErrorConnectivity  ErrorKind = "connectivity"
	ErrorQuery         ErrorKind = "query"
	ErrorPool          ErrorKind = "pool"
	ErrorMigration     ErrorKind = "migration"
)

// Error is a structured, credential-safe database error.
type Error struct {
	Kind      ErrorKind
	Operation string
	cause     error
	detail    string
}

func (e *Error) Error() string {
	if e.detail != "" {
		return fmt.Sprintf("database %s: %s", e.Operation, e.detail)
	}
	return fmt.Sprintf("database %s failed", e.Operation)
}

// GoString prevents %#v formatting from rendering the private driver cause,
// which can contain connection credentials.
func (e *Error) GoString() string {
	return e.Error()
}

// Is preserves deadline/cancellation checks without exposing the underlying
// driver's potentially sensitive diagnostic text through error formatting.
func (e *Error) Is(target error) bool {
	return errors.Is(e.cause, target)
}

// Timeout reports whether the operation ended because its deadline expired.
func (e *Error) Timeout() bool {
	return errors.Is(e.cause, context.DeadlineExceeded)
}

func configError(field, detail string) error {
	return &Error{
		Kind:      ErrorConfiguration,
		Operation: "configuration",
		detail:    field + " " + detail,
	}
}

func operationError(kind ErrorKind, operation string, cause error) error {
	return &Error{Kind: kind, Operation: operation, cause: cause}
}
