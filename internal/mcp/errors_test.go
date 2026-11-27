package mcp_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/bit-issues/cli/internal/mcp"
	clientgo "github.com/bit-issues/client-go"
	restkit "github.com/capcom6/go-restkit"
)

const (
	secretBodyMarker = "SECRET_UPSTREAM_BODY"
	longTextLength   = mcp.TruncateLength + 64
)

func TestMapErrorSessionExpired(t *testing.T) {
	t.Parallel()

	err := mcp.MapError(clientgo.ErrSessionExpired, "run 'bitissues-cli auth login'", false)
	if !errors.Is(err, clientgo.ErrSessionExpired) {
		t.Errorf("error = %v, want ErrSessionExpired", err)
	}
	if !strings.Contains(err.Error(), "log in again") || !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error lacks recovery hint: %v", err)
	}
}

func TestMapErrorNetworkBlipIsNotSessionExpired(t *testing.T) {
	t.Parallel()

	err := mcp.MapError(
		&restkit.InfrastructureError{Err: errors.New("dial failed"), URL: "http://backend"},
		"use list_tasks",
		false,
	)
	if errors.Is(err, clientgo.ErrSessionExpired) {
		t.Errorf("network blip mapped to session expired: %v", err)
	}
	if !errors.Is(err, mcp.ErrBackendUnreachable) {
		t.Errorf("error = %v, want ErrBackendUnreachable", err)
	}
}

func TestMapErrorByStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		staticMode bool
		want       error
		contains   string
	}{
		{
			name:     "bad request",
			status:   http.StatusBadRequest,
			want:     mcp.ErrInvalidRequest,
			contains: "check the tool arguments",
		},
		{
			name:     "unauthorized file mode",
			status:   http.StatusUnauthorized,
			want:     mcp.ErrNotAuthenticated,
			contains: "auth login",
		},
		{
			name:       "unauthorized static mode",
			status:     http.StatusUnauthorized,
			staticMode: true,
			want:       mcp.ErrNotAuthenticated,
			contains:   "BITISSUES_TOKEN",
		},
		{name: "forbidden", status: http.StatusForbidden, want: mcp.ErrForbidden, contains: "forbidden"},
		{
			name:     "server error",
			status:   http.StatusInternalServerError,
			want:     mcp.ErrBackendError,
			contains: "backend error",
		},
		{name: "gateway error", status: http.StatusBadGateway, want: mcp.ErrBackendError, contains: "backend error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cause := &restkit.APIError{
				StatusCode: tt.status,
				URL:        "http://backend/api/v1/tasks",
				Body:       []byte(secretBodyMarker),
			}
			err := mcp.MapError(cause, "run 'bitissues-cli auth login'", tt.staticMode)

			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("error = %v, want it to contain %q", err, tt.contains)
			}
			if strings.Contains(err.Error(), secretBodyMarker) {
				t.Errorf("error leaked upstream body: %v", err)
			}
			if errors.Is(err, clientgo.ErrSessionExpired) {
				t.Errorf("status %d mapped to session expired", tt.status)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	if !mcp.IsNotFound(&restkit.APIError{StatusCode: http.StatusNotFound}) {
		t.Error("404 not recognized")
	}
	if mcp.IsNotFound(&restkit.APIError{StatusCode: http.StatusForbidden}) {
		t.Error("403 recognized as not found")
	}
}

func TestPresignedURLsNeverProjected(t *testing.T) {
	t.Parallel()

	details := &clientgo.TaskDetailsResponse{
		TaskResponse: clientgo.TaskResponse{ID: 1, Title: "task"},
		Project:      &clientgo.Project{ID: "demo"},
		Attachments: []clientgo.AttachmentResponse{
			{ID: 2, FileName: "file", DownloadURL: "https://presigned.example/secret"},
		},
		Comments: []clientgo.CommentResponse{
			{ID: 3, Content: "said https://presigned.example/secret"},
		},
	}

	data, err := json.Marshal(mcp.ProjectTaskFromDetails(details))
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	for _, forbidden := range []string{"download_url", "upload_url", "presigned"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("projected task contains %q: %s", forbidden, data)
		}
	}

	data, err = json.Marshal(mcp.ProjectComment(details.Comments[0]))
	if err != nil {
		t.Fatalf("marshal comment: %v", err)
	}
	if strings.Contains(string(data), "download_url") || strings.Contains(string(data), "upload_url") {
		t.Errorf("projected comment contains presigned URL: %s", data)
	}
}

func TestLongContentIsTruncated(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", longTextLength)
	task := mcp.ProjectTaskFromDetails(&clientgo.TaskDetailsResponse{
		TaskResponse: clientgo.TaskResponse{ID: 1, Description: long},
	})

	if !strings.Contains(task.Description, "[truncated]") {
		t.Errorf("description not truncated: %q", task.Description)
	}
	if len([]rune(task.Description)) >= longTextLength {
		t.Errorf("description not bounded: %d runes", len([]rune(task.Description)))
	}

	comment := mcp.ProjectComment(clientgo.CommentResponse{ID: 1, Content: long})
	if !strings.Contains(comment.Content, "[truncated]") {
		t.Errorf("comment not truncated: %q", comment.Content)
	}
}
