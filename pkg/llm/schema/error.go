package schema

import "errors"

type ErrorKind string

const (
	ErrorInvalidRequest  ErrorKind = "invalid_request"
	ErrorAuth            ErrorKind = "auth"
	ErrorPermission      ErrorKind = "permission"
	ErrorBilling         ErrorKind = "billing"
	ErrorQuotaExceeded   ErrorKind = "quota_exceeded"
	ErrorRateLimited     ErrorKind = "rate_limited"
	ErrorRequestTooLarge ErrorKind = "request_too_large"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorOverloaded      ErrorKind = "overloaded"
	ErrorTemporary       ErrorKind = "temporary"
	ErrorNetwork         ErrorKind = "network"
	ErrorMalformed       ErrorKind = "malformed_response"
	ErrorUnknown         ErrorKind = "unknown"
)

type Error struct {
	Provider     string
	Operation    string
	Kind         ErrorKind
	StatusCode   int
	ProviderType string
	ProviderCode string
	RequestID    string
	Retry        bool
	Cause        error
}

func (e *Error) Error() string {
	msg := e.Provider + ": " + e.Operation + ": " + string(e.Kind)
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) Retryable() bool {
	return e.Retry
}

func IsRetryable(err error) bool {
	var llmErr *Error
	return errors.As(err, &llmErr) && llmErr != nil && llmErr.Retryable()
}
