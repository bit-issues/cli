package mcp

import (
	"errors"
	"fmt"
	"net/http"

	clientgo "github.com/bit-issues/client-go"
	restkit "github.com/capcom6/go-restkit"
)

const (
	hintReauth        = "run 'bitissues-cli auth login'"
	hintReplaceStatic = "unset BITISSUES_TOKEN or mint a new one"
	hintCheckArgs     = "check the tool arguments"
	hintCheckURL      = "check BITISSUES_URL and that the backend is running"
)

var (
	// ErrProjectNotFound classifies a failed project slug pre-flight.
	ErrProjectNotFound = errors.New("project not found")
	// ErrTaskNotFound classifies a missing task lookup.
	ErrTaskNotFound = errors.New("task not found")
	// ErrCommentNotFound classifies a missing comment lookup.
	ErrCommentNotFound = errors.New("comment not found")
	// ErrInvalidArguments classifies a tool call with unusable arguments.
	ErrInvalidArguments = errors.New("invalid arguments")
	// ErrInvalidRequest classifies an upstream 400 response.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrNotAuthenticated classifies an upstream 401 response.
	ErrNotAuthenticated = errors.New("not authenticated or session expired")
	// ErrForbidden classifies an upstream 403 response.
	ErrForbidden = errors.New("forbidden")
	// ErrBackendError classifies an upstream HTTP failure.
	ErrBackendError = errors.New("backend error")
	// ErrBackendUnreachable classifies a transport failure.
	ErrBackendUnreachable = errors.New("backend unreachable")
)

// IsNotFound reports whether err is an upstream 404 response.
func IsNotFound(err error) bool {
	apiErr, ok := restkit.AsAPIError(err)

	return ok && apiErr.StatusCode == http.StatusNotFound
}

// IsUnauthorized reports whether err is an upstream 401 response.
func IsUnauthorized(err error) bool {
	apiErr, ok := restkit.AsAPIError(err)

	return ok && apiErr.StatusCode == http.StatusUnauthorized
}

// MapError converts a client-go error into a sanitized, actionable tool error.
// The HTTP status drives the mapping; the upstream response body is never
// echoed. A rejected refresh keeps clientgo.ErrSessionExpired distinct from a
// generic 401, and a transport failure never becomes a session-expired error.
func MapError(err error, hint string, staticMode bool) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, clientgo.ErrSessionExpired):
		return fmt.Errorf("%w; %s", clientgo.ErrSessionExpired, hintReauth)
	case restkit.IsInfrastructureError(err):
		return fmt.Errorf("%w: %s", ErrBackendUnreachable, hintCheckURL)
	}

	apiErr, ok := restkit.AsAPIError(err)
	if !ok {
		return fmt.Errorf("%w: %s", ErrBackendError, hint)
	}

	switch apiErr.StatusCode {
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrInvalidRequest, hintCheckArgs)
	case http.StatusUnauthorized:
		if staticMode {
			return fmt.Errorf("%w: %s", ErrNotAuthenticated, hintReplaceStatic)
		}

		return fmt.Errorf("%w: %s", ErrNotAuthenticated, hint)
	case http.StatusForbidden:
		return fmt.Errorf("%w: the stored credential cannot read this resource", ErrForbidden)
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: %s", ErrInvalidArguments, hintCheckArgs)
	default:
		return fmt.Errorf("%w (HTTP %d): %s", ErrBackendError, apiErr.StatusCode, hint)
	}
}
