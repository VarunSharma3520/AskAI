package vector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// OllamaEmbedder implements the Embedder interface using Ollama's API
type OllamaEmbedder struct {
	baseURL string
	model  string
}

// NewOllamaEmbedder creates a new Ollama embedder
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "mxbai-embed-large"
	}
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:  model,
	}
}

// EmbedRequest represents the request body for Ollama's embedding API
type EmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// EmbedResponse represents the response from Ollama's embedding API
type EmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed converts text to a vector using Ollama's embedding model
func (e *OllamaEmbedder) Embed(text string) ([]float32, error) {
	// Prepare the request payload
	reqBody := EmbedRequest{
		Model:  e.model,
		Prompt: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest(
		"POST",
		e.baseURL+"/api/embeddings",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API returned non-200 status: %s", resp.Status)
	}

	// Parse the response
	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	if len(embedResp.Embedding) == 0 {
		return nil, fmt.Errorf("no embedding data returned from Ollama")
	}

	return embedResp.Embedding, nil
}

// DummyEmbedder is a simple embedder for testing that returns deterministic vectors
// It should not be used in production
type DummyEmbedder struct{}

// NewDummyEmbedder creates a new dummy embedder for testing
func NewDummyEmbedder() *DummyEmbedder {
	return &DummyEmbedder{}
}

// Embed returns a simple deterministic vector based on the input text length
// This is only for testing and should not be used in production
func (d *DummyEmbedder) Embed(text string) ([]float32, error) {
	// Create a simple deterministic vector based on the text length
	// This is just for testing purposes
	vector := make([]float32, 1024) // mxbai-embed-large uses 1024-dimensional vectors
	for i := range vector {
		if i < len(text) {
			vector[i] = float32(text[i%len(text)]) / 256.0
		} else {
			vector[i] = float32(i) / 1024.0
		}
	}
	return vector, nil
}
