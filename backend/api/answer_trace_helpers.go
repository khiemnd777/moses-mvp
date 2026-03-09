package api

import "github.com/khiemnd777/legal_api/core/retrieval"

func traceChunkIDs(results []retrieval.Result) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		if result.ChunkID == "" {
			continue
		}
		out = append(out, result.ChunkID)
	}
	return out
}
