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
	"github.com/mashiike/estellm"

	//builtin agents import
	_ "github.com/mashiike/estellm/agent/constant"
)

type CLI struct {
	Debug        bool              `help:"Enable debug mode" env:"DEBUG"`
	SetLogLevvel func(slog.Level)  `kong:"-"`
	ExtVar       map[string]string `help:"External variables external string values for Jsonnet" env:"EXT_VAR"`
	ExtCode      map[string]string `help:"External code external string values for Jsonnet" env:"EXT_CODE"`
	Project      string            `cmd:"" help:"Project directory" default:"./"`
	Prompts      string            `cmd:"" help:"Prompts directory" default:"./prompts"`
	Includes     string            `cmd:"" help:"Includes directory" default:"./includes"`
	Exec         ExecOption        `cmd:"" help:"Execute the estellm"`
	Rendoer      RenderOption      `cmd:"" help:"Render prompt/config the estellm"`
	Docs         DocsOptoin        `cmd:"" help:"Show agents documentation"`
	Version      struct{}          `cmd:"" help:"Show version"`
}

func New(setLogLevel func(slog.Level)) (*CLI, error) {
	return &CLI{
		SetLogLevvel: setLogLevel,
	}, nil
}

func (c *CLI) Run(ctx context.Context) error {
	k := kong.Parse(c,
		kong.Name("estellm"),
		kong.Description("Estellm is a tool for llm agetnts flow control."),
		kong.UsageOnError(),
	)
	var err error
	if c.Debug && c.SetLogLevvel != nil {
		c.SetLogLevvel(slog.LevelDebug)
	}
	cmd := k.Command()
	if cmd == "version" {
		fmt.Printf("estellm version %s\n", estellm.Version)
		return nil
	}
	mux, err := c.newAgentMux(ctx)
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
	w := estellm.NewBatchResponseWriter()
	if err := mux.Execute(ctx, req, w); err != nil {
		return fmt.Errorf("execute prompt: %w", err)
	}
	resp := w.Response()
	switch c.Exec.OutputFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode state: %w", err)
		}
	case "text":
		enc := estellm.NewMessageEncoder(os.Stdout)
		if err := enc.EncodeMessage(resp.Message); err != nil {
			return fmt.Errorf("encode messages: %w", err)
		}
	}
	return nil
}

func (c *CLI) runRender(ctx context.Context, mux *estellm.AgentMux) error {
	data, err := c.Rendoer.ParsePayload()
	if err != nil {
		return fmt.Errorf("new execute input: %w", err)
	}
	req, err := estellm.NewRequest(c.Rendoer.PromptName, data)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	switch c.Rendoer.Target {
	case "":
		prompt, err := mux.Render(ctx, req)
		if err != nil {
			return fmt.Errorf("render prompt: %w", err)
		}
		fmt.Println(prompt)
		return nil
	case "config":
		rendered, err := mux.RenderConfig(ctx, c.Rendoer.PromptName, c.Rendoer.Jsonnet)
		if err != nil {
			return fmt.Errorf("render config: %w", err)
		}
		fmt.Println(rendered)
		return nil
	default:
		rendered, err := mux.RenderBlock(ctx, c.Rendoer.Target, req)
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

func (c *CLI) newAgentMux(ctx context.Context) (*estellm.AgentMux, error) {
	promptsDir := filepath.Join(c.Project, c.Prompts)
	includesDir := filepath.Join(c.Project, c.Includes)
	slog.InfoContext(ctx, "load prompts", "prompts", promptsDir, "includes", includesDir)
	if _, err := os.Stat(promptsDir); err != nil {
		return nil, fmt.Errorf("prompts directory: %w", err)
	}
	promptsFS := os.DirFS(promptsDir)
	opts := []estellm.NewAgentMuxOption{
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
}

type RenderOption struct {
	PromptOption
	Target  string `arg:"" help:"rendering target" default:""`
	Jsonnet bool   `help:"if render target is \"config\", render as jsonnet"`
}

type DocsOptoin struct {
}
