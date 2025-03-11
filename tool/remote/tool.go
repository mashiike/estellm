package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/mashiike/estellm"
)

type SpecificationCache struct {
	mu             sync.RWMutex
	cache          map[string]Specification
	cacheAt        map[string]time.Time
	expireDuration time.Duration
}

func NewSpecificationCache(expireDuration time.Duration) *SpecificationCache {
	return &SpecificationCache{
		cache:          make(map[string]Specification),
		cacheAt:        make(map[string]time.Time),
		expireDuration: expireDuration,
	}
}

func (sc *SpecificationCache) Get(name string) (Specification, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	spec, ok := sc.cache[name]
	if !ok {
		return Specification{}, false
	}
	at, ok := sc.cacheAt[name]
	if !ok {
		return Specification{}, false
	}
	if flextime.Since(at) > sc.expireDuration {
		sc.mu.RUnlock()
		sc.Delete(name)
		sc.mu.RLock()
		return Specification{}, false
	}
	return spec, ok
}

func (sc *SpecificationCache) Set(name string, spec Specification) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[name] = spec
	sc.cacheAt[name] = flextime.Now()
}

func (sc *SpecificationCache) Delete(name string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.cache, name)
	delete(sc.cacheAt, name)
}

var DefaultSpecificationCache = NewSpecificationCache(15 * time.Minute)

type Tool struct {
	endpoint      *url.URL
	baseEndpoint  *url.URL
	spec          Specification
	newReqFunc    RequestConstructor
	inputSchema   map[string]any
	client        *http.Client
	newErr        func(error) (types.ToolResultBlock, error)
	signer        func(*http.Request, string) (*http.Request, error)
	respValidator func(*http.Response, *http.Request) error
}

type RequestConstructor func(ctx context.Context, method string, url string, toolName string, input any) (*http.Request, error)

var DefaultRequestConstructor = func(ctx context.Context, method string, url string, toolName string, input any) (*http.Request, error) {
	bs, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bs))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderToolName, toolName)
	if toolUseID, ok := estellm.ToolUseIDFromContext(ctx); ok {
		req.Header.Set(HeaderToolUseID, toolUseID)
	}
	return req, nil
}

type ToolConfig struct {
	Endpoint           string
	SpecificationPath  string
	SpecificationCache *SpecificationCache
	RequestConstructor RequestConstructor
	HTTPClient         *http.Client
	ErrorConstractor   func(error) (types.ToolResultBlock, error)
	RequestSigner      func(req *http.Request, subject string) (*http.Request, error)
	ResponseValidator  func(resp *http.Response, req *http.Request) error
}

type remoteWorker struct {
	tool *Tool
}

func NewTool(ctx context.Context, cfg ToolConfig) (*Tool, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	if cfg.SpecificationCache == nil {
		cfg.SpecificationCache = DefaultSpecificationCache
	}
	if cfg.RequestConstructor == nil {
		cfg.RequestConstructor = DefaultRequestConstructor
	}
	if cfg.SpecificationPath == "" {
		cfg.SpecificationPath = DefaultSpecificationPath
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.ErrorConstractor == nil {
		cfg.ErrorConstractor = func(err error) (types.ToolResultBlock, error) {
			return types.ToolResultBlock{}, err
		}
	}
	if cfg.RequestSigner == nil {
		cfg.RequestSigner = func(req *http.Request, _ string) (*http.Request, error) {
			return req, nil
		}
	}
	if cfg.ResponseValidator == nil {
		cfg.ResponseValidator = func(_ *http.Response, _ *http.Request) error {
			return nil
		}
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	t := &Tool{
		endpoint:      u.JoinPath(cfg.SpecificationPath),
		baseEndpoint:  u,
		newReqFunc:    cfg.RequestConstructor,
		client:        cfg.HTTPClient,
		newErr:        cfg.ErrorConstractor,
		signer:        cfg.RequestSigner,
		respValidator: cfg.ResponseValidator,
	}
	spec, ok := cfg.SpecificationCache.Get(u.String())
	if !ok {
		spec, err = t.fetchSpecification(ctx)
		if err != nil {
			return nil, err
		}
		cfg.SpecificationCache.Set(u.String(), spec)
	}
	t.spec = spec
	if err := json.Unmarshal(spec.InputSchema, &t.inputSchema); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Tool) SignRequest(req *http.Request, subject string) (*http.Request, error) {
	return t.signer(req, subject)
}

func (t *Tool) fetchSpecification(ctx context.Context) (Specification, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.endpoint.String(), nil)
	if err != nil {
		return Specification{}, err
	}
	req, err = t.signer(req, "specification")
	if err != nil {
		return Specification{}, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return Specification{}, err
	}
	defer resp.Body.Close()
	if err := t.respValidator(resp, req); err != nil {
		return Specification{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return Specification{}, errors.New("failed to fetch specification")
	}
	var spec Specification
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return Specification{}, err
	}
	workerEndpoint, err := url.Parse(spec.WorkerEndpoint)
	if err != nil {
		return Specification{}, fmt.Errorf("failed to parse worker endpoint; %w", err)
	}
	if !workerEndpoint.IsAbs() {
		workerEndpoint = t.baseEndpoint.ResolveReference(workerEndpoint)
	}
	spec.WorkerEndpoint = workerEndpoint.String()
	return spec, nil
}

var _ estellm.Tool = (*Tool)(nil)

func (t *Tool) Name() string {
	return t.spec.Name
}

func (t *Tool) Description() string {
	return t.spec.Description
}

func (t *Tool) Specification() Specification {
	return t.spec
}

func (t *Tool) InputSchema() map[string]any {
	return t.inputSchema
}

func (t *Tool) Call(ctx context.Context, input any, w estellm.ResponseWriter) error {
	req, err := t.newReqFunc(ctx, http.MethodPost, t.spec.WorkerEndpoint, t.Name(), input)
	if err != nil {
		return err
	}
	subject := "tool/" + t.Name()
	if toolUseID, ok := estellm.ToolUseIDFromContext(ctx); ok {
		subject += "/" + toolUseID
	}
	req, err = t.signer(req, subject)
	if err != nil {
		return err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	if err := t.respValidator(resp, req); err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("status code is not 200")
	}
	var tr ToolResult
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return err
	}
	parts, err := tr.MarshalParts()
	if err != nil {
		return err
	}
	w.WritePart(parts...)
	w.Finish(estellm.FinishReasonEndTurn, tr.Status)
	return nil
}
