package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mashiike/estellm"
	"github.com/mashiike/estellm/interanal/jsonutil"
)

type Config struct {
	Servers map[string]ClientConfig `json:"mcpServers"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type Alias Config
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.Servers = aux.Servers
	for name, server := range c.Servers {
		server.Name = name
		c.Servers[name] = server
	}
	return nil
}

type ClientConfig struct {
	Name     string            `json:"-"`
	Endpoint string            `json:"endpoint"`
	Command  string            `json:"command"`
	Env      map[string]string `json:"env"`
	Args     []string          `json:"args"`
}

type ClientMux struct {
	clients []*Client
}

func NewClientMuxFromConfig(ctx context.Context, cfg *Config) (*ClientMux, error) {
	mux := &ClientMux{}
	for _, clientCfg := range cfg.Servers {
		client, err := newClientFromConfig(ctx, clientCfg)
		if err != nil {
			defer func() {
				if err := mux.Close(); err != nil {
					slog.WarnContext(ctx, "failed to close clients", "details", err)
				}
			}()
			return nil, fmt.Errorf("failed to create client `%s`: %w", clientCfg.Name, err)
		}
		mux.clients = append(mux.clients, client)
	}
	return mux, nil
}

func (mux *ClientMux) Close() error {
	var err error
	for _, client := range mux.clients {
		if cerr := client.Close(); cerr != nil {
			err = cerr
		}
	}
	return err
}

func newClientFromConfig(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.Command == "" && cfg.Endpoint == "" {
		return nil, fmt.Errorf("either command or endpoint must be set")
	}
	if cfg.Command != "" && cfg.Endpoint != "" {
		return nil, fmt.Errorf("only one of command or endpoint must be set")
	}
	c := &Client{
		config: cfg,
	}
	if cfg.Command != "" {
		c.transport = "stdio"
		return prepareStdioClient(ctx, c)
	} else {
		c.transport = "sse"
		return prepareSSEClient(ctx, c)
	}
}

func prepareStdioClient(_ context.Context, c *Client) (*Client, error) {
	env := make([]string, 0, len(c.config.Env))
	for k, v := range c.config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	impl, err := client.NewStdioMCPClient(c.config.Command, env, c.config.Args...)
	if err != nil {
		return nil, err
	}
	c.impl = impl
	return c, nil
}

func prepareSSEClient(_ context.Context, c *Client) (*Client, error) {
	impl, err := client.NewSSEMCPClient(c.config.Endpoint)
	if err != nil {
		return nil, err
	}
	c.impl = impl
	return c, nil
}

func (mux *ClientMux) Tools(ctx context.Context) ([]estellm.Tool, error) {
	tools := make([]estellm.Tool, 0)
	for _, c := range mux.clients {
		_tools, err := c.Tools(ctx)
		if err != nil {
			return nil, err
		}
		tools = append(tools, _tools...)
	}
	return tools, nil

}

type Client struct {
	transport string
	config    ClientConfig
	impl      client.MCPClient
	onceInit  sync.Once
	initErr   error
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) init(ctx context.Context) error {
	c.onceInit.Do(func() {
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{
			Name:    "estellm",
			Version: estellm.Version,
		}
		var initResult *mcp.InitializeResult
		initResult, c.initErr = c.impl.Initialize(ctx, initRequest)
		if c.initErr != nil {
			return
		}
		slog.Info("initialized mcp client", "server", initResult.ServerInfo.Name, "version", initResult.ServerInfo.Version)
	})
	return c.initErr
}

func (c *Client) Tools(ctx context.Context) ([]estellm.Tool, error) {
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := c.impl.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, err
	}
	ret := make([]estellm.Tool, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		var s map[string]any
		if err := json.Unmarshal(tool.RawInputSchema, &s); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input schema for tool `%s`: %w", tool.Name, err)
		}
		slog.InfoContext(ctx, "found mcp tool", "name", tool.Name, "description", tool.Description)
		ret = append(ret, &mcpTool{
			name:        tool.Name,
			desc:        tool.Description,
			inputSchema: s,
			impl:        c.impl,
		})
	}
	return ret, nil
}

type mcpTool struct {
	name        string
	desc        string
	inputSchema map[string]any
	impl        client.MCPClient
}

func (t *mcpTool) Name() string {
	return t.name
}

func (t *mcpTool) Description() string {
	return t.desc
}

func (t *mcpTool) InputSchema() map[string]any {
	return t.inputSchema
}

func (t *mcpTool) Call(ctx context.Context, input any, w estellm.ResponseWriter) error {
	req := mcp.CallToolRequest{}
	req.Params.Name = t.name
	var args map[string]any
	if err := jsonutil.Remarshal(input, &args); err != nil {
		return fmt.Errorf("failed to remarshal input: %w", err)
	}
	req.Params.Arguments = args
	slog.DebugContext(ctx, "calling mcp tool", "name", t.name, "args", args)
	res, err := t.impl.CallTool(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to call tool `%s`: %w", t.name, err)
	}
	if res.IsError {
		w.WritePart(estellm.TextPart(fmt.Sprintf("error calling tool `%s`: %s", t.name, res.Content)))
		w.Finish(estellm.FinishReasonEndTurn, "error")
		return nil
	}
	meta := w.Metadata()
	slog.DebugContext(ctx, "mcp tool response", "name", t.name, "meta", meta)
	for k, v := range res.Meta {
		meta.Set(k, v)
	}
	for k, v := range res.Result.Meta {
		meta.Set(k, v)
	}
	parts := make([]estellm.ContentPart, 0, len(res.Content))
	for _, content := range res.Content {
		part, err := mcpContentToPart(content)
		if err != nil {
			slog.WarnContext(ctx, "failed to convert content part", "details", err)
			continue
		}
		parts = append(parts, part)
	}
	w.WritePart(parts...)
	w.Finish(estellm.FinishReasonEndTurn, "done")
	return nil
}

func mcpContentToPart(content mcp.Content) (estellm.ContentPart, error) {
	var part estellm.ContentPart
	switch content := content.(type) {
	case *mcp.TextContent:
		part = estellm.TextPart(content.Text)
		return part, nil
	case *mcp.ImageContent:
		data, err := base64.StdEncoding.DecodeString(content.Data)
		if err != nil {
			return part, fmt.Errorf("failed to decode base64 image: %w", err)
		}
		part = estellm.BinaryPart(content.MIMEType, data)
		return part, nil
	case *mcp.EmbeddedResource:
		switch resource := content.Resource.(type) {
		case *mcp.TextResourceContents:
			part = estellm.TextPart(resource.Text)
			return part, nil
		case *mcp.BlobResourceContents:
			data, err := base64.StdEncoding.DecodeString(resource.Blob)
			if err != nil {
				return part, fmt.Errorf("failed to decode base64 blob: %w", err)
			}
			part = estellm.BinaryPart(resource.MIMEType, data)
			return part, nil
		default:
			return part, fmt.Errorf("unknown resource type: %T", resource)
		}
	default:
		return part, fmt.Errorf("unknown content type: %T", content)
	}
}
