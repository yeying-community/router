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

  useEffect(() => {
    loadUserBalance().then();
    loadTopupPlans().then();
    loadDisplayCurrencies().then();
  }, [loadDisplayCurrencies, loadTopupPlans, loadUserBalance]);

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
        };
      setUserBalanceYYC(normalizedResult.after_yyc_balance);
      showSuccess(t('topup.redeem.success'));
      return normalizedResult;
    },
    [t, userBalanceYYC],
  );

  const contextValue = useMemo(
    () => ({
      userBalanceYYC,
      topupPlans,
      displayCurrency,
      displayCurrencyIndex,
      loadingDisplayCurrencies,
      renderDisplayAmount,
      loadUserBalance,
      createTopupOrder,
      submitRedemption,
    }),
    [
      createTopupOrder,
      displayCurrency,
      displayCurrencyIndex,
      loadDisplayCurrencies,
      loadTopupPlans,
      loadUserBalance,
      loadingDisplayCurrencies,
      renderDisplayAmount,
      submitRedemption,
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
