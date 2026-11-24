package tui

import (
	"context"
	"strings"
	"testing"
)

// rowID returns a single-character ASCII string for use as a row ID.
func rowIDChar(i int) string {
	return string(byte('0' + i%10))
}

// buildTranscriptRows creates a transcript with N diff rows, each consisting of
// a hunk header and added/deleted lines. Each row has 300+ lines of diff content.
func buildTranscriptRows(n int) []transcriptRow {
	rows := make([]transcriptRow, 0, n)
	for i := 0; i < n; i++ {
		// Build diff lines for this row
		var diffBuf strings.Builder
		diffBuf.WriteString("--- a/test_file_")
		diffBuf.WriteByte(byte('a' + i%26))
		diffBuf.WriteString(".go\n")
		diffBuf.WriteString("+++ b/test_file_")
		diffBuf.WriteByte(byte('a' + i%26))
		diffBuf.WriteString(".go\n")
		diffBuf.WriteString("@@ -1,1 +1,1 @@\n")

		for j := 0; j < 150; j++ {
			diffBuf.WriteString("+func example")
			diffBuf.WriteByte(byte('a' + byte(j%26)))
			diffBuf.WriteString("() string { return \"old")
			diffBuf.WriteByte(byte('A' + byte(j%26)))
			diffBuf.WriteString("\" }\n")
			diffBuf.WriteString("-func example")
			diffBuf.WriteByte(byte('a' + byte(j%26)))
			diffBuf.WriteString("() string { return \"new")
			diffBuf.WriteByte(byte('A' + byte(j%26)))
			diffBuf.WriteString("\" }\n")
		}

		rows = append(rows, transcriptRow{
			kind:     rowToolCall,
			id:       rowIDChar(i),
			runID:    0, // runID=0 means settled (not pending)
			tool:     "bash",
			detail:   diffBuf.String(),
			final:    true,
			expanded: true,
		})
	}
	return rows
}

// TestRenderDiffCPUMeasuresCache measures render cache behavior across 50 frames
// with large diff transcripts. The hypothesis is that settled (non-pending) diff
// rows stay cached across frames. If the cache works, Misses should not grow.
func TestRenderDiffCPUMeasuresCache(t *testing.T) {
	// Replace the default cache with a test instance so we can measure stats
	cache := newStaticRenderCache(256, 512*1024)
	previous := defaultRenderCache
	defaultRenderCache = cache
	defer func() { defaultRenderCache = previous }()

	// Create a model with settled (non-pending) diff rows
	rows := buildTranscriptRows(50)

	m := newModel(context.Background(), Options{})
	m.transcript = rows
	m.flushed = 0
	m.flushedAny = false
	m.flushedPreviousKind = rowWelcome
	m.flushedHavePreviousKind = false
	m.altScreen = true
	m.width = 80
	m.height = 50
	m.pending = false
	m.activeRunID = 0

	// Run 50 frames and measure cache stats
	var frameStats [51]renderCacheStats // index 1..50

	for frame := 1; frame <= 50; frame++ {
		// Call transcriptView which triggers the rendering pipeline
		m.altScreen = true
		m.width = 80
		m.height = 50
		_ = m.transcriptView()

		stats := cache.stats()
		frameStats[frame] = stats
	}

	// Verify the hypothesis:
	// If the hypothesis is TRUE (cache doesn't work for settled lines):
	//   - Misses should grow from frame 1 to frame 50
	//   - highlightCodeWithLexerAndLineBackgrounds should be called > 0 times per frame

	// If the hypothesis is FALSE (cache works for settled lines):
	//   - Misses should NOT grow (delta = 0)
	//   - highlightCodeWithLexerAndLineBackgrounds calls per frame should be 0

	// Check: Misses delta from frame 1 to frame 50
	missesDelta := frameStats[50].Misses - frameStats[1].Misses
	evictionsDelta := frameStats[50].Evictions - frameStats[1].Evictions

	t.Logf("Frame 1 stats - Hits: %d, Misses: %d, Evictions: %d, SkippedOversized: %d",
		frameStats[1].Hits, frameStats[1].Misses, frameStats[1].Evictions, frameStats[1].SkippedOversized)
	t.Logf("Frame 50 stats - Hits: %d, Misses: %d, Evictions: %d, SkippedOversized: %d",
		frameStats[50].Hits, frameStats[50].Misses, frameStats[50].Evictions, frameStats[50].SkippedOversized)
	t.Logf("Deltas 1->50: Misses=%d Evictions=%d Hits=%d SkippedOversized=%d",
		missesDelta, evictionsDelta, frameStats[50].Hits-frameStats[1].Hits, frameStats[50].SkippedOversized-frameStats[1].SkippedOversized)

	// The test should PASS (hypothesis refuted) when:
	// - Misses delta is 0 (cache works for settled lines)
	// - Or we can confirm the cache is functioning correctly

	if missesDelta != 0 {
		t.Fatalf("HYPOTHESIS CONFIRMED: Misses grew from %d to %d (delta=%d) across 50 frames for settled diff rows",
			frameStats[1].Misses, frameStats[50].Misses, missesDelta)
	}

	if evictionsDelta > 10 {
		t.Logf("WARNING: %d evictions across 50 frames (may be within cache capacity)", evictionsDelta)
	}

	// Verify the test passes: cache works for settled diff rows
	t.Logf("PASS: Cache works for settled diff rows - Misses did not grow from frame 1 to frame 50 (delta=%d)", missesDelta)
}
