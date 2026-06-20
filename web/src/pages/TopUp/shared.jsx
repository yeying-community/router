import { createContext, useContext } from 'react';
import {
  convertChargeAmountToDisplayAmount,
  DEFAULT_FIAT_DISPLAY_CODE,
  buildPublicDisplayCurrencyIndex,
  normalizeDisplayCurrencyCode,
  resolvePreferredDisplayCurrency,
  YYC_DISPLAY_CODE,
} from '../../helpers/billing';
import { formatAmountWithUnit } from '../../helpers/render';
import { AppTag, AppTooltip } from '../../router-ui';

export const TOPUP_DISPLAY_CURRENCY_STORAGE_KEY = 'topup_display_currency';
export const TOPUP_DEFAULT_TAB = 'balance';
export const TOPUP_TAB_KEYS = ['balance', 'package', 'records'];
export const TOPUP_DEFAULT_RECORD = 'topup';
export const TOPUP_RECORD_KEYS = ['topup', 'package', 'redeem'];
export const TOPUP_ALLOWED_QUERY_KEYS = ['tab', 'record', 'intent'];
export const TopUpWorkspaceContext = createContext(null);

export const normalizeTopUpTab = (rawTab) =>
  TOPUP_TAB_KEYS.includes(rawTab) ? rawTab : TOPUP_DEFAULT_TAB;

export const normalizeTopUpRecord = (rawRecord) =>
  TOPUP_RECORD_KEYS.includes(rawRecord) ? rawRecord : TOPUP_DEFAULT_RECORD;

export const sanitizeTopUpSearchParams = (rawSearch = '') => {
  const source = new URLSearchParams(rawSearch || '');
  const next = new URLSearchParams();
  TOPUP_ALLOWED_QUERY_KEYS.forEach((key) => {
    const value = source.get(key);
    if (value) {
      next.set(key, value);
    }
  });
  return next;
};

export const buildTopUpReturnURL = () => {
  if (typeof window === 'undefined') {
    return '';
  }
  try {
    const currentURL = new URL(window.location.href);
    const nextURL = new URL(currentURL.origin + currentURL.pathname);
    const currentParams = sanitizeTopUpSearchParams(currentURL.search || '');
    currentParams.forEach((value, key) => {
      nextURL.searchParams.set(key, value);
    });
    return nextURL.toString();
  } catch (error) {
    return window.location.origin + window.location.pathname;
  }
};

export const normalizeTopUpResult = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  const redeemedAmount = Number(raw?.redeemed_amount ?? 0) || 0;
  const beforeBalanceAmount = Number(raw?.before_balance_amount ?? 0) || 0;
  const afterBalanceAmount = Number(raw?.after_balance_amount ?? 0) || 0;
  return {
    redeemed_amount: redeemedAmount,
    before_balance_amount: beforeBalanceAmount,
    after_balance_amount: afterBalanceAmount,
    redemption_id: raw?.redemption_id || '',
    redemption_name: raw?.redemption_name || '',
    group_id: raw?.group_id || '',
    group_name: raw?.group_name || '',
    face_value_amount: Number(raw?.face_value_amount ?? 0) || 0,
    face_value_unit: raw?.face_value_unit || '',
    redeemed_at: Number(raw?.redeemed_at ?? 0) || 0,
    credit_expires_at: Number(raw?.credit_expires_at ?? 0) || 0,
  };
};

export const normalizeRedemptionRecord = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  const redeemedTime = Number(raw?.redeemed_time ?? raw?.redeemed_at ?? 0) || 0;
  const createdAt = Number(raw?.created_at ?? 0) || 0;
  const normalizedTime = redeemedTime || createdAt;
  return {
    ...raw,
    created_at: normalizedTime,
    chargeAmount: Number(raw?.charge_amount ?? raw?.credit_amount ?? raw?.quota ?? 0) || 0,
    redemptionName:
      String(raw?.redemption_name || raw?.name || '').trim(),
    redemptionCode: String(raw?.code || '').trim(),
    groupName: String(raw?.group_name || '').trim(),
    faceValueAmount: Number(raw?.face_value_amount ?? 0) || 0,
    faceValueUnit: String(raw?.face_value_unit || '').trim().toUpperCase(),
    detailText: String(raw?.content || '').trim(),
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
        <AppTag color='blue' className='router-tag'>
          {t('topup.external_topup_orders.status.created')}
        </AppTag>
      );
    case 'pending':
      return (
        <AppTag color='orange' className='router-tag'>
          {t('topup.external_topup_orders.status.pending')}
        </AppTag>
      );
    case 'paid':
      return (
        <AppTag color='teal' className='router-tag'>
          {t('topup.external_topup_orders.status.paid')}
        </AppTag>
      );
    case 'fulfilled':
      return (
        <AppTag color='green' className='router-tag'>
          {t('topup.external_topup_orders.status.fulfilled')}
        </AppTag>
      );
    case 'failed':
      return (
        <AppTag color='red' className='router-tag'>
          {t('topup.external_topup_orders.status.failed')}
        </AppTag>
      );
    case 'canceled':
      return (
        <AppTag className='router-tag'>
          {t('topup.external_topup_orders.status.canceled')}
        </AppTag>
      );
    default:
      return <AppTag className='router-tag'>{status || '-'}</AppTag>;
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

const splitTopupDisplayTextUnit = (displayText) => {
  const trimmed = String(displayText || '').trim();
  if (trimmed === '') {
    return {
      amount: '-',
      unit: '',
    };
  }
  // YYC already contains leading symbol, keep as a single text block.
  if (trimmed.startsWith('Ɏ ')) {
    return {
      amount: trimmed,
      unit: '',
    };
  }
  const separatorIndex = trimmed.lastIndexOf(' ');
  if (separatorIndex <= 0) {
    return {
      amount: trimmed,
      unit: '',
    };
  }
  const amount = trimmed.slice(0, separatorIndex).trim();
  const unit = trimmed.slice(separatorIndex + 1).trim().toUpperCase();
  if (!/^[A-Z]{2,8}$/.test(unit)) {
    return {
      amount: trimmed,
      unit: '',
    };
  }
  return {
    amount,
    unit,
  };
};

const renderTopupAmountTrigger = (displayText) => {
  const { amount, unit } = splitTopupDisplayTextUnit(displayText);
  if (!unit) {
    return <span>{amount}</span>;
  }
  return (
    <span className='router-topup-amount-trigger'>
      <span>{amount}</span>
      <span className='router-topup-amount-unit'>{unit}</span>
    </span>
  );
};

const formatTopupAmountWithFixedFraction = (amount, unit, fractionDigits = 2) => {
  const normalizedUnit = (unit || '').toString().trim().toUpperCase();
  const normalizedAmount = Number(amount);
  if (!Number.isFinite(normalizedAmount)) {
    return '-';
  }
  const display = new Intl.NumberFormat(undefined, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(normalizedAmount);
  return normalizedUnit ? `${display} ${normalizedUnit}` : display;
};

const buildTopupAmountDisplayTexts = ({
  chargeAmount,
  displayCurrency,
  displayCurrencyIndex,
  displayFractionDigits = 2,
  exactFractionDigits = 6,
}) => {
  const normalizedChargeAmount = Number(chargeAmount ?? 0);
  if (!Number.isFinite(normalizedChargeAmount)) {
    return {
      displayText: '-',
      exactText: '-',
    };
  }
  const normalizedDisplayCurrency = normalizeDisplayCurrencyCode(displayCurrency);
  if (normalizedDisplayCurrency === YYC_DISPLAY_CODE) {
    return {
      displayText: formatTopupAmountWithFixedFraction(
        normalizedChargeAmount,
        YYC_DISPLAY_CODE,
        displayFractionDigits,
      ),
      exactText: formatAmountWithUnit(
        normalizedChargeAmount,
        YYC_DISPLAY_CODE,
        exactFractionDigits,
      ),
    };
  }
  const convertedAmount = convertChargeAmountToDisplayAmount(
    normalizedChargeAmount,
    normalizedDisplayCurrency,
    displayCurrencyIndex,
  );
  if (!Number.isFinite(convertedAmount)) {
    return {
      displayText: '-',
      exactText: '-',
    };
  }
  return {
    displayText: formatTopupAmountWithFixedFraction(
      convertedAmount,
      normalizedDisplayCurrency,
      displayFractionDigits,
    ),
    exactText: formatAmountWithUnit(
      convertedAmount,
      normalizedDisplayCurrency,
      exactFractionDigits,
    ),
  };
};

export const renderTopupIntegerAmountWithExactPopup = ({
  chargeAmount,
  displayCurrency,
  displayCurrencyIndex,
  displayFractionDigits = 2,
  exactFractionDigits = 6,
}) => {
  const { displayText, exactText } = buildTopupAmountDisplayTexts({
    chargeAmount,
    displayCurrency,
    displayCurrencyIndex,
    displayFractionDigits,
    exactFractionDigits,
  });
  if (displayText === '-') {
    return '-';
  }
  return (
    <AppTooltip title={exactText}>
      {renderTopupAmountTrigger(displayText)}
    </AppTooltip>
  );
};

export const useTopUpWorkspace = () => useContext(TopUpWorkspaceContext);

export { YYC_DISPLAY_CODE };
