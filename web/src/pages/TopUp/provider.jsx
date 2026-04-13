import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { formatAmountWithUnit, renderYYC } from '../../helpers/render';
import {
  convertYYCToDisplayAmount,
  loadPublicDisplayCurrencyCatalog,
} from '../../helpers/billing';
import {
  TopUpWorkspaceContext,
  buildInitialDisplayCurrencyIndex,
  normalizeTopUpResult,
  resolveDisplayCurrency,
  storeDisplayCurrency,
  YYC_DISPLAY_CODE,
} from './shared.jsx';

const TopUpWorkspaceProvider = ({ children }) => {
  const { t } = useTranslation();
  const initialCurrencyIndex = buildInitialDisplayCurrencyIndex();
  const [userBalanceYYC, setUserBalanceYYC] = useState(0);
  const [topupBalanceYYC, setTopupBalanceYYC] = useState(0);
  const [redeemBalanceYYC, setRedeemBalanceYYC] = useState(0);
  const [balanceLots, setBalanceLots] = useState([]);
  const [loadingBalanceLots, setLoadingBalanceLots] = useState(false);
  const [topupPlans, setTopupPlans] = useState([]);
  const [displayCurrencyIndex, setDisplayCurrencyIndex] = useState(
    initialCurrencyIndex,
  );
  const [displayCurrency, setDisplayCurrency] = useState(
    resolveDisplayCurrency(initialCurrencyIndex),
  );
  const [loadingDisplayCurrencies, setLoadingDisplayCurrencies] =
    useState(false);

  const renderDisplayAmount = useCallback(
    (yycAmount) => {
      const normalizedAmount = Number(yycAmount || 0);
      if (!Number.isFinite(normalizedAmount)) {
        return renderYYC(0, t);
      }
      if (displayCurrency === YYC_DISPLAY_CODE) {
        return renderYYC(normalizedAmount, t);
      }
      const displayAmount = convertYYCToDisplayAmount(
        normalizedAmount,
        displayCurrency,
        displayCurrencyIndex,
      );
      if (!Number.isFinite(displayAmount)) {
        return renderYYC(normalizedAmount, t);
      }
      return formatAmountWithUnit(displayAmount, displayCurrency, 6);
    },
    [displayCurrency, displayCurrencyIndex, t],
  );

  const loadDisplayCurrencies = useCallback(async () => {
    setLoadingDisplayCurrencies(true);
    try {
      const { currencyIndex: nextIndex, defaultCurrency } =
        await loadPublicDisplayCurrencyCatalog();
      setDisplayCurrencyIndex(nextIndex);
      setDisplayCurrency((previous) => {
        const next = resolveDisplayCurrency(
          nextIndex,
          previous || defaultCurrency,
        );
        storeDisplayCurrency(next);
        return next;
      });
    } finally {
      setLoadingDisplayCurrencies(false);
    }
  }, []);

  const loadUserBalance = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/public/user/self');
      const { success, message, data } = res?.data || {};
      if (success) {
        setUserBalanceYYC(Number(data?.yyc_balance ?? data?.quota ?? 0) || 0);
        return;
      }
      showError(message || t('topup.external_topup.request_failed'));
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
    }
  }, [t]);

  const loadBalanceSummary = useCallback(
    async ({ silent = false } = {}) => {
      try {
        const res = await API.get('/api/v1/public/user/topup/balance/summary');
        const { success, message, data } = res?.data || {};
        if (!success) {
          if (!silent) {
            showError(message || t('topup.external_topup.request_failed'));
          }
          return false;
        }
        const totalBalance = Number(
          data?.total_yyc_balance ?? data?.yyc_balance ?? data?.quota ?? 0,
        );
        const topupBalance = Number(data?.topup_yyc_balance ?? 0);
        const redeemBalance = Number(data?.redeem_yyc_balance ?? 0);
        setUserBalanceYYC(Number.isFinite(totalBalance) ? totalBalance : 0);
        setTopupBalanceYYC(Number.isFinite(topupBalance) ? topupBalance : 0);
        setRedeemBalanceYYC(Number.isFinite(redeemBalance) ? redeemBalance : 0);
        return true;
      } catch (error) {
        if (!silent) {
          showError(error?.message || t('topup.external_topup.request_failed'));
        }
        return false;
      }
    },
    [t],
  );

  const loadTopupPlans = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/public/topup/plans');
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.external_topup.request_failed'));
        return;
      }
      setTopupPlans(Array.isArray(data) ? data : []);
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
    }
  }, [t]);

  const loadBalanceLots = useCallback(
    async ({ silent = false } = {}) => {
      if (!silent) {
        setLoadingBalanceLots(true);
      }
      try {
        const res = await API.get('/api/v1/public/user/topup/balance/lots', {
          params: {
            page: 1,
            page_size: 20,
            status: 'active',
            positive_only: true,
          },
        });
        const { success, message, data } = res?.data || {};
        if (!success) {
          if (!silent) {
            showError(message || t('topup.external_topup.request_failed'));
          }
          return false;
        }
        const items = Array.isArray(data?.items) ? data.items : [];
        setBalanceLots(items);
        return true;
      } catch (error) {
        if (!silent) {
          showError(error?.message || t('topup.external_topup.request_failed'));
        }
        return false;
      } finally {
        if (!silent) {
          setLoadingBalanceLots(false);
        }
      }
    },
    [t],
  );

  useEffect(() => {
    loadBalanceSummary().then((success) => {
      if (!success) {
        loadUserBalance().then();
      }
    });
    loadTopupPlans().then();
    loadDisplayCurrencies().then();
    loadBalanceLots({ silent: true }).then();
  }, [loadBalanceLots, loadBalanceSummary, loadDisplayCurrencies, loadTopupPlans, loadUserBalance]);

  const createTopupOrder = useCallback(
    async (payload) => {
      const popup = window.open('', '_blank');
      if (!popup) {
        showError(t('topup.external_topup.popup_blocked'));
        return false;
      }
      try {
        popup.opener = null;
        popup.document.write(`
          <!doctype html>
          <html>
            <head>
              <meta charset="utf-8" />
              <title>${t('common.loading')}</title>
              <style>
                body {
                  margin: 0;
                  min-height: 100vh;
                  display: grid;
                  place-items: center;
                  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
                  color: #111827;
                  background: #f8fafc;
                }
                .router-topup-loading {
                  padding: 1.25rem 1.5rem;
                  border-radius: 14px;
                  background: #ffffff;
                  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.08);
                  font-size: 14px;
                }
              </style>
            </head>
            <body>
              <div class="router-topup-loading">${t('common.loading')}</div>
            </body>
          </html>
        `);
        popup.document.close();
        popup.focus();
      } catch (error) {
        // Ignore same-origin popup bootstrap failures and continue with redirect.
      }
      try {
        const res = await API.post('/api/v1/public/user/topup/orders', payload);
        const { success, message, data } = res.data || {};
        if (!success) {
          if (!popup.closed) {
            popup.close();
          }
          showError(message || t('topup.external_topup.request_failed'));
          return false;
        }
        const currentStatus = (data?.status || '').trim();
        if (currentStatus === 'paid' || currentStatus === 'fulfilled') {
          if (!popup.closed) {
            popup.close();
          }
          await Promise.all([
            loadUserBalance(),
            loadBalanceSummary({ silent: true }),
            loadBalanceLots({ silent: true }),
          ]);
          showSuccess(t('topup.records.order_paid'));
          return true;
        }
        const redirectURL = data?.redirect_url;
        if (!redirectURL) {
          if (!popup.closed) {
            popup.close();
          }
          showError(t('topup.external_topup.request_failed'));
          return false;
        }
        popup.location.href = redirectURL;
        popup.focus();
        return true;
      } catch (error) {
        if (!popup.closed) {
          popup.close();
        }
        showError(error?.message || t('topup.external_topup.request_failed'));
        return false;
      }
    },
    [loadBalanceLots, loadBalanceSummary, loadUserBalance, t],
  );

  const previewPackagePurchase = useCallback(
    async (payload) => {
      try {
        const res = await API.post(
          '/api/v1/public/user/topup/package/preview',
          payload || {},
        );
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('topup.external_topup.request_failed'));
          return null;
        }
        if (!data || typeof data !== 'object') {
          return null;
        }
        return data;
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
        return null;
      }
    },
    [t],
  );

  const submitRedemption = useCallback(
    async (code) => {
      const res = await API.post('/api/v1/public/user/topup', {
        code,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('topup.redeem.request_failed'));
        return null;
      }
      const normalizedResult =
        normalizeTopUpResult(data) || {
          redeemed_yyc: Number(data ?? 0) || 0,
          before_yyc_balance: userBalanceYYC,
          after_yyc_balance: userBalanceYYC + (Number(data ?? 0) || 0),
          redemption_id: '',
          redemption_name: '',
          group_id: '',
          group_name: '',
          face_value_amount: 0,
          face_value_unit: '',
          redeemed_at: 0,
          credit_expires_at: 0,
        };
      setUserBalanceYYC(normalizedResult.after_yyc_balance);
      setRedeemBalanceYYC((previous) =>
        Math.max(0, previous + normalizedResult.redeemed_yyc),
      );
      loadBalanceLots({ silent: true }).then();
      showSuccess(t('topup.redeem.success'));
      return normalizedResult;
    },
    [loadBalanceLots, t, userBalanceYYC],
  );

  const contextValue = useMemo(
    () => ({
      userBalanceYYC,
      topupBalanceYYC,
      redeemBalanceYYC,
      balanceLots,
      loadingBalanceLots,
      topupPlans,
      displayCurrency,
      displayCurrencyIndex,
      loadingDisplayCurrencies,
      renderDisplayAmount,
      loadUserBalance,
      loadBalanceSummary,
      loadBalanceLots,
      createTopupOrder,
      previewPackagePurchase,
      submitRedemption,
    }),
    [
      createTopupOrder,
      balanceLots,
      displayCurrency,
      displayCurrencyIndex,
      loadBalanceLots,
      loadBalanceSummary,
      loadUserBalance,
      loadingBalanceLots,
      loadingDisplayCurrencies,
      previewPackagePurchase,
      redeemBalanceYYC,
      renderDisplayAmount,
      submitRedemption,
      topupBalanceYYC,
      topupPlans,
      userBalanceYYC,
    ],
  );

  return (
    <TopUpWorkspaceContext.Provider value={contextValue}>
      {children}
    </TopUpWorkspaceContext.Provider>
  );
};

export default TopUpWorkspaceProvider;
