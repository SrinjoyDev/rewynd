package diag

import (
	"strings"
	"testing"

	"github.com/SrinjoyDev/rewynd/core/internal/model"
)

func TestDiagnoseDetectionsAndExceptions(t *testing.T) {
	r := &model.Request{
		Detections: []model.Detection{
			{Type: model.DetectNPlusOne, Title: "N+1 query — 10 identical statements", Suggestion: "Batch into one query"},
		},
		Exceptions: []model.Exception{
			{Type: "DatabaseError", Message: "null value in column \"total\"", Stack: "at run (db.ts:42)\nat next"},
			{Type: "DatabaseError", Message: "null value in column \"total\"", Stack: "dup"}, // deduped
		},
	}
	ps := Diagnose(r)
	if len(ps) != 2 {
		t.Fatalf("expected 2 problems (1 detection + 1 deduped exception), got %d: %+v", len(ps), ps)
	}
	if ps[0].Type != string(model.DetectNPlusOne) || ps[0].Suggestion != "Batch into one query" {
		t.Errorf("detection problem wrong: %+v", ps[0])
	}
	if ps[1].Type != "exception" || !strings.Contains(ps[1].Summary, "DatabaseError") || ps[1].Suggestion != "at run (db.ts:42)" {
		t.Errorf("exception problem wrong: %+v", ps[1])
	}
}

func TestDiagnoseUsesTitleWhenSummaryEmpty(t *testing.T) {
	r := &model.Request{Detections: []model.Detection{{Type: model.DetectSlowQuery, Title: "Slow query — 120ms"}}}
	ps := Diagnose(r)
	if len(ps) != 1 || ps[0].Summary != "Slow query — 120ms" {
		t.Errorf("expected the title as summary, got %+v", ps)
	}
}

func TestDiagnoseClean(t *testing.T) {
	if ps := Diagnose(&model.Request{}); len(ps) != 0 {
		t.Errorf("a clean request should have no problems, got %+v", ps)
	}
}
