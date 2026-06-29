package service

import "errors"

type Code string

const (
	CodeValidation   Code = "validation_error"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeRateLimited  Code = "rate_limited"
	CodeDependency   Code = "dependency_error"
	CodeInternal     Code = "internal_error"
)

type AppError struct {
	Code    Code
	Message string
	Fields  map[string]string
	Err     error
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewError(code Code, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

func ValidationError(fields map[string]string) *AppError {
	return &AppError{Code: CodeValidation, Message: "request validation failed", Fields: fields}
}

func Classify(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
