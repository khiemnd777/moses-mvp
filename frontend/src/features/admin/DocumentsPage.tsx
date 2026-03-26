import { useEffect, useState } from 'react';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import {
  createDocument,
  createDocumentVersion,
  deleteDocument,
  enqueueIngestJob,
  listDocuments,
  unwrapError,
  uploadDocumentAsset
} from '@/core/api';
import type { DocumentItem } from '@/core/types';
import { useAdminStore } from './adminStore';

const DocumentsPage = () => {
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [title, setTitle] = useState('');
  const [docTypeCode, setDocTypeCode] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [assetIdsByDocument, setAssetIdsByDocument] = useState<Record<string, string>>({});
  const [versionIdsByDocument, setVersionIdsByDocument] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [deletingDocumentId, setDeletingDocumentId] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [creatingVersion, setCreatingVersion] = useState(false);
  const [enqueuing, setEnqueuing] = useState(false);
  const [copiedTitle, setCopiedTitle] = useState<string | null>(null);
  const [copiedDocTypeCode, setCopiedDocTypeCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const { lastOpenedDocumentId, setLastOpenedDocumentId } = useAdminStore();

  const fetchDocs = async () => {
    setLoading(true);
    try {
      const data = await listDocuments();
      setDocuments(data);
      setError(null);
      if (!data.length) {
        setLastOpenedDocumentId(undefined);
        return;
      }
      const hasSelected = data.some((doc) => doc.id === lastOpenedDocumentId);
      if (!lastOpenedDocumentId || !hasSelected) setLastOpenedDocumentId(data[0].id);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchDocs();
  }, []);

  const handleCreate = async () => {
    if (!title.trim() || !docTypeCode.trim()) return;
    try {
      const created = await createDocument({ title: title.trim(), doc_type_code: docTypeCode.trim() });
      setDocuments((prev) => [created, ...prev]);
      setLastOpenedDocumentId(created.id);
      setTitle('');
      setDocTypeCode('');
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleDeleteDocument = async (documentId: string) => {
    const confirmed = window.confirm('Are you sure you want to delete this document?');
    if (!confirmed) return;
    setDeletingDocumentId(documentId);
    try {
      await deleteDocument(documentId);
      setDocuments((prev) => prev.filter((doc) => doc.id !== documentId));
      setAssetIdsByDocument((prev) => {
        const next = { ...prev };
        delete next[documentId];
        return next;
      });
      setVersionIdsByDocument((prev) => {
        const next = { ...prev };
        delete next[documentId];
        return next;
      });
      if (lastOpenedDocumentId === documentId) {
        const remaining = documents.filter((doc) => doc.id !== documentId);
        setLastOpenedDocumentId(remaining[0]?.id);
      }
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setDeletingDocumentId(null);
    }
  };

  const selected = documents.find((doc) => doc.id === lastOpenedDocumentId);
  const selectedAssetId = selected ? assetIdsByDocument[selected.id] ?? '' : '';
  const selectedVersionId = selected ? versionIdsByDocument[selected.id] ?? '' : '';

  const handleUpload = async () => {
    if (!selected || !file) return;
    setUploading(true);
    try {
      const asset = await uploadDocumentAsset(selected.id, file);
      setAssetIdsByDocument((prev) => ({ ...prev, [selected.id]: asset.id }));
      setVersionIdsByDocument((prev) => ({ ...prev, [selected.id]: '' }));
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setUploading(false);
    }
  };

  const handleCreateVersion = async () => {
    if (!selected || !selectedAssetId) return;
    setCreatingVersion(true);
    try {
      const version = await createDocumentVersion(selected.id, { asset_id: selectedAssetId });
      setVersionIdsByDocument((prev) => ({ ...prev, [selected.id]: version.id }));
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setCreatingVersion(false);
    }
  };

  const handleEnqueue = async () => {
    if (!selectedVersionId) return;
    setEnqueuing(true);
    try {
      await enqueueIngestJob(selectedVersionId);
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setEnqueuing(false);
    }
  };

  const formatCreatedAt = (value?: string) => {
    if (!value) return '-';
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return value;
    return d.toLocaleString();
  };

  const handleCopyDocTypeCode = async (value?: string) => {
    if (!value) return;
    await navigator.clipboard.writeText(value);
    setCopiedDocTypeCode(value);
    setTimeout(() => setCopiedDocTypeCode(null), 1200);
  };

  const handleCopyTitle = async (value?: string) => {
    if (!value) return;
    await navigator.clipboard.writeText(value);
    setCopiedTitle(value);
    setTimeout(() => setCopiedTitle(null), 1200);
  };

  return (
    <>
      <Panel title="Documents">
        <div className="grid">
          <div className="grid two">
            <Input label="Title" value={title} onChange={(e) => setTitle(e.target.value)} />
            <Input label="Doc Type Code" value={docTypeCode} onChange={(e) => setDocTypeCode(e.target.value)} />
          </div>
          <Button onClick={handleCreate}>Create Document</Button>
          {loading && <div className="badge">Loading documents...</div>}
          {error && <div className="badge">Error: {error}</div>}
          <div className="grid">
            {documents.map((doc) => (
              <div
                key={doc.id}
                className={`source-item admin-list-item ${lastOpenedDocumentId === doc.id ? 'selected' : ''}`}
              >
                <div className="admin-list-item-header">
                  <div className="admin-list-item-title-row">
                    <button
                      className="button outline admin-list-item-button"
                      onClick={() => setLastOpenedDocumentId(doc.id)}
                    >
                      <span className="admin-list-item-title">{doc.title}</span>
                    </button>
                    <Button
                      variant="outline"
                      className="admin-copy-button"
                      onClick={() => void handleCopyTitle(doc.title)}
                      disabled={!doc.title}
                    >
                      {copiedTitle === doc.title ? 'Copied' : 'Copy'}
                    </Button>
                  </div>
                  <Button
                    variant={lastOpenedDocumentId === doc.id ? 'secondary' : 'outline'}
                    onClick={() => void handleDeleteDocument(doc.id)}
                    disabled={deletingDocumentId === doc.id}
                  >
                    {deletingDocumentId === doc.id ? 'Deleting...' : 'Delete'}
                  </Button>
                </div>
                <div className="admin-list-item-subtitle-row">
                  <span className="admin-list-item-subtitle">{doc.doc_type_code || '-'}</span>
                  <Button
                    variant="outline"
                    className="admin-copy-button"
                    onClick={() => void handleCopyDocTypeCode(doc.doc_type_code)}
                    disabled={!doc.doc_type_code}
                  >
                    {copiedDocTypeCode === doc.doc_type_code ? 'Copied' : 'Copy'}
                  </Button>
                </div>
                <div className="grid admin-document-assets">
                  {doc.assets && doc.assets.length > 0 ? (
                    doc.assets.map((asset, index) => (
                      <div key={`${doc.id}-${asset.file_name}-${asset.created_at ?? index}`} className="admin-document-asset">
                        <div className="admin-document-asset-title">{asset.file_name}</div>
                        <div className="badge">Type: {asset.content_type || '-'}</div>
                        <div className="badge">Created: {formatCreatedAt(asset.created_at)}</div>
                        <div className="badge">
                          Versions:{' '}
                          {asset.versions && asset.versions.length > 0
                            ? asset.versions.map((version) => `v${version}`).join(', ')
                            : '-'}
                        </div>
                      </div>
                    ))
                  ) : (
                    <div className="badge">No uploaded files.</div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </Panel>
      <Panel title="Document Actions">
        {selected ? (
          <div className="grid">
            <div className="badge">Selected: {selected.title}</div>
            {selectedAssetId && <div className="badge">Last Asset ID: {selectedAssetId}</div>}
            {selectedVersionId && <div className="badge">Last Version ID: {selectedVersionId}</div>}
            <div className="grid two">
              <input type="file" onChange={(e) => setFile(e.target.files?.[0] || null)} />
              <Button onClick={handleUpload} disabled={!file || uploading}>
                {uploading ? 'Uploading...' : 'Upload Asset'}
              </Button>
            </div>
            <Button variant="secondary" onClick={handleCreateVersion} disabled={!selectedAssetId || creatingVersion}>
              {creatingVersion ? 'Creating...' : 'Create Version'}
            </Button>
            <Button onClick={handleEnqueue} disabled={!selectedAssetId || !selectedVersionId || enqueuing}>
              {enqueuing ? 'Enqueuing...' : 'Enqueue Ingest Job'}
            </Button>
          </div>
        ) : (
          <div className="badge">Select a document to manage assets and ingest jobs.</div>
        )}
      </Panel>
    </>
  );
};

export default DocumentsPage;
