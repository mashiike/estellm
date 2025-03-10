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
	"github.com/mashiike/estellm"
	"github.com/mashiike/slogutils"
)

type CLI struct {
	LogFormat string            `help:"Log format" enum:"json,text" default:"json"`
	Color     bool              `help:"Enable color output" negatable:"" default:"true"`
	Debug     bool              `help:"Enable debug mode" env:"DEBUG"`
	Verbose   bool              `help:"Enable log verbose mode" env:"VERBOSE"`
	ExtVar    map[string]string `help:"External variables external string values for Jsonnet" env:"EXT_VAR"`
	ExtCode   map[string]string `help:"External code external string values for Jsonnet" env:"EXT_CODE"`
	Project   string            `cmd:"" help:"Project directory" default:"./"`
	Prompts   string            `cmd:"" help:"Prompts directory" default:"./prompts"`
	Includes  string            `cmd:"" help:"Includes directory" default:"./includes"`
	Exec      ExecOption        `cmd:"" help:"Execute the estellm"`
	Render    RenderOption      `cmd:"" help:"Render prompt/config the estellm"`
	Docs      DocsOptoin        `cmd:"" help:"Show agents documentation"`
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
	)
	logLevel := slog.LevelInfo
	if c.Debug {
		logLevel = slog.LevelDebug
	}
	logger := newLogger(logLevel, c.LogFormat, c.Color)
	if c.Verbose {
		slog.SetDefault(logger)
	}
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
	case "exec <prompt-name>":
		return c.runExec(ctx, mux)
	case "render <prompt-name>":
		return c.runRender(ctx, mux)
	case "docs":
		return c.runDocs(ctx, mux)
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
	req.IncludeDeps = c.Exec.IncludeDeps
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
	PromptName string `arg:"" help:"Prompt name"`
	Payload    []byte `help:"Execution Payload"`
}

type ExecOption struct {
	PromptOption
	OutputFormat string `help:"Output format" enum:"json,text" default:"text"`
	IncludeDeps  bool   `help:"Include upstream dependencies"`
	DumpMetadata bool   `help:"Dump metadata if output format is text"`
}

type RenderOption struct {
	PromptOption
	Target  string `arg:"" help:"rendering target" default:""`
	Jsonnet bool   `help:"if render target is \"config\", render as jsonnet"`
}

type DocsOptoin struct {
}
