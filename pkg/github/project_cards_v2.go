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

// ListProjectCards creates a tool to list cards in a project with full GraphQL support
func ListProjectCards(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_project_cards",
			mcp.WithDescription(t("TOOL_LIST_PROJECT_CARDS_DESCRIPTION", "List cards in a project board with filtering options")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PROJECT_CARDS_USER_TITLE", "List project cards"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("board_id",
				mcp.Required(),
				mcp.Description("ID of the project board"),
			),
			mcp.WithString("column_id",
				mcp.Description("Filter by specific column"),
			),
			mcp.WithString("content_type",
				mcp.Description("Filter by content type (issue, pull_request)"),
			),
			mcp.WithBoolean("include_archived",
				mcp.Description("Include archived cards"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of cards to return"),
			),
			mcp.WithString("after",
				mcp.Description("Cursor for pagination"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			boardID, err := RequiredParam[string](request, "board_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			columnID, _ := OptionalParam[string](request, "column_id")
			contentType, _ := OptionalParam[string](request, "content_type")
			includeArchived, _ := OptionalParam[bool](request, "include_archived")
			limit, _ := OptionalIntParamWithDefault(request, "limit", 20)
			if limit > 100 {
				limit = 100
			}
			after, _ := OptionalParam[string](request, "after")

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			// Query project items with full details
			var query struct {
				Node struct {
					ProjectV2 struct {
						ID    githubv4.String
						Title githubv4.String
						Items struct {
							PageInfo struct {
								EndCursor   githubv4.String
								HasNextPage githubv4.Boolean
							}
							TotalCount githubv4.Int
							Nodes      []struct {
								ID        githubv4.String
								Archived  githubv4.Boolean
								CreatedAt githubv4.DateTime
								UpdatedAt githubv4.DateTime
								FieldValues struct {
									Nodes []struct {
										FieldValue struct {
											TypeName githubv4.String `graphql:"__typename"`
											SingleSelectValue struct {
												ID          githubv4.String
												Name        githubv4.String
												Description githubv4.String
												Color       githubv4.String
											} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
											TextValue struct {
												Text githubv4.String
											} `graphql:"... on ProjectV2ItemFieldTextValue"`
											NumberValue struct {
												Number githubv4.Float
											} `graphql:"... on ProjectV2ItemFieldNumberValue"`
											DateValue struct {
												Date githubv4.String
											} `graphql:"... on ProjectV2ItemFieldDateValue"`
										}
										Field struct {
											TypeName githubv4.String `graphql:"__typename"`
											Name     githubv4.String
										}
									}
								} `graphql:"fieldValues(first: 20)"`
								Content struct {
									TypeName githubv4.String `graphql:"__typename"`
									Issue    struct {
										ID         githubv4.String
										Number     githubv4.Int
										Title      githubv4.String
										State      githubv4.String
										URL        githubv4.String
										Labels     struct {
											Nodes []struct {
												Name  githubv4.String
												Color githubv4.String
											}
										} `graphql:"labels(first: 10)"`
										Assignees struct {
											Nodes []struct {
												Login     githubv4.String
												AvatarURL githubv4.String `graphql:"avatarUrl"`
											}
										} `graphql:"assignees(first: 5)"`
									} `graphql:"... on Issue"`
									PullRequest struct {
										ID           githubv4.String
										Number       githubv4.Int
										Title        githubv4.String
										State        githubv4.String
										URL          githubv4.String
										IsDraft      githubv4.Boolean
										ReviewDecision githubv4.String
										Labels       struct {
											Nodes []struct {
												Name  githubv4.String
												Color githubv4.String
											}
										} `graphql:"labels(first: 10)"`
										Assignees struct {
											Nodes []struct {
												Login     githubv4.String
												AvatarURL githubv4.String `graphql:"avatarUrl"`
											}
										} `graphql:"assignees(first: 5)"`
									} `graphql:"... on PullRequest"`
								}
							}
						} `graphql:"items(first: $limit, after: $after, includeArchived: $includeArchived)"`
					} `graphql:"... on ProjectV2"`
				} `graphql:"node(id: $id)"`
			}

			variables := map[string]interface{}{
				"id":              githubv4.ID(boardID),
				"limit":           githubv4.Int(limit),
				"includeArchived": githubv4.Boolean(includeArchived),
			}

			if after != "" {
				variables["after"] = githubv4.String(after)
			}

			err = client.Query(ctx, &query, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list project cards: %v", err)), nil
			}

			// Process and filter results
			cards := []map[string]interface{}{}
			for _, item := range query.Node.ProjectV2.Items.Nodes {
				// Skip archived if not requested
				if bool(item.Archived) && !includeArchived {
					continue
				}

				// Filter by content type if specified
				if contentType != "" {
					itemType := string(item.Content.TypeName)
					if (contentType == "issue" && itemType != "Issue") ||
						(contentType == "pull_request" && itemType != "PullRequest") {
						continue
					}
				}

				// Extract field values
				fields := map[string]interface{}{}
				var currentColumn string
				for _, fv := range item.FieldValues.Nodes {
					fieldName := string(fv.Field.Name)
					switch string(fv.FieldValue.TypeName) {
					case "ProjectV2ItemFieldSingleSelectValue":
						fields[fieldName] = map[string]interface{}{
							"id":          string(fv.FieldValue.SingleSelectValue.ID),
							"name":        string(fv.FieldValue.SingleSelectValue.Name),
							"description": string(fv.FieldValue.SingleSelectValue.Description),
							"color":       string(fv.FieldValue.SingleSelectValue.Color),
						}
						if fieldName == "Status" {
							currentColumn = string(fv.FieldValue.SingleSelectValue.ID)
						}
					case "ProjectV2ItemFieldTextValue":
						fields[fieldName] = string(fv.FieldValue.TextValue.Text)
					case "ProjectV2ItemFieldNumberValue":
						fields[fieldName] = float64(fv.FieldValue.NumberValue.Number)
					case "ProjectV2ItemFieldDateValue":
						fields[fieldName] = string(fv.FieldValue.DateValue.Date)
					}
				}

				// Filter by column if specified
				if columnID != "" && currentColumn != columnID {
					continue
				}

				card := map[string]interface{}{
					"id":         string(item.ID),
					"archived":   bool(item.Archived),
					"created_at": item.CreatedAt.Format("2006-01-02T15:04:05Z"),
					"updated_at": item.UpdatedAt.Format("2006-01-02T15:04:05Z"),
					"type":       string(item.Content.TypeName),
					"fields":     fields,
				}

				// Add content details
				if item.Content.TypeName == "Issue" {
					labels := []map[string]string{}
					for _, label := range item.Content.Issue.Labels.Nodes {
						labels = append(labels, map[string]string{
							"name":  string(label.Name),
							"color": string(label.Color),
						})
					}

					assignees := []map[string]string{}
					for _, assignee := range item.Content.Issue.Assignees.Nodes {
						assignees = append(assignees, map[string]string{
							"login":      string(assignee.Login),
							"avatar_url": string(assignee.AvatarURL),
						})
					}

					card["content"] = map[string]interface{}{
						"id":        string(item.Content.Issue.ID),
						"number":    int(item.Content.Issue.Number),
						"title":     string(item.Content.Issue.Title),
						"state":     string(item.Content.Issue.State),
						"url":       string(item.Content.Issue.URL),
						"labels":    labels,
						"assignees": assignees,
					}
				} else if item.Content.TypeName == "PullRequest" {
					labels := []map[string]string{}
					for _, label := range item.Content.PullRequest.Labels.Nodes {
						labels = append(labels, map[string]string{
							"name":  string(label.Name),
							"color": string(label.Color),
						})
					}

					assignees := []map[string]string{}
					for _, assignee := range item.Content.PullRequest.Assignees.Nodes {
						assignees = append(assignees, map[string]string{
							"login":      string(assignee.Login),
							"avatar_url": string(assignee.AvatarURL),
						})
					}

					card["content"] = map[string]interface{}{
						"id":              string(item.Content.PullRequest.ID),
						"number":          int(item.Content.PullRequest.Number),
						"title":           string(item.Content.PullRequest.Title),
						"state":           string(item.Content.PullRequest.State),
						"url":             string(item.Content.PullRequest.URL),
						"is_draft":        bool(item.Content.PullRequest.IsDraft),
						"review_decision": string(item.Content.PullRequest.ReviewDecision),
						"labels":          labels,
						"assignees":       assignees,
					}
				}

				cards = append(cards, card)
			}

			result := map[string]interface{}{
				"board_id":    boardID,
				"board_title": string(query.Node.ProjectV2.Title),
				"cards":       cards,
				"count":       len(cards),
				"total_count": int(query.Node.ProjectV2.Items.TotalCount),
				"page_info": map[string]interface{}{
					"end_cursor":    string(query.Node.ProjectV2.Items.PageInfo.EndCursor),
					"has_next_page": bool(query.Node.ProjectV2.Items.PageInfo.HasNextPage),
				},
			}

			if columnID != "" {
				result["filtered_by_column"] = columnID
			}
			if contentType != "" {
				result["filtered_by_type"] = contentType
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetProjectCard creates a tool to get detailed information about a specific project card
func GetProjectCard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_project_card",
			mcp.WithDescription(t("TOOL_GET_PROJECT_CARD_DESCRIPTION", "Get detailed information about a specific project card")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PROJECT_CARD_USER_TITLE", "Get project card"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("card_id",
				mcp.Required(),
				mcp.Description("ID of the card to retrieve"),
			),
			mcp.WithBoolean("include_history",
				mcp.Description("Include card history and timeline"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cardID, err := RequiredParam[string](request, "card_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			includeHistory, _ := OptionalParam[bool](request, "include_history")

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			// Query card details with all field values
			var query struct {
				Node struct {
					ProjectV2Item struct {
						ID        githubv4.String
						Archived  githubv4.Boolean
						CreatedAt githubv4.DateTime
						UpdatedAt githubv4.DateTime
						Creator   struct {
							Login     githubv4.String
							AvatarURL githubv4.String `graphql:"avatarUrl"`
						}
						Project struct {
							ID    githubv4.String
							Title githubv4.String
							URL   githubv4.String
						}
						FieldValues struct {
							Nodes []struct {
								FieldValue struct {
									TypeName githubv4.String `graphql:"__typename"`
									SingleSelectValue struct {
										ID          githubv4.String
										Name        githubv4.String
										Description githubv4.String
										Color       githubv4.String
									} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
									TextValue struct {
										ID   githubv4.String
										Text githubv4.String
									} `graphql:"... on ProjectV2ItemFieldTextValue"`
									NumberValue struct {
										ID     githubv4.String
										Number githubv4.Float
									} `graphql:"... on ProjectV2ItemFieldNumberValue"`
									DateValue struct {
										ID   githubv4.String
										Date githubv4.String
									} `graphql:"... on ProjectV2ItemFieldDateValue"`
									IterationValue struct {
										ID        githubv4.String
										Title     githubv4.String
										StartDate githubv4.String
										Duration  githubv4.Int
									} `graphql:"... on ProjectV2ItemFieldIterationValue"`
									RepositoryValue struct {
										Repository struct {
											ID       githubv4.String
											Name     githubv4.String
											URL      githubv4.String
											IsPrivate githubv4.Boolean
										}
									} `graphql:"... on ProjectV2ItemFieldRepositoryValue"`
									UserValue struct {
										Users struct {
											Nodes []struct {
												ID        githubv4.String
												Login     githubv4.String
												AvatarURL githubv4.String `graphql:"avatarUrl"`
											}
										} `graphql:"users(first: 10)"`
									} `graphql:"... on ProjectV2ItemFieldUserValue"`
									LabelValue struct {
										Labels struct {
											Nodes []struct {
												ID    githubv4.String
												Name  githubv4.String
												Color githubv4.String
											}
										} `graphql:"labels(first: 20)"`
									} `graphql:"... on ProjectV2ItemFieldLabelValue"`
									MilestoneValue struct {
										Milestone struct {
											ID          githubv4.String
											Title       githubv4.String
											Description githubv4.String
											DueOn       githubv4.DateTime
											State       githubv4.String
										}
									} `graphql:"... on ProjectV2ItemFieldMilestoneValue"`
									PullRequestValue struct {
										PullRequests struct {
											Nodes []struct {
												ID     githubv4.String
												Number githubv4.Int
												Title  githubv4.String
												URL    githubv4.String
											}
										} `graphql:"pullRequests(first: 10)"`
									} `graphql:"... on ProjectV2ItemFieldPullRequestValue"`
								}
								Field struct {
									ID       githubv4.String
									Name     githubv4.String
									DataType githubv4.String
									Config   githubv4.String
								}
							}
						} `graphql:"fieldValues(first: 50)"`
						Content struct {
							TypeName githubv4.String `graphql:"__typename"`
							Issue    struct {
								ID          githubv4.String
								Number      githubv4.Int
								Title       githubv4.String
								Body        githubv4.String
								State       githubv4.String
								StateReason githubv4.String
								URL         githubv4.String
								CreatedAt   githubv4.DateTime
								UpdatedAt   githubv4.DateTime
								ClosedAt    githubv4.DateTime
								Author      struct {
									Login     githubv4.String
									AvatarURL githubv4.String `graphql:"avatarUrl"`
								}
								Labels struct {
									Nodes []struct {
										ID          githubv4.String
										Name        githubv4.String
										Color       githubv4.String
										Description githubv4.String
									}
								} `graphql:"labels(first: 20)"`
								Assignees struct {
									Nodes []struct {
										ID        githubv4.String
										Login     githubv4.String
										AvatarURL githubv4.String `graphql:"avatarUrl"`
									}
								} `graphql:"assignees(first: 10)"`
								Milestone struct {
									ID    githubv4.String
									Title githubv4.String
									State githubv4.String
								}
								Comments struct {
									TotalCount githubv4.Int
								}
								Reactions struct {
									TotalCount githubv4.Int
								}
							} `graphql:"... on Issue"`
							PullRequest struct {
								ID             githubv4.String
								Number         githubv4.Int
								Title          githubv4.String
								Body           githubv4.String
								State          githubv4.String
								URL            githubv4.String
								CreatedAt      githubv4.DateTime
								UpdatedAt      githubv4.DateTime
								ClosedAt       githubv4.DateTime
								MergedAt       githubv4.DateTime
								IsDraft        githubv4.Boolean
								ReviewDecision githubv4.String
								Author         struct {
									Login     githubv4.String
									AvatarURL githubv4.String `graphql:"avatarUrl"`
								}
								Labels struct {
									Nodes []struct {
										ID          githubv4.String
										Name        githubv4.String
										Color       githubv4.String
										Description githubv4.String
									}
								} `graphql:"labels(first: 20)"`
								Assignees struct {
									Nodes []struct {
										ID        githubv4.String
										Login     githubv4.String
										AvatarURL githubv4.String `graphql:"avatarUrl"`
									}
								} `graphql:"assignees(first: 10)"`
								Reviews struct {
									TotalCount githubv4.Int
								}
								Comments struct {
									TotalCount githubv4.Int
								}
								Additions githubv4.Int
								Deletions githubv4.Int
								ChangedFiles githubv4.Int
							} `graphql:"... on PullRequest"`
						}
					} `graphql:"... on ProjectV2Item"`
				} `graphql:"node(id: $id)"`
			}

			variables := map[string]interface{}{
				"id": githubv4.ID(cardID),
			}

			err = client.Query(ctx, &query, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get card details: %v", err)), nil
			}

			item := query.Node.ProjectV2Item

			// Process field values
			fields := map[string]interface{}{}
			for _, fv := range item.FieldValues.Nodes {
				fieldName := string(fv.Field.Name)
				fieldInfo := map[string]interface{}{
					"field_id":   string(fv.Field.ID),
					"data_type":  string(fv.Field.DataType),
					"field_name": fieldName,
				}

				switch string(fv.FieldValue.TypeName) {
				case "ProjectV2ItemFieldSingleSelectValue":
					fieldInfo["value"] = map[string]interface{}{
						"id":          string(fv.FieldValue.SingleSelectValue.ID),
						"name":        string(fv.FieldValue.SingleSelectValue.Name),
						"description": string(fv.FieldValue.SingleSelectValue.Description),
						"color":       string(fv.FieldValue.SingleSelectValue.Color),
					}
				case "ProjectV2ItemFieldTextValue":
					fieldInfo["value"] = string(fv.FieldValue.TextValue.Text)
				case "ProjectV2ItemFieldNumberValue":
					fieldInfo["value"] = float64(fv.FieldValue.NumberValue.Number)
				case "ProjectV2ItemFieldDateValue":
					fieldInfo["value"] = string(fv.FieldValue.DateValue.Date)
				case "ProjectV2ItemFieldIterationValue":
					fieldInfo["value"] = map[string]interface{}{
						"id":         string(fv.FieldValue.IterationValue.ID),
						"title":      string(fv.FieldValue.IterationValue.Title),
						"start_date": string(fv.FieldValue.IterationValue.StartDate),
						"duration":   int(fv.FieldValue.IterationValue.Duration),
					}
				case "ProjectV2ItemFieldRepositoryValue":
					fieldInfo["value"] = map[string]interface{}{
						"id":         string(fv.FieldValue.RepositoryValue.Repository.ID),
						"name":       string(fv.FieldValue.RepositoryValue.Repository.Name),
						"url":        string(fv.FieldValue.RepositoryValue.Repository.URL),
						"is_private": bool(fv.FieldValue.RepositoryValue.Repository.IsPrivate),
					}
				case "ProjectV2ItemFieldUserValue":
					users := []map[string]string{}
					for _, user := range fv.FieldValue.UserValue.Users.Nodes {
						users = append(users, map[string]string{
							"id":         string(user.ID),
							"login":      string(user.Login),
							"avatar_url": string(user.AvatarURL),
						})
					}
					fieldInfo["value"] = users
				case "ProjectV2ItemFieldLabelValue":
					labels := []map[string]string{}
					for _, label := range fv.FieldValue.LabelValue.Labels.Nodes {
						labels = append(labels, map[string]string{
							"id":    string(label.ID),
							"name":  string(label.Name),
							"color": string(label.Color),
						})
					}
					fieldInfo["value"] = labels
				case "ProjectV2ItemFieldMilestoneValue":
					fieldInfo["value"] = map[string]interface{}{
						"id":          string(fv.FieldValue.MilestoneValue.Milestone.ID),
						"title":       string(fv.FieldValue.MilestoneValue.Milestone.Title),
						"description": string(fv.FieldValue.MilestoneValue.Milestone.Description),
						"due_on":      fv.FieldValue.MilestoneValue.Milestone.DueOn.Format("2006-01-02T15:04:05Z"),
						"state":       string(fv.FieldValue.MilestoneValue.Milestone.State),
					}
				case "ProjectV2ItemFieldPullRequestValue":
					prs := []map[string]interface{}{}
					for _, pr := range fv.FieldValue.PullRequestValue.PullRequests.Nodes {
						prs = append(prs, map[string]interface{}{
							"id":     string(pr.ID),
							"number": int(pr.Number),
							"title":  string(pr.Title),
							"url":    string(pr.URL),
						})
					}
					fieldInfo["value"] = prs
				}

				fields[fieldName] = fieldInfo
			}

			result := map[string]interface{}{
				"id":         string(item.ID),
				"archived":   bool(item.Archived),
				"created_at": item.CreatedAt.Format("2006-01-02T15:04:05Z"),
				"updated_at": item.UpdatedAt.Format("2006-01-02T15:04:05Z"),
				"creator": map[string]string{
					"login":      string(item.Creator.Login),
					"avatar_url": string(item.Creator.AvatarURL),
				},
				"project": map[string]interface{}{
					"id":    string(item.Project.ID),
					"title": string(item.Project.Title),
					"url":   string(item.Project.URL),
				},
				"type":   string(item.Content.TypeName),
				"fields": fields,
			}

			// Add detailed content information
			if item.Content.TypeName == "Issue" {
				issue := item.Content.Issue

				labels := []map[string]string{}
				for _, label := range issue.Labels.Nodes {
					labels = append(labels, map[string]string{
						"id":          string(label.ID),
						"name":        string(label.Name),
						"color":       string(label.Color),
						"description": string(label.Description),
					})
				}

				assignees := []map[string]string{}
				for _, assignee := range issue.Assignees.Nodes {
					assignees = append(assignees, map[string]string{
						"id":         string(assignee.ID),
						"login":      string(assignee.Login),
						"avatar_url": string(assignee.AvatarURL),
					})
				}

				content := map[string]interface{}{
					"id":             string(issue.ID),
					"number":         int(issue.Number),
					"title":          string(issue.Title),
					"body":           string(issue.Body),
					"state":          string(issue.State),
					"state_reason":   string(issue.StateReason),
					"url":            string(issue.URL),
					"created_at":     issue.CreatedAt.Format("2006-01-02T15:04:05Z"),
					"updated_at":     issue.UpdatedAt.Format("2006-01-02T15:04:05Z"),
					"labels":         labels,
					"assignees":      assignees,
					"comment_count":  int(issue.Comments.TotalCount),
					"reaction_count": int(issue.Reactions.TotalCount),
					"author": map[string]string{
						"login":      string(issue.Author.Login),
						"avatar_url": string(issue.Author.AvatarURL),
					},
				}

				if !issue.ClosedAt.IsZero() {
					content["closed_at"] = issue.ClosedAt.Format("2006-01-02T15:04:05Z")
				}

				if issue.Milestone.ID != "" {
					content["milestone"] = map[string]interface{}{
						"id":    string(issue.Milestone.ID),
						"title": string(issue.Milestone.Title),
						"state": string(issue.Milestone.State),
					}
				}

				result["content"] = content
			} else if item.Content.TypeName == "PullRequest" {
				pr := item.Content.PullRequest

				labels := []map[string]string{}
				for _, label := range pr.Labels.Nodes {
					labels = append(labels, map[string]string{
						"id":          string(label.ID),
						"name":        string(label.Name),
						"color":       string(label.Color),
						"description": string(label.Description),
					})
				}

				assignees := []map[string]string{}
				for _, assignee := range pr.Assignees.Nodes {
					assignees = append(assignees, map[string]string{
						"id":         string(assignee.ID),
						"login":      string(assignee.Login),
						"avatar_url": string(assignee.AvatarURL),
					})
				}

				content := map[string]interface{}{
					"id":              string(pr.ID),
					"number":          int(pr.Number),
					"title":           string(pr.Title),
					"body":            string(pr.Body),
					"state":           string(pr.State),
					"url":             string(pr.URL),
					"created_at":      pr.CreatedAt.Format("2006-01-02T15:04:05Z"),
					"updated_at":      pr.UpdatedAt.Format("2006-01-02T15:04:05Z"),
					"is_draft":        bool(pr.IsDraft),
					"review_decision": string(pr.ReviewDecision),
					"labels":          labels,
					"assignees":       assignees,
					"review_count":    int(pr.Reviews.TotalCount),
					"comment_count":   int(pr.Comments.TotalCount),
					"additions":       int(pr.Additions),
					"deletions":       int(pr.Deletions),
					"changed_files":   int(pr.ChangedFiles),
					"author": map[string]string{
						"login":      string(pr.Author.Login),
						"avatar_url": string(pr.Author.AvatarURL),
					},
				}

				if !pr.ClosedAt.IsZero() {
					content["closed_at"] = pr.ClosedAt.Format("2006-01-02T15:04:05Z")
				}
				if !pr.MergedAt.IsZero() {
					content["merged_at"] = pr.MergedAt.Format("2006-01-02T15:04:05Z")
				}

				result["content"] = content
			}

			// TODO: Add history/timeline if includeHistory is true (requires additional query)
			if includeHistory {
				result["history_note"] = "Card history/timeline feature coming in Phase 3"
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}