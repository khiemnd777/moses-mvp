package api

import (
	"context"
	"testing"

	"github.com/khiemnd777/legal_api/core/answer"
)

func TestVietnameseEvalSuiteCitationValidatorAcceptsDiacriticAndNoDiacriticSources(t *testing.T) {
	handler := newTestHandler("http://example.invalid", newMemoryTraceRepo())
	sources := []answer.Source{
		{
			Text: "Điều 54. Hòa giải tại Tòa án theo Luật Hôn nhân và gia đình 2014.",
			Citation: answer.Citation{
				ID:            "citation-marriage-law",
				ChunkID:       "chunk-marriage-law",
				DocumentTitle: "Luật Hôn nhân và gia đình 2014",
				LawName:       "Luật Hôn nhân và gia đình 2014",
				DocumentType:  "LUẬT",
				Article:       "54",
			},
		},
		{
			Text: "Điều 6. Giá trị pháp lý của Giấy khai sinh theo Nghị định 123/2015/NĐ-CP.",
			Citation: answer.Citation{
				ID:             "citation-decree-6",
				ChunkID:        "chunk-decree-6",
				DocumentTitle:  "Nghị định số 123/2015/NĐ-CP ngày 15 tháng 11 năm 2015 quy định chi tiết Luật Hộ tịch",
				LawName:        "Nghị định số 123/2015/NĐ-CP",
				DocumentNumber: "123/2015/NĐ-CP",
				DocumentType:   "NGHỊ ĐỊNH",
				Article:        "6",
			},
		},
	}
	cases := []struct {
		name             string
		answerText       string
		wantCitationID   string
		wantDocumentNo   string
		wantArticle      string
		wantCitationSize int
	}{
		{
			name: "diacritic law title mention",
			answerText: "1. Vấn đề pháp lý\nXác định căn cứ ly hôn.\n\n" +
				"2. Áp dụng pháp luật\nLuật Hôn nhân và gia đình 2014 điều chỉnh quan hệ hôn nhân và gia đình.\n\n" +
				"3. Phân tích pháp lý\nNguồn truy xuất cho thấy luật này là văn bản điều chỉnh trực tiếp.\n\n" +
				"4. Kết luận\nCó thể đối chiếu yêu cầu ly hôn với Luật Hôn nhân và gia đình 2014.",
			wantCitationID:   "citation-marriage-law",
			wantArticle:      "54",
			wantCitationSize: 1,
		},
		{
			name: "no diacritic law title mention",
			answerText: "1. Van de phap ly\nXac dinh can cu ly hon.\n\n" +
				"2. Ap dung phap luat\nLuat Hon nhan va gia dinh 2014 dieu chinh quan he hon nhan va gia dinh.\n\n" +
				"3. Phan tich phap ly\nNguon truy xuat cho thay luat nay la van ban dieu chinh truc tiep.\n\n" +
				"4. Ket luan\nCo the doi chieu yeu cau ly hon voi Luat Hon nhan va gia dinh 2014.",
			wantCitationID:   "citation-marriage-law",
			wantArticle:      "54",
			wantCitationSize: 1,
		},
		{
			name: "no diacritic document number",
			answerText: "1. Vấn đề pháp lý\nXác định giá trị pháp lý giấy khai sinh.\n\n" +
				"2. Áp dụng pháp luật\nNghị định 123/2015/ND-CP: Điều 6.\n\n" +
				"3. Phân tích pháp lý\nĐiều 6 Nghị định 123/2015/ND-CP quy định về giá trị pháp lý của Giấy khai sinh.\n\n" +
				"4. Kết luận\nÁp dụng Điều 6 Nghị định 123/2015/ND-CP.",
			wantCitationID:   "citation-decree-6",
			wantDocumentNo:   "123/2015/NĐ-CP",
			wantArticle:      "6",
			wantCitationSize: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, citations, valid, err := handler.validateGeneratedLegalAnswer(context.Background(), tt.answerText, sources)
			if err != nil {
				t.Fatalf("validate legal answer: %v", err)
			}
			if !valid {
				t.Fatalf("expected answer to be valid, got invalid response %q", got)
			}
			if got != tt.answerText {
				t.Fatalf("expected answer to be preserved, got %q", got)
			}
			if len(citations) != tt.wantCitationSize {
				t.Fatalf("citations = %#v, want %d item(s)", citations, tt.wantCitationSize)
			}
			if citations[0].ID != tt.wantCitationID {
				t.Fatalf("citation ID = %q, want %q", citations[0].ID, tt.wantCitationID)
			}
			if citations[0].Article != tt.wantArticle {
				t.Fatalf("citation Article = %q, want %q", citations[0].Article, tt.wantArticle)
			}
			if tt.wantDocumentNo != "" && citations[0].DocumentNumber != tt.wantDocumentNo {
				t.Fatalf("citation DocumentNumber = %q, want %q", citations[0].DocumentNumber, tt.wantDocumentNo)
			}
		})
	}
}

func TestVietnameseEvalSuiteCitationValidatorRejectsUnsupportedVietnameseReferences(t *testing.T) {
	handler := newTestHandler("http://example.invalid", newMemoryTraceRepo())
	sources := []answer.Source{
		{
			Text: "Điều 6. Giá trị pháp lý của Giấy khai sinh theo Nghị định 123/2015/NĐ-CP.",
			Citation: answer.Citation{
				ID:             "citation-decree-6",
				ChunkID:        "chunk-decree-6",
				DocumentTitle:  "Nghị định số 123/2015/NĐ-CP ngày 15 tháng 11 năm 2015 quy định chi tiết Luật Hộ tịch",
				LawName:        "Nghị định số 123/2015/NĐ-CP",
				DocumentNumber: "123/2015/NĐ-CP",
				DocumentType:   "NGHỊ ĐỊNH",
				Article:        "6",
			},
		},
	}
	answerText := "1. Vấn đề pháp lý\nXác định giá trị giấy khai sinh.\n\n" +
		"2. Áp dụng pháp luật\nNghị định 999/2020/ND-CP: Điều 6.\n\n" +
		"3. Phân tích pháp lý\nĐiều 6 Nghị định 999/2020/ND-CP điều chỉnh vấn đề này.\n\n" +
		"4. Kết luận\nÁp dụng Điều 6 Nghị định 999/2020/ND-CP."

	got, citations, valid, err := handler.validateGeneratedLegalAnswer(context.Background(), answerText, sources)
	if err != nil {
		t.Fatalf("validate legal answer: %v", err)
	}
	if valid {
		t.Fatalf("expected unsupported no-diacritic document reference to be invalid")
	}
	if got == answerText {
		t.Fatalf("expected invalid answer to be replaced")
	}
	if len(citations) != 0 {
		t.Fatalf("expected no citations for invalid answer, got %#v", citations)
	}
}
