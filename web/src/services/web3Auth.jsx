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

export async function loginWithWallet() {
  const { provider, address } = await getWalletContext();
  const loginResult = await loginWithChallenge({
    provider,
    address,
    ...WEB3_AUTH_OPTIONS,
  });
  return { ...loginResult, provider, address };
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
