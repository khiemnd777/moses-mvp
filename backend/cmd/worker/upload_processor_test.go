package main

import (
	"encoding/json"
	"testing"

	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
)

func TestResolveUploadDocTypePrefersExactVietnameseDocumentNumber(t *testing.T) {
	docTypes := []domain.DocType{
		testUploadDocType("decree-126", "vn_decree_marriage_family_126_2014", "Nghị định 126/2014/NĐ-CP"),
		testUploadDocType("decree-123", "vn_decree_civil_status_123_2015", "Nghị định 123/2015/NĐ-CP"),
	}
	text := `
CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM
CHÍNH PHỦ
NGHỊ ĐỊNH
Số: 123/2015/NĐ-CP
Điều 1. Phạm vi điều chỉnh
`

	got, ok := resolveUploadDocType(text, docTypes)
	if !ok {
		t.Fatalf("resolveUploadDocType() was not confident: %#v", got)
	}
	if got.DocType.Code != "vn_decree_civil_status_123_2015" {
		t.Fatalf("doc type = %q, want vn_decree_civil_status_123_2015", got.DocType.Code)
	}
	if got.DocNumber != "123/2015/NĐ-CP" {
		t.Fatalf("document number = %q", got.DocNumber)
	}
}

func TestResolveUploadDocTypeRejectsNonNormativeLectureDeck(t *testing.T) {
	docTypes := []domain.DocType{
		testUploadDocType("law-family", "vn_marriage_family_law", "Vietnam Marriage & Family Law"),
	}
	text := `
Bài giảng hôn nhân gia đình
Kết hôn, ly hôn, tài sản chung của vợ chồng
Các ví dụ thảo luận trên lớp
`

	got, ok := resolveUploadDocType(text, docTypes)
	if ok {
		t.Fatalf("resolveUploadDocType() = %#v, want low confidence", got)
	}
}

func testUploadDocType(id, code, name string) domain.DocType {
	form := schema.DocTypeForm{
		Version: 1,
		DocType: schema.DocType{
			Code: code,
			Name: name,
		},
		MappingRules: []schema.MappingRule{
			{Field: "document_number", Regex: `(?i)Số:\s*([0-9]{1,4}[a-z]?/[0-9]{4}/[0-9A-ZĐđ\-]+)`, Group: 1},
			{Field: "document_type", Regex: `(?i)NGHỊ ĐỊNH|LUẬT|BỘ LUẬT|NGHỊ QUYẾT|THÔNG TƯ`, Group: 0},
		},
		QueryProfile: schema.QueryProfile{
			QuerySignals:   []string{"hôn nhân", "gia đình", "hộ tịch", "ly hôn"},
			CanonicalTerms: []string{"hon nhan gia dinh", "ho tich", "ly hon"},
		},
	}
	raw, err := json.Marshal(form)
	if err != nil {
		panic(err)
	}
	return domain.DocType{
		ID:       id,
		Code:     code,
		Name:     name,
		FormJSON: raw,
	}
}
