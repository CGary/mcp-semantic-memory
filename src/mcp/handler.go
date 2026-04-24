package mcp

import (
	"bufio"
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
	fmt.Fprintf(os.Stderr, "HSME MCP server starting (v1.0.1)...\n")
	
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintf(os.Stderr, "Standard input closed (EOF)\n")
				return
			}
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			continue
		}

		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON-RPC request: %v | Raw: %s\n", err, line)
			continue
		}

		fmt.Fprintf(os.Stderr, "Received request: %s (ID: %s)\n", req.Method, string(req.ID))
		// Despacho concurrente: un tool call lento (ej. search_fuzzy llamando a Ollama)
		// ya no bloquea el resto del tráfico JSON-RPC. sendResponse está protegido por
		// writeMutex, y el mapa de tools es inmutable después de RegisterTool
		// (todas las registraciones ocurren en main() antes de Serve), así que las
		// goroutines solo lo leen sin carrera.
		go s.handleRequest(req)
	}
}

func (s *Server) sendResponse(resp JSONRPCResponse) {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()
	
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
		return
	}
	
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

func (s *Server) handleRequest(req JSONRPCRequest) {
	isNotification := req.ID == nil || string(req.ID) == "null"

	var resp JSONRPCResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": false,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "hsme",
				"version": "1.0.1",
			},
		}
	case "notifications/initialized":
		fmt.Fprintf(os.Stderr, "Handshake complete\n")
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
	case "ping":
		resp.Result = map[string]interface{}{}
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
