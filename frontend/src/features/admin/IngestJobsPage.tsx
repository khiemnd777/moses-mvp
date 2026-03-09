import { useEffect, useState } from 'react';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import { deleteIngestJob, listIngestJobs, unwrapError } from '@/core/api';
import type { IngestJob } from '@/core/types';

const IngestJobsPage = () => {
  const [jobs, setJobs] = useState<IngestJob[]>([]);
  const [polling, setPolling] = useState(false);
  const [loading, setLoading] = useState(false);
  const [deletingJobId, setDeletingJobId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const fetchJobs = async () => {
    setLoading(true);
    try {
      const data = await listIngestJobs();
      setJobs(data);
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    const confirmed = window.confirm('Are you sure you want to delete this ingest job?');
    if (!confirmed) return;
    setDeletingJobId(id);
    try {
      await deleteIngestJob(id);
      setJobs((prev) => prev.filter((job) => job.id !== id));
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setDeletingJobId(null);
    }
  };

  useEffect(() => {
    void fetchJobs();
  }, []);

  useEffect(() => {
    if (!polling) return;
    const interval = setInterval(() => {
      void fetchJobs();
    }, 4000);
    return () => clearInterval(interval);
  }, [polling]);

  return (
    <Panel title="Ingest Jobs">
      <div className="grid">
        <Button variant={polling ? 'secondary' : 'primary'} onClick={() => setPolling((prev) => !prev)}>
          {polling ? 'Stop Polling' : 'Start Polling'}
        </Button>
        {loading && <div className="badge">Loading jobs...</div>}
        {error && <div className="badge">Error: {error}</div>}
        <div className="grid">
          {!loading && jobs.length === 0 && <div className="badge">No ingest jobs found.</div>}
          {jobs.map((job) => (
            <div className="source-item" key={job.id}>
              <div style={{ fontWeight: 600 }}>{job.id}</div>
              <div className="badge">Status: {job.status}</div>
              {job.document_version_id && <div className="badge">Version: {job.document_version_id}</div>}
              {job.error_message && <div className="badge">Error: {job.error_message}</div>}
              <div style={{ marginTop: 10 }}>
                <Button
                  variant="outline"
                  onClick={() => void handleDelete(job.id)}
                  disabled={deletingJobId === job.id}
                >
                  {deletingJobId === job.id ? 'Deleting...' : 'Delete Job'}
                </Button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </Panel>
  );
};

export default IngestJobsPage;
