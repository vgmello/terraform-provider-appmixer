package client

import "fmt"

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
