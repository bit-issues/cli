// Package mcp implements the read-only Model Context Protocol tools served by
// bitissues-cli over stdio.
package mcp

import (
	"encoding/json"

	clientgo "github.com/bit-issues/client-go"
	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName = "bitissues"
	indentText = "  "
)

// Options configures the MCP server. StaticMode changes the recovery hint for
// a 401: a static token cannot be refreshed in place.
type Options struct {
	Version    string
	StaticMode bool
}

// NewServer registers the six read-only tools over the stateful Session.
func NewServer(session *clientgo.Session, opts Options) *mcpsdk.Server {
	var impl mcpsdk.Implementation
	impl.Name = serverName
	impl.Version = opts.Version

	server := mcpsdk.NewServer(&impl, nil)
	schemas := Schemas()

	addProjectTools(server, session, schemas, opts)
	addTaskTools(server, session, schemas, opts)
	addCommentTools(server, session, schemas, opts)

	return server
}

func newTool(name, title, description string, schema *jsonschema.Schema) *mcpsdk.Tool {
	readOnly := true
	idempotent := true
	openWorld := true
	destructive := false

	var annotations mcpsdk.ToolAnnotations
	annotations.DestructiveHint = &destructive
	annotations.IdempotentHint = idempotent
	annotations.OpenWorldHint = &openWorld
	annotations.ReadOnlyHint = readOnly
	annotations.Title = title

	var tool mcpsdk.Tool
	tool.Name = name
	tool.Title = title
	tool.Description = description
	tool.InputSchema = schema
	tool.Annotations = &annotations

	return &tool
}

func textResult(out any) *mcpsdk.CallToolResult {
	data, err := json.MarshalIndent(out, "", indentText)

	var text mcpsdk.TextContent
	if err != nil {
		text.Text = "result encoding failed"

		return callResult(text)
	}
	text.Text = string(data)

	return callResult(text)
}

func callResult(text mcpsdk.TextContent) *mcpsdk.CallToolResult {
	var result mcpsdk.CallToolResult
	result.Content = []mcpsdk.Content{&text}

	return &result
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}

	return offset
}
