package llm

import "fmt"

// Provider error code constants matching §6.10 of the master architecture doc.
const (
	ErrAuthentication      = "provider.auth"
	ErrRateLimit           = "provider.rate_limit"
	ErrContextWindow       = "provider.context_overflow"
	ErrProviderUnavailable = "provider.unavailable"
	ErrBadRequest          = "provider.invalid_request"
	ErrUnknown             = "provider.unknown"
)

// ProviderError is a typed error returned by any LLM adapter.
// The Code field is one of the Err* constants above.
type ProviderError struct {
	Code       string // one of the Err* constants
	StatusHTTP int    // HTTP status code from the provider (0 if not applicable)
	Retryable  bool   // whether the engine should retry
	Err        error  // underlying error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s (http=%d): %v", e.Code, e.StatusHTTP, e.Err)
	}
	return fmt.Sprintf("%s (http=%d)", e.Code, e.StatusHTTP)
}

// Unwrap allows errors.Is / errors.As to inspect the wrapped error.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// ClassifyHTTPError maps an HTTP status code and body to a typed ProviderError.
// This is called by adapters after receiving a non-2xx response.
func ClassifyHTTPError(statusCode int, body []byte) *ProviderError {
	msg := string(body)
	switch {
	case statusCode == 401 || statusCode == 403:
		return &ProviderError{
			Code:       ErrAuthentication,
			StatusHTTP: statusCode,
			Retryable:  false,
			Err:        fmt.Errorf("%s", msg),
		}
	case statusCode == 429:
		return &ProviderError{
			Code:       ErrRateLimit,
			StatusHTTP: statusCode,
			Retryable:  true,
			Err:        fmt.Errorf("%s", msg),
		}
	case statusCode == 400:
		// Check for context length exceeded heuristic.
		return &ProviderError{
			Code:       ErrBadRequest,
			StatusHTTP: statusCode,
			Retryable:  false,
			Err:        fmt.Errorf("%s", msg),
		}
	case statusCode >= 500:
		return &ProviderError{
			Code:       ErrProviderUnavailable,
			StatusHTTP: statusCode,
			Retryable:  true,
			Err:        fmt.Errorf("%s", msg),
		}
	case statusCode >= 400:
		return &ProviderError{
			Code:       ErrBadRequest,
			StatusHTTP: statusCode,
			Retryable:  false,
			Err:        fmt.Errorf("%s", msg),
		}
	default:
		return &ProviderError{
			Code:       ErrUnknown,
			StatusHTTP: statusCode,
			Retryable:  false,
			Err:        fmt.Errorf("%s", msg),
		}
	}
}
