package ingest

import (
	"testing"

	"github.com/khiemnd777/legal_api/core/schema"
)

func TestCompileSegmentPlanPreservesConfiguredHierarchy(t *testing.T) {
	plan, err := compileSegmentPlan(schema.SegmentRules{
		Strategy:      "legal_article",
		Hierarchy:     "part.chapter.article.clause.point",
		Normalization: "basic",
		LevelPatterns: map[string]string{
			"part":    `(?im)^\s*Phần\s+thứ\s+([a-zà-ỹ]+)\s*$`,
			"chapter": `(?im)^\s*Chương\s+([IVXLCDM]+)\s*$`,
			"article": `(?im)^\s*Điều\s+([0-9]+)\.?\s*(.*)$`,
			"clause":  `(?m)^\s*([0-9]+)\.\s*(.*)$`,
			"point":   `(?m)^\s*([a-zđ])\)\s*(.*)$`,
		},
	})
	if err != nil {
		t.Fatalf("compileSegmentPlan() error = %v", err)
	}
	got := planLevelNames(plan)
	want := []string{"part", "chapter", "article", "clause", "point"}
	if len(got) != len(want) {
		t.Fatalf("unexpected level count: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected level order: got=%v want=%v", got, want)
		}
	}
}
