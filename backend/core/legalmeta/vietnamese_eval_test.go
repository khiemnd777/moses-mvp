package legalmeta

import "testing"

func TestVietnameseEvalSuiteLegalMetadataNormalization(t *testing.T) {
	cases := []struct {
		name         string
		domainInput  string
		wantDomain   string
		docTypeInput string
		wantDocType  string
		statusInput  string
		wantStatus   string
		docNoInput   string
		wantDocument string
	}{
		{
			name:         "diacritic legal metadata",
			domainInput:  "Hôn nhân và gia đình",
			wantDomain:   "marriage_family",
			docTypeInput: "Nghị định",
			wantDocType:  "decree",
			statusInput:  "có hiệu lực thi hành từ ngày 01 tháng 01 năm 2017",
			wantStatus:   "active",
			docNoInput:   "123/2015/NĐ-CP",
			wantDocument: "123/2015/NĐ-CP",
		},
		{
			name:         "no diacritic legal metadata",
			domainInput:  "hon nhan gia dinh",
			wantDomain:   "marriage_family",
			docTypeInput: "nghi quyet",
			wantDocType:  "resolution",
			statusInput:  "het hieu luc",
			wantStatus:   "archived",
			docNoInput:   "01/2024/NQ-HDTP",
			wantDocument: "01/2024/NQ-HĐTP",
		},
		{
			name:         "upper case civil status metadata",
			domainInput:  "HỘ TỊCH",
			wantDomain:   "civil_status",
			docTypeInput: "THÔNG TƯ",
			wantDocType:  "circular",
			statusInput:  "ACTIVE",
			wantStatus:   "active",
			docNoInput:   "02A/2015/tt-btp",
			wantDocument: "02A/2015/TT-BTP",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeLegalDomain(tt.domainInput); got != tt.wantDomain {
				t.Fatalf("NormalizeLegalDomain(%q) = %q, want %q", tt.domainInput, got, tt.wantDomain)
			}
			if got := NormalizeDocumentType(tt.docTypeInput); got != tt.wantDocType {
				t.Fatalf("NormalizeDocumentType(%q) = %q, want %q", tt.docTypeInput, got, tt.wantDocType)
			}
			if got := NormalizeEffectiveStatus(tt.statusInput); got != tt.wantStatus {
				t.Fatalf("NormalizeEffectiveStatus(%q) = %q, want %q", tt.statusInput, got, tt.wantStatus)
			}
			if got := NormalizeDocumentNumber(tt.docNoInput); got != tt.wantDocument {
				t.Fatalf("NormalizeDocumentNumber(%q) = %q, want %q", tt.docNoInput, got, tt.wantDocument)
			}
		})
	}
}
