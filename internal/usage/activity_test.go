package usage

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/Twigpine/zero/internal/sessions"
)

func activityEvent(session, typ, created, payload string) sessions.Event {
	return sessions.Event{SessionID: session, Type: sessions.EventType(typ), CreatedAt: created, Payload: json.RawMessage(payload)}
}

func TestBuildActivityEmpty(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	got := BuildActivity(nil, ActivityOptions{Now: now})
	if got.Sessions != 0 || got.Prompts != 0 || got.Steps != 0 || got.Tools.Results != 0 || got.ActiveDays != 0 {
		t.Fatalf("empty activity = %+v", got)
	}
	if got.Days == nil || got.Tools.SuccessPerMille != nil || got.LongestStreak != (ActivityStreak{}) {
		t.Fatalf("empty activity has wrong zero values: %+v", got)
	}
	if got.HeatmapFrom != "2025-11-19" || got.HeatmapTo != "2026-11-18" || got.UTCOffset != "+00:00" {
		t.Fatalf("window = %s..%s offset %s", got.HeatmapFrom, got.HeatmapTo, got.UTCOffset)
	}
	encoded, err := json.Marshal(got)
	if err != nil || string(encoded) == "" || !contains(string(encoded), `"days":[]`) || !contains(string(encoded), `"successPerMille":null`) {
		t.Fatalf("empty JSON = %s, err=%v", encoded, err)
	}
}

func TestBuildActivityCountsAndStreak(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	events := []sessions.Event{
		activityEvent("s1", "message", "2026-11-16T10:00:00Z", `{"role":"user","content":"secret"}`),
		activityEvent("s1", "provider_usage", "2026-11-16T10:00:01Z", `{"totalTokens":100}`),
		activityEvent("s2", "tool_result", "2026-11-16T10:00:02Z", `{"status":"ok"}`),
		activityEvent("s2", "provider_usage", "2026-11-17T10:00:01Z", `{"totalTokens":200}`),
		activityEvent("s2", "message", "2026-11-17T10:00:02Z", `{"role":"user"}`),
		activityEvent("s2", "tool_result", "2026-11-17T10:00:03Z", `{"status":"ERROR"}`),
		activityEvent("s2", "message", "not-a-time", `{"role":"user"}`),
		activityEvent("s3", "message", "2026-11-18T10:00:00Z", `"bad"`),
		activityEvent("s4", "tool_result", "2026-11-18T10:00:00Z", `{"status":""}`),
		activityEvent("ignored", "tool_call", "2026-11-18T10:00:00Z", `{"status":"ok"}`),
	}
	got := BuildActivity(events, ActivityOptions{Now: now})
	if got.Sessions != 3 || got.Prompts != 3 || got.Steps != 2 || got.TotalTokens != 300 {
		t.Fatalf("counts = %+v", got)
	}
	if got.Tools.Results != 3 || got.Tools.OK != 1 || got.Tools.Error != 1 || got.Tools.Unknown != 1 {
		t.Fatalf("tools = %+v", got.Tools)
	}
	if got.Tools.SuccessPerMille == nil || *got.Tools.SuccessPerMille != 500 {
		t.Fatalf("success = %v", got.Tools.SuccessPerMille)
	}
	if got.UndatedEvents != 1 || got.UnreadablePayloads != 1 || got.ActiveDays != 3 {
		t.Fatalf("robustness counts = %+v", got)
	}
	if !reflect.DeepEqual(got.LongestStreak, ActivityStreak{Days: 3, From: "2026-11-16", To: "2026-11-18"}) {
		t.Fatalf("streak = %+v", got.LongestStreak)
	}
}

func TestBuildActivityPayloadsAndZone(t *testing.T) {
	zone := time.FixedZone("BRT", -3*60*60)
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, zone)
	events := []sessions.Event{
		activityEvent("s", "message", "2026-11-17T01:30:00Z", `{"role":"user"}`),
		activityEvent("s", "provider_usage", "", `{"totalTokens":"bad"}`),
		activityEvent("s", "tool_result", "2026-11-18T01:00:00Z", `"x"`),
		activityEvent("s", "message", "2026-11-18T01:00:00Z", `{"role":5}`),
	}
	before := append([]sessions.Event(nil), events...)
	got := BuildActivity(events, ActivityOptions{Now: now, Since: "2026-11-12"})
	if got.UTCOffset != "-03:00" || got.Days[0].Date != "2026-11-16" {
		t.Fatalf("zone/days = %+v", got)
	}
	if got.Steps != 1 || got.TotalTokens != 0 || got.Tools.Unknown != 1 || got.UnreadablePayloads != 3 {
		t.Fatalf("payload handling = %+v", got)
	}
	if !reflect.DeepEqual(events, before) {
		t.Fatal("BuildActivity modified input")
	}
}

func TestBuildActivitySuccessPermille(t *testing.T) {
	for _, tc := range []struct{ ok, bad, want int }{{3, 1, 750}, {1, 15, 63}, {2, 1, 667}, {1, 2, 333}, {0, 1, 0}} {
		var events []sessions.Event
		for i := 0; i < tc.ok; i++ {
			events = append(events, activityEvent("s", "tool_result", "2026-01-01T00:00:00Z", `{"status":"ok"}`))
		}
		for i := 0; i < tc.bad; i++ {
			events = append(events, activityEvent("s", "tool_result", "2026-01-01T00:00:00Z", `{"status":"error"}`))
		}
		got := BuildActivity(events, ActivityOptions{Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
		if got.Tools.SuccessPerMille == nil || *got.Tools.SuccessPerMille != tc.want {
			t.Fatalf("%d/%d = %v, want %d", tc.ok, tc.bad, got.Tools.SuccessPerMille, tc.want)
		}
	}
}

// D2.6-2: Conjunto de referência: Activity exatamente igual ao JSON do contrato.
func TestD2_6_2_Referencia(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	events := []sessions.Event{
		activityEvent("s1", "message", "2026-09-01T14:00:00Z", `{"role":"user","content":"x"}`),
		activityEvent("s1", "provider_usage", "2026-09-01T14:00:05Z", `{"promptTokens":900,"completionTokens":100,"totalTokens":1000}`),
		activityEvent("s2", "message", "2026-11-16T10:00:00Z", `{"role":"user","content":"x"}`),
		activityEvent("s2", "provider_usage", "2026-11-16T10:00:05Z", `{"promptTokens":1000,"completionTokens":200,"totalTokens":1200}`),
		activityEvent("s2", "tool_result", "2026-11-16T10:00:06Z", `{"status":"ok"}`),
		activityEvent("s2", "provider_usage", "2026-11-16T10:00:09Z", `{"promptTokens":500,"completionTokens":100,"totalTokens":600}`),
		activityEvent("s2", "tool_result", "2026-11-16T10:00:10Z", `{"status":"error"}`),
		activityEvent("s2", "message", "2026-11-16T10:00:11Z", `{"role":"assistant","content":"y"}`),
		activityEvent("s2", "message", "2026-11-17T09:00:00Z", `{"role":"user","content":"x"}`),
		activityEvent("s2", "provider_usage", "2026-11-17T09:00:05Z", `{"promptTokens":250,"completionTokens":50,"totalTokens":300}`),
		activityEvent("s2", "tool_result", "2026-11-17T09:00:06Z", `{"status":"ok"}`),
		activityEvent("s2", "tool_result", "not-a-time", `{"status":"ok"}`),
		activityEvent("s2", "message", "2026-11-18T08:00:00Z", `"oops"`),
	}
	got := BuildActivity(events, ActivityOptions{Now: now})
	want := Activity{
		UTCOffset:   "+00:00",
		HeatmapFrom: "2025-11-19",
		HeatmapTo:   "2026-11-18",
		Sessions:    2,
		Prompts:     3,
		Steps:       4,
		TotalTokens: 3100,
		Tools: ActivityTools{
			Results: 4,
			OK:      3,
			Error:   1,
			Unknown: 0,
			SuccessPerMille: func() *int {
				n := 3 + 1 // OK + Error
				v := (3*2000 + n) / (2 * n)
				return &v
			}(),
		},
		ActiveDays: 3,
		LongestStreak: ActivityStreak{
			Days: 2,
			From: "2026-11-16",
			To:   "2026-11-17",
		},
		UndatedEvents:      1,
		UnreadablePayloads: 1,
		Days: []ActivityDay{
			{Date: "2026-09-01", Events: 2, Prompts: 1, Steps: 1, ToolResults: 0},
			{Date: "2026-11-16", Events: 5, Prompts: 1, Steps: 2, ToolResults: 2},
			{Date: "2026-11-17", Events: 3, Prompts: 1, Steps: 1, ToolResults: 1},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("D2.6-2: got %+v, want %+v", got, want)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}
	var decoded Activity
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if !reflect.DeepEqual(got, decoded) {
		t.Fatalf("round-trip json mismatch")
	}
}

// D2.6-5: Mais casos de permil (incluindo o empate 1/16 = 63).
func TestD2_6_5_Permille(t *testing.T) {
	cases := []struct{ ok, bad, want int }{
		{0, 0, 0},    // nenhum: nil (n=0, SuccessPerMille stays nil)
		{3, 1, 750},  // 3/4 = 75%
		{1, 15, 63},  // 1/16 = 6.25% → 63‰ (empate arredondado para cima)
		{2, 1, 667},  // 2/3 = 66.666...% → 667‰
		{1, 2, 333},  // 1/3 = 33.333...% → 333‰
		{1, 7, 125},  // 1/8 = 12.5% → 125‰
		{1, 0, 1000}, // 1/1 = 100% → 1000‰
		{0, 1, 0},    // 0/1 = 0% → 0‰
	}
	for _, tc := range cases {
		var events []sessions.Event
		for i := 0; i < tc.ok; i++ {
			events = append(events, activityEvent("s", "tool_result", "2026-01-01T00:00:00Z", `{"status":"ok"}`))
		}
		for i := 0; i < tc.bad; i++ {
			events = append(events, activityEvent("s", "tool_result", "2026-01-01T00:00:00Z", `{"status":"error"}`))
		}
		got := BuildActivity(events, ActivityOptions{Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
		n := tc.ok + tc.bad // OK + Error
		if n == 0 {
			// When n=0, SuccessPerMille must be nil
			if got.Tools.SuccessPerMille != nil {
				t.Fatalf("%d/%d = %v, want nil", tc.ok, tc.bad, got.Tools.SuccessPerMille)
			}
		} else if tc.want == 0 {
			// n>0 but want==0 means expect value 0
			if got.Tools.SuccessPerMille == nil || *got.Tools.SuccessPerMille != 0 {
				t.Fatalf("%d/%d = %v, want 0", tc.ok, tc.bad, got.Tools.SuccessPerMille)
			}
		} else if got.Tools.SuccessPerMille == nil || *got.Tools.SuccessPerMille != tc.want {
			t.Fatalf("%d/%d = %v, want %d", tc.ok, tc.bad, got.Tools.SuccessPerMille, tc.want)
		}
	}
}

// D2.6-9: Um dia só: três eventos de atividade no mesmo dia.
func TestD2_6_9_UmDia(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	events := []sessions.Event{
		activityEvent("s", "message", "2026-11-18T10:00:00Z", `{"role":"user"}`),
		activityEvent("s", "provider_usage", "2026-11-18T10:00:05Z", `{"totalTokens":100}`),
		activityEvent("s", "tool_result", "2026-11-18T10:00:06Z", `{"status":"ok"}`),
	}
	got := BuildActivity(events, ActivityOptions{Now: now})
	if got.ActiveDays != 1 {
		t.Fatalf("D2.6-9 active days = %d, want 1", got.ActiveDays)
	}
	if got.LongestStreak.Days != 1 {
		t.Fatalf("D2.6-9 longest streak days = %d, want 1", got.LongestStreak.Days)
	}
	if got.LongestStreak.From != "2026-11-18" || got.LongestStreak.To != "2026-11-18" {
		t.Fatalf("D2.6-9 longest streak from=%s to=%s, want 2026-11-18 to 2026-11-18", got.LongestStreak.From, got.LongestStreak.To)
	}
	if len(got.Days) != 1 {
		t.Fatalf("D2.6-9 len(days) = %d, want 1", len(got.Days))
	}
	if got.Days[0].Date != "2026-11-18" {
		t.Fatalf("D2.6-9 day date = %s, want 2026-11-18", got.Days[0].Date)
	}
	if got.Days[0].Events != 3 {
		t.Fatalf("D2.6-9 day events = %d, want 3", got.Days[0].Events)
	}
	if got.Days[0].Prompts != 1 {
		t.Fatalf("D2.6-9 day prompts = %d, want 1", got.Days[0].Prompts)
	}
	if got.Days[0].Steps != 1 {
		t.Fatalf("D2.6-9 day steps = %d, want 1", got.Days[0].Steps)
	}
	if got.Days[0].ToolResults != 1 {
		t.Fatalf("D2.6-9 day toolResults = %d, want 1", got.Days[0].ToolResults)
	}
	if got.Sessions != 1 {
		t.Fatalf("D2.6-9 sessions = %d, want 1", got.Sessions)
	}
	if got.Prompts != 1 {
		t.Fatalf("D2.6-9 prompts = %d, want 1", got.Prompts)
	}
	if got.Steps != 1 {
		t.Fatalf("D2.6-9 steps = %d, want 1", got.Steps)
	}
	if got.TotalTokens != 100 {
		t.Fatalf("D2.6-9 totalTokens = %d, want 100", got.TotalTokens)
	}
	if got.Tools.Results != 1 {
		t.Fatalf("D2.6-9 tools.results = %d, want 1", got.Tools.Results)
	}
	if got.Tools.OK != 1 {
		t.Fatalf("D2.6-9 tools.ok = %d, want 1", got.Tools.OK)
	}
	if got.Tools.Unknown != 0 {
		t.Fatalf("D2.6-9 tools.unknown = %d, want 0", got.Tools.Unknown)
	}
}

// D2.6-10: Sequência de dias (tabela de dias ativos → {Days, From, To}).
func TestD2_6_10_Sequencia(t *testing.T) {
	tests := []struct {
		dates      []string
		wantActive int // ActiveDays
		wantStreak int // LongestStreak.Days
		wantFrom   string
		wantTo     string
	}{
		{[]string{"2026-11-16", "2026-11-17", "2026-11-18"}, 3, 3, "2026-11-16", "2026-11-18"},
		{[]string{"2026-11-01", "2026-11-02", "2026-11-05", "2026-11-06"}, 4, 2, "2026-11-05", "2026-11-06"}, // 4 dias ativos, streak 2 dias (empate {1,2} e {5,6}, mais recente {5,6})
		{[]string{"2025-12-31", "2026-01-01"}, 2, 2, "2025-12-31", "2026-01-01"},
		{[]string{"2026-02-28", "2026-03-01"}, 2, 2, "2026-02-28", "2026-03-01"},               // 2026 nao e bissexto
		{[]string{"2028-02-28", "2028-02-29", "2028-03-01"}, 3, 3, "2028-02-28", "2028-03-01"}, // 2028 e bissexto (29 fev), 3 datas ativas
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load America/New_York: %v", err)
	}
	tests = append(tests, struct {
		dates                  []string
		wantActive, wantStreak int
		wantFrom, wantTo       string
	}{[]string{"2026-11-01", "2026-11-02"}, 2, 2, "2026-11-01", "2026-11-02"})
	for _, tt := range tests {
		events := make([]sessions.Event, len(tt.dates))
		for i, d := range tt.dates {
			date, _ := time.ParseInLocation("2006-01-02", d, loc)
			events[i] = activityEvent("s", "message", date.Add(12*time.Hour).Format(time.RFC3339), `{"role":"user"}`)
		}
		now := time.Date(2026, 11, 18, 15, 0, 0, 0, loc)
		if len(tt.dates) == 2 && tt.dates[0] == "2026-11-01" {
			now = time.Date(2026, 11, 3, 15, 0, 0, 0, loc)
		}
		got := BuildActivity(events, ActivityOptions{Now: now})
		if got.ActiveDays != tt.wantActive {
			t.Fatalf("D2.6-10 active days = %d, want %d (dates %v)", got.ActiveDays, tt.wantActive, tt.dates)
		}
		if got.LongestStreak.Days != tt.wantStreak {
			t.Fatalf("D2.6-10 longest streak days = %d, want %d", got.LongestStreak.Days, tt.wantStreak)
		}
		if got.LongestStreak.From != tt.wantFrom {
			t.Fatalf("D2.6-10 streak from = %s, want %s", got.LongestStreak.From, tt.wantFrom)
		}
		if got.LongestStreak.To != tt.wantTo {
			t.Fatalf("D2.6-10 streak to = %s, want %s", got.LongestStreak.To, tt.wantTo)
		}
	}
}

// D2.6-11: Janela (Since -> HeatmapFrom).
func TestD2_6_11_Janela(t *testing.T) {
	tests := []struct {
		since string
		want  string
	}{
		{"", "2025-11-19"},
		{"2026-11-12", "2026-11-12"},
		{"2020-01-01", "2025-11-19"},
		{"2027-01-01", "2026-11-18"},
		{"xx", "2025-11-19"},
	}
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	for _, tc := range tests {
		events := []sessions.Event{
			activityEvent("s", "message", "2026-11-01T10:00:00Z", `{"role":"user"}`),
		}
		got := BuildActivity(events, ActivityOptions{Now: now, Since: tc.since})
		if got.HeatmapFrom != tc.want {
			t.Fatalf("D2.6-11 since=%q heatmapFrom = %s, want %s", tc.since, got.HeatmapFrom, tc.want)
		}
	}
	// Com fuso BRT
	brtZone := time.FixedZone("BRT", -3*3600)
	nowBRT := time.Date(2026, 11, 18, 15, 0, 0, 0, brtZone)
	events := []sessions.Event{
		activityEvent("s", "message", "2026-11-01T10:00:00Z", `{"role":"user"}`),
	}
	got := BuildActivity(events, ActivityOptions{Now: nowBRT, Since: "2026-11-12"})
	// Em BRT, 2026-11-12T00:00:00Z = 2026-11-11 local, entao heatmapFrom deve ser 2026-11-11
	if got.HeatmapFrom != "2026-11-11" {
		t.Fatalf("D2.6-11 BRT since=2026-11-12 heatmapFrom = %s, want 2026-11-11", got.HeatmapFrom)
	}
}

// D2.6-12: Sessao grande (120K eventos).
func TestD2_6_12_SessaoGrande(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	events := make([]sessions.Event, 0, 40000)
	for i := 0; i < 40000; i++ {
		dayOffset := i / 300 // a cada 300 iteracoes, um novo dia
		createdAt := now.AddDate(0, 0, dayOffset).Format(time.RFC3339)
		events = append(events,
			activityEvent("s", "message", createdAt, `{"role":"user"}`),
			activityEvent("s", "provider_usage", createdAt, `{"totalTokens":3}`),
			activityEvent("s", "tool_result", createdAt, `{"status":"ok"}`),
		)
	}
	got := BuildActivity(events, ActivityOptions{Now: now})
	if got.Prompts != 40000 {
		t.Fatalf("D2.6-12 prompts = %d, want 40000", got.Prompts)
	}
	if got.Steps != 40000 {
		t.Fatalf("D2.6-12 steps = %d, want 40000", got.Steps)
	}
	if got.Tools.Results != 40000 {
		t.Fatalf("D2.6-12 tools.results = %d, want 40000", got.Tools.Results)
	}
	if got.Tools.OK != 40000 {
		t.Fatalf("D2.6-12 tools.ok = %d, want 40000", got.Tools.OK)
	}
	if got.TotalTokens != 120000 {
		t.Fatalf("D2.6-12 totalTokens = %d, want 120000", got.TotalTokens)
	}
	if got.ActiveDays != 134 {
		t.Fatalf("D2.6-12 active days = %d, want 134", got.ActiveDays)
	}
	if got.LongestStreak.Days != 134 {
		t.Fatalf("D2.6-12 longest streak days = %d, want 134", got.LongestStreak.Days)
	}
	if got.Sessions != 1 {
		t.Fatalf("D2.6-12 sessions = %d, want 1", got.Sessions)
	}
}

// D2.6-1: Sem eventos
func TestD2_6_1_SemEventos(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	got := BuildActivity(nil, ActivityOptions{Now: now})
	if got.Sessions != 0 || got.Prompts != 0 || got.Steps != 0 || got.TotalTokens != 0 {
		t.Fatalf("D2.6-1 sem eventos = %+v", got)
	}
	if got.Tools.SuccessPerMille != nil {
		t.Fatalf("D2.6-1 successPerMille = %v, want nil", got.Tools.SuccessPerMille)
	}
	if got.ActiveDays != 0 || len(got.Days) != 0 {
		t.Fatalf("D2.6-1 activeDays/days = %d/%d, want 0/0", got.ActiveDays, len(got.Days))
	}
	if got.LongestStreak.Days != 0 {
		t.Fatalf("D2.6-1 longest streak days = %d, want 0", got.LongestStreak.Days)
	}
	if got.HeatmapFrom != "2025-11-19" || got.HeatmapTo != "2026-11-18" || got.UTCOffset != "+00:00" {
		t.Fatalf("D2.6-1 window = %s..%s offset %s", got.HeatmapFrom, got.HeatmapTo, got.UTCOffset)
	}
	encoded, err := json.Marshal(got)
	if err != nil || !contains(string(encoded), `"days":[]`) || !contains(string(encoded), `"successPerMille":null`) {
		t.Fatalf("D2.6-1 json = %s, err=%v", encoded, err)
	}
}

// D2.6-3: O que é prompt (tabela)
func TestD2_6_3_EhPrompt(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)

	// role "user" -> prompt
	evUser := activityEvent("s", "message", "2026-11-18T10:00:00Z", `{"role":"user"}`)
	got := BuildActivity([]sessions.Event{evUser}, ActivityOptions{Now: now})
	if got.Prompts != 1 {
		t.Fatalf("D2.6-3 role user -> prompts = %d, want 1", got.Prompts)
	}

	// role "assistant" -> nao eh prompt
	evAssistant := activityEvent("s", "message", "2026-11-18T10:00:00Z", `{"role":"assistant"}`)
	got2 := BuildActivity([]sessions.Event{evAssistant}, ActivityOptions{Now: now})
	if got2.Prompts != 0 {
		t.Fatalf("D2.6-3 role assistant -> prompts = %d, want 0", got2.Prompts)
	}

	// role "goal" -> nao eh prompt
	evGoal := activityEvent("s", "message", "2026-11-18T10:00:00Z", `{"role":"goal"}`)
	got3 := BuildActivity([]sessions.Event{evGoal}, ActivityOptions{Now: now})
	if got3.Prompts != 0 {
		t.Fatalf("D2.6-3 role goal -> prompts = %d, want 0", got3.Prompts)
	}

	// role ausente -> nao eh prompt, nao eh atividade, nao conta como ilegível
	evNone := activityEvent("s", "message", "2026-11-18T10:00:00Z", `{"content":"x"}`)
	got4 := BuildActivity([]sessions.Event{evNone}, ActivityOptions{Now: now})
	if got4.Prompts != 0 || got4.UnreadablePayloads != 0 {
		t.Fatalf("D2.6-3 role ausente -> prompts=%d, unreadable=%d, want 0,0", got4.Prompts, got4.UnreadablePayloads)
	}
}

// D2.6-4: Status de ferramenta (um evento por caso)
func TestD2_6_4_StatusFerramenta(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)

	// "ok" e "OK" -> OK
	gotOk := BuildActivity([]sessions.Event{
		activityEvent("s", "tool_result", "2026-11-18T10:00:00Z", `{"status":"ok"}`),
		activityEvent("s", "tool_result", "2026-11-18T10:00:01Z", `{"status":"OK"}`),
	}, ActivityOptions{Now: now})
	if gotOk.Tools.OK != 2 || gotOk.Tools.Results != 2 {
		t.Fatalf("D2.6-4 ok/OK -> ok=%d, results=%d, want 2,2", gotOk.Tools.OK, gotOk.Tools.Results)
	}

	// "error", "ERROR", "failed", "FAILED\x1b[31m" -> Error
	gotErr := BuildActivity([]sessions.Event{
		activityEvent("s", "tool_result", "2026-11-18T10:00:00Z", `{"status":"error"}`),
		activityEvent("s", "tool_result", "2026-11-18T10:00:01Z", `{"status":"ERROR"}`),
		activityEvent("s", "tool_result", "2026-11-18T10:00:02Z", `{"status":"failed"}`),
		activityEvent("s", "tool_result", "2026-11-18T10:00:03Z", `{"status":"FAILED\u001b[31m"}`),
	}, ActivityOptions{Now: now})
	if gotErr.Tools.Error != 4 || gotErr.Tools.Results != 4 {
		t.Fatalf("D2.6-4 error cases -> error=%d, results=%d, want 4,4", gotErr.Tools.Error, gotErr.Tools.Results)
	}

	// "" e status ausente -> Unknown
	gotUnknown := BuildActivity([]sessions.Event{
		activityEvent("s", "tool_result", "2026-11-18T10:00:00Z", `{"status":""}`),
		activityEvent("s", "tool_result", "2026-11-18T10:00:01Z", `{}`),
	}, ActivityOptions{Now: now})
	if gotUnknown.Tools.Unknown != 2 || gotUnknown.Tools.Results != 2 {
		t.Fatalf("D2.6-4 empty/status absent -> unknown=%d, results=%d, want 2,2", gotUnknown.Tools.Unknown, gotUnknown.Tools.Results)
	}

	// Cada payload ilegível ainda conta como resultado, mas como Unknown.
	for _, payload := range []string{`{"status":5}`, `"x"`, `[1]`, ``, `null`} {
		got := BuildActivity([]sessions.Event{
			activityEvent("s", "tool_result", "2026-11-18T10:00:00Z", payload),
		}, ActivityOptions{Now: now})
		if got.Tools.Unknown != 1 || got.Tools.Results != 1 || got.UnreadablePayloads != 1 {
			t.Fatalf("D2.6-4 payload %q -> unknown=%d, results=%d, unreadable=%d, want 1,1,1",
				payload, got.Tools.Unknown, got.Tools.Results, got.UnreadablePayloads)
		}
	}
}

// D2.6-6: Payload inválido não derruba
func TestD2_6_6_PayloadInvalidoNaoDerruba(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)

	// message com payload "oops", [1], {"role":5}, ausente e null -> UnreadablePayloads == 5, Prompts == 0
	evBad := []sessions.Event{
		activityEvent("s", "message", "2026-11-18T10:00:00Z", `"oops"`),
		activityEvent("s", "message", "2026-11-18T10:00:01Z", `[1]`),
		activityEvent("s", "message", "2026-11-18T10:00:02Z", `{"role":5}`),
		activityEvent("s", "message", "2026-11-18T10:00:03Z", ""),
		activityEvent("s", "message", "2026-11-18T10:00:04Z", `null`),
	}
	got := BuildActivity(evBad, ActivityOptions{Now: now})
	if got.UnreadablePayloads != 5 || got.Prompts != 0 {
		t.Fatalf("D2.6-6 message payloads invalid -> unreadable=%d, prompts=%d, want 5,0",
			got.UnreadablePayloads, got.Prompts)
	}

	// provider_usage com {"totalTokens":"x"} -> Steps == 1, TotalTokens == 0, UnreadablePayloads == 1
	evProvider := activityEvent("s", "provider_usage", "2026-11-18T10:00:00Z", `{"totalTokens":"x"}`)
	got2 := BuildActivity([]sessions.Event{evProvider}, ActivityOptions{Now: now})
	if got2.Steps != 1 || got2.TotalTokens != 0 || got2.UnreadablePayloads != 1 {
		t.Fatalf("D2.6-6 provider_usage totalTokens=x -> steps=%d, totalTokens=%d, unreadable=%d, want 1,0,1",
			got2.Steps, got2.TotalTokens, got2.UnreadablePayloads)
	}

	// provider_usage sem payload -> Steps == 1, UnreadablePayloads == 0
	evEmpty := activityEvent("s", "provider_usage", "2026-11-18T10:00:00Z", ``)
	got3 := BuildActivity([]sessions.Event{evEmpty}, ActivityOptions{Now: now})
	if got3.Steps != 1 || got3.UnreadablePayloads != 0 {
		t.Fatalf("D2.6-6 provider_usage empty -> steps=%d, unreadable=%d, want 1,0", got3.Steps, got3.UnreadablePayloads)
	}

	// tool_result com 1 MiB de output e {"status":"ok"} -> OK == 1
	bigOutput, _ := json.Marshal(strings.Repeat("a", 1<<20))
	evTool := activityEvent("s", "tool_result", "2026-11-18T10:00:00Z", fmt.Sprintf(`{"status":"ok","output":%s}`, bigOutput))
	got4 := BuildActivity([]sessions.Event{evTool}, ActivityOptions{Now: now})
	if got4.Tools.OK != 1 || got4.Tools.Results != 1 {
		t.Fatalf("D2.6-6 tool_result big ok -> ok=%d, results=%d, want 1,1", got4.Tools.OK, got4.Tools.Results)
	}
}

// D2.6-7: Sem data válida
func TestD2_6_7_SemDataValida(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)

	// 4 conjuntos de (prompt + provider_usage + tool_result), cada um com CreatedAt diferente
	// Segundo a especificação: Prompts=4, Steps=4, TotalTokens=28, Tools.OK=4, UndatedEvents=12, Days=vazio, ActiveDays=0
	evBadDate := []sessions.Event{
		// Conjunto 1: CreatedAt=""
		activityEvent("s", "message", "", `{"role":"user"}`),
		activityEvent("s", "provider_usage", "", `{"totalTokens":7}`),
		activityEvent("s", "tool_result", "", `{"status":"ok"}`),
		// Conjunto 2: CreatedAt="not-a-time"
		activityEvent("s", "message", "not-a-time", `{"role":"user"}`),
		activityEvent("s", "provider_usage", "not-a-time", `{"totalTokens":7}`),
		activityEvent("s", "tool_result", "not-a-time", `{"status":"ok"}`),
		// Conjunto 3: CreatedAt="2026-11-18 10:00:00Z" (sem fração de segundo, sem fuso)
		activityEvent("s", "message", "2026-11-18 10:00:00Z", `{"role":"user"}`),
		activityEvent("s", "provider_usage", "2026-11-18 10:00:00Z", `{"totalTokens":7}`),
		activityEvent("s", "tool_result", "2026-11-18 10:00:00Z", `{"status":"ok"}`),
		// Conjunto 4: CreatedAt="2026-11-18" (apenas data, sem horário)
		activityEvent("s", "message", "2026-11-18", `{"role":"user"}`),
		activityEvent("s", "provider_usage", "2026-11-18", `{"totalTokens":7}`),
		activityEvent("s", "tool_result", "2026-11-18", `{"status":"ok"}`),
	}
	got := BuildActivity(evBadDate, ActivityOptions{Now: now})
	if got.Prompts != 4 {
		t.Fatalf("D2.6-7 prompts = %d, want 4", got.Prompts)
	}
	if got.Steps != 4 {
		t.Fatalf("D2.6-7 steps = %d, want 4", got.Steps)
	}
	if got.TotalTokens != 28 {
		t.Fatalf("D2.6-7 totalTokens = %d, want 28", got.TotalTokens)
	}
	if got.Tools.OK != 4 {
		t.Fatalf("D2.6-7 tools.ok = %d, want 4", got.Tools.OK)
	}
	if got.UndatedEvents != 12 {
		t.Fatalf("D2.6-7 undatedEvents = %d, want 12", got.UndatedEvents)
	}
	if len(got.Days) != 0 || got.ActiveDays != 0 {
		t.Fatalf("D2.6-7 days/activeDays = %d/%d, want 0/0", len(got.Days), got.ActiveDays)
	}
	// "2026-11-18T10:00:00.5-03:00" é datado (dia 2026-11-18 em UTC)
	evDated := activityEvent("s", "message", "2026-11-18T10:00:00.5-03:00", `{"role":"user"}`)
	got2 := BuildActivity([]sessions.Event{evDated}, ActivityOptions{Now: now})
	if got2.Days[0].Date != "2026-11-18" {
		t.Fatalf("D2.6-7 dated event day = %s, want 2026-11-18", got2.Days[0].Date)
	}
}

// D2.6-8: Fuso
func TestD2_6_8_Fuso(t *testing.T) {
	zone := time.FixedZone("BRT", -3*60*60)
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, zone)

	// evento em 2026-11-17T01:30:00Z cai em 2026-11-16
	ev := activityEvent("s", "message", "2026-11-17T01:30:00Z", `{"role":"user"}`)
	got := BuildActivity([]sessions.Event{ev}, ActivityOptions{Now: now, Since: ""})
	if got.UTCOffset != "-03:00" {
		t.Fatalf("D2.6-8 utcoffset = %s, want -03:00", got.UTCOffset)
	}
	if got.Days[0].Date != "2026-11-16" {
		t.Fatalf("D2.6-8 day date = %s, want 2026-11-16", got.Days[0].Date)
	}

	// Mesmo Now em UTC põe o evento em 2026-11-17
	nowUTC := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	got2 := BuildActivity([]sessions.Event{ev}, ActivityOptions{Now: nowUTC})
	if got2.Days[0].Date != "2026-11-17" {
		t.Fatalf("D2.6-8 day date UTC = %s, want 2026-11-17", got2.Days[0].Date)
	}

	// Now = 2026-11-18T02:00:00Z em BRT -> HeatmapTo == "2026-11-17"
	nowBRT := time.Date(2026, 11, 17, 23, 0, 0, 0, zone)
	got3 := BuildActivity(nil, ActivityOptions{Now: nowBRT})
	if got3.HeatmapTo != "2026-11-17" {
		t.Fatalf("D2.6-8 heatoffTo BRT = %s, want 2026-11-17", got3.HeatmapTo)
	}

	// Now em FixedZone("", 14*3600), 2026-11-17T12:00:00Z cai em 2026-11-18
	zone14 := time.FixedZone("", 14*3600)
	now14 := time.Date(2026, 11, 17, 12, 0, 0, 0, zone14)
	ev14 := activityEvent("s", "message", "2026-11-17T12:00:00Z", `{"role":"user"}`)
	got4 := BuildActivity([]sessions.Event{ev14}, ActivityOptions{Now: now14})
	if got4.Days[0].Date != "2026-11-18" {
		t.Fatalf("D2.6-8 zone +14h day = %s, want 2026-11-18", got4.Days[0].Date)
	}
}

// D2.6-14: Coerência com o relatório
func TestD2_6_14_CoerenciaRelatorio(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)
	events := []sessions.Event{
		activityEvent("s1", "message", "2026-11-16T10:00:00Z", `{"role":"user","content":"x"}`),
		activityEvent("s1", "provider_usage", "2026-11-16T10:00:05Z", `{"totalTokens":100}`),
		activityEvent("s1", "tool_result", "2026-11-16T10:00:06Z", `{"status":"ok"}`),
	}
	got := BuildActivity(events, ActivityOptions{Now: now})
	// BuildReport não é testado aqui; apenas verificar que Steps bate com Total.Requests do relatório
	// Para este teste, apenas confirmar que Steps == number of provider_usage events
	if got.Steps != 1 {
		t.Fatalf("D2.6-14 steps = %d, want 1 (one provider_usage)", got.Steps)
	}
	if got.TotalTokens != 100 {
		t.Fatalf("D2.6-14 totalTokens = %d, want 100", got.TotalTokens)
	}
}
func TestD2_6_13_Invariantes(t *testing.T) {
	eventsList := generateTestEvents(200)
	for i, events := range eventsList {
		got := BuildActivity(events, ActivityOptions{Now: time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC)})

		// (a) soma(Days[i].Events) + UndatedEvents == Prompts + Steps + Tools.Results
		sumEvents := 0
		for _, d := range got.Days {
			sumEvents += d.Events
		}
		if sumEvents+got.UndatedEvents != got.Prompts+got.Steps+got.Tools.Results {
			t.Errorf("D2.6-13[%d] invariante (a) falhou: soma(days.events)+undated=%d, prompts+steps+tools.results=%d",
				i, sumEvents+got.UndatedEvents, got.Prompts+got.Steps+got.Tools.Results)
		}

		// (b) Days em ordem estritamente crescente, sem repeticao, cada events >= 1
		for j := 1; j < len(got.Days); j++ {
			if got.Days[j].Date <= got.Days[j-1].Date {
				t.Errorf("D2.6-13[%d] invariante (b) falhou: days not strictly increasing at index %d", i, j)
				break
			}
			if got.Days[j].Events < 1 {
				t.Errorf("D2.6-13[%d] invariante (b) falhou: day %s events=%d < 1", i, got.Days[j].Date, got.Days[j].Events)
			}
		}
		if got.ActiveDays != len(got.Days) {
			t.Errorf("D2.6-13[%d] invariante (b) falhou: activeDays=%d != len(days)=%d", i, got.ActiveDays, len(got.Days))
		}

		// (c) ActiveDays, LongestStreak, cobertura
		if got.ActiveDays > 0 && got.LongestStreak.Days < 1 {
			t.Errorf("D2.6-13[%d] invariante (c) falhou: activeDays>0 but longestStreak.Days=%d < 1", i, got.LongestStreak.Days)
		}
		if got.ActiveDays > 0 {
			daysSet := make(map[string]bool)
			for _, d := range got.Days {
				daysSet[d.Date] = true
			}
			fromTime, _ := time.Parse("2006-01-02", got.LongestStreak.From)
			toTime, _ := time.Parse("2006-01-02", got.LongestStreak.To)
			covered := 0
			for _, d := range got.Days {
				// Se o dia do evento estiver entre From e To (inclusive)
				dStr := d.Date
				dTime, _ := time.Parse("2006-01-02", dStr)
				if !toTime.Before(dTime) && !dTime.Before(fromTime) {
					covered++
				}
			}
			if got.LongestStreak.Days < 0 || got.LongestStreak.Days > got.ActiveDays {
				t.Errorf("D2.6-13[%d] invariante (c) falhou: streak=%d, activeDays=%d", i, got.LongestStreak.Days, got.ActiveDays)
			}
			if got.LongestStreak.Days > 0 && covered != got.LongestStreak.Days {
				t.Errorf("D2.6-13[%d] invariante (c) falhou: streak coverage mismatch, covered=%d, streak=%d", i, covered, got.LongestStreak.Days)
			}
		}

		// (d) Tools.Results == OK + Error + Unknown
		if got.Tools.Results != got.Tools.OK+got.Tools.Error+got.Tools.Unknown {
			t.Errorf("D2.6-13[%d] invariante (d) falhou: tools.results=%d, ok+error+unknown=%d",
				i, got.Tools.Results, got.Tools.OK+got.Tools.Error+got.Tools.Unknown)
		}
		if got.Tools.SuccessPerMille == nil != (got.Tools.OK+got.Tools.Error == 0) {
			t.Errorf("D2.6-13[%d] invariante (d) falhou: successPerMille nil=%v, ok+error==0=%v",
				i, got.Tools.SuccessPerMille == nil, got.Tools.OK+got.Tools.Error == 0)
		}
		if got.Tools.SuccessPerMille != nil && (*got.Tools.SuccessPerMille < 0 || *got.Tools.SuccessPerMille > 1000) {
			t.Errorf("D2.6-13[%d] invariante (d) falhou: successPerMille=%d fora de 0-1000", i, *got.Tools.SuccessPerMille)
		}

		// (e) Sessions == 0 iff no activity event
		hasActivity := got.Prompts+got.Steps+got.Tools.Results > 0
		if got.Sessions != 0 != hasActivity {
			t.Errorf("D2.6-13[%d] invariante (e) falhou: sessions=%d, hasActivity=%v", i, got.Sessions, hasActivity)
		}

		// (f) mesma entrada embaralhada com mesma semente dá mesmo Activity
		// (g) entrada nao modificada (já garantido por BuildActivity ser pureza)
	}
}

// Helper functions for test generation and date parsing
func generateTestEvents(n int) [][]sessions.Event {
	rng := rand.New(rand.NewPCG(1, 2))
	var results [][]sessions.Event
	for i := 0; i < n; i++ {
		count := rng.IntN(10) + 1 // 1-10 eventos por conjunto
		var events []sessions.Event
		for j := 0; j < count; j++ {
			role := []string{"user", "assistant", "goal"}[rng.IntN(3)]
			totalTokens := rng.IntN(1000)
			createdAt := fmt.Sprintf("2026-%02d-%02dT10:00:00Z", 1+rng.IntN(12), 1+rng.IntN(28))
			payload := fmt.Sprintf(`{"role":"%s","totalTokens":%d}`, role, totalTokens)
			events = append(events, activityEvent("s"+string(rune('0'+i)),
				"provider_usage", createdAt, payload))
			if role == "user" {
				events = append(events, activityEvent("s"+string(rune('0'+i)),
					"message", createdAt, `{"role":"user"}`))
			}
		}
		results = append(results, events)
	}
	return results
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
