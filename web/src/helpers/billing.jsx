import { API } from './api';
import { renderNumber, YYC_SYMBOL } from './render';

export const YYC_DISPLAY_CODE = 'YYC';
export const DEFAULT_FIAT_DISPLAY_CODE = 'USD';

export const normalizeDisplayCurrencyCode = (value) =>
  (value || '').toString().trim().toUpperCase();

export const getFallbackUSDYYCPerUnit = () => {
  if (typeof window === 'undefined') {
    return 0;
  }
  const raw = Number(window.localStorage.getItem('quota_per_unit') || '');
  if (!Number.isFinite(raw) || raw <= 0) {
    return 0;
  }
  return raw;
};

const PLACEHOLDER_CURRENCY_MAP = {
  [DEFAULT_FIAT_DISPLAY_CODE]: {
    code: DEFAULT_FIAT_DISPLAY_CODE,
    name: 'US Dollar',
    symbol: '$',
    minor_unit: 6,
    yyc_per_unit: 0,
  },
  CNY: {
    code: 'CNY',
    name: 'Chinese Yuan',
    symbol: '¥',
    minor_unit: 6,
    yyc_per_unit: 0,
  },
};

export const buildBillingCurrencyIndex = (
  rows,
  {
    fallbackUsdYYCPerUnit = 0,
    activeOnly = false,
    requirePositiveRate = false,
    placeholderCodes = [],
  } = {},
) => {
  const next = {
    [YYC_DISPLAY_CODE]: {
      code: YYC_DISPLAY_CODE,
      name: 'Yeying Coin',
      symbol: YYC_SYMBOL,
      minor_unit: 0,
      yyc_per_unit: 1,
    },
  };

  (Array.isArray(placeholderCodes) ? placeholderCodes : []).forEach((codeValue) => {
    const code = normalizeDisplayCurrencyCode(codeValue);
    const placeholder = PLACEHOLDER_CURRENCY_MAP[code];
    if (!code || !placeholder) {
      return;
    }
    next[code] = {
      ...placeholder,
    };
  });

  if (Number.isFinite(fallbackUsdYYCPerUnit) && fallbackUsdYYCPerUnit > 0) {
    next[DEFAULT_FIAT_DISPLAY_CODE] = {
      ...PLACEHOLDER_CURRENCY_MAP[DEFAULT_FIAT_DISPLAY_CODE],
      yyc_per_unit: fallbackUsdYYCPerUnit,
    };
  }

  (Array.isArray(rows) ? rows : []).forEach((item) => {
    const code = normalizeDisplayCurrencyCode(item?.code);
    const status = Number(item?.status ?? 1);
    const rawRate = Number(item?.yyc_per_unit ?? 0);
    if (!code) {
      return;
    }
    if (activeOnly && status !== 1) {
      return;
    }
    if (requirePositiveRate && (!Number.isFinite(rawRate) || rawRate <= 0)) {
      return;
    }
    const current = next[code] || {};
    next[code] = {
      ...current,
      ...item,
      code,
      minor_unit: Number(item?.minor_unit ?? current.minor_unit ?? 6),
      yyc_per_unit: Number.isFinite(rawRate)
        ? rawRate
        : Number(current?.yyc_per_unit ?? 0),
    };
  });

  return next;
};

export const buildPublicDisplayCurrencyIndex = (
  rows,
  fallbackUsdYYCPerUnit = getFallbackUSDYYCPerUnit(),
) =>
  buildBillingCurrencyIndex(rows, {
    fallbackUsdYYCPerUnit,
    activeOnly: true,
    requirePositiveRate: true,
  });

export const resolvePreferredDisplayCurrency = (
  currencyIndex,
  preferred = DEFAULT_FIAT_DISPLAY_CODE,
) => {
  const candidates = [
    normalizeDisplayCurrencyCode(preferred),
    DEFAULT_FIAT_DISPLAY_CODE,
    'CNY',
    YYC_DISPLAY_CODE,
  ];
  const availableCodes = Object.keys(currencyIndex || {}).sort((a, b) =>
    a.localeCompare(b),
  );
  candidates.push(...availableCodes);
  for (const code of candidates) {
    if (code && currencyIndex?.[code]) {
      return code;
    }
  }
  return YYC_DISPLAY_CODE;
};

export const listDisplayCurrencies = (currencyIndex) =>
  Object.values(currencyIndex || {})
    .filter((item) => item?.code)
    .sort((a, b) => {
      if (a.code === DEFAULT_FIAT_DISPLAY_CODE) return -1;
      if (b.code === DEFAULT_FIAT_DISPLAY_CODE) return 1;
      if (a.code === YYC_DISPLAY_CODE) return 1;
      if (b.code === YYC_DISPLAY_CODE) return -1;
      return `${a.code}`.localeCompare(`${b.code}`);
    });

const listBillingUnitCurrencies = (currencyIndex) =>
  Object.values(currencyIndex || {})
    .filter((item) => item?.code)
    .sort((a, b) => {
      if (a.code === DEFAULT_FIAT_DISPLAY_CODE) return -1;
      if (b.code === DEFAULT_FIAT_DISPLAY_CODE) return 1;
      if (a.code === YYC_DISPLAY_CODE) return -1;
      if (b.code === YYC_DISPLAY_CODE) return 1;
      return `${a.code}`.localeCompare(`${b.code}`);
    });

const listYYCFirstCurrencies = (currencyIndex) => {
  const items = Object.values(currencyIndex || {}).filter((item) => item?.code);
  const yycItem = items.find(
    (item) => normalizeDisplayCurrencyCode(item.code) === YYC_DISPLAY_CODE,
  );
  const others = items
    .filter((item) => normalizeDisplayCurrencyCode(item.code) !== YYC_DISPLAY_CODE)
    .sort((a, b) => `${a.code}`.localeCompare(`${b.code}`));
  return yycItem ? [yycItem, ...others] : others;
};

const buildCurrencyOptionLabel = (item, { includeCode = false } = {}) => {
  const code = normalizeDisplayCurrencyCode(item?.code);
  if (!code) {
    return '';
  }
  const symbol = (item?.symbol || '').toString().trim();
  if (includeCode) {
    return symbol ? `${symbol} ${code}` : code;
  }
  if (code === YYC_DISPLAY_CODE) {
    return YYC_SYMBOL;
  }
  return symbol || code;
};

export const buildBillingUnitOptions = (currencyIndex) => {
  const seen = new Set();
  return listBillingUnitCurrencies(currencyIndex).reduce((items, item) => {
    const code = normalizeDisplayCurrencyCode(item?.code);
    if (!code || seen.has(code)) {
      return items;
    }
    seen.add(code);
    items.push({
      value: code,
      label: buildCurrencyOptionLabel(item),
    });
    return items;
  }, []);
};

export const buildDisplayUnitOptions = (
  currencyIndex,
  { order = 'display', includeCode = false } = {},
) => {
  const items =
    order === 'yyc-first'
      ? listYYCFirstCurrencies(currencyIndex)
      : listDisplayCurrencies(currencyIndex);
  return items.map((item) => ({
    value: normalizeDisplayCurrencyCode(item?.code),
    label: buildCurrencyOptionLabel(item, { includeCode }),
  }));
};

export const buildFaceValueUnitOptions = (
  rows,
  { currentUnit = '', includeName = true } = {},
) => {
  const options = [
    {
      value: YYC_DISPLAY_CODE,
      label: YYC_DISPLAY_CODE,
    },
  ];
  const seen = new Set([YYC_DISPLAY_CODE]);
  (Array.isArray(rows) ? rows : [])
    .filter((item) => Number(item?.status ?? 0) === 1)
    .forEach((item) => {
      const code = normalizeDisplayCurrencyCode(item?.code);
      if (!code || seen.has(code)) {
        return;
      }
      seen.add(code);
      const name = (item?.name || '').toString().trim();
      options.push({
        value: code,
        label: includeName && name ? `${code} (${name})` : code,
      });
    });

  const normalizedCurrentUnit = normalizeDisplayCurrencyCode(currentUnit);
  if (normalizedCurrentUnit && !seen.has(normalizedCurrentUnit)) {
    options.push({
      value: normalizedCurrentUnit,
      label: normalizedCurrentUnit,
    });
  }
  return options;
};

export const BILLING_OPTION_SETTING_KEYS = [
  'QuotaForNewUser',
  'PreConsumedQuota',
  'QuotaForInviter',
  'QuotaForInvitee',
];

export const createBillingUnitState = (defaultUnit = DEFAULT_FIAT_DISPLAY_CODE) =>
  BILLING_OPTION_SETTING_KEYS.reduce((result, key) => {
    result[key] = defaultUnit;
    return result;
  }, {});

export const resolveDefaultBillingUnit = (currencyIndex) => {
  if (currencyIndex?.[DEFAULT_FIAT_DISPLAY_CODE]) {
    return DEFAULT_FIAT_DISPLAY_CODE;
  }
  if (currencyIndex?.[YYC_DISPLAY_CODE]) {
    return YYC_DISPLAY_CODE;
  }
  return (
    Object.keys(currencyIndex || {})
      .filter((code) => code)
      .sort((a, b) => a.localeCompare(b))[0] || YYC_DISPLAY_CODE
  );
};

export const getCurrencyRateToYYC = (unit, currencyIndex) => {
  const normalizedUnit = normalizeDisplayCurrencyCode(unit);
  if (normalizedUnit === YYC_DISPLAY_CODE) {
    return 1;
  }
  const rate = Number(currencyIndex?.[normalizedUnit]?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return 0;
  }
  return rate;
};

export const formatBillingInputAmount = (amount, unit, currencyIndex) => {
  const normalizedAmount = Number(amount || 0);
  if (!Number.isFinite(normalizedAmount) || normalizedAmount === 0) {
    return '0';
  }
  const normalizedUnit = normalizeDisplayCurrencyCode(unit);
  if (normalizedUnit === YYC_DISPLAY_CODE) {
    return `${Math.round(normalizedAmount)}`;
  }
  const minorUnit = Number(currencyIndex?.[normalizedUnit]?.minor_unit);
  const fractionDigits =
    Number.isInteger(minorUnit) && minorUnit >= 0 ? Math.min(minorUnit, 8) : 6;
  return normalizedAmount.toFixed(fractionDigits).replace(/\.?0+$/, '');
};

export const yycToBillingInputValue = (yycValue, unit, currencyIndex) => {
  const storedYYC = Number(yycValue || 0);
  if (!Number.isFinite(storedYYC) || storedYYC <= 0) {
    return '0';
  }
  const rate = getCurrencyRateToYYC(unit, currencyIndex);
  if (rate <= 0) {
    return '0';
  }
  return formatBillingInputAmount(storedYYC / rate, unit, currencyIndex);
};

export const billingInputValueToYYC = (value, unit, currencyIndex) => {
  const normalizedAmount = Number(value ?? 0);
  if (!Number.isFinite(normalizedAmount) || normalizedAmount < 0) {
    return NaN;
  }
  const rate = getCurrencyRateToYYC(unit, currencyIndex);
  if (rate <= 0) {
    return NaN;
  }
  if (normalizeDisplayCurrencyCode(unit) === YYC_DISPLAY_CODE) {
    return Math.round(normalizedAmount);
  }
  return Math.round(normalizedAmount * rate);
};

export const convertBillingInputValueUnit = (value, fromUnit, toUnit, currencyIndex) => {
  const normalizedAmount = Number(value ?? 0);
  if (!Number.isFinite(normalizedAmount) || normalizedAmount <= 0) {
    return '0';
  }
  const storedYYC = billingInputValueToYYC(
    normalizedAmount,
    fromUnit,
    currencyIndex,
  );
  if (!Number.isFinite(storedYYC) || storedYYC < 0) {
    return '0';
  }
  return yycToBillingInputValue(storedYYC, toUnit, currencyIndex);
};

export const resolveBillingInputStep = (unit, currencyIndex) => {
  const normalizedUnit = normalizeDisplayCurrencyCode(unit);
  if (normalizedUnit === YYC_DISPLAY_CODE) {
    return '1';
  }
  const minorUnit = Number(currencyIndex?.[normalizedUnit]?.minor_unit);
  if (!Number.isInteger(minorUnit) || minorUnit <= 0) {
    return '0.01';
  }
  return (1 / 10 ** Math.min(minorUnit, 8)).toFixed(Math.min(minorUnit, 8));
};

export const applyBillingInputValues = (
  rawInputs,
  billingUnits,
  currencyIndex,
  settingKeys = BILLING_OPTION_SETTING_KEYS,
) => {
  const next = {
    ...(rawInputs || {}),
  };
  (Array.isArray(settingKeys) ? settingKeys : []).forEach((key) => {
    next[key] = yycToBillingInputValue(
      rawInputs?.[key] ?? 0,
      billingUnits?.[key],
      currencyIndex,
    );
  });
  return next;
};

export const convertYYCToDisplayAmount = (
  yycAmount,
  displayUnit,
  currencyIndex,
) => {
  const normalizedAmount = Number(yycAmount || 0);
  if (!Number.isFinite(normalizedAmount)) {
    return NaN;
  }
  const normalizedUnit = normalizeDisplayCurrencyCode(displayUnit);
  if (normalizedUnit === YYC_DISPLAY_CODE) {
    return normalizedAmount;
  }
  const rate = Number(currencyIndex?.[normalizedUnit]?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return NaN;
  }
  return normalizedAmount / rate;
};

export const formatCompactDisplayAmount = (
  amount,
  {
    fractionDigits = 4,
    compactThreshold = 10000,
    compactDivisor = 10000,
    compactFractionDigits = 2,
    compactLabel = '',
  } = {},
) => {
  const normalizedAmount = Number(amount);
  if (!Number.isFinite(normalizedAmount)) {
    return '0.0000';
  }
  const abs = Math.abs(normalizedAmount);
  if (compactLabel && abs >= compactThreshold) {
    const display = (normalizedAmount / compactDivisor).toFixed(
      compactFractionDigits,
    );
    return `${display}${compactLabel}`;
  }
  return normalizedAmount.toFixed(fractionDigits);
};

export const formatDisplayAmountFromYYC = (
  yycAmount,
  displayUnit,
  currencyIndex,
  {
    fractionDigits = 6,
    includeSymbol = false,
    yycMode = 'fixed',
    invalidValue = '-',
  } = {},
) => {
  const yycValue = Number(yycAmount || 0);
  if (!Number.isFinite(yycValue)) {
    return invalidValue;
  }

  const normalizedUnit = normalizeDisplayCurrencyCode(displayUnit);
  if (normalizedUnit === YYC_DISPLAY_CODE) {
    if (yycMode === 'compact') {
      return renderNumber(yycValue);
    }
    return Number(yycValue).toFixed(fractionDigits);
  }

  const amount = convertYYCToDisplayAmount(
    yycValue,
    normalizedUnit,
    currencyIndex,
  );
  if (!Number.isFinite(amount)) {
    return invalidValue;
  }
  const text = Number(amount).toFixed(fractionDigits);
  if (!includeSymbol) {
    return text;
  }
  const symbol = (currencyIndex?.[normalizedUnit]?.symbol || '').toString().trim();
  if (symbol) {
    return `${symbol}${text}`;
  }
  return text;
};

export const loadPublicDisplayCurrencyCatalog = async () => {
  const fallbackUsdYYCPerUnit = getFallbackUSDYYCPerUnit();
  try {
    const res = await API.get('/api/v1/public/billing/currencies');
    const { success, data } = res.data || {};
    if (!success) {
      throw new Error('load public billing currencies failed');
    }
    const index = buildPublicDisplayCurrencyIndex(
      Array.isArray(data?.items) ? data.items : data,
      fallbackUsdYYCPerUnit,
    );
    return {
      currencyIndex: index,
      defaultCurrency: resolvePreferredDisplayCurrency(
        index,
        data?.default_currency || DEFAULT_FIAT_DISPLAY_CODE,
      ),
    };
  } catch {
    const index = buildPublicDisplayCurrencyIndex([], fallbackUsdYYCPerUnit);
    return {
      currencyIndex: index,
      defaultCurrency: resolvePreferredDisplayCurrency(index),
    };
  }
};
