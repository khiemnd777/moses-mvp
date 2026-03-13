package domain

import (
	"encoding/json"
	"testing"
)

func TestAIGuardPolicyJSONSnakeCase(t *testing.T) {
	input := []byte(`{
		"name":"legal_balanced_guard_policy",
		"enabled":true,
		"min_retrieved_chunks":1,
		"min_similarity_score":0.45,
		"on_empty_retrieval":"ask_clarification",
		"on_low_confidence":"fallback_llm"
	}`)

	var got AIGuardPolicy
	if err := json.Unmarshal(input, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got.Name != "legal_balanced_guard_policy" {
		t.Fatalf("unexpected name: %q", got.Name)
	}
	if !got.Enabled {
		t.Fatalf("enabled should be true")
	}
	if got.MinRetrievedChunks != 1 {
		t.Fatalf("unexpected min_retrieved_chunks: %d", got.MinRetrievedChunks)
	}
	if got.MinSimilarityScore != 0.45 {
		t.Fatalf("unexpected min_similarity_score: %v", got.MinSimilarityScore)
	}
	if got.OnEmptyRetrieval != "ask_clarification" {
		t.Fatalf("unexpected on_empty_retrieval: %q", got.OnEmptyRetrieval)
	}
	if got.OnLowConfidence != "fallback_llm" {
		t.Fatalf("unexpected on_low_confidence: %q", got.OnLowConfidence)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal marshaled payload failed: %v", err)
	}

	if _, ok := out["min_retrieved_chunks"]; !ok {
		t.Fatalf("missing min_retrieved_chunks in marshaled json: %s", string(raw))
	}
	if _, ok := out["on_empty_retrieval"]; !ok {
		t.Fatalf("missing on_empty_retrieval in marshaled json: %s", string(raw))
	}
	if _, ok := out["on_low_confidence"]; !ok {
		t.Fatalf("missing on_low_confidence in marshaled json: %s", string(raw))
	}
}

