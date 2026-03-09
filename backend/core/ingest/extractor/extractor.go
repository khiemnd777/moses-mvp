package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

var (
	spaceRunPattern       = regexp.MustCompile(`[ \t\f\v]+`)
	docxCoordTailPattern  = regexp.MustCompile(`(?:\s+\d{5,7}\s+\d{4,6}){2,}\s*$`)
	docxCoordLinePattern  = regexp.MustCompile(`^\d{5,7}\s+\d{4,6}(?:\s+\d{5,7}\s+\d{4,6})+$`)
	docxNumberOnlyPattern = regexp.MustCompile(`^\d+(?:\s+\d+)+$`)
)

func ExtractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	var (
		text string
		err  error
	)

	switch ext {
	case ".doc":
		text, err = extractDOC(path)
	case ".docx":
		text, err = extractDOCX(path)
	case ".pdf":
		text, err = extractPDF(path)
	case ".txt":
		text, err = extractTXT(path)
	default:
		return "", fmt.Errorf("unsupported extension: %s", ext)
	}
	if err != nil {
		return "", err
	}

	text = cleanUTF8AndWhitespace(text)
	slog.Default().Info("document_text_extracted", slog.String("file", path), slog.Int("size", len(text)))
	if len(text) < 50 {
		slog.Default().Warn("extracted_text_too_small", slog.String("file", path), slog.Int("size", len(text)))
	}
	return text, nil
}

func extractDOC(path string) (string, error) {

	cmd := exec.Command("antiword", path)

	out, err := cmd.Output()

	if err == nil {
		return string(out), nil
	}

	// fallback #1 try catdoc
	cmd = exec.Command("catdoc", path)
	out, err = cmd.Output()

	if err == nil {
		return string(out), nil
	}

	// fallback #2 return raw file
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", err
	}

	return string(data), nil
}

func extractDOCX(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	var xmlData []byte
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		xmlData, err = io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		break
	}
	if len(xmlData) == 0 {
		return "", errors.New("word/document.xml not found in docx")
	}

	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	var (
		out          strings.Builder
		insideTextEl bool
	)

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t", "instrText", "delText":
				insideTextEl = true
			case "tab":
				out.WriteByte('\t')
			case "br", "cr":
				out.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t", "instrText", "delText":
				insideTextEl = false
			case "p", "tr":
				out.WriteByte('\n')
			}
		case xml.CharData:
			if insideTextEl {
				out.WriteString(string(t))
			}
		}
	}

	text := html.UnescapeString(out.String())
	return cleanDOCXLayoutArtifacts(text), nil
}

func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	reader, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func extractTXT(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func cleanUTF8AndWhitespace(in string) string {
	valid := bytes.ToValidUTF8([]byte(in), []byte(""))
	text := string(valid)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(spaceRunPattern.ReplaceAllString(lines[i], " "))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func cleanDOCXLayoutArtifacts(in string) string {
	lines := strings.Split(in, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = docxCoordTailPattern.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if docxCoordLinePattern.MatchString(line) || docxNumberOnlyPattern.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
