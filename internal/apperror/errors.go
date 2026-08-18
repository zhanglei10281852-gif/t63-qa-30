package apperror

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeValidation   Code = "validation_error"
	CodeUnauthorized Code = "unauthorized"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeUnavailable  Code = "unavailable"
	CodeCancelled    Code = "cancelled"
	CodeInternal     Code = "internal_error"
)

type AppError struct {
	Code   Code
	Status int
	Op     string
	Err    error
}

func (e *AppError) Error() string {
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code Code, status int, err error) *AppError {
	return &AppError{Code: code, Status: status, Err: err}
}

func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	var app *AppError
	if errors.As(err, &app) {
		return &AppError{Code: app.Code, Status: app.Status, Op: op, Err: err}
	}
	return &AppError{Code: CodeInternal, Status: 500, Op: op, Err: err}
}

var (
	ErrInvalidState = errors.New("invalid state transition")
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrValidation   = errors.New("invalid business input")
	ErrUnavailable  = errors.New("resource unavailable")
	ErrCancelled    = errors.New("operation cancelled")
)

func Validation(err error) error   { return New(CodeValidation, 400, err) }
func Unauthorized(err error) error { return New(CodeUnauthorized, 401, err) }
func NotFound(err error) error     { return New(CodeNotFound, 404, err) }
func Conflict(err error) error     { return New(CodeConflict, 409, err) }
func Unavailable(err error) error  { return New(CodeUnavailable, 503, err) }
