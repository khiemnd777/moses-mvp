package retrieval

import (
	"testing"

	"github.com/khiemnd777/legal_api/core/answer"
)

func TestBuildFollowUpSearchQueryUsesPriorUserTurn(t *testing.T) {
	history := []answer.ConversationMessage{
		{Role: "user", Content: "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon?"},
		{Role: "assistant", Content: "..."},
	}

	got := BuildFollowUpSearchQuery(history, "Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?")

	if want := "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon? Cam on! Toi muon hoi them ve cac khoan chi phi hau ly hon?"; got != want {
		t.Fatalf("BuildFollowUpSearchQuery() = %q, want %q", got, want)
	}
}

func TestBuildFollowUpSearchQueryLeavesStandaloneQuestionAlone(t *testing.T) {
	got := BuildFollowUpSearchQuery(nil, "Thu tuc ly hon thuan tinh la gi?")
	if got != "Thu tuc ly hon thuan tinh la gi?" {
		t.Fatalf("BuildFollowUpSearchQuery() = %q, want standalone question unchanged", got)
	}
}

func TestBuildFollowUpSearchQueryDoesNotDragHistoryIntoStandaloneTurn(t *testing.T) {
	history := []answer.ConversationMessage{
		{Role: "user", Content: "Tu van cho toi ve quyen va nghia vu cua vo chong sau khi ly hon?"},
	}

	got := BuildFollowUpSearchQuery(history, "Thu tuc giai quyet ly hon don phuong la gi?")
	if got != "Thu tuc giai quyet ly hon don phuong la gi?" {
		t.Fatalf("BuildFollowUpSearchQuery() = %q, want standalone question unchanged even with history", got)
	}
}
