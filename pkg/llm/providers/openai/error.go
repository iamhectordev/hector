package openai

import (
	"errors"
	"net/http"
	"strings"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	sdkopenai "github.com/openai/openai-go/v3"
)

func mapError(operation string, err error) error {
	var apiErr *sdkopenai.Error
	if !errors.As(err, &apiErr) {
		return &schema.Error{
			Provider:  "openai",
			Operation: operation,
			Kind:      schema.ErrorNetwork,
			Retry:     true,
			Cause:     err,
		}
	}

	kind, retry := classifyAPIError(apiErr.StatusCode, apiErr.Type, apiErr.Code, apiErr.Message)
	return &schema.Error{
		Provider:     "openai",
		Operation:    operation,
		Kind:         kind,
		StatusCode:   apiErr.StatusCode,
		ProviderType: apiErr.Type,
		ProviderCode: apiErr.Code,
		Retry:        retry,
		Cause:        err,
	}
}

func classifyAPIError(statusCode int, providerType, providerCode, message string) (schema.ErrorKind, bool) {
	switch {
	case isQuotaError(providerType, providerCode, message):
		return schema.ErrorQuotaExceeded, false
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
	case statusCode == http.StatusServiceUnavailable:
		return schema.ErrorOverloaded, true
	case statusCode >= http.StatusInternalServerError:
		return schema.ErrorTemporary, true
	case statusCode >= http.StatusBadRequest:
		return schema.ErrorInvalidRequest, false
	default:
		return schema.ErrorUnknown, false
	}
}

func isQuotaError(providerType, providerCode, message string) bool {
	text := strings.ToLower(providerType + " " + providerCode + " " + message)
	return strings.Contains(text, "insufficient_quota") ||
		strings.Contains(text, "exceeded your current quota") ||
		strings.Contains(text, "credit") ||
		strings.Contains(text, "billing") ||
		strings.Contains(text, "spend cap")
}
