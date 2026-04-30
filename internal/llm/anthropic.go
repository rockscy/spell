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

// Anthropic implements the native Messages streaming protocol.
type Anthropic struct {
	BaseURL    string // default: https://api.anthropic.com
	APIKey     string
	Model      string
	Version    string // default: 2023-06-01
	MaxTokens  int    // default: 1024
	HTTPClient *http.Client
}

func (a *Anthropic) Name() string { return "anthropic" }

type anthReq struct {
	Model     string        `json:"model"`
	System    string        `json:"system,omitempty"`
	Messages  []anthMessage `json:"messages"`
	Stream    bool          `json:"stream"`
	MaxTokens int           `json:"max_tokens"`
}

type anthMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func (a *Anthropic) Stream(ctx context.Context, system, user string) (<-chan Chunk, error) {
	if a.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	if a.Model == "" {
		return nil, ErrNoModel
	}
	base := a.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	version := a.Version
	if version == "" {
		version = "2023-06-01"
	}
	maxTok := a.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}

	body, err := json.Marshal(anthReq{
		Model:     a.Model,
		System:    system,
		Messages:  []anthMessage{{Role: "user", Content: user}},
		Stream:    true,
		MaxTokens: maxTok,
	})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(base, "/") + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", version)
	req.Header.Set("Accept", "text/event-stream")

	client := a.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, strings.TrimSpace(string(buf)))
	}

	out := make(chan Chunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 || !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(payload) == 0 {
				continue
			}
			var ev anthStreamEvent
			if err := json.Unmarshal(payload, &ev); err != nil {
				continue
			}
			if ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
				select {
				case out <- Chunk{Delta: ev.Delta.Text}:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := sc.Err(); err != nil {
			out <- Chunk{Err: err}
		}
	}()
	return out, nil
}
