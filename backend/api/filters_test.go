package api

import "testing"

func TestNormalizeEffectiveStatusPreservesEmptyAsNoFilter(t *testing.T) {
	if got := normalizeEffectiveStatus(""); got != "" {
		t.Fatalf("normalizeEffectiveStatus empty = %q, want empty", got)
	}
	if got := normalizeEffectiveStatus("unknown"); got != "" {
		t.Fatalf("normalizeEffectiveStatus unknown = %q, want empty", got)
	}
	if got := normalizeEffectiveStatus("có hiệu lực"); got != "active" {
		t.Fatalf("normalizeEffectiveStatus active = %q", got)
	}
}

func TestNormalizeAnswerRequestNormalizesDocumentNumber(t *testing.T) {
	_, filters := normalizeAnswerRequest(answerRequest{
		Question: "Điều 6 NĐ 123",
		Filters: ChatFilters{
			DocumentNumber: "123/2015/ND-CP",
		},
	}, nil)
	if filters.DocumentNumber != "123/2015/NĐ-CP" {
		t.Fatalf("document number = %q", filters.DocumentNumber)
	}
	if filters.EffectiveStatus != "" {
		t.Fatalf("effective status = %q, want empty", filters.EffectiveStatus)
	}
}
