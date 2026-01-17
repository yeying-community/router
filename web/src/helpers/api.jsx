import { showError } from './utils';
import axios from 'axios';
import { WEB3_TOKEN_STORAGE_KEY } from './web3';
import {
  getStoredAccessToken,
  refreshWalletAccessToken,
} from '../services/web3Auth';

export const API = axios.create({
  baseURL: import.meta.env.VITE_SERVER ? import.meta.env.VITE_SERVER : '',
});

API.interceptors.request.use(
  (config) => {
    if (typeof window !== 'undefined') {
      let token = getStoredAccessToken();
      if (!token) {
        token = localStorage.getItem(WEB3_TOKEN_STORAGE_KEY);
      }
      try {
        const userStr = localStorage.getItem('user');
        if (!token && userStr) {
          const u = JSON.parse(userStr);
          token = u?.token;
        }
      } catch (e) {
        // ignore json parse error
      }
      if (token && !config.headers['Authorization']) {
        config.headers['Authorization'] = `Bearer ${token}`;
      }
    }
    return config;
  },
  (error) => Promise.reject(error)
);

API.interceptors.response.use(
  (response) => response,
  async (error) => {
    const status = error?.response?.status;
    const config = error?.config;
    if (status === 401 && config && !config._retry) {
      const token = getStoredAccessToken();
      if (token) {
        config._retry = true;
        try {
          const refreshed = await refreshWalletAccessToken();
          if (refreshed?.token) {
            config.headers = config.headers || {};
            config.headers['Authorization'] = `Bearer ${refreshed.token}`;
          }
          return API.request(config);
        } catch (refreshError) {
          // fallback to default error handling
        }
      }
    }
    showError(error);
    return Promise.reject(error);
  }
);
