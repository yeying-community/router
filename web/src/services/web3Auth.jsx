import {
  clearAccessToken as sdkClearAccessToken,
  focusPendingApproval,
  getAccessToken as sdkGetAccessToken,
  getChainId,
  getProvider,
  isUserRejectedWalletAction,
  isWalletReconnectError,
  loginWithChallenge,
  logout as sdkLogout,
  refreshAccessToken as sdkRefreshAccessToken,
  requestAccounts,
  signMessage,
  watchProvider,
} from '@yeying-community/web3-bs';

import { WEB3_AUTH_OPTIONS, WEB3_TOKEN_STORAGE_KEY } from '../helpers/web3';
import {
  getAccessTokenExpiresAt,
  isAccessTokenFresh,
} from '../helpers/walletSession.mjs';

const WALLET_RECONNECT_TIMEOUT_MS = 1600;

function getRefreshPayload(refreshResult) {
  return refreshResult?.response?.data || refreshResult?.response || {};
}

function persistRefreshedWalletSession(refreshResult) {
  const token = refreshResult?.token;
  if (!token || typeof window === 'undefined') {
    return;
  }
  const payload = getRefreshPayload(refreshResult);
  const expiresAt =
    Number(payload?.expiresAt || 0) || getAccessTokenExpiresAt(token);
  if (expiresAt > 0) {
    localStorage.setItem(
      'wallet_token_expires_at',
      new Date(expiresAt).toISOString(),
    );
  }
  try {
    const storedUserRaw = localStorage.getItem('user');
    if (!storedUserRaw) {
      return;
    }
    const storedUser = JSON.parse(storedUserRaw);
    if (storedUser?.id) {
      localStorage.setItem(
        'user',
        JSON.stringify({
          ...storedUser,
          token,
        }),
      );
    }
  } catch (error) {
    // Keep the refreshed SDK token even if the legacy user cache is malformed.
  }
}

export function normalizeChainId(chainId) {
  if (!chainId) return '';
  if (typeof chainId !== 'string') return String(chainId);
  if (chainId.startsWith('0x')) {
    const parsed = parseInt(chainId, 16);
    if (!Number.isNaN(parsed)) {
      return parsed.toString();
    }
  }
  return chainId;
}

function waitForWalletProviderReconnect(timeoutMs = WALLET_RECONNECT_TIMEOUT_MS) {
  if (typeof window === 'undefined') {
    return Promise.resolve();
  }
  return new Promise((resolve) => {
    let settled = false;
    let stopWatching = () => {};
    const finish = () => {
      if (settled) return;
      settled = true;
      stopWatching();
      window.clearTimeout(timer);
      resolve();
    };
    const timer = window.setTimeout(finish, timeoutMs);
    stopWatching = watchProvider(
      ({ present }) => {
        if (present) {
          finish();
        }
      },
      { preferYeYing: true, pollIntervalMs: 100, maxPolls: 16 },
    );
    if (settled) {
      stopWatching();
    }
  });
}

export async function requireWalletProvider() {
  const provider = await getProvider();
  if (!provider) {
    throw new Error('未检测到钱包，请安装 MetaMask 或开启浏览器钱包');
  }
  return provider;
}

export async function getWalletContext(preferredAddress = '') {
  const provider = await requireWalletProvider();
  const accounts = await requestAccounts({ provider });
  const normalizedPreferredAddress = String(preferredAddress || '').trim().toLowerCase();
  const matchedAddress = Array.isArray(accounts)
    ? accounts.find(
        (item) =>
          String(item || '').trim().toLowerCase() === normalizedPreferredAddress,
      )
    : '';
  const address = matchedAddress || accounts?.[0];
  if (!address) {
    throw new Error('未获取到钱包账户');
  }
  const chainId = normalizeChainId(await getChainId(provider));
  return { provider, address, chainId };
}

async function loginWithWalletOnce(preferredAddress = '') {
  const { provider, address } = await getWalletContext(preferredAddress);
  const loginResult = await loginWithChallenge({
    provider,
    address,
    ...WEB3_AUTH_OPTIONS,
  });
  return { ...loginResult, provider, address };
}

export async function loginWithWallet(preferredAddress = '') {
  try {
    return await loginWithWalletOnce(preferredAddress);
  } catch (error) {
    if (!isWalletReconnectError(error) || isUserRejectedWalletAction(error)) {
      throw error;
    }
    await waitForWalletProviderReconnect();
    return await loginWithWalletOnce(preferredAddress);
  }
}

export async function focusWalletPendingApproval(provider) {
  try {
    return await focusPendingApproval(provider);
  } catch (error) {
    return { focused: false, type: null };
  }
}

export function isWalletUserRejectedError(error) {
  return isUserRejectedWalletAction(error);
}

export async function signWalletMessage(message, address, provider) {
  const activeProvider = provider || (await requireWalletProvider());
  const signature = await signMessage({
    provider: activeProvider,
    message,
    address,
  });
  return { signature, provider: activeProvider };
}

export function getStoredAccessToken() {
  return sdkGetAccessToken({ tokenStorageKey: WEB3_TOKEN_STORAGE_KEY });
}

export async function refreshWalletAccessToken() {
  const result = await sdkRefreshAccessToken(WEB3_AUTH_OPTIONS);
  persistRefreshedWalletSession(result);
  return result;
}

export async function restoreWalletSession() {
  const token = getStoredAccessToken();
  if (isAccessTokenFresh(token)) {
    return {
      token,
      refreshed: false,
    };
  }
  const result = await refreshWalletAccessToken();
  return {
    ...result,
    refreshed: true,
  };
}

export async function logoutWallet() {
  try {
    await sdkLogout(WEB3_AUTH_OPTIONS);
  } finally {
    sdkClearAccessToken({ tokenStorageKey: WEB3_TOKEN_STORAGE_KEY });
  }
}
