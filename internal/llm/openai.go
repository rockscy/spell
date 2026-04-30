package llm

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

// OpenAI implements the OpenAI Chat Completions streaming protocol.
// Works with any OpenAI-compatible endpoint: OpenAI, DeepSeek, Moonshot/Kimi,
// Zhipu, Qwen/DashScope, Doubao, Groq, Together, OpenRouter, Ollama (/v1), etc.
type OpenAI struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxTokens  int // 0 = omit the field (let the server pick a default)
	HTTPClient *http.Client
	Label      string // human-readable provider name (e.g. "deepseek")
}

func (o *OpenAI) Name() string {
	if o.Label != "" {
		return o.Label
	}
	return "openai"
}

type openAIReq struct {
	Model     string          `json:"model"`
	Messages  []openAIMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (o *OpenAI) Stream(ctx context.Context, system, user string) (<-chan Chunk, error) {
	if o.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	if o.Model == "" {
		return nil, ErrNoModel
	}
	if o.BaseURL == "" {
		return nil, ErrNoURL
	}

	out := make(chan Chunk, 16)
	go func() {
		defer close(out)

		// All I/O happens here — including the HTTP round-trip — so the
		// caller can render a spinner while we wait for response headers
		// (reasoning models often sit on this for 1-3 seconds before
		// emitting the first byte).
		body, err := json.Marshal(openAIReq{
			Model: o.Model,
			Messages: []openAIMessage{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
			Stream:    true,
			MaxTokens: o.MaxTokens,
		})
		if err != nil {
			out <- Chunk{Err: err}
			return
		}

		url := strings.TrimRight(o.BaseURL, "/") + "/chat/completions"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			out <- Chunk{Err: err}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
		req.Header.Set("Accept", "text/event-stream")

		client := o.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		if err != nil {
			out <- Chunk{Err: err}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			buf, _ := io.ReadAll(resp.Body)
			out <- Chunk{Err: fmt.Errorf("%s: http %d: %s", o.Name(), resp.StatusCode, strings.TrimSpace(string(buf)))}
			return
		}

		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
				continue
			}
			var c openAIStreamChunk
			if err := json.Unmarshal(payload, &c); err != nil {
				// some compat servers emit non-JSON keepalives; ignore
				continue
			}
			for _, ch := range c.Choices {
				if ch.Delta.ReasoningContent != "" {
					select {
					case out <- Chunk{Delta: ch.Delta.ReasoningContent, Reasoning: true}:
					case <-ctx.Done():
						return
					}
				}
				if ch.Delta.Content != "" {
					select {
					case out <- Chunk{Delta: ch.Delta.Content}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
		if err := sc.Err(); err != nil {
			out <- Chunk{Err: err}
		}
	}()
	return out, nil
}
