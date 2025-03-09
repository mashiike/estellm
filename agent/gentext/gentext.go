package gentext

import (
	"context"
	"fmt"

	"github.com/mashiike/estellm"
)

const (
	AgentName = "generate_text"
)

func init() {
	err := estellm.RegisterAgent(AgentName, NewAgent)
	if err != nil {
		panic(fmt.Sprintf("failed to register agent %s: %v", AgentName, err))
	}
}

type Config struct {
	ModelProvider string         `json:"model_provider"`
	ModelID       string         `json:"model_id"`
	ModelParams   map[string]any `json:"model_params"`
}

type Agent struct {
	p             *estellm.Prompt
	cfg           *Config
	modelProvider estellm.ModelProvider
}

func NewAgent(ctx context.Context, p *estellm.Prompt) (estellm.Agent, error) {
	var cfg Config
	if err := p.Config().Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode `generate_text` agent config: %w", err)
	}
	if cfg.ModelProvider == "" {
		return nil, fmt.Errorf("model_provider is required")
	}
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	modelProvider, err := estellm.GetModelProvider(ctx, cfg.ModelProvider)
	if err != nil {
		return nil, fmt.Errorf("model_provider `%s`: %w", cfg.ModelProvider, err)
	}
	return &Agent{
		p:             p,
		cfg:           &cfg,
		modelProvider: modelProvider,
	}, nil
}

func (a *Agent) Execute(ctx context.Context, req *estellm.Request, w estellm.ResponseWriter) error {
	system, msgs, err := a.p.Decode(ctx, req)
	if err != nil {
		return fmt.Errorf("decode prompt: %w", err)
	}
	modelReq := &estellm.GenerateTextRequest{
		ModelID:     a.cfg.ModelID,
		ModelParams: a.cfg.ModelParams,
		System:      system,
		Messages:    msgs,
		Tools:       req.Tools,
		Metadata:    req.Metadata,
	}
	return a.modelProvider.GenerateText(ctx, modelReq, w)
}
