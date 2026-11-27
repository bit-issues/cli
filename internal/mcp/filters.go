package mcp

import (
	"errors"
	"fmt"
	"strings"

	clientgo "github.com/bit-issues/client-go"
)

const (
	defaultLimit = 20
	maxLimit     = 100
	minLimit     = 1
	defaultSort  = "-created_at"
	stripChars   = `+-<>()~*"@`

	statusValues   = "New, Open, In Progress, Resolved, Closed, Reopened, Invalid, Duplicate, Wontfix, On Hold"
	priorityValues = "Trivial, Minor, Major, Critical, Blocker"
)

var (
	// ErrInvalidStatus classifies a status filter outside the allowed enum.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrInvalidPriority classifies a priority filter outside the allowed enum.
	ErrInvalidPriority = errors.New("invalid priority")
	// ErrEmptySearch classifies a search that sanitizes to an empty string.
	ErrEmptySearch = errors.New("search normalizes to empty")
)

// SanitizeSearch mirrors backend/internal/tasks/domain.go sanitizeSearch and
// the shared testdata/search_strip_vectors.json fixture. Stripped operators
// are removed anywhere in the input; surrounding whitespace is trimmed.
func SanitizeSearch(search string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(stripChars, r) {
			return -1
		}

		return r
	}, search)

	return strings.TrimSpace(clean)
}

// ValidateStatuses checks every status against the client-go enum. The REST
// layer does not validate these values, so a bad value would silently match
// nothing; client-side validation turns that into an actionable tool error.
func ValidateStatuses(values []string) error {
	for _, value := range values {
		if !clientgo.Status(value).IsValid() {
			return fmt.Errorf("%w: %q; allowed values: %s", ErrInvalidStatus, value, statusValues)
		}
	}

	return nil
}

// ValidatePriorities checks every priority against the client-go enum for the
// same reason as ValidateStatuses.
func ValidatePriorities(values []string) error {
	for _, value := range values {
		if !clientgo.Priority(value).IsValid() {
			return fmt.Errorf("%w: %q; allowed values: %s", ErrInvalidPriority, value, priorityValues)
		}
	}

	return nil
}

// JoinCSV renders a validated filter slice as the comma-separated REST query
// value.
func JoinCSV(values []string) string {
	return strings.Join(values, ",")
}

// ClampLimit applies the REST page-size contract: zero means the default,
// negative values become the minimum and large values become the maximum.
func ClampLimit(limit int) int {
	switch {
	case limit == 0:
		return defaultLimit
	case limit < minLimit:
		return minLimit
	case limit > maxLimit:
		return maxLimit
	default:
		return limit
	}
}

// NormalizeSort returns a supported sort key, falling back to newest-first.
func NormalizeSort(sort string) string {
	switch sort {
	case "created_at", "-created_at", "priority", "-priority",
		"due_date", "-due_date", "status", "-status":
		return sort
	default:
		return defaultSort
	}
}
