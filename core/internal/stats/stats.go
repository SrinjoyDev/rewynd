// Package stats aggregates a window of recorded flows into a load/performance summary:
// throughput, latency percentiles, error rate, and a per-endpoint breakdown. It answers
// "how is this performing, where does it hurt, is it getting better" for humans and agents.
package stats

import (
	"sort"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

type Percentiles struct {
	P50 float64 `json:"p50_ms"`
	P95 float64 `json:"p95_ms"`
	P99 float64 `json:"p99_ms"`
	Max float64 `json:"max_ms"`
}

// Endpoint is the per-route rollup, the unit a developer actually tunes.
type Endpoint struct {
	Method    string  `json:"method"`
	Route     string  `json:"route"`
	Count     int     `json:"count"`
	Errors    int     `json:"errors"`
	ErrorRate float64 `json:"error_rate"`
	P95Ms     float64 `json:"p95_ms"`
	MaxMs     float64 `json:"max_ms"`
	NPlusOne  bool    `json:"n_plus_one,omitempty"`

	durs []float64 // accumulated while aggregating; unexported, not serialized
}

type Stats struct {
	Total        int         `json:"total"`
	WindowMs     float64     `json:"window_ms"`
	ReqPerSec    float64     `json:"req_per_sec"`
	Errors       int         `json:"errors"`
	ErrorRate    float64     `json:"error_rate"`
	ServerErrors int         `json:"server_errors_5xx"`
	ClientErrors int         `json:"client_errors_4xx"`
	FailedJobs   int         `json:"failed_jobs"`
	NPlusOne     int         `json:"n_plus_one"`
	Slow         int         `json:"slow"`
	Latency      Percentiles `json:"latency"`
	Endpoints    []Endpoint  `json:"endpoints"`
}

const slowRequestMs = 1000

// Compute rolls a window of requests (newest-first, as the store returns them) into a summary.
func Compute(reqs []model.Request) Stats {
	s := Stats{Total: len(reqs)}
	if len(reqs) == 0 {
		return s
	}

	durs := make([]float64, 0, len(reqs))
	groups := map[string]*Endpoint{}
	order := []string{}
	var minStart, maxEnd int64

	for i := range reqs {
		r := &reqs[i]
		durs = append(durs, r.DurationMs)

		if minStart == 0 || r.StartedAt < minStart {
			minStart = r.StartedAt
		}
		if r.EndedAt > maxEnd {
			maxEnd = r.EndedAt
		}

		isErr := r.Error || r.StatusCode >= 500
		if isErr {
			s.Errors++
		}
		switch {
		case r.Kind == model.KindJob && r.Error:
			s.FailedJobs++
		case r.StatusCode >= 500:
			s.ServerErrors++
		case r.StatusCode >= 400:
			s.ClientErrors++
		}
		nPlusOne := hasNPlusOne(r)
		if nPlusOne {
			s.NPlusOne++
		}
		if r.DurationMs >= slowRequestMs {
			s.Slow++
		}

		key, route := endpointKey(r)
		g := groups[key]
		if g == nil {
			g = &Endpoint{Method: r.Method, Route: route}
			groups[key] = g
			order = append(order, key)
		}
		g.Count++
		if isErr {
			g.Errors++
		}
		if nPlusOne {
			g.NPlusOne = true
		}
		if r.DurationMs > g.MaxMs {
			g.MaxMs = r.DurationMs
		}
		g.durs = append(g.durs, r.DurationMs)
	}

	s.Latency = percentiles(durs)
	s.ErrorRate = ratio(s.Errors, s.Total)
	if w := float64(maxEnd-minStart) / 1e6; w > 0 {
		s.WindowMs = w
		s.ReqPerSec = float64(s.Total) / (w / 1000)
	}

	for _, key := range order {
		g := groups[key]
		g.ErrorRate = ratio(g.Errors, g.Count)
		g.P95Ms = percentiles(g.durs).P95
		s.Endpoints = append(s.Endpoints, *g)
	}
	// Worst first: the endpoints a developer should look at are the slow, erroring ones.
	sort.SliceStable(s.Endpoints, func(i, j int) bool {
		a, b := s.Endpoints[i], s.Endpoints[j]
		if a.ErrorRate != b.ErrorRate {
			return a.ErrorRate > b.ErrorRate
		}
		return a.P95Ms > b.P95Ms
	})
	return s
}

func endpointKey(r *model.Request) (key, route string) {
	route = r.Route
	if route == "" {
		route = r.Path
	}
	return r.Method + " " + route, route
}

func percentiles(durs []float64) Percentiles {
	if len(durs) == 0 {
		return Percentiles{}
	}
	s := append([]float64(nil), durs...)
	sort.Float64s(s)
	return Percentiles{
		P50: pick(s, 0.50),
		P95: pick(s, 0.95),
		P99: pick(s, 0.99),
		Max: s[len(s)-1],
	}
}

// pick returns the nearest-rank percentile from a sorted slice.
func pick(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(q * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func ratio(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total)
}

func hasNPlusOne(r *model.Request) bool {
	for _, d := range r.Detections {
		if d.Type == model.DetectNPlusOne {
			return true
		}
	}
	return false
}
