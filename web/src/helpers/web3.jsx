const base = (import.meta.env.VITE_SERVER || '').replace(/\/+$/, '');

export const WEB3_AUTH_BASE_URL = base
  ? `${base}/api/v1/public/auth`
  : '/api/v1/public/auth';

export const WEB3_TOKEN_STORAGE_KEY = 'wallet_token';

export const WEB3_AUTH_OPTIONS = {
  baseUrl: WEB3_AUTH_BASE_URL,
  tokenStorageKey: WEB3_TOKEN_STORAGE_KEY,
  credentials: 'include',
};
