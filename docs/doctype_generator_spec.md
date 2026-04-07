# DocType Generator Specification

## Purpose

This document defines the current-project specification for using ChatGPT to analyze uploaded Vietnamese legal files and propose the correct:

- `DocType`
- `DocType form JSON`
- `Document`
- `Asset` and `Version` placeholders for ingest orchestration

This spec is aligned with the current backend implementation, not the older draft specs.

## Source Of Truth

Use these files as the authoritative reference:

- `backend/core/schema/doc_type_form.go`
- `backend/domain/models.go`
- `backend/api/handlers.go`
- `backend/infra/migrations/001_init.sql`
- `backend/infra/migrations/011_doc_type_query_profiles.sql`
- `backend/core/ingest/ingest.go`
- `backend/core/ingest/segment_plan.go`
- `backend/core/ingest/legal_structure_parser.go`
- `backend/core/ingest/legal_chunk_generator.go`
- `backend/core/ingest/chunk_metadata_builder.go`
- `backend/core/ingest/extractor/extractor.go`
- `backend/core/legalmeta/normalize.go`

## Current Backend Reality

The current backend data model is:

`DocType -> Document -> DocumentAsset -> DocumentVersion -> IngestJob -> Chunk`

Important consequences:

- `DocType` defines ingestion behavior.
- `Document` is only the legal record shell and belongs to exactly one `DocType`.
- Uploaded files are stored as `DocumentAsset`.
- Ingest runs against a `DocumentVersion`, not directly against `Document`.
- Chunk metadata is produced during ingest from `mapping_rules` plus structural path data.

The current API does **not** support a single native action of:

`upload file -> auto detect/create DocType -> auto create Document -> auto ingest`

Instead, the current backend flow is:

1. create or choose `DocType`
2. create `Document`
3. upload `DocumentAsset`
4. create `DocumentVersion`
5. enqueue ingest

Therefore ChatGPT should be treated as an orchestration/planning layer that proposes the correct structures before the backend API is called.

## Data Model

### DocType

Current persisted fields:

- `id`
- `code`
- `name`
- `form_json`
- `form_hash`
- `created_at`
- `updated_at`

`form_json` must match the Go struct `schema.DocTypeForm`.

### DocType Form

Current structure:

```json
{
  "version": 1,
  "doc_type": {
    "code": "string",
    "name": "string"
  },
  "segment_rules": {
    "strategy": "string",
    "hierarchy": "string",
    "normalization": "string",
    "level_patterns": {
      "level": "regex"
    }
  },
  "metadata_schema": {
    "fields": [
      { "name": "string", "type": "string" }
    ]
  },
  "mapping_rules": [
    {
      "field": "string",
      "regex": "string",
      "group": 1,
      "default": "",
      "value_map": {}
    }
  ],
  "reindex_policy": {
    "on_content_change": true,
    "on_form_change": true
  },
  "query_profile": {
    "canonical_terms": [],
    "synonym_groups": [],
    "query_signals": [],
    "intent_rules": [],
    "domain_topic_rules": [],
    "legal_signal_rules": [],
    "followup_markers": [],
    "preferred_doc_types": [],
    "routing_priority": 0
  }
}
```

Notes:

- `query_profile` is optional in the schema, but it is part of the current project architecture and seeded `DocType`s already use it.
- Any new spec for ChatGPT Sources should mention `query_profile`, otherwise the description is incomplete versus the current repo.

### DocType Validation Rules

The backend currently enforces:

- `version > 0`
- `doc_type.code` and `doc_type.name` are required
- `segment_rules.strategy` is required
- `metadata_schema.fields` must not be empty
- each metadata field must have unique `name`
- `mapping_rules` must exist
- every metadata field must have at least one mapping rule
- every mapping rule must reference an existing metadata field
- regex values in `mapping_rules` and `level_patterns` must compile
- `query_profile`, if present, must also validate

### Document

Current persisted fields:

- `id`
- `doc_type_id`
- `doc_type_code` as response projection
- `title`
- `created_at`
- `updated_at`

Important:

- `Document` does not persist metadata fields like `document_number`, `signed_date`, or `legal_domain`.
- Those values are extracted during ingest from `mapping_rules`.

### Asset And Version

Current persisted fields:

`DocumentAsset`

- `id`
- `document_id`
- `file_name`
- `content_type`
- `storage_path`
- `created_at`

`DocumentVersion`

- `id`
- `document_id`
- `asset_id`
- `version`
- `created_at`

Notes:

- API responses may project assets with `versions`, but that is not the persisted `Document` shape.
- `DocumentVersion` is the actual ingest target.

## File Support

Current ingest extraction supports:

- `.doc`
- `.docx`
- `.pdf`
- `.txt`

For `.docx`, the backend reads `word/document.xml` and extracts text before ingest.

## Ingest Semantics

During ingest, the system:

1. reads file text from storage
2. normalizes text
3. extracts metadata using `mapping_rules`
4. segments content using `segment_rules`
5. builds chunk metadata from:
   - extracted document metadata
   - structural path metadata such as `chapter`, `article`, `clause`, `point`
   - system metadata like `document_id`, `document_version_id`, `chunk_index`
6. writes chunks and vectors

Retrieval payload is normalized from chunk metadata and commonly includes:

- `legal_domain`
- `document_type`
- `document_number`
- `article_number`
- `effective_status`
- `issuing_authority`
- `signed_year`

This means ChatGPT should prefer metadata fields that are useful both for ingest and retrieval.

## Recommended ChatGPT Task

When a user uploads a Vietnamese legal file, ChatGPT should do this:

1. determine whether the file is a legal document
2. classify the legal document category
3. detect issuing authority
4. infer legal domain
5. detect legal hierarchy from the text
6. check whether an existing `DocType` is likely reusable
7. if not reusable, propose a new `DocType`
8. propose a `Document`
9. propose an `Asset` placeholder and `Version` placeholder for orchestration

## Document Classification

Recommended normalized `document_type` values:

- `law`
- `decree`
- `circular`
- `resolution`
- `judgment`
- `precedent`
- `other`

Detection hints:

- `LUẬT`, `BỘ LUẬT` -> `law`
- `NGHỊ ĐỊNH` -> `decree`
- `THÔNG TƯ` -> `circular`
- `NGHỊ QUYẾT` -> `resolution`
- `BẢN ÁN` -> `judgment`
- `ÁN LỆ` -> `precedent`

These should align with `legalmeta.NormalizeDocumentType`, which normalizes variants into canonical retrieval values.

## Legal Domain Detection

Recommended normalized `legal_domain` values should align with current aliases where possible:

- `general_legal`
- `civil`
- `marriage_family`
- `criminal_law`
- `civil_status`

The current repo normalizes common aliases into those canonical values.

If no strong signal exists, prefer `general_legal` instead of vague values like `general`.

## DocType Reuse Strategy

ChatGPT should prefer matching an existing `DocType` before creating a new one.

Use these signals:

- document class
- issuing authority
- legal domain
- hierarchy shape
- distinctive title or document number pattern

Examples of existing seeded `DocType`s in the current project include:

- `vn_marriage_family_law`
- `vn_civil_code`
- `vn_decree_marriage_family_126_2014`
- `vn_resolution_marriage_family_01_2024`
- `vn_circular_marriage_family_foreign_2015`

If the uploaded file appears to be a new concrete legal record of a known family, reuse the existing `DocType`.

If the uploaded file has materially different:

- structure
- metadata extraction pattern
- legal family
- or query behavior

then propose a new `DocType`.

## DocType Generation Rules

### Step 1: Determine `doc_type.code` and `doc_type.name`

Use a stable machine code and human-readable label.

Examples:

- `vn_marriage_family_law`
- `vn_civil_code`
- `vn_decree_land_2024`

Prefer codes that are:

- lowercase
- underscore-separated
- stable across repeated runs

### Step 2: Determine Segment Rules

For Vietnamese statutes and normative legal documents, prefer:

- `strategy: "legal_article"`

Use hierarchy based on detected structure, for example:

- `part.chapter.article.clause.point`
- `chapter.section.article.clause.point`
- `chapter.article.clause.point`
- `article.clause.point`
- `clause.point`

Do not force `paragraph` strategy for legal statutes if article-based structure is visible.  
The current backend has specialized `legal_article` parsing and structural metadata support, so this should be preferred whenever legal headings are detectable.

### Step 3: Choose Normalization

For Vietnamese legal text, prefer:

- `trim_whitespace_preserve_numbering`

This matches current seeded `DocType`s better than a generic paragraph-only approach.

### Step 4: Define `level_patterns`

Include `level_patterns` when the document structure is known or needs precision.

Typical patterns:

```json
{
  "chapter": "(?im)^\\s*(?:CHƯƠNG|Chương)\\s+([IVXLCDM]+)",
  "section": "(?im)^\\s*(?:MỤC|Mục)\\s+([0-9]+)",
  "article": "(?im)^\\s*Điều\\s+([0-9]+)",
  "clause": "(?m)^\\s*([0-9]+)\\.\\s",
  "point": "(?m)^\\s*([a-zđ])\\)\\s"
}
```

### Step 5: Define Metadata Schema

Recommended core fields for Vietnamese normative legal documents:

- `document_number`
- `document_type`
- `issuing_authority`
- `signed_date`
- `effective_date`
- `legal_domain`

Recommended optional fields when strongly extractable and useful:

- `effective_status`
- `signed_year`

If you add a metadata field, you must also add a corresponding mapping rule.

### Step 6: Define Mapping Rules

Each metadata field should map from text using regex and optional `default` or `value_map`.

Example:

```json
{
  "field": "document_number",
  "regex": "(?i)(?:Số|Luật\\s+số)\\s*:?\\s*([0-9]+/[0-9A-Z\\-]+)",
  "group": 1,
  "default": "unknown"
}
```

Important:

- metadata comes from text extraction rules, not from manually invented `Document` fields
- `group` must be `>= 0`
- regex should be robust to Vietnamese diacritics and uppercase headers

### Step 7: Define Reindex Policy

Default:

```json
{
  "on_content_change": true,
  "on_form_change": true
}
```

### Step 8: Define Query Profile

For the current project, ChatGPT should also propose `query_profile` for reusable legal `DocType`s.

Recommended contents:

- `canonical_terms`
- `synonym_groups`
- `query_signals`
- `intent_rules`
- `domain_topic_rules`
- `legal_signal_rules`
- `followup_markers`
- `preferred_doc_types`
- `routing_priority`

This is especially useful for stable, high-value legal families such as marriage/family, civil code, decrees, and resolutions.

For one-off or low-confidence `DocType`s, `query_profile` may be omitted.

## Document Proposal Rules

ChatGPT should propose a `Document` as the concrete uploaded legal record.

Recommended logical output:

```json
{
  "title": "Luật Hôn nhân và Gia đình 2014",
  "doc_type_code": "vn_marriage_family_law",
  "assets": [
    {
      "file_name": "luat-hon-nhan-va-gia-dinh-2014.docx",
      "content_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "versions": [1]
    }
  ]
}
```

Important interpretation:

- this is an orchestration view, not the raw `POST /documents` body
- the current API creates `Document`, `Asset`, and `Version` in separate steps

Do not place metadata fields directly on `Document` or `Asset`.

## Recommended Output Contract For ChatGPT

When analyzing an uploaded file, ChatGPT should return sections in this order:

1. `Document classification`
2. `Detected issuing authority`
3. `Detected legal domain`
4. `Matched or proposed DocType`
5. `DocType Form JSON`
6. `Proposed Document record`
7. `Asset and Version placeholders`
8. `Reasoning and ingest explanation`

## Outdated Assumptions To Avoid

The older spec set should be considered outdated in these areas:

1. It treats paragraph segmentation as the default for Vietnamese legal documents.  
   Current project reality prefers `legal_article` when structure is detectable.

2. It omits `query_profile`.  
   Current `DocTypeForm` includes it and seeded forms already use it.

3. It implies `Document` plus `assets` is the whole ingest model.  
   Current project also requires `DocumentVersion` and ingests at the version level.

4. It suggests the backend automatically creates `DocType` and `Document` from upload.  
   Current API does not do this natively.

5. It uses `general` as fallback legal domain.  
   Current normalization model is closer to `general_legal`.

6. It presents `Document` output as if it were the exact persisted shape.  
   In the current project, `assets` with `versions` is better treated as an orchestration or response projection.

## Recommended ChatGPT Instruction

Use this instruction style:

```text
You are a legal document ingestion architect for the current legal_api project.

Your job is to analyze an uploaded Vietnamese legal file and propose the correct
DocType, DocType form JSON, Document shell, Asset placeholder, and Version
placeholder needed for the ingest workflow.

Important project rules:
- DocType defines schema, segmentation, metadata extraction, reindex policy, and optional query_profile.
- Document is a concrete legal record and belongs to exactly one DocType.
- Asset is the uploaded file.
- Version is the ingest target.
- Metadata must be extracted from mapping_rules, not stored directly on Document or Asset.
- Chunk metadata is created only during ingest.
- Prefer legal_article segmentation when legal structure is visible.
- Reuse an existing DocType when compatible; create a new one only when structure,
  metadata patterns, or legal family materially differ.
- Normalize document_type and legal_domain to project canonical values when possible.

Return:
1. Document classification
2. Detected issuing authority
3. Detected legal domain
4. Matched or proposed DocType
5. DocType Form JSON
6. Proposed Document record
7. Asset and Version placeholders
8. Ingest explanation
```

## Practical Recommendation For ChatGPT Sources

If you want ChatGPT Sources to support the task:

`Upload docx -> analyze -> output DocType and Document set`

then the minimum useful source set is:

- this file: `docs/doctype_generator_spec.md`
- `backend/core/schema/doc_type_form.go`
- `backend/domain/models.go`
- `backend/api/handlers.go`
- `backend/core/ingest/ingest.go`
- `backend/core/ingest/segment_plan.go`
- `backend/core/ingest/legal_structure_parser.go`
- `backend/core/ingest/legal_chunk_generator.go`
- `backend/core/ingest/extractor/extractor.go`
- `backend/core/legalmeta/normalize.go`
- `backend/infra/migrations/011_doc_type_query_profiles.sql`

That source set is enough for ChatGPT to understand:

- the actual schema
- the actual ingest pipeline
- the current project’s preferred legal segmentation model
- how `.docx` is handled
- and how existing `DocType`s are shaped in practice

