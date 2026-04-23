package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type Server struct {
	tools      map[string]ToolDefinition
	writeMutex sync.Mutex
}

type ToolDefinition struct {
	Tool    Tool
	Handler func(json.RawMessage) (interface{}, error)
}

func NewServer() *Server {
	return &Server{
		tools: make(map[string]ToolDefinition),
	}
}

func (s *Server) RegisterTool(name string, description string, schema interface{}, handler func(json.RawMessage) (interface{}, error)) {
	s.tools[name] = ToolDefinition{
		Tool: Tool{
			Name:        name,
			Description: description,
			InputSchema: schema,
		},
		Handler: handler,
	}
}

func (s *Server) Serve() {
	fmt.Fprintf(os.Stderr, "HSME MCP server starting...\n")
	decoder := json.NewDecoder(os.Stdin)
	for {
		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return
			}
			fmt.Fprintf(os.Stderr, "Error decoding request: %v\n", err)
			continue
		}

		s.handleRequest(req)
	}
}

func (s *Server) sendResponse(resp JSONRPCResponse) {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()
	json.NewEncoder(os.Stdout).Encode(resp)
}

func (s *Server) handleRequest(req JSONRPCRequest) {
	// Notifications (id is null) do not expect a response
	isNotification := req.ID == nil || string(req.ID) == "null"

	var resp JSONRPCResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"serverInfo": map[string]interface{}{
				"name":    "hsme",
				"version": "1.0.0",
			},
		}
	case "notifications/initialized":
		fmt.Fprintf(os.Stderr, "Client initialized\n")
		return
	case "tools/list", "list_tools":
		var tools []Tool
		for _, def := range s.tools {
			tools = append(tools, def.Tool)
		}
		resp.Result = map[string]interface{}{
			"tools": tools,
		}
	case "tools/call", "call_tool":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &JSONRPCError{Code: -32602, Message: "Invalid params"}
		} else {
			if def, ok := s.tools[params.Name]; ok {
				result, err := def.Handler(params.Arguments)
				if err != nil {
					resp.Error = &JSONRPCError{Code: -32000, Message: err.Error()}
				} else {
					resp.Result = map[string]interface{}{
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": formatResult(result),
							},
						},
					}
				}
			} else {
				resp.Error = &JSONRPCError{Code: -32601, Message: "Tool not found"}
			}
		}
	default:
		if isNotification {
			return
		}
		resp.Error = &JSONRPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}

	if !isNotification {
		s.sendResponse(resp)
	}
}

func formatResult(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
