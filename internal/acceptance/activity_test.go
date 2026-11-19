package acceptance

// Acceptance tests of task 2.6 (FabioLeitao/zero#45): the activity block that
// `zero usage report` prints after the report of today, and the "activity" key
// of `--json`. Spec: docs/desenho/2.6.md, A2.6-1 to A2.6-10.
//
// HOW THE DEV TURNS THESE TESTS ON (one line, in this file only):
//
//	const activityImplemented = false   ->   const activityImplemented = true
//
// While the constant is false every test below is skipped with the reason
// "pending task 2.6", except TestA2_6_0_PendingSwitch, which is the strict
// guard: it FAILS when the binary already prints the activity block and the
// constant is still false (the equivalent of XPASS(strict): the pending mark
// cannot be forgotten). With the constant true and no implementation, every
// test fails on its own assertion (ACCEPT-FAIL), never on the preparation
// (SETUP-FAIL).
//
// Black box: the real `zero` binary, the fake provider of the 2.1 tests and the
// files a client can see (events.jsonl). The binary uses the real clock, the
// machine zone and the environment, so no test here asserts the heatmap layout
// or a date that depends on "today": that is the job of the D2.6-* tests, with
// the clock, the zone and the locale injected. Event dates are derived from the
// timestamps the real `zero exec` wrote ("anchor"), never from this process's
// own clock, so the active days and the streaks are the same in any fixed zone.
//
// Premises (docs/desenho/2.6.md): P1, the store accepts lines appended to the
// events.jsonl of a real session (checked by appendEvents on every call; a
// refusal is a SETUP-FAIL, not a test failure); P2, TZ only works on Linux and
// macOS, so the tests that need a zone skip or relax on Windows.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// activityImplemented is the ONLY switch the dev flips (see the top of the file).
const activityImplemented = false

// pendingActivity skips a test until task 2.6 is implemented.
func pendingActivity(t *testing.T) {
	t.Helper()
	if !activityImplemented {
		t.Skip("pending: task 2.6 (#45); the dev sets activityImplemented = true in internal/acceptance/activity_test.go")
	}
}

const (
	activityHeaderPrefix = "activity (local days, UTC"
	activityEmptyLine    = "  no activity in the selected range"
)

var (
	activityHeaderRE = regexp.MustCompile(`^activity \(local days, UTC[+-][0-9]{2}:[0-9]{2}\)$`)
	// topKeysToday are the keys of `zero usage report --json` without a model
	// attribution (A2.1-13), in order, followed by the new one.
	topKeysWithActivity = []string{"buckets", "total", "netLOC", "netLOCPositive", "tokensPerNetLOC", "costPerNetLOC", "locEstimated", "costEstimated", "activity"}
)

// --- JSON of the report ------------------------------------------------------

type actTools struct {
	Results         int  `json:"results"`
	OK              int  `json:"ok"`
	Error           int  `json:"error"`
	Unknown         int  `json:"unknown"`
	SuccessPerMille *int `json:"successPerMille"`
}

type actStreak struct {
	Days int    `json:"days"`
	From string `json:"from"`
	To   string `json:"to"`
}

type actDay struct {
	Date        string `json:"date"`
	Events      int    `json:"events"`
	Prompts     int    `json:"prompts"`
	Steps       int    `json:"steps"`
	ToolResults int    `json:"toolResults"`
}

type activityJSON struct {
	UTCOffset          string    `json:"utcOffset"`
	HeatmapFrom        string    `json:"heatmapFrom"`
	HeatmapTo          string    `json:"heatmapTo"`
	Sessions           int       `json:"sessions"`
	Prompts            int       `json:"prompts"`
	Steps              int       `json:"steps"`
	TotalTokens        int       `json:"totalTokens"`
	Tools              actTools  `json:"tools"`
	ActiveDays         int       `json:"activeDays"`
	LongestStreak      actStreak `json:"longestStreak"`
	UndatedEvents      int       `json:"undatedEvents"`
	UnreadablePayloads int       `json:"unreadablePayloads"`
	Days               []actDay  `json:"days"`
}

// actReport is the --json document of `zero usage report`, decoded the way a
// client would: the ordered top-level keys, and the parts the tests read.
type actReport struct {
	Raw         string
	Keys        []string
	Total       usageTotal
	Buckets     []usageTotal
	Activity    activityJSON
	ActivityRaw json.RawMessage
	ToolsRaw    map[string]json.RawMessage
}

// orderedKeys returns the top-level keys of a JSON object in document order.
func orderedKeys(t testing.TB, doc string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(doc))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		setupFatal(t, "usage report --json is not an object: %v", err)
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			setupFatal(t, "usage report --json: %v", err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			setupFatal(t, "usage report --json: %v", err)
		}
	}
	return keys
}

// reportDoc runs `zero usage report --json args...` and decodes it. A non-zero
// exit is returned, not fatal, so a test can assert it.
func (s *sandbox) reportDoc(args ...string) (actReport, result) {
	s.t.Helper()
	r := s.zero(append([]string{"usage", "report", "--json"}, args...)...)
	var doc actReport
	doc.Raw = r.Stdout
	if r.Code != 0 {
		return doc, r
	}
	doc.Keys = orderedKeys(s.t, r.Stdout)
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r.Stdout), &top); err != nil {
		setupFatal(s.t, "usage report --json is not JSON: %v\n%s", err, r.Stdout)
	}
	_ = json.Unmarshal(top["total"], &doc.Total)
	_ = json.Unmarshal(top["buckets"], &doc.Buckets)
	doc.ActivityRaw = top["activity"]
	_ = json.Unmarshal(top["activity"], &doc.Activity)
	var act map[string]json.RawMessage
	if json.Unmarshal(top["activity"], &act) == nil {
		_ = json.Unmarshal(act["tools"], &doc.ToolsRaw)
	}
	return doc, r
}

// mustReportDoc is reportDoc for a run that has to succeed (the report of today
// works the same before and after the task).
func (s *sandbox) mustReportDoc(args ...string) actReport {
	s.t.Helper()
	doc, r := s.reportDoc(args...)
	if r.Code != 0 {
		expectf(s.t, false, "zero usage report --json %v exited %d\nstderr: %s", args, r.Code, r.Stderr)
	}
	return doc
}

// reportText runs `zero usage report args...`.
func (s *sandbox) reportText(args ...string) result {
	s.t.Helper()
	return s.zero(append([]string{"usage", "report"}, args...)...)
}

// textLines splits the output in lines, dropping the final newline.
func textLines(out string) []string {
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// activityBlock returns the lines of the activity block (from its header line to
// the end of the output) and whether the header was found.
func activityBlock(out string) ([]string, bool) {
	lines := textLines(out)
	for i, l := range lines {
		if strings.HasPrefix(l, activityHeaderPrefix) {
			return lines[i:], true
		}
	}
	return nil, false
}

// --- real sessions and appended events ---------------------------------------

// rawEvent is one line to append to the events.jsonl of a session. payload is a
// JSON text; "" omits the key.
type rawEvent struct {
	typ, createdAt, payload string
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

const promptPayload = `{"role":"user","content":"x"}`

// appendEvents adds events to the events.jsonl of an existing session, the file a
// client could edit (premise P1), and checks that the store still accepts it:
// the report of the session must still exit 0 and count the same requests.
func (s *sandbox) appendEvents(id string, events ...rawEvent) {
	s.t.Helper()
	path := filepath.Join(s.Data, "zero", "sessions", id, "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		setupFatal(s.t, "read events: %v", err)
	}
	maxSeq := 0
	for _, line := range strings.Split(string(data), "\n") {
		var e struct {
			Sequence int `json:"sequence"`
		}
		if line != "" && json.Unmarshal([]byte(line), &e) == nil && e.Sequence > maxSeq {
			maxSeq = e.Sequence
		}
	}
	before := s.reportRequests(id)
	var buf bytes.Buffer
	if len(data) > 0 && data[len(data)-1] != '\n' {
		buf.WriteByte('\n')
	}
	for i, ev := range events {
		seq := maxSeq + 1 + i
		idJSON, _ := json.Marshal("x" + strconv.Itoa(seq))
		sidJSON, _ := json.Marshal(id)
		typJSON, _ := json.Marshal(ev.typ)
		atJSON, _ := json.Marshal(ev.createdAt)
		fmt.Fprintf(&buf, `{"id":%s,"sessionId":%s,"sequence":%d,"type":%s,"createdAt":%s`, idJSON, sidJSON, seq, typJSON, atJSON)
		if ev.payload != "" {
			fmt.Fprintf(&buf, `,"payload":%s`, ev.payload)
		}
		buf.WriteString("}\n")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		setupFatal(s.t, "open events for append: %v", err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		setupFatal(s.t, "append events: %v", err)
	}
	if err := f.Close(); err != nil {
		setupFatal(s.t, "close events: %v", err)
	}
	if after := s.reportRequests(id); after != before {
		setupFatal(s.t, "premise P1 failed: the store does not accept the appended lines (requests %d -> %d)", before, after)
	}
}

// reportRequests is total.requests of one session, read from the report of
// today (the same before and after the task). A refusal is a SETUP-FAIL: it is
// how premise P1 is checked.
func (s *sandbox) reportRequests(id string) int {
	s.t.Helper()
	r := s.zero("usage", "report", "--json", "--session", id)
	if r.Code != 0 {
		setupFatal(s.t, "premise P1 failed: usage report of session %s exited %d: %s", id, r.Code, r.Stderr)
	}
	var rep struct {
		Total usageTotal `json:"total"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &rep); err != nil {
		setupFatal(s.t, "usage report is not JSON: %v", err)
	}
	return rep.Total.Requests
}

// anchor is the time of the last event of a session, as the real `zero exec`
// wrote it. All the dates the tests append are anchor - k*24h.
func (s *sandbox) anchor(id string) time.Time {
	s.t.Helper()
	data, err := os.ReadFile(filepath.Join(s.Data, "zero", "sessions", id, "events.jsonl"))
	if err != nil {
		setupFatal(s.t, "read events: %v", err)
	}
	var first, last time.Time
	for _, line := range strings.Split(string(data), "\n") {
		var e struct {
			CreatedAt string `json:"createdAt"`
		}
		if line == "" || json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		at, err := time.Parse(time.RFC3339, e.CreatedAt)
		if err != nil {
			setupFatal(s.t, "event createdAt %q is not RFC3339: %v", e.CreatedAt, err)
		}
		if first.IsZero() {
			first = at
		}
		last = at
	}
	if last.IsZero() {
		setupFatal(s.t, "session %s has no events", id)
	}
	if first.UTC().Format("2006-01-02") != last.UTC().Format("2006-01-02") {
		s.t.Skip("the real zero exec crossed midnight UTC while recording the session; run it again")
	}
	return last
}

// execSession runs the real `zero exec` once (one prompt, one model call of
// 11+7 tokens) and returns the session id.
func execSession(t *testing.T, s *sandbox) string {
	t.Helper()
	id, r, _ := execChat(t, s, chatRun{})
	if r.Code != 0 {
		setupFatal(t, "zero exec exited %d\nstderr: %s", r.Code, r.Stderr)
	}
	return id
}

// requireSameUTCDay skips the test when midnight UTC passed since anchor, so a
// "today" derived from the anchor stays valid.
func requireSameUTCDay(t *testing.T, anchor time.Time) {
	t.Helper()
	if time.Now().UTC().Format("2006-01-02") != anchor.UTC().Format("2006-01-02") {
		t.Skip("midnight UTC passed during the test; run it again")
	}
}

// toolStates are the A2.6-4 results: ok x3, error x1, FAILED x1, "" x1, "x" x1.
func toolStateEvents(at time.Time) []rawEvent {
	ts := stamp(at)
	return []rawEvent{
		{"tool_result", ts, `{"status":"ok"}`},
		{"tool_result", ts, `{"status":"ok"}`},
		{"tool_result", ts, `{"status":"ok"}`},
		{"tool_result", ts, `{"status":"error"}`},
		{"tool_result", ts, `{"status":"FAILED"}`},
		{"tool_result", ts, `{"status":""}`},
		{"tool_result", ts, `"x"`},
	}
}

// undatedEvents are the A2.6-6 events: a prompt without a valid date and a
// message whose payload is not an object.
func undatedEvents(at time.Time) []rawEvent {
	return []rawEvent{
		{"message", "not-a-time", promptPayload},
		{"message", stamp(at), `"oops"`},
	}
}

// --- tests ---------------------------------------------------------------------

// TestA2_6_0_PendingSwitch is the strict guard of the pending mark. While the
// switch is false, the binary must not print the activity block; when it does,
// the task is implemented and the dev has to turn the tests on.
func TestA2_6_0_PendingSwitch(t *testing.T) {
	if activityImplemented {
		t.Skip("the switch is on: the A2.6-* tests are active")
	}
	s := newSandbox(t)
	text := s.reportText()
	doc, _ := s.reportDoc()
	_, hasBlock := activityBlock(text.Stdout)
	hasKey := false
	for _, k := range doc.Keys {
		hasKey = hasKey || k == "activity"
	}
	expectf(t, !hasBlock && !hasKey,
		"task 2.6 is implemented (activity block: %v, \"activity\" key: %v) but activityImplemented is still false: set it to true in internal/acceptance/activity_test.go", hasBlock, hasKey)
}

func TestA2_6_1_NoSessions(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)

	r := s.reportText()
	expectf(t, r.Code == 0, "usage report exited %d\nstderr: %s", r.Code, r.Stderr)
	lines := textLines(r.Stdout)
	if expectf(t, len(lines) >= 3, "text report too short:\n%s", r.Stdout) {
		header := lines[len(lines)-2]
		expectf(t, lines[len(lines)-1] == activityEmptyLine, "last line = %q, want %q", lines[len(lines)-1], activityEmptyLine)
		expectf(t, activityHeaderRE.MatchString(header), "line before the last = %q, want %q followed by the UTC offset", header, activityHeaderPrefix)
		if runtime.GOOS != "windows" {
			expectf(t, header == "activity (local days, UTC+00:00)", "header = %q under TZ=UTC", header)
		}
		expectf(t, lines[len(lines)-3] == "", "the block must follow one blank line, got %q", lines[len(lines)-3])
	}

	doc, jr := s.reportDoc()
	if !expectf(t, jr.Code == 0, "usage report --json exited %d\nstderr: %s", jr.Code, jr.Stderr) {
		return
	}
	expectf(t, reflect.DeepEqual(doc.Keys, topKeysWithActivity), "top-level keys = %v, want %v", doc.Keys, topKeysWithActivity)
	a := doc.Activity
	expectf(t, a.Sessions == 0 && a.Prompts == 0 && a.Steps == 0 && a.TotalTokens == 0, "counts = %+v, want zeros", a)
	var act map[string]json.RawMessage
	_ = json.Unmarshal(doc.ActivityRaw, &act)
	expectf(t, string(bytes.TrimSpace(act["days"])) == "[]", `days = %s, want []`, act["days"])
	expectf(t, string(bytes.TrimSpace(doc.ToolsRaw["successPerMille"])) == "null", `tools.successPerMille = %s, want null`, doc.ToolsRaw["successPerMille"])
	var streak map[string]any
	_ = json.Unmarshal(act["longestStreak"], &streak)
	expectf(t, reflect.DeepEqual(streak, map[string]any{"days": float64(0)}), "longestStreak = %s, want {\"days\": 0}", act["longestStreak"])
}

func TestA2_6_2_OneRealSession(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	id := execSession(t, s)
	doc := s.mustReportDoc()
	a := doc.Activity
	expectf(t, a.Sessions == 1, "activity.sessions = %d, want 1", a.Sessions)
	expectf(t, a.Prompts == 1, "activity.prompts = %d, want 1", a.Prompts)
	expectf(t, doc.Total.Requests >= 1 && a.Steps == doc.Total.Requests, "activity.steps = %d, total.requests = %d, want them equal and >= 1", a.Steps, doc.Total.Requests)
	expectf(t, doc.Total.TotalTokens == 18 && a.TotalTokens == doc.Total.TotalTokens, "activity.totalTokens = %d, total.totalTokens = %d, want them equal (18 from the fake provider)", a.TotalTokens, doc.Total.TotalTokens)
	expectf(t, a.ActiveDays == 1, "activity.activeDays = %d, want 1", a.ActiveDays)
	expectf(t, a.LongestStreak.Days == 1, "activity.longestStreak.days = %d, want 1", a.LongestStreak.Days)
	if expectf(t, len(a.Days) == 1, "activity.days = %+v, want one day", a.Days) {
		expectf(t, a.Days[0].Prompts == 1, "days[0].prompts = %d, want 1", a.Days[0].Prompts)
	}
	_ = id
}

func TestA2_6_3_RestOfReportUnchanged(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	execSession(t, s)

	r := s.reportText()
	expectf(t, r.Code == 0, "usage report exited %d\nstderr: %s", r.Code, r.Stderr)
	lines := textLines(r.Stdout)
	expectf(t, len(lines) > 0 && lines[0] == "Usage report (cost is a reconstructed estimate)", "first line = %q", lines[0])
	var hasHeader, hasTotal bool
	costAt := -1
	for i, l := range lines {
		hasHeader = hasHeader || strings.HasPrefix(l, "date")
		hasTotal = hasTotal || strings.HasPrefix(l, "total")
		if strings.HasPrefix(l, "est. cost per net LOC:") {
			costAt = i
		}
	}
	expectf(t, hasHeader && hasTotal, "the day table (date header and total line) is missing:\n%s", r.Stdout)
	if expectf(t, costAt >= 0, "the line \"est. cost per net LOC: ...\" is missing:\n%s", r.Stdout) {
		expectf(t, len(lines) > costAt+2 && lines[costAt+1] == "" && activityHeaderRE.MatchString(lines[costAt+2]),
			"the cost line must be followed by a blank line and the activity header, got %q", lines[costAt+1:])
	}

	doc := s.mustReportDoc()
	expectf(t, doc.Total.Requests == 1 && doc.Total.InputTokens == 11 && doc.Total.OutputTokens == 7 && doc.Total.TotalTokens == 18,
		"total = %+v, want the values of the fake provider (1 request, 11 + 7 = 18 tokens)", doc.Total)
	if expectf(t, len(doc.Buckets) == 1, "buckets = %+v, want one", doc.Buckets) {
		expectf(t, doc.Buckets[0].Requests == 1 && doc.Buckets[0].TotalTokens == 18, "bucket = %+v, want 1 request and 18 tokens", doc.Buckets[0])
	}
	expectf(t, reflect.DeepEqual(doc.Keys, topKeysWithActivity), "top-level keys = %v, want %v", doc.Keys, topKeysWithActivity)
}

func TestA2_6_4_ToolResults(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	id := execSession(t, s)
	s.appendEvents(id, toolStateEvents(s.anchor(id))...)

	doc := s.mustReportDoc()
	wantRate := 600
	want := actTools{Results: 7, OK: 3, Error: 2, Unknown: 2, SuccessPerMille: &wantRate}
	got := doc.Activity.Tools
	same := got.Results == want.Results && got.OK == want.OK && got.Error == want.Error && got.Unknown == want.Unknown &&
		got.SuccessPerMille != nil && *got.SuccessPerMille == wantRate
	expectf(t, same, "activity.tools = %+v (rate %v), want results 7, ok 3, error 2, unknown 2, successPerMille 600", got, got.SuccessPerMille)
	expectf(t, doc.Activity.UnreadablePayloads == 1, "activity.unreadablePayloads = %d, want 1", doc.Activity.UnreadablePayloads)

	r := s.reportText()
	expectf(t, r.Code == 0, "usage report exited %d\nstderr: %s", r.Code, r.Stderr)
	const wantLine = "  tool results 7: ok 3, error 2, unknown 2, success 60.0%"
	expectf(t, containsLine(r.Stdout, wantLine), "text lacks the line %q:\n%s", wantLine, r.Stdout)
}

func containsLine(out, want string) bool {
	for _, l := range textLines(out) {
		if l == want {
			return true
		}
	}
	return false
}

func TestA2_6_5_DaysAndStreak(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	id := execSession(t, s)
	anchor := s.anchor(id)
	var evs []rawEvent
	for _, k := range []int{0, 1, 2, 5, 6} {
		evs = append(evs, rawEvent{"message", stamp(anchor.Add(-time.Duration(k) * 24 * time.Hour)), promptPayload})
	}
	s.appendEvents(id, evs...)

	doc := s.mustReportDoc()
	requireSameUTCDay(t, anchor)
	a := doc.Activity
	expectf(t, a.ActiveDays == 5, "activity.activeDays = %d, want 5 (days k = 0, 1, 2, 5, 6)", a.ActiveDays)
	expectf(t, a.LongestStreak.Days == 3, "activity.longestStreak.days = %d, want 3", a.LongestStreak.Days)
	if runtime.GOOS != "windows" { // P2: TZ=UTC only holds off Windows
		today := anchor.UTC().Format("2006-01-02")
		from := anchor.UTC().AddDate(0, 0, -2).Format("2006-01-02")
		expectf(t, a.LongestStreak.To == today && a.LongestStreak.From == from,
			"longestStreak = %s to %s, want %s to %s", a.LongestStreak.From, a.LongestStreak.To, from, today)
	}
}

func TestA2_6_6_UndatedAndInvalidDoNotBreak(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	id := execSession(t, s)
	s.appendEvents(id, undatedEvents(s.anchor(id))...)

	doc, jr := s.reportDoc()
	if !expectf(t, jr.Code == 0, "usage report --json exited %d\nstderr: %s", jr.Code, jr.Stderr) {
		return
	}
	a := doc.Activity
	expectf(t, a.UndatedEvents == 1, "activity.undatedEvents = %d, want 1", a.UndatedEvents)
	expectf(t, a.UnreadablePayloads == 1, "activity.unreadablePayloads = %d, want 1", a.UnreadablePayloads)
	expectf(t, a.Prompts == 2, "activity.prompts = %d, want 2 (the real one and the undated one; the \"oops\" message is not a prompt)", a.Prompts)

	r := s.reportText()
	expectf(t, r.Code == 0, "usage report exited %d\nstderr: %s", r.Code, r.Stderr)
	const wantLine = "  undated events 1, unreadable payloads 1"
	expectf(t, containsLine(r.Stdout, wantLine), "text lacks the line %q:\n%s", wantLine, r.Stdout)
}

func TestA2_6_7_TimeZone(t *testing.T) {
	pendingActivity(t)
	if runtime.GOOS == "windows" {
		t.Skip("P2: the Go runtime on Windows ignores TZ and uses the system zone")
	}
	if _, err := time.LoadLocation("Etc/GMT+3"); err != nil {
		setupFatal(t, "this host has no time zone database (Etc/GMT+3): %v", err)
	}
	s := newSandbox(t)
	id := execSession(t, s)
	anchor := s.anchor(id)
	today := anchor.UTC().Format("2006-01-02")
	yesterday := anchor.UTC().AddDate(0, 0, -1).Format("2006-01-02")
	s.appendEvents(id, rawEvent{"message", today + "T01:30:00Z", promptPayload})

	dayOf := func(a activityJSON, date string) (actDay, bool) {
		for _, d := range a.Days {
			if d.Date == date {
				return d, true
			}
		}
		return actDay{}, false
	}

	// TZ=UTC: both prompts are in today.
	utc := s.mustReportDoc()
	requireSameUTCDay(t, anchor)
	expectf(t, utc.Activity.UTCOffset == "+00:00", "TZ=UTC: activity.utcOffset = %q, want +00:00", utc.Activity.UTCOffset)
	d, ok := dayOf(utc.Activity, today)
	expectf(t, ok && d.Prompts == 2 && len(utc.Activity.Days) == 1, "TZ=UTC: days = %+v, want one day (%s) with 2 prompts", utc.Activity.Days, today)

	// TZ=Etc/GMT+3 (-03:00): the 01:30Z prompt is on the day before.
	s.Extra = []string{"TZ=Etc/GMT+3"}
	br := s.mustReportDoc()
	expectf(t, br.Activity.UTCOffset == "-03:00", "TZ=Etc/GMT+3: activity.utcOffset = %q, want -03:00", br.Activity.UTCOffset)
	d, ok = dayOf(br.Activity, yesterday)
	expectf(t, ok && d.Prompts >= 1, "TZ=Etc/GMT+3: days = %+v, want the 01:30Z prompt on %s", br.Activity.Days, yesterday)
	r := s.reportText()
	lines := textLines(r.Stdout)
	expectf(t, r.Code == 0 && containsLine(r.Stdout, "activity (local days, UTC-03:00)"), "TZ=Etc/GMT+3: header line \"activity (local days, UTC-03:00)\" missing (%d lines, exit %d)", len(lines), r.Code)
}

func TestA2_6_8_Filters(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	first := execSession(t, s)
	execSession(t, s)

	one := s.mustReportDoc("--session", first)
	expectf(t, one.Activity.Sessions == 1, "--session: activity.sessions = %d, want 1", one.Activity.Sessions)
	expectf(t, one.Total.Requests >= 1 && one.Activity.Steps == one.Total.Requests, "--session: activity.steps = %d, total.requests = %d, want them equal", one.Activity.Steps, one.Total.Requests)

	all := s.mustReportDoc()
	expectf(t, all.Activity.Sessions == 2, "no filter: activity.sessions = %d, want 2", all.Activity.Sessions)
	expectf(t, all.Total.Requests > one.Total.Requests && all.Activity.Steps == all.Total.Requests, "no filter: activity.steps = %d, total.requests = %d, want them equal and more than the one session (%d)", all.Activity.Steps, all.Total.Requests, one.Total.Requests)

	if runtime.GOOS != "windows" { // P2: the local day is the UTC day only under TZ=UTC
		days1 := s.mustReportDoc("--days", "1")
		expectf(t, days1.Activity.HeatmapFrom != "" && days1.Activity.HeatmapFrom == days1.Activity.HeatmapTo,
			"--days 1: heatmapFrom = %q, heatmapTo = %q, want the same day", days1.Activity.HeatmapFrom, days1.Activity.HeatmapTo)
	}
}

var heatmapRows = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// richSession is the state of A2.6-4 to A2.6-6 together.
func richSession(t *testing.T, s *sandbox) string {
	t.Helper()
	id := execSession(t, s)
	at := s.anchor(id)
	s.appendEvents(id, toolStateEvents(at)...)
	s.appendEvents(id, undatedEvents(at)...)
	return id
}

func TestA2_6_9_Shape(t *testing.T) {
	pendingActivity(t)
	glyphs := map[rune]bool{'\u00b7': true, '\u2591': true, '\u2592': true, '\u2593': true, '\u2588': true}
	s := newSandbox(t)
	richSession(t, s)

	check := func(t *testing.T, name string, extra []string, wantUnicode bool) {
		s.Extra = extra
		r := s.reportText()
		if !expectf(t, r.Code == 0, "%s: usage report exited %d\nstderr: %s", name, r.Code, r.Stderr) {
			return
		}
		block, ok := activityBlock(r.Stdout)
		if !expectf(t, ok, "%s: no activity block in:\n%s", name, r.Stdout) {
			return
		}
		nonASCII, glyphCount := 0, 0
		for _, l := range block {
			for _, ru := range l {
				if ru >= 0x80 {
					if glyphs[ru] {
						glyphCount++
					} else {
						nonASCII++
					}
				}
			}
		}
		expectf(t, nonASCII == 0, "%s: the block has runes that are neither ASCII nor one of the five glyphs", name)
		if wantUnicode {
			expectf(t, glyphCount > 0, "%s: the block has no glyph U+00B7/U+2591/U+2592/U+2593/U+2588", name)
		} else {
			expectf(t, glyphCount == 0, "%s: the block has non-ASCII glyphs where ASCII is required", name)
		}
		for _, l := range textLines(r.Stdout) {
			expectf(t, utf8.RuneCountInString(l) <= 80, "%s: line has %d runes (max 80): %q", name, utf8.RuneCountInString(l), l)
			expectf(t, !strings.HasSuffix(l, " "), "%s: line ends with a space: %q", name, l)
		}
		for _, day := range heatmapRows {
			n := 0
			for _, l := range textLines(r.Stdout) {
				if l == "  "+day || strings.HasPrefix(l, "  "+day+" ") {
					n++
				}
			}
			expectf(t, n == 1, "%s: %d lines of the heatmap row %s, want exactly 1", name, n, day)
		}
		less := 0
		for _, l := range textLines(r.Stdout) {
			if strings.HasPrefix(l, "      less ") {
				less++
			}
		}
		expectf(t, less == 1, "%s: %d legend lines starting with \"      less \", want 1", name, less)
	}
	t.Run("no-locale", func(t *testing.T) { check(t, "no LANG", nil, false) })
	t.Run("LC_ALL=C", func(t *testing.T) { check(t, "LC_ALL=C", []string{"LC_ALL=C"}, false) })
	t.Run("LC_ALL=C.UTF-8", func(t *testing.T) { check(t, "LC_ALL=C.UTF-8", []string{"LC_ALL=C.UTF-8"}, true) })
}

func TestA2_6_10_NoLeakFromEvents(t *testing.T) {
	pendingActivity(t)
	s := newSandbox(t)
	// A session with a model attribution, so the part of the report that does
	// carry " -> " and "resolved models" exists and the block has to stay apart.
	id, r, _ := execChat(t, s, chatRun{literal: `"deepseek/deepseek-r1:free"`, everyChunk: true})
	if r.Code != 0 {
		setupFatal(t, "zero exec exited %d\nstderr: %s", r.Code, r.Stderr)
	}
	at := s.anchor(id)
	ts := stamp(at)
	// Text read from events: control and format characters, the marks of 2.1.
	s.appendEvents(id,
		rawEvent{"tool_result", ts, `{"status":"FAILED\u001b[31m\u202e"}`},
		rawEvent{"tool_result", ts, `{"status":"x -> y resolved models \u0085\u200b","output":"\u001b[31m -> resolved models"}`},
		rawEvent{"message", ts, `{"role":"assistant","content":"a -> b resolved models \u001b[31m \u202e"}`},
		rawEvent{"message", ts, `{"role":"\u202e user\u001b[31m","content":"x"}`},
	)

	text := s.reportText()
	if !expectf(t, text.Code == 0, "usage report exited %d\nstderr: %s", text.Code, text.Stderr) {
		return
	}
	block, ok := activityBlock(text.Stdout)
	if !expectf(t, ok, "no activity block in:\n%s", text.Stdout) {
		return
	}
	before := strings.TrimSuffix(text.Stdout, strings.Join(block, "\n")+"\n")
	expectf(t, strings.Contains(before, "resolved models") && strings.Contains(before, " -> "),
		"precondition: the report of today must show the attribution before the block (feature of task 2.1):\n%s", text.Stdout)

	joined := strings.Join(block, "\n")
	expectf(t, !strings.Contains(joined, " -> ") && !strings.Contains(joined, "resolved models"), "the activity block has an attribution mark:\n%s", joined)
	for _, l := range block {
		for _, ru := range l {
			if ru < 0x20 || (ru >= 0x7f && ru <= 0x9f) || isFormatRune(ru) {
				expectf(t, false, "the activity block has the control or format rune U+%04X in %q", ru, l)
			}
		}
	}

	doc := s.mustReportDoc()
	raw := string(doc.ActivityRaw)
	expectf(t, raw != "" && !strings.Contains(raw, " -> ") && !strings.Contains(raw, "resolved models"), "the \"activity\" JSON has an attribution mark or is missing: %s", raw)
	for _, bad := range []string{`\u001b`, `\u202e`, `\u0085`, `\u200b`, "\x1b", "\u202e", "\u0085", "\u200b"} {
		expectf(t, !strings.Contains(raw, bad), "the \"activity\" JSON has the text read from an event (%q): %s", bad, raw)
	}
	// The events above are still counted: only their text is not shown.
	expectf(t, doc.Activity.Tools.Results == 2 && doc.Activity.Tools.Error == 2, "activity.tools = %+v, want 2 results, both errors", doc.Activity.Tools)
}

func isFormatRune(r rune) bool {
	switch {
	case r >= 0x200b && r <= 0x200f, r >= 0x2028 && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0xfeff, r == 0x00ad:
		return true
	}
	return false
}
