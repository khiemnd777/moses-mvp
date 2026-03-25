import axios from 'axios';

const AUTH_TOKEN_KEY = 'auth_token';
const PLAYGROUND_LOGIN_PATH = '/playground/login';
const CHANGE_PASSWORD_PATH = '/change-password';
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true
});

const refreshClient = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true
});

let refreshPromise = null;

const getStoredToken = () => window.localStorage.getItem(AUTH_TOKEN_KEY);

const setStoredToken = (token) => {
  window.localStorage.setItem(AUTH_TOKEN_KEY, token);
};

const clearStoredToken = () => {
  window.localStorage.removeItem(AUTH_TOKEN_KEY);
};

apiClient.interceptors.request.use((config) => {
  const token = getStoredToken();
  if (token && !config.skipAuthToken) {
    config.headers = config.headers ?? {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

const redirectToLogin = () => {
  if (window.location.pathname !== PLAYGROUND_LOGIN_PATH) {
    window.location.assign(PLAYGROUND_LOGIN_PATH);
  }
};

const maybeRedirectPasswordChange = (error, skipUnauthorizedRedirect) => {
  const status = error?.response?.status;
  const errorCode = error?.response?.data?.error?.code;
  const errorMessage = error?.response?.data?.error?.message;
  if (
    status === 403 &&
    !skipUnauthorizedRedirect &&
    (errorCode === 'password_change_required' || errorMessage === 'password_change_required') &&
    window.location.pathname !== CHANGE_PASSWORD_PATH
  ) {
    window.location.assign(CHANGE_PASSWORD_PATH);
  }
};

const refreshAccessToken = async () => {
  if (!refreshPromise) {
    refreshPromise = refreshClient
      .post('/auth/refresh', undefined, {
        skipUnauthorizedRedirect: true,
        skipAuthRefresh: true,
        skipAuthToken: true
      })
      .then(({ data }) => {
        setStoredToken(data.access_token);
        return data;
      })
      .catch((error) => {
        clearStoredToken();
        throw error;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }

  return refreshPromise;
};

apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const status = error?.response?.status;
    const skipUnauthorizedRedirect = Boolean(error?.config?.skipUnauthorizedRedirect);
    const skipAuthRefresh = Boolean(error?.config?.skipAuthRefresh);
    const originalRequest = error?.config;

    if (status === 401 && !skipAuthRefresh && originalRequest && !originalRequest._retry) {
      originalRequest._retry = true;
      try {
        const refreshed = await refreshAccessToken();
        originalRequest.headers = originalRequest.headers ?? {};
        originalRequest.headers.Authorization = `Bearer ${refreshed.access_token}`;
        return apiClient(originalRequest);
      } catch (refreshError) {
        maybeRedirectPasswordChange(refreshError, skipUnauthorizedRedirect);
        if (!skipUnauthorizedRedirect) {
          redirectToLogin();
        }
        return Promise.reject(refreshError);
      }
    }
    maybeRedirectPasswordChange(error, skipUnauthorizedRedirect);
    return Promise.reject(error);
  }
);

export default apiClient;
export {
  AUTH_TOKEN_KEY,
  PLAYGROUND_LOGIN_PATH,
  CHANGE_PASSWORD_PATH,
  getStoredToken,
  setStoredToken,
  clearStoredToken,
  refreshAccessToken,
  redirectToLogin
};
