import axios from 'axios';

const AUTH_TOKEN_KEY = 'auth_token';
const PLAYGROUND_LOGIN_PATH = '/playground/login';
const CHANGE_PASSWORD_PATH = '/change-password';

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL
});

apiClient.interceptors.request.use((config) => {
  const token = window.localStorage.getItem(AUTH_TOKEN_KEY);
  if (token) {
    config.headers = config.headers ?? {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status;
    const skipUnauthorizedRedirect = Boolean(error?.config?.skipUnauthorizedRedirect);
    const errorCode = error?.response?.data?.error?.code;
    const errorMessage = error?.response?.data?.error?.message;
    if (status === 401 && !skipUnauthorizedRedirect) {
      window.localStorage.removeItem(AUTH_TOKEN_KEY);
      if (window.location.pathname !== PLAYGROUND_LOGIN_PATH) {
        window.location.assign(PLAYGROUND_LOGIN_PATH);
      }
    }
    if (
      status === 403 &&
      !skipUnauthorizedRedirect &&
      (errorCode === 'password_change_required' || errorMessage === 'password_change_required') &&
      window.location.pathname !== CHANGE_PASSWORD_PATH
    ) {
      window.location.assign(CHANGE_PASSWORD_PATH);
    }
    return Promise.reject(error);
  }
);

export default apiClient;
export { AUTH_TOKEN_KEY, PLAYGROUND_LOGIN_PATH, CHANGE_PASSWORD_PATH };
