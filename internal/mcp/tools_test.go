package mcp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bit-issues/cli/internal/mcp"
	clientgo "github.com/bit-issues/client-go"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testVersion     = "test-version"
	clientVersion   = "1.0.0"
	clientNameAlpha = "claude-code"
	clientNameBeta  = "some-other-client"
)

func TestToolsListAnnotationsAndDescriptions(t *testing.T) {
	t.Parallel()

	for _, name := range []string{clientNameAlpha, clientNameBeta} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			session := testSession(t)
			tools := listTools(t, session, name)
			if len(tools) != expectedToolCount {
				t.Fatalf("tools count = %d, want %d", len(tools), expectedToolCount)
			}

			for _, tool := range tools {
				if len(tool.Name) > maxToolNameLength {
					t.Errorf("tool %q name too long", tool.Name)
				}
				if tool.Annotations == nil {
					t.Errorf("tool %q has no annotations", tool.Name)
					continue
				}
				if tool.Annotations.Title == "" || !tool.Annotations.ReadOnlyHint ||
					!tool.Annotations.IdempotentHint || tool.Annotations.OpenWorldHint == nil ||
					!*tool.Annotations.OpenWorldHint {
					t.Errorf("tool %q has wrong annotations: %+v", tool.Name, tool.Annotations)
				}
				if tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint {
					t.Errorf("tool %q is marked destructive", tool.Name)
				}
				if tool.Description == "" {
					t.Errorf("tool %q has no description", tool.Name)
				}
				for _, banned := range []string{"always", "you must", "call list_"} {
					if strings.Contains(strings.ToLower(tool.Description), banned) {
						t.Errorf("tool %q description is imperative: %q", tool.Name, tool.Description)
					}
				}
			}
		})
	}
}

func TestClientInfoDoesNotChangeTools(t *testing.T) {
	t.Parallel()

	session := testSession(t)
	first := listTools(t, session, clientNameAlpha)
	second := listTools(t, session, clientNameBeta)

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first tools: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second tools: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Errorf("tools differ by clientInfo:\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestToolCallReturnsTextAndStructuredContent(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(
			[]byte(
				`{"items":[{"id":"demo","name":"Demo","repo_url":"","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}],"total":1}`,
			),
		)
	}))
	defer backend.Close()

	session, err := clientgo.NewSession(
		backend.URL+"/api/v1",
		clientgo.Tokens{},
		clientgo.WithHTTPClient(backend.Client()),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	st, ct := mcpsdk.NewInMemoryTransports()
	server := mcp.NewServer(session, mcp.Options{Version: testVersion, StaticMode: false})
	if _, connectErr := server.Connect(t.Context(), st, nil); connectErr != nil {
		t.Fatalf("server connect: %v", connectErr)
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: clientNameAlpha, Version: clientVersion}, nil)
	cs, err := client.Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	result, err := cs.CallTool(t.Context(), &mcpsdk.CallToolParams{
		Name:      "list_projects",
		Arguments: map[string]any{"limit": 5},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool call isError: %+v", result.Content)
	}
	if result.StructuredContent == nil {
		t.Error("structuredContent is missing")
	}
	if len(result.Content) == 0 {
		t.Fatal("text content block is missing")
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("first content is %T, want *TextContent", result.Content[0])
	}
	var decoded struct {
		Projects []struct {
			ID string `json:"id"`
		} `json:"projects"`
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if decodeErr := json.Unmarshal([]byte(text.Text), &decoded); decodeErr != nil {
		t.Fatalf("text content is not project JSON: %v", decodeErr)
	}
	if decoded.Total != 1 || decoded.Limit != 5 || len(decoded.Projects) != 1 {
		t.Errorf("unexpected text content: %+v", decoded)
	}
}

func testSession(t *testing.T) *clientgo.Session {
	t.Helper()

	session, err := clientgo.NewSession("http://127.0.0.1:1/api/v1", clientgo.Tokens{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	return session
}

func listTools(t *testing.T, session *clientgo.Session, clientName string) []*mcpsdk.Tool {
	t.Helper()

	st, ct := mcpsdk.NewInMemoryTransports()
	server := mcp.NewServer(session, mcp.Options{Version: testVersion, StaticMode: false})
	if _, connectErr := server.Connect(t.Context(), st, nil); connectErr != nil {
		t.Fatalf("server connect: %v", connectErr)
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: clientName, Version: clientVersion}, nil)
	cs, err := client.Connect(t.Context(), ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	result, err := cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	return result.Tools
}
