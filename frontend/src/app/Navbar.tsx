import { NavLink, useLocation } from 'react-router-dom';
import Button from '@/shared/Button';
import { logout } from '@/playground/auth.js';
import { useDisplayModeStore, type DisplayMode } from './displayModeStore';
import {
  ArticleIcon,
  DarkModeIcon,
  LightModeIcon,
  LogoutIcon,
  MonitorIcon,
  PsychologyIcon,
  TuneIcon
} from '@/shared/muiIcons';

const DISPLAY_MODE_ORDER: DisplayMode[] = ['light', 'dark', 'system'];

const getNextDisplayMode = (displayMode: DisplayMode): DisplayMode => {
  const currentIndex = DISPLAY_MODE_ORDER.indexOf(displayMode);
  return DISPLAY_MODE_ORDER[(currentIndex + 1) % DISPLAY_MODE_ORDER.length];
};

const DisplayModeIcon = ({ displayMode }: { displayMode: DisplayMode }) => {
  if (displayMode === 'light') {
    return <LightModeIcon aria-hidden="true" />;
  }

  if (displayMode === 'dark') {
    return <DarkModeIcon aria-hidden="true" />;
  }

  return <MonitorIcon aria-hidden="true" />;
};

const Navbar = () => {
  const location = useLocation();
  const isLoginPage = location.pathname === '/playground/login';
  const displayMode = useDisplayModeStore((state) => state.displayMode);
  const setDisplayMode = useDisplayModeStore((state) => state.setDisplayMode);

  return (
    <nav className="top-nav">
      <div className="top-nav-links">
        <NavLink to="/playground" end className={({ isActive }) => (isActive ? 'active' : '')}>
          <PsychologyIcon aria-hidden="true" />
          Playground
        </NavLink>
        <NavLink to="/tuning" className={({ isActive }) => (isActive ? 'active' : '')}>
          <TuneIcon aria-hidden="true" />
          Operations
        </NavLink>
        <NavLink to="/how-to-rag" className={({ isActive }) => (isActive ? 'active' : '')}>
          <ArticleIcon aria-hidden="true" />
          Docs
        </NavLink>
      </div>
      <button
        type="button"
        className="display-mode-button"
        aria-label={`Display mode: ${displayMode}. Tap to switch to ${getNextDisplayMode(displayMode)}.`}
        title={`Display mode: ${displayMode}`}
        onClick={() => setDisplayMode(getNextDisplayMode(displayMode))}
      >
        <DisplayModeIcon displayMode={displayMode} />
      </button>
      {!isLoginPage && (
        <Button type="button" variant="outline" onClick={logout}>
          <LogoutIcon aria-hidden="true" />
          Logout
        </Button>
      )}
    </nav>
  );
};

export default Navbar;
