import type { DocType, DocTypeForm, QueryProfile } from '@/core/types';

type DocTypeIdentity = Pick<DocType, 'code' | 'name'>;
type MetadataField = DocTypeForm['metadata_schema']['fields'][number];
type MappingRule = DocTypeForm['mapping_rules'][number];

const legalMetadataFields: MetadataField[] = [
  { name: 'title', type: 'string' },
  { name: 'document_type', type: 'string' },
  { name: 'document_number', type: 'string' },
  { name: 'legal_domain', type: 'string' },
  { name: 'effective_status', type: 'string' },
  { name: 'issuing_authority', type: 'string' },
  { name: 'signed_year', type: 'number' },
  { name: 'article_number', type: 'string' }
];

const legalMappingRules: MappingRule[] = [
  {
    field: 'title',
    regex: '(?im)^\\s*(?:Title|Ten van ban|Tên văn bản)\\s*[:：]\\s*(.+)$',
    group: 1,
    default: ''
  },
  {
    field: 'document_type',
    regex: '(?im)^\\s*(LUẬT|BỘ LUẬT|NGHỊ ĐỊNH|THÔNG TƯ|NGHỊ QUYẾT|QUYẾT ĐỊNH)\\b',
    group: 1,
    default: 'law',
    value_map: {
      LUẬT: 'law',
      'BỘ LUẬT': 'law',
      'NGHỊ ĐỊNH': 'decree',
      'THÔNG TƯ': 'circular',
      'NGHỊ QUYẾT': 'resolution',
      'QUYẾT ĐỊNH': 'decision'
    }
  },
  {
    field: 'document_number',
    regex: '(?im)(?:Số|So)\\s*[:：]?\\s*([0-9]+/[0-9]{4}/[A-Za-zĐđ\\-]+)',
    group: 1,
    default: ''
  },
  {
    field: 'legal_domain',
    regex: '(?im)^\\s*(?:Lĩnh vực|Linh vuc)\\s*[:：]\\s*(.+)$',
    group: 1,
    default: 'general_legal'
  },
  {
    field: 'effective_status',
    regex: '(?im)(còn hiệu lực|đang có hiệu lực|có hiệu lực|hết hiệu lực|chưa có hiệu lực)',
    group: 1,
    default: 'active',
    value_map: {
      'còn hiệu lực': 'active',
      'đang có hiệu lực': 'active',
      'có hiệu lực': 'active',
      'hết hiệu lực': 'archived',
      'chưa có hiệu lực': 'pending'
    }
  },
  {
    field: 'issuing_authority',
    regex: '(?im)^\\s*(?:Cơ quan ban hành|Co quan ban hanh)\\s*[:：]\\s*(.+)$',
    group: 1,
    default: ''
  },
  {
    field: 'signed_year',
    regex: '(?im)(?:ngày|nam|năm)[^\\n]*(19\\d{2}|20\\d{2})',
    group: 1,
    default: ''
  },
  {
    field: 'article_number',
    regex: '(?im)^\\s*(?:Điều|Dieu)\\s+([0-9]+[a-zA-Z]?)\\b',
    group: 1,
    default: ''
  }
];

const legalQueryProfile: QueryProfile = {
  canonical_terms: [
    'căn cứ pháp lý',
    'quy định pháp luật',
    'điều khoản',
    'thủ tục',
    'xử phạt',
    'hợp đồng',
    'ly hôn',
    'hộ tịch',
    'thừa kế'
  ],
  synonym_groups: [
    { canonical: 'ly hôn', aliases: ['ly dị', 'li hôn'] },
    { canonical: 'căn cứ pháp lý', aliases: ['cơ sở pháp lý', 'quy định pháp luật'] },
    { canonical: 'xử phạt', aliases: ['mức phạt', 'phạt hành chính'] }
  ],
  query_signals: ['điều', 'khoản', 'luật', 'nghị định', 'thông tư', 'căn cứ pháp lý', 'thủ tục', 'mức phạt'],
  intent_rules: [
    { intent: 'lookup_article', terms: ['điều', 'khoản', 'quy định'] },
    { intent: 'procedure', terms: ['thủ tục', 'hồ sơ', 'nộp ở đâu'] },
    { intent: 'penalty', terms: ['mức phạt', 'xử phạt', 'phạt hành chính'] }
  ],
  domain_topic_rules: [
    { legal_domain: 'marriage_family', legal_topic: 'divorce', terms: ['ly hôn', 'ly dị', 'hôn nhân'] },
    { legal_domain: 'civil_status', legal_topic: 'birth_certificate', terms: ['hộ tịch', 'khai sinh'] },
    { legal_domain: 'civil', legal_topic: 'contract', terms: ['hợp đồng', 'dân sự'] },
    { legal_domain: 'criminal_law', legal_topic: 'criminal', terms: ['hình sự', 'tội phạm'] },
    { legal_domain: 'administrative', legal_topic: 'penalty', terms: ['xử phạt', 'hành chính'] }
  ],
  legal_signal_rules: ['điều', 'khoản', 'luật', 'nghị định', 'thông tư', 'căn cứ pháp lý', 'hiệu lực'],
  followup_markers: ['hỏi thêm', 'trường hợp này', 'vậy', 'còn', 'nếu'],
  preferred_doc_types: ['law', 'decree', 'circular', 'resolution', 'decision'],
  routing_priority: 10
};

export const buildLegalRAGForm = (code: string, name: string): DocTypeForm => {
  return applyLegalRAGPreset({
    version: 2,
    doc_type: { code, name },
    segment_rules: { strategy: 'legal_article', hierarchy: 'article', normalization: 'basic' },
    metadata_schema: { fields: [] },
    mapping_rules: [],
    reindex_policy: { on_content_change: true, on_form_change: true },
    query_profile: {}
  });
};

export const applyLegalRAGPreset = (form: DocTypeForm, docType?: DocTypeIdentity): DocTypeForm => {
  const next: DocTypeForm = JSON.parse(JSON.stringify(form)) as DocTypeForm;
  next.version = Math.max(next.version || 1, 2);
  next.doc_type = {
    code: next.doc_type?.code || docType?.code || '',
    name: next.doc_type?.name || docType?.name || ''
  };
  next.segment_rules = {
    strategy: 'legal_article',
    hierarchy: 'article',
    normalization: 'basic'
  };
  next.metadata_schema = { fields: [...(next.metadata_schema?.fields || [])] };
  next.mapping_rules = [...(next.mapping_rules || [])];
  next.reindex_policy = {
    on_content_change: next.reindex_policy?.on_content_change ?? true,
    on_form_change: next.reindex_policy?.on_form_change ?? true
  };

  for (const field of legalMetadataFields) {
    upsertField(next.metadata_schema.fields, field);
  }
  for (const rule of legalMappingRules) {
    upsertRule(next.mapping_rules, rule);
  }
  next.query_profile = mergeQueryProfile(next.query_profile, legalQueryProfile);

  return next;
};

const upsertField = (fields: MetadataField[], field: MetadataField) => {
  const existing = fields.find((item) => item.name === field.name);
  if (!existing) {
    fields.push({ ...field });
    return;
  }
  if (!existing.type) existing.type = field.type;
};

const upsertRule = (rules: MappingRule[], rule: MappingRule) => {
  const existing = rules.find((item) => item.field === rule.field);
  if (!existing) {
    rules.push({ ...rule, value_map: rule.value_map ? { ...rule.value_map } : undefined });
    return;
  }
  existing.regex = existing.regex || rule.regex;
  existing.group = existing.group ?? rule.group;
  existing.default = existing.default || rule.default;
  existing.value_map = existing.value_map || (rule.value_map ? { ...rule.value_map } : undefined);
};

const mergeQueryProfile = (current: QueryProfile | undefined, preset: QueryProfile): QueryProfile => ({
  canonical_terms: mergeUnique(current?.canonical_terms, preset.canonical_terms),
  synonym_groups: mergeSynonymGroups(current?.synonym_groups, preset.synonym_groups),
  query_signals: mergeUnique(current?.query_signals, preset.query_signals),
  intent_rules: mergeRulesByKey(current?.intent_rules, preset.intent_rules, 'intent'),
  domain_topic_rules: mergeRulesByKey(current?.domain_topic_rules, preset.domain_topic_rules, 'legal_domain', 'legal_topic'),
  legal_signal_rules: mergeUnique(current?.legal_signal_rules, preset.legal_signal_rules),
  followup_markers: mergeUnique(current?.followup_markers, preset.followup_markers),
  preferred_doc_types: mergeUnique(current?.preferred_doc_types, preset.preferred_doc_types),
  routing_priority: Math.max(current?.routing_priority ?? 0, preset.routing_priority ?? 0)
});

const mergeUnique = (current: string[] | undefined, preset: string[] | undefined): string[] => {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const item of [...(current || []), ...(preset || [])]) {
    const trimmed = item.trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    out.push(trimmed);
  }
  return out;
};

const mergeSynonymGroups = (
  current: QueryProfile['synonym_groups'] = [],
  preset: QueryProfile['synonym_groups'] = []
): NonNullable<QueryProfile['synonym_groups']> => {
  const byCanonical = new Map<string, NonNullable<QueryProfile['synonym_groups']>[number]>();
  for (const group of [...current, ...preset]) {
    const canonical = group.canonical?.trim();
    if (!canonical) continue;
    const existing = byCanonical.get(canonical);
    if (!existing) {
      byCanonical.set(canonical, { canonical, aliases: mergeUnique([], group.aliases) });
      continue;
    }
    existing.aliases = mergeUnique(existing.aliases, group.aliases);
  }
  return Array.from(byCanonical.values());
};

const mergeRulesByKey = <T extends object>(
  current: T[] | undefined,
  preset: T[] | undefined,
  ...keys: Array<keyof T>
): T[] => {
  const byKey = new Map<string, T>();
  for (const rule of [...(current || []), ...(preset || [])]) {
    const key = keys.map((item) => String(rule[item] || '')).join(':');
    if (!key.replace(/:/g, '').trim() || byKey.has(key)) continue;
    byKey.set(key, rule);
  }
  return Array.from(byKey.values());
};
