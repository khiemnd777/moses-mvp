declare module '@/playground/apiClient.js' {
  import type { AxiosInstance } from 'axios';

  export const AUTH_TOKEN_KEY: string;
  export const PLAYGROUND_LOGIN_PATH: string;
  const apiClient: AxiosInstance;
  export default apiClient;
}

declare module '@/playground/auth.js' {
  export const AUTH_TOKEN_KEY: string;
  export const PLAYGROUND_LOGIN_PATH: string;

  export function getToken(): string | null;
  export function setToken(token: string): void;
  export function clearToken(): void;
  export function login(
    username: string,
    password: string
  ): Promise<{ access_token: string; expires_in: number }>;
  export function me(): Promise<{ id: string; username: string; role: string }>;
  export function verifyToken(): Promise<boolean>;
  export function logout(): void;
}
