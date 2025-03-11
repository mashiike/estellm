package estellm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

type weatherInput struct {
	City string `json:"city" jsonschema:"description=都市名 (例: 横浜,東京),default=東京, required=true"`
	When string `json:"when" jsonschema:"description=日時 RFC3339 (例: 2022-01-01T00:00:00Z), required=false"`
}

func TestRemoteToolHandler(t *testing.T) {
	tool, err := estellm.NewTool("weather", "return weather", func(ctx context.Context, input weatherInput, w estellm.ResponseWriter) error {
		require.EqualValues(t, "東京", input.City)
		require.EqualValues(t, "2022-01-01T00:00:00Z", input.When)
		w.WritePart(estellm.TextPart("sunny"))
		w.Finish(estellm.FinishReasonEndTurn, "success")
		return nil
	})
	require.NoError(t, err)
	h, err := estellm.NewRemoteToolHandler(estellm.RemoteToolHandlerConfig{
		WorkerPath: "/worker/execute",
		Tool:       tool,
	})
	require.NoError(t, err)
	specReq := httptest.NewRequest(http.MethodGet, "http://localhost:8080/.well-known/estellm-tool-specification", nil)
	specResp := httptest.NewRecorder()
	h.ServeHTTP(specResp, specReq)
	require.Equal(t, http.StatusOK, specResp.Code)
	t.Log(specResp.Body.String())
	expected := `{
  "name": "weather",
  "description": "return weather",
  "input_schema": {
    "properties": {
      "city": {
        "default": "東京",
        "type": "string",
        "description": "都市名 (例: 横浜"
      },
      "when": {
        "type": "string",
        "description": "日時 RFC3339 (例: 2022-01-01T00:00:00Z)"
      }
    },
    "additionalProperties": false,
    "type": "object",
    "required": [
      "city",
      "when"
    ]
  },
  "worker_endpoint": "/worker/execute"
}`

	require.JSONEq(t, expected, specResp.Body.String())
	input := `{"city":"東京","when":"2022-01-01T00:00:00Z"}`
	workerReq := httptest.NewRequest(http.MethodPost, "http://localhost:8080/worker/execute", strings.NewReader(input))
	workerResp := httptest.NewRecorder()
	h.ServeHTTP(workerResp, workerReq)
	require.Equal(t, http.StatusOK, workerResp.Code)
	t.Log(workerResp.Body.String())
	expected = `{"content":[{"type":"text","text":"sunny"}],"status":"success"}`
	require.JSONEq(t, expected, workerResp.Body.String())
}

func TestHandler__NotFound(t *testing.T) {
	tool, err := estellm.NewTool("weather", "return weather", func(ctx context.Context, input weatherInput, w estellm.ResponseWriter) error {
		return nil
	})
	require.NoError(t, err)
	h, err := estellm.NewRemoteToolHandler(estellm.RemoteToolHandlerConfig{
		WorkerPath: "/worker/execute",
		Tool:       tool,
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/notfound", nil)
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
	require.JSONEq(t, `{"error":"Not Found", "message":"the requested resource \"/notfound\" was not found", "status":404}`, resp.Body.String())
}

func TestRemoteTool(t *testing.T) {
	var h http.Handler
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("request: %s %s", r.Method, r.URL)
		h.ServeHTTP(w, r)
	}))
	defer server.Close()
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	internalTool, err := estellm.NewTool("weather", "return weather", func(ctx context.Context, input weatherInput, w estellm.ResponseWriter) error {
		require.EqualValues(t, "東京", input.City)
		require.EqualValues(t, "2022-01-01T00:00:00Z", input.When)
		toolName, ok := estellm.ToolNameFromContext(ctx)
		require.True(t, ok)
		toolUseID, ok := estellm.ToolUseIDFromContext(ctx)
		require.True(t, ok)
		require.EqualValues(t, "weather", toolName)
		require.NotEmpty(t, toolUseID)
		w.WritePart(estellm.TextPart("sunny"))
		return nil
	})
	require.NoError(t, err)
	h, err = estellm.NewRemoteToolHandler(estellm.RemoteToolHandlerConfig{
		Endpoint:   u,
		WorkerPath: "/worker/execute",
		Tool:       internalTool,
	})
	require.NoError(t, err)

	ctx := context.Background()
	tool, err := estellm.NewRemoteTool(ctx, estellm.RemoteToolConfig{
		Endpoint: server.URL,
	})
	require.NoError(t, err)
	require.Equal(t, "weather", tool.Name())
	require.Equal(t, "return weather", tool.Description())
	w := estellm.NewBatchResponseWriter()
	cctx := estellm.WithToolUseID(ctx, "test")
	err = tool.Call(cctx, weatherInput{City: "東京", When: "2022-01-01T00:00:00Z"}, w)
	require.NoError(t, err)
	excpect := estellm.Message{
		Role: estellm.RoleAssistant,
		Parts: []estellm.ContentPart{
			estellm.TextPart("sunny"),
		},
	}
	require.EqualValues(t, excpect, w.Response().Message)
}

func TestToolResult__Marshal(t *testing.T) {
	parts := []estellm.ContentPart{
		estellm.TextPart("Hello, World!"),
		estellm.TextPart(`{"key":"value"}`),
		estellm.BinaryPartWithName("text/csv", "example", []byte("a,b,c\n1,2,3")),
		estellm.BinaryPart("image/png", []byte("image data")),
	}

	var tr estellm.RemoteToolResult
	err := tr.UnmarshalParts(parts)
	require.NoError(t, err)
	tr.Status = "success"
	bs, err := json.MarshalIndent(tr, "", "  ")
	require.NoError(t, err)
	expected := `{
	"content": [
		{
			"type": "text",
			"text": "Hello, World!"
		},
		{
			"type": "json",
			"json": "{\"key\":\"value\"}"
		},
		{
			"type": "document",
			"format": "csv",
			"name": "example",
			"source": "YSxiLGMKMSwyLDM="
		},
		{
			"type": "image",
			"format": "png",
			"source": "aW1hZ2UgZGF0YQ=="
		}
	],
	"status": "success"
}
`
	t.Log(string(bs))
	require.JSONEq(t, expected, string(bs))
	var tr2 estellm.RemoteToolResult
	err = json.Unmarshal(bs, &tr2)
	require.NoError(t, err)
	require.EqualValues(t, tr, tr2)
	acutal, err := tr2.MarshalParts()
	require.NoError(t, err)
	require.EqualValues(t, parts, acutal)
}
