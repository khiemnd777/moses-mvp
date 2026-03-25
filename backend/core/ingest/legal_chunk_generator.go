package ingest

import (
	"fmt"
	"strings"

	"github.com/khiemnd777/legal_api/core/schema"
)

type legalChunkGenerator struct {
	plan     segmentPlan
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

func newLegalChunkGenerator(rules schema.SegmentRules) (legalChunkGenerator, error) {
	plan, err := compileSegmentPlan(rules)
	if err != nil {
		return legalChunkGenerator{}, err
	}
	parser, err := newLegalStructureParser(rules)
	if err != nil {
		return legalChunkGenerator{}, err
	}
	return legalChunkGenerator{
		plan:   plan,
		parser: parser,
		splitter: tokenSafeSplitter{
			maxTokens:    defaultMaxChunkTokens,
			targetTokens: defaultTargetChunkTokens,
		},
		overlap:  chunkOverlapEngine{overlapTokens: defaultOverlapTokens},
		metadata: chunkMetadataBuilder{},
	}, nil
}

func (g legalChunkGenerator) Generate(documentID, versionID, text string, baseMetadata map[string]interface{}) ([]generatedChunk, chunkGenerationStats, error) {
	doc := g.parser.Parse(text)
	chunks := make([]generatedChunk, 0)
	maxTokens := 0
	totalTokens := 0

	appendChunk := func(path structuralPath, text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		tokens := estimateTokenCount(text)
		if tokens > hardAbortChunkTokens {
			return fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
		}
		idx := len(chunks)
		metaRaw, metaMap, err := g.metadata.Build(baseMetadata, documentID, versionID, idx, path)
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

	baseDepth := 0
	if len(g.plan.Levels) > 1 {
		baseDepth = len(g.plan.Levels) - 2
	}

	var walk func(node segmentNode, depth int, ancestorHeaders []string) error
	walk = func(node segmentNode, depth int, ancestorHeaders []string) error {
		headers := ancestorHeaders
		if header := chunkContextHeader(node, depth); header != "" {
			headers = append(headers, header)
		}
		if depth < baseDepth && len(node.Children) > 0 {
			for _, child := range node.Children {
				if err := walk(child, depth+1, headers); err != nil {
					return err
				}
			}
			return nil
		}

		fullText := strings.TrimSpace(joinChunkSections(append(append([]string(nil), headers...), node.Content)...))
		if len(node.Children) > 0 && depth == baseDepth && estimateTokenCount(fullText) > defaultMaxChunkTokens {
			parts := make([]chunkPart, 0, len(node.Children))
			for _, child := range node.Children {
				childText := strings.TrimSpace(joinChunkSections(chunkContextHeader(child, depth+1), child.Content))
				if childText == "" {
					continue
				}
				parts = append(parts, chunkPart{
					Text: childText,
					Path: child.Path,
				})
			}
			if len(parts) > 0 {
				return g.appendSplitChunks(&chunks, &totalTokens, &maxTokens, baseMetadata, documentID, versionID, headers, node.Path, parts)
			}
		}

		if estimateTokenCount(fullText) > defaultMaxChunkTokens {
			splitParts, err := g.splitter.Split(node.Content, node.Path)
			if err != nil {
				return err
			}
			return g.appendSplitChunks(&chunks, &totalTokens, &maxTokens, baseMetadata, documentID, versionID, headers, node.Path, splitParts)
		}
		return appendChunk(node.Path, fullText)
	}

	for _, node := range doc.Nodes {
		if err := walk(node, 0, nil); err != nil {
			return nil, chunkGenerationStats{}, err
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
	documentID, versionID string,
	headers []string,
	basePath structuralPath,
	parts []chunkPart,
) error {
	expanded := make([]chunkPart, 0)
	for _, part := range parts {
		if part.Path.values == nil {
			part.Path = basePath
		}
		split, err := g.splitter.Split(part.Text, part.Path)
		if err != nil {
			return err
		}
		expanded = append(expanded, split...)
	}
	expanded = g.overlap.Apply(expanded)
	for _, part := range expanded {
		text := strings.TrimSpace(joinChunkSections(append(append([]string(nil), headers...), part.Text)...))
		tokens := estimateTokenCount(text)
		if tokens > hardAbortChunkTokens {
			return fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
		}
		if tokens > defaultMaxChunkTokens {
			return fmt.Errorf("unable to enforce chunk token safety: estimated_tokens=%d limit=%d", tokens, defaultMaxChunkTokens)
		}
		idx := len(*chunks)
		metaRaw, metaMap, err := g.metadata.Build(baseMetadata, documentID, versionID, idx, part.Path)
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

func chunkContextHeader(node segmentNode, depth int) string {
	header := strings.TrimSpace(node.Header)
	if depth == 0 || node.Level == "" {
		return header
	}
	value := strings.TrimSpace(node.Value)
	if value == "" {
		return header
	}
	switch node.Level {
	case "clause":
		return "Khoản " + value
	case "point":
		return "Điểm " + value
	default:
		if header != "" {
			return header
		}
		return value
	}
}
