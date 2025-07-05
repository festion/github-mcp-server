package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GraphQLError represents a GraphQL error response
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// GraphQLErrors represents a collection of GraphQL errors
type GraphQLErrors struct {
	Errors []GraphQLError `json:"errors"`
}

// Error implements the error interface
func (e GraphQLErrors) Error() string {
	if len(e.Errors) == 0 {
		return "unknown GraphQL error"
	}
	
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Message)
	}
	return strings.Join(messages, "; ")
}

// ParseGraphQLError extracts structured error information from a GraphQL error
func ParseGraphQLError(err error) (*GraphQLErrors, bool) {
	if err == nil {
		return nil, false
	}
	
	// Try to parse as JSON first
	var gqlErrors GraphQLErrors
	if jsonErr := json.Unmarshal([]byte(err.Error()), &gqlErrors); jsonErr == nil && len(gqlErrors.Errors) > 0 {
		return &gqlErrors, true
	}
	
	// Check if the error message contains GraphQL error patterns
	errMsg := err.Error()
	if strings.Contains(errMsg, "Could not resolve to") || 
	   strings.Contains(errMsg, "was not found") ||
	   strings.Contains(errMsg, "must be a member") ||
	   strings.Contains(errMsg, "insufficient scopes") {
		return &GraphQLErrors{
			Errors: []GraphQLError{
				{Message: errMsg},
			},
		}, true
	}
	
	return nil, false
}

// FormatGraphQLError formats a GraphQL error for user display
func FormatGraphQLError(err error) string {
	gqlErr, ok := ParseGraphQLError(err)
	if !ok {
		return err.Error()
	}
	
	if len(gqlErr.Errors) == 1 {
		return formatSingleError(gqlErr.Errors[0])
	}
	
	var formatted []string
	for i, e := range gqlErr.Errors {
		formatted = append(formatted, fmt.Sprintf("%d. %s", i+1, formatSingleError(e)))
	}
	return "Multiple errors:\n" + strings.Join(formatted, "\n")
}

func formatSingleError(err GraphQLError) string {
	msg := err.Message
	
	// Add path information if available
	if len(err.Path) > 0 {
		pathStr := make([]string, len(err.Path))
		for i, p := range err.Path {
			pathStr[i] = fmt.Sprintf("%v", p)
		}
		msg += fmt.Sprintf(" (at path: %s)", strings.Join(pathStr, "."))
	}
	
	// Add helpful context based on error type
	if strings.Contains(msg, "Could not resolve to a node with the global id") {
		msg += "\nThis usually means the item was deleted or you don't have permission to access it."
	} else if strings.Contains(msg, "must be a member of the") {
		msg += "\nYou need to be a member of the organization to perform this action."
	} else if strings.Contains(msg, "insufficient scopes") {
		msg += "\nYour GitHub token needs additional permissions. Check the required scopes in the documentation."
	} else if strings.Contains(msg, "was not found") {
		msg += "\nVerify that the ID is correct and that you have access to this resource."
	}
	
	return msg
}

// HandleNotFoundError provides user-friendly messages for not found errors
func HandleNotFoundError(resourceType string, id string) string {
	suggestions := map[string]string{
		"project": "Use 'list_project_boards' to find available projects.",
		"column": "Use 'list_project_columns' to find available columns.",
		"card": "Use 'list_project_cards' to find available cards.",
		"field": "Use 'get_project_board' with include_fields=true to see available fields.",
	}
	
	msg := fmt.Sprintf("%s with ID '%s' not found.", strings.Title(resourceType), id)
	if suggestion, ok := suggestions[resourceType]; ok {
		msg += " " + suggestion
	}
	return msg
}

// HandlePermissionError provides user-friendly messages for permission errors
func HandlePermissionError(action string, resourceType string) string {
	return fmt.Sprintf("Permission denied: Cannot %s %s. Ensure your GitHub token has the 'project' scope and you have appropriate access to this %s.", 
		action, resourceType, resourceType)
}

// ValidateProjectID checks if a project ID has the correct format
func ValidateProjectID(id string) error {
	if !strings.HasPrefix(id, "PVT_") {
		return fmt.Errorf("invalid project ID format: '%s'. Project IDs should start with 'PVT_' (e.g., 'PVT_kwDOAM6J184ACzDx')", id)
	}
	return nil
}

// ValidateColumnID checks if a column ID has the correct format
func ValidateColumnID(id string) error {
	if !strings.HasPrefix(id, "PVTFSC_") && !strings.HasPrefix(id, "PVTSSF_") {
		return fmt.Errorf("invalid column ID format: '%s'. Column IDs should start with 'PVTFSC_' or 'PVTSSF_'", id)
	}
	return nil
}

// ValidateItemID checks if an item/card ID has the correct format
func ValidateItemID(id string) error {
	if !strings.HasPrefix(id, "PVTI_") {
		return fmt.Errorf("invalid item ID format: '%s'. Item IDs should start with 'PVTI_'", id)
	}
	return nil
}