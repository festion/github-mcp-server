package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

// MoveProjectCard moves a card to a different column with proper GraphQL implementation
func MoveProjectCard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("move_project_card",
			mcp.WithDescription(t("TOOL_MOVE_PROJECT_CARD_DESCRIPTION", "Move a project card to a different column")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_MOVE_PROJECT_CARD_USER_TITLE", "Move project card"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("card_id",
				mcp.Required(),
				mcp.Description("ID of the card to move"),
			),
			mcp.WithString("column_id",
				mcp.Required(),
				mcp.Description("ID of the target column"),
			),
			mcp.WithString("position",
				mcp.Description("Position in column (top, bottom)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get GraphQL client: %v", err)), nil
			}

			cardID, err := RequiredParam[string](request, "card_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			columnID, err := RequiredParam[string](request, "column_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// First, get the column information to find the project and field IDs
			var columnQuery struct {
				Node struct {
					ProjectV2SingleSelectFieldOption struct {
						ID        githubv4.String
						Name      githubv4.String
						Field struct {
							ID githubv4.String
							Project struct {
								ID githubv4.String
							} `graphql:"project"`
						} `graphql:"field"`
					} `graphql:"... on ProjectV2SingleSelectFieldOption"`
				} `graphql:"node(id: $columnId)"`
			}

			err = client.Query(ctx, &columnQuery, map[string]interface{}{
				"columnId": githubv4.ID(columnID),
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to query column: %v", err)), nil
			}

			fieldID := string(columnQuery.Node.ProjectV2SingleSelectFieldOption.Field.ID)
			projectID := string(columnQuery.Node.ProjectV2SingleSelectFieldOption.Field.Project.ID)

			// Use updateProjectV2ItemFieldValue mutation to update the Status field
			var mutation struct {
				UpdateProjectV2ItemFieldValue struct {
					ClientMutationID githubv4.String
					ProjectV2Item struct {
						ID githubv4.String
						FieldValues struct {
							Nodes []struct {
								Field struct {
									ID   githubv4.String
									Name githubv4.String
								} `graphql:"... on ProjectV2FieldCommon"`
								Value struct {
									SingleSelectFieldOption struct {
										ID   githubv4.String
										Name githubv4.String
									} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
								} `graphql:"... on ProjectV2ItemFieldValueCommon"`
							}
						} `graphql:"fieldValues(first: 10)"`
					}
				} `graphql:"updateProjectV2ItemFieldValue(input: $input)"`
			}

			// Build the input
			singleSelectOptionID := githubv4.String(columnID)
			input := map[string]interface{}{
				"projectId": githubv4.ID(projectID),
				"itemId":    githubv4.ID(cardID),
				"fieldId":   githubv4.ID(fieldID),
				"value": map[string]interface{}{
					"singleSelectOptionId": &singleSelectOptionID,
				},
			}

			err = client.Mutate(ctx, &mutation, input, nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to move card: %v", err)), nil
			}

			// Get the updated status field
			var updatedColumn string
			for _, field := range mutation.UpdateProjectV2ItemFieldValue.ProjectV2Item.FieldValues.Nodes {
				if string(field.Field.ID) == fieldID {
					updatedColumn = string(field.Value.SingleSelectFieldOption.Name)
					break
				}
			}

			result := map[string]interface{}{
				"success":     true,
				"card_id":     cardID,
				"column_id":   columnID,
				"column_name": updatedColumn,
				"project_id":  projectID,
				"message":     "Card moved successfully",
			}

			marshalled, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(marshalled)), nil
		}
}

// UpdateProjectCard updates card properties including custom fields
func UpdateProjectCard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("update_project_card",
			mcp.WithDescription(t("TOOL_UPDATE_PROJECT_CARD_DESCRIPTION", "Update properties and custom fields of a project card")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_PROJECT_CARD_USER_TITLE", "Update project card"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("card_id",
				mcp.Required(),
				mcp.Description("ID of the card to update"),
			),
			mcp.WithObject("fields",
				mcp.Description("Custom field values to update"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get GraphQL client: %v", err)), nil
			}

			cardID, err := RequiredParam[string](request, "card_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			fields, _ := OptionalParam[map[string]interface{}](request, "fields")

			// First, get the item information to find the project ID
			var itemQuery struct {
				Node struct {
					ProjectV2Item struct {
						ID      githubv4.String
						Project struct {
							ID     githubv4.String
							Fields struct {
								Nodes []struct {
									ProjectV2Field struct {
										ID   githubv4.String
										Name githubv4.String
									} `graphql:"... on ProjectV2Field"`
									ProjectV2SingleSelectField struct {
										ID   githubv4.String
										Name githubv4.String
									} `graphql:"... on ProjectV2SingleSelectField"`
								}
							} `graphql:"fields(first: 20)"`
						}
					} `graphql:"... on ProjectV2Item"`
				} `graphql:"node(id: $itemId)"`
			}

			err = client.Query(ctx, &itemQuery, map[string]interface{}{
				"itemId": githubv4.ID(cardID),
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to query item: %v", err)), nil
			}

			projectID := string(itemQuery.Node.ProjectV2Item.Project.ID)

			// Update each field
			var updates []map[string]interface{}
			for fieldName, value := range fields {
				// Find the field ID by name
				var fieldID string
				for _, field := range itemQuery.Node.ProjectV2Item.Project.Fields.Nodes {
					if string(field.ProjectV2Field.Name) == fieldName || string(field.ProjectV2SingleSelectField.Name) == fieldName {
						if field.ProjectV2Field.ID != "" {
							fieldID = string(field.ProjectV2Field.ID)
						} else {
							fieldID = string(field.ProjectV2SingleSelectField.ID)
						}
						break
					}
				}

				if fieldID == "" {
					continue // Skip unknown fields
				}

				// Execute the mutation for this field
				var mutation struct {
					UpdateProjectV2ItemFieldValue struct {
						ClientMutationID githubv4.String
					} `graphql:"updateProjectV2ItemFieldValue(input: $input)"`
				}

				input := map[string]interface{}{
					"projectId": githubv4.ID(projectID),
					"itemId":    githubv4.ID(cardID),
					"fieldId":   githubv4.ID(fieldID),
				}

				// Handle different value types
				switch v := value.(type) {
				case string:
					input["value"] = map[string]interface{}{
						"text": githubv4.String(v),
					}
				case float64:
					input["value"] = map[string]interface{}{
						"number": githubv4.Float(v),
					}
				case bool:
					// For checkboxes
					input["value"] = map[string]interface{}{
						"text": githubv4.String(fmt.Sprintf("%v", v)),
					}
				default:
					// For complex values, try to use as-is
					input["value"] = v
				}

				err = client.Mutate(ctx, &mutation, input, nil)
				if err == nil {
					updates = append(updates, map[string]interface{}{
						"field": fieldName,
						"value": value,
						"status": "updated",
					})
				} else {
					updates = append(updates, map[string]interface{}{
						"field": fieldName,
						"value": value,
						"status": "failed",
						"error": err.Error(),
					})
				}
			}

			result := map[string]interface{}{
				"success":    true,
				"card_id":    cardID,
				"project_id": projectID,
				"updates":    updates,
				"message":    fmt.Sprintf("Updated %d fields", len(updates)),
			}

			marshalled, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(marshalled)), nil
		}
}

// RemoveCardFromProject removes or archives a card from a project
func RemoveCardFromProject(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("remove_card_from_project",
			mcp.WithDescription(t("TOOL_REMOVE_CARD_FROM_PROJECT_DESCRIPTION", "Remove a card from a project board or archive it")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_REMOVE_CARD_FROM_PROJECT_USER_TITLE", "Remove card from project"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("card_id",
				mcp.Required(),
				mcp.Description("ID of the card to remove"),
			),
			mcp.WithBoolean("archive",
				mcp.Description("Archive the card instead of removing"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get GraphQL client: %v", err)), nil
			}

			cardID, err := RequiredParam[string](request, "card_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			archive, _ := OptionalParam[bool](request, "archive")

			if archive {
				// Archive the item
				var mutation struct {
					ArchiveProjectV2Item struct {
						ClientMutationID githubv4.String
						Item struct {
							ID        githubv4.String
							IsArchived githubv4.Boolean
						}
					} `graphql:"archiveProjectV2Item(input: $input)"`
				}

				input := map[string]interface{}{
					"itemId": githubv4.ID(cardID),
				}

				err = client.Mutate(ctx, &mutation, input, nil)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to archive card: %v", err)), nil
				}

				result := map[string]interface{}{
					"success":     true,
					"card_id":     cardID,
					"archived":    true,
					"message":     "Card archived successfully",
				}

				marshalled, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
				}

				return mcp.NewToolResultText(string(marshalled)), nil
			} else {
				// Delete the item
				var mutation struct {
					DeleteProjectV2Item struct {
						ClientMutationID githubv4.String
						DeletedItemId   githubv4.String
					} `graphql:"deleteProjectV2Item(input: $input)"`
				}

				input := map[string]interface{}{
					"itemId": githubv4.ID(cardID),
				}

				err = client.Mutate(ctx, &mutation, input, nil)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to delete card: %v", err)), nil
				}

				result := map[string]interface{}{
					"success":     true,
					"card_id":     string(mutation.DeleteProjectV2Item.DeletedItemId),
					"deleted":    true,
					"message":     "Card removed from project successfully",
				}

				marshalled, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
				}

				return mcp.NewToolResultText(string(marshalled)), nil
			}
		}
}

// BulkMoveCards moves multiple cards to a column
func BulkMoveCards(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("bulk_move_cards",
			mcp.WithDescription(t("TOOL_BULK_MOVE_CARDS_DESCRIPTION", "Move multiple cards between columns in bulk")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_BULK_MOVE_CARDS_USER_TITLE", "Bulk move cards"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithArray("card_ids",
				mcp.Required(),
				mcp.Description("Array of card IDs to move"),
			),
			mcp.WithString("target_column_id",
				mcp.Required(),
				mcp.Description("ID of the target column"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get GraphQL client: %v", err)), nil
			}

			cardIDs, err := OptionalStringArrayParam(request, "card_ids")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(cardIDs) == 0 {
				return mcp.NewToolResultError("card_ids array cannot be empty"), nil
			}

			columnID, err := RequiredParam[string](request, "target_column_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// First, get the column information to find the project and field IDs
			var columnQuery struct {
				Node struct {
					ProjectV2SingleSelectFieldOption struct {
						ID        githubv4.String
						Name      githubv4.String
						Field struct {
							ID githubv4.String
							Project struct {
								ID githubv4.String
							} `graphql:"project"`
						} `graphql:"field"`
					} `graphql:"... on ProjectV2SingleSelectFieldOption"`
				} `graphql:"node(id: $columnId)"`
			}

			err = client.Query(ctx, &columnQuery, map[string]interface{}{
				"columnId": githubv4.ID(columnID),
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to query column: %v", err)), nil
			}

			fieldID := string(columnQuery.Node.ProjectV2SingleSelectFieldOption.Field.ID)
			projectID := string(columnQuery.Node.ProjectV2SingleSelectFieldOption.Field.Project.ID)

			// Move each card
			var results []map[string]interface{}
			singleSelectOptionID := githubv4.String(columnID)

			for _, cardID := range cardIDs {
				var mutation struct {
					UpdateProjectV2ItemFieldValue struct {
						ClientMutationID githubv4.String
						ProjectV2Item struct {
							ID githubv4.String
						}
					} `graphql:"updateProjectV2ItemFieldValue(input: $input)"`
				}

				input := map[string]interface{}{
					"projectId": githubv4.ID(projectID),
					"itemId":    githubv4.ID(cardID),
					"fieldId":   githubv4.ID(fieldID),
					"value": map[string]interface{}{
						"singleSelectOptionId": &singleSelectOptionID,
					},
				}

				err = client.Mutate(ctx, &mutation, input, nil)
				if err != nil {
					results = append(results, map[string]interface{}{
						"card_id": cardID,
						"status":  "failed",
						"error":   err.Error(),
					})
				} else {
					results = append(results, map[string]interface{}{
						"card_id": cardID,
						"status":  "moved",
					})
				}
			}

			// Count successes
			successCount := 0
			for _, r := range results {
				if r["status"] == "moved" {
					successCount++
				}
			}

			result := map[string]interface{}{
				"success":          successCount > 0,
				"total_cards":      len(cardIDs),
				"moved_count":      successCount,
				"failed_count":     len(cardIDs) - successCount,
				"target_column_id": columnID,
				"results":          results,
				"message":          fmt.Sprintf("Moved %d of %d cards successfully", successCount, len(cardIDs)),
			}

			marshalled, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
			}

			return mcp.NewToolResultText(string(marshalled)), nil
		}
}