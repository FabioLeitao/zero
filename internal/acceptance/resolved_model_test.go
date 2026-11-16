package acceptance

// Acceptance tests of task 2.1: the provider_usage event of a session records
// the model that really answered (resolvedModel), cleaned and only when it
// differs from the requested one. Spec: docs/desenho/2.1.md, A2.1-1 to A2.1-12.
//
// Tests that need the feature are wrapped in pending(t, "2.1", ...). A2.1-9 and
// A2.1-11 guard behavior that already exists and are not pending.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	requestedModel = "gpt-test"
	codexRequested = "gpt-5"
)

var sessionSeq atomic.Int64

// escapedLiteral encodes s as a JSON string literal where every non-printable
// ASCII and every non-ASCII character is written as a \uXXXX escape.
func escapedLiteral(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r >= 0x20 && r < 0x7f:
			b.WriteRune(r)
		case r > 0xFFFF:
			r1, r2 := utf16.EncodeRune(r)
			fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// rawLiteral encodes s as a JSON string literal the way encoding/json does:
// C0 controls are escaped, C1 and format characters are written as raw UTF-8.
func rawLiteral(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (e usageEvent) has(key string) bool { _, ok := e.Payload[key]; return ok }

func (e usageEvent) str(t testing.TB, key string) string {
	t.Helper()
	var v string
	if raw, ok := e.Payload[key]; ok {
		if err := json.Unmarshal(raw, &v); err != nil {
			setupFatal(t, "payload %s is not a string: %s", key, raw)
		}
	}
	return v
}

func (e usageEvent) num(t testing.TB, key string) int {
	t.Helper()
	var v int
	if raw, ok := e.Payload[key]; ok {
		if err := json.Unmarshal(raw, &v); err != nil {
			setupFatal(t, "payload %s is not a number: %s", key, raw)
		}
	}
	return v
}

func (e usageEvent) keys() []string {
	var ks []string
	for k := range e.Payload {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func (e usageEvent) raw() map[string]string {
	m := map[string]string{}
	for k, v := range e.Payload {
		m[k] = string(v)
	}
	return m
}

// chatRun describes one `zero exec` against a fake chat-completions provider.
type chatRun struct {
	// literal is the JSON string literal of the "model" key; "" omits the key.
	literal string
	// everyChunk puts the model on all chunks, not only on the usage chunk.
	everyChunk bool
	// requested is the model of the profile (default requestedModel).
	requested          string
	prompt, completion int
}

func (s *sandbox) newSessionID() string {
	return fmt.Sprintf("acc-%d", sessionSeq.Add(1))
}

// execChat runs the real `zero exec` once and returns the session id.
func execChat(t *testing.T, s *sandbox, c chatRun) (string, result, *fakeProvider) {
	t.Helper()
	p := newFakeProvider(t)
	p.ModelLiteral, p.ModelOnEveryChunk = c.literal, c.everyChunk
	if c.prompt != 0 {
		p.Prompt, p.Completion = c.prompt, c.completion
	}
	model := c.requested
	if model == "" {
		model = requestedModel
	}
	s.writeConfig(map[string]any{
		"name": "fake", "catalogID": "custom-openai-compatible",
		"baseURL": p.URL(), "apiKey": "test-key", "model": model,
	})
	id := s.newSessionID()
	r := s.zero("exec", "--prompt", "hello", "--init-session-id", id, "-C", s.Work)
	return id, r, p
}

// execCodex runs the real `zero exec` against a fake Responses-API backend
// through the chatgpt catalog profile, with a static auth header (no login).
func execCodex(t *testing.T, s *sandbox, literal, created string) (string, result) {
	t.Helper()
	p := newFakeProvider(t)
	p.ModelLiteral, p.CreatedModelLiteral = literal, created
	s.writeConfig(map[string]any{
		"name": "chatgpt", "catalogID": "chatgpt",
		"baseURL": p.Server.URL, "authHeaderValue": "Bearer fake-token", "model": codexRequested,
	})
	id := s.newSessionID()
	r := s.zero("exec", "--prompt", "hello", "--init-session-id", id, "-C", s.Work)
	return id, r
}

func (s *sandbox) onlyUsage(id string) usageEvent {
	s.t.Helper()
	evs := s.usageEvents(id)
	if len(evs) != 1 {
		setupFatal(s.t, "session %s has %d provider_usage events, want 1", id, len(evs))
	}
	return evs[0]
}

// chatUsage runs a chat session and returns its only provider_usage event.
func chatUsage(t *testing.T, s *sandbox, c chatRun) usageEvent {
	t.Helper()
	id, r, _ := execChat(t, s, c)
	if r.Code != 0 {
		setupFatal(t, "zero exec exited %d\nstderr: %s", r.Code, r.Stderr)
	}
	return s.onlyUsage(id)
}

// requireFeature runs a control session whose reported model is plainly
// different from the requested one and asserts that it is recorded. It keeps the
// "key is absent" tests from passing just because the feature does not exist.
func requireFeature(t *testing.T, s *sandbox) {
	t.Helper()
	const want = "deepseek/control-model"
	ev := chatUsage(t, s, chatRun{literal: escapedLiteral(want), everyChunk: true})
	expectf(t, ev.str(t, "resolvedModel") == want,
		"control: a reported model different from the requested one must be recorded as resolvedModel=%q, payload=%v", want, ev.raw())
}

// assertCleanLine fails when the stored event line contains, raw or as a JSON
// \u escape, any of the banned runes.
func assertCleanLine(t testing.TB, line string, banned []rune) {
	t.Helper()
	for _, r := range banned {
		raw := string(r)
		esc := fmt.Sprintf(`\u%04x`, r)
		escUp := fmt.Sprintf(`\u%04X`, r)
		if strings.Contains(line, raw) || strings.Contains(line, esc) || strings.Contains(line, escUp) {
			expectf(t, false, "event line contains U+%04X (raw or escaped): %q", r, line)
		}
	}
}

func runesIn(ranges ...[2]rune) []rune {
	var out []rune
	for _, rg := range ranges {
		for r := rg[0]; r <= rg[1]; r++ {
			out = append(out, r)
		}
	}
	return out
}

var (
	controlRunes = runesIn([2]rune{0x00, 0x1f}, [2]rune{0x7f, 0x9f})
	formatRunes  = runesIn([2]rune{0x200b, 0x200f}, [2]rune{0x2028, 0x202e}, [2]rune{0x2066, 0x2069}, [2]rune{0xfeff, 0xfeff}, [2]rune{0x00ad, 0x00ad})
)

// --- A2.1-1 ---------------------------------------------------------------

func TestA2_1_1_ResolvedModelRecordedWithTokens(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		ev := chatUsage(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true, prompt: 123, completion: 45})
		expectf(t, ev.str(t, "resolvedModel") == "deepseek/deepseek-r1:free",
			"resolvedModel = %q, want %q (payload %v)", ev.str(t, "resolvedModel"), "deepseek/deepseek-r1:free", ev.raw())
		expectf(t, ev.num(t, "promptTokens") == 123, "promptTokens = %d, want 123", ev.num(t, "promptTokens"))
		expectf(t, ev.num(t, "completionTokens") == 45, "completionTokens = %d, want 45", ev.num(t, "completionTokens"))
		expectf(t, ev.num(t, "totalTokens") == 168, "totalTokens = %d, want 168", ev.num(t, "totalTokens"))
	})
}

// --- A2.1-2 ---------------------------------------------------------------

func TestA2_1_2_OmittedWhenEqualToRequested(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		echoed := chatUsage(t, s, chatRun{literal: `"` + requestedModel + `"`, everyChunk: true})
		today := chatUsage(t, s, chatRun{}) // same server, no model key: the event of today
		expectf(t, !echoed.has("resolvedModel"), "resolvedModel present although the reported model equals the requested one: %v", echoed.raw())
		expectf(t, reflect.DeepEqual(echoed.raw(), today.raw()),
			"payload differs from the event of today: got %v, want %v", echoed.raw(), today.raw())
		requireFeature(t, s)
	})
}

// --- A2.1-3 ---------------------------------------------------------------

const evilControl = "\x1b[31mevil\x1b[0m\x1b]0;title\x07\x00\x7f\u0085\u009b-x"

func TestA2_1_3_ControlCharactersRemoved(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		for name, lit := range map[string]string{"escaped": escapedLiteral(evilControl), "raw-utf8": rawLiteral(evilControl)} {
			t.Run(name, func(t *testing.T) {
				s := newSandbox(t)
				ev := chatUsage(t, s, chatRun{literal: lit})
				const want = "[31mevil[0m]0;title-x"
				expectf(t, ev.str(t, "resolvedModel") == want, "resolvedModel = %q, want %q", ev.str(t, "resolvedModel"), want)
				assertCleanLine(t, ev.Line, controlRunes)
			})
		}
	})
}

// --- A2.1-4 ---------------------------------------------------------------

const evilFormat = "\u202eevil\u200b-\ufeffx\u00ad\u2066y\u2069"

func TestA2_1_4_FormatCharactersRemoved(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		for name, lit := range map[string]string{"escaped": escapedLiteral(evilFormat), "raw-utf8": rawLiteral(evilFormat)} {
			t.Run(name, func(t *testing.T) {
				s := newSandbox(t)
				ev := chatUsage(t, s, chatRun{literal: lit})
				expectf(t, ev.str(t, "resolvedModel") == "evil-xy", "resolvedModel = %q, want %q", ev.str(t, "resolvedModel"), "evil-xy")
				assertCleanLine(t, ev.Line, formatRunes)
			})
		}
	})
}

// --- A2.1-5 ---------------------------------------------------------------

func TestA2_1_5_LengthCapped(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		sent := strings.Repeat("\u6a21", 100_000)
		id, r, _ := execChat(t, s, chatRun{literal: rawLiteral(sent)})
		expectf(t, r.Code == 0, "zero exec did not finish normally: exit %d, stderr %q", r.Code, r.Stderr)
		got := s.onlyUsage(id).str(t, "resolvedModel")
		expectf(t, got != "", "resolvedModel missing for an oversized reported model")
		expectf(t, len(got) <= 256, "resolvedModel has %d bytes, want at most 256", len(got))
		expectf(t, got != "" && len(got) == 255, "resolvedModel has %d bytes, want 255 (the largest rune-aligned prefix)", len(got))
		expectf(t, utf8.ValidString(got), "resolvedModel is not valid UTF-8")
		expectf(t, strings.HasPrefix(sent, got), "resolvedModel is not a prefix of the reported model")
	})
}

// --- A2.1-6 ---------------------------------------------------------------

func TestA2_1_6_CleanedBeforeComparing(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		for name, reported := range map[string]string{"nul": "gpt-\x00test", "bidi": "gpt-\u202etest"} {
			t.Run(name, func(t *testing.T) {
				s := newSandbox(t)
				ev := chatUsage(t, s, chatRun{literal: escapedLiteral(reported), everyChunk: true})
				expectf(t, !ev.has("resolvedModel"), "resolvedModel recorded (%q) although the cleaned value equals the requested model", ev.str(t, "resolvedModel"))
				requireFeature(t, s)
			})
		}
	})
}

// --- A2.1-7 and A2.1-8 (Codex) --------------------------------------------
// Premise confirmed: the chatgpt catalog profile runs against a fake Responses
// server in an isolated environment with a static authHeaderValue, no login.

func TestA2_1_7_CodexRecordsCleanedResolvedModel(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		// response.created carries another model: only the terminal event counts.
		id, r := execCodex(t, s, escapedLiteral("\x1b[31mgpt-5-2025-08-07"), `"ignored/created-model"`)
		if r.Code != 0 {
			setupFatal(t, "zero exec exited %d\nstderr: %s", r.Code, r.Stderr)
		}
		ev := s.onlyUsage(id)
		const want = "[31mgpt-5-2025-08-07"
		expectf(t, ev.str(t, "resolvedModel") == want, "resolvedModel = %q, want %q", ev.str(t, "resolvedModel"), want)
		expectf(t, ev.num(t, "promptTokens") == 11 && ev.num(t, "completionTokens") == 7, "tokens changed: %v", ev.raw())
		assertCleanLine(t, ev.Line, controlRunes)
	})
}

func TestA2_1_8_CodexOmittedWhenEqualOrAbsent(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		for name, lit := range map[string]string{"equal": `"` + codexRequested + `"`, "absent": ""} {
			id, r := execCodex(t, s, lit, "")
			if r.Code != 0 {
				setupFatal(t, "%s: zero exec exited %d\nstderr: %s", name, r.Code, r.Stderr)
			}
			ev := s.onlyUsage(id)
			expectf(t, !ev.has("resolvedModel"), "%s: resolvedModel recorded: %v", name, ev.raw())
		}
		// Control: a dated model differs from the requested one and is recorded.
		id, _ := execCodex(t, s, `"gpt-5-2025-08-07"`, "")
		expectf(t, s.onlyUsage(id).str(t, "resolvedModel") == "gpt-5-2025-08-07", "control: a different response.model must be recorded")
	})
}

// --- A2.1-9 (guard, not pending) -------------------------------------------

func TestA2_1_9_NoModelInChunksLeavesEventUnchanged(t *testing.T) {
	s := newSandbox(t)
	ev := chatUsage(t, s, chatRun{prompt: 31, completion: 17})
	expectf(t, reflect.DeepEqual(ev.keys(), []string{"completionTokens", "promptTokens", "totalTokens"}),
		"payload keys = %v, want exactly completionTokens, promptTokens, totalTokens", ev.keys())
	expectf(t, ev.num(t, "promptTokens") == 31 && ev.num(t, "completionTokens") == 17 && ev.num(t, "totalTokens") == 48, "tokens: %v", ev.raw())
}

// --- A2.1-10 ---------------------------------------------------------------

type usageTotal struct {
	Requests, InputTokens, OutputTokens, TotalTokens int
	TotalCost                                        float64
}

func (s *sandbox) reportJSON(id string) usageTotal {
	s.t.Helper()
	r := s.zero("usage", "report", "--json", "--session", id)
	if r.Code != 0 {
		setupFatal(s.t, "usage report exited %d: %s", r.Code, r.Stderr)
	}
	var rep struct {
		Total usageTotal `json:"total"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &rep); err != nil {
		setupFatal(s.t, "usage report is not JSON: %v", err)
	}
	return rep.Total
}

func TestA2_1_10_PriceAndTokensUnchanged(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		// gpt-4o has a price in the embedded registry.
		withID, r1, _ := execChat(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true, requested: "gpt-4o", prompt: 1000, completion: 500})
		plainID, r2, _ := execChat(t, s, chatRun{requested: "gpt-4o", prompt: 1000, completion: 500})
		if r1.Code != 0 || r2.Code != 0 {
			setupFatal(t, "zero exec failed: %d %d", r1.Code, r2.Code)
		}
		withEv, plainEv := s.onlyUsage(withID), s.onlyUsage(plainID)
		expectf(t, withEv.has("resolvedModel"), "precondition: the session with a reported model must record resolvedModel: %v", withEv.raw())
		expectf(t, !withEv.has("model") && !plainEv.has("model"), "payload has a model key (no escalation here): %v / %v", withEv.raw(), plainEv.raw())
		with, plain := s.reportJSON(withID), s.reportJSON(plainID)
		expectf(t, plain.TotalCost > 0, "precondition: gpt-4o must be priced, got cost %v", plain.TotalCost)
		expectf(t, with == plain, "usage report differs: with resolvedModel %+v, without %+v", with, plain)
		expectf(t, with.InputTokens == 1000 && with.OutputTokens == 500 && with.TotalTokens == 1500, "tokens: %+v", with)
	})
}

// --- A2.1-11 (guard, not pending) ------------------------------------------

func TestA2_1_11_UsageReportStaysClean(t *testing.T) {
	s := newSandbox(t)
	var ids []string
	for _, lit := range []string{escapedLiteral(evilControl), escapedLiteral(evilFormat), rawLiteral(evilFormat)} {
		id, r, _ := execChat(t, s, chatRun{literal: lit})
		if r.Code != 0 {
			setupFatal(t, "zero exec exited %d: %s", r.Code, r.Stderr)
		}
		ids = append(ids, id)
	}
	banned := append(append([]rune{}, controlRunes...), formatRunes...)
	for _, id := range ids {
		for _, args := range [][]string{{"usage", "report", "--session", id}, {"usage", "report", "--json", "--session", id}} {
			r := s.zero(args...)
			if r.Code != 0 {
				setupFatal(t, "%v exited %d: %s", args, r.Code, r.Stderr)
			}
			out := strings.ReplaceAll(r.Stdout, "\n", "")
			assertCleanLine(t, out, banned)
			expectf(t, !strings.Contains(out, "evil"), "%v shows the reported model: %q", args, out)
		}
	}
}

// --- A2.1-12 ---------------------------------------------------------------

func TestA2_1_12_OnlyRemovableCharactersLeaveNothing(t *testing.T) {
	pending(t, "2.1", func(t *testing.T) {
		s := newSandbox(t)
		ev := chatUsage(t, s, chatRun{literal: escapedLiteral("\x1b\x00\u200b"), everyChunk: true})
		expectf(t, !ev.has("resolvedModel"), "resolvedModel recorded for a value made only of removable characters: %q", ev.str(t, "resolvedModel"))
		requireFeature(t, s)
	})
}
