import apiClient, { AUTH_TOKEN_KEY, CHANGE_PASSWORD_PATH, PLAYGROUND_LOGIN_PATH } from './apiClient.js';

export const getToken = () => window.localStorage.getItem(AUTH_TOKEN_KEY);

export const setToken = (token) => {
  window.localStorage.setItem(AUTH_TOKEN_KEY, token);
};

export const clearToken = () => {
  window.localStorage.removeItem(AUTH_TOKEN_KEY);
};

export const login = async (username, password) => {
  const { data } = await apiClient.post(
    '/auth/login',
    { username, password },
    { skipUnauthorizedRedirect: true }
  );
  setToken(data.access_token);
  return data;
};

export const me = async () => {
  const { data } = await apiClient.get('/auth/me', { skipUnauthorizedRedirect: true });
  return data;
};

export const getSessionState = async () => {
  const token = getToken();
  if (!token) return { valid: false, mustChangePassword: false };
  try {
    const identity = await me();
    return { valid: true, mustChangePassword: Boolean(identity?.must_change_password) };
  } catch {
    clearToken();
    return { valid: false, mustChangePassword: false };
  }
};

export const verifyToken = async () => {
  const state = await getSessionState();
  return state.valid;
};

export const logout = () => {
  clearToken();
  window.location.assign(PLAYGROUND_LOGIN_PATH);
};

export { AUTH_TOKEN_KEY, PLAYGROUND_LOGIN_PATH, CHANGE_PASSWORD_PATH };
