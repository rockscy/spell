package llm

import (
	"context"
	"errors"
)

// Chunk is a single streamed token batch from a Provider.
type Chunk struct {
	Delta string
	Err   error
}

// Provider is the minimal contract every LLM backend must satisfy.
// Stream returns a channel of Chunks; the channel is closed when
// generation completes or the context is cancelled.
type Provider interface {
	Name() string
	Stream(ctx context.Context, system, user string) (<-chan Chunk, error)
}

var (
	ErrNoAPIKey = errors.New("provider: api key not set")
	ErrNoModel  = errors.New("provider: model not set")
	ErrNoURL    = errors.New("provider: base_url not set")
)
