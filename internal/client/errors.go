package client

import (
	"errors"
	"fmt"
	"net/http"
)

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

// Error is the verbose human-readable form, including response body.
// Safe for logs inside the provider binary (not surfaced to Terraform
// diagnostics, which should use SafeMessage instead).
func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: status %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

// SafeMessage is the diagnostic-safe form that omits the response body.
// Response bodies can contain echoes of the submitted request (including
// credentials, tokens, or PII), so this form is what resource and provider
// code MUST use when constructing Terraform diagnostics.
func (e *APIError) SafeMessage() string {
	return fmt.Sprintf("%s %s: status %d", e.Method, e.Path, e.StatusCode)
}

// DiagDetail returns a diagnostic-safe error detail string. For APIErrors it
// uses SafeMessage (no response body); for other errors it falls through to
// Error(). Callers MUST use this when surfacing client errors to Terraform.
func DiagDetail(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.SafeMessage()
	}
	return err.Error()
}

// IsNotFound reports whether err is an *APIError with a 404 status.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
