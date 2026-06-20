import { getChannelProtocolOption } from './helper';
import React from 'react';
import { AppAlert, AppTag, AppTooltip } from '../router-ui';

export const YYC_SYMBOL = 'Ɏ';

export function renderText(text, limit) {
  if (text.length > limit) {
    return text.slice(0, limit - 3) + '...';
  }
  return text;
}

export function renderGroup(group) {
  if (group === '') {
    return '-';
  }
  let groups = group.split(',');
  groups.sort();
  return (
    <div className='router-group-tag-list'>
      {groups.map((group) => {
        if (group === 'vip' || group === 'pro') {
          return (
            <AppTag key={group} color='yellow'>
              {group}
            </AppTag>
          );
        } else if (group === 'svip' || group === 'premium') {
          return (
            <AppTag key={group} color='red'>
              {group}
            </AppTag>
          );
        }
        return <AppTag key={group}>{group}</AppTag>;
      })}
    </div>
  );
}

export function renderNumber(num) {
  if (num >= 1000000000) {
    return (num / 1000000000).toFixed(1) + 'B';
  } else if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M';
  } else if (num >= 10000) {
    return (num / 1000).toFixed(1) + 'k';
  } else {
    return num;
  }
}

export function formatCompactNumber(value) {
  const num = Number(value);
  if (!Number.isFinite(num)) return '0';
  const abs = Math.abs(num);
  const units = [
    { value: 1e20, symbol: '垓' },
    { value: 1e16, symbol: '京' },
    { value: 1e12, symbol: '兆' },
    { value: 1e8, symbol: '亿' },
    { value: 1e4, symbol: '万' },
  ];
  let unit = '';
  let display = num;
  for (const item of units) {
    if (abs >= item.value) {
      unit = item.symbol;
      display = num / item.value;
      break;
    }
  }
  const absDisplay = Math.abs(display);
  const integerDigits =
    absDisplay >= 1 ? Math.floor(Math.log10(absDisplay)) + 1 : 0;
  const decimals = Math.max(0, 4 - integerDigits);
  const factor = 10 ** decimals;
  const truncated = Math.trunc(display * factor) / factor;
  let text = truncated.toFixed(decimals).replace(/\.?0+$/, '');
  return `${text}${unit}`;
}

export function renderDisplayAmount(chargeAmount, t, precision = 2) {
  const chargeRate = getChargeRateValue();
  if (chargeRate > 0 && typeof t === 'function') {
    const amount = (chargeAmount / chargeRate).toFixed(precision);
    return t('common.quota.display_short', { amount });
  }

  return renderNumber(chargeAmount);
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const renderQuota = renderDisplayAmount;

export function isChargeDisplayedInCurrency() {
  return getChargeRateValue() > 0;
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const isQuotaDisplayedInCurrency = isChargeDisplayedInCurrency;

export function getChargeRateValue() {
  if (typeof window === 'undefined') {
    return 0;
  }
  const value = parseFloat(localStorage.getItem('quota_per_unit') || '0');
  if (!Number.isFinite(value) || value <= 0) {
    return 0;
  }
  return value;
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const getQuotaPerUnitValue = getChargeRateValue;

export function formatChargeEquivalentAmount(chargeAmount, precision = 6) {
  const normalized = Number(chargeAmount || 0);
  const chargeRate = getChargeRateValue();
  if (!Number.isFinite(normalized)) {
    return '';
  }
  if (!Number.isFinite(chargeRate) || chargeRate <= 0) {
    return '';
  }
  return (normalized / chargeRate)
    .toFixed(precision)
    .replace(/\.?0+$/, '');
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const formatQuotaEquivalentAmount = formatChargeEquivalentAmount;

export function formatCreditAmount(chargeAmount, compact = false) {
  const normalized = Number(chargeAmount || 0);
  if (!Number.isFinite(normalized)) {
    return `${YYC_SYMBOL} 0`;
  }
  const display = compact
    ? formatCompactNumber(normalized)
    : normalized.toLocaleString();
  return `${YYC_SYMBOL} ${display}`;
}

export function formatDecimalNumber(value, maximumFractionDigits = 8) {
  const normalized = Number(value);
  if (!Number.isFinite(normalized)) {
    return '0';
  }
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits,
  }).format(normalized);
}

export function formatAmountWithUnit(amount, unit, maximumFractionDigits = 8) {
  const normalizedUnit = (unit || '').toString().trim().toUpperCase();
  const normalizedAmount = Number(amount);
  if (normalizedUnit === 'YYC') {
    return formatCreditAmount(normalizedAmount, false);
  }
  const display = formatDecimalNumber(normalizedAmount, maximumFractionDigits);
  return normalizedUnit ? `${display} ${normalizedUnit}` : display;
}

export function renderChargeAmount(chargeAmount, t, compact = true, amountPrecision = 6) {
  const normalized = Number(chargeAmount || 0);
  const triggerText = formatCreditAmount(normalized, compact);
  const amount = formatChargeEquivalentAmount(normalized, amountPrecision);
  if (!amount || typeof t !== 'function') {
    return <span>{triggerText}</span>;
  }
  return (
    <AppTooltip
      title={`${formatCreditAmount(normalized, false)} (${t('common.quota.display', { amount })})`}
    >
      <span>{triggerText}</span>
    </AppTooltip>
  );
}

export function chargeAmountToDisplayInputValue(chargeAmount, precision = 6) {
  const normalized = Number(chargeAmount || 0);
  if (!Number.isFinite(normalized) || normalized === 0) {
    return '0';
  }
  if (!isChargeDisplayedInCurrency()) {
    return `${Math.trunc(normalized)}`;
  }
  return (normalized / getChargeRateValue())
    .toFixed(precision)
    .replace(/\.?0+$/, '');
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const quotaToInputValue = chargeAmountToDisplayInputValue;

export function displayInputToChargeAmount(value) {
  const normalized = Number(value ?? 0);
  if (!Number.isFinite(normalized) || normalized < 0) {
    return NaN;
  }
  if (!isChargeDisplayedInCurrency()) {
    return Math.trunc(normalized);
  }
  return Math.round(normalized * getChargeRateValue());
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const quotaInputToStoredValue = displayInputToChargeAmount;

export function displayAmountInputStep() {
  return isChargeDisplayedInCurrency() ? '0.01' : '1';
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const quotaInputStep = displayAmountInputStep;

export function renderAmountEquivalentPrompt(chargeAmount, t) {
  if (getChargeRateValue() > 0) {
    const amount = formatChargeEquivalentAmount(chargeAmount, 2);
    return ` (${t('common.quota.display', { amount })})`;
  }

  return '';
}

// Alias kept for quota-named UI callsites that still display charge amounts.
export const renderChargeAmountEquivalentPrompt = renderAmountEquivalentPrompt;

const colors = [
  'red',
  'orange',
  'yellow',
  'olive',
  'green',
  'teal',
  'blue',
  'violet',
  'purple',
  'pink',
  'brown',
  'grey',
  'black',
];

export function renderColorLabel(text) {
  let hash = 0;
  for (let i = 0; i < text.length; i++) {
    hash = text.charCodeAt(i) + ((hash << 5) - hash);
  }
  let index = Math.abs(hash % colors.length);
  return (
    <AppTag color={colors[index]} className='router-tag'>
      {text}
    </AppTag>
  );
}

export function renderChannelTip(protocol) {
  let channel = getChannelProtocolOption(protocol);
  if (channel === undefined || channel.tip === undefined) {
    return <></>;
  }
  return (
    <AppAlert
      type='info'
      showIcon
      className='router-section-message'
      title={<div dangerouslySetInnerHTML={{ __html: channel.tip }}></div>}
    />
  );
}
