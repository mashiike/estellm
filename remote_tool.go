package estellm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

type RemoteToolResult struct {
	Content []ToolResultContent `json:"content,omitempty"`
	Status  string              `json:"status,omitempty"`
}

func (tr RemoteToolResult) MarshalParts() ([]ContentPart, error) {
	content := make([]ContentPart, 0, len(tr.Content))
	for _, c := range tr.Content {
		tc, err := c.MarshalPart()
		if err != nil {
			return nil, err
		}
		content = append(content, tc)
	}
	return content, nil
}

func (tr *RemoteToolResult) UnmarshalParts(parts []ContentPart) error {
	tr.Content = make([]ToolResultContent, 0, len(parts))
	for _, part := range parts {
		var tc ToolResultContent
		if err := tc.UnmarshalPart(part); err != nil {
			return err
		}
		tr.Content = append(tr.Content, tc)
	}
	return nil
}

type ToolResultContent struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Format string `json:"format,omitempty"`
	JSON   string `json:"json,omitempty"`
	Name   string `json:"name,omitempty"`
	Source []byte `json:"source,omitempty"`
}

func (trc ToolResultContent) MarshalPart() (ContentPart, error) {
	switch trc.Type {
	case "text":
		return TextPart(trc.Text), nil
	case "json":
		return TextPart(trc.JSON), nil
	case "reasoning":
		return ReasoningPart(trc.Text), nil
	case "document":
		mimeType := trc.Format
		switch mimeType {
		case "pdf":
			mimeType = "application/" + mimeType
		case "csv", "html":
			mimeType = "text/" + mimeType
		case "doc", "docx":
			mimeType = "application/msword"
		case "xls", "xlsx":
			mimeType = "application/vnd.ms-excel"
		case "txt":
			mimeType = "text/plain"
		case "md":
			mimeType = "text/markdown"
		}
		part := BinaryPart(mimeType, trc.Source)
		part.Name = trc.Name
		return part, nil
	case "image":
		return BinaryPart("image/"+trc.Format, trc.Source), nil
	default:
		return ContentPart{}, fmt.Errorf("unsupported content type: %s", trc.Type)
	}
}

func (trc *ToolResultContent) UnmarshalPart(part ContentPart) error {
	switch part.Type {
	case PartTypeText:
		if json.Valid([]byte(part.Text)) {
			trc.Type = "json"
			trc.JSON = part.Text
		} else {
			trc.Type = "text"
			trc.Text = part.Text
		}
	case PartTypeBinary:
		if part.Name != "" {
			trc.Name = part.Name
		}
		switch {
		case part.MIMEType == "application/pdf":
			trc.Type = "document"
			trc.Format = "pdf"
		case part.MIMEType == "text/csv":
			trc.Type = "document"
			trc.Format = "csv"
		case part.MIMEType == "text/html":
			trc.Type = "document"
			trc.Format = "html"
		case part.MIMEType == "application/msword":
			trc.Type = "document"
			trc.Format = "doc"
		case part.MIMEType == "application/vnd.ms-excel":
			trc.Type = "document"
			trc.Format = "xls"
		case part.MIMEType == "text/plain":
			trc.Type = "document"
			trc.Format = "txt"
		case part.MIMEType == "text/markdown":
			trc.Type = "document"
			trc.Format = "md"
		case part.MIMEType == "image/jpeg":
			trc.Type = "image"
			trc.Format = "jpeg"
		case part.MIMEType == "image/png":
			trc.Type = "image"
			trc.Format = "png"
		case part.MIMEType == "image/gif":
			trc.Type = "image"
			trc.Format = "gif"
		case part.MIMEType == "image/webp":
			trc.Type = "image"
			trc.Format = "webp"
		default:
			return fmt.Errorf("unsupported binary type: %s", part.MIMEType)
		}
		trc.Source = part.Data
	case PartTypeReasoning:
		trc.Type = "reasoning"
		trc.Text = part.Text
	default:
		return fmt.Errorf("unsupported content type: %s", part.Type)
	}
	return nil
}

type RemoteTool struct {
	endpoint      *url.URL
	baseEndpoint  *url.URL
	spec          Specification
	newReqFunc    func(ctx context.Context, method string, url string, toolName string, input any) (*http.Request, error)
	inputSchema   map[string]any
	client        *http.Client
	signer        func(*http.Request, string) (*http.Request, error)
	respValidator func(*http.Response, *http.Request) error
}

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
	if toolUseID, ok := ToolUseIDFromContext(ctx); ok {
		req.Header.Set(HeaderToolUseID, toolUseID)
	}
	return req, nil
}

type RemoteToolConfig struct {
	Endpoint           string
	SpecificationPath  string
	SpecificationCache *SpecificationCache
	RequestConstructor func(ctx context.Context, method string, url string, toolName string, input any) (*http.Request, error)
	HTTPClient         *http.Client
	ErrorWriter        func(error, ResponseWriter) error
	RequestSigner      func(req *http.Request, subject string) (*http.Request, error)
	ResponseValidator  func(resp *http.Response, req *http.Request) error
}

type remoteWorker struct {
	tool *Tool
}

func NewRemoteTool(ctx context.Context, cfg RemoteToolConfig) (*RemoteTool, error) {
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
	if cfg.ErrorWriter == nil {
		cfg.ErrorWriter = func(err error, _ ResponseWriter) error {
			return err
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
	t := &RemoteTool{
		endpoint:      u.JoinPath(cfg.SpecificationPath),
		baseEndpoint:  u,
		newReqFunc:    cfg.RequestConstructor,
		client:        cfg.HTTPClient,
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

func (t *RemoteTool) SignRequest(req *http.Request, subject string) (*http.Request, error) {
	return t.signer(req, subject)
}

func (t *RemoteTool) fetchSpecification(ctx context.Context) (Specification, error) {
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

func (t *RemoteTool) Name() string {
	return t.spec.Name
}

func (t *RemoteTool) Description() string {
	return t.spec.Description
}

func (t *RemoteTool) Specification() Specification {
	return t.spec
}

func (t *RemoteTool) InputSchema() map[string]any {
	return t.inputSchema
}

func (t *RemoteTool) Call(ctx context.Context, input any, w ResponseWriter) error {
	req, err := t.newReqFunc(ctx, http.MethodPost, t.spec.WorkerEndpoint, t.Name(), input)
	if err != nil {
		return err
	}
	subject := "tool/" + t.Name()
	if toolUseID, ok := ToolUseIDFromContext(ctx); ok {
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
	var tr RemoteToolResult
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return err
	}
	parts, err := tr.MarshalParts()
	if err != nil {
		return err
	}
	w.WritePart(parts...)
	w.Finish(FinishReasonEndTurn, tr.Status)
	return nil
}

type RemoteToolHandler struct {
	cfg                   *RemoteToolHandlerConfig
	workerEndpoint        *url.URL
	specificationEndpoint *url.URL
	inputSchema           json.RawMessage
	mux                   *http.ServeMux
}

type RemoteToolHandlerConfig struct {
	Endpoint                *url.URL
	WorkerPath              string
	SpecificationPath       string
	ErrorHandler            func(w http.ResponseWriter, r *http.Request, err error, code int)
	MethodNotAllowedHandler func(w http.ResponseWriter, r *http.Request)
	NotFoundHandler         func(w http.ResponseWriter, r *http.Request)
	Tool                    Tool
	Logger                  *slog.Logger
}

const (
	HeaderToolUseID = "Estellm-Tool-Use-ID"
	HeaderToolName  = "Estellm-Tool-Name"
)

var _ http.Handler = (*RemoteToolHandler)(nil)

func NewRemoteToolHandler(cfg RemoteToolHandlerConfig) (*RemoteToolHandler, error) {
	h := &RemoteToolHandler{
		cfg: &cfg,
		mux: http.NewServeMux(),
	}
	if cfg.Tool == nil {
		return nil, errors.New("tool is required")
	}
	if cfg.Endpoint == nil {
		cfg.Endpoint = &url.URL{}
	}
	h.workerEndpoint = cfg.Endpoint.JoinPath(cfg.WorkerPath)
	if cfg.SpecificationPath == "" {
		cfg.SpecificationPath = DefaultSpecificationPath
	}
	h.specificationEndpoint = cfg.Endpoint.JoinPath(cfg.SpecificationPath)
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error, code int) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			var body bytes.Buffer
			if err := json.NewEncoder(&body).Encode(map[string]any{
				"error":   http.StatusText(code),
				"message": err.Error(),
				"status":  code,
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(code)
			w.Write(body.Bytes())
		}
	}

	if cfg.NotFoundHandler == nil {
		cfg.NotFoundHandler = func(w http.ResponseWriter, r *http.Request) {
			cfg.ErrorHandler(w, r, fmt.Errorf("the requested resource %q was not found", r.URL.Path), http.StatusNotFound)
		}
	}
	if cfg.MethodNotAllowedHandler == nil {
		cfg.MethodNotAllowedHandler = func(w http.ResponseWriter, r *http.Request) {
			cfg.ErrorHandler(w, r, fmt.Errorf("the requested resource %q does not support the method %q", r.URL.Path, r.Method), http.StatusMethodNotAllowed)
		}
	}
	var err error
	h.inputSchema, err = json.Marshal(cfg.Tool.InputSchema())
	if err != nil {
		return nil, err
	}
	if h.cfg.Logger == nil {
		h.cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	}
	h.mux.HandleFunc("/"+strings.TrimPrefix(h.workerEndpoint.Path, "/"),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				h.cfg.MethodNotAllowedHandler(w, r)
				return
			}
			h.serveHTTPWorker(w, r)
		},
	)
	h.mux.HandleFunc("/"+strings.TrimPrefix(h.specificationEndpoint.Path, "/"),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				h.cfg.MethodNotAllowedHandler(w, r)
				return
			}
			h.serveHTTPSpecification(w, r)
		},
	)
	return h, nil
}

func (h *RemoteToolHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.cfg.Logger.InfoContext(r.Context(), "request", "method", r.Method, "url", r.URL)
	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	matched, pattern := h.mux.Handler(r)
	if pattern == "" {
		h.cfg.NotFoundHandler(w, r)
		return
	}
	matched.ServeHTTP(w, r)
}

// WorkerHandler returns an http.Handler that serves the worker endpoint.
func (h *RemoteToolHandler) WorkerHandler() http.Handler {
	return http.HandlerFunc(h.serveHTTPWorker)
}

func (h *RemoteToolHandler) serveHTTPWorker(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var v interface{}
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		h.cfg.Logger.WarnContext(ctx, "failed to decode request body", "error", err)
		h.cfg.ErrorHandler(w, r, fmt.Errorf("failed to decode request body: %w", err), http.StatusBadRequest)
		return
	}
	batch := NewBatchResponseWriter()
	var tr RemoteToolResult
	ctx = WithToolName(ctx, h.cfg.Tool.Name())
	if useID := r.Header.Get(HeaderToolUseID); useID != "" {
		ctx = WithToolUseID(ctx, useID)
	}
	if err := h.cfg.Tool.Call(ctx, v, batch); err != nil {
		tr = RemoteToolResult{
			Status: "error",
			Content: []ToolResultContent{
				{
					Type: "text",
					Text: err.Error(),
				},
			},
		}
	} else {
		result := batch.Response()
		if err := tr.UnmarshalParts(result.Message.Parts); err != nil {
			h.cfg.Logger.WarnContext(ctx, "failed to marshal tool result", "error", err)
			h.cfg.ErrorHandler(w, r, fmt.Errorf("failed to marshal tool result: %w", err), http.StatusInternalServerError)
			return
		}
		tr.Status = result.FinishMessage
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(tr); err != nil {
		h.cfg.Logger.WarnContext(ctx, "failed to encode response", "error", err)
		h.cfg.ErrorHandler(w, r, fmt.Errorf("failed to encode response: %w", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func (h *RemoteToolHandler) SpecificationHandler() http.Handler {
	return http.HandlerFunc(h.serveHTTPSpecification)
}

func (h *RemoteToolHandler) serveHTTPSpecification(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	workerEndpoint := h.workerEndpoint
	if !workerEndpoint.IsAbs() && !strings.HasPrefix(workerEndpoint.Path, "/") {
		workerEndpoint.Path = "/" + workerEndpoint.Path
	}
	spec := Specification{
		Name:           h.cfg.Tool.Name(),
		Description:    h.cfg.Tool.Description(),
		InputSchema:    h.inputSchema,
		WorkerEndpoint: workerEndpoint.String(),
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(spec); err != nil {
		h.cfg.Logger.WarnContext(ctx, "failed to encode specification", "error", err)
		h.cfg.ErrorHandler(w, req, fmt.Errorf("failed to encode specification: %w", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}
