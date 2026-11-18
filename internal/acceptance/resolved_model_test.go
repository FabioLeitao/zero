package acceptance

// Acceptance tests of task 2.1: the provider_usage event of a session records
// the model that really answered (resolvedModel), cleaned and only when it
// differs from the requested one. Spec: docs/desenho/2.1.md, A2.1-1 to A2.1-15.
//
// Tests that fail on the base of the fork are wrapped in pending(t, "2.1", ...).
// A2.1-1, A2.1-9 and A2.1-10 are not pending: the fork base already records a
// raw resolvedModel, so they pass there (they fail on a bare upstream/main
// until the recording commits land). A2.1-11 and A2.1-13 to A2.1-15 need the
// report display commit (R10) and are pending.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	s := newSandbox(t)
	ev := chatUsage(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true, prompt: 123, completion: 45})
	expectf(t, ev.str(t, "resolvedModel") == "deepseek/deepseek-r1:free",
		"resolvedModel = %q, want %q (payload %v)", ev.str(t, "resolvedModel"), "deepseek/deepseek-r1:free", ev.raw())
	expectf(t, ev.num(t, "promptTokens") == 123, "promptTokens = %d, want 123", ev.num(t, "promptTokens"))
	expectf(t, ev.num(t, "completionTokens") == 45, "completionTokens = %d, want 45", ev.num(t, "completionTokens"))
	expectf(t, ev.num(t, "totalTokens") == 168, "totalTokens = %d, want 168", ev.num(t, "totalTokens"))
}

// --- A2.1-2 ---------------------------------------------------------------

func TestA2_1_2_OmittedWhenEqualToRequested(t *testing.T) {
	s := newSandbox(t)
	echoed := chatUsage(t, s, chatRun{literal: `"` + requestedModel + `"`, everyChunk: true})
	today := chatUsage(t, s, chatRun{}) // same server, no model key: the event of today
	expectf(t, !echoed.has("resolvedModel"), "resolvedModel present although the reported model equals the requested one: %v", echoed.raw())
	expectf(t, reflect.DeepEqual(echoed.raw(), today.raw()),
		"payload differs from the event of today: got %v, want %v", echoed.raw(), today.raw())
	requireFeature(t, s)
}

// --- A2.1-3 ---------------------------------------------------------------

const evilControl = "\x1b[31mevil\x1b[0m\x1b]0;title\x07\x00\x7f\u0085\u009b-x"

func TestA2_1_3_ControlCharactersRemoved(t *testing.T) {
	for name, lit := range map[string]string{"escaped": escapedLiteral(evilControl), "raw-utf8": rawLiteral(evilControl)} {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			ev := chatUsage(t, s, chatRun{literal: lit})
			const want = "[31mevil[0m]0;title-x"
			expectf(t, ev.str(t, "resolvedModel") == want, "resolvedModel = %q, want %q", ev.str(t, "resolvedModel"), want)
			assertCleanLine(t, ev.Line, controlRunes)
		})
	}
}

// --- A2.1-4 ---------------------------------------------------------------

const evilFormat = "\u202eevil\u200b-\ufeffx\u00ad\u2066y\u2069"

func TestA2_1_4_FormatCharactersRemoved(t *testing.T) {
	for name, lit := range map[string]string{"escaped": escapedLiteral(evilFormat), "raw-utf8": rawLiteral(evilFormat)} {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			ev := chatUsage(t, s, chatRun{literal: lit})
			expectf(t, ev.str(t, "resolvedModel") == "evil-xy", "resolvedModel = %q, want %q", ev.str(t, "resolvedModel"), "evil-xy")
			assertCleanLine(t, ev.Line, formatRunes)
		})
	}
}

// --- A2.1-5 ---------------------------------------------------------------

func TestA2_1_5_LengthCapped(t *testing.T) {
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
}

// --- A2.1-6 ---------------------------------------------------------------

func TestA2_1_6_CleanedBeforeComparing(t *testing.T) {
	for name, reported := range map[string]string{"nul": "gpt-\x00test", "bidi": "gpt-\u202etest"} {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			ev := chatUsage(t, s, chatRun{literal: escapedLiteral(reported), everyChunk: true})
			expectf(t, !ev.has("resolvedModel"), "resolvedModel recorded (%q) although the cleaned value equals the requested model", ev.str(t, "resolvedModel"))
			requireFeature(t, s)
		})
	}
}

// --- A2.1-7 and A2.1-8 (Codex) --------------------------------------------
// Premise confirmed: the chatgpt catalog profile runs against a fake Responses
// server in an isolated environment with a static authHeaderValue, no login.

func TestA2_1_7_CodexRecordsCleanedResolvedModel(t *testing.T) {
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
}

func TestA2_1_8_CodexOmittedWhenEqualOrAbsent(t *testing.T) {
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
}

// --- A2.1-11 (R10) ---------------------------------------------------------

const reportBlockHeader = "resolved models (reported by provider; cost uses the requested model)"

type resolvedEntry struct {
	RequestedModel string `json:"requestedModel"`
	ResolvedModel  string `json:"resolvedModel"`
	Requests       int    `json:"requests"`
	TotalTokens    int    `json:"totalTokens"`
}

type fullReport struct {
	Total          usageTotal      `json:"total"`
	ResolvedModels []resolvedEntry `json:"resolvedModels"`
	keys           map[string]json.RawMessage
}

// reports runs `zero usage report` in text and in --json form with args
// appended (for example --session <id>) and returns both outputs.
func (s *sandbox) reports(args ...string) (string, fullReport, string, int) {
	s.t.Helper()
	txt := s.zero(append([]string{"usage", "report"}, args...)...)
	js := s.zero(append([]string{"usage", "report", "--json"}, args...)...)
	var rep fullReport
	if err := json.Unmarshal([]byte(js.Stdout), &rep); err != nil {
		setupFatal(s.t, "usage report --json is not JSON: %v\n%s", err, js.Stdout)
	}
	if err := json.Unmarshal([]byte(js.Stdout), &rep.keys); err != nil {
		setupFatal(s.t, "usage report --json is not an object: %v", err)
	}
	code := txt.Code
	if js.Code != 0 {
		code = js.Code
	}
	return txt.Stdout, rep, js.Stdout, code
}

// textBlock returns the lines of the "resolved models" block of the text
// report (after its header, up to the blank line) and whether the header was
// found after the "total" line.
func textBlock(text string) ([]string, bool) {
	lines := strings.Split(text, "\n")
	total, header := -1, -1
	for i, l := range lines {
		if strings.HasPrefix(l, "total") && total < 0 {
			total = i
		}
		if l == reportBlockHeader {
			header = i
		}
	}
	if header < 0 {
		return nil, false
	}
	var block []string
	for _, l := range lines[header+1:] {
		if strings.TrimSpace(l) == "" {
			break
		}
		block = append(block, l)
	}
	return block, total >= 0 && header > total
}

// expectOneAttribution asserts the report of one session shows exactly one
// attribution gpt-test -> want, with the tokens of the fake server (18).
func expectOneAttribution(t *testing.T, text string, rep fullReport, jsonOut, want string) {
	t.Helper()
	block, afterTotal := textBlock(text)
	expectf(t, afterTotal, "text report lacks the %q block after the total line:\n%s", reportBlockHeader, text)
	wantLine := "  requests 1, tokens 18: gpt-test -> " + want
	expectf(t, len(block) == 1 && block[0] == wantLine, "text block = %q, want [%q]", block, wantLine)
	wantJSON := []resolvedEntry{{RequestedModel: requestedModel, ResolvedModel: want, Requests: 1, TotalTokens: 18}}
	expectf(t, reflect.DeepEqual(rep.ResolvedModels, wantJSON), "resolvedModels = %+v, want %+v", rep.ResolvedModels, wantJSON)
	banned := append(append([]rune{}, controlRunes...), formatRunes...)
	assertCleanLine(t, strings.ReplaceAll(text, "\n", ""), banned)
	assertCleanLine(t, strings.ReplaceAll(jsonOut, "\n", ""), banned)
}

func TestA2_1_11_UsageReportShowsCleanResolvedModel(t *testing.T) {
	cases := []struct{ name, literal, want, rawValue string }{
		{"control-escaped", escapedLiteral(evilControl), "[31mevil[0m]0;title-x", evilControl},
		{"format-escaped", escapedLiteral(evilFormat), "evil-xy", evilFormat},
		{"format-raw-utf8", rawLiteral(evilFormat), "evil-xy", evilFormat},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSandbox(t)
			id := chatSession(t, s, chatRun{literal: c.literal})
			text, rep, js, code := s.reports("--session", id)
			expectf(t, code == 0, "usage report exited %d", code)
			expectOneAttribution(t, text, rep, js, c.want)
			expectf(t, !strings.Contains(text, c.rawValue) && !strings.Contains(js, c.rawValue), "the raw reported value reached the report")
		})
	}
}

// chatSession runs a chat session and returns its id, failing the setup when
// zero exec fails.
func chatSession(t *testing.T, s *sandbox, c chatRun) string {
	t.Helper()
	id, r, _ := execChat(t, s, c)
	if r.Code != 0 {
		setupFatal(t, "zero exec exited %d\nstderr: %s", r.Code, r.Stderr)
	}
	return id
}

// --- A2.1-12 ---------------------------------------------------------------

func TestA2_1_12_OnlyRemovableCharactersLeaveNothing(t *testing.T) {
	s := newSandbox(t)
	ev := chatUsage(t, s, chatRun{literal: escapedLiteral("\x1b\x00\u200b"), everyChunk: true})
	expectf(t, !ev.has("resolvedModel"), "resolvedModel recorded for a value made only of removable characters: %q", ev.str(t, "resolvedModel"))
	requireFeature(t, s)
}

// requireReportFeature runs a control session whose reported model differs from
// the requested one and asserts that the report shows it, so that the "nothing
// to show" tests cannot pass just because the report has no such block.
func requireReportFeature(t *testing.T, s *sandbox) {
	t.Helper()
	id := chatSession(t, s, chatRun{literal: `"deepseek/control-model"`, everyChunk: true})
	_, rep, _, _ := s.reports("--session", id)
	expectf(t, len(rep.ResolvedModels) == 1 && rep.ResolvedModels[0].ResolvedModel == "deepseek/control-model",
		"control: the report must show a reported model different from the requested one, resolvedModels = %+v", rep.ResolvedModels)
}

// expectNothingShown asserts the report of a session has no attribution block.
func expectNothingShown(t *testing.T, s *sandbox, label, id string) {
	t.Helper()
	text, rep, _, code := s.reports("--session", id)
	expectf(t, code == 0, "%s: usage report exited %d", label, code)
	expectf(t, !strings.Contains(text, "resolved models") && !strings.Contains(text, " -> "), "%s: text report shows an attribution:\n%s", label, text)
	_, has := rep.keys["resolvedModels"]
	expectf(t, !has, "%s: JSON report has a resolvedModels key: %s", label, rep.keys["resolvedModels"])
}

// --- A2.1-13 (R10) ---------------------------------------------------------

func TestA2_1_13_NothingToShow(t *testing.T) {
	s := newSandbox(t)
	equal := chatSession(t, s, chatRun{literal: `"` + requestedModel + `"`, everyChunk: true})
	absent := chatSession(t, s, chatRun{})
	removable := chatSession(t, s, chatRun{literal: escapedLiteral("\x1b\x00\u200b"), everyChunk: true})
	expectNothingShown(t, s, "equal to requested", equal)
	expectNothingShown(t, s, "no model", absent)
	expectNothingShown(t, s, "only removable characters", removable)
	requireReportFeature(t, s)
}

// --- A2.1-14 (R10) ---------------------------------------------------------

// rewriteResolvedModel edits the stored provider_usage event of a session the
// way an older build would have written it. The literal is a JSON string
// literal, or "" to drop the key.
func (s *sandbox) rewriteResolvedModel(id, literal string) {
	s.t.Helper()
	path := filepath.Join(s.Data, "zero", "sessions", id, "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		setupFatal(s.t, "read events: %v", err)
	}
	var out []string
	edited := false
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		var env map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &env) == nil && string(env["type"]) == `"provider_usage"` {
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(env["payload"], &payload); err != nil {
				setupFatal(s.t, "payload: %v", err)
			}
			payload["resolvedModel"] = json.RawMessage(literal)
			pb, _ := json.Marshal(payload)
			env["payload"] = pb
			lb, _ := json.Marshal(env)
			line, edited = string(lb), true
		}
		out = append(out, line)
	}
	if !edited {
		setupFatal(s.t, "no provider_usage event to edit")
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o600); err != nil {
		setupFatal(s.t, "write events: %v", err)
	}
	// Premise: the session store accepts the edited file.
	if _, rep, _, code := s.reports("--session", id); code != 0 || rep.Total.Requests != 1 {
		setupFatal(s.t, "premise failed: the store does not accept the edited events.jsonl (exit %d, requests %d)", code, rep.Total.Requests)
	}
}

func TestA2_1_14_CleansOldSessionsOnRead(t *testing.T) {
	const oldStyle = "\x1b[31mold\u0085\u202e-x"
	t.Run("a-escaped", func(t *testing.T) {
		s := newSandbox(t)
		id := chatSession(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true})
		s.rewriteResolvedModel(id, escapedLiteral(oldStyle))
		text, rep, js, _ := s.reports("--session", id)
		expectOneAttribution(t, text, rep, js, "[31mold-x")
	})
	t.Run("a-raw-utf8", func(t *testing.T) {
		s := newSandbox(t)
		id := chatSession(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true})
		s.rewriteResolvedModel(id, rawLiteral(oldStyle))
		text, rep, js, _ := s.reports("--session", id)
		expectOneAttribution(t, text, rep, js, "[31mold-x")
	})
	t.Run("b-oversized", func(t *testing.T) {
		s := newSandbox(t)
		id := chatSession(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true})
		sent := strings.Repeat("\u6a21", 100_000)
		s.rewriteResolvedModel(id, rawLiteral(sent))
		text, rep, _, code := s.reports("--session", id)
		expectf(t, code == 0, "usage report exited %d", code)
		if expectf(t, len(rep.ResolvedModels) == 1, "resolvedModels = %+v, want one entry", rep.ResolvedModels) {
			got := rep.ResolvedModels[0].ResolvedModel
			expectf(t, len(got) == 255 && utf8.ValidString(got) && strings.HasPrefix(sent, got), "JSON value has %d bytes (valid %v), want 255 bytes, a prefix of the stored value", len(got), utf8.ValidString(got))
			block, _ := textBlock(text)
			expectf(t, len(block) == 1 && strings.HasSuffix(block[0], " -> "+got), "text block does not show the capped value: %d line(s)", len(block))
		}
	})
	t.Run("c-equal-after-cleaning", func(t *testing.T) {
		s := newSandbox(t)
		for name, lit := range map[string]string{"nul": escapedLiteral("gpt-\x00test"), "same": `"gpt-test"`} {
			id := chatSession(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true})
			s.rewriteResolvedModel(id, lit)
			expectNothingShown(t, s, name, id)
		}
		requireReportFeature(t, s)
	})
}

// --- A2.1-15 (R10) ---------------------------------------------------------

func TestA2_1_15_MixedLinesAndOrder(t *testing.T) {
	s := newSandbox(t)
	for _, lit := range []string{`"deepseek/deepseek-r1:free"`, `"x/y"`, "", `"deepseek/deepseek-r1:free"`} {
		chatSession(t, s, chatRun{literal: lit, everyChunk: true})
	}
	text, rep, _, code := s.reports()
	expectf(t, code == 0, "usage report exited %d", code)
	expectf(t, rep.Total.Requests == 4, "total.requests = %d, want 4", rep.Total.Requests)
	want := []resolvedEntry{
		{RequestedModel: requestedModel, ResolvedModel: "deepseek/deepseek-r1:free", Requests: 2, TotalTokens: 36},
		{RequestedModel: requestedModel, ResolvedModel: "x/y", Requests: 1, TotalTokens: 18},
	}
	expectf(t, reflect.DeepEqual(rep.ResolvedModels, want), "resolvedModels = %+v, want %+v", rep.ResolvedModels, want)
	block, afterTotal := textBlock(text)
	expectf(t, afterTotal, "text report lacks the block after the total line:\n%s", text)
	wantBlock := []string{
		"  requests 2, tokens 36: gpt-test -> deepseek/deepseek-r1:free",
		"  requests 1, tokens 18: gpt-test -> x/y",
	}
	expectf(t, reflect.DeepEqual(block, wantBlock), "text block = %q, want %q", block, wantBlock)
	var header []string
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "date") {
			header = strings.Fields(l)
		}
	}
	expectf(t, reflect.DeepEqual(header, []string{"date", "requests", "tokens", "est.", "cost"}), "day table header = %q, want the one of today without a new column", header)
}
