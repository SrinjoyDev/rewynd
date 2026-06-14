package cli

import (
	"strings"
	"testing"

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

func TestDeltaDur(t *testing.T) {
	if got := deltaDur(340, 120); !strings.Contains(got, "340ms -> 120ms") || !strings.Contains(got, "-65%") {
		t.Errorf("deltaDur = %q", got)
	}
	if got := deltaDur(0, 50); !strings.Contains(got, "->") || strings.Contains(got, "%") {
		t.Errorf("deltaDur from zero baseline should not show a percent: %q", got)
	}
}
