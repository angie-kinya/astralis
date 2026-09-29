package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// A URL is just a string to the compiler, so a typo in it compiles fine and
// only fails against the real API — which is exactly how "/v1/message"
// reached production. This test pins the request contract instead: the path,
// the headers, and the body shape, against a local fake of the API.
func TestCompleteSendsTheRequestAnthropicExpects(t *testing.T) {
	var got struct {
		path, method, key, version, ctype string
		body                              reqBody
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.method = r.URL.Path, r.Method
		got.key, got.version = r.Header.Get("x-api-key"), r.Header.Get("anthropic-version")
		got.ctype = r.Header.Get("Content-Type")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got.body))
		// Two text blocks plus a non-text block: Complete must join the text
		// and ignore anything else.
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"Hello, "},
			{"type":"thinking","text":"ignored"},{"type":"text","text":"luna."}]}`))
	}))
	defer srv.Close()

	c := NewAnthropic("test-key", "claude-test")
	c.baseURL = srv.URL

	out, err := c.Complete(context.Background(), "be kind",
		[]Message{{Role: "user", Content: "hi"}}, 64)
	require.NoError(t, err)
	require.Equal(t, "Hello, luna.", out)

	require.Equal(t, "/v1/messages", got.path)
	require.Equal(t, http.MethodPost, got.method)
	require.Equal(t, "test-key", got.key)
	require.Equal(t, "2023-06-01", got.version)
	require.Equal(t, "application/json", got.ctype)
	require.Equal(t, "claude-test", got.body.Model)
	require.Equal(t, 64, got.body.MaxTokens)
	require.Equal(t, "be kind", got.body.System)
	require.Equal(t, []Message{{Role: "user", Content: "hi"}}, got.body.Messages)
}

func TestCompleteSurfacesAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"Not found"}}`))
	}))
	defer srv.Close()

	c := NewAnthropic("k", "m")
	c.baseURL = srv.URL
	_, err := c.Complete(context.Background(), "", []Message{{Role: "user", Content: "x"}}, 8)
	require.ErrorContains(t, err, "anthropic status 404")
}

func TestDefaultBaseURLIsTheRealAPI(t *testing.T) {
	require.Equal(t, "https://api.anthropic.com", NewAnthropic("k", "m").baseURL)
}
