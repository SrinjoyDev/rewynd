// Package detect holds rewynd's deterministic per-request detectors. They run over one
// request's correlated data only, so a detection is as trustworthy as the correlation.
package detect

import (
	"fmt"
	"sort"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

// DefaultNPlusOneThreshold is the minimum count of identical normalized statements within
// one request before we call it an N+1. Conservative on purpose: never fire on a single query.
const DefaultNPlusOneThreshold = 5

// NPlusOne flags groups of identical normalized statements inside one request.
func NPlusOne(reqID string, queries []model.Query, threshold int) []model.Detection {
	if threshold <= 0 {
		threshold = DefaultNPlusOneThreshold
	}
	type group struct {
		count int
		total float64
	}
	groups := map[string]*group{}
	order := []string{}
	for _, q := range queries {
		key := q.StatementNormalized
		if key == "" {
			key = q.Statement
		}
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &group{}
			groups[key] = g
			order = append(order, key)
		}
		g.count++
		g.total += q.DurationMs
	}

	var out []model.Detection
	for _, key := range order {
		g := groups[key]
		if g.count < threshold {
			continue
		}
		out = append(out, model.Detection{
			RequestID: reqID,
			Type:      model.DetectNPlusOne,
			Severity:  "high",
			Title:     fmt.Sprintf("N+1 query — %d identical statements", g.count),
			Summary: fmt.Sprintf(
				"%d runs of %q totalling %.1fms in one request — likely a loop issuing one query per row.",
				g.count, truncate(key, 80), g.total,
			),
			Evidence: map[string]any{
				"statement_normalized": key,
				"count":                g.count,
				"total_ms":             g.total,
			},
			Suggestion: "Batch into a single query (WHERE id IN (...)) or use a join / eager-load.",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Evidence["count"].(int) > out[j].Evidence["count"].(int)
	})
	return out
}

const (
	DefaultSlowQueryMs   = 100
	DefaultSlowRequestMs = 1000
)

// SlowQueries flags individual queries over the threshold.
func SlowQueries(reqID string, queries []model.Query, thresholdMs float64) []model.Detection {
	if thresholdMs <= 0 {
		thresholdMs = DefaultSlowQueryMs
	}
	var out []model.Detection
	for _, q := range queries {
		if q.DurationMs < thresholdMs {
			continue
		}
		out = append(out, model.Detection{
			RequestID: reqID, Type: model.DetectSlowQuery, Severity: "warn",
			Title:      fmt.Sprintf("Slow query — %.0fms", q.DurationMs),
			Summary:    fmt.Sprintf("%.0fms: %s", q.DurationMs, truncate(q.Statement, 80)),
			Evidence:   map[string]any{"duration_ms": q.DurationMs, "statement": q.Statement},
			Suggestion: "Add an index, narrow the result set, or cache.",
		})
	}
	return out
}

// SlowRequest flags the request itself when it exceeds the threshold.
func SlowRequest(reqID string, durationMs, thresholdMs float64) []model.Detection {
	if thresholdMs <= 0 {
		thresholdMs = DefaultSlowRequestMs
	}
	if durationMs < thresholdMs {
		return nil
	}
	return []model.Detection{{
		RequestID: reqID, Type: model.DetectSlowRequest, Severity: "warn",
		Title:    fmt.Sprintf("Slow request — %.0fms", durationMs),
		Summary:  fmt.Sprintf("This request took %.0fms end to end.", durationMs),
		Evidence: map[string]any{"duration_ms": durationMs},
	}}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
