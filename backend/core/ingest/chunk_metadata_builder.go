package ingest

import "encoding/json"

type chunkMetadataBuilder struct{}

type chunkLocation struct {
	Chapter string
	Article string
	Clause  string
	Point   string
}

func (b chunkMetadataBuilder) Build(base map[string]interface{}, documentID, versionID string, chunkIndex int, loc chunkLocation) ([]byte, map[string]interface{}, error) {
	documentMeta := make(map[string]interface{}, len(base))
	for k, v := range base {
		documentMeta[k] = v
	}
	structuralMeta := map[string]interface{}{}
	if loc.Chapter != "" {
		structuralMeta["chapter"] = loc.Chapter
	}
	if loc.Article != "" {
		structuralMeta["article"] = loc.Article
	}
	if loc.Clause != "" {
		structuralMeta["clause"] = loc.Clause
	}
	if loc.Point != "" {
		structuralMeta["point"] = loc.Point
	}
	systemMeta := map[string]interface{}{
		"document_id":         documentID,
		"document_version_id": versionID,
		"chunk_index":         chunkIndex,
	}
	wire := map[string]interface{}{
		"document":   documentMeta,
		"structural": structuralMeta,
		"system":     systemMeta,
	}
	flat := make(map[string]interface{}, len(documentMeta)+len(structuralMeta)+len(systemMeta))
	for k, v := range documentMeta {
		flat[k] = v
	}
	for k, v := range structuralMeta {
		flat[k] = v
	}
	for k, v := range systemMeta {
		flat[k] = v
	}
	raw, err := json.Marshal(wire)
	return raw, flat, err
}
