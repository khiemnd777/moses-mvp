package api

import (
	"strings"
	"testing"

	"github.com/khiemnd777/legal_api/core/answer"
)

func TestBuildTelegramOutgoingMessagesSplitsAndAddsCitationLinks(t *testing.T) {
	h := &Handler{
		PublicBaseURL: "https://example.test",
		SigningSecret: "test-secret",
	}
	content := strings.Repeat("nội dung trả lời. ", 500)
	messages := h.buildTelegramOutgoingMessages(content, []answer.Citation{
		{
			ChunkID:       "chunk-1",
			DocumentTitle: "Luật thử nghiệm",
			Article:       "1",
			Excerpt:       "trích dẫn",
		},
	}, 1000)
	if len(messages) < 2 {
		t.Fatalf("expected long answer to be split, got %d message(s)", len(messages))
	}
	for _, message := range messages {
		if len([]rune(message.Text)) > 1000 {
			t.Fatalf("message length exceeded limit: %d", len([]rune(message.Text)))
		}
	}
	last := messages[len(messages)-1]
	if !strings.Contains(last.Text, "Nguồn:") {
		t.Fatalf("expected final message to include citation block: %q", last.Text)
	}
	if last.ReplyMarkup == nil || len(last.ReplyMarkup.InlineKeyboard) == 0 {
		t.Fatalf("expected final message to include citation buttons")
	}
	if !strings.HasPrefix(last.ReplyMarkup.InlineKeyboard[0][0].URL, "https://example.test/public/citations/") {
		t.Fatalf("expected signed citation URL, got %q", last.ReplyMarkup.InlineKeyboard[0][0].URL)
	}
}
