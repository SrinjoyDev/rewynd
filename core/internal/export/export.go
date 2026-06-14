// Package export renders one recorded request as a self-contained, shareable HTML page — no
// server, no assets, no dependencies. Drop it in a PR, attach it to CI, send it to a teammate.
package export

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

// HTML returns a complete, standalone HTML document for the request.
func HTML(r *model.Request) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString("<title>" + esc(title(r)) + " · rewynd</title>")
	b.WriteString("<style>" + css + "</style></head><body><main>")

	writeHeader(&b, r)
	writeDetections(&b, r)
	writeExceptions(&b, r)
	writeWaterfall(&b, r)
	writeOutbound(&b, r)
	writeLogs(&b, r)
	writePayload(&b, "Request", r.Request)
	writePayload(&b, "Response", r.Response)

	b.WriteString(`<footer>recorded by <a href="https://github.com/SrinjoyDev/rewynd">rewynd</a> · a local flight recorder for your backend</footer>`)
	b.WriteString("</main></body></html>")
	return b.String()
}

func title(r *model.Request) string {
	if r.Kind == model.KindJob {
		return "JOB " + r.Method + " " + r.Path
	}
	return r.Method + " " + r.Path
}

func writeHeader(b *strings.Builder, r *model.Request) {
	cls := statusClass(r)
	b.WriteString(`<header class="` + cls + `">`)
	if r.Kind == model.KindJob {
		b.WriteString(`<span class="badge">JOB</span> `)
		b.WriteString(`<span class="method">` + esc(r.Method) + `</span> `)
		b.WriteString(`<span class="path">` + esc(r.Path) + `</span>`)
		b.WriteString(`<span class="status">` + jobOutcome(r) + `</span>`)
	} else {
		b.WriteString(`<span class="method">` + esc(r.Method) + `</span> `)
		b.WriteString(`<span class="path">` + esc(r.Path) + `</span>`)
		b.WriteString(fmt.Sprintf(`<span class="status">%d</span>`, r.StatusCode))
	}
	b.WriteString(`<span class="dur">` + esc(durStr(r.DurationMs)) + `</span></header>`)

	b.WriteString(`<div class="meta">`)
	metaItem(b, "trace", short(r.TraceID))
	if r.Service != "" {
		metaItem(b, "service", r.Service)
	}
	if svcs := distinctServices(r); len(svcs) > 1 {
		metaItem(b, "services", strings.Join(svcs, " → "))
	}
	metaItem(b, "queries", fmt.Sprintf("%d", r.Counts.Queries))
	metaItem(b, "outbound", fmt.Sprintf("%d", r.Counts.Outbound))
	metaItem(b, "logs", fmt.Sprintf("%d", r.Counts.Logs))
	b.WriteString(`</div>`)
}

func writeDetections(b *strings.Builder, r *model.Request) {
	if len(r.Detections) == 0 {
		return
	}
	b.WriteString(section("Detections"))
	for _, d := range r.Detections {
		b.WriteString(`<div class="detection"><div class="d-title">` + esc(d.Title) + `</div>`)
		if d.Suggestion != "" {
			b.WriteString(`<div class="d-sugg">` + esc(d.Suggestion) + `</div>`)
		}
		b.WriteString(`</div>`)
	}
}

func writeExceptions(b *strings.Builder, r *model.Request) {
	if len(r.Exceptions) == 0 {
		return
	}
	b.WriteString(section(fmt.Sprintf("Exceptions (%d)", len(r.Exceptions))))
	seen := map[string]bool{}
	for _, e := range r.Exceptions {
		if seen[e.Message] {
			continue
		}
		seen[e.Message] = true
		head := e.Message
		if e.Type != "" {
			head = e.Type + ": " + e.Message
		}
		b.WriteString(`<div class="exc"><div class="e-msg">` + esc(head) + `</div>`)
		if e.Stack != "" {
			b.WriteString(`<pre class="stack">` + esc(e.Stack) + `</pre>`)
		}
		b.WriteString(`</div>`)
	}
}

func writeWaterfall(b *strings.Builder, r *model.Request) {
	if len(r.Queries) == 0 {
		return
	}
	multi := len(distinctServices(r)) > 1
	b.WriteString(section(fmt.Sprintf("Queries (%d)", len(r.Queries))))
	span := float64(r.EndedAt - r.StartedAt)
	if span <= 0 {
		span = 1
	}
	b.WriteString(`<div class="waterfall">`)
	for _, q := range r.Queries {
		off := clamp(float64(q.StartedAt-r.StartedAt)/span*100, 0, 100)
		w := clamp(q.DurationMs/(span/1e6)*100, 0.6, 100-off)
		tag := ""
		if multi && q.Service != "" {
			tag = `<span class="svc">` + esc(q.Service) + `</span>`
		}
		b.WriteString(`<div class="wf-row"><div class="wf-stmt">` + esc(oneLine(q.Statement)) + tag + `</div>`)
		b.WriteString(`<div class="wf-track"><div class="wf-bar" style="left:` +
			fmt.Sprintf("%.2f%%;width:%.2f%%", off, w) + `"></div></div>`)
		b.WriteString(`<div class="wf-dur">` + esc(durStr(q.DurationMs)) + `</div></div>`)
	}
	b.WriteString(`</div>`)
}

func writeOutbound(b *strings.Builder, r *model.Request) {
	if len(r.Outbound) == 0 {
		return
	}
	b.WriteString(section(fmt.Sprintf("Outbound (%d)", len(r.Outbound))))
	b.WriteString(`<table class="rows">`)
	for _, o := range r.Outbound {
		b.WriteString(`<tr><td class="o-status ` + statusClassCode(o.StatusCode) + `">` + fmt.Sprintf("%d", o.StatusCode) +
			`</td><td class="o-method">` + esc(o.Method) + `</td><td class="o-url">` + esc(o.URL) +
			`</td><td class="o-dur">` + esc(durStr(o.DurationMs)) + `</td></tr>`)
	}
	b.WriteString(`</table>`)
}

func writeLogs(b *strings.Builder, r *model.Request) {
	if len(r.Logs) == 0 {
		return
	}
	b.WriteString(section(fmt.Sprintf("Logs (%d)", len(r.Logs))))
	b.WriteString(`<div class="logs">`)
	for _, l := range r.Logs {
		lvl := l.Level
		if lvl == "" {
			lvl = "log"
		}
		b.WriteString(`<div class="log"><span class="lvl lvl-` + esc(lvl) + `">` + esc(lvl) +
			`</span> ` + esc(oneLine(l.Message)) + `</div>`)
	}
	b.WriteString(`</div>`)
}

func writePayload(b *strings.Builder, label string, p *model.HTTPPayload) {
	if p == nil || (len(p.Headers) == 0 && p.Body == "") {
		return
	}
	b.WriteString(section(label))
	if len(p.Headers) > 0 {
		keys := make([]string, 0, len(p.Headers))
		for k := range p.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(`<table class="headers">`)
		for _, k := range keys {
			b.WriteString(`<tr><td class="h-key">` + esc(k) + `</td><td class="h-val">` + esc(p.Headers[k]) + `</td></tr>`)
		}
		b.WriteString(`</table>`)
	}
	if p.Body != "" {
		b.WriteString(`<pre class="body">` + esc(p.Body) + `</pre>`)
	}
}

func metaItem(b *strings.Builder, k, v string) {
	b.WriteString(`<span class="m-item"><span class="m-k">` + esc(k) + `</span> ` + esc(v) + `</span>`)
}

func section(name string) string { return `<h2>` + esc(name) + `</h2>` }

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

func statusClass(r *model.Request) string {
	if r.Kind == model.KindJob {
		if r.Error {
			return "s-err"
		}
		return "s-ok"
	}
	return statusClassCode(r.StatusCode)
}

func statusClassCode(c int) string {
	switch {
	case c >= 500:
		return "s-err"
	case c >= 400:
		return "s-warn"
	case c >= 200:
		return "s-ok"
	default:
		return "s-na"
	}
}

func jobOutcome(r *model.Request) string {
	if r.Error {
		return "fail"
	}
	return "ok"
}

func durStr(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2fs", ms/1000)
	}
	return fmt.Sprintf("%.0fms", ms)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func short(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func esc(s string) string { return html.EscapeString(s) }
