package mcp_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bit-issues/cli/internal/mcp"
)

const (
	maxToolNameLength = 64
	searchMaxLength   = 255
	defaultLimitValue = 20
	firstPageOffset   = 0
	expectedToolCount = 6
	priorityEnumCount = 5
	statusEnumCount   = 10
	maxOffsetValue    = 100000
)

var allowedSorts = []string{
	"created_at", "-created_at", "priority", "-priority",
	"due_date", "-due_date", "status", "-status",
}

type toolShape struct {
	name string
	in   any
}

func toolShapes() []toolShape {
	return []toolShape{
		{name: "list_projects", in: mcp.ListProjectsIn{}},
		{name: "get_project", in: mcp.GetProjectIn{}},
		{name: "list_tasks", in: mcp.ListTasksIn{}},
		{name: "get_task", in: mcp.GetTaskIn{}},
		{name: "list_task_comments", in: mcp.ListTaskCommentsIn{}},
		{name: "get_comment", in: mcp.GetCommentIn{}},
	}
}

func TestSchemasMirrorInputStructs(t *testing.T) {
	t.Parallel()

	schemas := mcp.Schemas()
	if len(schemas) != expectedToolCount {
		t.Fatalf("schema count = %d, want %d", len(schemas), expectedToolCount)
	}

	for _, shape := range toolShapes() {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()

			schema := schemas[shape.name]
			if schema == nil {
				t.Fatalf("missing schema for %s", shape.name)
			}
			if schema.Type != "object" {
				t.Errorf("%s root type = %q, want object", shape.name, schema.Type)
			}

			want := jsonFieldNames(t, shape.in)
			for name, property := range schema.Properties {
				if _, ok := want[name]; !ok {
					t.Errorf("%s has unexpected property %q", shape.name, name)
				}
				if property.Description == "" {
					t.Errorf("%s.%s has no description", shape.name, name)
				}
			}
			for name := range want {
				if _, ok := schema.Properties[name]; !ok {
					t.Errorf("%s is missing property %q", shape.name, name)
				}
			}
		})
	}
}

func TestArrayEnumsAreOnItems(t *testing.T) {
	t.Parallel()

	schemas := mcp.Schemas()
	listTasks := schemas["list_tasks"]

	statuses := listTasks.Properties["statuses"]
	if len(statuses.Enum) != 0 {
		t.Errorf("statuses.Enum = %v, want empty", statuses.Enum)
	}
	if statuses.Items == nil || len(statuses.Items.Enum) != statusEnumCount {
		t.Fatalf("statuses.Items.Enum = %v, want %d values", statuses.Items, statusEnumCount)
	}
	if !containsValue(statuses.Items.Enum, "In Progress") || !containsValue(statuses.Items.Enum, "On Hold") {
		t.Errorf("statuses items missing spaced values: %v", statuses.Items.Enum)
	}

	priorities := listTasks.Properties["priorities"]
	if len(priorities.Enum) != 0 {
		t.Errorf("priorities.Enum = %v, want empty", priorities.Enum)
	}
	if priorities.Items == nil || len(priorities.Items.Enum) != priorityEnumCount {
		t.Fatalf("priorities.Items.Enum = %v, want %d values", priorities.Items, priorityEnumCount)
	}
}

func TestScalarEnumAndDefaults(t *testing.T) {
	t.Parallel()

	sortSchema := mcp.Schemas()["list_tasks"].Properties["sort"]
	if len(sortSchema.Enum) != len(allowedSorts) {
		t.Fatalf("sort.Enum = %v, want %d values", sortSchema.Enum, len(allowedSorts))
	}
	for _, want := range allowedSorts {
		if !containsValue(sortSchema.Enum, want) {
			t.Errorf("sort.Enum missing %q", want)
		}
	}
	if string(sortSchema.Default) != `"-created_at"` {
		t.Errorf("sort.Default = %s, want \"-created_at\"", sortSchema.Default)
	}

	limit := mcp.Schemas()["list_tasks"].Properties["limit"]
	if string(limit.Default) != "20" {
		t.Errorf("limit.Default = %s, want %d", limit.Default, defaultLimitValue)
	}
	if limit.Minimum == nil || *limit.Minimum != limitLowerBound ||
		limit.Maximum == nil || *limit.Maximum != limitUpperBound {
		t.Errorf("limit bounds = %v..%v, want %d..%d",
			limit.Minimum, limit.Maximum, limitLowerBound, limitUpperBound)
	}

	offset := mcp.Schemas()["list_tasks"].Properties["offset"]
	if string(offset.Default) != "0" {
		t.Errorf("offset.Default = %s, want %d", offset.Default, firstPageOffset)
	}
}

func TestIdentifierPropertiesHaveNoMaximum(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool     string
		property string
	}{
		{tool: "get_task", property: "id"},
		{tool: "get_task", property: "number"},
		{tool: "list_task_comments", property: "task_id"},
		{tool: "get_comment", property: "task_id"},
		{tool: "get_comment", property: "comment_id"},
		{tool: "list_tasks", property: "author"},
		{tool: "list_tasks", property: "assignee"},
	}

	for _, tt := range cases {
		t.Run(tt.tool+"."+tt.property, func(t *testing.T) {
			t.Parallel()

			prop := mcp.Schemas()[tt.tool].Properties[tt.property]
			if prop == nil {
				t.Fatalf("missing property %s.%s", tt.tool, tt.property)
			}
			if prop.Maximum != nil {
				t.Errorf("%s.%s Maximum = %v, want nil", tt.tool, tt.property, *prop.Maximum)
			}
			if prop.Minimum == nil || *prop.Minimum != firstPageOffset {
				t.Errorf("%s.%s Minimum = %v, want %d", tt.tool, tt.property, prop.Minimum, firstPageOffset)
			}
		})
	}
}

func TestPaginationMaximumUnchanged(t *testing.T) {
	t.Parallel()

	for _, tool := range []string{"list_projects", "list_tasks", "list_task_comments"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			offset := mcp.Schemas()[tool].Properties["offset"]
			if offset == nil {
				t.Fatalf("%s missing offset", tool)
			}
			if offset.Maximum == nil || *offset.Maximum != maxOffsetValue {
				t.Errorf("%s.offset Maximum = %v, want %d", tool, offset.Maximum, maxOffsetValue)
			}
		})
	}
}

func TestSearchMaxLength(t *testing.T) {
	t.Parallel()

	search := mcp.Schemas()["list_tasks"].Properties["search"]
	if search.MaxLength == nil || *search.MaxLength != searchMaxLength {
		t.Errorf("search.MaxLength = %v, want %d", search.MaxLength, searchMaxLength)
	}
}

func TestToolNamesWithinLimit(t *testing.T) {
	t.Parallel()

	for name := range mcp.Schemas() {
		if len(name) > maxToolNameLength {
			t.Errorf("tool name %q exceeds %d characters", name, maxToolNameLength)
		}
	}
}

func jsonFieldNames(t *testing.T, value any) map[string]struct{} {
	t.Helper()

	typ := reflect.TypeOf(value)
	names := make(map[string]struct{}, typ.NumField())
	for i := range typ.NumField() {
		tag := typ.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		names[name] = struct{}{}
	}

	return names
}

func containsValue(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}
