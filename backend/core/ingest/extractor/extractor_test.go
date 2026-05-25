package extractor

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestExtractPPTXReadsSlideTextInOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legal.pptx")
	writeZip(t, path, map[string]string{
		"ppt/slides/slide2.xml": pptxSlideXML("Điều 2. Đối tượng áp dụng", "1. Cơ quan, tổ chức, cá nhân."),
		"ppt/slides/slide1.xml": pptxSlideXML("Điều 1. Phạm vi điều chỉnh", "1. Văn bản này quy định về hộ tịch."),
	})

	got, err := ExtractText(path)
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if !strings.Contains(got, "Điều 1. Phạm vi điều chỉnh") || !strings.Contains(got, "Điều 2. Đối tượng áp dụng") {
		t.Fatalf("missing slide text: %q", got)
	}
	if strings.Index(got, "Điều 1") > strings.Index(got, "Điều 2") {
		t.Fatalf("slides extracted out of order: %q", got)
	}
}

func TestExtractPPTXReturnsClearErrorWhenSlidesMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pptx")
	writeZip(t, path, map[string]string{"docProps/core.xml": "<xml/>"})

	_, err := ExtractText(path)
	if err == nil || !strings.Contains(err.Error(), "ppt/slides/slide*.xml not found in pptx") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func pptxSlideXML(lines ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>`)
	for _, line := range lines {
		b.WriteString(`<p:sp><p:txBody><a:p><a:r><a:t>`)
		b.WriteString(line)
		b.WriteString(`</a:t></a:r></a:p></p:txBody></p:sp>`)
	}
	b.WriteString(`</p:spTree></p:cSld></p:sld>`)
	return b.String()
}
