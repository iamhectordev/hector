# Provider Error Normalization

Provider adapters translate implementation-specific errors into package-owned typed errors.

## Principles
- The package that owns the interface also owns the normalized error type.
- Implementations map SDK/provider errors before returning across the interface boundary.
- Callers make retry, escalation, and logging decisions from normalized categories.
- Preserve the original error with `Unwrap()`.
- Use structured provider fields first; message matching is a last resort.
- Separate retryable transient failures from operator-action failures.
- Do not include credentials, prompts, or raw request/response bodies in normalized errors.
