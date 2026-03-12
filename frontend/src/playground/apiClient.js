import axios from 'axios';

const AUTH_TOKEN_KEY = 'auth_token';
const PLAYGROUND_LOGIN_PATH = '/playground/login';

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
    if (status === 401 && !skipUnauthorizedRedirect) {
      window.localStorage.removeItem(AUTH_TOKEN_KEY);
      if (window.location.pathname !== PLAYGROUND_LOGIN_PATH) {
        window.location.assign(PLAYGROUND_LOGIN_PATH);
      }
    }
    return Promise.reject(error);
  }
);

export default apiClient;
export { AUTH_TOKEN_KEY, PLAYGROUND_LOGIN_PATH };
