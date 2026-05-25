INSERT INTO ai_guard_policies (
  name, enabled, min_retrieved_chunks, min_similarity_score, on_empty_retrieval, on_low_confidence
)
VALUES (
  'production_balanced_vietnamese_guard',
  FALSE,
  1,
  0.25,
  'ask_clarification',
  'ask_clarification'
)
ON CONFLICT (name) DO UPDATE
SET min_retrieved_chunks = EXCLUDED.min_retrieved_chunks,
    min_similarity_score = EXCLUDED.min_similarity_score,
    on_empty_retrieval = EXCLUDED.on_empty_retrieval,
    on_low_confidence = EXCLUDED.on_low_confidence,
    updated_at = NOW();

DO $$
DECLARE
  should_activate BOOLEAN;
BEGIN
  SELECT
    NOT EXISTS (SELECT 1 FROM ai_guard_policies WHERE enabled = TRUE)
    OR EXISTS (
      SELECT 1
      FROM ai_guard_policies
      WHERE enabled = TRUE
        AND name = 'default_legal_guard_policy'
        AND min_similarity_score >= 0.69
        AND on_empty_retrieval = 'refuse'
    )
  INTO should_activate;

  IF should_activate THEN
    UPDATE ai_guard_policies
    SET enabled = FALSE, updated_at = NOW()
    WHERE enabled = TRUE;

    UPDATE ai_guard_policies
    SET enabled = TRUE, updated_at = NOW()
    WHERE name = 'production_balanced_vietnamese_guard';
  END IF;
END $$;

INSERT INTO ai_prompts (name, prompt_type, system_prompt, temperature, max_tokens, retry, enabled)
VALUES
(
  'production_legal_guard_vi',
  'legal_guard',
  $prompt$Bạn là lớp điều phối an toàn cho trợ lý pháp lý Việt Nam. Cho phép trả lời khi có ít nhất một nguồn liên quan; nếu nguồn rỗng hoặc lệch chủ đề thì yêu cầu làm rõ. Không kết luận pháp lý khi không có căn cứ truy xuất.$prompt$,
  0.1,
  300,
  0,
  FALSE
),
(
  'production_legal_answer_vi',
  'legal_answer',
  $prompt$Bạn là trợ lý pháp lý Việt Nam trong môi trường production.

Nguyên tắc trả lời:
- Trả lời khi Retrieved Legal Context có nguồn liên quan; không từ chối chỉ vì câu hỏi rộng.
- Chỉ nêu căn cứ, điều kiện, thủ tục hoặc kết luận có thể đối chiếu từ nguồn truy xuất.
- Không tự tạo số điều, tên văn bản, ngày hiệu lực, án lệ hoặc sự kiện không có trong nguồn.
- Nếu nguồn chỉ trả lời được một phần, nói rõ phần nào có căn cứ và phần nào cần thêm dữ kiện.
- Với câu hỏi mở đầu như ly hôn, đất đai, lao động, hãy đưa định hướng thực tế, các điểm cần kiểm tra và câu hỏi tiếp theo.
- Luôn trả lời bằng tiếng Việt tự nhiên, rõ ràng, có cấu trúc 1-4 mục, và gắn nhận định với nhãn nguồn/trích dẫn khi có.$prompt$,
  0.2,
  1600,
  1,
  FALSE
),
(
  'production_legal_refusal_vi',
  'legal_refusal',
  $prompt$Mình chưa thấy căn cứ pháp lý phù hợp trong tài liệu đã truy xuất. Bạn có thể bổ sung văn bản, điều khoản, thời điểm hoặc tình huống cụ thể để mình kiểm tra tiếp.$prompt$,
  0.1,
  120,
  0,
  FALSE
),
(
  'production_legal_clarification_vi',
  'legal_clarification',
  $prompt$Mình có thể hỗ trợ, nhưng cần thêm vài chi tiết như quan hệ tranh chấp, thời điểm, giấy tờ hoặc điều khoản bạn muốn kiểm tra để đối chiếu đúng nguồn.$prompt$,
  0.1,
  140,
  0,
  FALSE
),
(
  'production_smalltalk_vi',
  'smalltalk',
  $prompt$Chào bạn, mình có thể giúp tra cứu văn bản, tóm tắt căn cứ và phân tích tình huống pháp lý Việt Nam dựa trên tài liệu đã ingest. Bạn cứ mô tả vụ việc hoặc hỏi văn bản cần kiểm tra.$prompt$,
  0.2,
  160,
  0,
  FALSE
)
ON CONFLICT (name) DO UPDATE
SET prompt_type = EXCLUDED.prompt_type,
    system_prompt = EXCLUDED.system_prompt,
    temperature = EXCLUDED.temperature,
    max_tokens = EXCLUDED.max_tokens,
    retry = EXCLUDED.retry,
    updated_at = NOW();

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_guard' AND enabled = TRUE)
     OR EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_guard' AND enabled = TRUE AND name = 'legal_guard_prompt') THEN
    UPDATE ai_prompts SET enabled = FALSE, updated_at = NOW() WHERE prompt_type = 'legal_guard' AND enabled = TRUE;
    UPDATE ai_prompts SET enabled = TRUE, updated_at = NOW() WHERE name = 'production_legal_guard_vi';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_answer' AND enabled = TRUE)
     OR EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_answer' AND enabled = TRUE AND name = 'legal_answer_prompt') THEN
    UPDATE ai_prompts SET enabled = FALSE, updated_at = NOW() WHERE prompt_type = 'legal_answer' AND enabled = TRUE;
    UPDATE ai_prompts SET enabled = TRUE, updated_at = NOW() WHERE name = 'production_legal_answer_vi';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_refusal' AND enabled = TRUE)
     OR EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_refusal' AND enabled = TRUE AND name = 'legal_refusal_prompt') THEN
    UPDATE ai_prompts SET enabled = FALSE, updated_at = NOW() WHERE prompt_type = 'legal_refusal' AND enabled = TRUE;
    UPDATE ai_prompts SET enabled = TRUE, updated_at = NOW() WHERE name = 'production_legal_refusal_vi';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_clarification' AND enabled = TRUE)
     OR EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'legal_clarification' AND enabled = TRUE AND name = 'legal_clarification_prompt') THEN
    UPDATE ai_prompts SET enabled = FALSE, updated_at = NOW() WHERE prompt_type = 'legal_clarification' AND enabled = TRUE;
    UPDATE ai_prompts SET enabled = TRUE, updated_at = NOW() WHERE name = 'production_legal_clarification_vi';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM ai_prompts WHERE prompt_type = 'smalltalk' AND enabled = TRUE) THEN
    UPDATE ai_prompts SET enabled = TRUE, updated_at = NOW() WHERE name = 'production_smalltalk_vi';
  END IF;
END $$;

INSERT INTO ai_retrieval_configs (
  name,
  enabled,
  default_top_k,
  rerank_enabled,
  rerank_vector_weight,
  rerank_keyword_weight,
  rerank_metadata_weight,
  rerank_article_weight,
  adjacent_chunk_enabled,
  adjacent_chunk_window,
  max_context_chunks,
  max_context_chars,
  default_effective_status,
  preferred_doc_types_json,
  legal_domain_defaults_json
)
VALUES (
  'production_balanced_vietnamese_retrieval',
  FALSE,
  8,
  TRUE,
  0.45,
  0.30,
  0.15,
  0.10,
  TRUE,
  1,
  16,
  30000,
  'active',
  '["law", "resolution", "decree", "circular"]'::jsonb,
  '{
    "marriage_family": {"top_k": 10, "preferred_doc_types": ["law", "resolution", "decree"]},
    "civil": {"top_k": 10, "preferred_doc_types": ["law", "decree", "circular"]},
    "land": {"top_k": 10, "preferred_doc_types": ["law", "decree", "circular"]},
    "labor": {"top_k": 10, "preferred_doc_types": ["law", "decree", "circular"]},
    "criminal_law": {"top_k": 10, "preferred_doc_types": ["law", "resolution", "decree"]}
  }'::jsonb
)
ON CONFLICT (name) DO UPDATE
SET default_top_k = EXCLUDED.default_top_k,
    rerank_enabled = EXCLUDED.rerank_enabled,
    rerank_vector_weight = EXCLUDED.rerank_vector_weight,
    rerank_keyword_weight = EXCLUDED.rerank_keyword_weight,
    rerank_metadata_weight = EXCLUDED.rerank_metadata_weight,
    rerank_article_weight = EXCLUDED.rerank_article_weight,
    adjacent_chunk_enabled = EXCLUDED.adjacent_chunk_enabled,
    adjacent_chunk_window = EXCLUDED.adjacent_chunk_window,
    max_context_chunks = EXCLUDED.max_context_chunks,
    max_context_chars = EXCLUDED.max_context_chars,
    default_effective_status = EXCLUDED.default_effective_status,
    preferred_doc_types_json = EXCLUDED.preferred_doc_types_json,
    legal_domain_defaults_json = EXCLUDED.legal_domain_defaults_json,
    updated_at = NOW();

DO $$
DECLARE
  should_activate BOOLEAN;
BEGIN
  SELECT
    NOT EXISTS (SELECT 1 FROM ai_retrieval_configs WHERE enabled = TRUE)
    OR EXISTS (
      SELECT 1
      FROM ai_retrieval_configs
      WHERE enabled = TRUE
        AND name = 'default_legal_retrieval_config'
        AND default_top_k <= 5
        AND max_context_chars <= 12000
    )
  INTO should_activate;

  IF should_activate THEN
    UPDATE ai_retrieval_configs
    SET enabled = FALSE, updated_at = NOW()
    WHERE enabled = TRUE;

    UPDATE ai_retrieval_configs
    SET enabled = TRUE, updated_at = NOW()
    WHERE name = 'production_balanced_vietnamese_retrieval';
  END IF;
END $$;
