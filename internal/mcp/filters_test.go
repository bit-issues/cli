package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bit-issues/cli/internal/mcp"
)

const (
	vectorSchemaVersion = 1
	limitUpperBound     = 100
	limitLowerBound     = 1
	defaultPageSize     = 20
)

type stripVectorFile struct {
	Schema             int           `json:"schema"`
	Description        string        `json:"description"`
	StrippedCharacters []string      `json:"stripped_characters"`
	Cases              []stripVector `json:"cases"`
}

type stripVector struct {
	Name     string `json:"name"`
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Accepted bool   `json:"accepted"`
}

func TestSanitizeSearchGoldenVectors(t *testing.T) {
	t.Parallel()

	vectors := loadStripVectors(t)
	if vectors.Schema != vectorSchemaVersion {
		t.Fatalf("unexpected fixture schema: got %d, want %d", vectors.Schema, vectorSchemaVersion)
	}
	if len(vectors.StrippedCharacters) == 0 {
		t.Fatal("fixture has no stripped characters")
	}

	for _, vector := range vectors.Cases {
		t.Run(vector.Name, func(t *testing.T) {
			t.Parallel()

			got := mcp.SanitizeSearch(vector.Input)
			if got != vector.Expected {
				t.Errorf("SanitizeSearch(%q) = %q, want %q", vector.Input, got, vector.Expected)
			}
			if accepted := got != ""; accepted != vector.Accepted {
				t.Errorf("SanitizeSearch(%q) accepted = %t, want %t", vector.Input, accepted, vector.Accepted)
			}
		})
	}
}

func TestFilterValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statuses   []string
		priorities []string
	}{
		{
			name:       "valid multi-value",
			statuses:   []string{"New", "In Progress", "On Hold"},
			priorities: []string{"Trivial", "Blocker"},
		},
		{
			name:       "empty slices",
			statuses:   nil,
			priorities: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := mcp.ValidateStatuses(tt.statuses); err != nil {
				t.Errorf("ValidateStatuses(%v) = %v, want nil", tt.statuses, err)
			}
			if err := mcp.ValidatePriorities(tt.priorities); err != nil {
				t.Errorf("ValidatePriorities(%v) = %v, want nil", tt.priorities, err)
			}
		})
	}
}

func TestFilterRejectsInvalidEnums(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   []string
		validate func([]string) error
	}{
		{name: "lowercase status", values: []string{"in progress"}, validate: mcp.ValidateStatuses},
		{name: "unknown status", values: []string{"New", "Nope"}, validate: mcp.ValidateStatuses},
		{name: "lowercase priority", values: []string{"minor"}, validate: mcp.ValidatePriorities},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.validate(tt.values); err == nil {
				t.Errorf("values %v accepted by %s", tt.values, tt.name)
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "zero", input: 0, want: defaultPageSize},
		{name: "within", input: defaultPageSize, want: defaultPageSize},
		{name: "above", input: limitUpperBound + 1, want: limitUpperBound},
		{name: "below", input: -1, want: limitLowerBound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := mcp.ClampLimit(tt.input); got != tt.want {
				t.Errorf("ClampLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: "-created_at"},
		{name: "ascending", input: "created_at", want: "created_at"},
		{name: "descending priority", input: "-priority", want: "-priority"},
		{name: "invalid", input: "bogus", want: "-created_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := mcp.NormalizeSort(tt.input); got != tt.want {
				t.Errorf("NormalizeSort(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func loadStripVectors(t *testing.T) stripVectorFile {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}

	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "search_strip_vectors.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden vectors: %v", err)
	}

	var vectors stripVectorFile
	if parseErr := json.Unmarshal(data, &vectors); parseErr != nil {
		t.Fatalf("parse golden vectors: %v", parseErr)
	}

	return vectors
}
