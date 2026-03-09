import { NavLink, Route, Routes, Navigate } from 'react-router-dom';
import ChatPage from '@/features/chat/ChatPage';
import AdminLayout from '@/features/admin/AdminLayout';
import DocTypesPage from '@/features/admin/DocTypesPage';
import DocumentsPage from '@/features/admin/DocumentsPage';
import IngestJobsPage from '@/features/admin/IngestJobsPage';
import PlaygroundPage from '@/features/admin/PlaygroundPage';
import GuardPoliciesPage from '@/features/admin/ai/GuardPoliciesPage';
import PromptsPage from '@/features/admin/ai/PromptsPage';
import RetrievalConfigsPage from '@/features/admin/ai/RetrievalConfigsPage';

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
          <Route path="/admin" element={<AdminLayout />}>
            <Route index element={<Navigate to="/admin/doc-types" replace />} />
            <Route path="doc-types" element={<DocTypesPage />} />
            <Route path="documents" element={<DocumentsPage />} />
            <Route path="ingest-jobs" element={<IngestJobsPage />} />
            <Route path="ai/guard-policies" element={<GuardPoliciesPage />} />
            <Route path="ai/prompts" element={<PromptsPage />} />
            <Route path="ai/retrieval-configs" element={<RetrievalConfigsPage />} />
            {/* <Route path="playground" element={<PlaygroundPage />} /> */}
          </Route>
        </Routes>
      </main>
    </div>
  );
};

export default App;
