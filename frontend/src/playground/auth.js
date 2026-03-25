import apiClient, {
  AUTH_TOKEN_KEY,
  CHANGE_PASSWORD_PATH,
  PLAYGROUND_LOGIN_PATH,
  clearStoredToken,
  getStoredToken,
  redirectToLogin,
  setStoredToken
} from './apiClient.js';

export const getToken = () => getStoredToken();

export const setToken = (token) => {
  setStoredToken(token);
};

export const clearToken = () => {
  clearStoredToken();
};

export const login = async (username, password) => {
  const { data } = await apiClient.post(
    '/auth/login',
    { username, password },
    { skipUnauthorizedRedirect: true, skipAuthRefresh: true }
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
  void apiClient
    .post('/auth/logout', undefined, { skipUnauthorizedRedirect: true, skipAuthRefresh: true })
    .catch(() => undefined)
    .finally(() => {
      clearToken();
      redirectToLogin();
    });
};

export { AUTH_TOKEN_KEY, PLAYGROUND_LOGIN_PATH, CHANGE_PASSWORD_PATH };
