package github

import (
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

func Test_ProjectBoardTools(t *testing.T) {
	translate := translations.NullTranslationHelper

	// Test tool definitions match snapshots
	tests := []struct {
		name string
		getToolDef func(GetGQLClientFn, translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc)
	}{
		{"create_project_board", CreateProjectBoard},
		{"update_project_board", UpdateProjectBoard},
		{"delete_project_board", DeleteProjectBoard},
		{"list_project_boards", ListProjectBoards},
		{"get_project_board", GetProjectBoard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify tool definition matches snapshot
			tool, _ := tt.getToolDef(nil, translate)
			require.NoError(t, toolsnaps.Test(tool.Name, tool))
		})
	}
}

func Test_ProjectColumnTools(t *testing.T) {
	translate := translations.NullTranslationHelper

	// Test tool definitions match snapshots
	tests := []struct {
		name string
		getToolDef func(GetGQLClientFn, translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc)
	}{
		{"create_project_column", CreateProjectColumn},
		{"update_project_column", UpdateProjectColumn},
		{"delete_project_column", DeleteProjectColumn},
		{"reorder_project_columns", ReorderProjectColumns},
		{"list_project_columns", ListProjectColumns},
		{"get_project_column", GetProjectColumn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify tool definition matches snapshot
			tool, _ := tt.getToolDef(nil, translate)
			require.NoError(t, toolsnaps.Test(tool.Name, tool))
		})
	}
}

func Test_ProjectCardTools(t *testing.T) {
	translate := translations.NullTranslationHelper

	// Test tool definitions match snapshots
	tests := []struct {
		name string
		getToolDef func(GetGQLClientFn, translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc)
	}{
		{"add_card_to_project", AddCardToProjectSimple},
		{"move_project_card", MoveProjectCardSimple},
		{"update_project_card", UpdateProjectCardSimple},
		{"remove_card_from_project", RemoveCardFromProjectSimple},
		{"bulk_move_cards", BulkMoveCardsSimple},
		{"list_project_cards", ListProjectCards},
		{"get_project_card", GetProjectCard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify tool definition matches snapshot
			tool, _ := tt.getToolDef(nil, translate)
			require.NoError(t, toolsnaps.Test(tool.Name, tool))
		})
	}
}