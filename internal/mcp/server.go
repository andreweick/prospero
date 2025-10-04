package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Server struct {
	name            string
	version         string
	promptRegistry  *PromptRegistry
	initialized     bool
	protocolVersion string
}

func NewServer(name, version string) *Server {
	return &Server{
		name:            name,
		version:         version,
		promptRegistry:  NewPromptRegistry(),
		protocolVersion: "2025-03-26",
	}
}

func (s *Server) RegisterPrompt(prompt Prompt, handler PromptHandler) {
	s.promptRegistry.Register(prompt, handler)
}

func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil && err != io.EOF {
					return fmt.Errorf("error reading from stdin: %w", err)
				}
				return nil
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var request JSONRPCRequest
			if err := json.Unmarshal(line, &request); err != nil {
				s.sendError(encoder, nil, -32700, "Parse error", nil)
				continue
			}

			response := s.handleRequest(ctx, request)
			if response != nil {
				if err := encoder.Encode(response); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing response: %v\n", err)
				}
			}
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, request JSONRPCRequest) *JSONRPCResponse {
	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "initialized":
		s.initialized = true
		return nil // Notification, no response
	case "prompts/list":
		if !s.initialized {
			return s.errorResponse(request.ID, -32002, "Server not initialized", nil)
		}
		return s.handlePromptsList(request)
	case "prompts/get":
		if !s.initialized {
			return s.errorResponse(request.ID, -32002, "Server not initialized", nil)
		}
		return s.handlePromptsGet(ctx, request)
	default:
		return s.errorResponse(request.ID, -32601, "Method not found", nil)
	}
}

func (s *Server) handleInitialize(request JSONRPCRequest) *JSONRPCResponse {
	var params InitializeParams
	if request.Params != nil {
		paramBytes, _ := json.Marshal(request.Params)
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			return s.errorResponse(request.ID, -32602, "Invalid params", nil)
		}
	}

	result := InitializeResult{
		ProtocolVersion: s.protocolVersion,
		Capabilities: ServerCapabilities{
			Prompts: &PromptsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    s.name,
			Version: s.version,
		},
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}
}

func (s *Server) handlePromptsList(request JSONRPCRequest) *JSONRPCResponse {
	prompts := s.promptRegistry.List()
	result := ListPromptsResult{
		Prompts: prompts,
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}
}

func (s *Server) handlePromptsGet(ctx context.Context, request JSONRPCRequest) *JSONRPCResponse {
	var params GetPromptParams
	if request.Params != nil {
		paramBytes, _ := json.Marshal(request.Params)
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			return s.errorResponse(request.ID, -32602, "Invalid params", nil)
		}
	}

	result, err := s.promptRegistry.Execute(ctx, params.Name, params.Arguments)
	if err != nil {
		return s.errorResponse(request.ID, -32603, err.Error(), nil)
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}
}

func (s *Server) errorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func (s *Server) sendError(encoder *json.Encoder, id interface{}, code int, message string, data interface{}) {
	response := s.errorResponse(id, code, message, data)
	encoder.Encode(response)
}
