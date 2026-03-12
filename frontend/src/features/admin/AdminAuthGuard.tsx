import { Navigate, Outlet, useLocation } from 'react-router-dom';
import Panel from '@/shared/Panel';

const AdminAuthGuard = () => {
  const location = useLocation();
  const hasAdminKey = Boolean(import.meta.env.VITE_ADMIN_API_KEY);
  const hasBearerToken = Boolean(import.meta.env.VITE_ADMIN_BEARER_TOKEN);
  const hasAdminAuth = hasAdminKey || hasBearerToken;

  if (!hasAdminAuth) {
    return (
      <Panel title="Admin Access Required">
        <div className="grid">
          <div className="badge">Missing admin credential configuration.</div>
          <div className="badge">
            Configure `VITE_ADMIN_API_KEY` or `VITE_ADMIN_BEARER_TOKEN` before accessing admin routes.
          </div>
          <div className="badge">Blocked route: {location.pathname}</div>
        </div>
      </Panel>
    );
  }

  if (location.pathname === '/admin') {
    return <Navigate to="/admin/doc-types" replace />;
  }

  return <Outlet />;
};

export default AdminAuthGuard;
