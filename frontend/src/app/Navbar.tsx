import { NavLink, useLocation } from 'react-router-dom';
import Button from '@/shared/Button';
import { logout } from '@/playground/auth.js';

const Navbar = () => {
  const location = useLocation();
  const isLoginPage = location.pathname === '/playground/login';

  return (
    <nav className="top-nav">
      <NavLink to="/playground" end className={({ isActive }) => (isActive ? 'active' : '')}>
        Playground
      </NavLink>
      <NavLink to="/tuning" className={({ isActive }) => (isActive ? 'active' : '')}>
        Tuning
      </NavLink>
      <NavLink to="/how-to-rag" className={({ isActive }) => (isActive ? 'active' : '')}>
        How to RAG
      </NavLink>
      <Button type="button" variant="outline" onClick={logout} disabled={isLoginPage}>
        Logout
      </Button>
    </nav>
  );
};

export default Navbar;
