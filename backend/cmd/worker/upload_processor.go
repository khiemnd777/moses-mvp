package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/khiemnd777/legal_api/core/language"
	"github.com/khiemnd777/legal_api/core/legalmeta"
	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

var uploadDocumentNumberPattern = regexp.MustCompile(`(?i)(?:số\s*:?\s*)?([0-9]{1,4}[a-z]?/[0-9]{4}/[0-9A-ZĐđ-]+)`)

type uploadDocTypeResolution struct {
	DocType    domain.DocType
	Confidence float64
	Score      float64
	Reasons    []string
	Candidates []uploadDocTypeCandidate
	DocNumber  string
}

type uploadDocTypeCandidate struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}

func processNextDocumentUpload(ctx context.Context, logger *slog.Logger, store *infra.Store, storage *infra.Storage) (bool, error) {
	upload, claimed, err := store.ClaimNextDocumentUpload(ctx)
	if err != nil || !claimed {
		return claimed, err
	}
	if err := processDocumentUpload(ctx, logger, store, storage, upload); err != nil {
		return true, err
	}
	return true, nil
}

func processDocumentUpload(ctx context.Context, logger *slog.Logger, store *infra.Store, storage *infra.Storage, upload domain.DocumentUpload) error {
	text, err := storage.Read(upload.StoragePath)
	if err != nil {
		message := err.Error()
		analysis := uploadAnalysisJSON(map[string]interface{}{
			"pipeline_version": "rag_upload_intake_v1",
			"stage":            "extract",
			"status_reason":    "extract_failed",
		})
		return store.UpdateDocumentUploadStatus(ctx, upload.ID, "extract_failed", analysis, &message)
	}
	if err := recordUploadStageEvent(ctx, store, upload, "stage_transition", "extract", "done", "Extracted text from uploaded file", uploadAnalysisJSON(map[string]interface{}{
		"pipeline_version": "rag_upload_intake_v1",
		"text_chars":       len([]rune(text)),
	})); err != nil {
		return err
	}
	if err := recordUploadStageEvent(ctx, store, upload, "stage_transition", "normalize_vi", "done", "Vietnamese text normalized for classification and retrieval keys", uploadAnalysisJSON(map[string]interface{}{
		"pipeline_version": "rag_upload_intake_v1",
		"normalizer":       "language.SearchKey",
	})); err != nil {
		return err
	}

	docTypes, err := store.ListDocTypes(ctx)
	if err != nil {
		return err
	}
	resolution, ok := resolveUploadDocType(text, docTypes)
	analysis := uploadResolutionAnalysis(resolution, ok)
	if !ok {
		message := "unable to classify upload with sufficient confidence"
		return store.UpdateDocumentUploadStatus(ctx, upload.ID, "classification_low_confidence", analysis, &message)
	}
	if err := recordUploadStageEvent(ctx, store, upload, "stage_transition", "classify", "done", "Classified document type with sufficient confidence", analysis); err != nil {
		return err
	}

	promotion, err := store.PromoteDocumentUpload(ctx, upload, resolution.DocType, analysis)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Info("document_upload_promoted",
			slog.String("upload_id", upload.ID),
			slog.String("document_id", promotion.DocumentID),
			slog.String("document_version_id", promotion.DocumentVersionID),
			slog.String("ingest_job_id", promotion.IngestJobID),
			slog.String("doc_type_code", resolution.DocType.Code),
			slog.Float64("confidence", resolution.Confidence),
		)
	}
	return nil
}

func recordUploadStageEvent(ctx context.Context, store *infra.Store, upload domain.DocumentUpload, eventType, stage, status, message string, evidenceJSON []byte) error {
	_, err := store.CreateDocumentUploadEvent(ctx, domain.DocumentUploadEventInput{
		UploadID:      &upload.ID,
		EventType:     eventType,
		Stage:         stage,
		Status:        status,
		Message:       message,
		EvidenceJSON:  evidenceJSON,
		Actor:         "worker",
		FileName:      upload.FileName,
		ContentType:   upload.ContentType,
		FileSizeBytes: upload.FileSizeBytes,
		SHA256:        upload.SHA256,
	})
	return err
}

func resolveUploadDocType(text string, docTypes []domain.DocType) (uploadDocTypeResolution, bool) {
	docNumber := extractUploadDocumentNumber(text)
	textKey := language.SearchKey(firstN(text, 12000))
	candidates := make([]uploadDocTypeCandidate, 0, len(docTypes))
	for _, docType := range docTypes {
		score, reasons := scoreUploadDocType(text, textKey, docNumber, docType)
		candidates = append(candidates, uploadDocTypeCandidate{
			Code:    docType.Code,
			Name:    docType.Name,
			Score:   score,
			Reasons: reasons,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) == 0 || candidates[0].Score < 5 {
		return uploadDocTypeResolution{Candidates: candidates, DocNumber: docNumber}, false
	}
	best := candidates[0]
	secondScore := 0.0
	if len(candidates) > 1 {
		secondScore = candidates[1].Score
	}
	confidence := best.Score / (best.Score + secondScore + 1)
	if confidence < 0.55 {
		return uploadDocTypeResolution{Confidence: confidence, Score: best.Score, Candidates: candidates, DocNumber: docNumber}, false
	}
	for _, docType := range docTypes {
		if docType.Code == best.Code {
			return uploadDocTypeResolution{
				DocType:    docType,
				Confidence: confidence,
				Score:      best.Score,
				Reasons:    best.Reasons,
				Candidates: candidates,
				DocNumber:  docNumber,
			}, true
		}
	}
	return uploadDocTypeResolution{Confidence: confidence, Score: best.Score, Candidates: candidates, DocNumber: docNumber}, false
}

func scoreUploadDocType(text, textKey, docNumber string, docType domain.DocType) (float64, []string) {
	score := 0.0
	reasons := make([]string, 0)
	docProfileKey := language.SearchKey(docType.Code + " " + docType.Name)
	var form schema.DocTypeForm
	if len(docType.FormJSON) > 0 && json.Unmarshal(docType.FormJSON, &form) == nil {
		docProfileKey = language.SearchKey(docProfileKey + " " + form.DocType.Code + " " + form.DocType.Name)
		for _, rule := range form.MappingRules {
			matched, exactDocNumber := mappingRuleMatchesUpload(text, docNumber, rule)
			if !matched {
				continue
			}
			weight := 1.0
			switch rule.Field {
			case "document_number":
				weight = 3
				if exactDocNumber {
					weight = 8
				}
			case "document_type", "issuing_authority", "legal_domain":
				weight = 2
			}
			score += weight
			reasons = append(reasons, "mapping:"+rule.Field)
		}
		for _, term := range form.QueryProfile.QuerySignals {
			if containsUploadTerm(textKey, term) {
				score += 0.4
			}
		}
		for _, term := range form.QueryProfile.CanonicalTerms {
			if containsUploadTerm(textKey, term) {
				score += 0.5
			}
		}
	}

	refKey := uploadReferenceKey(docNumber)
	if refKey != "" && strings.Contains(uploadReferenceKey(docProfileKey), refKey) {
		score += 10
		reasons = append(reasons, "document_number_profile_match")
	}
	if scoreDocTypeTopic(textKey, docType.Code) {
		score += 10
		reasons = append(reasons, "topic_profile_match")
	}
	if docNumber == "" && !looksLikeNormativeLegalDocument(textKey) {
		score *= 0.25
		reasons = append(reasons, "no_normative_marker")
	}
	return score, reasons
}

func mappingRuleMatchesUpload(text, docNumber string, rule schema.MappingRule) (bool, bool) {
	if strings.TrimSpace(rule.Regex) == "" {
		return false, false
	}
	re, err := regexp.Compile(rule.Regex)
	if err != nil {
		return false, false
	}
	matches := re.FindStringSubmatch(text)
	if len(matches) == 0 {
		return false, false
	}
	if rule.Field != "document_number" || docNumber == "" {
		return true, false
	}
	idx := rule.Group
	if idx < 0 || idx >= len(matches) {
		idx = 0
	}
	return true, legalmeta.NormalizeDocumentNumber(matches[idx]) == docNumber
}

func scoreDocTypeTopic(textKey, code string) bool {
	switch code {
	case "vn_civil_code":
		return strings.Contains(textKey, "bo luat dan su")
	case "vn_marriage_family_law":
		return strings.Contains(textKey, "luat hon nhan va gia dinh")
	case "vn_decree_civil_status_123_2015":
		return strings.Contains(textKey, "123/2015") || strings.Contains(textKey, "luat ho tich")
	case "vn_decree_marriage_family_126_2014":
		return strings.Contains(textKey, "126/2014")
	case "vn_circular_marriage_family_foreign_2015":
		return strings.Contains(textKey, "02a/2015") || strings.Contains(textKey, "bo tu phap")
	case "vn_resolution_marriage_family_01_2024":
		return strings.Contains(textKey, "01/2024") && strings.Contains(textKey, "hoi dong tham phan")
	case "vn_resolution_marriage_family_35_2000":
		return strings.Contains(textKey, "35/2000")
	default:
		return false
	}
}

func looksLikeNormativeLegalDocument(textKey string) bool {
	return strings.Contains(textKey, "cong hoa xa hoi chu nghia viet nam") ||
		strings.Contains(textKey, "quoc hoi") ||
		strings.Contains(textKey, "chinh phu") ||
		strings.Contains(textKey, "bo tu phap") ||
		strings.Contains(textKey, "toa an nhan dan toi cao")
}

func extractUploadDocumentNumber(text string) string {
	match := uploadDocumentNumberPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return legalmeta.NormalizeDocumentNumber(strings.Trim(match[1], " \t\r\n.,;:()[]{}"))
}

func containsUploadTerm(textKey, term string) bool {
	termKey := language.SearchKey(term)
	return termKey != "" && strings.Contains(textKey, termKey)
}

func uploadReferenceKey(value string) string {
	key := language.SearchKey(value)
	replacer := strings.NewReplacer("/", " ", "_", " ", "-", " ")
	return strings.Join(strings.Fields(replacer.Replace(key)), " ")
}

func uploadResolutionAnalysis(resolution uploadDocTypeResolution, ok bool) []byte {
	statusReason := "classified"
	if !ok {
		statusReason = "classification_low_confidence"
	}
	return uploadAnalysisJSON(map[string]interface{}{
		"pipeline_version":       "rag_upload_intake_v1",
		"status_reason":          statusReason,
		"detected_doc_type_code": resolution.DocType.Code,
		"document_number":        resolution.DocNumber,
		"confidence":             resolution.Confidence,
		"score":                  resolution.Score,
		"reasons":                resolution.Reasons,
		"candidates":             resolution.Candidates,
	})
}

func uploadAnalysisJSON(value map[string]interface{}) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func firstN(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	count := 0
	for idx := range value {
		if count == limit {
			return value[:idx]
		}
		count++
	}
	return value
}
