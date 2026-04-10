import { createContext, useContext } from 'react';
import { Label, Popup } from 'semantic-ui-react';
import {
  convertYYCToDisplayAmount,
  DEFAULT_FIAT_DISPLAY_CODE,
  buildPublicDisplayCurrencyIndex,
  normalizeDisplayCurrencyCode,
  resolvePreferredDisplayCurrency,
  YYC_DISPLAY_CODE,
} from '../../helpers/billing';
import { formatAmountWithUnit } from '../../helpers/render';

export const TOPUP_DISPLAY_CURRENCY_STORAGE_KEY = 'topup_display_currency';
export const TOPUP_DEFAULT_TAB = 'balance';
export const TOPUP_TAB_KEYS = ['balance', 'package', 'records'];
export const TOPUP_DEFAULT_RECORD = 'topup';
export const TOPUP_RECORD_KEYS = ['topup', 'package', 'redeem'];
export const TopUpWorkspaceContext = createContext(null);

export const normalizeTopUpTab = (rawTab) =>
  TOPUP_TAB_KEYS.includes(rawTab) ? rawTab : TOPUP_DEFAULT_TAB;

export const normalizeTopUpRecord = (rawRecord) =>
  TOPUP_RECORD_KEYS.includes(rawRecord) ? rawRecord : TOPUP_DEFAULT_RECORD;

export const normalizeTopUpResult = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  const redeemedYYC = Number(raw?.redeemed_yyc ?? 0) || 0;
  const beforeYYCBalance = Number(raw?.before_yyc_balance ?? 0) || 0;
  const afterYYCBalance = Number(raw?.after_yyc_balance ?? 0) || 0;
  return {
    redeemed_yyc: redeemedYYC,
    before_yyc_balance: beforeYYCBalance,
    after_yyc_balance: afterYYCBalance,
    redemption_id: raw?.redemption_id || '',
    redemption_name: raw?.redemption_name || '',
    group_id: raw?.group_id || '',
    group_name: raw?.group_name || '',
    face_value_amount: Number(raw?.face_value_amount ?? 0) || 0,
    face_value_unit: raw?.face_value_unit || '',
    redeemed_at: Number(raw?.redeemed_at ?? 0) || 0,
  };
};

export const normalizeRedemptionRecord = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  return {
    ...raw,
    yycAmount: Number(raw?.yyc_amount ?? raw?.quota ?? 0) || 0,
  };
};

export const getStoredDisplayCurrency = () => {
  if (typeof window === 'undefined') {
    return '';
  }
  return normalizeDisplayCurrencyCode(
    window.localStorage.getItem(TOPUP_DISPLAY_CURRENCY_STORAGE_KEY),
  );
};

export const storeDisplayCurrency = (code) => {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(
    TOPUP_DISPLAY_CURRENCY_STORAGE_KEY,
    normalizeDisplayCurrencyCode(code),
  );
};

export const resolveDisplayCurrency = (currencyIndex, current = '') =>
  resolvePreferredDisplayCurrency(
    currencyIndex,
    current || getStoredDisplayCurrency() || DEFAULT_FIAT_DISPLAY_CODE,
  );

export const getStoredStatusConfig = () => {
  if (typeof window === 'undefined') {
    return {};
  }
  try {
    const raw = window.localStorage.getItem('status');
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (error) {
    return {};
  }
};

export const buildInitialDisplayCurrencyIndex = () =>
  buildPublicDisplayCurrencyIndex([]);

export const renderTopupOrderStatus = (status, t) => {
  switch (status) {
    case 'created':
      return (
        <Label basic color='blue' className='router-tag'>
          {t('topup.external_topup_orders.status.created')}
        </Label>
      );
    case 'pending':
      return (
        <Label basic color='orange' className='router-tag'>
          {t('topup.external_topup_orders.status.pending')}
        </Label>
      );
    case 'paid':
      return (
        <Label basic color='teal' className='router-tag'>
          {t('topup.external_topup_orders.status.paid')}
        </Label>
      );
    case 'fulfilled':
      return (
        <Label basic color='green' className='router-tag'>
          {t('topup.external_topup_orders.status.fulfilled')}
        </Label>
      );
    case 'failed':
      return (
        <Label basic color='red' className='router-tag'>
          {t('topup.external_topup_orders.status.failed')}
        </Label>
      );
    case 'canceled':
      return (
        <Label basic className='router-tag'>
          {t('topup.external_topup_orders.status.canceled')}
        </Label>
      );
    default:
      return <Label basic className='router-tag'>{status || '-'}</Label>;
  }
};

export const formatTopupOrderStatusHint = (status, t) => {
  switch ((status || '').trim()) {
    case 'created':
      return t('topup.external_topup_orders.status_hint.created');
    case 'pending':
      return t('topup.external_topup_orders.status_hint.pending');
    case 'paid':
      return t('topup.external_topup_orders.status_hint.paid');
    case 'fulfilled':
      return t('topup.external_topup_orders.status_hint.fulfilled');
    case 'failed':
      return t('topup.external_topup_orders.status_hint.failed');
    case 'canceled':
      return t('topup.external_topup_orders.status_hint.canceled');
    default:
      return '';
  }
};

export const formatTopupBusinessType = (type, t) => {
  switch ((type || '').trim()) {
    case 'balance_topup':
      return t('topup.business_type.balance_topup');
    case 'package_purchase':
      return t('topup.business_type.package_purchase');
    default:
      return type || '-';
  }
};

const buildTopupAmountDisplayTexts = ({
  yycAmount,
  displayCurrency,
  displayCurrencyIndex,
  exactFractionDigits = 6,
}) => {
  const normalizedYYCAmount = Number(yycAmount ?? 0);
  if (!Number.isFinite(normalizedYYCAmount)) {
    return {
      integerText: '-',
      exactText: '-',
    };
  }
  const normalizedDisplayCurrency = normalizeDisplayCurrencyCode(displayCurrency);
  if (normalizedDisplayCurrency === YYC_DISPLAY_CODE) {
    return {
      integerText: formatAmountWithUnit(
        Math.round(normalizedYYCAmount),
        YYC_DISPLAY_CODE,
        0,
      ),
      exactText: formatAmountWithUnit(
        normalizedYYCAmount,
        YYC_DISPLAY_CODE,
        exactFractionDigits,
      ),
    };
  }
  const convertedAmount = convertYYCToDisplayAmount(
    normalizedYYCAmount,
    normalizedDisplayCurrency,
    displayCurrencyIndex,
  );
  if (!Number.isFinite(convertedAmount)) {
    return {
      integerText: '-',
      exactText: '-',
    };
  }
  return {
    integerText: formatAmountWithUnit(
      Math.round(convertedAmount),
      normalizedDisplayCurrency,
      0,
    ),
    exactText: formatAmountWithUnit(
      convertedAmount,
      normalizedDisplayCurrency,
      exactFractionDigits,
    ),
  };
};

export const renderTopupIntegerAmountWithExactPopup = ({
  yycAmount,
  displayCurrency,
  displayCurrencyIndex,
  exactFractionDigits = 6,
}) => {
  const { integerText, exactText } = buildTopupAmountDisplayTexts({
    yycAmount,
    displayCurrency,
    displayCurrencyIndex,
    exactFractionDigits,
  });
  if (integerText === '-') {
    return '-';
  }
  return (
    <Popup
      content={exactText}
      trigger={<span>{integerText}</span>}
    />
  );
};

export const useTopUpWorkspace = () => useContext(TopUpWorkspaceContext);

export { YYC_DISPLAY_CODE };
