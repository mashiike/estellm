package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/mashiike/estellm"
)

type Handler struct {
	cfg                   *HandlerConfig
	workerEndpoint        *url.URL
	specificationEndpoint *url.URL
	inputSchema           json.RawMessage
	mux                   *http.ServeMux
}

type HandlerConfig struct {
	Endpoint                *url.URL
	WorkerPath              string
	SpecificationPath       string
	ErrorHandler            func(w http.ResponseWriter, r *http.Request, err error, code int)
	MethodNotAllowedHandler func(w http.ResponseWriter, r *http.Request)
	NotFoundHandler         func(w http.ResponseWriter, r *http.Request)
	Tool                    estellm.Tool
	Logger                  *slog.Logger
}

const (
	HeaderToolUseID = "Estellm-Tool-Use-ID"
	HeaderToolName  = "Estellm-Tool-Name"
)

var _ http.Handler = (*Handler)(nil)

func NewHandler(cfg HandlerConfig) (*Handler, error) {
	h := &Handler{
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

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) WorkerHandler() http.Handler {
	return http.HandlerFunc(h.serveHTTPWorker)
}

func (h *Handler) serveHTTPWorker(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var v interface{}
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		h.cfg.Logger.WarnContext(ctx, "failed to decode request body", "error", err)
		h.cfg.ErrorHandler(w, r, fmt.Errorf("failed to decode request body: %w", err), http.StatusBadRequest)
		return
	}
	batch := estellm.NewBatchResponseWriter()
	var tr ToolResult
	ctx = estellm.WithToolName(ctx, h.cfg.Tool.Name())
	if useID := r.Header.Get(HeaderToolUseID); useID != "" {
		ctx = estellm.WithToolUseID(ctx, useID)
	}
	if err := h.cfg.Tool.Call(ctx, v, batch); err != nil {
		tr = ToolResult{
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

// SpecificationHandler returns an http.Handler that serves the tool specification.
func (h *Handler) SpecificationHandler() http.Handler {
	return http.HandlerFunc(h.serveHTTPSpecification)
}

func (h *Handler) serveHTTPSpecification(w http.ResponseWriter, req *http.Request) {
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
