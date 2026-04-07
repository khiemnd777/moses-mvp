import { ReactNode, useEffect, useMemo, useState } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { json } from '@codemirror/lang-json';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import Select from '@/shared/Select';
import type { DocType, DocTypeForm, QueryProfile } from '@/core/types';
import { canonicalStringify, sha256 } from '@/core/utils';
import { useDisplayModeStore } from '@/app/displayModeStore';

const safeParseForm = (value: string): DocTypeForm | null => {
  try {
    return JSON.parse(value) as DocTypeForm;
  } catch {
    return null;
  }
};

const createDefaultMappingRule = (fieldName: string): DocTypeForm['mapping_rules'][number] => ({
  field: fieldName,
  regex: '',
  group: 1,
  default: ''
});

const syncMappingRulesWithMetadata = (form: DocTypeForm): DocTypeForm => {
  const normalizedFields = (form.metadata_schema?.fields || []).map((field) => ({
    name: field.name?.trim?.() ?? '',
    type: field.type || 'string'
  }));
  const existing = new Map<string, DocTypeForm['mapping_rules'][number]>();
  for (const rule of form.mapping_rules || []) {
    if (!rule?.field || existing.has(rule.field)) continue;
    existing.set(rule.field, {
      field: rule.field,
      regex: rule.regex || '',
      group: rule.group ?? 1,
      default: rule.default ?? '',
      value_map: rule.value_map
    });
  }

  const syncedRules: DocTypeForm['mapping_rules'] = [];
  for (const field of normalizedFields) {
    if (!field.name) continue;
    const rule = existing.get(field.name) || createDefaultMappingRule(field.name);
    syncedRules.push({ ...rule, field: field.name, group: rule.group ?? 1, default: rule.default ?? '' });
  }

  return {
    ...form,
    metadata_schema: {
      fields: normalizedFields
    },
    mapping_rules: syncedRules
  };
};

const nextFieldName = (fields: DocTypeForm['metadata_schema']['fields']): string => {
  let idx = fields.length + 1;
  let candidate = `field_${idx}`;
  const names = new Set(fields.map((f) => f.name));
  for (; names.has(candidate); idx++) {
    candidate = `field_${idx}`;
  }
  return candidate;
};

const upsertMappingRule = (
  draft: DocTypeForm,
  fieldName: string,
  updater: (rule: DocTypeForm['mapping_rules'][number]) => void
) => {
  const normalizedField = fieldName.trim();
  if (!normalizedField) return;
  const existing = draft.mapping_rules.find((rule) => rule.field === normalizedField);
  if (existing) {
    updater(existing);
    return;
  }
  const created = createDefaultMappingRule(normalizedField);
  updater(created);
  draft.mapping_rules.push(created);
};

const parseValueMapInput = (raw: string): Record<string, string> | undefined => {
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return undefined;
    const out: Record<string, string> = {};
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof value !== 'string') return undefined;
      out[key] = value;
    }
    return out;
  } catch {
    return undefined;
  }
};

const parseCsv = (raw: string): string[] => {
  return raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
};

const parseJsonObjectArray = <T,>(raw: string): T[] => {
  const trimmed = raw.trim();
  if (!trimmed) return [];
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    return Array.isArray(parsed) ? (parsed as T[]) : [];
  } catch {
    return [];
  }
};

const normalizeQueryProfile = (profile?: QueryProfile): QueryProfile => ({
  canonical_terms: profile?.canonical_terms || [],
  synonym_groups: profile?.synonym_groups || [],
  query_signals: profile?.query_signals || [],
  intent_rules: profile?.intent_rules || [],
  domain_topic_rules: profile?.domain_topic_rules || [],
  legal_signal_rules: profile?.legal_signal_rules || [],
  followup_markers: profile?.followup_markers || [],
  preferred_doc_types: profile?.preferred_doc_types || [],
  routing_priority: profile?.routing_priority ?? 0
});

const normalizeForm = (form: DocTypeForm, docType: DocType): DocTypeForm => ({
  ...syncMappingRulesWithMetadata(form),
  version: form.version || 1,
  doc_type: {
    code: form.doc_type?.code || docType.code,
    name: form.doc_type?.name || docType.name
  }
});

const withDefaults = (form: DocTypeForm | null | undefined, docType: DocType): DocTypeForm => ({
  version: form?.version || 1,
  doc_type: {
    code: form?.doc_type?.code || docType.code,
    name: form?.doc_type?.name || docType.name
  },
  segment_rules: {
    strategy: form?.segment_rules?.strategy || '',
    hierarchy: form?.segment_rules?.hierarchy || '',
    normalization: form?.segment_rules?.normalization || ''
  },
  metadata_schema: {
    fields: form?.metadata_schema?.fields ? [...form.metadata_schema.fields] : []
  },
  mapping_rules: form?.mapping_rules ? [...form.mapping_rules] : [],
  reindex_policy: {
    on_content_change: form?.reindex_policy?.on_content_change ?? true,
    on_form_change: form?.reindex_policy?.on_form_change ?? true
  },
  query_profile: normalizeQueryProfile(form?.query_profile)
});

const DocTypeEditor = ({ docType, onSave }: { docType: DocType; onSave: (docType: DocType) => void }) => {
  const [formText, setFormText] = useState<string>('{}');
  const [hash, setHash] = useState<string>('');
  const [error, setError] = useState<string | undefined>();
  const [valueMapErrors, setValueMapErrors] = useState<Record<string, string>>({});
  const resolvedDisplayMode = useDisplayModeStore((state) => state.resolvedDisplayMode);

  useEffect(() => {
    setFormText(JSON.stringify(docType.form, null, 2));
    setError(undefined);
    setValueMapErrors({});
  }, [docType]);

  const jsonExtensions = useMemo(() => [json()], []);
  const parsedForm = useMemo(() => safeParseForm(formText), [formText]);
  const isFormValid = parsedForm !== null;
  const displayedForm = useMemo(
    () => syncMappingRulesWithMetadata(withDefaults(parsedForm ?? docType.form, docType)),
    [parsedForm, docType]
  );
  const fieldTypeOptions = ['string', 'number', 'boolean', 'date', 'datetime'];

  const updateForm = (updater: (draft: DocTypeForm) => void) => {
    if (!parsedForm) return;
    const next = JSON.parse(
      JSON.stringify(syncMappingRulesWithMetadata(withDefaults(parsedForm, docType)))
    ) as DocTypeForm;
    updater(next);
    setFormText(JSON.stringify(syncMappingRulesWithMetadata(next), null, 2));
  };

  useEffect(() => {
    const run = async () => {
      const parsed = safeParseForm(formText);
      if (!parsed) {
        setHash('Invalid JSON');
        return;
      }
      const canonical = canonicalStringify(parsed);
      const nextHash = await sha256(canonical);
      setHash(nextHash);
    };
    run();
  }, [formText]);

  const handleSave = () => {
    const parsed = safeParseForm(formText);
    if (!parsed) {
      setError('Form JSON is invalid.');
      return;
    }
    const payload: DocType = {
      ...docType,
      form: normalizeForm(parsed, docType)
    };
    setError(undefined);
    onSave(payload);
  };

  return (
    <div className="grid">
      {!isFormValid && <div className="badge">Fix JSON to edit structured fields.</div>}
      {error && <div className="badge">{error}</div>}
      <PanelSection title="Doc Type">
        <div className="grid">
          <Input
            label="doc_type.code"
            value={displayedForm.doc_type.code}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.doc_type.code = e.target.value;
              })
            }
          />
          <Input
            label="doc_type.name"
            value={displayedForm.doc_type.name}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.doc_type.name = e.target.value;
              })
            }
          />
        </div>
      </PanelSection>
      <PanelSection title="Segment Rules">
        <div className="grid">
          <Input
            label="segment_rules.strategy"
            value={displayedForm.segment_rules.strategy}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.segment_rules.strategy = e.target.value;
              })
            }
          />
          <Input
            label="segment_rules.hierarchy"
            value={displayedForm.segment_rules.hierarchy}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.segment_rules.hierarchy = e.target.value;
              })
            }
          />
          <Input
            label="segment_rules.normalization"
            value={displayedForm.segment_rules.normalization}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.segment_rules.normalization = e.target.value;
              })
            }
          />
        </div>
      </PanelSection>
      <PanelSection title="Metadata Fields">
        <div className="grid">
          {displayedForm.metadata_schema.fields.map((field, index) => (
            <div className="grid" key={`${field.name}-${index}`}>
              <Input
                label="name"
                value={field.name}
                disabled={!isFormValid}
                onChange={(e) =>
                  updateForm((draft) => {
                    const prevName = draft.metadata_schema.fields[index].name;
                    draft.metadata_schema.fields[index].name = e.target.value;
                    const linkedRule = draft.mapping_rules.find((rule) => rule.field === prevName);
                    if (linkedRule) linkedRule.field = e.target.value;
                  })
                }
              />
              <Select
                label="type"
                value={field.type}
                disabled={!isFormValid}
                onChange={(e) =>
                  updateForm((draft) => {
                    draft.metadata_schema.fields[index].type = e.target.value;
                  })
                }
              >
                {fieldTypeOptions.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>
              <Button
                type="button"
                variant="secondary"
                disabled={!isFormValid}
                onClick={() =>
                  updateForm((draft) => {
                    draft.metadata_schema.fields.splice(index, 1);
                  })
                }
              >
                Remove
              </Button>
            </div>
          ))}
          <Button
            type="button"
            variant="secondary"
            disabled={!isFormValid}
            onClick={() =>
              updateForm((draft) => {
                draft.metadata_schema.fields.push({ name: nextFieldName(draft.metadata_schema.fields), type: 'string' });
              })
            }
          >
            Add field
          </Button>
        </div>
      </PanelSection>
      <PanelSection title="Mapping Rules (Aligned With Metadata)">
        <div className="grid">
          {displayedForm.mapping_rules.map((rule, index) => (
            <div className="grid" key={`${rule.field}-${index}`}>
              <Input label="field" value={rule.field} disabled />
              <Input
                label="regex"
                value={rule.regex}
                disabled={!isFormValid}
                onChange={(e) =>
                  updateForm((draft) => {
                    upsertMappingRule(draft, rule.field, (item) => {
                      item.regex = e.target.value;
                    });
                  })
                }
              />
              <Input
                label="group"
                type="number"
                min={0}
                value={String(rule.group ?? 1)}
                disabled={!isFormValid}
                onChange={(e) =>
                  updateForm((draft) => {
                    const parsed = Number.parseInt(e.target.value, 10);
                    upsertMappingRule(draft, rule.field, (item) => {
                      item.group = Number.isNaN(parsed) ? 0 : Math.max(parsed, 0);
                    });
                  })
                }
              />
              <Input
                label="default"
                value={rule.default || ''}
                disabled={!isFormValid}
                onChange={(e) =>
                  updateForm((draft) => {
                    upsertMappingRule(draft, rule.field, (item) => {
                      item.default = e.target.value;
                    });
                  })
                }
              />
              <label>
                <div className="label">value_map (JSON object)</div>
                <textarea
                  className="input"
                  rows={4}
                  value={rule.value_map ? JSON.stringify(rule.value_map, null, 2) : ''}
                  disabled={!isFormValid}
                  onChange={(e) =>
                    updateForm((draft) => {
                      const next = parseValueMapInput(e.target.value);
                      if (e.target.value.trim() && !next) {
                        setValueMapErrors((prev) => ({
                          ...prev,
                          [rule.field]: 'Invalid JSON object of string values'
                        }));
                        return;
                      }
                      setValueMapErrors((prev) => {
                        const copy = { ...prev };
                        delete copy[rule.field];
                        return copy;
                      });
                      upsertMappingRule(draft, rule.field, (item) => {
                        item.value_map = next;
                      });
                    })
                  }
                  placeholder='{"Luật":"LAW","Nghị định":"DECREE"}'
                />
                {valueMapErrors[rule.field] && <div className="badge">{valueMapErrors[rule.field]}</div>}
              </label>
            </div>
          ))}
          <div className="badge">
            Mapping rules are auto-aligned to metadata fields.
          </div>
        </div>
      </PanelSection>
      <PanelSection title="Query Profile">
        <div className="grid">
          <Input
            label="query_profile.canonical_terms (csv)"
            value={(displayedForm.query_profile?.canonical_terms || []).join(', ')}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.query_profile = normalizeQueryProfile(draft.query_profile);
                draft.query_profile.canonical_terms = parseCsv(e.target.value);
              })
            }
          />
          <Input
            label="query_profile.query_signals (csv)"
            value={(displayedForm.query_profile?.query_signals || []).join(', ')}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.query_profile = normalizeQueryProfile(draft.query_profile);
                draft.query_profile.query_signals = parseCsv(e.target.value);
              })
            }
          />
          <Input
            label="query_profile.legal_signal_rules (csv)"
            value={(displayedForm.query_profile?.legal_signal_rules || []).join(', ')}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.query_profile = normalizeQueryProfile(draft.query_profile);
                draft.query_profile.legal_signal_rules = parseCsv(e.target.value);
              })
            }
          />
          <Input
            label="query_profile.followup_markers (csv)"
            value={(displayedForm.query_profile?.followup_markers || []).join(', ')}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.query_profile = normalizeQueryProfile(draft.query_profile);
                draft.query_profile.followup_markers = parseCsv(e.target.value);
              })
            }
          />
          <Input
            label="query_profile.preferred_doc_types (csv)"
            value={(displayedForm.query_profile?.preferred_doc_types || []).join(', ')}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.query_profile = normalizeQueryProfile(draft.query_profile);
                draft.query_profile.preferred_doc_types = parseCsv(e.target.value);
              })
            }
          />
          <Input
            label="query_profile.routing_priority"
            type="number"
            value={String(displayedForm.query_profile?.routing_priority ?? 0)}
            disabled={!isFormValid}
            onChange={(e) =>
              updateForm((draft) => {
                draft.query_profile = normalizeQueryProfile(draft.query_profile);
                draft.query_profile.routing_priority = Number.parseInt(e.target.value, 10) || 0;
              })
            }
          />
          <label>
            <div className="label">query_profile.synonym_groups (JSON)</div>
            <textarea
              className="textarea"
              rows={6}
              value={JSON.stringify(displayedForm.query_profile?.synonym_groups || [], null, 2)}
              disabled={!isFormValid}
              onChange={(e) =>
                updateForm((draft) => {
                  draft.query_profile = normalizeQueryProfile(draft.query_profile);
                  draft.query_profile.synonym_groups = parseJsonObjectArray(e.target.value || '[]');
                })
              }
            />
          </label>
          <label>
            <div className="label">query_profile.intent_rules (JSON)</div>
            <textarea
              className="textarea"
              rows={6}
              value={JSON.stringify(displayedForm.query_profile?.intent_rules || [], null, 2)}
              disabled={!isFormValid}
              onChange={(e) =>
                updateForm((draft) => {
                  draft.query_profile = normalizeQueryProfile(draft.query_profile);
                  draft.query_profile.intent_rules = parseJsonObjectArray(e.target.value || '[]');
                })
              }
            />
          </label>
          <label>
            <div className="label">query_profile.domain_topic_rules (JSON)</div>
            <textarea
              className="textarea"
              rows={6}
              value={JSON.stringify(displayedForm.query_profile?.domain_topic_rules || [], null, 2)}
              disabled={!isFormValid}
              onChange={(e) =>
                updateForm((draft) => {
                  draft.query_profile = normalizeQueryProfile(draft.query_profile);
                  draft.query_profile.domain_topic_rules = parseJsonObjectArray(e.target.value || '[]');
                })
              }
            />
          </label>
          <div className="badge">
            Query understanding for chat, answer, follow-up, and smalltalk now comes from DOC TYPE query_profile.
          </div>
        </div>
      </PanelSection>
      <label>
        <div className="label">Doc Type Form (JSON)</div>
        <div className="codemirror">
          <CodeMirror
            value={formText}
            height="320px"
            theme={resolvedDisplayMode}
            extensions={jsonExtensions}
            onChange={setFormText}
          />
        </div>
      </label>
      <div className="badge">Canonical JSON Hash: {hash}</div>
      <Button onClick={handleSave}>Save</Button>
    </div>
  );
};

export default DocTypeEditor;

const PanelSection = ({ title, children }: { title: string; children: ReactNode }) => {
  return (
    <div className="panel-section">
      <div className="label">{title}</div>
      {children}
    </div>
  );
};
