package tui

import (
	"strings"
	"testing"
)

// TestRenderDiffCPUMeasuresCache mede o comportamento do render cache
// em 50 frames consecutivos com transcripts de diffs grandes.
// Verifica se as linhas assentadas ficam no cache (hypotese refutada)
// ou se o cache é invalidado a cada frame (hypotese confirmada).
func TestRenderDiffCPUMeasuresCache(t *testing.T) {
	// Cria um modelo base e popula o transcript com 50 rows de diff Go.
	m := transcriptViewTestModel()
	m.altScreen = true
	m.pending = false // estado "settled": cache deve funcionar
	m.activeRunID = 0 // garante stable=true para rowToolCall

	const numRows = 50
	const linesPerRow = 300

	// Gera 50 diffs de Go (+ e - linhas), cada um com ~300 linhas.
	for i := 0; i < numRows; i++ {
		var plusLines, minusLines []string
		for j := 0; j < linesPerRow; j++ {
			if j%2 == 0 {
				plusLines = append(plusLines, "+ func handle"+string(rune('0'+i%10))+"(x int) int {")
				minusLines = append(minusLines, "- func handle"+string(rune('0'+i%10))+"(x int) int {")
			} else {
				plusLines = append(plusLines, "// some comment line "+string(rune('a'+j%26)))
				minusLines = append(minusLines, "// some comment line "+string(rune('a'+j%26)))
			}
		}
		diffContent := "--- a/file.go\n+++ b/file.go\n" + strings.Join(plusLines, "\n") + "\n" + strings.Join(minusLines, "\n")
		m.transcript = append(m.transcript, transcriptRow{
			kind:  rowToolCall,
			id:    "diff-" + string(rune('0'+i%10)),
			text:  diffContent,
			runID: 0,
		})
	}
	m.flushed = len(m.transcript)
	m.flushedAny = true

	width := m.chatColumnWidth()

	// Captura estatísticas após o 1º frame
	firstFrameStats := defaultRenderCache.statsData

	// Executa 50 frames de transcriptBodyItems
	for i := 0; i < 50; i++ {
		_ = m.transcriptBodyItems(width, "", false)
	}

	// Captura estatísticas após o 50º frame
	frame50Stats := defaultRenderCache.statsData

	// Calcula delta
	missDelta := frame50Stats.Misses - firstFrameStats.Misses
	evictDelta := frame50Stats.Evictions - firstFrameStats.Evictions
	hitDelta := frame50Stats.Hits - firstFrameStats.Hits
	skippedDelta := frame50Stats.SkippedOversized - firstFrameStats.SkippedOversized

	t.Logf("=== Render Cache Stats ===")
	t.Logf("Frame 1:     Hits=%d Misses=%d Evictions=%d SkippedOversized=%d",
		firstFrameStats.Hits, firstFrameStats.Misses, firstFrameStats.Evictions, firstFrameStats.SkippedOversized)
	t.Logf("Frame 50:    Hits=%d Misses=%d Evictions=%d SkippedOversized=%d",
		frame50Stats.Hits, frame50Stats.Misses, frame50Stats.Evictions, frame50Stats.SkippedOversized)
	t.Logf("Deltas 1→50: Hits=%+d Misses=%+d Evictions=%+d SkippedOversized=%+d",
		hitDelta, missDelta, evictDelta, skippedDelta)

	// A hipótese afirma que o cache NÃO retém linhas assentadas,
	// então Misses deveria crescer a cada frame.
	// Se Misses permanecer estável (delta = 0), a hipótese é REFUTADA.
	if missDelta > 0 {
		t.Errorf("Hipótese CONFIRMADA: Misses cresceram de %d para %d (delta=%d). O cache não retém linhas assentadas.",
			firstFrameStats.Misses, frame50Stats.Misses, missDelta)
		return
	}

	t.Logf("Hipótese REFUTADA: O cache retém linhas assentadas across frames (Misses delta = %d).", missDelta)
	t.Logf("Arquivo de teste: internal/tui/render_diff_cpu_test.go")
	t.Logf("Commit: test(tui): add render diff CPU cache measures benchmark")
}

// buildLongDiffTranscriptModel cria um modelo com transcript de diffs longos
// para benchmark de performance.
func buildLongDiffTranscriptModel() model {
	m := transcriptViewTestModel()
	m.altScreen = true
	m.pending = false // settled state
	m.activeRunID = 0

	const numRows = 50
	const linesPerRow = 300

	for i := 0; i < numRows; i++ {
		var plusLines, minusLines []string
		for j := 0; j < linesPerRow; j++ {
			if j%2 == 0 {
				plusLines = append(plusLines, "+ func handle"+string(rune('0'+i%10))+"(x int) int {")
				minusLines = append(minusLines, "- func handle"+string(rune('0'+i%10))+"(x int) int {")
			} else {
				plusLines = append(plusLines, "// some comment line "+string(rune('a'+j%26)))
				minusLines = append(minusLines, "// some comment line "+string(rune('a'+j%26)))
			}
		}
		diffContent := "--- a/file.go\n+++ b/file.go\n" + strings.Join(plusLines, "\n") + "\n" + strings.Join(minusLines, "\n")
		m.transcript = append(m.transcript, transcriptRow{
			kind:  rowToolCall,
			id:    "diff-" + string(rune('0'+i%10)),
			text:  diffContent,
			runID: 0,
		})
	}
	m.flushed = len(m.transcript)
	m.flushedAny = true
	// Prime o settled cache
	m, _ = m.settleTranscript()
	return m
}

// BenchmarkViewLongDiffTranscript mede o custo de renderizar um transcript
// com 50 diffs grandes (300 linhas cada) no estado settled.
func BenchmarkViewLongDiffTranscript(b *testing.B) {
	m := buildLongDiffTranscriptModel()
	width := m.chatColumnWidth()

	// Verifica que o settled cache está primado
	if m.altScreenSettledFrontier != m.flushed || m.altScreenSettledWidth != width {
		b.Fatalf("settled cache miss before benchmark: frontier=%d flushed=%d width=%d wantWidth=%d",
			m.altScreenSettledFrontier, m.flushed, m.altScreenSettledWidth, width)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.transcriptBodyItems(width, "", false)
	}
}
