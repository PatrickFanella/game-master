package llm

import "fmt"

// ErrConnection indicates a provider connection failure.
type ErrConnection struct {
	URL string
	Err error
}

func (e *ErrConnection) Error() string {
	if e.URL == "" {
		return fmt.Sprintf("provider connection failed: %v", e.Err)
	}
	return fmt.Sprintf("provider connection failed for %s: %v", e.URL, e.Err)
}

func (e *ErrConnection) Unwrap() error { return e.Err }

// ErrTimeout indicates a provider request timed out.
type ErrTimeout struct {
	URL string
	Err error
}

func (e *ErrTimeout) Error() string {
	if e.URL == "" {
		return fmt.Sprintf("provider request timed out: %v", e.Err)
	}
	return fmt.Sprintf("provider request timed out for %s: %v", e.URL, e.Err)
}

func (e *ErrTimeout) Unwrap() error { return e.Err }

// ErrRateLimit indicates provider throttling.
type ErrRateLimit struct {
	URL        string
	StatusCode int
	Err        error
}

func (e *ErrRateLimit) Error() string {
	return fmt.Sprintf("provider rate limited request to %s (status %d): %v", e.URL, e.StatusCode, e.Err)
}

func (e *ErrRateLimit) Unwrap() error { return e.Err }

// ErrAuth indicates provider authentication or authorization failure.
type ErrAuth struct {
	URL        string
	StatusCode int
	Err        error
}

func (e *ErrAuth) Error() string {
	return fmt.Sprintf("provider auth failed for %s (status %d): %v", e.URL, e.StatusCode, e.Err)
}

func (e *ErrAuth) Unwrap() error { return e.Err }

// ErrMalformedResponse indicates provider returned malformed or unexpected data.
type ErrMalformedResponse struct {
	URL string
	Err error
}

func (e *ErrMalformedResponse) Error() string {
	if e.URL == "" {
		return fmt.Sprintf("provider returned malformed response: %v", e.Err)
	}
	return fmt.Sprintf("provider returned malformed response from %s: %v", e.URL, e.Err)
}

func (e *ErrMalformedResponse) Unwrap() error { return e.Err }

// ErrModelNotFound indicates the requested model does not exist for the provider.
type ErrModelNotFound struct {
	URL        string
	Model      string
	StatusCode int
	Err        error
}

func (e *ErrModelNotFound) Error() string {
	if e.Model == "" {
		return fmt.Sprintf("provider model not found at %s (status %d): %v", e.URL, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("provider model %q not found at %s (status %d): %v", e.Model, e.URL, e.StatusCode, e.Err)
}

func (e *ErrModelNotFound) Unwrap() error { return e.Err }
