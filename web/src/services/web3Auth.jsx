import {
  getProvider,
  requestAccounts,
  loginWithChallenge,
  logout as sdkLogout,
  getAccessToken as sdkGetAccessToken,
  refreshAccessToken as sdkRefreshAccessToken,
  clearAccessToken as sdkClearAccessToken,
  signMessage,
  getChainId,
} from '@yeying-community/web3-bs';

import { WEB3_AUTH_OPTIONS, WEB3_TOKEN_STORAGE_KEY } from '../helpers/web3';

const WALLET_RECONNECT_TIMEOUT_MS = 1600;

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

function isWalletReconnectError(error) {
  const message = String(error?.message || '').toLowerCase();
  const reason = String(error?.data?.reason || '').toLowerCase();
  if (error?.code === 4001) {
    return false;
  }
  return (
    error?.code === 4900 ||
    reason.includes('extension_context_invalidated') ||
    message.includes('extension context invalidated') ||
    message.includes('wallet extension reconnected') ||
    message.includes('wallet not connected') ||
    message.includes('please refresh the page') ||
    message.includes('provider disconnected')
  );
}

function waitForWalletProviderReconnect(timeoutMs = WALLET_RECONNECT_TIMEOUT_MS) {
  if (typeof window === 'undefined') {
    return Promise.resolve();
  }
  return new Promise((resolve) => {
    let settled = false;
    const finish = () => {
      if (settled) return;
      settled = true;
      window.removeEventListener('ethereum#initialized', finish);
      window.removeEventListener('eip6963:announceProvider', finish);
      window.clearTimeout(timer);
      resolve();
    };
    const timer = window.setTimeout(finish, timeoutMs);
    window.addEventListener('ethereum#initialized', finish, { once: true });
    window.addEventListener('eip6963:announceProvider', finish, { once: true });
    try {
      window.dispatchEvent(new Event('eip6963:requestProvider'));
    } catch (error) {
      // Ignore browsers that cannot dispatch the provider discovery event.
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

export async function getWalletContext() {
  const provider = await requireWalletProvider();
  const accounts = await requestAccounts({ provider });
  const address = accounts?.[0];
  if (!address) {
    throw new Error('未获取到钱包账户');
  }
  const chainId = normalizeChainId(await getChainId(provider));
  return { provider, address, chainId };
}

async function loginWithWalletOnce() {
  const { provider, address } = await getWalletContext();
  const loginResult = await loginWithChallenge({
    provider,
    address,
    ...WEB3_AUTH_OPTIONS,
  });
  return { ...loginResult, provider, address };
}

export async function loginWithWallet() {
  try {
    return await loginWithWalletOnce();
  } catch (error) {
    if (!isWalletReconnectError(error)) {
      throw error;
    }
    await waitForWalletProviderReconnect();
    return await loginWithWalletOnce();
  }
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
  return sdkRefreshAccessToken(WEB3_AUTH_OPTIONS);
}

export async function logoutWallet() {
  try {
    await sdkLogout(WEB3_AUTH_OPTIONS);
  } finally {
    sdkClearAccessToken({ tokenStorageKey: WEB3_TOKEN_STORAGE_KEY });
  }
}
