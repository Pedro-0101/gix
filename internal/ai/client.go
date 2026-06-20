package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

func New(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{},
		apiKey:     apiKey,
		baseURL:    endpoint,
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Usage contém a contagem de tokens retornada pela API.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

// Stream faz a chamada com stream:true e invoca onDelta a cada pedaço de texto.
// Retorna o Usage (pode ser nil se a API não enviar) e um possível erro.
// ctx cancelado aborta o request. Status != 2xx vira erro com o corpo.
func (c *Client) Stream(ctx context.Context, model string, messages []Message, onDelta func(string)) (*Usage, error) {
	body, err := json.Marshal(chatRequest{Model: model, Messages: messages, Stream: true})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "gix")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var usage *Usage
	reader := bufio.NewReader(resp.Body)
	for {
		line, readErr := reader.ReadString('\n')
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if data == "[DONE]" {
				return usage, nil
			}
			var chunk streamChunk
			if json.Unmarshal([]byte(data), &chunk) == nil {
				if chunk.Usage != nil {
					usage = chunk.Usage
				}
				for _, ch := range chunk.Choices {
					if ch.Delta.Content != "" {
						onDelta(ch.Delta.Content)
					}
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return usage, nil
			}
			return usage, readErr
		}
	}
}
