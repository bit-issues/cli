package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	clientgo "github.com/bit-issues/client-go"
	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	listTasksTitle = "List tasks"
	getTaskTitle   = "Get task"
	listTasksDesc  = "Lists tasks filtered by project, author, assignee, status, priority and prefix search, newest first. Returns tasks, total, limit and offset. Author and assignee take raw user ids. Does not return comments, attachments or soft-deleted tasks."
	getTaskDesc    = "Returns one task by numeric id or by project slug plus number. Returns the task fields only. Does not return comments or attachments; sibling tools list_task_comments and get_comment read comments."
	tasksPath      = "/tasks"
)

func addTaskTools(
	server *mcpsdk.Server,
	session *clientgo.Session,
	schemas map[string]*jsonschema.Schema,
	opts Options,
) {
	mcpsdk.AddTool(server,
		newTool("list_tasks", listTasksTitle, listTasksDesc, schemas["list_tasks"]),
		listTasksHandler(session, opts),
	)
	mcpsdk.AddTool(server,
		newTool("get_task", getTaskTitle, getTaskDesc, schemas["get_task"]),
		getTaskHandler(session, opts),
	)
}

func listTasksHandler(
	session *clientgo.Session,
	opts Options,
) mcpsdk.ToolHandlerFor[ListTasksIn, ListTasksOut] {
	return func(
		ctx context.Context,
		_ *mcpsdk.CallToolRequest,
		in ListTasksIn,
	) (*mcpsdk.CallToolResult, ListTasksOut, error) {
		if err := ValidateStatuses(in.Statuses); err != nil {
			return nil, ListTasksOut{}, fmt.Errorf("%w; use list_tasks", err)
		}
		if err := ValidatePriorities(in.Priorities); err != nil {
			return nil, ListTasksOut{}, fmt.Errorf("%w; use list_tasks", err)
		}

		search := SanitizeSearch(in.Search)
		if in.Search != "" && search == "" {
			return nil, ListTasksOut{}, fmt.Errorf("%w: %q; use a different search term", ErrEmptySearch, in.Search)
		}

		limit := ClampLimit(in.Limit)
		offset := clampOffset(in.Offset)

		if in.Project != "" {
			if _, err := session.GetProject(ctx, in.Project); err != nil {
				if IsNotFound(err) {
					return nil, ListTasksOut{}, fmt.Errorf("%w: %s; use list_projects", ErrProjectNotFound, in.Project)
				}

				return nil, ListTasksOut{}, MapError(err, "use list_projects", opts.StaticMode)
			}
		}

		query := taskQuery(in, search, limit, offset)

		var list clientgo.TaskListResponse
		if err := session.Do(ctx, http.MethodGet, tasksPath, query, nil, &list); err != nil {
			return nil, ListTasksOut{}, MapError(err, "use list_projects to find a project slug", opts.StaticMode)
		}

		tasks := make([]Task, 0, len(list.Items))
		for _, item := range list.Items {
			tasks = append(tasks, taskFrom(item))
		}

		out := ListTasksOut{
			Tasks:  tasks,
			Total:  list.Total,
			Limit:  limit,
			Offset: offset,
			Note:   pageNote(offset, limit, list.Total),
		}

		return textResult(out), out, nil
	}
}

func taskQuery(in ListTasksIn, search string, limit, offset int) url.Values {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))
	query.Set("sort", NormalizeSort(in.Sort))

	if in.Project != "" {
		query.Set("project", in.Project)
	}
	if in.Author > 0 {
		query.Set("author", strconv.FormatInt(in.Author, 10))
	}
	if in.Assignee != nil {
		query.Set("assignee", strconv.FormatInt(*in.Assignee, 10))
	}
	if len(in.Statuses) > 0 {
		query.Set("statuses", JoinCSV(in.Statuses))
	}
	if len(in.Priorities) > 0 {
		query.Set("priorities", JoinCSV(in.Priorities))
	}
	if search != "" {
		query.Set("search", search)
	}

	return query
}

func getTaskHandler(
	session *clientgo.Session,
	opts Options,
) mcpsdk.ToolHandlerFor[GetTaskIn, TaskResult] {
	return func(
		ctx context.Context,
		_ *mcpsdk.CallToolRequest,
		in GetTaskIn,
	) (*mcpsdk.CallToolResult, TaskResult, error) {
		byID := in.ID > 0
		byNumber := in.Project != "" && in.Number > 0
		if byID == byNumber {
			return nil, TaskResult{}, fmt.Errorf(
				"%w: provide either id or project plus number; use list_tasks",
				ErrInvalidArguments,
			)
		}

		var (
			task *clientgo.TaskResponse
			err  error
			ref  string
		)
		if byID {
			ref = strconv.FormatInt(in.ID, 10)
			task, err = session.GetTaskByID(ctx, in.ID)
		} else {
			ref = in.Project + "/" + strconv.Itoa(in.Number)
			task, err = session.GetTaskByNumber(ctx, in.Project, in.Number)
		}
		if err != nil {
			if IsNotFound(err) {
				return nil, TaskResult{}, fmt.Errorf("%w: %s; use list_tasks", ErrTaskNotFound, ref)
			}

			return nil, TaskResult{}, MapError(err, "use list_tasks", opts.StaticMode)
		}

		out := TaskResult{Task: taskFrom(*task)}

		return textResult(out), out, nil
	}
}
