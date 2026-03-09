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

	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type Config struct {
	ChunkSize    int
	ChunkOverlap int
}

type Service struct {
	Store  *infra.Store
	Qdrant *infra.QdrantClient
	Embed  Embedder
	Config Config
	Logger *slog.Logger
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
	normalized := normalize(content)
	contentHash := contentHash(normalized)
	segments := segment(normalized, form.SegmentRules.Strategy)
	chunks := chunkSegments(segments, s.Config.ChunkSize, s.Config.ChunkOverlap)
	if len(chunks) == 0 {
		return errors.New("no chunks produced")
	}
	metadata := extractMetadata(normalized, form.MappingRules)

	logger.Info("ingest_started",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(chunks)),
	)
	if err := s.Store.TouchJob(ctx, job.ID); err != nil {
		return err
	}

	shouldSkip, err := s.shouldSkipIngest(ctx, bundle.Version.ID, len(chunks), contentHash, formHash)
	if err != nil {
		return err
	}
	if shouldSkip {
		logger.Info("ingest_completed",
			slog.Int("attempt", attempt),
			slog.Int("chunk_count", len(chunks)),
			slog.Duration("duration", time.Since(startedAt)),
			slog.Bool("skipped", true),
		)
		return nil
	}

	logger.Info("chunks_created",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(chunks)),
	)

	vectors, err := s.embedInBatches(ctx, chunks, 32)
	if err != nil {
		return err
	}
	if len(vectors) != len(chunks) {
		return errors.New("embedding vector count mismatch")
	}
	logger.Info("embedding_done",
		slog.Int("attempt", attempt),
		slog.Int("chunk_count", len(chunks)),
	)
	if err := s.Store.TouchJob(ctx, job.ID); err != nil {
		return err
	}

	oldChunkCount, err := s.Store.CountChunksByVersion(ctx, bundle.Version.ID)
	if err != nil {
		return err
	}

	replacement := make([]domain.Chunk, 0, len(chunks))
	for i, text := range chunks {
		metaJSON, _ := json.Marshal(metadata)
		embedJSON, _ := json.Marshal(vectors[i])
		replacement = append(replacement, domain.Chunk{
			DocumentVersionID: bundle.Version.ID,
			Index:             i,
			Text:              text,
			MetadataJSON:      metaJSON,
			EmbeddingJSON:     embedJSON,
		})
	}

	insertedChunks, err := s.Store.ReplaceChunks(ctx, bundle.Version.ID, replacement)
	if err != nil {
		return err
	}

	points := make([]infra.PointInput, 0, len(insertedChunks))
	for _, chunk := range insertedChunks {
		vectorID := VectorPointID(bundle.Version.ID, chunk.Index)
		points = append(points, infra.PointInput{
			ID:     vectorID,
			Vector: vectors[chunk.Index],
			Payload: map[string]interface{}{
				"chunk_id":            chunk.ID,
				"document_version_id": bundle.Version.ID,
				"chunk_index":         chunk.Index,
				"citation_id":         citationID(bundle.Version.ID, chunk.Index, chunk.Text),
				"content_hash":        contentHash,
				"form_hash":           formHash,
			},
		})
	}
	if err := s.Qdrant.Upsert(ctx, points); err != nil {
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

func normalize(in string) string {
	return strings.TrimSpace(strings.ReplaceAll(in, "\r", ""))
}

func segment(text, strategy string) []string {
	switch strategy {
	case "legal_article":
		return splitByRegex(text, `(?m)^Article\s+\d+`)
	case "judgement_structure":
		return splitByRegex(text, `(?m)^\s*(Facts|Reasoning|Decision)\b`)
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

func citationID(versionID string, idx int, text string) string {
	h := sha256.Sum256([]byte(versionID + ":" + strconv.Itoa(idx) + ":" + text))
	return hex.EncodeToString(h[:])
}

func contentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func VectorPointID(versionID string, idx int) string {
	return fmt.Sprintf("doc_%s_chunk_%04d", versionID, idx)
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
	return payloadContentHash == contentHash && payloadFormHash == formHash, nil
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
