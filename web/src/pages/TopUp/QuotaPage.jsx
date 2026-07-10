import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { API, showError } from '../../helpers';
import {
  formatRequestCount,
  getServicePackagePeriodLabel,
  isRequestQuotaPackage,
} from '../../helpers/package';
import {
  AppButton,
  AppModal,
  AppSection,
  AppStatistic,
  AppTabs,
} from '../../router-ui';
import BalanceStatusPage from './BalanceStatusPage';
import CurrentPackagePage from './CurrentPackagePage';
import TopUpRecordsPage from './TopUpRecordsPage';
import {
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';

const HISTORY_KEYS = ['topup', 'package', 'redeem', 'gift'];

const normalizeActivePackages = (raw) =>
  Array.isArray(raw?.active_packages)
    ? raw.active_packages.filter((item) => item && typeof item === 'object')
    : [];

const QuotaPage = ({ historyKey = 'topup' }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { displayCurrency, displayCurrencyIndex, loadBalanceSummary } =
    useTopUpWorkspace();
  const [overview, setOverview] = useState(null);
  const [activePackages, setActivePackages] = useState([]);
  const [loading, setLoading] = useState(false);
  const [detailModal, setDetailModal] = useState('');

  const loadQuotaOverview = useCallback(
    async ({ silent = false } = {}) => {
      if (!silent) {
        setLoading(true);
      }
      try {
        const [overviewResponse, packageResponse] = await Promise.all([
          API.get('/api/v1/public/user/quota/overview'),
          API.get('/api/v1/public/user/package/subscription'),
        ]);
        const overviewPayload = overviewResponse?.data || {};
        const packagePayload = packageResponse?.data || {};
        if (!overviewPayload.success) {
          throw new Error(
            overviewPayload.message ||
              t('topup.quota_overview.load_failed', '加载额度总览失败')
          );
        }
        if (!packagePayload.success) {
          throw new Error(
            packagePayload.message ||
              t('user.messages.active_package_load_failed')
          );
        }
        setOverview(overviewPayload.data || null);
        setActivePackages(normalizeActivePackages(packagePayload.data));
      } catch (error) {
        if (!silent) {
          showError(
            error?.message ||
              t('topup.quota_overview.load_failed', '加载额度总览失败')
          );
        }
      } finally {
        if (!silent) {
          setLoading(false);
        }
      }
    },
    [t]
  );

  useEffect(() => {
    loadQuotaOverview().then();
  }, [loadQuotaOverview]);

  const renderAmount = useCallback(
    (amount) =>
      renderTopupIntegerAmountWithExactPopup({
        chargeAmount: Number(amount || 0),
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex]
  );

  const requestQuotaPackages = useMemo(
    () => activePackages.filter((item) => isRequestQuotaPackage(item)),
    [activePackages]
  );

  const normalizedHistoryKey = HISTORY_KEYS.includes(historyKey)
    ? historyKey
    : 'topup';

  const handleHistoryChange = useCallback(
    (nextHistoryKey) => {
      const nextSearchParams = new URLSearchParams(searchParams.toString());
      nextSearchParams.set('tab', 'quota');
      nextSearchParams.set('history', nextHistoryKey);
      nextSearchParams.delete('record');
      navigate(`/workspace/topup?${nextSearchParams.toString()}`);
    },
    [navigate, searchParams]
  );

  const historyItems = useMemo(
    () => [
      {
        key: 'topup',
        label: t('topup.record_nav.topup'),
        children:
          normalizedHistoryKey === 'topup' ? (
            <TopUpRecordsPage recordKey='topup' embedded />
          ) : null,
      },
      {
        key: 'package',
        label: t('topup.record_nav.package'),
        children:
          normalizedHistoryKey === 'package' ? (
            <TopUpRecordsPage recordKey='package' embedded />
          ) : null,
      },
      {
        key: 'redeem',
        label: t('topup.record_nav.redeem'),
        children:
          normalizedHistoryKey === 'redeem' ? (
            <TopUpRecordsPage recordKey='redeem' embedded />
          ) : null,
      },
      {
        key: 'gift',
        label: t('topup.record_nav.gift', '赠送'),
        children:
          normalizedHistoryKey === 'gift' ? (
            <TopUpRecordsPage recordKey='gift' embedded />
          ) : null,
      },
    ],
    [normalizedHistoryKey, t]
  );

  const closeDetailModal = useCallback(() => {
    setDetailModal('');
    Promise.all([
      loadQuotaOverview({ silent: true }),
      loadBalanceSummary({ silent: true }),
    ]).then();
  }, [loadBalanceSummary, loadQuotaOverview]);

  const totalAmount = Number(overview?.total_amount || 0);
  const usedAmount = Number(overview?.used_amount || 0);
  const remainingAmount = Number(overview?.remaining_amount || 0);
  const packageOverview = overview?.package || {};
  const balanceOverview = overview?.balance || {};

  return (
    <div className='router-topup-quota-layout'>
      <AppSection
        title={t('topup.quota_overview.title', '今日额度总览')}
        extra={
          <AppButton
            className='router-section-button'
            loading={loading}
            onClick={() => loadQuotaOverview()}
          >
            {t('common.refresh')}
          </AppButton>
        }
      >
        <div className='router-quota-summary-grid'>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-accent-statistic router-topup-statistic'
              title={t('topup.quota_overview.total', '总额度')}
              value={0}
              formatter={() => renderAmount(totalAmount)}
            />
          </div>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-topup-statistic'
              title={t('topup.quota_overview.used', '已用额度')}
              value={0}
              formatter={() => renderAmount(usedAmount)}
            />
          </div>
          <div className='router-quota-summary-item'>
            <AppStatistic
              className='router-topup-statistic'
              title={t('topup.quota_overview.remaining', '剩余额度')}
              value={0}
              formatter={() => renderAmount(remainingAmount)}
            />
          </div>
        </div>
        <div className='router-form-hint router-quota-summary-hint'>
          {t(
            'topup.quota_overview.hint',
            '金额额度合并计算今日套餐额度与余额；套餐预留中的额度计入已用额度。'
          )}
        </div>
      </AppSection>

      {requestQuotaPackages.length > 0 ? (
        <AppSection title={t('topup.quota_overview.count_title', '次数额度')}>
          <div className='router-quota-count-grid'>
            {requestQuotaPackages.map((item) => {
              const usage = item?.usage || {};
              const unlimited = usage?.unlimited === true;
              const limit = Number(
                usage?.limit_amount ?? item?.period_limit ?? 0
              );
              const used = Number(usage?.consumed_amount || 0);
              const reserved = Number(usage?.reserved_amount || 0);
              const remaining = Number(usage?.remaining_amount || 0);
              return (
                <div
                  key={item?.id || item?.package_id}
                  className='router-quota-count-card'
                >
                  <div className='router-quota-count-header'>
                    <div className='router-quota-count-title'>
                      {item?.package_name || item?.package_id || '-'}
                    </div>
                    <div className='router-text-muted'>
                      {getServicePackagePeriodLabel(item?.period_type, t)}
                    </div>
                  </div>
                  <div className='router-quota-count-values'>
                    <div>
                      <span className='router-text-muted'>
                        {t('topup.quota_overview.count_total', '总次数')}
                      </span>
                      <strong>
                        {unlimited
                          ? t('common.unlimited')
                          : formatRequestCount(limit)}
                      </strong>
                    </div>
                    <div>
                      <span className='router-text-muted'>
                        {t('topup.quota_overview.count_used', '已用')}
                      </span>
                      <strong>{formatRequestCount(used + reserved)}</strong>
                    </div>
                    <div>
                      <span className='router-text-muted'>
                        {t('topup.quota_overview.count_remaining', '剩余')}
                      </span>
                      <strong>
                        {unlimited
                          ? t('common.unlimited')
                          : formatRequestCount(remaining)}
                      </strong>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </AppSection>
      ) : null}

      <AppSection title={t('topup.quota_overview.sources', '额度来源')}>
        <div className='router-quota-source-grid'>
          <button
            type='button'
            className='router-quota-source-card'
            onClick={() => setDetailModal('package')}
          >
            <div className='router-quota-source-card-header'>
              <span>{t('topup.quota_overview.package_card', '套餐额度')}</span>
              <span className='router-quota-source-card-action'>
                {t('topup.quota_overview.view_detail', '查看详情')}
              </span>
            </div>
            <div className='router-quota-source-card-value'>
              {renderAmount(packageOverview?.remaining_amount || 0)}
            </div>
            <div className='router-quota-source-card-meta'>
              <span>
                {t('topup.quota_overview.package_total', '今日套餐')}{' '}
                {renderAmount(packageOverview?.limit_amount || 0)}
              </span>
              <span>
                {t('topup.quota_overview.package_used', '已用')}{' '}
                {renderAmount(
                  Number(packageOverview?.consumed_amount || 0) +
                    Number(packageOverview?.reserved_amount || 0)
                )}
              </span>
            </div>
          </button>

          <button
            type='button'
            className='router-quota-source-card'
            onClick={() => setDetailModal('balance')}
          >
            <div className='router-quota-source-card-header'>
              <span>{t('topup.quota_overview.balance_card', '余额')}</span>
              <span className='router-quota-source-card-action'>
                {t('topup.quota_overview.view_detail', '查看详情')}
              </span>
            </div>
            <div className='router-quota-source-card-value'>
              {renderAmount(balanceOverview?.current_amount || 0)}
            </div>
            <div className='router-quota-source-card-meta'>
              <span>
                {t('topup.quota_overview.balance_used_today', '今日已用')}{' '}
                {renderAmount(balanceOverview?.consumed_today_amount || 0)}
              </span>
              <span>
                {t('topup.quota_overview.balance_gift', '含赠送')}{' '}
                {renderAmount(balanceOverview?.gift_balance_amount || 0)}
              </span>
            </div>
          </button>
        </div>
      </AppSection>

      <AppSection title={t('topup.quota_overview.history', '历史流水')}>
        <AppTabs
          activeKey={normalizedHistoryKey}
          onChange={handleHistoryChange}
          items={historyItems}
        />
      </AppSection>

      <AppModal
        size='large'
        open={detailModal === 'package'}
        onClose={closeDetailModal}
        title={t('topup.quota_overview.package_detail', '套餐详情')}
        footer={[
          <AppButton key='close' onClick={closeDetailModal}>
            {t('common.close')}
          </AppButton>,
        ]}
      >
        <CurrentPackagePage />
      </AppModal>

      <AppModal
        size='large'
        open={detailModal === 'balance'}
        onClose={closeDetailModal}
        title={t('topup.quota_overview.balance_detail', '余额详情')}
        footer={[
          <AppButton key='close' onClick={closeDetailModal}>
            {t('common.close')}
          </AppButton>,
        ]}
      >
        <BalanceStatusPage />
      </AppModal>
    </div>
  );
};

export default QuotaPage;
