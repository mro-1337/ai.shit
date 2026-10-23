package ai

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/config"
)

// OpenAIBrain talks to any OpenAI-compatible chat completions endpoint.
type OpenAIBrain struct {
	cfg    *config.Config
	client *http.Client
	log    *slog.Logger
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewOpenAIBrain builds a client for the configured endpoint.
func NewOpenAIBrain(cfg *config.Config, log *slog.Logger) *OpenAIBrain {
	if log == nil {
		log = slog.Default()
	}
	return &OpenAIBrain{
		cfg: cfg,
		log: log,
		client: &http.Client{
			Timeout: time.Duration(cfg.AI.TimeoutSec+5) * time.Second,
		},
	}
}

// Name implements Brain.
func (b *OpenAIBrain) Name() string { return "llm:" + b.cfg.AI.Model }

// Decide implements Brain. If the model returns malformed JSON it gets one
// chance to correct itself.
func (b *OpenAIBrain) Decide(ctx context.Context, obs Observation) (Decision, error) {
	messages := []chatMessage{
		{Role: "system", Content: SystemPrompt(b.cfg, obs)},
		{Role: "user", Content: UserPrompt(b.cfg, obs)},
	}
	raw, err := b.complete(ctx, messages)
	if err != nil {
		return Decision{}, err
	}
	d, err := ParseDecision(raw)
	if err == nil {
		b.log.Debug("decision", "action", d.Action, "thought", d.Thought)
		return d, nil
	}

	b.log.Warn("model returned an invalid decision; asking for a correction", "err", err, "raw", truncate(raw, 300))
	messages = append(messages,
		chatMessage{Role: "assistant", Content: raw},
		chatMessage{Role: "user", Content: "That was not a valid decision. Reply with exactly one JSON object matching the schema and nothing else."},
	)
	raw2, err2 := b.complete(ctx, messages)
	if err2 != nil {
		return Decision{}, fmt.Errorf("invalid model reply (%v) and retry failed: %w", err, err2)
	}
	d, err = ParseDecision(raw2)
	if err != nil {
		return Decision{}, fmt.Errorf("invalid model reply after retry: %w", err)
	}
	return d, nil
}
