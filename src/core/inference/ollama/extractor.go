package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hsme/core/src/core/worker"
)

type Extractor struct {
	client *Client
	model  string
}

func NewExtractor(client *Client, model string) *Extractor {
	if model == "" {
		model = "phi3.5"
	}
	return &Extractor{
		client: client,
		model:  model,
	}
}

func (e *Extractor) ExtractEntities(ctx context.Context, text string) (worker.KnowledgeGraph, error) {
	systemPrompt := `You are a technical graph extractor. Return ONLY valid JSON with the structure:
{"nodes": [{"type": "TECH|ERROR|FILE|CMD", "name": "string"}], "edges": [{"source": "string", "target": "string", "relation": "DEPENDS_ON|RESOLVES|CAUSES"}]}
Do not return explanations or markdown formatting like ` + "```" + `json. Just raw JSON.`

	reqBody := map[string]interface{}{
		"model":  e.model,
		"prompt": systemPrompt + "\n\nText to analyze:\n" + text,
		"format": "json",
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.0,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return worker.KnowledgeGraph{}, fmt.Errorf("failed to marshal extract request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.client.BaseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return worker.KnowledgeGraph{}, fmt.Errorf("failed to create extract request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.HTTPClient.Do(req)
	if err != nil {
		return worker.KnowledgeGraph{}, fmt.Errorf("failed to execute extract request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return worker.KnowledgeGraph{}, fmt.Errorf("ollama API returned status %d for extraction", resp.StatusCode)
	}

	var resBody struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resBody); err != nil {
		return worker.KnowledgeGraph{}, fmt.Errorf("failed to decode extract response: %w", err)
	}

	var kg worker.KnowledgeGraph
	if err := json.Unmarshal([]byte(resBody.Response), &kg); err != nil {
		return worker.KnowledgeGraph{}, fmt.Errorf("failed to parse extracted JSON: %w (response: %s)", err, resBody.Response)
	}

	return kg, nil
}
