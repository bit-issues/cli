package mcp

import (
	"context"
	"fmt"
	"strconv"

	clientgo "github.com/bit-issues/client-go"
	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	listCommentsTitle = "List task comments"
	getCommentTitle   = "Get comment"
	listCommentsDesc  = "Lists the comments of a task with pagination over the embedded comment list. Returns comments, total, limit and offset. Content is plain text. Does not return attachments or task fields."
	getCommentDesc    = "Returns one comment of a task by task id and comment id. Returns the comment record. Does not return the other comments or task fields; sibling tool list_task_comments lists them."
)

func addCommentTools(
	server *mcpsdk.Server,
	session *clientgo.Session,
	schemas map[string]*jsonschema.Schema,
	opts Options,
) {
	mcpsdk.AddTool(server,
		newTool("list_task_comments", listCommentsTitle, listCommentsDesc, schemas["list_task_comments"]),
		listCommentsHandler(session, opts),
	)
	mcpsdk.AddTool(server,
		newTool("get_comment", getCommentTitle, getCommentDesc, schemas["get_comment"]),
		getCommentHandler(session, opts),
	)
}

func listCommentsHandler(
	session *clientgo.Session,
	opts Options,
) mcpsdk.ToolHandlerFor[ListTaskCommentsIn, ListCommentsOut] {
	return func(
		ctx context.Context,
		_ *mcpsdk.CallToolRequest,
		in ListTaskCommentsIn,
	) (*mcpsdk.CallToolResult, ListCommentsOut, error) {
		details, err := session.GetTaskDetails(ctx, in.TaskID)
		if err != nil {
			return nil, ListCommentsOut{}, taskDetailsError(err, in.TaskID, opts)
		}

		limit := ClampLimit(in.Limit)
		offset := clampOffset(in.Offset)
		total := len(details.Comments)
		start := min(offset, total)
		end := min(start+limit, total)

		comments := make([]Comment, 0, end-start)
		for _, item := range details.Comments[start:end] {
			comments = append(comments, commentFrom(item))
		}

		out := ListCommentsOut{
			Comments: comments,
			Total:    total,
			Limit:    limit,
			Offset:   offset,
			Note:     pageNote(offset, limit, total),
		}

		return textResult(out), out, nil
	}
}

func getCommentHandler(
	session *clientgo.Session,
	opts Options,
) mcpsdk.ToolHandlerFor[GetCommentIn, CommentResult] {
	return func(
		ctx context.Context,
		_ *mcpsdk.CallToolRequest,
		in GetCommentIn,
	) (*mcpsdk.CallToolResult, CommentResult, error) {
		details, err := session.GetTaskDetails(ctx, in.TaskID)
		if err != nil {
			return nil, CommentResult{}, taskDetailsError(err, in.TaskID, opts)
		}

		for _, item := range details.Comments {
			if item.ID == in.CommentID {
				out := CommentResult{Comment: commentFrom(item)}

				return textResult(out), out, nil
			}
		}

		return nil, CommentResult{}, fmt.Errorf(
			"%w: %s; use list_task_comments",
			ErrCommentNotFound,
			strconv.FormatInt(in.CommentID, 10),
		)
	}
}

func taskDetailsError(err error, taskID int64, opts Options) error {
	if IsNotFound(err) {
		return fmt.Errorf("%w: %s; use list_tasks", ErrTaskNotFound, strconv.FormatInt(taskID, 10))
	}

	return MapError(err, "use list_tasks", opts.StaticMode)
}
