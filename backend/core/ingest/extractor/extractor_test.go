package extractor

import "testing"

func TestCleanDOCXLayoutArtifacts_RemovesCoordinateNoise(t *testing.T) {
	in := "QUỐC HỘI 662940 72390 662940 72390\nLuật số: 52/2014/QH13\n777240 111125 777240 111125"
	got := cleanDOCXLayoutArtifacts(in)
	want := "QUỐC HỘI\nLuật số: 52/2014/QH13"
	if got != want {
		t.Fatalf("unexpected cleaned text:\n got=%q\nwant=%q", got, want)
	}
}

func TestCleanDOCXLayoutArtifacts_KeepLegalNumbers(t *testing.T) {
	in := "Điều 81. Phạm vi điều chỉnh\nNghị định 12/2024/NĐ-CP"
	got := cleanDOCXLayoutArtifacts(in)
	if got != in {
		t.Fatalf("cleaning removed valid legal text:\n got=%q\nwant=%q", got, in)
	}
}
