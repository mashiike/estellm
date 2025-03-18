package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mashiike/estellm"
	"github.com/mashiike/estellm/mcp"
	"github.com/mashiike/slogutils"
)

type CLI struct {
	LogFormat string            `help:"Log format" enum:"json,text" default:"json" env:"LOG_FORMAT"`
	Color     bool              `help:"Enable color output" negatable:"" default:"true"`
	Debug     bool              `help:"Enable debug mode" env:"DEBUG"`
	ExtVar    map[string]string `help:"External variables external string values for Jsonnet" env:"EXT_VAR"`
	ExtCode   map[string]string `help:"External code external string values for Jsonnet" env:"EXT_CODE"`
	Project   string            `cmd:"" help:"Project directory" default:"./" env:"ESTELLM_PROJECT"`
	Prompts   string            `cmd:"" help:"Prompts directory" default:"./prompts" env:"ESTELLM_PROMPTS"`
	Includes  string            `cmd:"" help:"Includes directory" default:"./includes" env:"ESTELLM_INCLUDES"`
	Exec      ExecOption        `cmd:"" help:"Execute the estellm"`
	Render    RenderOption      `cmd:"" help:"Render prompt/config the estellm"`
	Docs      DocsOptoin        `cmd:"" help:"Show agents documentation"`
	Serve     ServeOption       `cmd:"" help:"Serve agents as MCP(Model Context Protocol) server"`
	Version   struct{}          `cmd:"" help:"Show version"`
}

func newLogger(level slog.Level, format string, c bool) *slog.Logger {
	var f func(io.Writer, *slog.HandlerOptions) slog.Handler
	switch format {
	case "text":
		f = func(w io.Writer, ho *slog.HandlerOptions) slog.Handler {
			return slog.NewTextHandler(w, ho)
		}
	default:
		f = func(w io.Writer, ho *slog.HandlerOptions) slog.Handler {
			return slog.NewJSONHandler(w, ho)
		}
	}
	var modifierFuncs map[slog.Level]slogutils.ModifierFunc
	if c {
		modifierFuncs = map[slog.Level]slogutils.ModifierFunc{
			slog.LevelDebug: slogutils.Color(color.FgBlack),
			slog.LevelInfo:  nil,
			slog.LevelWarn:  slogutils.Color(color.FgYellow),
			slog.LevelError: slogutils.Color(color.FgRed, color.Bold),
		}
	}
	middleware := slogutils.NewMiddleware(
		f,
		slogutils.MiddlewareOptions{
			Writer:        os.Stderr,
			ModifierFuncs: modifierFuncs,
			HandlerOptions: &slog.HandlerOptions{
				Level: level,
			},
		},
	)
	logger := slog.New(middleware)
	return logger
}

func (c *CLI) Run(ctx context.Context) int {
	k := kong.Parse(c,
		kong.Name("estellm"),
		kong.Description("Estellm is a tool for llm agetnts flow control."),
		kong.UsageOnError(),
		kong.Vars{"version": estellm.Version},
	)
	logLevel := slog.LevelInfo
	if c.Debug {
		logLevel = slog.LevelDebug
	}
	logger := newLogger(logLevel, c.LogFormat, c.Color)
	slog.SetDefault(logger)
	if err := c.run(ctx, k, logger); err != nil {
		logger.Error("runtime error", "details", err)
		return 1
	}
	return 0
}

func (c *CLI) run(ctx context.Context, k *kong.Context, logger *slog.Logger) error {
	var err error
	cmd := k.Command()
	if cmd == "version" {
		fmt.Printf("estellm version %s\n", estellm.Version)
		return nil
	}
	mux, err := c.newAgentMux(ctx, logger)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	switch cmd {
	case "exec <prompt-name>", "exec":
		return c.runExec(ctx, mux)
	case "render", "render <prompt-name>", "render <prompt-name> <target>":
		return c.runRender(ctx, mux)
	case "docs":
		return c.runDocs(ctx, mux)
	case "serve":
		serverVersion := estellm.Version
		if c.Serve.Version != "" {
			serverVersion = c.Serve.Version
		}
		s := mcp.NewServer(c.Serve.ServerName, serverVersion, mux)
		switch c.Serve.Transport {
		case "stdio":
			slog.InfoContext(ctx, "start mcp server as stdio")
			return s.ServeStdio()
		case "sse":
			serverOptions := []server.SSEOption{}
			if c.Serve.BaseURL != "" {
				serverOptions = append(serverOptions, server.WithBaseURL(c.Serve.BaseURL))
			}
			return s.ListenAndServeSSE(fmt.Sprintf(":%d", c.Serve.Port), serverOptions...)
		default:
			return fmt.Errorf("unknown transport: %s", c.Serve.Transport)
		}
	default:
		return fmt.Errorf("unknown command: %s", k.Command())
	}
}

func (c *CLI) runExec(ctx context.Context, mux *estellm.AgentMux) error {
	data, err := c.Exec.ParsePayload()
	if err != nil {
		return fmt.Errorf("new execute input: %w", err)
	}
	req, err := estellm.NewRequest(c.Exec.PromptName, data)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.IncludeUpstream = c.Exec.IncludeUpstream
	req.IncludeDownstream = c.Exec.IncludeDownstream
	switch c.Exec.OutputFormat {
	case "json":
		w := estellm.NewBatchResponseWriter()
		if err := mux.Execute(ctx, req, w); err != nil {
			return fmt.Errorf("execute prompt: %w", err)
		}
		resp := w.Response()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode state: %w", err)
		}
	case "text":
		w := estellm.NewTextStreamingResponseWriter(os.Stdout)
		if c.Exec.FileOutput != "" {
			w.SetBinaryOutputDir(c.Exec.FileOutput)
		}
		if err := mux.Execute(ctx, req, w); err != nil {
			return fmt.Errorf("execute prompt: %w", err)
		}
		if c.Exec.DumpMetadata {
			w.DumpMetadata()
		}
		fmt.Println()
	default:
		return fmt.Errorf("unknown output format: %s", c.Exec.OutputFormat)
	}
	return nil
}

func (c *CLI) runRender(ctx context.Context, mux *estellm.AgentMux) error {
	data, err := c.Render.ParsePayload()
	if err != nil {
		return fmt.Errorf("new execute input: %w", err)
	}
	req, err := estellm.NewRequest(c.Render.PromptName, data)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	switch c.Render.Target {
	case "":
		prompt, err := mux.Render(ctx, req)
		if err != nil {
			return fmt.Errorf("render prompt: %w", err)
		}
		fmt.Println(prompt)
		return nil
	case "config":
		rendered, err := mux.RenderConfig(ctx, c.Render.PromptName, c.Render.Jsonnet)
		if err != nil {
			return fmt.Errorf("render config: %w", err)
		}
		fmt.Println(rendered)
		return nil
	default:
		rendered, err := mux.RenderBlock(ctx, c.Render.Target, req)
		if err != nil {
			return fmt.Errorf("render block: %w", err)
		}
		fmt.Println(rendered)
		return nil
	}
}

func (c *CLI) runDocs(_ context.Context, mux *estellm.AgentMux) error {
	fmt.Println(mux.ToMarkdown())
	return nil
}

func (c *CLI) newAgentMux(ctx context.Context, logger *slog.Logger) (*estellm.AgentMux, error) {
	promptsDir := filepath.Join(c.Project, c.Prompts)
	includesDir := filepath.Join(c.Project, c.Includes)
	logger.InfoContext(ctx, "load prompts", "prompts", promptsDir, "includes", includesDir)
	if _, err := os.Stat(promptsDir); err != nil {
		return nil, fmt.Errorf("prompts directory: %w", err)
	}
	promptsFS := os.DirFS(promptsDir)
	opts := []estellm.NewAgentMuxOption{
		estellm.WithLogger(logger),
		estellm.WithPromptsFS(promptsFS),
	}
	if _, err := os.Stat(includesDir); err == nil {
		includesFS := os.DirFS(includesDir)
		opts = append(opts, estellm.WithIncludesFS(includesFS))
	}
	if c.ExtCode != nil {
		opts = append(opts, estellm.WithExtCodes(c.ExtCode))
	}
	if c.ExtVar != nil {
		opts = append(opts, estellm.WithExtVars(c.ExtVar))
	}
	return estellm.NewAgentMux(ctx, opts...)
}

func (e *PromptOption) ParsePayload() (map[string]any, error) {
	if e.Payload == nil {
		stat, err := os.Stdin.Stat()
		if err != nil {
			slog.Debug("failed to get stdin stat", "error", err)
			return map[string]any{}, nil
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return map[string]any{}, nil
		}
		e.Payload, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
	}
	var data map[string]any
	if err := json.Unmarshal(e.Payload, &data); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	slog.Debug("parsed payload", "data", data)
	return data, nil
}

type PromptOption struct {
	PromptName string `arg:"" help:"Prompt name" default:""`
	Payload    []byte `help:"Execution Payload"`
}

type ExecOption struct {
	PromptOption
	OutputFormat      string `help:"Output format" enum:"json,text" default:"text"`
	IncludeUpstream   bool   `help:"Include upstream dependencies" negatable:""`
	IncludeDownstream bool   `help:"Include downstream dependencies" default:"true" negatable:""`
	DumpMetadata      bool   `help:"Dump metadata if output format is text"`
	FileOutput        string `help:"Output file dir" default:"generated"`
}

type RenderOption struct {
	PromptOption
	Target  string `arg:"" help:"rendering target" default:""`
	Jsonnet bool   `help:"if render target is \"config\", render as jsonnet"`
}

type DocsOptoin struct {
}

type ServeOption struct {
	Transport  string `help:"Transport type" enum:"stdio,sse" default:"stdio" required:"" env:"ESTELLM_TRANSPORT" short:"t"`
	ServerName string `help:"Server name" default:"estellm" env:"ESTELLM_SERVER_NAME"`
	Version    string `help:"Server version" default:"" env:"ESTELLM_SERVER_VERSION"`
	Port       int    `help:"Server port" default:"8080" env:"ESTELLM_SERVER_PORT"`
	BaseURL    string `help:"Server base URL" default:"" env:"ESTELLM_SERVER_BASE_URL"`
}
