package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Twigpine/zero/internal/usage"
)

type usageReportWithActivity struct {
	usage.Report
	Activity usage.Activity `json:"activity"`
}

func usageActivityUnicode(getenv func(string) string) bool {
	if getenv == nil {
		return false
	}
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := strings.TrimSpace(getenv(key))
		if value != "" {
			value = strings.ToLower(value)
			return strings.Contains(value, "utf-8") || strings.Contains(value, "utf8")
		}
	}
	return strings.TrimSpace(getenv("WT_SESSION")) != ""
}

func formatUsageActivity(activity usage.Activity, unicode bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "activity (local days, UTC%s)\n", activity.UTCOffset)
	if activity.Prompts+activity.Steps+activity.Tools.Results == 0 {
		b.WriteString("  no activity in the selected range")
		return b.String()
	}
	fmt.Fprintf(&b, "  sessions %d, prompts %d, steps %d, tokens %s\n", activity.Sessions, activity.Prompts, activity.Steps, groupThousands(activity.TotalTokens))
	success := "n/a"
	if activity.Tools.SuccessPerMille != nil {
		success = fmt.Sprintf("%d.%d%%", *activity.Tools.SuccessPerMille/10, *activity.Tools.SuccessPerMille%10)
	}
	fmt.Fprintf(&b, "  tool results %d: ok %d, error %d, unknown %d, success %s\n", activity.Tools.Results, activity.Tools.OK, activity.Tools.Error, activity.Tools.Unknown, success)
	switch activity.LongestStreak.Days {
	case 0:
		fmt.Fprintf(&b, "  active days %d, longest streak 0 days\n", activity.ActiveDays)
	case 1:
		fmt.Fprintf(&b, "  active days %d, longest streak 1 day (%s)\n", activity.ActiveDays, activity.LongestStreak.From)
	default:
		fmt.Fprintf(&b, "  active days %d, longest streak %d days (%s to %s)\n", activity.ActiveDays, activity.LongestStreak.Days, activity.LongestStreak.From, activity.LongestStreak.To)
	}
	if activity.UndatedEvents+activity.UnreadablePayloads > 0 {
		fmt.Fprintf(&b, "  undated events %d, unreadable payloads %d\n", activity.UndatedEvents, activity.UnreadablePayloads)
	}
	b.WriteString("\n")
	b.WriteString(formatHeatmap(activity, unicode))
	return b.String()
}

func formatHeatmap(activity usage.Activity, unicode bool) string {
	from, err1 := time.Parse("2006-01-02", activity.HeatmapFrom)
	to, err2 := time.Parse("2006-01-02", activity.HeatmapTo)
	if err1 != nil || err2 != nil || from.After(to) {
		return "      less . : + * # more"
	}
	from = civilDate(from)
	to = civilDate(to)
	start := from.AddDate(0, 0, mondayOffset(from)*-1)
	end := to.AddDate(0, 0, mondayOffset(to)*-1)
	cols := civilDays(start, end)/7 + 1
	if cols > 53 {
		cols = 53
	}
	counts := make(map[string]int, len(activity.Days))
	max := 0
	for _, day := range activity.Days {
		counts[day.Date] = day.Events
		date, err := time.Parse("2006-01-02", day.Date)
		if err == nil && !date.Before(from) && !date.After(to) && day.Events > max {
			max = day.Events
		}
	}
	monthLine := make([]rune, 8+cols)
	for i := range monthLine {
		monthLine[i] = ' '
	}
	type monthMark struct {
		col  int
		text string
	}
	var marks []monthMark
	previous := ""
	for col := 0; col < cols; col++ {
		date := start.AddDate(0, 0, col*7)
		if date.Before(from) {
			date = from
		}
		month := date.Format("Jan")
		if col == 0 || month != previous {
			marks = append(marks, monthMark{col: col, text: month})
		}
		previous = month
	}
	nextStart := len(monthLine) + 4
	for i := len(marks) - 1; i >= 0; i-- {
		mark := marks[i]
		startCol := 6 + mark.col
		if startCol+4 > nextStart {
			continue
		}
		copy(monthLine[startCol:startCol+3], []rune(mark.text))
		nextStart = startCol
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(string(monthLine), " "))
	b.WriteByte('\n')
	rows := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	glyphs := []string{".", ":", "+", "*", "#"}
	if unicode {
		glyphs = []string{"\u00b7", "\u2591", "\u2592", "\u2593", "\u2588"}
	}
	for row, name := range rows {
		line := "  " + name + " "
		for col := 0; col < cols; col++ {
			date := start.AddDate(0, 0, col*7+row)
			if date.Before(from) || date.After(to) {
				line += " "
				continue
			}
			n := counts[date.Format("2006-01-02")]
			level := 0
			if n > 0 && max > 0 {
				level = (4*n + max - 1) / max
				if level > 4 {
					level = 4
				}
			}
			line += glyphs[level]
		}
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	if unicode {
		b.WriteString("      less · ░ ▒ ▓ █ more")
	} else {
		b.WriteString("      less . : + * # more")
	}
	return b.String()
}

func civilDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
func mondayOffset(t time.Time) int     { return (int(t.Weekday()) + 6) % 7 }
func civilDays(from, to time.Time) int { return int(to.Sub(from).Hours() / 24) }
