package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

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
	h, err = NewHandler(HandlerConfig{
		Endpoint:   u,
		WorkerPath: "/worker/execute",
		Tool:       internalTool,
	})
	require.NoError(t, err)

	ctx := context.Background()
	tool, err := NewTool(ctx, ToolConfig{
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

func TestSpecificationCache(t *testing.T) {
	now := time.Now()
	restore := flextime.Fix(now.AddDate(0, 0, -1))
	defer restore()
	cache := NewSpecificationCache(1 * time.Hour)

	spec := Specification{Name: "test"}

	// Set the specification in the cache
	cache.Set("https://example.com/", spec)

	// Get the specification from the cache
	retrievedSpec, ok := cache.Get("https://example.com/")
	require.True(t, ok)
	require.Equal(t, spec, retrievedSpec)

	flextime.Fix(now)

	// Try to get the specification from the cache after expiration
	_, ok = cache.Get("https://example.com/")
	require.False(t, ok)
	// Set the specification in the cache again
	cache.Set("http://www.example.com/", spec)

	// Delete the specification from the cache
	cache.Delete("http://www.example.com/")

	// Try to get the specification from the cache after deletion
	_, ok = cache.Get("http://www.example.com/")
	require.False(t, ok)
}
