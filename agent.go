package estellm

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/google/go-jsonnet"
)

type Agent interface {
	Execute(ctx context.Context, req *Request, w ResponseWriter) error
}

type AgentFunc func(ctx context.Context, req *Request, w ResponseWriter) error

func (f AgentFunc) Execute(ctx context.Context, req *Request, w ResponseWriter) error {
	return f(ctx, req, w)
}

type AgentMux struct {
	defaultAgent     string
	prompts          map[string]*Prompt
	agents           map[string]Agent
	dependents       map[string][]string
	toolsDepenedents map[string][]string
	isCycle          bool
	validate         func() error
	logger           *slog.Logger
	reg              *Registry
	middleware       []func(next Agent) Agent
}

type newAgentMuxOptions struct {
	registry        *Registry
	includesFs      fs.FS
	promptsFs       fs.FS
	extCodes        map[string]string
	extVars         map[string]string
	nativeFunctions []*jsonnet.NativeFunction
	templateFuncs   template.FuncMap
	logger          *slog.Logger
}

type NewAgentMuxOption func(*newAgentMuxOptions)

func WithRegistry(registry *Registry) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.registry = registry
	}
}

func WithIncludesFS(fsys fs.FS) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.includesFs = fsys
	}
}

func WithPromptsFS(fsys fs.FS) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.promptsFs = fsys
	}
}

func WithExtCodes(codes map[string]string) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.extCodes = codes
	}
}

func WithExtVars(vars map[string]string) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.extVars = vars
	}
}

func WithTemplateFuncs(fmap template.FuncMap) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		maps.Copy(o.templateFuncs, fmap)
	}
}

func WithLogger(logger *slog.Logger) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.logger = logger
	}
}

func WithNativeFunctions(functions ...*jsonnet.NativeFunction) NewAgentMuxOption {
	return func(o *newAgentMuxOptions) {
		o.nativeFunctions = append(o.nativeFunctions, functions...)
		slices.SortFunc(o.nativeFunctions, func(i, j *jsonnet.NativeFunction) int {
			return cmp.Compare(i.Name, j.Name)
		})
		o.nativeFunctions = slices.CompactFunc(o.nativeFunctions, func(i, j *jsonnet.NativeFunction) bool {
			return i.Name == j.Name
		})
	}
}

func NewAgentMux(ctx context.Context, optFns ...NewAgentMuxOption) (*AgentMux, error) {
	o := newAgentMuxOptions{
		registry:        defaultRegistory,
		promptsFs:       os.DirFS("prompts"),
		includesFs:      os.DirFS("includes"),
		extCodes:        map[string]string{},
		extVars:         map[string]string{},
		nativeFunctions: []*jsonnet.NativeFunction{},
		templateFuncs:   template.FuncMap{},
		logger:          slog.Default(),
	}
	for _, fn := range optFns {
		fn(&o)
	}
	reg := o.registry
	if reg == nil {
		return nil, fmt.Errorf("registry is required")
	}
	loader := NewLoader()
	loader.Includes(o.includesFs)
	loader.ExtCodes(o.extCodes)
	loader.ExtVars(o.extVars)
	loader.NativeFunctions(o.nativeFunctions...)
	loader.TemplateFuncs(o.templateFuncs)
	loader.Registry(reg)
	prompts, dependents, err := loader.LoadFS(ctx, o.promptsFs)
	if err != nil {
		return nil, err
	}
	toolsDepenedents := make(map[string][]string, len(dependents))
	agents := make(map[string]Agent, len(prompts))
	var defaultAgent string
	for name, p := range prompts {
		agent, err := reg.NewAgent(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("prompt `%s`: %w", name, err)
		}
		agents[name] = agent
		cfg := p.Config()
		toolsDepenedents[name] = cfg.Tools
		for _, tool := range toolsDepenedents[name] {
			if _, ok := dependents[tool]; !ok {
				return nil, fmt.Errorf("prompt `%s`: refarence `%s` as tool, but not found", name, tool)
			}
		}
		if cfg.Default {
			if defaultAgent != "" {
				return nil, fmt.Errorf("multiple default agents: %s, %s", defaultAgent, name)
			}
			defaultAgent = name
		}
	}
	mux := &AgentMux{
		defaultAgent:     defaultAgent,
		prompts:          prompts,
		agents:           agents,
		dependents:       dependents,
		logger:           o.logger,
		toolsDepenedents: toolsDepenedents,
		reg:              reg,
	}
	mux.validate = sync.OnceValue(mux.validateImpl)
	return mux, nil
}

func (mux *AgentMux) Validate() error {
	return mux.validate()
}

func (mux *AgentMux) Use(middleware ...func(next Agent) Agent) {
	mux.middleware = append(mux.middleware, middleware...)
}

func (mux *AgentMux) validateImpl() error {
	merged := make(map[string][]string, len(mux.dependents))
	for name, deps := range mux.dependents {
		merged[name] = slices.Clone(deps)
	}
	for name, tools := range mux.toolsDepenedents {
		merged[name] = append(merged[name], tools...)
	}
	for name, deps := range merged {
		slices.Sort(deps)
		merged[name] = slices.Compact(deps)
	}
	if _, err := topologicalSort(merged); err != nil {
		mux.isCycle = true
		return fmt.Errorf("topological sort: %w", err)
	}
	return nil
}

func (mux *AgentMux) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString("```mermaid\nflowchart TD\n")
	nodes := slices.Collect(maps.Keys(mux.dependents))
	slices.Sort(nodes)
	nodesAlias := make(map[string]string, len(nodes))
	wrapper := make(map[string]func(string) string, len(nodes))
	for i, node := range nodes {
		nodesAlias[node] = fmt.Sprintf("A%d", i)
		if p, ok := mux.prompts[node]; ok {
			wrapper[node] = mux.reg.getMarmaidNodeWrapper(p.Config().Type)
		}
		if wrapper[node] == nil {
			wrapper[node] = func(node string) string {
				return fmt.Sprintf("[%s]", node)
			}
		}
	}
	for _, node := range nodes {
		w := wrapper[node]
		sb.WriteString(fmt.Sprintf("    %s%s\n", nodesAlias[node], w(node)))
	}
	for _, node := range nodes {
		deps := mux.dependents[node]
		for _, dep := range deps {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", nodesAlias[node], nodesAlias[dep]))
		}
		for _, tool := range mux.toolsDepenedents[node] {
			sb.WriteString(fmt.Sprintf("    %s -.->|tool_call| %s\n", nodesAlias[node], nodesAlias[tool]))
		}
	}
	sb.WriteString("```\n")
	return sb.String()
}

func (mux *AgentMux) Execute(ctx context.Context, req *Request, w ResponseWriter) error {
	if err := mux.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if _, err := mux.validateRequest(req); err != nil {
		return err
	}
	graph, ok := pickupDAG(req.Name, mux.dependents)
	if !ok {
		return fmt.Errorf("agent `%s` not found", req.Name)
	}
	if !req.IncludeUpstream {
		graph = extractDownstreamSubgraph(graph, req.Name)
	}
	if !req.IncludeDownstream {
		graph = extractUpstreamSubgraph(graph, req.Name)
	}
	return mux.executeGraph(ctx, graph, req, w)
}

func (mux *AgentMux) executeGraph(ctx context.Context, graph map[string][]string, req *Request, w ResponseWriter) error {
	done := make(map[string]bool, len(graph))
	skipped := make(map[string]bool, len(graph))
	sortedNodes, err := topologicalSort(graph)
	if err != nil {
		return fmt.Errorf("topological sort: %w", err)
	}
	sinkNodes := findSinkNodes(graph)
	previousResults := maps.Clone(req.PreviousResults)
	if previousResults == nil {
		previousResults = make(map[string]*Response, len(graph))
	}
	for _, nodes := range sortedNodes {
		for _, node := range nodes {
			if _, ok := previousResults[node]; ok {
				done[node] = true
			}
			if done[node] {
				continue
			}
			cfg := mux.prompts[node].Config()
			if len(cfg.DependsOn) > 0 {
				allSkiped := true
				for _, dep := range cfg.DependsOn {
					if !skipped[dep] {
						allSkiped = false
						break
					}
				}
				if allSkiped {
					mux.logger.DebugContext(ctx, "skip node", "node", node)
					skipped[node] = true
					done[node] = true
					continue
				}
			}
			refined := mux.refineRequest(cfg, req)
			refined.PreviousResults = previousResults
			resp, err := mux.executeOne(ctx, cfg, refined, w, sinkNodes)
			if err != nil {
				return err
			}
			done[node] = true
			if resp == nil {
				continue
			}
			previousResults[node] = resp
			nextAgents := resp.Metadata.GetStrings(metadataKeyNextAgents)
			if len(nextAgents) == 0 {
				continue
			}
			deps := cfg.Dependents()
			skipTargets := make([]string, 0, len(deps))
			execTargets := make([]string, 0, len(deps))
			for _, dep := range deps {
				if slices.Contains(nextAgents, dep) {
					execTargets = append(execTargets, dep)
				} else {
					skipTargets = append(skipTargets, dep)
				}
			}
			if len(execTargets) == 0 {
				mux.logger.WarnContext(ctx, "next node all skipped", "targets", skipTargets)
				w.Finish(FinishReasonEndTurn, "agents all skipped")
				return nil
			}
			for _, target := range skipTargets {
				skipped[target] = true
				done[target] = true
			}
		}
	}
	return nil
}

func (mux *AgentMux) executeOne(ctx context.Context, cfg *Config, req *Request, w ResponseWriter, sinkNodes []string) (*Response, error) {
	node := cfg.Name
	agent, ok := mux.agents[cfg.Name]
	if !ok {
		return nil, fmt.Errorf("agent `%s` not found", node)
	}
	if !*cfg.Enabled {
		return nil, fmt.Errorf("prompt `%s` is disabled", node)
	}
	for _, mw := range mux.middleware {
		agent = mw(agent)
	}
	w.Metadata().MergeInPlace(cfg.ResponseMetadata)
	mux.logger.DebugContext(ctx, "execute node", "node", node, "metadata", w.Metadata())
	if slices.Contains(sinkNodes, node) {
		if err := agent.Execute(ctx, req, w); err != nil {
			return nil, fmt.Errorf("execute `%s`: %w", node, err)
		}
		return nil, nil
	}
	batchWriter := NewBatchResponseWriter()
	w = NewReasoningMirrorResponseWriter(batchWriter, w)
	if cfg.AsReasoning {
		w = NewAsReasoningResponseWriter(w)
	}
	if err := agent.Execute(ctx, req, w); err != nil {
		return nil, fmt.Errorf("execute `%s`: %w", node, err)
	}
	resp := batchWriter.Response()
	return resp, nil
}

func (mux *AgentMux) refineRequest(cfg *Config, req *Request) *Request {
	if req == nil {
		return nil
	}
	req = req.Clone()
	req.Name = cfg.Name
	req.Metadata = req.Metadata.Merge(cfg.RequestMetadata)
	tools := make(ToolSet, 0, len(cfg.Tools))
	for _, tool := range cfg.Tools {
		toolPrompt, ok := mux.prompts[tool]
		if !ok {
			continue
		}
		toolCfg := toolPrompt.Config()
		if !*toolCfg.Enabled {
			continue
		}
		tools = tools.Append(NewAgentTool(
			tool,
			toolCfg.Description,
			toolCfg.PayloadSchema,
			mux,
		))
	}
	req.Tools = req.Tools.Append(tools...)
	return req
}

func (mux *AgentMux) validateRequest(req *Request) (*Prompt, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.Name == "" {
		req.Name = mux.defaultAgent
	}
	if req.Name == "" {
		return nil, fmt.Errorf("agent name is required")
	}
	p, ok := mux.prompts[req.Name]
	if !ok {
		return nil, fmt.Errorf("agent `%s` not found", req.Name)
	}
	return p, nil
}

func (mux *AgentMux) Render(ctx context.Context, req *Request) (string, error) {
	p, err := mux.validateRequest(req)
	if err != nil {
		return "", err
	}
	return p.Render(ctx, mux.refineRequest(p.Config(), req))
}

func (mux *AgentMux) RenderBlock(ctx context.Context, blockName string, req *Request) (string, error) {
	p, err := mux.validateRequest(req)
	if err != nil {
		return "", err
	}
	if !slices.Contains(p.Blocks(), blockName) {
		return "", fmt.Errorf("block `%s` not found in agent `%s`", blockName, req.Name)
	}
	return p.RenderBlock(ctx, blockName, mux.refineRequest(p.Config(), req))
}

func (mux *AgentMux) RenderConfig(_ context.Context, name string, isJsonnet bool) (string, error) {
	if name == "" {
		name = mux.defaultAgent
	}
	if name == "" {
		return "", fmt.Errorf("agent name is required")
	}
	p, ok := mux.prompts[name]
	if !ok {
		return "", fmt.Errorf("agent `%s` not found", name)
	}
	if isJsonnet {
		return p.Config().Raw, nil
	}
	var v map[string]any
	if err := p.Config().Decode(&v); err != nil {
		return "", err
	}
	bs, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bs), nil
}
