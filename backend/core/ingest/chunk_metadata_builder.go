package ingest

import "encoding/json"

type chunkMetadataBuilder struct{}

type chunkLocation struct {
	Article string
	Clause  string
	Point   string
}

func (b chunkMetadataBuilder) Build(base map[string]interface{}, documentID, versionID string, chunkIndex int, loc chunkLocation) ([]byte, map[string]interface{}, error) {
	meta := make(map[string]interface{}, len(base)+6)
	for k, v := range base {
		meta[k] = v
	}
	meta["document_id"] = documentID
	meta["document_version_id"] = versionID
	meta["chunk_index"] = chunkIndex
	if loc.Article != "" {
		meta["article"] = loc.Article
	}
	if loc.Clause != "" {
		meta["clause"] = loc.Clause
	}
	if loc.Point != "" {
		meta["point"] = loc.Point
	}
	raw, err := json.Marshal(meta)
	return raw, meta, err
}
