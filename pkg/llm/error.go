package llm

import "github.com/iamhectordev/hector/pkg/llm/schema"

type ErrorKind = schema.ErrorKind

const (
	ErrorInvalidRequest  = schema.ErrorInvalidRequest
	ErrorAuth            = schema.ErrorAuth
	ErrorPermission      = schema.ErrorPermission
	ErrorBilling         = schema.ErrorBilling
	ErrorQuotaExceeded   = schema.ErrorQuotaExceeded
	ErrorRateLimited     = schema.ErrorRateLimited
	ErrorRequestTooLarge = schema.ErrorRequestTooLarge
	ErrorNotFound        = schema.ErrorNotFound
	ErrorTimeout         = schema.ErrorTimeout
	ErrorOverloaded      = schema.ErrorOverloaded
	ErrorTemporary       = schema.ErrorTemporary
	ErrorNetwork         = schema.ErrorNetwork
	ErrorMalformed       = schema.ErrorMalformed
	ErrorUnknown         = schema.ErrorUnknown
)

type Error = schema.Error

func IsRetryable(err error) bool {
	return schema.IsRetryable(err)
}
