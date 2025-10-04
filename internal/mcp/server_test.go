package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prospero/internal/mcp"
)

func TestServer_Initialize(t *testing.T) {
	t.Run("should initialize server with protocol version and capabilities", func(t *testing.T) {
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

		response := server.HTTPHandler()
		req := createTestRequest(t, request)
		w := executeRequest(t, response, req)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.Equal(t, "2.0", res.JSONRPC)
		assert.Equal(t, "1", res.ID)
		assert.Nil(t, res.Error)

		result := res.Result.(map[string]interface{})
		assert.Equal(t, "2025-03-26", result["protocolVersion"])
		assert.Contains(t, result, "capabilities")
		assert.Contains(t, result, "serverInfo")

		serverInfo := result["serverInfo"].(map[string]interface{})
		assert.Equal(t, "test-server", serverInfo["name"])
		assert.Equal(t, "1.0.0", serverInfo["version"])
	})

	t.Run("should return error for invalid initialize params", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "initialize",
			Params:  "invalid params",
		}

		response := server.HTTPHandler()
		req := createTestRequest(t, request)
		w := executeRequest(t, response, req)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.NotNil(t, res.Error)
		assert.Equal(t, -32602, res.Error.Code)
		assert.Equal(t, "Invalid params", res.Error.Message)
	})
}

func TestServer_Initialized(t *testing.T) {
	t.Run("should accept initialized notification and return no response", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}

		response := server.HTTPHandler()
		req := createTestRequest(t, request)
		w := executeRequest(t, response, req)

		// Should return 202 Accepted with no body
		assert.Equal(t, 202, w.Code)
		assert.Empty(t, w.Body.String())
	})
}

func TestServer_PromptsList(t *testing.T) {
	t.Run("should return error when server not initialized", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "prompts/list",
		}

		response := server.HTTPHandler()
		req := createTestRequest(t, request)
		w := executeRequest(t, response, req)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.NotNil(t, res.Error)
		assert.Equal(t, -32002, res.Error.Code)
		assert.Equal(t, "Server not initialized", res.Error.Message)
	})

	t.Run("should list prompts after initialization", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		// Register a test prompt
		testPrompt := mcp.Prompt{
			Name:        "test-prompt",
			Description: "A test prompt",
		}
		server.RegisterPrompt(testPrompt, func(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []mcp.PromptMessage{
					{
						Role: "user",
						Content: mcp.MessageContent{
							Type: "text",
							Text: "test message",
						},
					},
				},
			}, nil
		})

		// Initialize server
		initReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "init",
			Method:  "initialize",
		}
		initHTTPReq := createTestRequest(t, initReq)
		initW := executeRequest(t, server.HTTPHandler(), initHTTPReq)
		require.Equal(t, 200, initW.Code)

		// Send initialized notification
		notifReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}
		notifHTTPReq := createTestRequest(t, notifReq)
		notifW := executeRequest(t, server.HTTPHandler(), notifHTTPReq)
		require.Equal(t, 202, notifW.Code)

		// Now list prompts
		listReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "prompts/list",
		}

		listHTTPReq := createTestRequest(t, listReq)
		w := executeRequest(t, server.HTTPHandler(), listHTTPReq)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.Nil(t, res.Error)
		assert.NotNil(t, res.Result)

		result := res.Result.(map[string]interface{})
		prompts := result["prompts"].([]interface{})
		assert.Len(t, prompts, 1)

		firstPrompt := prompts[0].(map[string]interface{})
		assert.Equal(t, "test-prompt", firstPrompt["name"])
		assert.Equal(t, "A test prompt", firstPrompt["description"])
	})
}

func TestServer_PromptsGet(t *testing.T) {
	t.Run("should return error when server not initialized", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "prompts/get",
			Params: map[string]interface{}{
				"name": "test-prompt",
			},
		}

		response := server.HTTPHandler()
		req := createTestRequest(t, request)
		w := executeRequest(t, response, req)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.NotNil(t, res.Error)
		assert.Equal(t, -32002, res.Error.Code)
		assert.Equal(t, "Server not initialized", res.Error.Message)
	})

	t.Run("should execute prompt after initialization", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		// Register a test prompt
		testPrompt := mcp.Prompt{
			Name:        "test-prompt",
			Description: "A test prompt",
		}
		server.RegisterPrompt(testPrompt, func(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: "Test prompt result",
				Messages: []mcp.PromptMessage{
					{
						Role: "user",
						Content: mcp.MessageContent{
							Type: "text",
							Text: "Hello, " + args["name"],
						},
					},
				},
			}, nil
		})

		// Initialize server
		initReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "init",
			Method:  "initialize",
		}
		initHTTPReq := createTestRequest(t, initReq)
		initW := executeRequest(t, server.HTTPHandler(), initHTTPReq)
		require.Equal(t, 200, initW.Code)

		// Send initialized notification
		notifReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}
		notifHTTPReq := createTestRequest(t, notifReq)
		notifW := executeRequest(t, server.HTTPHandler(), notifHTTPReq)
		require.Equal(t, 202, notifW.Code)

		// Get prompt
		getReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "prompts/get",
			Params: map[string]interface{}{
				"name": "test-prompt",
				"arguments": map[string]string{
					"name": "World",
				},
			},
		}

		getHTTPReq := createTestRequest(t, getReq)
		w := executeRequest(t, server.HTTPHandler(), getHTTPReq)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.Nil(t, res.Error)
		assert.NotNil(t, res.Result)

		result := res.Result.(map[string]interface{})
		assert.Equal(t, "Test prompt result", result["description"])

		messages := result["messages"].([]interface{})
		assert.Len(t, messages, 1)

		firstMessage := messages[0].(map[string]interface{})
		assert.Equal(t, "user", firstMessage["role"])

		content := firstMessage["content"].(map[string]interface{})
		assert.Equal(t, "text", content["type"])
		assert.Equal(t, "Hello, World", content["text"])
	})

	t.Run("should return error for unknown prompt", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		// Initialize server
		initReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "init",
			Method:  "initialize",
		}
		initHTTPReq := createTestRequest(t, initReq)
		initW := executeRequest(t, server.HTTPHandler(), initHTTPReq)
		require.Equal(t, 200, initW.Code)

		// Send initialized notification
		notifReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialized",
		}
		notifHTTPReq := createTestRequest(t, notifReq)
		notifW := executeRequest(t, server.HTTPHandler(), notifHTTPReq)
		require.Equal(t, 202, notifW.Code)

		// Get unknown prompt
		getReq := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "prompts/get",
			Params: map[string]interface{}{
				"name": "unknown-prompt",
			},
		}

		getHTTPReq := createTestRequest(t, getReq)
		w := executeRequest(t, server.HTTPHandler(), getHTTPReq)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.NotNil(t, res.Error)
		assert.Equal(t, -32603, res.Error.Code)
	})
}

func TestServer_MethodNotFound(t *testing.T) {
	t.Run("should return method not found error for unknown method", func(t *testing.T) {
		server := mcp.NewServer("test-server", "1.0.0")

		request := mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "unknown/method",
		}

		response := server.HTTPHandler()
		req := createTestRequest(t, request)
		w := executeRequest(t, response, req)

		var res mcp.JSONRPCResponse
		err := json.NewDecoder(w.Body).Decode(&res)
		require.NoError(t, err)

		assert.NotNil(t, res.Error)
		assert.Equal(t, -32601, res.Error.Code)
		assert.Equal(t, "Method not found", res.Error.Message)
	})
}

// Helper functions

func createTestRequest(t *testing.T, request mcp.JSONRPCRequest) *http.Request {
	t.Helper()

	body, err := json.Marshal(request)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	return req
}

func executeRequest(t *testing.T, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	return w
}
