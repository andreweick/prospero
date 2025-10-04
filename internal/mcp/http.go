package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HTTPHandler creates an HTTP handler for the MCP server
// It implements the Streamable HTTP transport (MCP spec 2025-03-26)
func (s *Server) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleSSEStream(w, r)
		case http.MethodPost:
			s.handleJSONRPC(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleSSEStream handles GET requests and opens an SSE stream
func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Flush immediately to establish the connection
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Keep the connection open until context is cancelled
	<-r.Context().Done()
}

// handleJSONRPC handles POST requests with JSON-RPC messages
func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Try to parse as a single request first
	var request JSONRPCRequest
	if err := json.Unmarshal(body, &request); err != nil {
		// Try to parse as a batch
		var batch []JSONRPCRequest
		if err := json.Unmarshal(body, &batch); err != nil {
			s.sendJSONError(w, nil, -32700, "Parse error", nil)
			return
		}
		s.handleBatchRequest(w, r.Context(), batch)
		return
	}

	// Handle single request
	s.handleSingleRequest(w, r.Context(), request)
}

// handleSingleRequest processes a single JSON-RPC request
func (s *Server) handleSingleRequest(w http.ResponseWriter, ctx context.Context, request JSONRPCRequest) {
	response := s.handleRequest(ctx, request)

	// If this is a notification (no ID), return 202 Accepted with no body
	if request.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if response != nil {
		if err := json.NewEncoder(w).Encode(response); err != nil {
			fmt.Fprintf(w, "Error encoding response: %v", err)
		}
	}
}

// handleBatchRequest processes a batch of JSON-RPC requests
func (s *Server) handleBatchRequest(w http.ResponseWriter, ctx context.Context, batch []JSONRPCRequest) {
	responses := make([]*JSONRPCResponse, 0, len(batch))
	hasRequests := false

	for _, request := range batch {
		response := s.handleRequest(ctx, request)
		// Only include responses for requests (not notifications)
		if request.ID != nil {
			hasRequests = true
			if response != nil {
				responses = append(responses, response)
			}
		}
	}

	// If no requests (only notifications), return 202 Accepted
	if !hasRequests {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		fmt.Fprintf(w, "Error encoding response: %v", err)
	}
}

// sendJSONError sends a JSON-RPC error response
func (s *Server) sendJSONError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := s.errorResponse(id, code, message, data)
	json.NewEncoder(w).Encode(response)
}

// sendSSEMessage sends a JSON-RPC message over an SSE stream
func sendSSEMessage(w io.Writer, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// SSE format: "data: <json>\n\n"
	writer := bufio.NewWriter(w)
	if _, err := writer.WriteString("data: "); err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if _, err := writer.WriteString("\n\n"); err != nil {
		return err
	}
	return writer.Flush()
}
