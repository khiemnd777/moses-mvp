package legalmeta

import "testing"

func TestNormalizeDocumentType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "BỘ LUẬT", want: "law"},
		{in: "Luật", want: "law"},
		{in: "nghị quyết", want: "resolution"},
		{in: "Nghị định", want: "decree"},
		{in: "Thông tư", want: "circular"},
	}
	for _, tt := range tests {
		if got := NormalizeDocumentType(tt.in); got != tt.want {
			t.Fatalf("NormalizeDocumentType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeLegalDomain(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Hôn nhân và gia đình", want: "marriage_family"},
		{in: "DÂN SỰ", want: "civil"},
		{in: "hộ tịch", want: "civil_status"},
	}
	for _, tt := range tests {
		if got := NormalizeLegalDomain(tt.in); got != tt.want {
			t.Fatalf("NormalizeLegalDomain(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeEffectiveStatus(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "active", want: "active"},
		{in: "archived", want: "archived"},
		{in: "có hiệu lực thi hành từ ngày 01 tháng 01 năm 2017", want: "active"},
		{in: "hết hiệu lực", want: "archived"},
	}
	for _, tt := range tests {
		if got := NormalizeEffectiveStatus(tt.in); got != tt.want {
			t.Fatalf("NormalizeEffectiveStatus(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
