import { useEffect, useState } from 'react';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import { createDocType, deleteDocType, listDocTypes, unwrapError, updateDocType } from '@/core/api';
import type { DocType, DocTypeForm } from '@/core/types';
import { useAdminStore } from './adminStore';
import DocTypeEditor from './DocTypeEditor';

const buildDefaultForm = (code: string, name: string): DocTypeForm => ({
  version: 1,
  doc_type: { code, name },
  segment_rules: { strategy: 'legal_article', hierarchy: 'article', normalization: 'basic' },
  metadata_schema: { fields: [{ name: 'title', type: 'string' }] },
  mapping_rules: [{ field: 'title', regex: '^Title\\s*:\\s*(.+)$', group: 1 }],
  reindex_policy: { on_content_change: true, on_form_change: true }
});

const DocTypesPage = () => {
  const [docTypes, setDocTypes] = useState<DocType[]>([]);
  const [loading, setLoading] = useState(false);
  const [deletingDocTypeId, setDeletingDocTypeId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | undefined>();
  const { selectedDocTypeId, setSelectedDocTypeId } = useAdminStore();

  const fetchDocTypes = async () => {
    setLoading(true);
    try {
      const data = await listDocTypes();
      setDocTypes(data);
      setError(undefined);
      if (!selectedDocTypeId && data.length) setSelectedDocTypeId(data[0].id);
      if (selectedDocTypeId && !data.some((doc) => doc.id === selectedDocTypeId)) {
        setSelectedDocTypeId(data[0]?.id);
      }
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDocTypes();
  }, []);

  const handleCreate = async () => {
    if (!name.trim() || !code.trim()) return;
    try {
      const trimmedCode = code.trim();
      const trimmedName = name.trim();
      const created = await createDocType({
        code: trimmedCode,
        name: trimmedName,
        form: buildDefaultForm(trimmedCode, trimmedName)
      });
      setDocTypes((prev) => [created, ...prev]);
      setSelectedDocTypeId(created.id);
      setName('');
      setCode('');
      setError(undefined);
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleSave = async (docType: DocType) => {
    try {
      const updated = await updateDocType(docType.id, { form: docType.form });
      setDocTypes((prev) => prev.map((item) => (item.id === updated.id ? updated : item)));
      setError(undefined);
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleDelete = async (docTypeId: string) => {
    const confirmed = window.confirm('Are you sure you want to delete this doc type?');
    if (!confirmed) return;
    setDeletingDocTypeId(docTypeId);
    try {
      await deleteDocType(docTypeId);
      const remaining = docTypes.filter((doc) => doc.id !== docTypeId);
      setDocTypes(remaining);
      if (selectedDocTypeId === docTypeId) {
        setSelectedDocTypeId(remaining[0]?.id);
      }
      setError(undefined);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setDeletingDocTypeId(null);
    }
  };

  const selected = docTypes.find((doc) => doc.id === selectedDocTypeId);

  return (
    <>
      <Panel title="Doc Types">
        <div className="grid">
          {loading && <div className="badge">Loading...</div>}
          {error && <div className="badge">{error}</div>}
          <div className="grid">
            <Input label="Doc type code" value={code} onChange={(e) => setCode(e.target.value)} />
            <Input label="Doc type name" value={name} onChange={(e) => setName(e.target.value)} />
            <Button onClick={handleCreate}>Create</Button>
          </div>
          <div className="grid">
            {docTypes.map((doc) => (
              <div
                key={doc.id}
                className={`admin-list-item ${selectedDocTypeId === doc.id ? 'selected' : ''}`}
              >
                <button
                  className="button outline admin-list-item-button"
                  onClick={() => setSelectedDocTypeId(doc.id)}
                >
                  {doc.name} ({doc.code})
                </button>
                <Button
                  variant={selectedDocTypeId === doc.id ? 'secondary' : 'outline'}
                  onClick={() => void handleDelete(doc.id)}
                  disabled={deletingDocTypeId === doc.id}
                >
                  {deletingDocTypeId === doc.id ? 'Deleting...' : 'Delete'}
                </Button>
              </div>
            ))}
          </div>
        </div>
      </Panel>
      <Panel title="Dynamic Form Editor">
        {selected ? <DocTypeEditor docType={selected} onSave={handleSave} /> : <div>Select a doc type</div>}
      </Panel>
    </>
  );
};

export default DocTypesPage;
