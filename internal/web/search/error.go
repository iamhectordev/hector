package search

import "errors"

type ErrorKind string

const (
	ErrorInvalidConfig     ErrorKind = "invalid_config"
	ErrorInvalidRequest    ErrorKind = "invalid_request"
	ErrorUnauthorized      ErrorKind = "unauthorized"
	ErrorRateLimited       ErrorKind = "rate_limited"
	ErrorQuotaExceeded     ErrorKind = "quota_exceeded"
	ErrorTemporary         ErrorKind = "temporary"
	ErrorNetwork           ErrorKind = "network"
	ErrorMalformedResponse ErrorKind = "malformed_response"
)

type Error struct {
	Provider   ProviderName
	Operation  string
	Kind       ErrorKind
	StatusCode int
	Retry      bool
	Cause      error
}

func (e *Error) Error() string {
	msg := string(e.Provider) + ": " + e.Operation + ": " + string(e.Kind)
	if e.Retry {
		msg += ": retryable"
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
	var searchErr *Error
	return errors.As(err, &searchErr) && searchErr != nil && searchErr.Retryable()
}
