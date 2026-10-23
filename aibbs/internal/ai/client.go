package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/config"
)

func (b *OpenAIBrain) complete(ctx context.Context, messages []chatMessage) (string, error) {
	endpoint := strings.TrimRight(b.cfg.AI.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	jsonMode := b.cfg.AI.JSONMode
	payload := map[string]any{
		"model":       b.cfg.AI.Model,
		"messages":    messages,
		"temperature": b.cfg.AI.Temperature,
		"max_tokens":  b.cfg.AI.MaxTokens,
		"stream":      false,
	}
	if jsonMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		if b.cfg.AI.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+b.cfg.AI.APIKey)
		}
		for k, v := range b.cfg.AI.Headers {
			req.Header.Set(k, v)
		}

		resp, err := b.client.Do(req)
		if err != nil {
			lastErr = err
			sleepCtx(ctx, backoff(attempt))
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return parseCompletion(data)
		}

		lastErr = fmt.Errorf("http %d from %s: %s", resp.StatusCode, endpoint, truncate(string(data), 400))
		// Some servers do not implement response_format; retry without it.
		if resp.StatusCode == http.StatusBadRequest && jsonMode {
			b.log.Info("endpoint rejected JSON mode; retrying without response_format")
			jsonMode = false
			delete(payload, "response_format")
			body, _ = json.Marshal(payload)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			sleepCtx(ctx, backoff(attempt))
			continue
		}
		return "", lastErr
	}
	return "", fmt.Errorf("chat completion failed after retries: %w", lastErr)
}

func backoff(attempt int) time.Duration {
	d := time.Duration(1<<attempt) * 800 * time.Millisecond
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func parseCompletion(data []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("invalid completion response: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("endpoint error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("completion had no choices")
	}
	msg := resp.Choices[0].Message
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		content = strings.TrimSpace(msg.ReasoningContent)
	}
	if content == "" {
		return "", fmt.Errorf("completion was empty")
	}
	return content, nil
}

// NewBrain chooses the LLM or the offline heuristic based on the config.
func NewBrain(cfg *config.Config, log *slog.Logger, forceHeuristic bool) (Brain, error) {
	provider := strings.ToLower(cfg.AI.Provider)
	if forceHeuristic || provider == "heuristic" || provider == "none" {
		return NewHeuristicBrain(), nil
	}
	if cfg.AI.APIKey == "" && !isLocalEndpoint(cfg.AI.BaseURL) {
		return nil, fmt.Errorf("no API key for %s (set ai.api_key, ai.api_key_env, or use --brain heuristic)", cfg.AI.BaseURL)
	}
	return NewOpenAIBrain(cfg, log), nil
}

func isLocalEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
