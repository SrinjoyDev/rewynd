package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

var (
	cGreen  = lipgloss.Color("#a6e3a1")
	cYellow = lipgloss.Color("#f9e2af")
	cRed    = lipgloss.Color("#f38ba8")
	cBlue   = lipgloss.Color("#89b4fa")
	cMauve  = lipgloss.Color("#cba6f7")
	cSub    = lipgloss.Color("#9399b2")
	cText   = lipgloss.Color("#cdd6f4")
	cBase   = lipgloss.Color("#1e1e2e")

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(cBase).Background(cMauve)
	footerStyle = lipgloss.NewStyle().Foreground(cSub)
	headStyle   = lipgloss.NewStyle().Foreground(cSub).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(cSub)
)

func (a app) View() string {
	if a.width == 0 || a.height == 0 {
		return "starting rewynd…"
	}
	bodyH := a.height - 2
	if bodyH < 3 {
		bodyH = 3
	}
	listW := a.width * 2 / 5
	if listW < 40 {
		listW = 40
	}
	if listW > a.width-24 {
		listW = maxi(24, a.width-24)
	}
	detailW := a.width - listW - 1

	title := titleStyle.Width(a.width).Render(a.titleText())
	listBox := lipgloss.NewStyle().Width(listW).Height(bodyH).MaxHeight(bodyH).Render(a.renderList(listW, bodyH))
	sep := dimStyle.Render(strings.TrimRight(strings.Repeat("│\n", bodyH), "\n"))
	detailBox := lipgloss.NewStyle().Width(detailW).Height(bodyH).MaxHeight(bodyH).Render(a.renderDetail(detailW, bodyH))
	body := lipgloss.JoinHorizontal(lipgloss.Top, listBox, sep, detailBox)
	footer := footerStyle.Width(a.width).Render(" j/k move · f filter · e next error · c clear · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, title, body, footer)
}

func (a app) titleText() string {
	f := a.filter
	if f == "" {
		f = "all"
	}
	return fmt.Sprintf(" rewynd · %d requests · filter %s ", len(a.reqs), f)
}

func (a app) renderList(w, h int) string {
	if len(a.reqs) == 0 {
		return dimStyle.Render("\n  no requests yet — hit an endpoint")
	}
	start := 0
	if a.sel >= h {
		start = a.sel - h + 1
	}
	end := mini(start+h, len(a.reqs))
	maxDur := 1.0
	for _, r := range a.reqs {
		if r.DurationMs > maxDur {
			maxDur = r.DurationMs
		}
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(renderRow(a.reqs[i], i == a.sel, w, maxDur))
		b.WriteByte('\n')
	}
	return b.String()
}

func renderRow(r model.Request, selected bool, w int, maxDur float64) string {
	ind := "  "
	textStyle := lipgloss.NewStyle().Foreground(cSub)
	if selected {
		ind = lipgloss.NewStyle().Foreground(cMauve).Render("▌ ")
		textStyle = lipgloss.NewStyle().Foreground(cText).Bold(true)
	}
	dot := lipgloss.NewStyle().Foreground(statusColor(r.StatusCode)).Render("●")
	method := textStyle.Render(fmt.Sprintf("%-4s", r.Method))
	status := lipgloss.NewStyle().Foreground(statusColor(r.StatusCode)).Render(fmt.Sprintf("%3d", r.StatusCode))
	bar := miniBar(r.DurationMs, maxDur)
	durTxt := dimStyle.Render(fmt.Sprintf("%6s", durStr(r.DurationMs)))
	flags := rowFlags(r)

	used := 2 + 1 + 1 + 4 + 1 + 3 + 1 + lipgloss.Width(bar) + 1 + 6 + lipgloss.Width(flags)
	pathW := w - used
	if pathW < 6 {
		pathW = 6
	}
	path := textStyle.Render(fmt.Sprintf("%-*s", pathW, truncate(r.Path, pathW)))
	return ind + dot + " " + method + " " + path + " " + status + " " + bar + " " + durTxt + flags
}

func (a app) renderDetail(w, h int) string {
	if a.detail == nil {
		return dimStyle.Render("\n  select a request")
	}
	r := a.detail
	sc := statusColor(r.StatusCode)
	var lines []string
	lines = append(lines,
		lipgloss.NewStyle().Foreground(sc).Bold(true).Render(fmt.Sprintf(" %s %s", r.Method, truncate(r.Path, w-16)))+
			dimStyle.Render(fmt.Sprintf("  %d · %s", r.StatusCode, durStr(r.DurationMs))))
	lines = append(lines, dimStyle.Render(fmt.Sprintf(" trace %s", short(r.TraceID))))

	if len(r.Detections) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(cMauve).Bold(true).Render(" DETECTIONS"))
		for _, d := range r.Detections {
			lines = append(lines, " "+lipgloss.NewStyle().Foreground(cMauve).Render("! ")+truncate(d.Title, w-4))
		}
	}
	if wf := waterfall(r, w); len(wf) > 0 {
		lines = append(lines, "", headStyle.Render(" WATERFALL"))
		lines = append(lines, wf...)
	}
	if len(r.Logs) > 0 {
		lines = append(lines, "", headStyle.Render(fmt.Sprintf(" LOGS (%d)", len(r.Logs))))
		for _, l := range r.Logs {
			lines = append(lines, " "+logLevel(l.Level)+" "+truncate(oneLine(l.Message), w-9))
		}
	}
	if len(r.Exceptions) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(cRed).Bold(true).Render(fmt.Sprintf(" EXCEPTIONS (%d)", len(r.Exceptions))))
		for _, e := range dedupExc(r.Exceptions) {
			lines = append(lines, " "+lipgloss.NewStyle().Foreground(cRed).Render(truncate(oneLine(e.Message), w-2)))
		}
	}
	if r.Request != nil && len(r.Request.Headers) > 0 {
		lines = append(lines, "", headStyle.Render(" REQUEST HEADERS"))
		keys := make([]string, 0, len(r.Request.Headers))
		for k := range r.Request.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, " "+dimStyle.Render(k+": ")+truncate(r.Request.Headers[k], maxi(4, w-len(k)-4)))
		}
	}
	if r.Request != nil && r.Request.Body != "" {
		lines = append(lines, "", headStyle.Render(" REQUEST BODY"))
		lines = append(lines, " "+truncate(oneLine(r.Request.Body), w-2))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// waterfall renders each query as a positioned, duration-scaled bar — repeated identical
// queries stack into an obvious staircase (the N+1).
func waterfall(r *model.Request, w int) []string {
	if len(r.Queries) == 0 {
		return nil
	}
	labelW := 20
	if w < 50 {
		labelW = 12
	}
	barW := w - labelW - 11
	if barW < 8 {
		barW = 8
	}
	span := float64(r.EndedAt - r.StartedAt)
	if span <= 0 {
		span = 1
	}
	var out []string
	for i, q := range r.Queries {
		if i >= 14 {
			out = append(out, dimStyle.Render(fmt.Sprintf("   … +%d more queries", len(r.Queries)-i)))
			break
		}
		off := int(float64(q.StartedAt-r.StartedAt) / span * float64(barW))
		ln := int(q.DurationMs / (span / 1e6) * float64(barW)) // span is ns; /1e6 -> ms
		if ln < 1 {
			ln = 1
		}
		if off < 0 {
			off = 0
		}
		if off+ln > barW {
			ln = maxi(1, barW-off)
		}
		bar := strings.Repeat(" ", off) + lipgloss.NewStyle().Foreground(cBlue).Render(strings.Repeat("▇", ln))
		out = append(out, fmt.Sprintf(" %-*s %s %s", labelW, truncate(queryLabel(q.Statement), labelW), bar, dimStyle.Render(durStr(q.DurationMs))))
	}
	return out
}

func statusColor(c int) lipgloss.Color {
	switch {
	case c >= 500:
		return cRed
	case c >= 400:
		return cYellow
	case c >= 200:
		return cGreen
	default:
		return cSub
	}
}

func miniBar(d, max float64) string {
	const n = 6
	if max <= 0 {
		max = 1
	}
	f := int(d / max * n)
	if f > n {
		f = n
	}
	if f < 0 {
		f = 0
	}
	col := cGreen
	switch {
	case d > max*0.66:
		col = cRed
	case d > max*0.33:
		col = cYellow
	}
	return lipgloss.NewStyle().Foreground(col).Render(strings.Repeat("▇", f)) + dimStyle.Render(strings.Repeat("·", n-f))
}

func rowFlags(r model.Request) string {
	var s string
	if hasNPlusOne(r) {
		s += " " + lipgloss.NewStyle().Foreground(cMauve).Bold(true).Render("N+1")
	}
	if r.DurationMs >= 1000 {
		s += " " + lipgloss.NewStyle().Foreground(cYellow).Render("slow")
	}
	return s
}

func logLevel(level string) string {
	col := cSub
	switch level {
	case "error", "fatal":
		col = cRed
	case "warn":
		col = cYellow
	case "info":
		col = cGreen
	}
	if level == "" {
		level = "log"
	}
	return lipgloss.NewStyle().Foreground(col).Render(fmt.Sprintf("%-5s", level))
}

func hasNPlusOne(r model.Request) bool {
	for _, d := range r.Detections {
		if d.Type == model.DetectNPlusOne {
			return true
		}
	}
	return false
}

func dedupExc(exc []model.Exception) []model.Exception {
	seen := map[string]bool{}
	var out []model.Exception
	for _, e := range exc {
		if seen[e.Message] {
			continue
		}
		seen[e.Message] = true
		out = append(out, e)
	}
	return out
}

func queryLabel(sql string) string {
	fields := strings.Fields(sql)
	if len(fields) == 0 {
		return sql
	}
	verb := strings.ToUpper(fields[0])
	for i, f := range fields {
		u := strings.ToUpper(f)
		if u == "FROM" || u == "INTO" || u == "UPDATE" {
			if i+1 < len(fields) {
				return verb + " " + strings.Trim(fields[i+1], "\"`")
			}
		}
	}
	return verb
}

func durStr(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return fmt.Sprintf("%.0fms", ms)
}

func truncate(s string, n int) string {
	if n <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	return s[:maxi(0, n-1)] + "…"
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mini(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func nextFilter(f string) string {
	switch f {
	case "":
		return "5xx"
	case "5xx":
		return "4xx"
	case "4xx":
		return "2xx"
	default:
		return ""
	}
}
