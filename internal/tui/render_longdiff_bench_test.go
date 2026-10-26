package tui

import (
	"context"
	"testing"
)

// BenchmarkViewSettledDiffRows benchmarks View() with a settled (non-pending)
// diff transcript. This is a light synthetic control; earlier measurements of
// the same benchmark name cost ~463 ms/op with a working run in progress.
// See zero#21 for the field problem.
func BenchmarkViewSettledDiffRows(b *testing.B) {
	// Replace the default cache with a test instance so we can measure stats
	cache := newStaticRenderCache(256, 512*1024)
	previous := defaultRenderCache
	defaultRenderCache = cache
	defer func() { defaultRenderCache = previous }()

	// Create a model with settled (non-pending) diff rows
	rows := buildTranscriptRows(50)

	m := newModel(context.Background(), Options{
		Cwd:          "/work/zero",
		ProviderName: "openai",
		ModelName:    "gpt-test",
	})
	m.width = 96
	m.height = 30
	m.headerPrinted = true
	m.altScreen = true
	m.transcript = rows
	m.flushed = 0
	m.flushedAny = false
	m.flushedPreviousKind = rowWelcome
	m.pending = false
	m.activeRunID = 0

	for i := 0; i < b.N; i++ {
		_ = m.transcriptView()
	}
}
