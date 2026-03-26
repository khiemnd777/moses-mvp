import { NavLink, useLocation } from 'react-router-dom';
import Button from '@/shared/Button';
import { logout } from '@/playground/auth.js';
import { useDisplayModeStore, type DisplayMode } from './displayModeStore';

const DISPLAY_MODE_ORDER: DisplayMode[] = ['light', 'dark', 'system'];

const getNextDisplayMode = (displayMode: DisplayMode): DisplayMode => {
  const currentIndex = DISPLAY_MODE_ORDER.indexOf(displayMode);
  return DISPLAY_MODE_ORDER[(currentIndex + 1) % DISPLAY_MODE_ORDER.length];
};

const DisplayModeIcon = ({ displayMode }: { displayMode: DisplayMode }) => {
  if (displayMode === 'light') {
    return (
      <svg viewBox="0 0 24 24" aria-hidden="true" className="display-mode-icon">
        <circle cx="12" cy="12" r="4.25" fill="none" stroke="currentColor" strokeWidth="1.8" />
        <path
          d="M12 2.5v2.25M12 19.25v2.25M21.5 12h-2.25M4.75 12H2.5M18.72 5.28l-1.6 1.6M6.88 17.12l-1.6 1.6M18.72 18.72l-1.6-1.6M6.88 6.88l-1.6-1.6"
          fill="none"
          stroke="currentColor"
          strokeLinecap="round"
          strokeWidth="1.8"
        />
      </svg>
    );
  }

  if (displayMode === 'dark') {
    return (
      <svg viewBox="0 0 24 24" aria-hidden="true" className="display-mode-icon">
        <path
          d="M15.2 3.2a8.6 8.6 0 1 0 5.6 15.45A9.6 9.6 0 1 1 15.2 3.2Z"
          fill="none"
          stroke="currentColor"
          strokeLinejoin="round"
          strokeWidth="1.8"
        />
      </svg>
    );
  }

  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" className="display-mode-icon">
      <rect x="3.5" y="5" width="17" height="12" rx="2.5" fill="none" stroke="currentColor" strokeWidth="1.8" />
      <path d="M9.75 20h4.5" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M12 17v3" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M12 8.2v5.6" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
      <path d="M9.2 11h5.6" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
    </svg>
  );
};

const Navbar = () => {
  const location = useLocation();
  const isLoginPage = location.pathname === '/playground/login';
  const displayMode = useDisplayModeStore((state) => state.displayMode);
  const setDisplayMode = useDisplayModeStore((state) => state.setDisplayMode);

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
      <button
        type="button"
        className="display-mode-button"
        aria-label={`Display mode: ${displayMode}. Tap to switch to ${getNextDisplayMode(displayMode)}.`}
        title={`Display mode: ${displayMode}`}
        onClick={() => setDisplayMode(getNextDisplayMode(displayMode))}
      >
        <DisplayModeIcon displayMode={displayMode} />
      </button>
      <Button type="button" variant="outline" onClick={logout} disabled={isLoginPage}>
        Logout
      </Button>
    </nav>
  );
};

export default Navbar;
