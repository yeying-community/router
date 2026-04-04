import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Card, Header } from 'semantic-ui-react';
import { API, showError, showSuccess } from '../../helpers';
import { formatAmountWithUnit, renderYYC } from '../../helpers/render';
import {
  buildDisplayUnitOptions,
  convertYYCToDisplayAmount,
  loadPublicDisplayCurrencyCatalog,
} from '../../helpers/billing';
import UnitDropdown from '../../components/UnitDropdown';
import BalanceTopUpPage from './BalanceTopUpPage';
import PackagePurchasePage from './PackagePurchasePage';
import RedeemCodePage from './RedeemCodePage';
import TopUpRecordsPage from './TopUpRecordsPage';
import {
  TopUpWorkspaceContext,
  buildInitialDisplayCurrencyIndex,
  getStoredStatusConfig,
  normalizeTopUpResult,
  normalizeTopUpTab,
  resolveDisplayCurrency,
  storeDisplayCurrency,
  YYC_DISPLAY_CODE,
} from './shared.jsx';

const TopUpLayout = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const rawTab = searchParams.get('tab');
  const initialCurrencyIndex = buildInitialDisplayCurrencyIndex();
  const [externalTopupLink, setExternalTopupLink] = useState('');
  const [userBalanceYYC, setUserBalanceYYC] = useState(0);
  const [displayCurrencyIndex, setDisplayCurrencyIndex] = useState(
    initialCurrencyIndex,
  );
  const [displayCurrency, setDisplayCurrency] = useState(
    resolveDisplayCurrency(initialCurrencyIndex),
  );
  const [loadingDisplayCurrencies, setLoadingDisplayCurrencies] =
    useState(false);

  const displayCurrencyOptions = useMemo(
    () => buildDisplayUnitOptions(displayCurrencyIndex, { includeCode: true }),
    [displayCurrencyIndex],
  );

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

  useEffect(() => {
    const status = getStoredStatusConfig();
    if (status.top_up_link) {
      setExternalTopupLink(status.top_up_link);
    }
    loadUserBalance().then();
    loadDisplayCurrencies().then();
  }, [loadDisplayCurrencies, loadUserBalance]);

  useEffect(() => {
    const normalizedTab = normalizeTopUpTab(rawTab);
    if (rawTab === normalizedTab) {
      return;
    }
    navigate(`/workspace/topup?tab=${normalizedTab}`, { replace: true });
  }, [navigate, rawTab]);

  const createTopupOrder = useCallback(
    async (payload) => {
      if (!externalTopupLink) {
        showError(t('topup.external_topup.no_link'));
        return false;
      }
      const popup = window.open('', '_blank', 'noopener,noreferrer');
      if (!popup) {
        showError(t('topup.external_topup.popup_blocked'));
        return false;
      }
      try {
        const res = await API.post('/api/v1/public/user/topup/orders', payload);
        const { success, message, data } = res.data || {};
        if (!success) {
          popup.close();
          showError(message || t('topup.external_topup.request_failed'));
          return false;
        }
        const redirectURL = data?.redirect_url;
        if (!redirectURL) {
          popup.close();
          showError(t('topup.external_topup.request_failed'));
          return false;
        }
        popup.location.href = redirectURL;
        popup.focus();
        return true;
      } catch (error) {
        popup.close();
        showError(error?.message || t('topup.external_topup.request_failed'));
        return false;
      }
    },
    [externalTopupLink, t],
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

  const activeKey = normalizeTopUpTab(rawTab);

  const contextValue = useMemo(
    () => ({
      externalTopupLink,
      userBalanceYYC,
      displayCurrency,
      displayCurrencyIndex,
      displayCurrencyOptions,
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
      displayCurrencyOptions,
      externalTopupLink,
      loadUserBalance,
      loadingDisplayCurrencies,
      renderDisplayAmount,
      submitRedemption,
      userBalanceYYC,
    ],
  );

  const activeContent = useMemo(() => {
    switch (activeKey) {
      case 'package':
        return <PackagePurchasePage />;
      case 'redeem':
        return <RedeemCodePage />;
      case 'records':
        return <TopUpRecordsPage />;
      case 'balance':
      default:
        return <BalanceTopUpPage />;
    }
  }, [activeKey]);

  return (
    <TopUpWorkspaceContext.Provider value={contextValue}>
      <div className='dashboard-container'>
        <Card fluid className='chart-card'>
          <Card.Content>
            <Card.Header className='router-card-header'>
              <div className='router-toolbar'>
                <Header as='h2' className='router-page-title'>
                  {t('topup.title')}
                </Header>
                <div
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '1rem',
                    flexWrap: 'wrap',
                    justifyContent: 'flex-end',
                  }}
                >
                  <div
                    style={{
                      display: 'inline-flex',
                      alignItems: 'baseline',
                      gap: '0.5rem',
                    }}
                  >
                    <span className='router-text-muted'>
                      {t('topup.external_topup.current_balance')}
                    </span>
                    <strong>{renderDisplayAmount(userBalanceYYC)}</strong>
                  </div>
                  <div
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: '0.5rem',
                    }}
                  >
                    <span className='router-text-muted'>
                      {t('topup.display_currency')}
                    </span>
                    <UnitDropdown
                      variant='inline'
                      compact
                      style={{ minWidth: '108px' }}
                      options={displayCurrencyOptions}
                      value={displayCurrency}
                      loading={loadingDisplayCurrencies}
                      disabled={
                        loadingDisplayCurrencies ||
                        displayCurrencyOptions.length === 0
                      }
                      onChange={(_, { value }) => {
                        const next = resolveDisplayCurrency(
                          displayCurrencyIndex,
                          value,
                        );
                        setDisplayCurrency(next);
                        storeDisplayCurrency(next);
                      }}
                    />
                  </div>
                </div>
              </div>
            </Card.Header>

            {activeContent}
          </Card.Content>
        </Card>
      </div>
    </TopUpWorkspaceContext.Provider>
  );
};

export default TopUpLayout;
