package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/fujiwara/ridge"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mashiike/estellm"
)

type Server struct {
	s *server.MCPServer
}

func NewServer(serverName string, version string, mux *estellm.AgentMux) *Server {
	s := server.NewMCPServer(
		serverName,
		version,
		server.WithToolCapabilities(true),
	)
	for name, cfg := range mux.Published() {
		for _, publishType := range cfg.PublishTypes {
			switch publishType {
			case estellm.PublishTypeTool:
				addTool(s, cfg, mux)
			case estellm.PublishTypePrompt:
				addPrompt(s, cfg, mux)
			default:
				slog.Warn("unknown publish type", "name", name, "type", publishType)
			}
		}
	}
	return &Server{s: s}
}

func addTool(s *server.MCPServer, cfg *estellm.Config, mux *estellm.AgentMux) {
	bs, err := json.Marshal(cfg.PayloadSchema)
	if err != nil {
		slog.Warn("failed to marshal payload schema", "name", cfg.Name, "details", err)
		return
	}
	tool := mcp.NewToolWithRawSchema(cfg.Name, cfg.Description, bs)
	s.AddTool(tool, newToolHandler(cfg, mux))
	slog.Info("add mcp tool", "name", cfg.Name)
}

func addPrompt(s *server.MCPServer, cfg *estellm.Config, mux *estellm.AgentMux) {
	options := []mcp.PromptOption{}
	if cfg.Description != "" {
		options = append(options, mcp.WithPromptDescription(cfg.Description))
	}
	if len(cfg.Arguments) > 0 {
		for _, arg := range cfg.Arguments {
			argOpts := []mcp.ArgumentOption{}
			if arg.Description != "" {
				argOpts = append(argOpts, mcp.ArgumentDescription(arg.Description))
			}
			if arg.Required {
				argOpts = append(argOpts, mcp.RequiredArgument())
			}
			options = append(options, mcp.WithArgument(arg.Name, argOpts...))
		}
	}
	prompt := mcp.NewPrompt(cfg.Name, options...)
	s.AddPrompt(prompt, newPromptHandler(cfg, mux))
	slog.Info("add mcp prompt", "name", cfg.Name)
}

func (s *Server) ListenAndServeSSE(addr string, opts ...server.SSEOption) error {
	baseURL := addr
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}
	if u.Hostname() == "" {
		u.Host = "localhost" + u.Host
	}
	if hostname := u.Hostname(); hostname == "localhost" || hostname == "127.0.0.1" {
		u.Scheme = "http"
	}
	options := []server.SSEOption{
		server.WithBaseURL(u.String()),
	}
	options = append(options, opts...)
	sseServer := server.NewSSEServer(s.s, options...)
	ridge.Run(addr, "/", sseServer)
	return nil
}

func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.s)
}

func newToolHandler(cfg *estellm.Config, mux *estellm.AgentMux) server.ToolHandlerFunc {
	return func(ctx context.Context, mcpReq mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		req, err := estellm.NewRequest(cfg.Name, mcpReq.Params.Arguments)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("failed to create execution request: %v", err),
					},
				},
			}, nil
		}
		w := estellm.NewBatchResponseWriter()
		if err := mux.Execute(ctx, req, w); err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("failed to execute: %v", err),
					},
				},
			}, nil
		}
		callToolResult := &mcp.CallToolResult{}
		resp := w.Response()
		callToolResult.Meta = resp.Metadata
		for _, part := range resp.Message.Parts {
			if content := convertPart(part); content != nil {
				callToolResult.Content = append(callToolResult.Content, content)
			}
		}
		return callToolResult, nil
	}
}

func convertPart(part estellm.ContentPart) mcp.Content {
	switch part.Type {
	case estellm.PartTypeText:
		return &mcp.TextContent{
			Type: "text",
			Text: part.Text,
		}
	case estellm.PartTypeBinary:
		return &mcp.EmbeddedResource{
			Type: "resource",
			Resource: &mcp.BlobResourceContents{
				MIMEType: part.MIMEType,
				Blob:     base64.StdEncoding.EncodeToString(part.Data),
			},
		}
	case estellm.PartTypeReasoning:
		// skip reasoning
		return nil
	default:
		slog.Warn("unknown part type", "type", part.Type)
		return nil
	}
}

func newPromptHandler(cfg *estellm.Config, mux *estellm.AgentMux) server.PromptHandlerFunc {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		ret := &mcp.GetPromptResult{
			Description: cfg.Description,
			Messages:    []mcp.PromptMessage{},
		}
		req, err := estellm.NewRequest(cfg.Name, request.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to create execution request: %w", err)
		}
		promptStr, err := mux.Render(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to render prompt: %w", err)
		}
		dec := estellm.NewMessageDecoder(strings.NewReader(promptStr))
		system, messages, err := dec.Decode()
		if err != nil {
			return nil, fmt.Errorf("failed to decode prompt: %w", err)
		}
		if system != "" {
			slog.Warn("system message ignored", "message", system)
		}
		for _, msg := range messages {
			var role mcp.Role
			switch msg.Role {
			case estellm.RoleUser:
				role = mcp.RoleUser
			case estellm.RoleAssistant:
				role = mcp.RoleAssistant
			default:
				slog.Warn("unknown role", "role", msg.Role)
				continue
			}
			for _, part := range msg.Parts {
				if content := convertPart(part); content != nil {
					ret.Messages = append(ret.Messages, mcp.PromptMessage{
						Role:    role,
						Content: content,
					})
				}
			}
		}
		return ret, nil
	}
}
