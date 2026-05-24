package language

import "testing"

func TestNormalizeNFCPreservesVietnameseD(t *testing.T) {
	got := NormalizeNFC("Ðiều 1 về đăng ký")
	want := "Điều 1 về đăng ký"
	if got != want {
		t.Fatalf("NormalizeNFC() = %q, want %q", got, want)
	}
}

func TestFoldAccentsForSearchHandlesVietnameseD(t *testing.T) {
	got := FoldAccentsForSearch("Đăng ký hộ tịch theo Điều 5")
	want := "dang ky ho tich theo dieu 5"
	if got != want {
		t.Fatalf("FoldAccentsForSearch() = %q, want %q", got, want)
	}
}

func TestCanonicalLegalAbbreviationHandlesVietnameseAndNoDiacriticInput(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "NĐ-CP", want: "NĐ-CP"},
		{in: "ND-CP", want: "NĐ-CP"},
		{in: "nghi dinh chinh phu", want: "NĐ-CP"},
		{in: "NQ-HĐTP", want: "NQ-HĐTP"},
		{in: "NQ-HDTP", want: "NQ-HĐTP"},
		{in: "nghi quyet hoi dong tham phan", want: "NQ-HĐTP"},
		{in: "TT-BTP", want: "TT-BTP"},
		{in: "thong tu bo tu phap", want: "TT-BTP"},
	}
	for _, tt := range tests {
		if got := CanonicalLegalAbbreviation(tt.in); got != tt.want {
			t.Fatalf("CanonicalLegalAbbreviation(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeLegalDocumentNumberCanonicalizesAbbreviationSuffix(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "123/2015/ND-CP", want: "123/2015/NĐ-CP"},
		{in: "01/2024/NQ-HDTP", want: "01/2024/NQ-HĐTP"},
		{in: "02A/2015/tt-btp", want: "02A/2015/TT-BTP"},
	}
	for _, tt := range tests {
		if got := NormalizeLegalDocumentNumber(tt.in); got != tt.want {
			t.Fatalf("NormalizeLegalDocumentNumber(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
