package usage

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Twigpine/zero/internal/sessions"
)

const ActivityHeatmapDays = 365

type ActivityOptions struct {
	Now   time.Time
	Since string
}

type Activity struct {
	UTCOffset          string         `json:"utcOffset"`
	HeatmapFrom        string         `json:"heatmapFrom"`
	HeatmapTo          string         `json:"heatmapTo"`
	Sessions           int            `json:"sessions"`
	Prompts            int            `json:"prompts"`
	Steps              int            `json:"steps"`
	TotalTokens        int            `json:"totalTokens"`
	Tools              ActivityTools  `json:"tools"`
	ActiveDays         int            `json:"activeDays"`
	LongestStreak      ActivityStreak `json:"longestStreak"`
	UndatedEvents      int            `json:"undatedEvents"`
	UnreadablePayloads int            `json:"unreadablePayloads"`
	Days               []ActivityDay  `json:"days"`
}

type ActivityTools struct {
	Results         int  `json:"results"`
	OK              int  `json:"ok"`
	Error           int  `json:"error"`
	Unknown         int  `json:"unknown"`
	SuccessPerMille *int `json:"successPerMille"`
}

type ActivityStreak struct {
	Days int    `json:"days"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type ActivityDay struct {
	Date        string `json:"date"`
	Events      int    `json:"events"`
	Prompts     int    `json:"prompts"`
	Steps       int    `json:"steps"`
	ToolResults int    `json:"toolResults"`
}

func BuildActivity(events []sessions.Event, opts ActivityOptions) Activity {
	loc := opts.Now.Location()
	if loc == nil {
		loc = time.UTC
	}
	today := opts.Now.In(loc)
	to := today.Format("2006-01-02")
	fromTime := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(ActivityHeatmapDays - 1))
	if since, err := time.Parse("2006-01-02", opts.Since); err == nil {
		sinceLocal := since.In(loc).Format("2006-01-02")
		if sinceLocal > fromTime.Format("2006-01-02") {
			fromTime = time.Date(since.In(loc).Year(), since.In(loc).Month(), since.In(loc).Day(), 0, 0, 0, 0, loc)
			if fromTime.Format("2006-01-02") > to {
				fromTime = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
			}
		}
	}
	out := Activity{UTCOffset: opts.Now.Format("-07:00"), HeatmapFrom: fromTime.Format("2006-01-02"), HeatmapTo: to, Days: []ActivityDay{}}
	days := make(map[string]*ActivityDay)
	sessionsSeen := make(map[string]struct{})
	for _, event := range events {
		if event.Type != sessions.EventMessage && event.Type != sessions.EventProviderUsage && event.Type != sessions.EventToolResult {
			continue
		}
		day, dated := activityDay(event.CreatedAt, loc, days)
		activityEvent := false
		switch event.Type {
		case sessions.EventMessage:
			var payload struct {
				Role string `json:"role"`
			}
			if !validObjectPayload(event.Payload) || json.Unmarshal(event.Payload, &payload) != nil {
				out.UnreadablePayloads++
			} else if payload.Role == "user" {
				activityEvent = true
				out.Prompts++
				if dated {
					day.Prompts++
				}
			}
		case sessions.EventProviderUsage:
			activityEvent = true
			var payload struct {
				TotalTokens int `json:"totalTokens"`
			}
			if !isEmptyPayload(event.Payload) && json.Unmarshal(event.Payload, &payload) != nil {
				out.UnreadablePayloads++
			}
			out.Steps++
			if !isEmptyPayload(event.Payload) && json.Unmarshal(event.Payload, &payload) == nil {
				out.TotalTokens += payload.TotalTokens
			}
			if dated {
				day.Steps++
			}
		case sessions.EventToolResult:
			activityEvent = true
			out.Tools.Results++
			var payload struct {
				Status string `json:"status"`
			}
			if !validObjectPayload(event.Payload) || json.Unmarshal(event.Payload, &payload) != nil {
				out.UnreadablePayloads++
				out.Tools.Unknown++
			} else {
				switch strings.ToLower(payload.Status) {
				case "ok":
					out.Tools.OK++
				case "":
					out.Tools.Unknown++
				default:
					out.Tools.Error++
				}
			}
			if dated {
				day.ToolResults++
			}
		}
		if activityEvent {
			sessionsSeen[event.SessionID] = struct{}{}
			if !dated {
				out.UndatedEvents++
			}
			if dated {
				day.Events++
			}
		}
	}
	out.Sessions = len(sessionsSeen)
	for _, day := range days {
		if day.Events > 0 {
			out.Days = append(out.Days, *day)
		}
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Date < out.Days[j].Date })
	out.ActiveDays = len(out.Days)
	out.LongestStreak = activityStreak(out.Days)
	if n := out.Tools.OK + out.Tools.Error; n > 0 {
		value := (out.Tools.OK*2000 + n) / (2 * n)
		out.Tools.SuccessPerMille = &value
	}
	return out
}

func activityDay(created string, loc *time.Location, days map[string]*ActivityDay) (*ActivityDay, bool) {
	parsed, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return nil, false
	}
	date := parsed.In(loc).Format("2006-01-02")
	if day, ok := days[date]; ok {
		return day, true
	}
	day := &ActivityDay{Date: date}
	days[date] = day
	return day, true
}

func isEmptyPayload(payload []byte) bool {
	return len(bytes.TrimSpace(payload)) == 0 || bytes.Equal(bytes.TrimSpace(payload), []byte("null"))
}
func validObjectPayload(payload []byte) bool {
	trimmed := bytes.TrimSpace(payload)
	return len(trimmed) > 0 && bytes.Equal(trimmed[:1], []byte("{")) && !bytes.Equal(trimmed, []byte("null"))
}

func activityStreak(days []ActivityDay) ActivityStreak {
	if len(days) == 0 {
		return ActivityStreak{}
	}
	best := ActivityStreak{Days: 1, From: days[0].Date, To: days[0].Date}
	start := days[0].Date
	length := 1
	for i := 1; i <= len(days); i++ {
		contiguous := i < len(days) && civilNext(start, length) == days[i].Date
		if contiguous {
			length++
			continue
		}
		candidate := ActivityStreak{Days: length, From: start, To: days[i-1].Date}
		if candidate.Days > best.Days || (candidate.Days == best.Days && candidate.To > best.To) {
			best = candidate
		}
		if i < len(days) {
			start, length = days[i].Date, 1
		}
	}
	return best
}

func civilNext(start string, count int) string {
	date, _ := time.Parse("2006-01-02", start)
	return date.AddDate(0, 0, count).Format("2006-01-02")
}
