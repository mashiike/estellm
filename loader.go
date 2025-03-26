package estellm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"text/template"

	"github.com/google/go-jsonnet"
	"github.com/mashiike/estellm/jsonutil"
	aliasimporter "github.com/mashiike/go-jsonnet-alias-importer"
)

type Loader struct {
	importer        *aliasimporter.AliasImpoter
	includesFS      fs.FS
	extCodes        map[string]string
	extVars         map[string]string
	nativeFunctions map[string]*jsonnet.NativeFunction
	tmpl            *template.Template
	prepareTemplate func() (*template.Template, error)
	patterns        []string
	gen             jsonutil.ValueGenerator
	reg             *Registry
}

func NewLoader() *Loader {
	tmpl := template.New("prompt")
	l := &Loader{
		importer:        aliasimporter.New(),
		tmpl:            tmpl,
		gen:             jsonutil.DefaultSchemaValueGenerator,
		patterns:        []string{"*.md", "*.mdx"},
		extCodes:        make(map[string]string),
		extVars:         make(map[string]string),
		nativeFunctions: make(map[string]*jsonnet.NativeFunction),
		reg:             defaultRegistory,
	}
	l.resetPrepare()
	return l
}

func (l *Loader) makeVM() *jsonnet.VM {
	vm := jsonutil.MakeVM()
	for k, v := range l.extVars {
		vm.ExtVar(k, v)
	}
	for k, v := range l.extCodes {
		vm.ExtCode(k, v)
	}
	vm.Importer(l.importer)
	for _, f := range l.nativeFunctions {
		vm.NativeFunction(f)
	}
	return vm
}

func (l *Loader) Registry(reg *Registry) {
	l.reg = reg
	l.resetPrepare()
}

func (l *Loader) ExtVars(extVars map[string]string) {
	l.extVars = extVars
	l.resetPrepare()
}

func (l *Loader) ExtCodes(extCodes map[string]string) {
	l.extCodes = extCodes
	l.resetPrepare()
}

func (l *Loader) NativeFunctions(nativeFunctions ...*jsonnet.NativeFunction) {
	for _, f := range nativeFunctions {
		l.nativeFunctions[f.Name] = f
	}
	l.resetPrepare()
}

func (l *Loader) Includes(fsys fs.FS) {
	l.includesFS = fsys
	l.importer.Register("includes", fsys)
	l.resetPrepare()
}

func (l *Loader) TemplateFuncs(fmap template.FuncMap) {
	l.tmpl = l.tmpl.Funcs(fmap)
	l.resetPrepare()
}

func (l *Loader) ValueGenerator(gen jsonutil.ValueGenerator) {
	l.gen = gen
}

func (l *Loader) PromptPathPatterns(patterns []string) {
	l.patterns = patterns
	l.resetPrepare()
}

func (l *Loader) load(_ context.Context, fsys fs.FS, promptPath string) (*Prompt, error) {
	tmpl, err := l.parseTemplate(fsys, promptPath)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	cfg, err := l.parseConfig(tmpl, promptPath)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	preRendered, err := l.preRender(tmpl, cfg)
	if err != nil {
		return nil, fmt.Errorf("pre-render: %w", err)
	}
	p := &Prompt{
		cfg:         cfg,
		tmpl:        tmpl,
		preRendered: preRendered,
		reg:         l.reg,
	}
	return p, nil
}

func (l *Loader) Load(ctx context.Context, fsys fs.FS, promptPath string) (*Prompt, error) {
	l.importer.Register("prompts", fsys)
	l.importer.ClearCache()
	return l.load(ctx, fsys, promptPath)
}

func (l *Loader) LoadFS(ctx context.Context, fsys fs.FS) (map[string]*Prompt, map[string][]string, error) {
	l.importer.Register("prompts", fsys)
	l.importer.ClearCache()
	prompts := make(map[string]*Prompt, 1)
	paths, err := recursiveGlob(fsys, l.patterns...)
	if err != nil {
		return nil, nil, fmt.Errorf("walk prompts: %w", err)
	}
	for _, path := range paths {
		p, err := l.load(ctx, fsys, path)
		if err != nil {
			return nil, nil, fmt.Errorf("load prompt for `%s`: %w", path, err)
		}
		name := p.Name()
		if _, ok := prompts[name]; ok {
			return nil, nil, fmt.Errorf("duplicate prompt name: %s", name)
		}
		prompts[name] = p
	}
	dependents, err := l.checkDependencies(prompts)
	if err != nil {
		return nil, nil, fmt.Errorf("check dependencies: %w", err)
	}
	for name, p := range prompts {
		p.cfg.AppendDependents(dependents[name]...)
	}
	return prompts, dependents, nil
}

func (l *Loader) checkDependencies(prompts map[string]*Prompt) (map[string][]string, error) {
	dependents := make(map[string][]string, len(prompts))
	for name := range prompts {
		dependents[name] = []string{}
	}
	for name, p := range prompts {
		cfg := p.Config()
		for _, dep := range cfg.DependsOn {
			if _, ok := prompts[dep]; !ok {
				return nil, fmt.Errorf("prompt `%s` depends on `%s` but not found", name, dep)
			}
			dependents[dep] = append(dependents[dep], name)
		}
	}
	for name := range prompts {
		slices.Sort(dependents[name])
		dependents[name] = slices.Compact(dependents[name])
		relatedPrompts := maps.Clone(prompts)
		delete(relatedPrompts, name)
		prompts[name].relatedPrompts = relatedPrompts
	}
	return dependents, nil
}

func (l *Loader) preRender(tmpl *template.Template, cfg *Config) (string, error) {
	tmpl, err := tmpl.Clone()
	if err != nil {
		return "", fmt.Errorf("clone template: %w", err)
	}
	tmpl = tmpl.Funcs(PreRenderPhaseTemplateFuncs(l.reg, cfg))
	dummyData, err := l.gen.Generate(cfg.PayloadSchema)
	if err != nil {
		return "", fmt.Errorf("generate dummy data: %w", err)
	}
	req, err := NewRequest(cfg.Name, dummyData)
	if err != nil {
		return "", fmt.Errorf("new dummy request: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, path.Base(cfg.PromptPath), req.TemplateData()); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (l *Loader) parseConfig(tmpl *template.Template, promptPath string) (*Config, error) {
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "config", nil); err != nil {
		return nil, fmt.Errorf("render config: %w", err)
	}
	raw := buf.String()
	vm := l.makeVM()
	return newConfig(vm, raw, promptPath)
}

func (l *Loader) parseTemplate(fsys fs.FS, promptPath string) (*template.Template, error) {
	tmpl, err := l.prepareTemplate()
	if err != nil {
		return nil, fmt.Errorf("prepare template: %w", err)
	}
	tmpl, err = tmpl.Clone()
	if err != nil {
		return nil, fmt.Errorf("clone template: %w", err)
	}
	tmpl, err = tmpl.ParseFS(fsys, promptPath)
	if err != nil {
		return nil, fmt.Errorf("parse prompt: %w", err)
	}
	return tmpl, nil
}

func (l *Loader) resetPrepare() {
	l.prepareTemplate = sync.OnceValues(l.prepareTemplateImpl)
}

func (l *Loader) prepareTemplateImpl() (*template.Template, error) {
	tmpl := l.tmpl.Funcs(ConfigLoadPhaseTemplateFuncs(l.reg))
	if l.includesFS == nil {
		return tmpl, nil
	}
	if _, err := fs.Stat(l.includesFS, "."); errors.Is(err, fs.ErrNotExist) {
		return tmpl, nil
	}
	paths, err := recursiveGlob(l.includesFS, l.patterns...)
	if err != nil {
		return nil, fmt.Errorf("walk includes: %w", err)
	}
	if len(paths) > 0 {
		var err error
		tmpl, err = tmpl.ParseFS(l.includesFS, paths...)
		if err != nil {
			return nil, fmt.Errorf("parse includes: %w", err)
		}
	}
	return tmpl, nil
}

func recursiveGlob(fsys fs.FS, patterns ...string) ([]string, error) {
	var matchedPaths []string
	err := fs.WalkDir(fsys, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, e.Name())
			if err != nil {
				return err
			}
			if matched {
				matchedPaths = append(matchedPaths, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matchedPaths, nil
}
