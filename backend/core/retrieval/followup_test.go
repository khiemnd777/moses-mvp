package retrieval

import (
	"encoding/json"
	"testing"

	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
)

func TestBuildFollowUpSearchQueryUsesPriorUserTurn(t *testing.T) {
	history := []answer.ConversationMessage{
		{Role: "user", Content: "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon?"},
		{Role: "assistant", Content: "..."},
	}

	got := buildFollowUpSearchQueryWithIndex(testQueryIndex(), history, "Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?")

	if want := "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon? Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?"; got != want {
		t.Fatalf("BuildFollowUpSearchQuery() = %q, want %q", got, want)
	}
}

func TestBuildFollowUpSearchQueryLeavesStandaloneQuestionAlone(t *testing.T) {
	got := buildFollowUpSearchQueryWithIndex(testQueryIndex(), nil, "Thu tuc ly hon thuan tinh la gi?")
	if got != "Thu tuc ly hon thuan tinh la gi?" {
		t.Fatalf("BuildFollowUpSearchQuery() = %q, want standalone question unchanged", got)
	}
}

func TestBuildFollowUpSearchQueryDoesNotDragHistoryIntoStandaloneTurn(t *testing.T) {
	history := []answer.ConversationMessage{
		{Role: "user", Content: "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon?"},
	}

	got := buildFollowUpSearchQueryWithIndex(testQueryIndex(), history, "Thu tuc giai quyet ly hon don phuong la gi?")
	if got != "Thu tuc giai quyet ly hon don phuong la gi?" {
		t.Fatalf("BuildFollowUpSearchQuery() = %q, want standalone question unchanged even with history", got)
	}
}

func testQueryIndex() queryUnderstandingIndex {
	return buildQueryUnderstandingIndex([]domain.DocType{{
		Code:     "legal_normative",
		FormHash: "hash-1",
		FormJSON: mustJSON(schema.DocTypeForm{
			Version:       1,
			DocType:       schema.DocType{Code: "legal_normative", Name: "Legal Normative"},
			SegmentRules:  schema.SegmentRules{Strategy: "legal_article", Hierarchy: "article", Normalization: "basic"},
			Metadata:      schema.MetadataSchema{Fields: []schema.MetadataField{{Name: "title", Type: "string"}}},
			MappingRules:  []schema.MappingRule{{Field: "title", Group: 1}},
			ReindexPolicy: schema.ReindexPolicy{OnContentChange: true, OnFormChange: true},
			QueryProfile: schema.QueryProfile{
				FollowUpMarkers: []string{"cam on", "hoi them"},
				SynonymGroups:   []schema.SynonymGroup{{Canonical: "ly hon", Aliases: []string{"ly di"}}},
			},
		}),
	}})
}

func mustJSON(value interface{}) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
