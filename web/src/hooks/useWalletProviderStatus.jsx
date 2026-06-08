import { useCallback, useEffect, useRef, useState } from 'react';
import { getProvider } from '@yeying-community/web3-bs';

import { normalizeChainId } from '../services/web3Auth';

const PROVIDER_DETECT_TIMEOUT_MS = 1200;

const getProviderAccounts = async (provider) => {
  if (!provider?.request) return [];
  const accounts = await provider.request({ method: 'eth_accounts' });
  return Array.isArray(accounts) ? accounts : [];
};

const getProviderChainId = async (provider) => {
  if (!provider?.request) return '';
  const chainId = await provider.request({ method: 'eth_chainId' });
  return normalizeChainId(chainId);
};

export function useWalletProviderStatus({
  onAccountsChanged,
  onConnect,
  onDisconnect,
} = {}) {
  const providerRef = useRef(null);
  const detectInFlightRef = useRef(null);
  const cleanupProviderListenersRef = useRef(() => {});
  const callbacksRef = useRef({ onAccountsChanged, onConnect, onDisconnect });
  const [status, setStatus] = useState({
    detecting: true,
    available: false,
    connected: false,
    accounts: [],
    chainId: '',
  });

  useEffect(() => {
    callbacksRef.current = { onAccountsChanged, onConnect, onDisconnect };
  }, [onAccountsChanged, onConnect, onDisconnect]);

  const updateProviderState = useCallback(async (provider) => {
    if (!provider) {
      setStatus({
        detecting: false,
        available: false,
        connected: false,
        accounts: [],
        chainId: '',
      });
      return;
    }

    try {
      const [accounts, chainId] = await Promise.all([
        getProviderAccounts(provider),
        getProviderChainId(provider).catch(() => ''),
      ]);
      setStatus({
        detecting: false,
        available: true,
        connected:
          Boolean(provider.isConnected?.()) || accounts.length > 0 || chainId !== '',
        accounts,
        chainId,
      });
    } catch (error) {
      setStatus((previous) => ({
        ...previous,
        detecting: false,
        available: true,
        connected: false,
        accounts: [],
      }));
    }
  }, []);

  const bindProviderListeners = useCallback(
    (provider) => {
      cleanupProviderListenersRef.current();
      providerRef.current = provider || null;

      if (!provider?.on) {
        cleanupProviderListenersRef.current = () => {};
        return;
      }

      const handleAccountsChanged = (accounts) => {
        const nextAccounts = Array.isArray(accounts) ? accounts : [];
        setStatus((previous) => ({
          ...previous,
          detecting: false,
          available: true,
          connected: nextAccounts.length > 0 || previous.chainId !== '',
          accounts: nextAccounts,
        }));
        if (nextAccounts.length > 0) {
          callbacksRef.current.onConnect?.();
        }
        callbacksRef.current.onAccountsChanged?.(nextAccounts);
      };

      const handleChainChanged = (chainId) => {
        setStatus((previous) => ({
          ...previous,
          detecting: false,
          available: true,
          connected: true,
          chainId: normalizeChainId(chainId),
        }));
      };

      const handleConnect = (data) => {
        setStatus((previous) => ({
          ...previous,
          detecting: false,
          available: true,
          connected: true,
          chainId: normalizeChainId(data?.chainId) || previous.chainId,
        }));
        callbacksRef.current.onConnect?.();
      };

      const handleDisconnect = (error) => {
        setStatus((previous) => ({
          ...previous,
          detecting: false,
          connected: false,
          accounts: [],
        }));
        callbacksRef.current.onDisconnect?.(error);
      };

      provider.on('accountsChanged', handleAccountsChanged);
      provider.on('chainChanged', handleChainChanged);
      provider.on('connect', handleConnect);
      provider.on('disconnect', handleDisconnect);

      cleanupProviderListenersRef.current = () => {
        provider.removeListener?.('accountsChanged', handleAccountsChanged);
        provider.removeListener?.('chainChanged', handleChainChanged);
        provider.removeListener?.('connect', handleConnect);
        provider.removeListener?.('disconnect', handleDisconnect);
      };
    },
    [],
  );

  const detectProvider = useCallback(async () => {
    if (detectInFlightRef.current) {
      return detectInFlightRef.current;
    }
    setStatus((previous) => ({ ...previous, detecting: true }));
    detectInFlightRef.current = (async () => {
      const provider = await getProvider({
        preferYeYing: true,
        timeoutMs: PROVIDER_DETECT_TIMEOUT_MS,
      });
      bindProviderListeners(provider);
      await updateProviderState(provider);
      return provider;
    })();
    try {
      return await detectInFlightRef.current;
    } finally {
      detectInFlightRef.current = null;
    }
  }, [bindProviderListeners, updateProviderState]);

  useEffect(() => {
    let active = true;
    const runDetectProvider = async () => {
      if (!active) return;
      await detectProvider();
    };

    const handleProviderChanged = () => {
      runDetectProvider();
    };

    runDetectProvider();
    window.addEventListener('ethereum#initialized', handleProviderChanged);
    window.addEventListener('eip6963:announceProvider', handleProviderChanged);
    try {
      window.dispatchEvent(new Event('eip6963:requestProvider'));
    } catch (error) {
      // Ignore browsers that cannot dispatch the provider discovery event.
    }

    return () => {
      active = false;
      window.removeEventListener('ethereum#initialized', handleProviderChanged);
      window.removeEventListener('eip6963:announceProvider', handleProviderChanged);
      cleanupProviderListenersRef.current();
    };
  }, [detectProvider]);

  return {
    ...status,
    provider: providerRef.current,
    refresh: detectProvider,
  };
}
