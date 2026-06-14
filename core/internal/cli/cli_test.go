package cli

import (
	"strings"
	"testing"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
	"github.com/SrinjoyDev/rewynd/core/internal/stats"
)

func TestSnapshotRoundTrip(t *testing.T) {
	t.Setenv("REWYND_HOME", t.TempDir())
	want := stats.Stats{Total: 7, ErrorRate: 0.25, Latency: stats.Percentiles{P95: 340}}
	if err := saveSnapshot("before", want); err != nil {
		t.Fatal(err)
	}
	got, err := loadSnapshot("before")
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 7 || got.ErrorRate != 0.25 || got.Latency.P95 != 340 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestLoadSnapshotMissing(t *testing.T) {
	t.Setenv("REWYND_HOME", t.TempDir())
	if _, err := loadSnapshot("nope"); err == nil || !strings.Contains(err.Error(), "--save") {
		t.Errorf("missing baseline should hint at --save, got %v", err)
	}
}

func TestCollapseRuns(t *testing.T) {
	// distinct, then a run of 3, then a singleton — keeps order, folds only the run.
	xs := []string{"a", "b", "b", "b", "c"}
	var runs [][2]int
	collapse(xs, func(s string) string { return s }, func(start, n int) {
		runs = append(runs, [2]int{start, n})
	})
	want := [][2]int{{0, 1}, {1, 3}, {4, 1}}
	if len(runs) != len(want) {
		t.Fatalf("got %v, want %v", runs, want)
	}
	for i := range want {
		if runs[i] != want[i] {
			t.Errorf("run %d = %v, want %v", i, runs[i], want[i])
		}
	}
	collapse([]string{}, func(s string) string { return s }, func(int, int) { t.Fatal("emit on empty") })
}

func TestCollapseNPlusOneByNormalizedStatement(t *testing.T) {
	// An N+1: one distinct SELECT, then 50 of the same normalized statement → 2 runs.
	qs := []model.Query{{StatementNormalized: "SELECT * FROM users"}}
	for k := 0; k < 50; k++ {
		qs = append(qs, model.Query{StatementNormalized: "SELECT * FROM posts WHERE user_id = ?", DurationMs: 16})
	}
	var counts []int
	collapse(qs, func(q model.Query) string { return q.Service + "\x00" + normStmt(q) }, func(start, n int) {
		counts = append(counts, n)
	})
	if len(counts) != 2 || counts[0] != 1 || counts[1] != 50 {
		t.Fatalf("expected runs [1, 50], got %v", counts)
	}
	if total := sumDur(qs[1:], func(q model.Query) float64 { return q.DurationMs }); total != 800 {
		t.Errorf("expected the N+1 to total 800ms, got %v", total)
	}
}

func TestDeltaDur(t *testing.T) {
	if got := deltaDur(340, 120); !strings.Contains(got, "340ms -> 120ms") || !strings.Contains(got, "-65%") {
		t.Errorf("deltaDur = %q", got)
	}
	if got := deltaDur(0, 50); !strings.Contains(got, "->") || strings.Contains(got, "%") {
		t.Errorf("deltaDur from zero baseline should not show a percent: %q", got)
	}
}
