package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Server struct {
	tools map[string]func(json.RawMessage) (interface{}, error)
}

func NewServer() *Server {
	return &Server{
		tools: make(map[string]func(json.RawMessage) (interface{}, error)),
	}
}

func (s *Server) RegisterTool(name string, handler func(json.RawMessage) (interface{}, error)) {
	s.tools[name] = handler
}

func (s *Server) Serve() {
	decoder := json.NewDecoder(os.Stdin)
	for {
		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error decoding request: %v\n", err)
			continue
		}

		go s.handleRequest(req)
	}
}

func (s *Server) handleRequest(req JSONRPCRequest) {
	var resp JSONRPCResponse
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"serverInfo": map[string]interface{}{
				"name":    "hsme-server",
				"version": "1.0.0",
			},
		}
	case "tools/list":
		var tools []Tool
		// Define tool descriptions here or pass them in
		tools = append(tools, Tool{
			Name:        "store_context",
			Description: "Store technical context in memory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content":     map[string]string{"type": "string"},
					"source_type": map[string]string{"type": "string"},
				},
				"required": []string{"content", "source_type"},
			},
		})
		tools = append(tools, Tool{
			Name:        "search_fuzzy",
			Description: "Search memory using fuzzy matching",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]string{"type": "string"},
					"limit": map[string]string{"type": "integer"},
				},
				"required": []string{"query"},
			},
		})
		tools = append(tools, Tool{
			Name:        "search_exact",
			Description: "Search memory using exact keyword matching",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]string{"type": "string"},
					"limit":   map[string]string{"type": "integer"},
				},
				"required": []string{"keyword"},
			},
		})
		tools = append(tools, Tool{
			Name:        "trace_dependencies",
			Description: "Trace entity dependencies in the knowledge graph",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_name": map[string]string{"type": "string"},
					"direction":   map[string]string{"type": "string"},
				},
				"required": []string{"entity_name"},
			},
		})
		resp.Result = map[string]interface{}{
			"tools": tools,
		}
	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = map[string]interface{}{
				"code":    -32602,
				"message": "Invalid params",
			}
		} else {
			if handler, ok := s.tools[params.Name]; ok {
				result, err := handler(params.Arguments)
				if err != nil {
					resp.Error = map[string]interface{}{
						"code":    -32000,
						"message": err.Error(),
					}
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
				resp.Error = map[string]interface{}{
					"code":    -32601,
					"message": "Tool not found",
				}
			}
		}
	default:
		resp.Error = map[string]interface{}{
			"code":    -32601,
			"message": "Method not found",
		}
	}

	json.NewEncoder(os.Stdout).Encode(resp)
}

func formatResult(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
