package cost

import (
	"testing"
	"time"
)

func TestTracker(t *testing.T) {
	t.Run("new tracker", func(t *testing.T) {
		tracker := NewTracker()
		if tracker.GetTotalCost() != 0 {
			t.Error("expected zero cost")
		}
	})

	t.Run("add api call", func(t *testing.T) {
		tracker := NewTracker()
		cost := tracker.AddAPICall("claude-sonnet-4-20250514", Usage{
			InputTokens:  1000,
			OutputTokens: 500,
		}, 2*time.Second)

		if cost <= 0 {
			t.Error("expected positive cost")
		}
		if tracker.GetTotalCost() <= 0 {
			t.Error("expected positive total cost")
		}
	})

	t.Run("format cost", func(t *testing.T) {
		tracker := NewTracker()
		tracker.AddAPICall("claude-sonnet-4-20250514", Usage{
			InputTokens:  1000000,
			OutputTokens: 500000,
		}, time.Second)

		formatted := tracker.FormatTotalCost()
		if formatted == "" {
			t.Error("expected formatted cost string")
		}
	})

	t.Run("summary", func(t *testing.T) {
		tracker := NewTracker()
		tracker.AddAPICall("claude-sonnet-4-20250514", Usage{
			InputTokens:  1000,
			OutputTokens: 500,
		}, time.Second)

		summary := tracker.GetSummary()
		if summary == "" {
			t.Error("expected summary string")
		}
	})

	t.Run("tool duration", func(t *testing.T) {
		tracker := NewTracker()
		tracker.AddToolDuration(5 * time.Second)
		if tracker.TotalToolDuration != 5*time.Second {
			t.Errorf("expected 5s, got %v", tracker.TotalToolDuration)
		}
	})

	t.Run("line changes", func(t *testing.T) {
		tracker := NewTracker()
		tracker.AddLineChanges(10, 5)
		if tracker.TotalLinesAdded != 10 || tracker.TotalLinesRemoved != 5 {
			t.Errorf("expected 10 added / 5 removed, got %d / %d",
				tracker.TotalLinesAdded, tracker.TotalLinesRemoved)
		}
	})
}
