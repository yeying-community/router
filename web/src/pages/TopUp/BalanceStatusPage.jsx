import React, { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { timestamp2string } from '../../helpers';
import {
  BALANCE_LOT_COLUMN_WIDTHS,
  BALANCE_LOT_TABLE_MIN_WIDTH,
} from '../../constants/tableWidthPresets';
import RedeemCodePage from './RedeemCodePage';
import {
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';
import {
  AppButton,
  AppSection,
  AppStatistic,
  AppTable,
  AppTag,
} from '../../router-ui';

const formatLotSource = (source, t) => {
  switch ((source || '').trim()) {
    case 'topup_order':
      return t('topup.balance_lots.source.topup_order');
    case 'redemption':
      return t('topup.balance_lots.source.redemption');
    case 'legacy_migration':
      return t('topup.balance_lots.source.legacy_migration');
    default:
      return source || '-';
  }
};

const renderLotStatus = (status, t) => {
  switch ((status || '').trim()) {
    case 'active':
      return (
        <AppTag color='green' className='router-tag'>
          {t('topup.balance_lots.status.active')}
        </AppTag>
      );
    case 'exhausted':
      return (
        <AppTag color='grey' className='router-tag'>
          {t('topup.balance_lots.status.exhausted')}
        </AppTag>
      );
    case 'expired':
      return (
        <AppTag color='orange' className='router-tag'>
          {t('topup.balance_lots.status.expired')}
        </AppTag>
      );
    default:
      return <AppTag className='router-tag'>{status || '-'}</AppTag>;
  }
};

const BalanceStatusPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [redeemModalOpen, setRedeemModalOpen] = useState(false);
  const {
    userBalanceAmount,
    topupBalanceAmount,
    redeemBalanceAmount,
    balanceLots,
    loadingBalanceLots,
    loadBalanceLots,
    displayCurrency,
    displayCurrencyIndex,
  } = useTopUpWorkspace();
  const balanceLotColumns = useMemo(
    () => [
      {
        title: t('topup.balance_lots.columns.source'),
        dataIndex: 'source_type',
        key: 'source_type',
        width: BALANCE_LOT_COLUMN_WIDTHS.source,
        render: (value) => formatLotSource(value, t),
      },
      {
        title: t('topup.balance_lots.columns.remaining'),
        dataIndex: 'remaining_amount',
        key: 'remaining_amount',
        width: BALANCE_LOT_COLUMN_WIDTHS.remaining,
        render: (_, row) =>
          renderTopupIntegerAmountWithExactPopup({
            chargeAmount: row.remaining_amount,
            displayCurrency,
            displayCurrencyIndex,
          }),
      },
      {
        title: t('topup.balance_lots.columns.total'),
        dataIndex: 'total_amount',
        key: 'total_amount',
        width: BALANCE_LOT_COLUMN_WIDTHS.total,
        render: (_, row) =>
          renderTopupIntegerAmountWithExactPopup({
            chargeAmount: row.total_amount,
            displayCurrency,
            displayCurrencyIndex,
          }),
      },
      {
        title: t('topup.balance_lots.columns.status'),
        dataIndex: 'status',
        key: 'status',
        className: 'router-table-col-status-compact',
        width: BALANCE_LOT_COLUMN_WIDTHS.status,
        render: (value) => renderLotStatus(value, t),
      },
      {
        title: t('topup.balance_lots.columns.granted_at'),
        dataIndex: 'granted_at',
        key: 'granted_at',
        className: 'router-table-col-datetime',
        width: BALANCE_LOT_COLUMN_WIDTHS.grantedAt,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('topup.balance_lots.columns.expires_at'),
        dataIndex: 'expires_at',
        key: 'expires_at',
        className: 'router-table-col-datetime',
        width: BALANCE_LOT_COLUMN_WIDTHS.expiresAt,
        render: (value) =>
          Number(value || 0) > 0 ? timestamp2string(value) : t('common.never'),
      },
    ],
    [displayCurrency, displayCurrencyIndex, t],
  );

  return (
    <div className='router-topup-balance-layout'>
      <AppSection className='router-section-fill'>
        <div className='router-section-stack-spread'>
          <div className='router-topup-balance-stat-grid'>
            <div className='router-center-panel router-center-panel-tight'>
              <AppStatistic
                className='router-accent-statistic router-topup-statistic'
                title={t('topup.external_topup.total_balance')}
                value={0}
                formatter={() =>
                  renderTopupIntegerAmountWithExactPopup({
                    chargeAmount: userBalanceAmount,
                    displayCurrency,
                    displayCurrencyIndex,
                  })
                }
              />
            </div>
            <div className='router-center-panel router-center-panel-tight'>
              <AppStatistic
                className='router-topup-statistic'
                title={t('topup.external_topup.topup_balance')}
                value={0}
                formatter={() =>
                  renderTopupIntegerAmountWithExactPopup({
                    chargeAmount: topupBalanceAmount,
                    displayCurrency,
                    displayCurrencyIndex,
                  })
                }
              />
            </div>
            <div className='router-center-panel router-center-panel-tight'>
              <AppStatistic
                className='router-topup-statistic'
                title={t('topup.external_topup.redeem_balance')}
                value={0}
                formatter={() =>
                  renderTopupIntegerAmountWithExactPopup({
                    chargeAmount: redeemBalanceAmount,
                    displayCurrency,
                    displayCurrencyIndex,
                  })
                }
              />
            </div>
          </div>
          <div className='router-action-footer router-topup-balance-actions'>
            <AppButton
              color='blue'
              className='router-section-button router-topup-balance-action-button'
              onClick={() => navigate('/workspace/service/pricing')}
            >
              {t('topup.record_nav.topup')}
            </AppButton>
            <AppButton
              className='router-section-button router-topup-balance-action-button'
              onClick={() => setRedeemModalOpen(true)}
            >
              {t('topup.record_nav.redeem')}
            </AppButton>
          </div>
        </div>
      </AppSection>
      <RedeemCodePage
        open={redeemModalOpen}
        onClose={() => setRedeemModalOpen(false)}
        onRedeemed={() => loadBalanceLots()}
      />
      <AppSection
        title={t('topup.balance_lots.title')}
        extra={
          <AppButton
            className='router-section-button'
            loading={loadingBalanceLots}
            onClick={() => loadBalanceLots()}
          >
            {t('common.refresh')}
          </AppButton>
        }
      >
        {balanceLots.length === 0 ? (
          <div className='router-empty'>{t('topup.balance_lots.empty')}</div>
        ) : (
          <div className='router-table-scroll-x'>
            <AppTable
              className='router-list-table router-table-fit-page'
              rowKey={(row) => row.id || `${row.source_type}-${row.source_id}`}
              pagination={false}
              scroll={{ x: BALANCE_LOT_TABLE_MIN_WIDTH }}
              dataSource={balanceLots}
              columns={balanceLotColumns}
            />
          </div>
        )}
      </AppSection>
    </div>
  );
};

export default BalanceStatusPage;
