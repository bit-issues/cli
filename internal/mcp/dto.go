package mcp

import (
	"strconv"

	clientgo "github.com/bit-issues/client-go"
)

const (
	TruncateLength  = 2000
	truncatedMarker = " [truncated]"
	maxOffset       = 100000
)

// ListProjectsIn is the input of list_projects.
type ListProjectsIn struct {
	Search string `json:"search,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// GetProjectIn is the input of get_project.
type GetProjectIn struct {
	Slug string `json:"slug"`
}

// ListTasksIn is the input of list_tasks.
type ListTasksIn struct {
	Project    string   `json:"project,omitempty"`
	Author     int64    `json:"author,omitempty"`
	Assignee   *int64   `json:"assignee,omitempty"`
	Statuses   []string `json:"statuses,omitempty"`
	Priorities []string `json:"priorities,omitempty"`
	Search     string   `json:"search,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

// GetTaskIn is the input of get_task.
type GetTaskIn struct {
	ID      int64  `json:"id,omitempty"`
	Project string `json:"project,omitempty"`
	Number  int    `json:"number,omitempty"`
}

// ListTaskCommentsIn is the input of list_task_comments.
type ListTaskCommentsIn struct {
	TaskID int64 `json:"task_id"`
	Limit  int   `json:"limit,omitempty"`
	Offset int   `json:"offset,omitempty"`
}

// GetCommentIn is the input of get_comment.
type GetCommentIn struct {
	TaskID    int64 `json:"task_id"`
	CommentID int64 `json:"comment_id"`
}

// UserBrief is the projected author/assignee shape.
type UserBrief struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// Project is the projected project shape.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RepoURL   string `json:"repo_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Task is the projected task shape. Attachments are deliberately absent:
// clientgo.AttachmentResponse carries a presigned download_url that must
// never reach an MCP client (see client-go/dto_attachment.go).
type Task struct {
	ID          int64      `json:"id"`
	ProjectSlug string     `json:"project_slug"`
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	Kind        string     `json:"kind,omitempty"`
	Author      UserBrief  `json:"author"`
	Assignee    *UserBrief `json:"assignee,omitempty"`
	DueDate     *string    `json:"due_date,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// Comment is the projected comment shape; content is plain text.
type Comment struct {
	ID        int64     `json:"id"`
	Author    UserBrief `json:"author"`
	Content   string    `json:"content"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

// ListProjectsOut is the output of list_projects.
type ListProjectsOut struct {
	Projects []Project `json:"projects"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	Note     string    `json:"note,omitempty"`
}

// ProjectResult is the output of get_project.
type ProjectResult struct {
	Project Project `json:"project"`
}

// ListTasksOut is the output of list_tasks.
type ListTasksOut struct {
	Tasks  []Task `json:"tasks"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Note   string `json:"note,omitempty"`
}

// TaskResult is the output of get_task.
type TaskResult struct {
	Task Task `json:"task"`
}

// ListCommentsOut is the output of list_task_comments.
type ListCommentsOut struct {
	Comments []Comment `json:"comments"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	Note     string    `json:"note,omitempty"`
}

// CommentResult is the output of get_comment.
type CommentResult struct {
	Comment Comment `json:"comment"`
}

// ProjectTaskFromDetails projects the base task fields and drops the project,
// attachments and comments of the details payload.
func ProjectTaskFromDetails(details *clientgo.TaskDetailsResponse) Task {
	return taskFrom(details.TaskResponse)
}

// ProjectComment projects a comment, truncating long plain-text content.
func ProjectComment(source clientgo.CommentResponse) Comment {
	return commentFrom(source)
}

func projectFrom(source clientgo.Project) Project {
	return Project{
		ID:        source.ID,
		Name:      source.Name,
		RepoURL:   source.RepoURL,
		CreatedAt: source.CreatedAt,
		UpdatedAt: source.UpdatedAt,
	}
}

func userBriefFrom(source clientgo.UserBrief) UserBrief {
	return UserBrief{
		ID:        source.ID,
		Name:      source.Name,
		Role:      source.Role,
		CreatedAt: source.CreatedAt,
	}
}

func taskFrom(source clientgo.TaskResponse) Task {
	return Task{
		ID:          source.ID,
		ProjectSlug: source.ProjectSlug,
		Number:      source.Number,
		Title:       source.Title,
		Description: truncateText(source.Description),
		Priority:    source.Priority,
		Status:      source.Status,
		Kind:        source.Kind,
		Author:      userBriefFrom(source.Author),
		Assignee:    userBriefFromPtr(source.Assignee),
		DueDate:     source.DueDate,
		CreatedAt:   source.CreatedAt,
		UpdatedAt:   source.UpdatedAt,
	}
}

func userBriefFromPtr(source *clientgo.UserBrief) *UserBrief {
	if source == nil {
		return nil
	}

	brief := userBriefFrom(*source)

	return &brief
}

func commentFrom(source clientgo.CommentResponse) Comment {
	return Comment{
		ID:        source.ID,
		Author:    userBriefFrom(source.Author),
		Content:   truncateText(source.Content),
		CreatedAt: source.CreatedAt,
		UpdatedAt: source.UpdatedAt,
	}
}

func truncateText(value string) string {
	runes := []rune(value)
	if len(runes) <= TruncateLength {
		return value
	}

	return string(runes[:TruncateLength]) + truncatedMarker
}

func pageNote(offset, limit, total int) string {
	if offset+limit >= total {
		return ""
	}

	return "Showing " + strconv.Itoa(offset+limit) + " of " + strconv.Itoa(total) + "; use offset to page"
}
