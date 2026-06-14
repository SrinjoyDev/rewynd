package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/SrinjoyDev/rewynd/internal/model"
	"github.com/SrinjoyDev/rewynd/internal/stats"
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
	if a.help {
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, helpBox())
	}
	if a.showStats {
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, statsBox(a.reqs, a.width))
	}
	listW, detailW, bodyH := a.layout()

	title := titleStyle.Width(a.width).Render(a.titleText())
	listBox := lipgloss.NewStyle().Width(listW).Height(bodyH).MaxHeight(bodyH).Render(a.renderList(listW, bodyH))
	sep := dimStyle.Render(strings.TrimRight(strings.Repeat("│\n", bodyH), "\n"))
	detailBox := lipgloss.NewStyle().Width(detailW).Height(bodyH).MaxHeight(bodyH).Render(a.renderDetail(detailW, bodyH))
	body := lipgloss.JoinHorizontal(lipgloss.Top, listBox, sep, detailBox)
	footer := footerStyle.Width(a.width).MaxWidth(a.width).Render(a.footerText())
	return lipgloss.JoinVertical(lipgloss.Left, title, body, footer)
}

// layout splits the screen into the list column, the detail column, and the body height.
func (a app) layout() (listW, detailW, bodyH int) {
	bodyH = a.height - 2
	if bodyH < 3 {
		bodyH = 3
	}
	listW = a.width * 2 / 5
	if listW < 40 {
		listW = 40
	}
	if listW > a.width-24 {
		listW = maxi(24, a.width-24)
	}
	detailW = a.width - listW - 1
	return
}

func (a app) titleText() string {
	f := a.filter
	if f == "" {
		f = "all"
	}
	s := fmt.Sprintf(" rewynd · %d requests · %s", len(a.reqs), f)
	if a.slowOnly {
		s += " · slow"
	}
	if a.search != "" {
		s += " · /" + a.search
	}
	return s + " "
}

func (a app) footerText() string {
	if a.searching {
		return " search /" + a.search + "█  (enter keep · esc clear)"
	}
	s := " j/k move · / search · f status · s slow · S stats · e error · ^d/^u scroll · c clear · ? help · q quit"
	if lipgloss.Width(s) > a.width {
		s = truncate(s, a.width)
	}
	return s
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
	dot := lipgloss.NewStyle().Foreground(rowColor(r)).Render("●")
	method := textStyle.Render(fmt.Sprintf("%-4s", r.Method))
	status := lipgloss.NewStyle().Foreground(rowColor(r)).Render(statusCell(r))
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
	lines := a.detailLines(a.detail, w)
	if len(lines) <= h {
		return strings.Join(lines, "\n")
	}
	// Scrollable: reserve the bottom line for a position indicator.
	winH := h - 1
	sc := clampi(a.detailScroll, 0, detailWindowMax(len(lines), h))
	end := mini(sc+winH, len(lines))
	win := append([]string{}, lines[sc:end]...)
	arrows := " "
	if sc > 0 {
		arrows = lipgloss.NewStyle().Foreground(cMauve).Render("▲")
	}
	if end < len(lines) {
		arrows += lipgloss.NewStyle().Foreground(cMauve).Render("▼")
	} else {
		arrows += " "
	}
	win = append(win, dimStyle.Render(fmt.Sprintf(" %s  %d–%d / %d", arrows, sc+1, end, len(lines))))
	return strings.Join(win, "\n")
}

// detailWindowMax is the largest scroll offset, accounting for the reserved indicator line.
func detailWindowMax(total, h int) int {
	if total <= h {
		return 0
	}
	return total - (h - 1)
}

// detailLines builds the full (unscrolled) detail view for one request, in debugging order:
// what's wrong first (detections, exception), then how it ran (waterfall, outbound, logs),
// then the raw payloads.
func (a app) detailLines(r *model.Request, w int) []string {
	sc := rowColor(*r)
	var lines []string
	header := lipgloss.NewStyle().Foreground(sc).Bold(true).Render(fmt.Sprintf(" %s %s", r.Method, truncate(r.Path, w-16)))
	if r.Kind == model.KindJob {
		header = lipgloss.NewStyle().Foreground(cMauve).Bold(true).Render(" JOB ") +
			lipgloss.NewStyle().Foreground(sc).Bold(true).Render(fmt.Sprintf("%s %s", r.Method, truncate(r.Path, w-20))) +
			dimStyle.Render("  "+jobOutcome(r)+" · "+durStr(r.DurationMs))
	} else {
		header += dimStyle.Render(fmt.Sprintf("  %d · %s", r.StatusCode, durStr(r.DurationMs)))
	}
	lines = append(lines, header)
	lines = append(lines, dimStyle.Render(fmt.Sprintf(" trace %s · %dq %do %dl", short(r.TraceID), r.Counts.Queries, r.Counts.Outbound, r.Counts.Logs)))

	svcs := distinctServices(r)
	multi := len(svcs) > 1
	if multi {
		lines = append(lines, lipgloss.NewStyle().Foreground(cBlue).Render(" services ")+dimStyle.Render(truncate(strings.Join(svcs, " → "), w-11)))
	}

	if len(r.Detections) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(cMauve).Bold(true).Render(" DETECTIONS"))
		for _, d := range r.Detections {
			lines = append(lines, " "+lipgloss.NewStyle().Foreground(cMauve).Render("! ")+truncate(d.Title, w-4))
			if d.Suggestion != "" {
				lines = append(lines, "   "+dimStyle.Render(truncate(d.Suggestion, w-4)))
			}
		}
	}
	if len(r.Exceptions) > 0 {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(cRed).Bold(true).Render(fmt.Sprintf(" EXCEPTIONS (%d)", len(r.Exceptions))))
		for _, e := range dedupExc(r.Exceptions) {
			head := e.Message
			if e.Type != "" {
				head = e.Type + ": " + e.Message
			}
			lines = append(lines, " "+lipgloss.NewStyle().Foreground(cRed).Render(truncate(oneLine(head), w-2)))
		}
	}
	if wf := waterfall(r, w, multi); len(wf) > 0 {
		lines = append(lines, "", headStyle.Render(fmt.Sprintf(" WATERFALL (%d queries)", len(r.Queries))))
		lines = append(lines, wf...)
	}
	if len(r.Outbound) > 0 {
		lines = append(lines, "", headStyle.Render(fmt.Sprintf(" OUTBOUND (%d)", len(r.Outbound))))
		for _, o := range r.Outbound {
			status := lipgloss.NewStyle().Foreground(statusColor(o.StatusCode)).Render(fmt.Sprintf("%3d", o.StatusCode))
			line := " " + status + " " + fmt.Sprintf("%-4s", o.Method) + " " + truncate(o.URL, maxi(6, w-12)) + " " + dimStyle.Render(durStr(o.DurationMs))
			if multi && o.Service != "" {
				line += " " + svcTag(o.Service)
			}
			lines = append(lines, line)
		}
	}
	if len(r.Logs) > 0 {
		lines = append(lines, "", headStyle.Render(fmt.Sprintf(" LOGS (%d)", len(r.Logs))))
		for _, l := range r.Logs {
			lines = append(lines, " "+logLevel(l.Level)+" "+truncate(oneLine(l.Message), w-9))
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
		lines = append(lines, bodyLines(r.Request.Body, w)...)
	}
	if r.Response != nil && r.Response.Body != "" {
		lines = append(lines, "", headStyle.Render(" RESPONSE BODY"))
		lines = append(lines, bodyLines(r.Response.Body, w)...)
	}
	return lines
}

// bodyLines wraps a captured body across up to a few lines so longer JSON is readable.
func bodyLines(body string, w int) []string {
	s := oneLine(body)
	width := maxi(8, w-2)
	var out []string
	for len(s) > 0 && len(out) < 6 {
		n := mini(width, len(s))
		out = append(out, " "+s[:n])
		s = s[n:]
	}
	if len(s) > 0 {
		out = append(out, " "+dimStyle.Render("…"))
	}
	return out
}

// waterfall renders each query as a positioned, duration-scaled bar — repeated identical
// queries stack into an obvious staircase (the N+1). When the request spans services, each
// query is tagged with the one that issued it.
func waterfall(r *model.Request, w int, multi bool) []string {
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
		dur := durStr(q.DurationMs)
		if multi && q.Service != "" {
			dur += " " + svcTag(q.Service)
		}
		out = append(out, fmt.Sprintf(" %-*s %s %s", labelW, truncate(queryLabel(q.Statement), labelW), bar, dimStyle.Render(dur)))
	}
	return out
}

// distinctServices lists the services that appear in a request's spans, entry-first by start time.
func distinctServices(r *model.Request) []string {
	seen := map[string]bool{}
	var out []string
	for _, sp := range r.Spans {
		if sp.Service != "" && !seen[sp.Service] {
			seen[sp.Service] = true
			out = append(out, sp.Service)
		}
	}
	return out
}

func svcTag(s string) string {
	return lipgloss.NewStyle().Foreground(cBlue).Render("[" + s + "]")
}

// statsBox is the load/performance overlay (S): throughput, latency percentiles, error rate,
// and the worst endpoints — computed from the visible window.
func statsBox(reqs []model.Request, width int) string {
	s := stats.Compute(reqs)
	var b strings.Builder
	title := lipgloss.NewStyle().Foreground(cMauve).Bold(true)
	label := lipgloss.NewStyle().Foreground(cSub)
	b.WriteString(title.Render("rewynd · load"))
	if s.Total == 0 {
		b.WriteString("\n\n" + dimStyle.Render("  nothing recorded yet"))
		return boxFrame(b.String())
	}
	b.WriteString("\n\n")
	tput := fmt.Sprintf("%d flows", s.Total)
	if s.WindowMs > 0 {
		tput += fmt.Sprintf(" · %s · %.1f req/s", durStr(s.WindowMs), s.ReqPerSec)
	}
	b.WriteString("  " + lipgloss.NewStyle().Foreground(cText).Render(tput) + "\n\n")
	b.WriteString("  " + label.Render("latency  ") +
		fmt.Sprintf("p50 %-7s p95 %-7s p99 %-7s max %s\n", durStr(s.Latency.P50), durStr(s.Latency.P95), durStr(s.Latency.P99), durStr(s.Latency.Max)))
	errColor := cGreen
	if s.ErrorRate > 0 {
		errColor = cRed
	}
	b.WriteString("  " + label.Render("errors   ") +
		lipgloss.NewStyle().Foreground(errColor).Render(fmt.Sprintf("%.1f%% (%d)", s.ErrorRate*100, s.Errors)) +
		dimStyle.Render(fmt.Sprintf("   5xx %d · 4xx %d · failed jobs %d", s.ServerErrors, s.ClientErrors, s.FailedJobs)) + "\n")
	b.WriteString("  " + label.Render("issues   ") + dimStyle.Render(fmt.Sprintf("N+1 in %d · slow %d", s.NPlusOne, s.Slow)) + "\n")

	if len(s.Endpoints) > 0 {
		b.WriteString("\n  " + headStyle.Render("BY ENDPOINT (worst first)") + "\n")
		for i, e := range s.Endpoints {
			if i >= 8 {
				b.WriteString(dimStyle.Render(fmt.Sprintf("    … +%d more\n", len(s.Endpoints)-i)))
				break
			}
			ec := cGreen
			if e.ErrorRate > 0 {
				ec = cRed
			}
			flag := ""
			if e.NPlusOne {
				flag = lipgloss.NewStyle().Foreground(cMauve).Render(" N+1")
			}
			b.WriteString(fmt.Sprintf("    %s  %s  %s  %s\n",
				lipgloss.NewStyle().Foreground(ec).Render(fmt.Sprintf("%3.0f%%", e.ErrorRate*100)),
				dimStyle.Render(fmt.Sprintf("p95 %-7s", durStr(e.P95Ms))),
				truncate(e.Method+" "+e.Route, maxi(10, width/2)), flag))
		}
	}
	b.WriteString("\n" + dimStyle.Render("  any key to close"))
	return boxFrame(b.String())
}

func boxFrame(s string) string {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cMauve).Padding(1, 3).Render(s)
}

func helpBox() string {
	keys := [][2]string{
		{"j, down", "move down"},
		{"k, up", "move up"},
		{"g / G", "jump to top / bottom"},
		{"/", "search by path (live)"},
		{"f", "cycle status filter (2xx/4xx/5xx)"},
		{"s", "toggle slow-only"},
		{"e", "jump to the next error"},
		{"S", "load stats (latency, errors, by endpoint)"},
		{"^d / ^u", "scroll the detail pane"},
		{"esc", "clear search / filter"},
		{"c", "clear the buffer"},
		{"?", "toggle this help"},
		{"q", "quit"},
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(cMauve).Bold(true).Render("rewynd · keys"))
	b.WriteString("\n\n")
	for _, k := range keys {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(cText).Bold(true).Render(fmt.Sprintf("%-7s", k[0])))
		b.WriteString("  " + dimStyle.Render(k[1]) + "\n")
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cMauve).Padding(1, 4).Render(strings.TrimRight(b.String(), "\n"))
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

// rowColor / statusCell render both flows in the list: HTTP by status code, jobs by outcome.
func rowColor(r model.Request) lipgloss.Color {
	if r.Kind == model.KindJob {
		if r.Error {
			return cRed
		}
		return cMauve
	}
	return statusColor(r.StatusCode)
}

func statusCell(r model.Request) string {
	if r.Kind == model.KindJob {
		return "job"
	}
	return fmt.Sprintf("%3d", r.StatusCode)
}

func jobOutcome(r *model.Request) string {
	if r.Error {
		return "fail"
	}
	return "ok"
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
