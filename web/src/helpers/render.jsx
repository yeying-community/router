import { Label, Message, Popup } from 'semantic-ui-react';
import { getChannelProtocolOption } from './helper';
import React from 'react';

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
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        flexWrap: 'wrap',
        gap: '2px',
        rowGap: '6px',
      }}
    >
      {groups.map((group) => {
        if (group === 'vip' || group === 'pro') {
          return <Label key={group} color='yellow'>{group}</Label>;
        } else if (group === 'svip' || group === 'premium') {
          return <Label key={group} color='red'>{group}</Label>;
        }
        return <Label key={group}>{group}</Label>;
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

export function renderQuota(quota, t, precision = 2) {
  const displayInCurrency =
    localStorage.getItem('display_in_currency') === 'true';
  const quotaPerUnit = parseFloat(
    localStorage.getItem('quota_per_unit') || '1'
  );

  if (displayInCurrency) {
    const amount = (quota / quotaPerUnit).toFixed(precision);
    return t('common.quota.display_short', { amount });
  }

  return renderNumber(quota);
}

export function isQuotaDisplayedInCurrency() {
  if (typeof window === 'undefined') {
    return false;
  }
  return localStorage.getItem('display_in_currency') === 'true';
}

export function getQuotaPerUnitValue() {
  if (typeof window === 'undefined') {
    return 1;
  }
  const value = parseFloat(localStorage.getItem('quota_per_unit') || '1');
  if (!Number.isFinite(value) || value <= 0) {
    return 1;
  }
  return value;
}

export function formatQuotaEquivalentAmount(quota, precision = 6) {
  const normalized = Number(quota || 0);
  if (!Number.isFinite(normalized)) {
    return '';
  }
  return (normalized / getQuotaPerUnitValue())
    .toFixed(precision)
    .replace(/\.?0+$/, '');
}

export function formatYYCValue(quota, compact = false) {
  const normalized = Number(quota || 0);
  if (!Number.isFinite(normalized)) {
    return '0 YYC';
  }
  const display = compact
    ? formatCompactNumber(normalized)
    : normalized.toLocaleString();
  return `${display} YYC`;
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
    return formatYYCValue(normalizedAmount, false);
  }
  const display = formatDecimalNumber(normalizedAmount, maximumFractionDigits);
  return normalizedUnit ? `${display} ${normalizedUnit}` : display;
}

export function renderYYC(quota, t, compact = true, amountPrecision = 6) {
  const normalized = Number(quota || 0);
  const triggerText = formatYYCValue(normalized, compact);
  const amount = formatQuotaEquivalentAmount(normalized, amountPrecision);
  if (!amount || typeof t !== 'function') {
    return <span>{triggerText}</span>;
  }
  return (
    <Popup
      content={`${formatYYCValue(normalized, false)} (${t('common.quota.display', { amount })})`}
      trigger={<span>{triggerText}</span>}
    />
  );
}

export function quotaToInputValue(quota, precision = 6) {
  const normalized = Number(quota || 0);
  if (!Number.isFinite(normalized) || normalized === 0) {
    return '0';
  }
  if (!isQuotaDisplayedInCurrency()) {
    return `${Math.trunc(normalized)}`;
  }
  return (normalized / getQuotaPerUnitValue())
    .toFixed(precision)
    .replace(/\.?0+$/, '');
}

export function quotaInputToStoredValue(value) {
  const normalized = Number(value ?? 0);
  if (!Number.isFinite(normalized) || normalized < 0) {
    return NaN;
  }
  if (!isQuotaDisplayedInCurrency()) {
    return Math.trunc(normalized);
  }
  return Math.round(normalized * getQuotaPerUnitValue());
}

export function quotaInputStep() {
  return isQuotaDisplayedInCurrency() ? '0.01' : '1';
}

export function renderQuotaWithPrompt(quota, t) {
  const displayInCurrency =
    localStorage.getItem('display_in_currency') === 'true';

  if (displayInCurrency) {
    const amount = formatQuotaEquivalentAmount(quota, 2);
    return ` (${t('common.quota.display', { amount })})`;
  }

  return '';
}

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
    <Label basic color={colors[index]} className='router-tag'>
      {text}
    </Label>
  );
}

export function renderChannelTip(protocol) {
  let channel = getChannelProtocolOption(protocol);
  if (channel === undefined || channel.tip === undefined) {
    return <></>;
  }
  return (
    <Message>
      <div dangerouslySetInnerHTML={{ __html: channel.tip }}></div>
    </Message>
  );
}
