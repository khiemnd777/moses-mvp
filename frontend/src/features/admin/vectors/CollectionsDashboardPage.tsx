import { useNavigate } from 'react-router-dom';
import { listQdrantCollections } from '@/core/api';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import type { QdrantCollectionSummary } from '@/core/types';
import { useCachedQuery } from './useAdminApi';

const renderNumber = (value?: number) => (typeof value === 'number' ? value.toLocaleString() : '-');

const fieldList = (collection: QdrantCollectionSummary) =>
  (collection.payload_schema_summary || []).map((field) => field.key).slice(0, 6).join(', ') || '-';

const CollectionsDashboardPage = () => {
  const navigate = useNavigate();
  const { data, error, isLoading, isRefreshing, refresh } = useCachedQuery('qdrant-collections', listQdrantCollections, {
    cacheMs: 10_000
  });

  return (
    <Panel title="Qdrant Collections">
      <div className="grid">
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <Button onClick={() => void refresh()} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing...' : 'Refresh'}
          </Button>
          {isLoading && <div className="badge">Loading collections...</div>}
          {error && <div className="badge">Error: {error}</div>}
          {data?.summary && <div className="badge">{data.summary}</div>}
        </div>
        <div className="grid">
          {!isLoading && !error && (data?.collections.length || 0) === 0 && (
            <div className="badge">No collections found.</div>
          )}
          {(data?.collections || []).map((collection) => (
            <button
              key={collection.collection_name}
              className="source-item vector-row-btn"
              onClick={() => navigate(`/admin/vectors/collections/${encodeURIComponent(collection.collection_name)}`)}
              type="button"
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center' }}>
                <strong>{collection.collection_name}</strong>
                <span className={`badge ${collection.validation.passed ? 'status-ok' : 'status-warn'}`}>
                  {collection.validation.passed ? 'Validated' : 'Check dimension'}
                </span>
              </div>
              <div className="vector-meta-grid">
                <div className="badge">Status: {collection.status || '-'}</div>
                <div className="badge">Vectors: {renderNumber(collection.vector_count)}</div>
                <div className="badge">Points: {renderNumber(collection.points_count)}</div>
                <div className="badge">Indexed: {renderNumber(collection.indexed_vectors_count)}</div>
                <div className="badge">Dimension: {renderNumber(collection.vector_dimension)}</div>
                <div className="badge">Distance: {collection.distance_metric || '-'}</div>
              </div>
              <div className="badge">Payload fields: {fieldList(collection)}</div>
            </button>
          ))}
        </div>
      </div>
    </Panel>
  );
};

export default CollectionsDashboardPage;
