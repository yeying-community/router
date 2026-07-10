import React from 'react';
import { timestamp2string } from '../../helpers';
import { formatRequestCount } from '../../helpers/package';
import { AppTag } from '../../router-ui';

const renderQuotaCardStatus = (status, t) => {
  switch (String(status || '').trim()) {
    case 'active':
      return <AppTag color='green'>{t('topup.quota_cards.status.active')}</AppTag>;
    case 'expired':
      return <AppTag color='grey'>{t('topup.quota_cards.status.expired')}</AppTag>;
    case 'exhausted':
      return <AppTag color='grey'>{t('topup.quota_cards.status.exhausted')}</AppTag>;
    case 'replaced':
      return <AppTag color='blue'>{t('topup.quota_cards.status.replaced')}</AppTag>;
    case 'canceled':
      return <AppTag color='red'>{t('topup.quota_cards.status.canceled')}</AppTag>;
    default:
      return <AppTag>{t('topup.quota_cards.status.unknown')}</AppTag>;
  }
};

const getQuotaCardKindLabel = (kind, t) => {
  switch (String(kind || '').trim()) {
    case 'package':
      return t('topup.quota_cards.kind.package');
    case 'redemption':
      return t('topup.quota_cards.kind.redemption');
    case 'gift':
      return t('topup.quota_cards.kind.gift');
    case 'topup':
    default:
      return t('topup.quota_cards.kind.topup');
  }
};

const QuotaCardItem = ({ card, renderAmount, onClick, t }) => {
  const isRequestCount = card?.metric === 'request_count';
  const active = card?.status === 'active';
  const unlimited =
    isRequestCount &&
    (card?.package?.usage?.unlimited === true ||
      Number(card?.package?.period_limit || 0) <= 0);
  const formatValue = (value, allowUnlimited = false) => {
    if (isRequestCount) {
      if (allowUnlimited && unlimited) {
        return t('common.unlimited');
      }
      return `${formatRequestCount(value)} ${t('package_manage.request_unit')}`;
    }
    return renderAmount(value);
  };
  const primaryValue = active ? card?.remaining_amount : card?.total_amount;
  const primaryLabel = active
    ? t('topup.quota_cards.remaining')
    : t('topup.quota_cards.total');

  return (
    <button
      type='button'
      className='router-quota-card'
      onClick={() => onClick?.(card)}
    >
      <div className='router-quota-card-header'>
        <div className='router-quota-card-heading'>
          <span className='router-quota-card-kind'>
            {getQuotaCardKindLabel(card?.kind, t)}
          </span>
          <span className='router-quota-card-name'>{card?.name || '-'}</span>
        </div>
        {renderQuotaCardStatus(card?.status, t)}
      </div>
      <div className='router-quota-card-primary-label'>{primaryLabel}</div>
      <div className='router-quota-card-primary-value'>
        {formatValue(primaryValue, active)}
      </div>
      <div className='router-quota-card-metrics'>
        <div>
          <span>{t('topup.quota_cards.total')}</span>
          <strong>{formatValue(card?.total_amount, true)}</strong>
        </div>
        <div>
          <span>{t('topup.quota_cards.used')}</span>
          <strong>{formatValue(card?.used_amount)}</strong>
        </div>
      </div>
      <div className='router-quota-card-footer'>
        <span>
          {t('topup.quota_cards.activated_at')}:{' '}
          {card?.activated_at ? timestamp2string(card.activated_at) : '-'}
        </span>
        <span>
          {t('topup.quota_cards.expires_at')}:{' '}
          {Number(card?.expires_at || 0) > 0
            ? timestamp2string(card.expires_at)
            : t('common.never')}
        </span>
      </div>
    </button>
  );
};

export default QuotaCardItem;
