package mcp

import (
	"encoding/json"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
)

const (
	typeObject  = "object"
	typeString  = "string"
	typeArray   = "array"
	typeInteger = "integer"
	propLimit   = "limit"
	propOffset  = "offset"
	propTaskID  = "task_id"
	propSearch  = "search"

	descLimit      = "Maximum number of items to return (1-100, default 20)."
	descOffset     = "Number of items to skip before the page (default 0)."
	descSearch     = "Full-text, AND-conjunctive prefix search; stripped operators are removed, empty after sanitizing is rejected, max 255 characters."
	descProject    = "Project slug; when supplied the project is checked first."
	descAuthor     = "Raw author user id, visible only in earlier tool output; there is no user-search tool."
	descAssignee   = "Raw assignee user id; 0 means unassigned only, omit to skip; visible only in earlier tool output."
	descSort       = "Sort key, optional leading '-' means descending; default -created_at (newest first)."
	descStatuses   = "Case-sensitive status values; valid multi-value filter, joined with commas."
	descPriorities = "Case-sensitive priority values; valid multi-value filter, joined with commas."
	descTaskID     = "Numeric task id."
	descCommentID  = "Numeric comment id within the task."
	descSlug       = "Project slug (the project id on the wire)."
	descCommentsID = "Numeric id of the task whose comments are read."
	descGetTaskID  = "Numeric task id; mutually exclusive with project plus number."
	descGetProject = "Project slug for a lookup by task number; requires number."
	descGetNumber  = "Task number within the project; requires project."
	descProjSearch = "Project name prefix search; max 255 characters after sanitizing."

	searchMaxLength = 255
)

// Schemas returns freshly built hand-written input schemas for all six tools.
// jsonschema-go does not infer enums, so every enum is set explicitly here;
// array enums live on the items schema and scalar enums on the property.
func Schemas() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"list_projects": objectSchema(map[string]*jsonschema.Schema{
			propSearch: stringProperty(descProjSearch),
			propLimit:  intProperty(descLimit, defaultLimit, 1, maxLimit),
			propOffset: intProperty(descOffset, 0, 0, maxOffset),
		}),
		"get_project": objectSchema(map[string]*jsonschema.Schema{
			"slug": stringProperty(descSlug),
		}, "slug"),
		"list_tasks": listTasksSchema(),
		"get_task": objectSchema(map[string]*jsonschema.Schema{
			"id":      identifierProperty(descGetTaskID),
			"project": stringProperty(descGetProject),
			"number":  identifierProperty(descGetNumber),
		}),
		"list_task_comments": objectSchema(map[string]*jsonschema.Schema{
			propTaskID: identifierProperty(descCommentsID),
			propLimit:  intProperty(descLimit, defaultLimit, 1, maxLimit),
			propOffset: intProperty(descOffset, 0, 0, maxOffset),
		}, propTaskID),
		"get_comment": objectSchema(map[string]*jsonschema.Schema{
			propTaskID:   identifierProperty(descTaskID),
			"comment_id": identifierProperty(descCommentID),
		}, propTaskID, "comment_id"),
	}
}

func listTasksSchema() *jsonschema.Schema {
	statuses := []string{
		"New", "Open", "In Progress", "Resolved", "Closed",
		"Reopened", "Invalid", "Duplicate", "Wontfix", "On Hold",
	}
	priorities := []string{"Trivial", "Minor", "Major", "Critical", "Blocker"}
	sorts := []string{
		"created_at", "-created_at", "priority", "-priority",
		"due_date", "-due_date", "status", "-status",
	}

	return objectSchema(map[string]*jsonschema.Schema{
		"project":    stringProperty(descProject),
		"author":     identifierProperty(descAuthor),
		"assignee":   identifierProperty(descAssignee),
		"statuses":   enumArrayProperty(descStatuses, statuses),
		"priorities": enumArrayProperty(descPriorities, priorities),
		propSearch:   stringProperty(descSearch),
		"sort":       enumProperty(descSort, sorts, json.RawMessage(`"-created_at"`)),
		propLimit:    intProperty(descLimit, defaultLimit, 1, maxLimit),
		propOffset:   intProperty(descOffset, 0, 0, maxOffset),
	})
}

func objectSchema(properties map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	var schema jsonschema.Schema
	schema.Type = typeObject
	schema.Properties = properties
	if len(required) > 0 {
		schema.Required = required
	}

	return &schema
}

func stringProperty(description string) *jsonschema.Schema {
	var schema jsonschema.Schema
	schema.Type = typeString
	schema.Description = description
	maxLength := searchMaxLength
	schema.MaxLength = &maxLength

	return &schema
}

func intProperty(description string, defaultValue, minimum, maximum int) *jsonschema.Schema {
	var schema jsonschema.Schema
	schema.Type = typeInteger
	schema.Description = description
	schema.Default = json.RawMessage(strconv.Itoa(defaultValue))
	minValue := float64(minimum)
	maxValue := float64(maximum)
	schema.Minimum = &minValue
	schema.Maximum = &maxValue

	return &schema
}

func identifierProperty(description string) *jsonschema.Schema {
	var schema jsonschema.Schema
	schema.Type = typeInteger
	schema.Description = description
	minValue := float64(0)
	schema.Minimum = &minValue

	return &schema
}

func enumProperty(description string, values []string, defaultValue json.RawMessage) *jsonschema.Schema {
	var schema jsonschema.Schema
	schema.Type = typeString
	schema.Description = description
	schema.Enum = anyValues(values)
	schema.Default = defaultValue

	return &schema
}

func enumArrayProperty(description string, values []string) *jsonschema.Schema {
	var schema jsonschema.Schema
	schema.Type = typeArray
	schema.Description = description
	var items jsonschema.Schema
	items.Type = typeString
	items.Enum = anyValues(values)
	schema.Items = &items

	return &schema
}

func anyValues(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}

	return result
}
