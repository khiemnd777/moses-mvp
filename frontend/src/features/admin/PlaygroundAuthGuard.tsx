import { useEffect, useState } from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import Panel from '@/shared/Panel';
import { getSessionState } from '@/playground/auth.js';

const PlaygroundAuthGuard = () => {
  const [checking, setChecking] = useState(true);
  const [authorized, setAuthorized] = useState(false);
  const [mustChangePassword, setMustChangePassword] = useState(false);

  useEffect(() => {
    let active = true;
    const check = async () => {
      const session = await getSessionState();
      if (!active) return;
      setAuthorized(session.valid);
      setMustChangePassword(session.mustChangePassword);
      setChecking(false);
    };
    void check();
    return () => {
      active = false;
    };
  }, []);

  if (checking) {
    return (
      <Panel title="Protected Area">
        <div className="badge">Verifying session...</div>
      </Panel>
    );
  }

  if (!authorized) {
    return <Navigate to="/playground/login" replace />;
  }
  if (mustChangePassword) {
    return <Navigate to="/change-password" replace />;
  }

  return <Outlet />;
};

export default PlaygroundAuthGuard;
