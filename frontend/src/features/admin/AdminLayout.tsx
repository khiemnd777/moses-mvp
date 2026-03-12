import { NavLink, Outlet } from 'react-router-dom';

const AdminLayout = () => {
  return (
    <div className="admin-shell">
      <aside className="side-nav">
        <NavLink to="/tuning/doc-types" className={({ isActive }) => (isActive ? 'active' : '')}>
          Doc Types
        </NavLink>
        <NavLink to="/tuning/documents" className={({ isActive }) => (isActive ? 'active' : '')}>
          Documents
        </NavLink>
        <NavLink to="/tuning/ingest-jobs" className={({ isActive }) => (isActive ? 'active' : '')}>
          Ingest Jobs
        </NavLink>
        <NavLink to="/tuning/ai/guard-policies" className={({ isActive }) => (isActive ? 'active' : '')}>
          AI Guard Policies
        </NavLink>
        <NavLink to="/tuning/ai/prompts" className={({ isActive }) => (isActive ? 'active' : '')}>
          AI Prompts
        </NavLink>
        <NavLink to="/tuning/ai/retrieval-configs" className={({ isActive }) => (isActive ? 'active' : '')}>
          AI Retrieval Configs
        </NavLink>
        <NavLink to="/tuning/vectors/collections" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Collections
        </NavLink>
        <NavLink to="/tuning/vectors/search-debug" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Search Debug
        </NavLink>
        <NavLink to="/tuning/vectors/health" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Health
        </NavLink>
        <NavLink to="/tuning/vectors/delete" className={({ isActive }) => (isActive ? 'active' : '')}>
          Vector Delete
        </NavLink>
        <NavLink to="/tuning/vectors/reindex" className={({ isActive }) => (isActive ? 'active' : '')}>
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
