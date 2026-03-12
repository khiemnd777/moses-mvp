import { NavLink, Route, Routes, Navigate } from 'react-router-dom';
import ChatPage from '@/features/chat/ChatPage';
import AdminLayout from '@/features/admin/AdminLayout';
import AdminAuthGuard from '@/features/admin/AdminAuthGuard';
import DocTypesPage from '@/features/admin/DocTypesPage';
import DocumentsPage from '@/features/admin/DocumentsPage';
import IngestJobsPage from '@/features/admin/IngestJobsPage';
import GuardPoliciesPage from '@/features/admin/ai/GuardPoliciesPage';
import PromptsPage from '@/features/admin/ai/PromptsPage';
import RetrievalConfigsPage from '@/features/admin/ai/RetrievalConfigsPage';
import CollectionsDashboardPage from '@/features/admin/vectors/CollectionsDashboardPage';
import CollectionDetailPage from '@/features/admin/vectors/CollectionDetailPage';
import SearchDebugPage from '@/features/admin/vectors/SearchDebugPage';
import VectorHealthPage from '@/features/admin/vectors/VectorHealthPage';
import DeleteByFilterPage from '@/features/admin/vectors/DeleteByFilterPage';
import ReindexControlsPage from '@/features/admin/vectors/ReindexControlsPage';

const App = () => {
  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="brand">Moses Console</div>
        <nav className="top-nav">
          <NavLink to="/" end className={({ isActive }) => (isActive ? 'active' : '')}>
            Playground
          </NavLink>
          <NavLink to="/admin/doc-types" className={({ isActive }) => (isActive ? 'active' : '')}>
            Tuning
          </NavLink>
        </nav>
      </header>
      <main className="app-main">
        <Routes>
          <Route path="/" element={<ChatPage />} />
          <Route path="/admin" element={<AdminAuthGuard />}>
            <Route element={<AdminLayout />}>
              <Route index element={<Navigate to="/admin/doc-types" replace />} />
              <Route path="doc-types" element={<DocTypesPage />} />
              <Route path="documents" element={<DocumentsPage />} />
              <Route path="ingest-jobs" element={<IngestJobsPage />} />
              <Route path="ai/guard-policies" element={<GuardPoliciesPage />} />
              <Route path="ai/prompts" element={<PromptsPage />} />
              <Route path="ai/retrieval-configs" element={<RetrievalConfigsPage />} />
              <Route path="vectors/collections" element={<CollectionsDashboardPage />} />
              <Route path="vectors/collections/:name" element={<CollectionDetailPage />} />
              <Route path="vectors/search-debug" element={<SearchDebugPage />} />
              <Route path="vectors/health" element={<VectorHealthPage />} />
              <Route path="vectors/delete" element={<DeleteByFilterPage />} />
              <Route path="vectors/reindex" element={<ReindexControlsPage />} />
              {/* <Route path="playground" element={<PlaygroundPage />} /> */}
            </Route>
          </Route>
        </Routes>
      </main>
    </div>
  );
};

export default App;
