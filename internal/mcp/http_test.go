package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prospero/internal/mcp"
)

func TestServer_HTTPHandler(t *testing.T) {
	t.Run("should open SSE stream on GET request", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		// Create a context that we can cancel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()

		// Run the handler in a goroutine and cancel the context after checking headers
		done := make(chan struct{})
		go func() {
			handler.ServeHTTP(w, req)
			close(done)
		}()

		// Give it a moment to set headers
		assert.Eventually(t, func() bool {
			return w.Header().Get("Content-Type") == "text/event-stream"
		}, 100*time.Millisecond, 10*time.Millisecond)

		assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

		// Cancel the context to stop the handler
		cancel()

		// Wait for handler to finish
		select {
		case <-done:
			// Handler finished successfully
		case <-time.After(1 * time.Second):
			t.Fatal("handler did not finish after context cancellation")
		}
	})

	t.Run("should handle single JSON-RPC request on POST", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "initialize",
			Params: map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]string{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		body, err := json.Marshal(request)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response mcp.JSONRPCResponse
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "2.0", response.JSONRPC)
		assert.Equal(t, "1", response.ID)
		assert.Nil(t, response.Error)
		assert.NotNil(t, response.Result)
	})

	t.Run("should handle batch JSON-RPC requests on POST", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		// First initialize the server
		initReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "init",
			Method:  "initialize",
		}
		initBody, _ := json.Marshal(initReq)
		initHTTPReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initBody))
		initW := httptest.NewRecorder()
		server.HTTPHandler().ServeHTTP(initW, initHTTPReq)

		// Send initialized notification
		notifReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}
		notifBody, _ := json.Marshal(notifReq)
		notifHTTPReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(notifBody))
		notifW := httptest.NewRecorder()
		server.HTTPHandler().ServeHTTP(notifW, notifHTTPReq)

		// Now send batch request
		batch := []mcp.JSONRPCRequest{
			{
				JSONRPC: "2.0",
				ID:      "1",
				Method:  "prompts/list",
			},
			{
				JSONRPC: "2.0",
				ID:      "2",
				Method:  "prompts/list",
			},
		}

		body, err := json.Marshal(batch)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var responses []mcp.JSONRPCResponse
		err = json.NewDecoder(w.Body).Decode(&responses)
		require.NoError(t, err)

		assert.Len(t, responses, 2)
		assert.Equal(t, "1", responses[0].ID)
		assert.Equal(t, "2", responses[1].ID)
	})

	t.Run("should return 202 Accepted for notification without ID", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}

		body, err := json.Marshal(request)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("should return 202 Accepted for batch with only notifications", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		batch := []mcp.JSONRPCRequest{
			{
				JSONRPC: "2.0",
				Method:  "initialized",
			},
			{
				JSONRPC: "2.0",
				Method:  "initialized",
			},
		}

		body, err := json.Marshal(batch)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("should return parse error for invalid JSON", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.NotNil(t, response.Error)
		assert.Equal(t, -32700, response.Error.Code)
		assert.Equal(t, "Parse error", response.Error.Message)
	})

	t.Run("should return method not allowed for unsupported HTTP methods", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		tests := []struct {
			method string
		}{
			{method: http.MethodPut},
			{method: http.MethodDelete},
			{method: http.MethodPatch},
		}

		for _, test := range tests {
			t.Run(test.method, func(t *testing.T) {
				req := httptest.NewRequest(test.method, "/mcp", nil)
				w := httptest.NewRecorder()

				handler := server.HTTPHandler()
				handler.ServeHTTP(w, req)

				assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
				assert.Contains(t, w.Body.String(), "Method not allowed")
			})
		}
	})

	t.Run("should handle mixed batch with requests and notifications", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		// Initialize server first
		initReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "init",
			Method:  "initialize",
		}
		initBody, _ := json.Marshal(initReq)
		initHTTPReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initBody))
		initW := httptest.NewRecorder()
		server.HTTPHandler().ServeHTTP(initW, initHTTPReq)

		// Send initialized notification
		notifReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}
		notifBody, _ := json.Marshal(notifReq)
		notifHTTPReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(notifBody))
		notifW := httptest.NewRecorder()
		server.HTTPHandler().ServeHTTP(notifW, notifHTTPReq)

		// Send mixed batch
		batch := []mcp.JSONRPCRequest{
			{
				JSONRPC: "2.0",
				ID:      "1",
				Method:  "prompts/list",
			},
			{
				JSONRPC: "2.0",
				Method:  "initialized", // notification
			},
			{
				JSONRPC: "2.0",
				ID:      "2",
				Method:  "prompts/list",
			},
		}

		body, err := json.Marshal(batch)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler := server.HTTPHandler()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var responses []mcp.JSONRPCResponse
		err = json.NewDecoder(w.Body).Decode(&responses)
		require.NoError(t, err)

		// Should only have 2 responses (notifications don't get responses)
		assert.Len(t, responses, 2)
		assert.Equal(t, "1", responses[0].ID)
		assert.Equal(t, "2", responses[1].ID)
	})
}
