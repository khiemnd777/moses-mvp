package ingest

import "encoding/json"

type chunkMetadataBuilder struct{}

type chunkMetadataContext struct {
	DocumentID        string
	DocumentVersionID string
	AssetID           string
	DocTypeCode       string
	DocumentVersion   int
	PipelineVersion   string
}

const currentIngestPipelineVersion = "ingest-v1"

func defaultChunkMetadataContext(documentID, versionID string) chunkMetadataContext {
	return chunkMetadataContext{
		DocumentID:        documentID,
		DocumentVersionID: versionID,
		PipelineVersion:   currentIngestPipelineVersion,
	}
}

func (b chunkMetadataBuilder) Build(base map[string]interface{}, documentID, versionID string, chunkIndex int, path structuralPath) ([]byte, map[string]interface{}, error) {
	return b.BuildWithContext(base, defaultChunkMetadataContext(documentID, versionID), chunkIndex, path)
}

func (b chunkMetadataBuilder) BuildWithContext(base map[string]interface{}, ctx chunkMetadataContext, chunkIndex int, path structuralPath) ([]byte, map[string]interface{}, error) {
	documentMeta := make(map[string]interface{}, len(base))
	for k, v := range base {
		documentMeta[k] = v
	}
	structuralMeta := path.StructuralMap()
	systemMeta := map[string]interface{}{
		"document_id":         ctx.DocumentID,
		"document_version_id": ctx.DocumentVersionID,
		"chunk_index":         chunkIndex,
		"pipeline_version":    nonEmpty(ctx.PipelineVersion, currentIngestPipelineVersion),
	}
	if ctx.AssetID != "" {
		systemMeta["asset_id"] = ctx.AssetID
	}
	if ctx.DocTypeCode != "" {
		systemMeta["doc_type_code"] = ctx.DocTypeCode
	}
	if ctx.DocumentVersion > 0 {
		systemMeta["document_version"] = ctx.DocumentVersion
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
	if v, ok := structuralMeta["article"]; ok {
		flat["article_number"] = v
	}
	if v, ok := structuralMeta["clause"]; ok {
		flat["clause_number"] = v
	}
	if v, ok := structuralMeta["point"]; ok {
		flat["point_marker"] = v
	}
	for k, v := range flat {
		wire[k] = v
	}
	raw, err := json.Marshal(wire)
	return raw, flat, err
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
