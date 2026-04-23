package client

import "fmt"

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: status %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}
