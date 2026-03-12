import { useEffect, useState } from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import Panel from '@/shared/Panel';
import { verifyToken } from '@/playground/auth.js';

const PlaygroundAuthGuard = () => {
  const [checking, setChecking] = useState(true);
  const [authorized, setAuthorized] = useState(false);

  useEffect(() => {
    let active = true;
    const check = async () => {
      const valid = await verifyToken();
      if (!active) return;
      setAuthorized(valid);
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

  return <Outlet />;
};

export default PlaygroundAuthGuard;
