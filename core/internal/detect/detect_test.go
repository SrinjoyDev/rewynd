package detect

import (
	"testing"

	"github.com/SrinjoyDev/rewynd/internal/model"
)

func q(norm string) model.Query {
	return model.Query{StatementNormalized: norm, DurationMs: 1}
}

func repeat(norm string, n int) []model.Query {
	out := make([]model.Query, n)
	for i := range out {
		out[i] = q(norm)
	}
	return out
}

func TestNPlusOneFires(t *testing.T) {
	qs := append([]model.Query{q("SELECT id FROM users")}, repeat("SELECT * FROM posts WHERE u=?", 5)...)
	ds := NPlusOne("r1", qs, 5)
	if len(ds) != 1 {
		t.Fatalf("want 1 detection, got %d", len(ds))
	}
	if ds[0].Type != model.DetectNPlusOne {
		t.Fatalf("want n_plus_one, got %s", ds[0].Type)
	}
	if c, _ := ds[0].Evidence["count"].(int); c != 5 {
		t.Fatalf("want count 5, got %d", c)
	}
}

func TestNPlusOneBelowThreshold(t *testing.T) {
	if ds := NPlusOne("r", repeat("SELECT x", 4), 5); len(ds) != 0 {
		t.Fatalf("must not fire below threshold, got %d", len(ds))
	}
}

func TestNPlusOneSkipsEmpty(t *testing.T) {
	if ds := NPlusOne("r", repeat("", 8), 5); len(ds) != 0 {
		t.Fatalf("empty statements must never be N+1, got %d", len(ds))
	}
}

func TestNPlusOneRanksByCount(t *testing.T) {
	qs := append(repeat("A", 6), repeat("B", 9)...)
	ds := NPlusOne("r", qs, 5)
	if len(ds) != 2 {
		t.Fatalf("want 2 detections, got %d", len(ds))
	}
	if ds[0].Evidence["count"].(int) < ds[1].Evidence["count"].(int) {
		t.Fatal("detections should be ordered by count desc")
	}
}
