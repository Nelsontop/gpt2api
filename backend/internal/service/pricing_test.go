package service

import (
	"testing"

	"github.com/kleinai/backend/internal/provider"
)

func TestDefaultPriceFnChargesOnePointForGPTImageWebRoute(t *testing.T) {
	got := DefaultPriceFn("gpt-image-2", provider.KindImage, map[string]any{
		"size": "1024x1024",
	})

	if got != 100 {
		t.Fatalf("expected chatgpt_web default cost 100, got %d", got)
	}
}

func TestDefaultPriceFnLeavesGPTImage2CodexRouteUnchanged(t *testing.T) {
	got := DefaultPriceFn("gpt-image-2", provider.KindImage, map[string]any{
		"size": "2048x2048",
	})

	if got != 0 {
		t.Fatalf("expected non-chatgpt_web default cost 0, got %d", got)
	}
}
