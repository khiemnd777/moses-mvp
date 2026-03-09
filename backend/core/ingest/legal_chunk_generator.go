package ingest

import (
	"fmt"
	"strings"
)

type legalChunkGenerator struct {
	parser   legalStructureParser
	splitter tokenSafeSplitter
	overlap  chunkOverlapEngine
	metadata chunkMetadataBuilder
}

type generatedChunk struct {
	Index    int
	Text     string
	Tokens   int
	Metadata []byte
	MetaMap  map[string]interface{}
}

type chunkGenerationStats struct {
	ChunkCount     int
	AvgChunkTokens int
	MaxChunkTokens int
}

func newLegalChunkGenerator() legalChunkGenerator {
	return legalChunkGenerator{
		parser: legalStructureParser{},
		splitter: tokenSafeSplitter{
			maxTokens:    defaultMaxChunkTokens,
			targetTokens: defaultTargetChunkTokens,
		},
		overlap:  chunkOverlapEngine{overlapTokens: defaultOverlapTokens},
		metadata: chunkMetadataBuilder{},
	}
}

func (g legalChunkGenerator) Generate(documentID, versionID, text string, baseMetadata map[string]interface{}) ([]generatedChunk, chunkGenerationStats, error) {
	doc := g.parser.Parse(text)
	chunks := make([]generatedChunk, 0)
	maxTokens := 0
	totalTokens := 0

	appendChunk := func(loc chunkLocation, text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		tokens := estimateTokenCount(text)
		if tokens > hardAbortChunkTokens {
			return fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
		}
		idx := len(chunks)
		metaRaw, metaMap, err := g.metadata.Build(baseMetadata, documentID, versionID, idx, loc)
		if err != nil {
			return err
		}
		chunks = append(chunks, generatedChunk{
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
		return nil
	}

	if len(doc.Articles) == 1 && doc.Articles[0].Number == "" {
		if err := g.appendSplitChunks(
			&chunks,
			&totalTokens,
			&maxTokens,
			baseMetadata,
			documentID,
			versionID,
			"RAW_DOCUMENT",
			"",
			"RAW_DOCUMENT",
			[]chunkPart{{Text: doc.Articles[0].Content}},
		); err != nil {
			return nil, chunkGenerationStats{}, err
		}
		stats := chunkGenerationStats{
			ChunkCount:     len(chunks),
			MaxChunkTokens: maxTokens,
		}
		if len(chunks) > 0 {
			stats.AvgChunkTokens = totalTokens / len(chunks)
		}
		return chunks, stats, nil
	}

	for _, article := range doc.Articles {
		if len(article.Clauses) == 0 {
			if err := appendChunk(chunkLocation{Article: article.Number}, strings.TrimSpace(joinChunkSections(article.Header, article.Content))); err != nil {
				return nil, chunkGenerationStats{}, err
			}
			continue
		}
		for _, clause := range article.Clauses {
			prefix := joinChunkSections(article.Header, clauseLabel(clause.Number))
			clauseText := strings.TrimSpace(joinChunkSections(prefix, clause.Content))
			if tokens := estimateTokenCount(clauseText); tokens <= defaultMaxChunkTokens {
				if err := appendChunk(chunkLocation{Article: article.Number, Clause: clause.Number}, clauseText); err != nil {
					return nil, chunkGenerationStats{}, err
				}
				continue
			}
			if len(clause.Points) > 0 {
				pointParts := make([]chunkPart, 0, len(clause.Points))
				for _, point := range clause.Points {
					pointParts = append(pointParts, chunkPart{
						Text:  strings.TrimSpace(pointLabel(point.Marker) + "\n" + point.Content),
						Point: point.Marker,
					})
				}
				if err := g.appendSplitChunks(&chunks, &totalTokens, &maxTokens, baseMetadata, documentID, versionID, article.Number, clause.Number, prefix, pointParts); err != nil {
					return nil, chunkGenerationStats{}, err
				}
				continue
			}

			if err := g.appendSplitChunks(&chunks, &totalTokens, &maxTokens, baseMetadata, documentID, versionID, article.Number, clause.Number, prefix, []chunkPart{{Text: clause.Content}}); err != nil {
				return nil, chunkGenerationStats{}, err
			}
		}
	}

	stats := chunkGenerationStats{
		ChunkCount:     len(chunks),
		MaxChunkTokens: maxTokens,
	}
	if len(chunks) > 0 {
		stats.AvgChunkTokens = totalTokens / len(chunks)
	}
	return chunks, stats, nil
}

func (g legalChunkGenerator) appendSplitChunks(
	chunks *[]generatedChunk,
	totalTokens *int,
	maxTokens *int,
	baseMetadata map[string]interface{},
	documentID, versionID, articleNumber, clauseNumber, prefix string,
	parts []chunkPart,
) error {
	expanded := make([]chunkPart, 0)
	for _, part := range parts {
		split, err := g.splitter.Split(part.Text, part.Point)
		if err != nil {
			return err
		}
		expanded = append(expanded, split...)
	}
	expanded = g.overlap.Apply(expanded)
	for _, part := range expanded {
		text := strings.TrimSpace(joinChunkSections(prefix, part.Text))
		tokens := estimateTokenCount(text)
		if tokens > hardAbortChunkTokens {
			return fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
		}
		if tokens > defaultMaxChunkTokens {
			return fmt.Errorf("unable to enforce chunk token safety: estimated_tokens=%d limit=%d", tokens, defaultMaxChunkTokens)
		}
		idx := len(*chunks)
		metaRaw, metaMap, err := g.metadata.Build(baseMetadata, documentID, versionID, idx, chunkLocation{
			Article: articleNumber,
			Clause:  clauseNumber,
			Point:   part.Point,
		})
		if err != nil {
			return err
		}
		*chunks = append(*chunks, generatedChunk{
			Index:    idx,
			Text:     text,
			Tokens:   tokens,
			Metadata: metaRaw,
			MetaMap:  metaMap,
		})
		*totalTokens += tokens
		if tokens > *maxTokens {
			*maxTokens = tokens
		}
	}
	return nil
}

func joinChunkSections(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func clauseLabel(number string) string {
	number = strings.TrimSpace(number)
	if number == "" {
		return ""
	}
	return "Khoản " + number
}

func pointLabel(marker string) string {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return ""
	}
	return "Điểm " + marker
}
