package ingest

import "encoding/json"

type chunkMetadataBuilder struct{}

func (b chunkMetadataBuilder) Build(base map[string]interface{}, documentID, versionID string, chunkIndex int, path structuralPath) ([]byte, map[string]interface{}, error) {
	documentMeta := make(map[string]interface{}, len(base))
	for k, v := range base {
		documentMeta[k] = v
	}
	structuralMeta := path.StructuralMap()
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
