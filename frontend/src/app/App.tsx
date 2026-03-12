import { Route, Routes, Navigate } from 'react-router-dom';
import AdminLayout from '@/features/admin/AdminLayout';
import PlaygroundAuthGuard from '@/features/admin/PlaygroundAuthGuard';
import PlaygroundPage from '@/features/admin/PlaygroundPage';
import PlaygroundLoginPage from '@/features/admin/PlaygroundLoginPage';
import ChangePasswordPage from '@/features/admin/ChangePasswordPage';
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
import Navbar from './Navbar';
import ChatPage from '@/features/chat/ChatPage';

const App = () => {
  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="brand">Moses Console</div>
        <Navbar />
      </header>
      <main className="app-main">
        <Routes>
          <Route path="/" element={<Navigate to="/playground" replace />} />
          <Route path="/playground/login" element={<PlaygroundLoginPage />} />
          <Route path="/change-password" element={<ChangePasswordPage />} />
          <Route element={<PlaygroundAuthGuard />}>
            <Route path="/playground" element={<ChatPage />} />
            <Route path="/tuning" element={<AdminLayout />}>
              <Route index element={<Navigate to="/tuning/doc-types" replace />} />
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
            </Route>
          </Route>
        </Routes>
      </main>
    </div>
  );
};

export default App;
