import { Link, useParams } from 'react-router-dom';
import { getQdrantCollection } from '@/core/api';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import { useCachedQuery } from './useAdminApi';

const CollectionDetailPage = () => {
  const { name = '' } = useParams();
  const collectionName = decodeURIComponent(name);
  const { data, error, isLoading, isRefreshing, refresh } = useCachedQuery(
    `qdrant-collection-${collectionName}`,
    () => getQdrantCollection(collectionName),
    { enabled: Boolean(collectionName), cacheMs: 10_000 }
  );

  const collection = data?.collection;

  return (
    <Panel title={`Collection: ${collectionName || '-'}`}>
      <div className="grid">
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <Button onClick={() => void refresh()} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing...' : 'Refresh'}
          </Button>
          <Link to="/tuning/vectors/collections">
            <Button variant="outline">Back to Collections</Button>
          </Link>
          {isLoading && <div className="badge">Loading collection...</div>}
          {error && <div className="badge">Error: {error}</div>}
          {data?.summary && <div className="badge">{data.summary}</div>}
        </div>

        {data && !collection && <div className="badge">Collection not found.</div>}

        {collection && (
          <>
            <div className="vector-meta-grid">
              <div className="badge">Status: {collection.status || '-'}</div>
              <div className="badge">Vectors: {collection.vector_count ?? '-'}</div>
              <div className="badge">Points: {collection.points_count ?? '-'}</div>
              <div className="badge">Indexed: {collection.indexed_vectors_count ?? '-'}</div>
              <div className="badge">Dimension: {collection.vector_dimension ?? '-'}</div>
              <div className="badge">Distance: {collection.distance_metric || '-'}</div>
            </div>
            <div className={`badge ${collection.validation.passed ? 'status-ok' : 'status-warn'}`}>
              Validation: {collection.validation.message || (collection.validation.passed ? 'passed' : 'not available')}
            </div>
            <div className="grid">
              <div className="label">Payload Schema</div>
              {(collection.payload_schema_summary || []).length === 0 && <div className="badge">No schema fields reported.</div>}
              {(collection.payload_schema_summary || []).map((field) => (
                <div key={field.key} className="source-item">
                  <strong>{field.key}</strong>
                  <div className="badge">Type: {field.type || '-'}</div>
                </div>
              ))}
            </div>
            <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
              <Link to="/tuning/vectors/health">
                <Button variant="secondary">Vector Health</Button>
              </Link>
              <Link to="/tuning/vectors/search-debug">
                <Button variant="secondary">Search Debug</Button>
              </Link>
              <Link to="/tuning/vectors/delete">
                <Button variant="secondary">Delete by Filter</Button>
              </Link>
            </div>
          </>
        )}
      </div>
    </Panel>
  );
};

export default CollectionDetailPage;
