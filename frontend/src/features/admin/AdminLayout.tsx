import { NavLink, Outlet } from 'react-router-dom';

const AdminLayout = () => {
  return (
    <div className="admin-shell">
      <aside className="side-nav">
        <NavLink to="/admin/doc-types" className={({ isActive }) => (isActive ? 'active' : '')}>
          Doc Types
        </NavLink>
        <NavLink to="/admin/documents" className={({ isActive }) => (isActive ? 'active' : '')}>
          Documents
        </NavLink>
        <NavLink to="/admin/ingest-jobs" className={({ isActive }) => (isActive ? 'active' : '')}>
          Ingest Jobs
        </NavLink>
        <NavLink to="/admin/ai/guard-policies" className={({ isActive }) => (isActive ? 'active' : '')}>
          AI Guard Policies
        </NavLink>
        <NavLink to="/admin/ai/prompts" className={({ isActive }) => (isActive ? 'active' : '')}>
          AI Prompts
        </NavLink>
        <NavLink to="/admin/ai/retrieval-configs" className={({ isActive }) => (isActive ? 'active' : '')}>
          AI Retrieval Configs
        </NavLink>
        <NavLink to="/admin/vectors/collections" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Collections
        </NavLink>
        <NavLink to="/admin/vectors/search-debug" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Search Debug
        </NavLink>
        <NavLink to="/admin/vectors/health" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Health
        </NavLink>
        <NavLink to="/admin/vectors/delete" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Delete
        </NavLink>
        <NavLink to="/admin/vectors/reindex" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Reindex
        </NavLink>
        {/* <NavLink to="/admin/playground" className={({ isActive }) => (isActive ? 'active' : '')}>
          Playground
        </NavLink> */}
      </aside>
      <div className="grid" style={{ gap: 20 }}>
        <Outlet />
      </div>
    </div>
  );
};

export default AdminLayout;
