package anthropic

import (
	"errors"
	"net/http"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/iamhectordev/hector/pkg/llm/schema"
)

func mapError(operation string, err error) error {
	var apiErr *sdk.Error
	if !errors.As(err, &apiErr) {
		return &schema.Error{
			Provider:  "anthropic",
			Operation: operation,
			Kind:      schema.ErrorNetwork,
			Retry:     true,
			Cause:     err,
		}
	}

	providerType := string(apiErr.Type())
	kind, retry := classifyAPIError(apiErr.StatusCode, providerType, apiErr.Error())
	return &schema.Error{
		Provider:     "anthropic",
		Operation:    operation,
		Kind:         kind,
		StatusCode:   apiErr.StatusCode,
		ProviderType: providerType,
		RequestID:    apiErr.RequestID,
		Retry:        retry,
		Cause:        err,
	}
}

func classifyAPIError(statusCode int, providerType, message string) (schema.ErrorKind, bool) {
	switch providerType {
	case "authentication_error":
		return schema.ErrorAuth, false
	case "billing_error":
		return schema.ErrorBilling, false
	case "permission_error":
		return schema.ErrorPermission, false
	case "not_found_error":
		return schema.ErrorNotFound, false
	case "rate_limit_error":
		return schema.ErrorRateLimited, true
	case "timeout_error":
		return schema.ErrorTimeout, true
	case "overloaded_error":
		return schema.ErrorOverloaded, true
	case "api_error":
		return schema.ErrorTemporary, true
	case "invalid_request_error":
		if isBillingMessage(message) {
			return schema.ErrorBilling, false
		}
		return schema.ErrorInvalidRequest, false
	}

	switch {
	case statusCode == http.StatusTooManyRequests:
		return schema.ErrorRateLimited, true
	case statusCode == http.StatusUnauthorized:
		return schema.ErrorAuth, false
	case statusCode == http.StatusForbidden:
		return schema.ErrorPermission, false
	case statusCode == http.StatusNotFound:
		return schema.ErrorNotFound, false
	case statusCode == http.StatusRequestEntityTooLarge:
		return schema.ErrorRequestTooLarge, false
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
		return schema.ErrorTimeout, true
	case statusCode == 529:
		return schema.ErrorOverloaded, true
	case statusCode >= http.StatusInternalServerError:
		return schema.ErrorTemporary, true
	case statusCode >= http.StatusBadRequest:
		return schema.ErrorInvalidRequest, false
	default:
		return schema.ErrorUnknown, false
	}
}

func isBillingMessage(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "credit balance") ||
		strings.Contains(message, "billing") ||
		strings.Contains(message, "purchase credits")
}
