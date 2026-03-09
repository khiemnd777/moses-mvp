CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_guard_policies_single_enabled
ON ai_guard_policies ((enabled))
WHERE enabled = TRUE;

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_prompts_single_enabled_per_type
ON ai_prompts (prompt_type)
WHERE enabled = TRUE;
