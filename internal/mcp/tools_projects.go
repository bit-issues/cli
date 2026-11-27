package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	clientgo "github.com/bit-issues/client-go"
	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	listProjectsTitle = "List projects"
	getProjectTitle   = "Get project"
	listProjectsDesc  = "Lists projects with an optional name prefix search and pagination. Returns projects, total, limit and offset. Does not modify projects or return tasks, comments or attachments."
	getProjectDesc    = "Returns one project by slug. Returns the project record. Does not return tasks, comments or attachments; sibling tool list_tasks reads a project's tasks."
	projectsPath      = "/projects"
)

func addProjectTools(
	server *mcpsdk.Server,
	session *clientgo.Session,
	schemas map[string]*jsonschema.Schema,
	opts Options,
) {
	mcpsdk.AddTool(server,
		newTool("list_projects", listProjectsTitle, listProjectsDesc, schemas["list_projects"]),
		listProjectsHandler(session, opts),
	)
	mcpsdk.AddTool(server,
		newTool("get_project", getProjectTitle, getProjectDesc, schemas["get_project"]),
		getProjectHandler(session, opts),
	)
}

func listProjectsHandler(
	session *clientgo.Session,
	opts Options,
) mcpsdk.ToolHandlerFor[ListProjectsIn, ListProjectsOut] {
	return func(
		ctx context.Context,
		_ *mcpsdk.CallToolRequest,
		in ListProjectsIn,
	) (*mcpsdk.CallToolResult, ListProjectsOut, error) {
		limit := ClampLimit(in.Limit)
		offset := clampOffset(in.Offset)

		query := url.Values{}
		query.Set("limit", strconv.Itoa(limit))
		query.Set("offset", strconv.Itoa(offset))
		if search := strings.TrimSpace(in.Search); search != "" {
			query.Set("search", search)
		}

		var body struct {
			Items []clientgo.Project `json:"items"`
			Total int                `json:"total"`
		}
		if err := session.Do(ctx, http.MethodGet, projectsPath, query, nil, &body); err != nil {
			return nil, ListProjectsOut{}, MapError(err, "check the server configuration", opts.StaticMode)
		}

		projects := make([]Project, 0, len(body.Items))
		for _, item := range body.Items {
			projects = append(projects, projectFrom(item))
		}

		out := ListProjectsOut{
			Projects: projects,
			Total:    body.Total,
			Limit:    limit,
			Offset:   offset,
			Note:     pageNote(offset, limit, body.Total),
		}

		return textResult(out), out, nil
	}
}

func getProjectHandler(
	session *clientgo.Session,
	opts Options,
) mcpsdk.ToolHandlerFor[GetProjectIn, ProjectResult] {
	return func(
		ctx context.Context,
		_ *mcpsdk.CallToolRequest,
		in GetProjectIn,
	) (*mcpsdk.CallToolResult, ProjectResult, error) {
		project, err := session.GetProject(ctx, in.Slug)
		if err != nil {
			if IsNotFound(err) {
				return nil, ProjectResult{}, fmt.Errorf("%w: %s; use list_projects", ErrProjectNotFound, in.Slug)
			}

			return nil, ProjectResult{}, MapError(err, "use list_projects", opts.StaticMode)
		}

		out := ProjectResult{Project: projectFrom(*project)}

		return textResult(out), out, nil
	}
}
