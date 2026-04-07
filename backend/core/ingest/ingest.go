package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/core/legalmeta"
	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type Config struct {
	ChunkSize    int
	ChunkOverlap int
}

type Service struct {
	Store  ingestStore
	Qdrant vectorStore
	Embed  Embedder
	Config Config
	Logger *slog.Logger
}

type ingestStore interface {
	TouchJob(ctx context.Context, id string) error
	CountChunksByVersion(ctx context.Context, documentVersionID string) (int, error)
	ReplaceChunks(ctx context.Context, documentVersionID string, chunks []domain.Chunk) ([]domain.Chunk, error)
	DeleteChunksByVersion(ctx context.Context, documentVersionID string) error
}

type vectorStore interface {
	Upsert(ctx context.Context, points []infra.PointInput) error
	Delete(ctx context.Context, ids []string) error
	GetPayloadByPointID(ctx context.Context, pointID string) (map[string]interface{}, bool, error)
}

type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float64, error)
}

func (s *Service) Run(ctx context.Context, job domain.IngestJob, bundle Bundle) error {
	if s.Store == nil || s.Qdrant == nil || s.Embed == nil {
		return errors.New("ingest service dependencies missing")
	}
	logger := s.logger().With(
		slog.String("document_id", bundle.Document.ID),
		slog.String("document_version_id", bundle.Version.ID),
		slog.String("job_id", job.ID),
	)
	startedAt := time.Now()
	attempt := infra.DecodeJobAttempt(job) + 1

	form, err := decodeForm(bundle.DocType.FormJSON)
	if err != nil {
		return err
	}
	formHash, _ := form.Hash()
	content, err := bundle.AssetContent()
	if err != nil {
		return err
	}
	normalized := normalize(content, form.SegmentRules.Normalization)
	contentHash := contentHash(normalized)
	metadata := extractMetadata(normalized, form.MappingRules)
	logger.Info("chunk_generation_started")
	chunkStartedAt := time.Now()
	generatedChunks, chunkStats, err := s.generateChunks(bundle.Document.ID, bundle.Version.ID, normalized, form.SegmentRules, metadata)
	if err != nil {
		return err
	}
	if len(generatedChunks) == 0 {
		return errors.New("no chunks produced")
	}
	logger.Info("chunk_generation_completed",
		slog.Int("chunk_count", chunkStats.ChunkCount),
		slog.Int("avg_chunk_tokens", chunkStats.AvgChunkTokens),
		slog.Int("max_chunk_tokens", chunkStats.MaxChunkTokens),
		slog.Int64("duration_ms", time.Since(chunkStartedAt).Milliseconds()),
	)

	logger.Info("ingest_started",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(generatedChunks)),
	)
	if err := s.Store.TouchJob(ctx, job.ID); err != nil {
		return err
	}
	if err := s.recoverPartialIngest(ctx, bundle.Version.ID); err != nil {
		return err
	}

	shouldSkip, err := s.shouldSkipIngest(ctx, bundle.Version.ID, len(generatedChunks), contentHash, formHash)
	if err != nil {
		return err
	}
	if shouldSkip {
		logger.Info("ingest_completed",
			slog.Int("attempt", attempt),
			slog.Int("chunk_count", len(generatedChunks)),
			slog.Duration("duration", time.Since(startedAt)),
			slog.Bool("skipped", true),
		)
		return nil
	}

	logger.Info("chunks_created",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(generatedChunks)),
	)

	chunkTexts := make([]string, 0, len(generatedChunks))
	for _, chunk := range generatedChunks {
		chunkTexts = append(chunkTexts, chunk.Text)
	}

	vectors, err := s.embedInBatches(ctx, chunkTexts, 32)
	if err != nil {
		return err
	}
	if len(vectors) != len(generatedChunks) {
		return errors.New("embedding vector count mismatch")
	}
	logger.Info("embedding_done",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(generatedChunks)),
	)
	if err := s.Store.TouchJob(ctx, job.ID); err != nil {
		return err
	}

	oldChunkCount, err := s.Store.CountChunksByVersion(ctx, bundle.Version.ID)
	if err != nil {
		return err
	}

	replacement := make([]domain.Chunk, 0, len(generatedChunks))
	for i, chunk := range generatedChunks {
		embedJSON, _ := json.Marshal(vectors[i])
		replacement = append(replacement, domain.Chunk{
			ID:                ChunkRecordID(bundle.Version.ID, chunk.Index),
			DocumentVersionID: bundle.Version.ID,
			Index:             chunk.Index,
			Text:              chunk.Text,
			MetadataJSON:      chunk.Metadata,
			EmbeddingJSON:     embedJSON,
		})
	}

	points := make([]infra.PointInput, 0, len(generatedChunks))
	for _, chunk := range replacement {
		vectorID := VectorPointID(bundle.Version.ID, chunk.Index)
		metaMap := generatedChunks[chunk.Index].MetaMap
		retrievalPayload := buildRetrievalPayload(metaMap)
		payload := map[string]interface{}{
			"chunk_id":            chunk.ID,
			"document_version_id": bundle.Version.ID,
			"chunk_index":         chunk.Index,
			"citation_id":         citationID(bundle.Version.ID, chunk.Index, chunk.Text),
			"content_hash":        contentHash,
			"form_hash":           formHash,
		}
		for k, v := range retrievalPayload {
			payload[k] = v
		}
		points = append(points, infra.PointInput{
			ID:      vectorID,
			Vector:  vectors[chunk.Index],
			Payload: payload,
		})
	}
	if err := s.Qdrant.Upsert(ctx, points); err != nil {
		return err
	}
	insertedChunks, err := s.Store.ReplaceChunks(ctx, bundle.Version.ID, replacement)
	if err != nil {
		return err
	}
	if oldChunkCount > len(insertedChunks) {
		if err := s.Qdrant.Delete(ctx, staleVectorIDs(bundle.Version.ID, len(insertedChunks), oldChunkCount)); err != nil {
			return err
		}
	}
	logger.Info("vector_write_done",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(insertedChunks)),
	)
	if err := s.Store.TouchJob(ctx, job.ID); err != nil {
		return err
	}
	logger.Info("ingest_completed",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(insertedChunks)),
		slog.Duration("duration", time.Since(startedAt)),
	)
	return nil
}

type Bundle struct {
	Version  domain.DocumentVersion
	Document domain.Document
	Asset    domain.DocumentAsset
	DocType  domain.DocType
	Storage  Storage
}

type Storage interface {
	Read(path string) (string, error)
}

func (b Bundle) AssetContent() (string, error) {
	return b.Storage.Read(b.Asset.StoragePath)
}

func decodeForm(b []byte) (schema.DocTypeForm, error) {
	var form schema.DocTypeForm
	if err := json.Unmarshal(b, &form); err != nil {
		return form, err
	}
	form = form.AlignMappingRules()
	if err := form.Validate(); err != nil {
		return form, err
	}
	return form, nil
}

func normalize(in string, mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "none":
		return strings.TrimSpace(strings.ReplaceAll(in, "\r", ""))
	default:
		return normalizeLegalText(in)
	}
}

func segment(text, strategy string) []string {
	switch strategy {

	case "legal_article":
		return splitByRegex(text, `(?m)^\s*Điều\s+\d+`)

	case "judgement_structure":
		return splitByRegex(text,
			`(?mi)^\s*(?:[IVXLC]+\.\s*)?(?:PHẦN\s+)?(NỘI DUNG VỤ ÁN|QUÁ TRÌNH TỐ TỤNG|NHẬN ĐỊNH CỦA TÒA ÁN|TÒA ÁN NHẬN ĐỊNH|VÌ CÁC LẼ TRÊN|QUYẾT ĐỊNH)\b`,
		)

	default:
		return splitByParagraph(text)
	}
}

func splitByRegex(text, pattern string) []string {
	re := regexp.MustCompile(pattern)
	locs := re.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return []string{text}
	}
	var out []string
	for i := 0; i < len(locs); i++ {
		start := locs[i][0]
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, strings.TrimSpace(text[start:end]))
	}
	return out
}

func splitByParagraph(text string) []string {
	parts := strings.Split(text, "\n\n")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func chunkSegments(segments []string, size, overlap int) []string {
	if size <= 0 {
		size = 800
	}
	if overlap < 0 {
		overlap = 0
	}
	var chunks []string
	for _, seg := range segments {
		words := strings.Fields(seg)
		for start := 0; start < len(words); {
			end := start + size
			if end > len(words) {
				end = len(words)
			}
			chunk := strings.Join(words[start:end], " ")
			chunks = append(chunks, chunk)
			if end == len(words) {
				break
			}
			start = end - overlap
			if start < 0 {
				start = 0
			}
		}
	}
	return chunks
}

func (s *Service) generateChunks(documentID, versionID, normalized string, segmentRules schema.SegmentRules, metadata map[string]interface{}) ([]generatedChunk, chunkGenerationStats, error) {
	if segmentRules.Strategy == "legal_article" {
		generator, err := newLegalChunkGenerator(segmentRules)
		if err != nil {
			return nil, chunkGenerationStats{}, err
		}
		return generator.Generate(documentID, versionID, normalized, metadata)
	}

	segments := segment(normalized, segmentRules.Strategy)
	chunks := chunkSegments(segments, s.Config.ChunkSize, s.Config.ChunkOverlap)
	out := make([]generatedChunk, 0, len(chunks))
	maxTokens := 0
	totalTokens := 0
	builder := chunkMetadataBuilder{}
	for idx, text := range chunks {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		tokens := estimateTokenCount(text)
		if tokens > hardAbortChunkTokens {
			return nil, chunkGenerationStats{}, fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
		}
		metaRaw, metaMap, err := builder.Build(metadata, documentID, versionID, idx, newStructuralPath(nil))
		if err != nil {
			return nil, chunkGenerationStats{}, err
		}
		out = append(out, generatedChunk{
			Index:    idx,
			Text:     text,
			Tokens:   tokens,
			Metadata: metaRaw,
			MetaMap:  metaMap,
		})
		totalTokens += tokens
		if tokens > maxTokens {
			maxTokens = tokens
		}
	}
	stats := chunkGenerationStats{ChunkCount: len(out), MaxChunkTokens: maxTokens}
	if len(out) > 0 {
		stats.AvgChunkTokens = totalTokens / len(out)
	}
	return out, stats, nil
}

func extractMetadata(text string, rules []schema.MappingRule) map[string]interface{} {
	meta := map[string]interface{}{}
	for _, rule := range rules {
		if rule.Regex == "" || rule.Field == "" {
			continue
		}
		re, err := regexp.Compile(rule.Regex)
		if err != nil {
			continue
		}
		m := re.FindStringSubmatch(text)
		if len(m) > rule.Group {
			value := strings.TrimSpace(m[rule.Group])
			if len(rule.ValueMap) > 0 {
				if mapped, ok := rule.ValueMap[value]; ok {
					value = mapped
				}
			}
			meta[rule.Field] = value
		} else if rule.Default != "" {
			meta[rule.Field] = rule.Default
		}
	}
	return meta
}

func buildRetrievalPayload(meta map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if v := pickMetaString(meta, "legal_domain", "domain", "legal_field"); v != "" {
		out["legal_domain"] = legalmeta.NormalizeLegalDomain(v)
	}
	if v := pickMetaString(meta, "document_type", "doc_type", "type"); v != "" {
		out["document_type"] = legalmeta.NormalizeDocumentType(v)
	}
	if v := pickMetaString(meta, "document_number", "number", "doc_number", "so_hieu"); v != "" {
		out["document_number"] = v
	}
	if v := pickMetaString(meta, "article_number", "article", "dieu"); v != "" {
		out["article_number"] = v
	}
	if v := pickMetaString(meta, "effective_status", "status", "hieu_luc"); v != "" {
		out["effective_status"] = legalmeta.NormalizeEffectiveStatus(v)
	}
	if v := pickMetaString(meta, "issuing_authority", "authority", "co_quan_ban_hanh"); v != "" {
		out["issuing_authority"] = v
	}
	if year := pickMetaInt(meta, "signed_year", "year", "nam_ban_hanh"); year > 0 {
		out["signed_year"] = year
	}
	return out
}

func pickMetaString(meta map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		raw, ok := meta[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				return trimmed
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			return strconv.Itoa(v)
		}
	}
	return ""
}

func pickMetaInt(meta map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		raw, ok := meta[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func citationID(versionID string, idx int, text string) string {
	h := sha256.Sum256([]byte(versionID + ":" + strconv.Itoa(idx) + ":" + text))
	return hex.EncodeToString(h[:])
}

func contentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func VectorPointID(versionID string, idx int) string {
	return chunkUUID(versionID, idx)
}

func ChunkRecordID(versionID string, idx int) string {
	return chunkUUID(versionID, idx)
}

func chunkUUID(documentVersionID string, index int) string {
	key := fmt.Sprintf("%s_%d", documentVersionID, index)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(key)).String()
}

func staleVectorIDs(versionID string, from, to int) []string {
	ids := make([]string, 0, max(0, to-from))
	for idx := from; idx < to; idx++ {
		ids = append(ids, VectorPointID(versionID, idx))
	}
	return ids
}

func (s *Service) shouldSkipIngest(ctx context.Context, versionID string, chunkCount int, contentHash, formHash string) (bool, error) {
	existingChunkCount, err := s.Store.CountChunksByVersion(ctx, versionID)
	if err != nil {
		return false, err
	}
	if existingChunkCount == 0 || existingChunkCount != chunkCount {
		return false, nil
	}
	payload, found, err := s.Qdrant.GetPayloadByPointID(ctx, VectorPointID(versionID, 0))
	if err != nil || !found {
		return false, err
	}
	payloadContentHash, _ := payload["content_hash"].(string)
	payloadFormHash, _ := payload["form_hash"].(string)
	if payloadContentHash != contentHash || payloadFormHash != formHash {
		return false, nil
	}
	// Guard against stale tail vectors left by a crash after DB commit and before vector cleanup.
	_, staleFound, err := s.Qdrant.GetPayloadByPointID(ctx, VectorPointID(versionID, chunkCount))
	if err != nil {
		return false, err
	}
	return !staleFound, nil
}

func (s *Service) recoverPartialIngest(ctx context.Context, versionID string) error {
	existingChunkCount, err := s.Store.CountChunksByVersion(ctx, versionID)
	if err != nil {
		return err
	}
	if existingChunkCount == 0 {
		return nil
	}
	_, found, err := s.Qdrant.GetPayloadByPointID(ctx, VectorPointID(versionID, 0))
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	s.logger().Warn("orphan_chunks_detected_cleaning", slog.String("document_version_id", versionID), slog.Int("chunk_count", existingChunkCount))
	return s.Store.DeleteChunksByVersion(ctx, versionID)
}

func (s *Service) embedInBatches(ctx context.Context, chunks []string, batchSize int) ([][]float64, error) {
	if batchSize <= 0 {
		batchSize = len(chunks)
	}
	vectors := make([][]float64, 0, len(chunks))
	for start := 0; start < len(chunks); start += batchSize {
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch, err := s.Embed.Embed(ctx, chunks[start:end])
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, batch...)
	}
	return vectors, nil
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
