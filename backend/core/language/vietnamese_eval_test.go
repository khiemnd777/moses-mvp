package language

import "testing"

func TestVietnameseEvalSuiteNormalization(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantNFC   string
		wantKey   string
		wantDocNo string
	}{
		{
			name:      "legacy D and combining marks",
			input:     "Ðiều 1 về đăng ký hộ tịch",
			wantNFC:   "Điều 1 về đăng ký hộ tịch",
			wantKey:   "dieu 1 ve dang ky ho tich",
			wantDocNo: "Điều 1 về đăng ký hộ tịch",
		},
		{
			name:      "legal abbreviation with no diacritics",
			input:     "123/2015/ND-CP",
			wantNFC:   "123/2015/ND-CP",
			wantKey:   "123/2015/nd cp",
			wantDocNo: "123/2015/NĐ-CP",
		},
		{
			name:      "resolution abbreviation with vietnamese D",
			input:     "01/2024/NQ-HĐTP",
			wantNFC:   "01/2024/NQ-HĐTP",
			wantKey:   "01/2024/nq hdtp",
			wantDocNo: "01/2024/NQ-HĐTP",
		},
		{
			name:      "dash variants",
			input:     "02A/2015/tt–btp",
			wantNFC:   "02A/2015/tt–btp",
			wantKey:   "02a/2015/tt btp",
			wantDocNo: "02A/2015/TT-BTP",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeNFC(tt.input); got != tt.wantNFC {
				t.Fatalf("NormalizeNFC(%q) = %q, want %q", tt.input, got, tt.wantNFC)
			}
			if got := SearchKey(tt.input); got != tt.wantKey {
				t.Fatalf("SearchKey(%q) = %q, want %q", tt.input, got, tt.wantKey)
			}
			if got := NormalizeLegalDocumentNumber(tt.input); got != tt.wantDocNo {
				t.Fatalf("NormalizeLegalDocumentNumber(%q) = %q, want %q", tt.input, got, tt.wantDocNo)
			}
		})
	}
}
