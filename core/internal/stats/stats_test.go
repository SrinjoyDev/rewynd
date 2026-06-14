package stats

import (
	"testing"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

func TestComputeAggregates(t *testing.T) {
	reqs := []model.Request{
		{Method: "POST", Route: "/orders", StatusCode: 500, Error: true, DurationMs: 1200, StartedAt: 0, EndedAt: 1_200_000_000,
			Detections: []model.Detection{{Type: model.DetectNPlusOne}}},
		{Method: "POST", Route: "/orders", StatusCode: 201, DurationMs: 200, StartedAt: 1_000_000_000, EndedAt: 1_200_000_000},
		{Method: "GET", Route: "/users", StatusCode: 200, DurationMs: 20, StartedAt: 2_000_000_000, EndedAt: 2_020_000_000},
		{Method: "GET", Route: "/users", StatusCode: 200, DurationMs: 40, StartedAt: 3_000_000_000, EndedAt: 3_040_000_000},
	}
	s := Compute(reqs)

	if s.Total != 4 {
		t.Fatalf("total = %d, want 4", s.Total)
	}
	if s.Errors != 1 || s.ServerErrors != 1 {
		t.Errorf("errors = %d (5xx %d), want 1/1", s.Errors, s.ServerErrors)
	}
	if s.ErrorRate < 0.24 || s.ErrorRate > 0.26 {
		t.Errorf("error rate = %.3f, want ~0.25", s.ErrorRate)
	}
	if s.NPlusOne != 1 {
		t.Errorf("n+1 = %d, want 1", s.NPlusOne)
	}
	if s.Slow != 1 { // the 1200ms request
		t.Errorf("slow = %d, want 1", s.Slow)
	}
	if s.Latency.Max != 1200 {
		t.Errorf("max latency = %.0f, want 1200", s.Latency.Max)
	}
	// The worst endpoint (highest error rate) must sort first.
	if len(s.Endpoints) != 2 || s.Endpoints[0].Route != "/orders" {
		t.Fatalf("endpoints not ranked worst-first: %+v", s.Endpoints)
	}
	if !s.Endpoints[0].NPlusOne || s.Endpoints[0].ErrorRate != 0.5 {
		t.Errorf("/orders endpoint = %+v, want N+1 + 0.5 error rate", s.Endpoints[0])
	}
	if s.ReqPerSec <= 0 {
		t.Errorf("req/sec = %.3f, want > 0", s.ReqPerSec)
	}
}

func TestComputeEmpty(t *testing.T) {
	s := Compute(nil)
	if s.Total != 0 || len(s.Endpoints) != 0 || s.ReqPerSec != 0 {
		t.Errorf("empty stats not zero: %+v", s)
	}
}

func TestPercentilesNearestRank(t *testing.T) {
	p := percentiles([]float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100})
	if p.P50 != 60 || p.Max != 100 {
		t.Errorf("p50=%.0f max=%.0f, want 60/100", p.P50, p.Max)
	}
}
