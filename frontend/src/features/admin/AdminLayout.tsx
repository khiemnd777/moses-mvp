import { NavLink, Outlet } from 'react-router-dom';

const navSections = [
  {
    label: 'Content',
    items: [
      { to: '/tuning/doc-types', label: 'Doc Types' },
      { to: '/tuning/documents', label: 'Documents' },
      { to: '/tuning/ingest-jobs', label: 'Ingest Jobs' }
    ]
  },
  {
    label: 'AI Control',
    items: [
      { to: '/tuning/ai/guard-policies', label: 'Guard Policies' },
      { to: '/tuning/ai/prompts', label: 'Prompts' },
      { to: '/tuning/ai/retrieval-configs', label: 'Retrieval Configs' }
    ]
  },
  {
    label: 'Vectors',
    items: [
      { to: '/tuning/vectors/collections', label: 'Collections' },
      { to: '/tuning/vectors/search-debug', label: 'Search Debug' },
      { to: '/tuning/vectors/health', label: 'Health' },
      { to: '/tuning/vectors/delete', label: 'Delete by Filter' },
      { to: '/tuning/vectors/reindex', label: 'Reindex' }
    ]
  },
  {
    label: 'Integrations',
    items: [{ to: '/tuning/integrations/telegram', label: 'Telegram Bots' }]
  }
];

const AdminLayout = () => {
  return (
    <div className="admin-shell">
      <aside className="side-nav card">
        <div className="side-nav-title">
          <span>Operations</span>
          <span className="badge">Admin</span>
        </div>
        {navSections.map((section) => (
          <div className="side-nav-section" key={section.label}>
            <div className="side-nav-label">{section.label}</div>
            {section.items.map((item) => (
              <NavLink key={item.to} to={item.to} className={({ isActive }) => (isActive ? 'active' : '')}>
                {item.label}
              </NavLink>
            ))}
          </div>
        ))}
      </aside>
      <main className="admin-content">
        <Outlet />
      </main>
    </div>
  );
};

export default AdminLayout;
