// Package cost tracks API usage costs across a session.
package cost

import (
	"fmt"
	"sync"
	"time"
)

// ModelUsage tracks cumulative usage for a specific model.
type ModelUsage struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// Tracker tracks costs and usage across a session.
type Tracker struct {
	mu sync.Mutex

	TotalCostUSD                    float64              `json:"total_cost_usd"`
	TotalAPIDuration                time.Duration        `json:"total_api_duration"`
	TotalAPIDurationWithoutRetries  time.Duration        `json:"total_api_duration_without_retries"`
	TotalToolDuration               time.Duration        `json:"total_tool_duration"`
	TotalLinesAdded                 int                  `json:"total_lines_added"`
	TotalLinesRemoved               int                  `json:"total_lines_removed"`
	ModelUsage                      map[string]*ModelUsage `json:"model_usage"`
}

// NewTracker creates a new cost tracker.
func NewTracker() *Tracker {
	return &Tracker{
		ModelUsage: make(map[string]*ModelUsage),
	}
}

// Usage contains token counts from an API call.
type Usage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// Pricing contains the per-token prices for a model.
type Pricing struct {
	InputPer1M      float64
	OutputPer1M     float64
	CacheReadPer1M  float64
	CacheWritePer1M float64
}

// Known model pricing (per million tokens).
var modelPricing = map[string]Pricing{
	"claude-sonnet-4-20250514":    {InputPer1M: 3.0, OutputPer1M: 15.0, CacheReadPer1M: 0.30, CacheWritePer1M: 3.75},
	"claude-3-5-sonnet-20241022":  {InputPer1M: 3.0, OutputPer1M: 15.0, CacheReadPer1M: 0.30, CacheWritePer1M: 3.75},
	"claude-3-5-haiku-20241022":   {InputPer1M: 0.80, OutputPer1M: 4.0, CacheReadPer1M: 0.08, CacheWritePer1M: 1.0},
	"claude-opus-4-20250514":      {InputPer1M: 15.0, OutputPer1M: 75.0, CacheReadPer1M: 1.50, CacheWritePer1M: 18.75},
}

func getPricing(model string) Pricing {
	if p, ok := modelPricing[model]; ok {
		return p
	}
	// Default to Sonnet pricing
	return modelPricing["claude-sonnet-4-20250514"]
}

// AddAPICall records the cost of an API call.
func (t *Tracker) AddAPICall(model string, usage Usage, duration time.Duration) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	pricing := getPricing(model)
	cost := float64(usage.InputTokens)/1_000_000*pricing.InputPer1M +
		float64(usage.OutputTokens)/1_000_000*pricing.OutputPer1M +
		float64(usage.CacheReadInputTokens)/1_000_000*pricing.CacheReadPer1M +
		float64(usage.CacheCreationInputTokens)/1_000_000*pricing.CacheWritePer1M

	t.TotalCostUSD += cost
	t.TotalAPIDuration += duration

	mu := t.getOrCreateModelUsage(model)
	mu.InputTokens += usage.InputTokens
	mu.OutputTokens += usage.OutputTokens
	mu.CacheReadInputTokens += usage.CacheReadInputTokens
	mu.CacheCreationInputTokens += usage.CacheCreationInputTokens
	mu.CostUSD += cost

	return cost
}

// AddToolDuration records the duration of a tool execution.
func (t *Tracker) AddToolDuration(duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalToolDuration += duration
}

// AddLineChanges records lines added and removed.
func (t *Tracker) AddLineChanges(added, removed int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TotalLinesAdded += added
	t.TotalLinesRemoved += removed
}

func (t *Tracker) getOrCreateModelUsage(model string) *ModelUsage {
	if mu, ok := t.ModelUsage[model]; ok {
		return mu
	}
	mu := &ModelUsage{}
	t.ModelUsage[model] = mu
	return mu
}

// GetTotalCost returns the total cost in USD.
func (t *Tracker) GetTotalCost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.TotalCostUSD
}

// FormatTotalCost returns a formatted string of the total cost.
func (t *Tracker) FormatTotalCost() string {
	cost := t.GetTotalCost()
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// GetSummary returns a human-readable summary of costs and usage.
func (t *Tracker) GetSummary() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var totalInput, totalOutput int
	for _, mu := range t.ModelUsage {
		totalInput += mu.InputTokens
		totalOutput += mu.OutputTokens
	}

	costStr := fmt.Sprintf("$%.4f", t.TotalCostUSD)
	if t.TotalCostUSD >= 0.01 {
		costStr = fmt.Sprintf("$%.2f", t.TotalCostUSD)
	}

	return fmt.Sprintf("Cost: %s | Tokens: %dk in, %dk out | Duration: %s",
		costStr,
		totalInput/1000,
		totalOutput/1000,
		t.TotalAPIDuration.Round(time.Second),
	)
}
