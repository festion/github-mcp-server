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

// CreateProjectBoard creates a tool to create a new project board
func CreateProjectBoard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_project_board",
			mcp.WithDescription(t("TOOL_CREATE_PROJECT_BOARD_DESCRIPTION", "Create a new GitHub project board with customizable settings")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_PROJECT_BOARD_USER_TITLE", "Create project board"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the project board"),
			),
			mcp.WithString("description",
				mcp.Description("Description of the project board"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner or organization login"),
			),
			mcp.WithString("repository",
				mcp.Description("Repository name (for repository-level projects)"),
			),
			mcp.WithString("template",
				mcp.Description("Template to use (kanban, scrum, bug_triage)"),
				mcp.Enum("kanban", "scrum", "bug_triage", "none"),
			),
			mcp.WithBoolean("public",
				mcp.Description("Whether the project should be public (default: false)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := RequiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			description, _ := OptionalParam[string](request, "description")
			repository, _ := OptionalParam[string](request, "repository")
			template, _ := OptionalParam[string](request, "template")
			public, _ := OptionalParam[bool](request, "public")

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			// Determine owner ID and type
			var mutation struct {
				CreateProjectV2 struct {
					ProjectV2 struct {
						ID          githubv4.String
						Number      githubv4.Int
						Title       githubv4.String
						Public      githubv4.Boolean
						URL         githubv4.String
						Description githubv4.String
					}
				} `graphql:"createProjectV2(input: $input)"`
			}

			// First, get the owner ID
			var ownerQuery struct {
				User struct {
					ID githubv4.String
				} `graphql:"user(login: $login)"`
				Organization struct {
					ID githubv4.String
				} `graphql:"organization(login: $login)"`
			}

			variables := map[string]interface{}{
				"login": githubv4.String(owner),
			}

			err = client.Query(ctx, &ownerQuery, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get owner ID: %v", err)), nil
			}

			var ownerID githubv4.String
			if ownerQuery.User.ID != "" {
				ownerID = ownerQuery.User.ID
			} else if ownerQuery.Organization.ID != "" {
				ownerID = ownerQuery.Organization.ID
			} else {
				return mcp.NewToolResultError("owner not found"), nil
			}

			// Create the project using raw GraphQL variables
			input := map[string]interface{}{
				"ownerId": ownerID,
				"title":   githubv4.String(name),
			}

			if description != "" {
				input["description"] = githubv4.String(description)
			}

			// Handle repository-level projects
			if repository != "" {
				var repoQuery struct {
					Repository struct {
						ID githubv4.String
					} `graphql:"repository(owner: $owner, name: $name)"`
				}
				repoVars := map[string]interface{}{
					"owner": githubv4.String(owner),
					"name":  githubv4.String(repository),
				}
				err = client.Query(ctx, &repoQuery, repoVars)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to get repository ID: %v", err)), nil
				}
				input["repositoryId"] = repoQuery.Repository.ID
			}

			// Apply template settings after creation if needed
			// GitHub Projects API v2 doesn't support templates directly in creation

			createVars := map[string]interface{}{
				"input": input,
			}

			err = client.Mutate(ctx, &mutation, nil, createVars)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create project board: %v", err)), nil
			}

			// Update visibility if public is requested
			if public {
				var updateMutation struct {
					UpdateProjectV2 struct {
						ProjectV2 struct {
							ID     githubv4.String
							Public githubv4.Boolean
						}
					} `graphql:"updateProjectV2(input: $input)"`
				}

				updateInput := githubv4.UpdateProjectV2Input{
					ProjectID: mutation.CreateProjectV2.ProjectV2.ID,
					Public:    githubv4.NewBoolean(githubv4.Boolean(public)),
				}

				updateVars := map[string]interface{}{
					"input": updateInput,
				}

				err = client.Mutate(ctx, &updateMutation, nil, updateVars)
				if err != nil {
					// Non-fatal error, project was created
					fmt.Printf("warning: failed to update project visibility: %v\n", err)
				}
			}

			result := map[string]interface{}{
				"id":          string(mutation.CreateProjectV2.ProjectV2.ID),
				"number":      int(mutation.CreateProjectV2.ProjectV2.Number),
				"title":       string(mutation.CreateProjectV2.ProjectV2.Title),
				"url":         string(mutation.CreateProjectV2.ProjectV2.URL),
				"description": string(mutation.CreateProjectV2.ProjectV2.Description),
				"public":      bool(mutation.CreateProjectV2.ProjectV2.Public),
				"template":    template,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// UpdateProjectBoard creates a tool to update project board settings
func UpdateProjectBoard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("update_project_board",
			mcp.WithDescription(t("TOOL_UPDATE_PROJECT_BOARD_DESCRIPTION", "Update settings and metadata of an existing project board")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_PROJECT_BOARD_USER_TITLE", "Update project board"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("board_id",
				mcp.Required(),
				mcp.Description("ID of the project board to update"),
			),
			mcp.WithString("title",
				mcp.Description("New title for the project board"),
			),
			mcp.WithString("description",
				mcp.Description("New description for the project board"),
			),
			mcp.WithString("short_description",
				mcp.Description("New short description for the project board"),
			),
			mcp.WithBoolean("public",
				mcp.Description("Update visibility of the project board"),
			),
			mcp.WithBoolean("closed",
				mcp.Description("Close or reopen the project board"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			boardID, err := RequiredParam[string](request, "board_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			// Build update input using raw GraphQL variables
			input := map[string]interface{}{
				"projectId": githubv4.ID(boardID),
			}

			// Add optional fields if provided
			if title, ok := request.GetArguments()["title"].(string); ok && title != "" {
				input["title"] = githubv4.String(title)
			}
			if description, ok := request.GetArguments()["description"].(string); ok && description != "" {
				input["description"] = githubv4.String(description)
			}
			if shortDesc, ok := request.GetArguments()["short_description"].(string); ok && shortDesc != "" {
				input["shortDescription"] = githubv4.String(shortDesc)
			}
			if public, ok := request.GetArguments()["public"].(bool); ok {
				input["public"] = githubv4.Boolean(public)
			}
			if closed, ok := request.GetArguments()["closed"].(bool); ok {
				input["closed"] = githubv4.Boolean(closed)
			}

			var mutation struct {
				UpdateProjectV2 struct {
					ProjectV2 struct {
						ID               githubv4.String
						Title            githubv4.String
						Description      githubv4.String
						ShortDescription githubv4.String
						Public           githubv4.Boolean
						Closed           githubv4.Boolean
						URL              githubv4.String
						UpdatedAt        githubv4.DateTime
					}
				} `graphql:"updateProjectV2(input: $input)"`
			}

			variables := map[string]interface{}{
				"input": input,
			}

			err = client.Mutate(ctx, &mutation, nil, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to update project board: %v", err)), nil
			}

			result := map[string]interface{}{
				"id":                string(mutation.UpdateProjectV2.ProjectV2.ID),
				"title":             string(mutation.UpdateProjectV2.ProjectV2.Title),
				"description":       string(mutation.UpdateProjectV2.ProjectV2.Description),
				"short_description": string(mutation.UpdateProjectV2.ProjectV2.ShortDescription),
				"public":            bool(mutation.UpdateProjectV2.ProjectV2.Public),
				"closed":            bool(mutation.UpdateProjectV2.ProjectV2.Closed),
				"url":               string(mutation.UpdateProjectV2.ProjectV2.URL),
				"updated_at":        mutation.UpdateProjectV2.ProjectV2.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// DeleteProjectBoard creates a tool to delete a project board
func DeleteProjectBoard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_project_board",
			mcp.WithDescription(t("TOOL_DELETE_PROJECT_BOARD_DESCRIPTION", "Delete or archive a project board")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_DELETE_PROJECT_BOARD_USER_TITLE", "Delete project board"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("board_id",
				mcp.Required(),
				mcp.Description("ID of the project board to delete"),
			),
			mcp.WithBoolean("confirm",
				mcp.Required(),
				mcp.Description("Confirmation flag to prevent accidental deletion"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			boardID, err := RequiredParam[string](request, "board_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			confirm, err := RequiredParam[bool](request, "confirm")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if !confirm {
				return mcp.NewToolResultError("deletion not confirmed - set confirm to true to delete"), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			var mutation struct {
				DeleteProjectV2 struct {
					ProjectV2 struct {
						ID githubv4.String
					}
				} `graphql:"deleteProjectV2(input: $input)"`
			}

			input := githubv4.DeleteProjectV2Input{
				ProjectID: githubv4.String(boardID),
			}

			variables := map[string]interface{}{
				"input": input,
			}

			err = client.Mutate(ctx, &mutation, nil, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to delete project board: %v", err)), nil
			}

			result := map[string]interface{}{
				"deleted": true,
				"id":      string(mutation.DeleteProjectV2.ProjectV2.ID),
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListProjectBoards creates a tool to list project boards
func ListProjectBoards(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_project_boards",
			mcp.WithDescription(t("TOOL_LIST_PROJECT_BOARDS_DESCRIPTION", "List all accessible project boards for a user or organization")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PROJECT_BOARDS_USER_TITLE", "List project boards"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("User or organization login"),
			),
			mcp.WithString("type",
				mcp.Description("Filter by owner type (user or organization)"),
				mcp.Enum("user", "organization", "all"),
			),
			mcp.WithBoolean("include_closed",
				mcp.Description("Include closed project boards (default: false)"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of boards to return (default: 20, max: 100)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, _ := OptionalParam[string](request, "type")
			if ownerType == "" {
				ownerType = "all"
			}
			includeClosed, _ := OptionalParam[bool](request, "include_closed")
			limit, _ := OptionalIntParamWithDefault(request, "limit", 20)
			if limit > 100 {
				limit = 100
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			// Query for both user and organization projects
			var query struct {
				User struct {
					ProjectsV2 struct {
						Nodes []struct {
							ID               githubv4.String
							Number           githubv4.Int
							Title            githubv4.String
							Description      githubv4.String
							ShortDescription githubv4.String
							Public           githubv4.Boolean
							Closed           githubv4.Boolean
							URL              githubv4.String
							CreatedAt        githubv4.DateTime
							UpdatedAt        githubv4.DateTime
							Items            struct {
								TotalCount githubv4.Int
							}
						}
						TotalCount githubv4.Int
					} `graphql:"projectsV2(first: $limit, includeClosed: $includeClosed)"`
				} `graphql:"user(login: $login)"`
				Organization struct {
					ProjectsV2 struct {
						Nodes []struct {
							ID               githubv4.String
							Number           githubv4.Int
							Title            githubv4.String
							Description      githubv4.String
							ShortDescription githubv4.String
							Public           githubv4.Boolean
							Closed           githubv4.Boolean
							URL              githubv4.String
							CreatedAt        githubv4.DateTime
							UpdatedAt        githubv4.DateTime
							Items            struct {
								TotalCount githubv4.Int
							}
						}
						TotalCount githubv4.Int
					} `graphql:"projectsV2(first: $limit, includeClosed: $includeClosed)"`
				} `graphql:"organization(login: $login)"`
			}

			variables := map[string]interface{}{
				"login":         githubv4.String(owner),
				"limit":         githubv4.Int(limit),
				"includeClosed": githubv4.Boolean(includeClosed),
			}

			err = client.Query(ctx, &query, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to list project boards: %v", err)), nil
			}

			var projects []map[string]interface{}

			// Add user projects
			if ownerType == "user" || ownerType == "all" {
				for _, project := range query.User.ProjectsV2.Nodes {
					projects = append(projects, map[string]interface{}{
						"id":                string(project.ID),
						"number":            int(project.Number),
						"title":             string(project.Title),
						"description":       string(project.Description),
						"short_description": string(project.ShortDescription),
						"public":            bool(project.Public),
						"closed":            bool(project.Closed),
						"url":               string(project.URL),
						"created_at":        project.CreatedAt.Format("2006-01-02T15:04:05Z"),
						"updated_at":        project.UpdatedAt.Format("2006-01-02T15:04:05Z"),
						"items_count":       int(project.Items.TotalCount),
						"owner_type":        "user",
					})
				}
			}

			// Add organization projects
			if ownerType == "organization" || ownerType == "all" {
				for _, project := range query.Organization.ProjectsV2.Nodes {
					projects = append(projects, map[string]interface{}{
						"id":                string(project.ID),
						"number":            int(project.Number),
						"title":             string(project.Title),
						"description":       string(project.Description),
						"short_description": string(project.ShortDescription),
						"public":            bool(project.Public),
						"closed":            bool(project.Closed),
						"url":               string(project.URL),
						"created_at":        project.CreatedAt.Format("2006-01-02T15:04:05Z"),
						"updated_at":        project.UpdatedAt.Format("2006-01-02T15:04:05Z"),
						"items_count":       int(project.Items.TotalCount),
						"owner_type":        "organization",
					})
				}
			}

			result := map[string]interface{}{
				"projects":    projects,
				"total_count": len(projects),
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetProjectBoard creates a tool to get detailed project board information
func GetProjectBoard(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_project_board",
			mcp.WithDescription(t("TOOL_GET_PROJECT_BOARD_DESCRIPTION", "Get detailed information and statistics for a specific project board")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PROJECT_BOARD_USER_TITLE", "Get project board details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("board_id",
				mcp.Required(),
				mcp.Description("ID of the project board"),
			),
			mcp.WithBoolean("include_fields",
				mcp.Description("Include field definitions (default: true)"),
			),
			mcp.WithBoolean("include_stats",
				mcp.Description("Include item statistics (default: true)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			boardID, err := RequiredParam[string](request, "board_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			includeFields, _ := OptionalParam[bool](request, "include_fields")
			if includeFields == false {
				includeFields = true
			}
			includeStats, _ := OptionalParam[bool](request, "include_stats")
			if includeStats == false {
				includeStats = true
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
			}

			var query struct {
				Node struct {
					ProjectV2 struct {
						ID               githubv4.String
						Number           githubv4.Int
						Title            githubv4.String
						Description      githubv4.String
						ShortDescription githubv4.String
						Public           githubv4.Boolean
						Closed           githubv4.Boolean
						URL              githubv4.String
						CreatedAt        githubv4.DateTime
						UpdatedAt        githubv4.DateTime
						Owner            struct {
							TypeName githubv4.String `graphql:"__typename"`
							User     struct {
								Login githubv4.String
							} `graphql:"... on User"`
							Organization struct {
								Login githubv4.String
							} `graphql:"... on Organization"`
						}
						Items struct {
							TotalCount githubv4.Int
						}
						Fields struct {
							Nodes []struct {
								TypeName githubv4.String `graphql:"__typename"`
								Field    struct {
									ID   githubv4.String
									Name githubv4.String
								} `graphql:"... on ProjectV2Field"`
								SingleSelectField struct {
									ID      githubv4.String
									Name    githubv4.String
									Options []struct {
										ID   githubv4.String
										Name githubv4.String
									}
								} `graphql:"... on ProjectV2SingleSelectField"`
								IterationField struct {
									ID            githubv4.String
									Name          githubv4.String
									Configuration struct {
										Duration      githubv4.Int
										StartDay      githubv4.Int
										CompletedAt   githubv4.DateTime
										Iterations    []struct {
											ID        githubv4.String
											Title     githubv4.String
											StartDate githubv4.Date
											Duration  githubv4.Int
										}
									}
								} `graphql:"... on ProjectV2IterationField"`
							}
							TotalCount githubv4.Int
						} `graphql:"fields(first: 20)"`
					} `graphql:"... on ProjectV2"`
				} `graphql:"node(id: $id)"`
			}

			variables := map[string]interface{}{
				"id": githubv4.ID(boardID),
			}

			err = client.Query(ctx, &query, variables)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get project board: %v", err)), nil
			}

			project := query.Node.ProjectV2

			// Determine owner login
			var ownerLogin string
			if project.Owner.TypeName == "User" {
				ownerLogin = string(project.Owner.User.Login)
			} else if project.Owner.TypeName == "Organization" {
				ownerLogin = string(project.Owner.Organization.Login)
			}

			result := map[string]interface{}{
				"id":                string(project.ID),
				"number":            int(project.Number),
				"title":             string(project.Title),
				"description":       string(project.Description),
				"short_description": string(project.ShortDescription),
				"public":            bool(project.Public),
				"closed":            bool(project.Closed),
				"url":               string(project.URL),
				"created_at":        project.CreatedAt.Format("2006-01-02T15:04:05Z"),
				"updated_at":        project.UpdatedAt.Format("2006-01-02T15:04:05Z"),
				"owner": map[string]interface{}{
					"login": ownerLogin,
					"type":  string(project.Owner.TypeName),
				},
			}

			// Add statistics if requested
			if includeStats {
				result["statistics"] = map[string]interface{}{
					"total_items": int(project.Items.TotalCount),
				}
			}

			// Add fields if requested
			if includeFields {
				var fields []map[string]interface{}
				for _, field := range project.Fields.Nodes {
					fieldData := map[string]interface{}{
						"type": string(field.TypeName),
					}

					switch field.TypeName {
					case "ProjectV2Field":
						fieldData["id"] = string(field.Field.ID)
						fieldData["name"] = string(field.Field.Name)
					case "ProjectV2SingleSelectField":
						fieldData["id"] = string(field.SingleSelectField.ID)
						fieldData["name"] = string(field.SingleSelectField.Name)
						var options []map[string]string
						for _, opt := range field.SingleSelectField.Options {
							options = append(options, map[string]string{
								"id":   string(opt.ID),
								"name": string(opt.Name),
							})
						}
						fieldData["options"] = options
					case "ProjectV2IterationField":
						fieldData["id"] = string(field.IterationField.ID)
						fieldData["name"] = string(field.IterationField.Name)
						// Add iteration configuration if needed
					}

					fields = append(fields, fieldData)
				}
				result["fields"] = fields
				result["fields_count"] = int(project.Fields.TotalCount)
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}