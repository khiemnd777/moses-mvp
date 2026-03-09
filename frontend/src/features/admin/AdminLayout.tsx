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
