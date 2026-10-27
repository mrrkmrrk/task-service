package domain

import "errors"

var (
	ErrUnauthorized = errors.New("invalid credentials or token")
	ErrForbidden    = errors.New("access denied")
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrVersion      = errors.New("task version is stale")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
func Invalid(message string) error       { return &ValidationError{Message: message} }
