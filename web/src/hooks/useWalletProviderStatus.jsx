import { useCallback, useEffect, useRef, useState } from 'react';
import {
  getAccounts,
  getChainId,
  getProvider,
  watchAccounts,
  watchProvider,
} from '@yeying-community/web3-bs';

import { normalizeChainId } from '../services/web3Auth';

const PROVIDER_DETECT_TIMEOUT_MS = 1200;

export function useWalletProviderStatus({
  onAccountsChanged,
  onConnect,
  onDisconnect,
} = {}) {
  const providerRef = useRef(null);
  const detectInFlightRef = useRef(null);
  const cleanupProviderListenersRef = useRef(() => {});
  const cleanupProviderWatcherRef = useRef(() => {});
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
        getAccounts(provider),
        getChainId(provider).catch(() => ''),
      ]);
      const normalizedChainId = normalizeChainId(chainId);
      setStatus({
        detecting: false,
        available: true,
        connected:
          Boolean(provider.isConnected?.()) ||
          accounts.length > 0 ||
          normalizedChainId !== '',
        accounts,
        chainId: normalizedChainId,
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

      const stopWatchingAccounts = watchAccounts(provider, ({ accounts }) => {
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
      });

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

      provider.on('chainChanged', handleChainChanged);
      provider.on('connect', handleConnect);
      provider.on('disconnect', handleDisconnect);

      cleanupProviderListenersRef.current = () => {
        stopWatchingAccounts();
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
    cleanupProviderWatcherRef.current = watchProvider(
      ({ provider }) => {
        if (!active) return;
        bindProviderListeners(provider);
        updateProviderState(provider);
      },
      { preferYeYing: true, pollIntervalMs: 100, maxPolls: 20 },
    );

    return () => {
      active = false;
      cleanupProviderWatcherRef.current();
      cleanupProviderListenersRef.current();
    };
  }, [bindProviderListeners, updateProviderState]);

  return {
    ...status,
    provider: providerRef.current,
    refresh: detectProvider,
  };
}
