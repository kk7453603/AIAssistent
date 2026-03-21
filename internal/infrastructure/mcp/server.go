package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
	"github.com/kirillkom/personal-ai-assistant/internal/core/ports"
)

// ServerDeps holds the service dependencies for the MCP server tool handlers.
type ServerDeps struct {
	QuerySvc       ports.DocumentQueryService
	WebSearcher    ports.WebSearcher
	ObsidianWriter ports.ObsidianNoteWriter
	Tasks          ports.TaskStore
	KnowledgeTopK  int
}

// NewMCPHandler creates an http.Handler that serves MCP tools over Streamable HTTP.
func NewMCPHandler(deps ServerDeps) http.Handler {
	s := mcpserver.NewMCPServer(
		"Personal AI Assistant",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	registerKnowledgeSearch(s, deps)
	registerWebSearch(s, deps)
	registerObsidianWrite(s, deps)
	registerTaskTool(s, deps)

	return mcpserver.NewStreamableHTTPServer(s,
		mcpserver.WithStateLess(true),
	)
}

func registerKnowledgeSearch(s *mcpserver.MCPServer, deps ServerDeps) {
	s.AddTool(
		mcpgo.NewTool("knowledge_search",
			mcpgo.WithDescription("Search the knowledge base (Obsidian vaults and uploaded documents) for relevant information."),
			mcpgo.WithString("question", mcpgo.Required(), mcpgo.Description("The search query")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 5)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			question := req.GetString("question", "")
			if question == "" {
				return mcpgo.NewToolResultError("question is required"), nil
			}
			limit := req.GetInt("limit", deps.KnowledgeTopK)
			if limit <= 0 {
				limit = deps.KnowledgeTopK
			}

			answer, err := deps.QuerySvc.Answer(ctx, question, limit, domain.SearchFilter{})
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("knowledge search failed", err), nil
			}

			payload, _ := json.Marshal(map[string]any{
				"answer":  answer.Text,
				"sources": answer.Sources,
			})
			return mcpgo.NewToolResultText(string(payload)), nil
		},
	)
}

func registerWebSearch(s *mcpserver.MCPServer, deps ServerDeps) {
	if deps.WebSearcher == nil {
		return
	}
	s.AddTool(
		mcpgo.NewTool("web_search",
			mcpgo.WithDescription("Search the internet via SearXNG web search engine."),
			mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("The search query")),
			mcpgo.WithNumber("limit", mcpgo.Description("Maximum number of results (default 5)")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			query := req.GetString("query", "")
			if query == "" {
				return mcpgo.NewToolResultError("query is required"), nil
			}
			limit := req.GetInt("limit", 5)

			results, err := deps.WebSearcher.Search(ctx, query, limit)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("web search failed", err), nil
			}

			payload, _ := json.Marshal(map[string]any{
				"query":   query,
				"results": results,
				"count":   len(results),
			})
			return mcpgo.NewToolResultText(string(payload)), nil
		},
	)
}

func registerObsidianWrite(s *mcpserver.MCPServer, deps ServerDeps) {
	if deps.ObsidianWriter == nil {
		return
	}
	s.AddTool(
		mcpgo.NewTool("obsidian_write",
			mcpgo.WithDescription("Create a new note in an Obsidian vault."),
			mcpgo.WithString("vault", mcpgo.Description("Vault ID")),
			mcpgo.WithString("title", mcpgo.Required(), mcpgo.Description("Note title")),
			mcpgo.WithString("content", mcpgo.Required(), mcpgo.Description("Note content in Markdown")),
			mcpgo.WithString("folder", mcpgo.Description("Folder inside the vault")),
		),
		func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			title := req.GetString("title", "")
			content := req.GetString("content", "")
			if title == "" || content == "" {
				return mcpgo.NewToolResultError("title and content are required"), nil
			}
			vault := req.GetString("vault", "")
			folder := req.GetString("folder", "")

			path, err := deps.ObsidianWriter.CreateNote(ctx, vault, title, content, folder)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("obsidian write failed", err), nil
			}

			payload, _ := json.Marshal(map[string]string{
				"status": "created",
				"vault":  vault,
				"title":  title,
				"path":   path,
			})
			return mcpgo.NewToolResultText(string(payload)), nil
		},
	)
}

func registerTaskTool(s *mcpserver.MCPServer, deps ServerDeps) {
	s.AddTool(
		mcpgo.NewTool("task_create",
			mcpgo.WithDescription("Create a new task for the user."),
			mcpgo.WithString("user_id", mcpgo.Required(), mcpgo.Description("User ID")),
			mcpgo.WithString("title", mcpgo.Required(), mcpgo.Description("Task title")),
			mcpgo.WithString("details", mcpgo.Description("Task details")),
			mcpgo.WithString("due_at", mcpgo.Description("Due date in RFC3339 format")),
		),
		makeTaskHandler(deps, "create"),
	)

	s.AddTool(
		mcpgo.NewTool("task_list",
			mcpgo.WithDescription("List all tasks for a user."),
			mcpgo.WithString("user_id", mcpgo.Required(), mcpgo.Description("User ID")),
			mcpgo.WithBoolean("include_deleted", mcpgo.Description("Include soft-deleted tasks")),
		),
		makeTaskHandler(deps, "list"),
	)

	s.AddTool(
		mcpgo.NewTool("task_get",
			mcpgo.WithDescription("Get a specific task by ID."),
			mcpgo.WithString("user_id", mcpgo.Required(), mcpgo.Description("User ID")),
			mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Task ID")),
		),
		makeTaskHandler(deps, "get"),
	)

	s.AddTool(
		mcpgo.NewTool("task_update",
			mcpgo.WithDescription("Update an existing task."),
			mcpgo.WithString("user_id", mcpgo.Required(), mcpgo.Description("User ID")),
			mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Task ID")),
			mcpgo.WithString("title", mcpgo.Description("New title")),
			mcpgo.WithString("details", mcpgo.Description("New details")),
			mcpgo.WithString("status", mcpgo.Description("New status: open or completed")),
		),
		makeTaskHandler(deps, "update"),
	)

	s.AddTool(
		mcpgo.NewTool("task_delete",
			mcpgo.WithDescription("Soft-delete a task."),
			mcpgo.WithString("user_id", mcpgo.Required(), mcpgo.Description("User ID")),
			mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Task ID")),
		),
		makeTaskHandler(deps, "delete"),
	)

	s.AddTool(
		mcpgo.NewTool("task_complete",
			mcpgo.WithDescription("Mark a task as completed."),
			mcpgo.WithString("user_id", mcpgo.Required(), mcpgo.Description("User ID")),
			mcpgo.WithString("id", mcpgo.Required(), mcpgo.Description("Task ID")),
		),
		makeTaskHandler(deps, "complete"),
	)
}

func makeTaskHandler(deps ServerDeps, action string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		userID := req.GetString("user_id", "")
		if userID == "" {
			return mcpgo.NewToolResultError("user_id is required"), nil
		}

		switch action {
		case "create":
			title := req.GetString("title", "")
			if title == "" {
				return mcpgo.NewToolResultError("title is required"), nil
			}
			now := time.Now().UTC()
			task := &domain.Task{
				ID:        generateID(),
				UserID:    userID,
				Title:     title,
				Details:   req.GetString("details", ""),
				Status:    domain.TaskStatusOpen,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := deps.Tasks.CreateTask(ctx, task); err != nil {
				return mcpgo.NewToolResultErrorFromErr("create task failed", err), nil
			}
			return marshalResult(task)

		case "list":
			tasks, err := deps.Tasks.ListTasks(ctx, userID, req.GetBool("include_deleted", false))
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("list tasks failed", err), nil
			}
			return marshalResult(tasks)

		case "get":
			id := req.GetString("id", "")
			if id == "" {
				return mcpgo.NewToolResultError("id is required"), nil
			}
			task, err := deps.Tasks.GetTaskByID(ctx, userID, id)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("get task failed", err), nil
			}
			return marshalResult(task)

		case "update":
			id := req.GetString("id", "")
			if id == "" {
				return mcpgo.NewToolResultError("id is required"), nil
			}
			task, err := deps.Tasks.GetTaskByID(ctx, userID, id)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("get task for update failed", err), nil
			}
			if title := strings.TrimSpace(req.GetString("title", "")); title != "" {
				task.Title = title
			}
			if details := req.GetString("details", ""); details != "" {
				task.Details = details
			}
			if status := strings.ToLower(req.GetString("status", "")); status != "" {
				switch domain.TaskStatus(status) {
				case domain.TaskStatusOpen, domain.TaskStatusCompleted:
					task.Status = domain.TaskStatus(status)
				default:
					return mcpgo.NewToolResultError(fmt.Sprintf("unsupported status: %s", status)), nil
				}
			}
			if err := deps.Tasks.UpdateTask(ctx, task); err != nil {
				return mcpgo.NewToolResultErrorFromErr("update task failed", err), nil
			}
			return marshalResult(task)

		case "delete":
			id := req.GetString("id", "")
			if id == "" {
				return mcpgo.NewToolResultError("id is required"), nil
			}
			if err := deps.Tasks.SoftDeleteTask(ctx, userID, id); err != nil {
				return mcpgo.NewToolResultErrorFromErr("delete task failed", err), nil
			}
			return marshalResult(map[string]string{"id": id, "status": "deleted"})

		case "complete":
			id := req.GetString("id", "")
			if id == "" {
				return mcpgo.NewToolResultError("id is required"), nil
			}
			task, err := deps.Tasks.GetTaskByID(ctx, userID, id)
			if err != nil {
				return mcpgo.NewToolResultErrorFromErr("get task for complete failed", err), nil
			}
			task.Status = domain.TaskStatusCompleted
			if err := deps.Tasks.UpdateTask(ctx, task); err != nil {
				return mcpgo.NewToolResultErrorFromErr("complete task failed", err), nil
			}
			return marshalResult(task)

		default:
			return mcpgo.NewToolResultError(fmt.Sprintf("unsupported action: %s", action)), nil
		}
	}
}

func marshalResult(v any) (*mcpgo.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcpgo.NewToolResultErrorFromErr("marshal result", err), nil
	}
	return mcpgo.NewToolResultText(string(data)), nil
}

func generateID() string {
	return uuid.NewString()
}
