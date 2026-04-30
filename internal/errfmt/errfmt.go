package errfmt

import (
	"errors"
	"fmt"
	"net/http"
)

// AuthRequiredError indicates the user needs to authenticate.
type AuthRequiredError struct{ Mediaspace string }

func (e *AuthRequiredError) Error() string {
	return fmt.Sprintf("authentication required for %s", e.Mediaspace)
}

// APIError wraps an HTTP error from the pixx.io API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Format returns a human-readable error message with actionable hints.
func Format(err error) string {
	var authErr *AuthRequiredError
	if errors.As(err, &authErr) {
		return fmt.Sprintf("%s\n  Run: pixx auth login --mediaspace %s --api-key <key>", authErr.Error(), authErr.Mediaspace)
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Sprintf("%s\n  Check your API key. Generate one at: Settings → API → API-Key", apiErr.Error())
		case http.StatusForbidden:
			return fmt.Sprintf("%s\n  Check your permissions in the pixx.io mediaspace.", apiErr.Error())
		case http.StatusTooManyRequests:
			return fmt.Sprintf("%s\n  Rate limited. Wait and retry.", apiErr.Error())
		}
		return apiErr.Error()
	}

	return err.Error()
}
