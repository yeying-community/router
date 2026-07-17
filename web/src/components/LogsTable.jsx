import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
  writePagedRows,
} from '../helpers';
import { useTranslation } from 'react-i18next';
import UnitDropdown from './UnitDropdown';

import { ITEMS_PER_PAGE } from '../constants';
import {
  renderColorLabel,
  isChargeDisplayedInCurrency,
  YYC_SYMBOL,
} from '../helpers/render';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  buildPublicDisplayCurrencyIndex,
  buildDisplayUnitOptions,
  formatDisplayAmountFromChargeAmount,
  loadPublicDisplayCurrencyCatalog,
  resolvePreferredDisplayCurrency,
  YYC_DISPLAY_CODE,
} from '../helpers/billing';
import {
  LOG_LIST_COLUMN_WIDTHS,
  LOG_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import {
  AppButton,
  AppFilterHeader,
  AppFormActions,
  AppPagination,
  AppPopover,
  resolvePopupContainer,
  AppSelect,
  AppTable,
  AppTag,
  AppToolbar,
} from '../router-ui';

const USER_LOG_COLUMN_ORDER_STORAGE_KEY = 'router_user_log_column_order_v1';
const DEFAULT_USER_LOG_COLUMN_ORDER = [
  'created_at',
  'billingSource',
  'model_name',
  'token_name',
  'prompt_tokens',
  'completion_tokens',
  'cacheQuantity',
  'chargeAmount',
];

function normalizeUserLogColumnOrder(rawOrder) {
  const nextOrder = [];
  const seen = new Set();
  const append = (key) => {
    const normalizedKey = key === 'type' ? 'billingSource' : key;
    if (
      !DEFAULT_USER_LOG_COLUMN_ORDER.includes(normalizedKey) ||
      seen.has(normalizedKey)
    ) {
      return;
    }
    seen.add(normalizedKey);
    nextOrder.push(normalizedKey);
  };
  if (Array.isArray(rawOrder)) {
    rawOrder.forEach((key) => append(String(key || '').trim()));
  }
  DEFAULT_USER_LOG_COLUMN_ORDER.forEach(append);
  return nextOrder;
}

function loadUserLogColumnOrder() {
  if (typeof window === 'undefined') {
    return [...DEFAULT_USER_LOG_COLUMN_ORDER];
  }
  try {
    const stored = JSON.parse(
      window.localStorage.getItem(USER_LOG_COLUMN_ORDER_STORAGE_KEY) || '[]',
    );
    return normalizeUserLogColumnOrder(stored);
  } catch (error) {
    return [...DEFAULT_USER_LOG_COLUMN_ORDER];
  }
}

const compareTextValue = (left, right) =>
  String(left || '').localeCompare(String(right || ''));

const compareNumberValue = (left, right) =>
  Number(left || 0) - Number(right || 0);

function formatCompactNumber(value) {
  const numericValue = Number(value || 0);
  if (!Number.isFinite(numericValue) || numericValue === 0) {
    return '';
  }
  if (Number.isInteger(numericValue)) {
    return numericValue.toLocaleString();
  }
  return numericValue.toLocaleString(undefined, {
    maximumFractionDigits: 6,
  });
}

function renderTimestamp(timestamp) {
  return <code>{timestamp2string(timestamp)}</code>;
}

function renderType(type) {
  switch (type) {
    case 1:
      return (
        <AppTag color='green' className='router-tag'>
          充值
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='olive' className='router-tag'>
          消费
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='orange' className='router-tag'>
          管理
        </AppTag>
      );
    case 4:
      return (
        <AppTag color='purple' className='router-tag'>
          系统
        </AppTag>
      );
    case 5:
      return (
        <AppTag color='violet' className='router-tag'>
          测试
        </AppTag>
      );
    case 6:
      return (
        <AppTag color='red' className='router-tag'>
          失败
        </AppTag>
      );
    default:
      return (
        <AppTag color='black' className='router-tag'>
          未知
        </AppTag>
      );
  }
}

function renderBillingSource(log, t) {
  const name = String(log?.billing_source_name || '').trim();
  const source = String(log?.billing_source || '').trim();
  const fallback =
    source === 'package'
      ? t('log.detail.billing_sources.package')
      : source === 'balance'
        ? t('log.detail.billing_sources.balance')
        : '';
  const label = name || fallback || '-';
  const color = source === 'package' ? 'blue' : source === 'balance' ? 'teal' : 'grey';
  const sourceID = String(log?.billing_source_id || '').trim();
  const sourceDetail = String(log?.billing_source_detail || '').trim();
  const title = [sourceDetail, sourceID].filter(Boolean).join(' / ');

  if (label === '-') {
    return '-';
  }
  return (
    <AppTag className='router-tag' color={color} title={title || label}>
      {label}
    </AppTag>
  );
}

function getColorByElapsedTime(elapsedTime) {
  if (elapsedTime === undefined || 0) return 'black';
  if (elapsedTime < 1000) return 'green';
  if (elapsedTime < 3000) return 'olive';
  if (elapsedTime < 5000) return 'yellow';
  if (elapsedTime < 10000) return 'orange';
  return 'red';
}

function renderDetail(log) {
  return (
    <>
      {log.content}
      <br />
      {log.elapsed_time && (
        <AppTag className='router-tag' color={getColorByElapsedTime(log.elapsed_time)}>
          {log.elapsed_time} ms
        </AppTag>
      )}
      {log.is_stream && (
        <AppTag className='router-tag' color='pink'>
          Stream
        </AppTag>
      )}
    </>
  );
}

function getLogChannelLabel(log) {
  if (!log) {
    return '';
  }
  return log.channel_name || log.channel || '';
}

function getLogPublicModelName(log) {
  return (log?.request_model_name || '').toString().trim();
}

function getLogActualModelName(log) {
  return (log?.actual_model_name || '').toString().trim();
}

function normalizeLogEntry(log) {
  const cacheReadQuantity = Number(log?.billing_cache_read_quantity ?? 0);
  const cacheWriteQuantity = Number(log?.billing_cache_write_quantity ?? 0);
  return {
    ...(log || {}),
    publicModelName: getLogPublicModelName(log),
    actualModelName: getLogActualModelName(log),
    // Prefer charge-amount settlement fields, fall back to legacy quota-based logs.
    chargeAmount: Number(log?.charge_amount ?? log?.quota ?? 0),
    userDailyChargeAmount: Number(log?.user_daily_charge_amount ?? log?.user_daily_quota ?? 0),
    userEmergencyChargeAmount: Number(log?.user_emergency_charge_amount ?? log?.user_emergency_quota ?? 0),
    cacheReadQuantity,
    cacheWriteQuantity,
    cacheQuantity: cacheReadQuantity + cacheWriteQuantity,
  };
}

function toDatetimeLocalValue(value) {
  const raw = (value || '').toString().trim();
  if (raw === '') {
  return '';
}

function toUserFilterOption(item) {
  const username = (item?.username || '').toString().trim();
  const displayName = (item?.display_name || '').toString().trim();
  const walletAddress = (item?.wallet_address || '').toString().trim();
  if (!username) {
    return null;
  }
  const label = [displayName || username, walletAddress].filter(Boolean).join(' / ');
  return {
    key: username,
    text: label || username,
    value: username,
  };
}

function toTokenFilterOption(item) {
  const tokenName = (item?.name || '').toString().trim();
  const tokenID = (item?.id || '').toString().trim();
  if (!tokenName) {
    return null;
  }
  return {
    key: tokenName,
    text: [tokenName, tokenID].filter(Boolean).join(' / '),
    value: tokenName,
  };
}
  if (raw.includes('T')) {
    return raw.slice(0, 16);
  }
  if (raw.includes(' ')) {
    return raw.replace(' ', 'T').slice(0, 16);
  }
  const parsed = Date.parse(raw);
  if (!Number.isFinite(parsed)) {
    return '';
  }
  const date = new Date(parsed);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  const hour = `${date.getHours()}`.padStart(2, '0');
  const minute = `${date.getMinutes()}`.padStart(2, '0');
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

function parseDatetimeInput(value) {
  const raw = (value || '').toString().trim();
  if (raw === '') {
    return 0;
  }
  const parsed = Date.parse(raw);
  if (!Number.isFinite(parsed)) {
    return 0;
  }
  return Math.floor(parsed / 1000);
}

function cleanupDatetimeLocalValue() {
  const date = new Date();
  date.setMonth(date.getMonth() - 1);
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  const hour = `${date.getHours()}`.padStart(2, '0');
  const minute = `${date.getMinutes()}`.padStart(2, '0');
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

function formatFilterDisplayValue(value) {
  return (value || '').toString().trim().replace('T', ' ');
}

function normalizeSearchDateTimeValue(value) {
  const raw = (value || '').toString().trim();
  if (raw === '') {
    return '';
  }
  if (/^\d+$/.test(raw)) {
    const timestamp = Number(raw);
    if (!Number.isFinite(timestamp)) {
      return '';
    }
    const normalizedTimestamp = raw.length > 10 ? timestamp : timestamp * 1000;
    const date = new Date(normalizedTimestamp);
    if (Number.isNaN(date.getTime())) {
      return '';
    }
    const year = date.getFullYear();
    const month = `${date.getMonth() + 1}`.padStart(2, '0');
    const day = `${date.getDate()}`.padStart(2, '0');
    const hour = `${date.getHours()}`.padStart(2, '0');
    const minute = `${date.getMinutes()}`.padStart(2, '0');
    return `${year}-${month}-${day}T${hour}:${minute}`;
  }
  return toDatetimeLocalValue(raw);
}

function parseLogFiltersFromSearch(search, isAdminScope) {
  const params = new URLSearchParams(search || '');
  const nextInputs = {
    username: '',
    token_name: '',
    model_name: '',
    start_timestamp: '',
    end_timestamp: '',
    channel: '',
    group_id: '',
  };
  const nextActiveFilterKeys = [];
  const nextLogType = Number(params.get('log_type') || params.get('type') || 0);
  if (Number.isFinite(nextLogType) && nextLogType > 0) {
    nextActiveFilterKeys.push('log_type');
  }
  const nextStart = normalizeSearchDateTimeValue(params.get('start_timestamp'));
  const nextEnd = normalizeSearchDateTimeValue(params.get('end_timestamp'));
  if (nextStart !== '' || nextEnd !== '') {
    nextInputs.start_timestamp = nextStart;
    nextInputs.end_timestamp = nextEnd;
    nextActiveFilterKeys.push('time_range');
  }
  const filterKeys = ['token_name', 'model_name'];
  if (isAdminScope) {
    filterKeys.push('channel', 'group_id', 'username');
  }
  filterKeys.forEach((key) => {
    const value = (params.get(key) || '').toString().trim();
    if (value === '') {
      return;
    }
    nextInputs[key] = value;
    nextActiveFilterKeys.push(key);
  });
  return {
    inputs: nextInputs,
    logType: Number.isFinite(nextLogType) && nextLogType > 0 ? nextLogType : 0,
    activeFilterKeys: Array.from(new Set(nextActiveFilterKeys)),
  };
}

function currentDatetimeLocalValue() {
  const now = new Date();
  const year = now.getFullYear();
  const month = `${now.getMonth() + 1}`.padStart(2, '0');
  const day = `${now.getDate()}`.padStart(2, '0');
  const hour = `${now.getHours()}`.padStart(2, '0');
  const minute = `${now.getMinutes()}`.padStart(2, '0');
  return `${year}-${month}-${day}T${hour}:${minute}`;
}

function startOfTodayDatetimeLocalValue() {
  const now = new Date();
  now.setHours(0, 0, 0, 0);
  const year = now.getFullYear();
  const month = `${now.getMonth() + 1}`.padStart(2, '0');
  const day = `${now.getDate()}`.padStart(2, '0');
  return `${year}-${month}-${day}T00:00`;
}

function renderFilterSummary(filterKey, inputs, t, extra = {}) {
  if (filterKey === 'time_range') {
    const start = formatFilterDisplayValue(inputs?.start_timestamp);
    const end = formatFilterDisplayValue(inputs?.end_timestamp);
    if (start === '' && end === '') {
      return t('log.filters.empty');
    }
    if (start !== '' && end !== '') {
      return `${start} ${t('log.filters.range_separator')} ${end}`;
    }
    return start || end || t('log.filters.empty');
  }
  if (filterKey === 'log_type') {
    return extra.logTypeLabel || t('log.filters.empty');
  }
  const value = (inputs?.[filterKey] || '').toString().trim();
  if (value === '') {
    return t('log.filters.empty');
  }
  if (typeof extra.resolveOptionLabel === 'function') {
    return extra.resolveOptionLabel(filterKey, value) || value;
  }
  return value;
}

const LogsTable = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const isAdminScope = location.pathname.startsWith('/admin/');
  const logSource = useMemo(() => {
    const params = new URLSearchParams(location.search || '');
    return (params.get('source') || '').trim();
  }, [location.search]);
  const breadcrumbs = useMemo(() => {
    const items = [
      {
        key: 'workspace',
        label: isAdminScope
          ? t('header.admin_workspace')
          : t('header.user_workspace'),
      },
      {
        key: 'section',
        label: isAdminScope ? t('header.platform_operation') : t('header.mine'),
      },
    ];
    if (!isAdminScope && logSource === 'quota') {
      items.push({
        key: 'quota',
        label: t('topup.mine.quota'),
        onClick: () => navigate('/workspace/topup?tab=quota'),
      });
    }
    items.push({ key: 'log', label: t('header.log'), active: true });
    return items;
  }, [isAdminScope, logSource, navigate, t]);
  const initialSearchFilters = useMemo(
    () => parseLogFiltersFromSearch(location.search, isAdminScope),
    [isAdminScope, location.search]
  );
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [tableSorter, setTableSorter] = useState({
    columnKey: 'created_at',
    order: 'descend',
  });
  const [searchKeyword, setSearchKeyword] = useState('');
  const [logType, setLogType] = useState(initialSearchFilters.logType);
  const [filterOptions, setFilterOptions] = useState({
    tokenNames: [],
    modelNames: [],
    usernames: [],
    channels: [],
    groups: [],
  });
  const [loadedFilterKeys, setLoadedFilterKeys] = useState([]);
  const [loadingFilterKeys, setLoadingFilterKeys] = useState([]);
  const [userFilterSearchLoading, setUserFilterSearchLoading] = useState(false);
  const [tokenFilterSearchLoading, setTokenFilterSearchLoading] = useState(false);
  const [modelFilterSearchLoading, setModelFilterSearchLoading] = useState(false);
  const [inputs, setInputs] = useState(initialSearchFilters.inputs);
  const {
    username,
    token_name,
    model_name,
    start_timestamp,
    end_timestamp,
    channel,
    group_id,
  } = inputs;
  const [activeFilterKeys, setActiveFilterKeys] = useState(
    initialSearchFilters.activeFilterKeys
  );
  const [addFilterPopupOpen, setAddFilterPopupOpen] = useState(false);
  const [draftFilterKey, setDraftFilterKey] = useState('');
  const [draftFilterInputs, setDraftFilterInputs] = useState({
    value: '',
    start_timestamp: '',
    end_timestamp: '',
  });
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([])
  );
  const [userLogColumnOrder, setUserLogColumnOrder] = useState(
    loadUserLogColumnOrder,
  );
  const [draggingColumnKey, setDraggingColumnKey] = useState('');
  const [dragOverColumnKey, setDragOverColumnKey] = useState('');
  const [cleanupTimestamp, setCleanupTimestamp] = useState(
    cleanupDatetimeLocalValue(),
  );
  const [cleaningLogs, setCleaningLogs] = useState(false);
  const draggingColumnKeyRef = useRef('');

  const LOG_OPTIONS = [
    { key: '0', text: t('log.type.all'), value: 0 },
    { key: '1', text: t('log.type.topup'), value: 1 },
    { key: '2', text: t('log.type.usage'), value: 2 },
    { key: '3', text: t('log.type.admin'), value: 3 },
    { key: '4', text: t('log.type.system'), value: 4 },
    { key: '5', text: t('log.type.test'), value: 5 },
    { key: '6', text: t('log.type.relay_failure'), value: 6 },
  ];

  const conditionalFilterConfig = useMemo(() => {
    const items = [
      {
        key: 'log_type',
        label: t('log.table.type'),
        type: 'select',
        options: LOG_OPTIONS.filter((item) => Number(item.value) !== 0),
      },
      {
        key: 'time_range',
        label: t('log.table.time_range'),
        type: 'time_range',
      },
      {
        key: 'token_name',
        label: t('log.table.token_name'),
        placeholder: t('log.table.token_name_placeholder'),
        type: isAdminScope ? 'select' : 'text',
        options: filterOptions.tokenNames,
      },
      {
        key: 'model_name',
        label: t('log.table.model_name'),
        placeholder: t('log.table.model_name_placeholder'),
        type: isAdminScope ? 'select' : 'text',
        options: filterOptions.modelNames,
      },
    ];
    if (isAdminScope) {
      items.push(
        {
          key: 'channel',
          label: t('log.table.channel'),
          placeholder: t('log.table.channel_id_placeholder'),
          type: filterOptions.channels.length > 0 ? 'select' : 'text',
          options: filterOptions.channels.map((item) => ({
            key: item.id,
            text: item.label,
            value: item.id,
          })),
        },
        {
          key: 'group_id',
          label: t('log.table.group'),
          placeholder: t('log.table.group_id_placeholder'),
          type: filterOptions.groups.length > 0 ? 'select' : 'text',
          options: filterOptions.groups.map((item) => ({
            key: item.id,
            text: item.label,
            value: item.id,
          })),
        },
        {
          key: 'username',
          label: t('log.table.username'),
          placeholder: t('log.table.username_placeholder'),
          type: 'select',
          options: filterOptions.usernames,
        }
      );
    }
    return items;
  }, [LOG_OPTIONS, filterOptions.channels, filterOptions.groups, filterOptions.modelNames, filterOptions.tokenNames, filterOptions.usernames, isAdminScope, t]);

  const conditionalFilterOptions = useMemo(
    () =>
      conditionalFilterConfig.map((item) => ({
        key: item.key,
        text: item.label,
        value: item.key,
      })),
    [conditionalFilterConfig]
  );

  const visibleFilterConfig = useMemo(
    () =>
      conditionalFilterConfig.filter((item) =>
        activeFilterKeys.includes(item.key)
      ),
    [activeFilterKeys, conditionalFilterConfig]
  );

  const availableConditionalFilterOptions = useMemo(
    () =>
      conditionalFilterOptions.filter(
        (item) => !activeFilterKeys.includes(item.value)
      ),
    [activeFilterKeys, conditionalFilterOptions]
  );

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex),
    [currencyIndex]
  );

  useEffect(() => {
    setInputs(initialSearchFilters.inputs);
    setLogType(initialSearchFilters.logType);
    setActiveFilterKeys(initialSearchFilters.activeFilterKeys);
    setActivePage(1);
  }, [initialSearchFilters]);

  const loadFilterOptions = useCallback(async (filterKey = '') => {
    const normalizedFilterKey = String(filterKey || '').trim();
    if (
      isAdminScope &&
      normalizedFilterKey !== 'channel' &&
      normalizedFilterKey !== 'group_id'
    ) {
      return;
    }
    if (
      normalizedFilterKey &&
      (loadedFilterKeys.includes(normalizedFilterKey) ||
        loadingFilterKeys.includes(normalizedFilterKey))
    ) {
      return;
    }
    if (normalizedFilterKey) {
      setLoadingFilterKeys((prev) =>
        prev.includes(normalizedFilterKey) ? prev : [...prev, normalizedFilterKey]
      );
    }
    try {
      if (isAdminScope && normalizedFilterKey === 'channel') {
        const res = await API.get('/api/v1/admin/channels/', {
          params: {
            page: 1,
            page_size: 100,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('log.messages.load_failed'));
          return;
        }
        setFilterOptions((prev) => ({
          ...prev,
          channels: Array.isArray(data?.items)
            ? data.items.map((item) => ({
                id: item.id,
                label: item.name || item.id,
              }))
            : [],
        }));
      } else if (isAdminScope && normalizedFilterKey === 'group_id') {
        const res = await API.get('/api/v1/admin/groups', {
          params: {
            page: 1,
            page_size: 100,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('log.messages.load_failed'));
          return;
        }
        setFilterOptions((prev) => ({
          ...prev,
          groups: Array.isArray(data?.items)
            ? data.items.map((item) => ({
                id: item.id,
                label: item.name || item.id,
              }))
            : [],
        }));
      } else {
        const suffix = normalizedFilterKey
          ? `?field=${encodeURIComponent(normalizedFilterKey)}`
          : '';
        const url = `/api/v1/public/log/options${suffix}`;
        const res = await API.get(url);
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('log.messages.load_failed'));
          return;
        }
        setFilterOptions((prev) => ({
          ...prev,
          tokenNames:
            !normalizedFilterKey || normalizedFilterKey === 'token_name'
              ? Array.isArray(data?.token_names)
                ? data.token_names
                : []
              : prev.tokenNames,
          modelNames:
            !normalizedFilterKey || normalizedFilterKey === 'model_name'
              ? Array.isArray(data?.model_names)
                ? data.model_names
                : []
              : prev.modelNames,
        }));
      }
      if (normalizedFilterKey) {
        setLoadedFilterKeys((prev) =>
          prev.includes(normalizedFilterKey) ? prev : [...prev, normalizedFilterKey]
        );
      }
    } finally {
      if (normalizedFilterKey) {
        setLoadingFilterKeys((prev) =>
          prev.filter((item) => item !== normalizedFilterKey)
        );
      }
    }
  }, [isAdminScope, loadedFilterKeys, loadingFilterKeys, t]);

  const openFilterDraft = useCallback(
    (filterKey) => {
      const config = conditionalFilterConfig.find((item) => item.key === filterKey);
      if (!config) {
        return;
      }
      if (config.type === 'time_range') {
        setDraftFilterInputs({
          value: '',
          start_timestamp:
            toDatetimeLocalValue(inputs.start_timestamp) ||
            startOfTodayDatetimeLocalValue(),
          end_timestamp:
            toDatetimeLocalValue(inputs.end_timestamp) ||
            currentDatetimeLocalValue(),
        });
      } else if (filterKey === 'log_type') {
        setDraftFilterInputs({
          value: logType > 0 ? logType : '',
          start_timestamp: '',
          end_timestamp: '',
        });
      } else {
        setDraftFilterInputs({
          value: (inputs[filterKey] || '').toString(),
          start_timestamp: '',
          end_timestamp: '',
        });
      }
      if (
        ['channel', 'group_id'].includes(
          filterKey
        )
      ) {
        loadFilterOptions(filterKey).then();
      }
      setDraftFilterKey(filterKey);
      setAddFilterPopupOpen(true);
    },
    [conditionalFilterConfig, inputs, loadFilterOptions, logType]
  );

  const closeFilterDraft = useCallback(() => {
    setAddFilterPopupOpen(false);
    setDraftFilterKey('');
    setDraftFilterInputs({
      value: '',
      start_timestamp: '',
      end_timestamp: '',
    });
  }, []);

  const applyFilterDraft = useCallback(() => {
    if (draftFilterKey === '') {
      return;
    }
    const config = conditionalFilterConfig.find((item) => item.key === draftFilterKey);
    if (!config) {
      return;
    }
    if (config.type === 'time_range') {
      const nextStart = draftFilterInputs.start_timestamp.trim();
      const nextEnd = draftFilterInputs.end_timestamp.trim();
      if (nextStart === '' && nextEnd === '') {
        showError(t('log.filters.empty'));
        return;
      }
      setInputs((prev) => ({
        ...prev,
        start_timestamp: nextStart,
        end_timestamp: nextEnd,
      }));
    } else if (draftFilterKey === 'log_type') {
      const nextValue = Number(draftFilterInputs.value || 0);
      if (!Number.isFinite(nextValue) || nextValue <= 0) {
        showError(t('log.filters.empty'));
        return;
      }
      setLogType(nextValue);
    } else {
      const nextValue = draftFilterInputs.value.trim();
      if (nextValue === '') {
        showError(t('log.filters.empty'));
        return;
      }
      setInputs((prev) => ({
        ...prev,
        [draftFilterKey]: nextValue,
      }));
    }
    setActiveFilterKeys((prev) =>
      prev.includes(draftFilterKey) ? prev : [...prev, draftFilterKey]
    );
    closeFilterDraft();
  }, [closeFilterDraft, conditionalFilterConfig, draftFilterInputs, draftFilterKey, t]);

  const searchAdminUsers = useCallback(async (keyword) => {
    const normalizedKeyword = String(keyword || '').trim();
    setUserFilterSearchLoading(true);
    try {
      const res = await API.get('/api/v1/admin/user/search', {
        params: normalizedKeyword ? { keyword: normalizedKeyword } : undefined,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.messages.load_failed'));
        return;
      }
      setFilterOptions((prev) => ({
        ...prev,
        usernames: (Array.isArray(data) ? data : [])
          .map(toUserFilterOption)
          .filter(Boolean),
      }));
    } finally {
      setUserFilterSearchLoading(false);
    }
  }, [t]);

  const searchAdminTokens = useCallback(async (keyword) => {
    const normalizedKeyword = String(keyword || '').trim();
    setTokenFilterSearchLoading(true);
    try {
      const res = await API.get('/api/v1/admin/token/search', {
        params: normalizedKeyword ? { keyword: normalizedKeyword } : undefined,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.messages.load_failed'));
        return;
      }
      setFilterOptions((prev) => ({
        ...prev,
        tokenNames: (Array.isArray(data) ? data : [])
          .map(toTokenFilterOption)
          .filter(Boolean),
      }));
    } finally {
      setTokenFilterSearchLoading(false);
    }
  }, [t]);

  const searchAdminModels = useCallback(async (keyword) => {
    const normalizedKeyword = String(keyword || '').trim();
    setModelFilterSearchLoading(true);
    try {
      const res = await API.get('/api/v1/admin/log/options', {
        params: {
          field: 'model_name',
          keyword: normalizedKeyword,
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.messages.load_failed'));
        return;
      }
      setFilterOptions((prev) => ({
        ...prev,
        modelNames: (Array.isArray(data?.model_names) ? data.model_names : []).map(
          (item) => ({
            key: item,
            text: item,
            value: item,
          })
        ),
      }));
    } finally {
      setModelFilterSearchLoading(false);
    }
  }, [t]);

  const removeConditionalFilter = useCallback((filterKey) => {
    setActiveFilterKeys((prev) => prev.filter((item) => item !== filterKey));
    if (filterKey === 'log_type') {
      setLogType(0);
      return;
    }
    if (filterKey === 'time_range') {
      setInputs((prev) => ({
        ...prev,
        start_timestamp: '',
        end_timestamp: '',
      }));
      return;
    }
    setInputs((prev) => ({
      ...prev,
      [filterKey]: '',
    }));
  }, []);

  const loadDisplayUnits = useCallback(async () => {
    try {
      if (!isAdminScope) {
        const { currencyIndex: nextIndex } = await loadPublicDisplayCurrencyCatalog();
        const preferredUnit = isChargeDisplayedInCurrency() ? 'USD' : YYC_DISPLAY_CODE;
        setCurrencyIndex(nextIndex);
        setDisplayUnit((current) =>
          resolvePreferredDisplayCurrency(
            nextIndex,
            preferredUnit || current || YYC_DISPLAY_CODE
          )
        );
        return;
      }
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.messages.load_failed'));
        return;
      }
      const next = buildPublicDisplayCurrencyIndex(Array.isArray(data) ? data : []);
      setCurrencyIndex(next);
      setDisplayUnit((current) => {
        return resolvePreferredDisplayCurrency(next, current || 'USD');
      });
    } catch (error) {
      showError(error?.message || error);
    }
  }, [isAdminScope, t]);

  const showAmountColumns = () => {
    const effectiveLogType = activeFilterKeys.includes('log_type') ? logType : 0;
    return effectiveLogType !== 5 && effectiveLogType !== 6;
  };

  const loadLogs = useCallback(
    async (page) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      let url = '';
      const enabledFilters = new Set(activeFilterKeys);
      const localStartTimestamp = enabledFilters.has('time_range')
        ? parseDatetimeInput(start_timestamp)
        : 0;
      const localEndTimestamp = enabledFilters.has('time_range')
        ? parseDatetimeInput(end_timestamp)
        : 0;
      const queryUsername = enabledFilters.has('username') ? username : '';
      const queryTokenName = enabledFilters.has('token_name') ? token_name : '';
      const queryModelName = enabledFilters.has('model_name') ? model_name : '';
      const queryChannel = enabledFilters.has('channel') ? channel : '';
      const queryGroupID = enabledFilters.has('group_id') ? group_id : '';
      const queryLogType = enabledFilters.has('log_type') ? logType : 0;
      if (isAdminScope) {
        url = `/api/v1/admin/log/?page=${normalizedPage}&type=${queryLogType}&username=${queryUsername}&token_name=${queryTokenName}&model_name=${queryModelName}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&group_id=${queryGroupID}&channel=${queryChannel}`;
      } else {
        url = `/api/v1/public/log?page=${normalizedPage}&type=${queryLogType}&token_name=${queryTokenName}&model_name=${queryModelName}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
      }
      const res = await API.get(url);
      const { success, message, data, meta } = res.data;
      if (success) {
        const normalizedRows = Array.isArray(data) ? data.map(normalizeLogEntry) : [];
        setTotalCount(Number(meta?.total || data?.length || 0));
        if (normalizedPage === 1) {
          setLogs(normalizedRows);
        } else {
          setLogs((prev) => writePagedRows(prev, normalizedPage, ITEMS_PER_PAGE, normalizedRows));
        }
      } else {
        showError(message);
      }
      setLoading(false);
    },
    [
      isAdminScope,
      logType,
      username,
      token_name,
      model_name,
      start_timestamp,
      end_timestamp,
      channel,
      group_id,
      activeFilterKeys,
    ]
  );

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      const nextPage = Number(activePage) > 0 ? Number(activePage) : 1;
      const hasLoadedPageRows = logs
        .slice((nextPage - 1) * ITEMS_PER_PAGE, nextPage * ITEMS_PER_PAGE)
        .some(Boolean);
      if (searchKeyword.trim() === '' && !hasLoadedPageRows) {
        await loadLogs(nextPage);
      }
      setActivePage(nextPage);
    })();
  };

  const refresh = useCallback(async () => {
    setLoading(true);
    setActivePage(1);
    await loadLogs(1);
  }, [loadLogs]);

  const deleteHistoryLogs = useCallback(async () => {
    const parsed = Date.parse(cleanupTimestamp);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      showError(t('log.cleanup.invalid_time'));
      return;
    }
    setCleaningLogs(true);
    try {
      const res = await API.delete(
        `/api/v1/admin/log/?target_timestamp=${Math.floor(parsed / 1000)}`,
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.cleanup.failed'));
        return;
      }
      showSuccess(t('log.cleanup.success', { count: Number(data || 0) || 0 }));
      await refresh();
    } catch (error) {
      showError(error?.message || t('log.cleanup.failed'));
    } finally {
      setCleaningLogs(false);
    }
  }, [cleanupTimestamp, refresh, t]);

  useEffect(() => {
    refresh().then();
  }, [refresh]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  useEffect(() => {
    if (isAdminScope || typeof window === 'undefined') {
      return;
    }
    try {
      window.localStorage.setItem(
        USER_LOG_COLUMN_ORDER_STORAGE_KEY,
        JSON.stringify(userLogColumnOrder),
      );
    } catch (error) {
      console.warn('Failed to persist user log column order:', error);
    }
  }, [isAdminScope, userLogColumnOrder]);

  useEffect(() => {
    setActivePage(1);
  }, [searchKeyword, activeFilterKeys, username, token_name, model_name, channel, group_id, start_timestamp, end_timestamp]);

  const handleTableChange = (_, __, sorter) => {
    if (!sorter || Array.isArray(sorter) || !sorter.columnKey || !sorter.order) {
      setTableSorter({ columnKey: null, order: null });
      return;
    }
    setTableSorter({
      columnKey: sorter.columnKey,
      order: sorter.order,
    });
  };

  const filteredLogs = useMemo(() => {
    const keyword = (searchKeyword || '').toString().trim().toLowerCase();
    if (keyword === '') {
      return logs;
    }
    return logs.filter((log) => {
      const haystacks = [
        log?.content,
        log?.publicModelName,
        log?.token_name,
        log?.username,
        log?.group_name,
        log?.group_id,
        log?.trace_id,
        ...(isAdminScope
          ? [
              log?.model_name,
              log?.actualModelName,
              log?.channel_name,
              log?.channel,
            ]
          : []),
      ]
        .map((item) => (item || '').toString().toLowerCase())
        .filter((item) => item !== '');
      return haystacks.some((item) => item.includes(keyword));
    });
  }, [isAdminScope, logs, searchKeyword]);

  const sortedFilteredLogs = useMemo(() => {
    if (!tableSorter.columnKey || !tableSorter.order) {
      return filteredLogs;
    }
    const nextLogs = [...filteredLogs];
    nextLogs.sort((left, right) => {
      switch (tableSorter.columnKey) {
        case 'created_at':
          return compareNumberValue(left.created_at, right.created_at);
        case 'channel':
          return compareTextValue(
            getLogChannelLabel(left),
            getLogChannelLabel(right),
          );
        case 'group_id':
          return compareTextValue(
            left.group_name || left.group_id,
            right.group_name || right.group_id,
          );
        case 'type':
          return compareNumberValue(left.type, right.type);
        case 'billingSource':
          return compareTextValue(
            left.billing_source_name || left.billing_source,
            right.billing_source_name || right.billing_source,
          );
        case 'model_name':
          return compareTextValue(left.publicModelName, right.publicModelName);
        case 'username':
          return compareTextValue(left.username, right.username);
        case 'token_name':
          return compareTextValue(left.token_name, right.token_name);
        case 'prompt_tokens':
          return compareNumberValue(left.prompt_tokens, right.prompt_tokens);
        case 'completion_tokens':
          return compareNumberValue(
            left.completion_tokens,
            right.completion_tokens,
          );
        case 'cacheQuantity':
          return compareNumberValue(left.cacheQuantity, right.cacheQuantity);
        case 'chargeAmount':
          return compareNumberValue(left.chargeAmount, right.chargeAmount);
        default:
          return 0;
      }
    });
    if (tableSorter.order === 'descend') {
      nextLogs.reverse();
    }
    return nextLogs;
  }, [filteredLogs, tableSorter]);

  const resolveOptionLabel = useCallback(
    (filterKey, value) => {
      if (filterKey === 'channel') {
        const matched = filterOptions.channels.find((item) => item.id === value);
        return matched?.label || value;
      }
      if (filterKey === 'group_id') {
        const matched = filterOptions.groups.find((item) => item.id === value);
        return matched?.label || value;
      }
      return value;
    },
    [filterOptions.channels, filterOptions.groups]
  );

  const getLogTypeLabel = useCallback(
    (value) => {
      const matched = LOG_OPTIONS.find((item) => Number(item.value) === Number(value));
      return matched?.text || t('log.filters.empty');
    },
    [LOG_OPTIONS, t]
  );

  const totalPages = Math.max(
    Math.ceil(
      (searchKeyword.trim() === '' ? totalCount : filteredLogs.length) /
        ITEMS_PER_PAGE,
    ),
    1,
  );

  const detailBasePath = isAdminScope ? '/admin/log' : '/workspace/log';
  const tableColSpan = isAdminScope
    ? showAmountColumns()
      ? 10
      : 5
    : showAmountColumns()
      ? 7
      : 3;

  const moveUserLogColumn = useCallback((sourceKey, targetKey, placeAfter) => {
    if (!sourceKey || !targetKey || sourceKey === targetKey) {
      return;
    }
    setUserLogColumnOrder((currentOrder) => {
      const normalizedOrder = normalizeUserLogColumnOrder(currentOrder);
      if (
        !normalizedOrder.includes(sourceKey) ||
        !normalizedOrder.includes(targetKey)
      ) {
        return normalizedOrder;
      }
      const nextOrder = normalizedOrder.filter((key) => key !== sourceKey);
      const targetIndex = nextOrder.indexOf(targetKey);
      nextOrder.splice(targetIndex + (placeAfter ? 1 : 0), 0, sourceKey);
      return nextOrder;
    });
  }, []);

  const clearColumnDragState = useCallback(() => {
    draggingColumnKeyRef.current = '';
    setDraggingColumnKey('');
    setDragOverColumnKey('');
  }, []);

  const resolveLogColumns = (columns) => {
    if (isAdminScope) {
      return columns;
    }
    const columnsByKey = new Map(
      columns.map((column) => [String(column.key || column.dataIndex), column]),
    );
    const visibleOrder = [
      ...userLogColumnOrder.filter((key) => columnsByKey.has(key)),
      ...columns
        .map((column) => String(column.key || column.dataIndex))
        .filter((key) => !userLogColumnOrder.includes(key)),
    ];
    return visibleOrder.map((columnKey) => {
      const column = columnsByKey.get(columnKey);
      const originalTitle = column.title;
      return {
        ...column,
        title: (
          <div className='router-log-column-title'>
            <span
              className='router-log-column-drag-handle'
              draggable
              aria-label={t('log.table.drag_column')}
              title={t('log.table.drag_column')}
              onMouseDown={(event) => event.stopPropagation()}
              onClick={(event) => event.stopPropagation()}
              onDragStart={(event) => {
                event.stopPropagation();
                draggingColumnKeyRef.current = columnKey;
                setDraggingColumnKey(columnKey);
                event.dataTransfer.effectAllowed = 'move';
                event.dataTransfer.setData('text/plain', columnKey);
              }}
              onDragEnd={clearColumnDragState}
            />
            <span className='router-log-column-title-content'>
              {originalTitle}
            </span>
          </div>
        ),
        onHeaderCell: () => ({
          className: [
            'router-log-column-draggable',
            draggingColumnKey === columnKey
              ? 'router-log-column-dragging'
              : '',
            dragOverColumnKey === columnKey
              ? 'router-log-column-drag-target'
              : '',
          ]
            .filter(Boolean)
            .join(' '),
          onDragOver: (event) => {
            const sourceKey = draggingColumnKeyRef.current;
            if (!sourceKey || sourceKey === columnKey) {
              return;
            }
            event.preventDefault();
            event.dataTransfer.dropEffect = 'move';
            setDragOverColumnKey(columnKey);
          },
          onDragLeave: (event) => {
            if (!event.currentTarget.contains(event.relatedTarget)) {
              setDragOverColumnKey((currentKey) =>
                currentKey === columnKey ? '' : currentKey,
              );
            }
          },
          onDrop: (event) => {
            event.preventDefault();
            event.stopPropagation();
            const sourceKey =
              draggingColumnKeyRef.current ||
              event.dataTransfer.getData('text/plain');
            const targetRect = event.currentTarget.getBoundingClientRect();
            const placeAfter =
              event.clientX >= targetRect.left + targetRect.width / 2;
            moveUserLogColumn(sourceKey, columnKey, placeAfter);
            clearColumnDragState();
          },
        }),
      };
    });
  };

  return (
    <>
      <AppFilterHeader
        breadcrumbs={breadcrumbs}
        title={t('header.log')}
        actions={
          isAdminScope ? (
            <div className='router-log-cleanup-actions'>
              <label className='router-log-cleanup-label'>
                <span>{t('log.cleanup.label')}</span>
                <input
                  className='router-log-cleanup-input'
                  type='datetime-local'
                  value={cleanupTimestamp}
                  onChange={(e) => setCleanupTimestamp(e.target.value)}
                />
              </label>
              <AppButton
                type='button'
                className='router-section-button'
                onClick={deleteHistoryLogs}
                loading={cleaningLogs}
                disabled={loading}
              >
                {t('log.cleanup.button')}
              </AppButton>
            </div>
          ) : null
        }
        picker={
            <AppPopover
              open={addFilterPopupOpen}
              trigger='click'
              placement='bottomLeft'
              onOpenChange={(open) => {
                if (open) {
                  setAddFilterPopupOpen(true);
                  return;
                }
                if (!open) {
                  closeFilterDraft();
                }
              }}
              content={
                <div className='router-log-filter-picker'>
                  <div className='router-log-filter-picker-options'>
                    {availableConditionalFilterOptions.map((item) => (
                      <AppButton
                        key={item.value}
                        type='button'
                        className='router-inline-button'
                        color={draftFilterKey === item.value ? 'blue' : undefined}
                        basic={draftFilterKey !== item.value}
                        onClick={() => openFilterDraft(item.value)}
                      >
                        {item.text}
                      </AppButton>
                    ))}
                  </div>
                  {draftFilterKey !== '' && (
                    <div className='router-log-filter-editor'>
                      <div className='router-log-filter-editor-title'>
                        {
                          conditionalFilterConfig.find((item) => item.key === draftFilterKey)
                            ?.label
                        }
                      </div>
                      {draftFilterKey === 'time_range' ? (
                        <div className='router-log-filter-editor-range'>
                          <input
                            type='datetime-local'
                            value={draftFilterInputs.start_timestamp}
                            onChange={(e) =>
                              setDraftFilterInputs((prev) => ({
                                ...prev,
                                start_timestamp: e.target.value,
                              }))
                            }
                          />
                          <input
                            type='datetime-local'
                            value={draftFilterInputs.end_timestamp}
                            onChange={(e) =>
                              setDraftFilterInputs((prev) => ({
                                ...prev,
                                end_timestamp: e.target.value,
                              }))
                            }
                          />
                        </div>
                      ) : conditionalFilterConfig.find((item) => item.key === draftFilterKey)
                          ?.type === 'select' ? (
                        <AppSelect
                          className='router-section-dropdown router-log-filter-select'
                          fluid
                          search
                          clearable
                          loading={
                            draftFilterKey === 'username'
                              ? userFilterSearchLoading
                              : draftFilterKey === 'token_name'
                                ? tokenFilterSearchLoading
                                : draftFilterKey === 'model_name'
                                  ? modelFilterSearchLoading
                                : loadingFilterKeys.includes(draftFilterKey)
                          }
                          getPopupContainer={resolvePopupContainer}
                          options={
                            conditionalFilterConfig.find((item) => item.key === draftFilterKey)
                              ?.options || []
                          }
                          value={draftFilterInputs.value}
                          onClick={() => {
                            if (draftFilterKey === 'username') {
                              searchAdminUsers(draftFilterInputs.value).then();
                            } else if (draftFilterKey === 'token_name' && isAdminScope) {
                              searchAdminTokens(draftFilterInputs.value).then();
                            } else if (draftFilterKey === 'model_name' && isAdminScope) {
                              searchAdminModels(draftFilterInputs.value).then();
                            }
                          }}
                          onSearch={(value) => {
                            if (draftFilterKey === 'username') {
                              searchAdminUsers(value).then();
                            } else if (draftFilterKey === 'token_name' && isAdminScope) {
                              searchAdminTokens(value).then();
                            } else if (draftFilterKey === 'model_name' && isAdminScope) {
                              searchAdminModels(value).then();
                            }
                          }}
                          onChange={(e, { value }) =>
                            setDraftFilterInputs((prev) => ({
                              ...prev,
                              value:
                                value === null || value === undefined || value === ''
                                  ? ''
                                  : value,
                            }))
                          }
                        />
                      ) : (
                        <input
                          className='router-log-filter-editor-input'
                          type='text'
                          value={draftFilterInputs.value}
                          placeholder={
                            conditionalFilterConfig.find((item) => item.key === draftFilterKey)
                              ?.placeholder || ''
                          }
                          onChange={(e) =>
                            setDraftFilterInputs((prev) => ({
                              ...prev,
                              value: e.target.value,
                            }))
                          }
                        />
                      )}
                      <AppFormActions className='router-log-filter-editor-actions'>
                        <AppButton
                          type='button'
                          className='router-inline-button'
                          onClick={closeFilterDraft}
                        >
                          {t('common.cancel')}
                        </AppButton>
                        <AppButton
                          type='button'
                          className='router-inline-button'
                          color='blue'
                          onClick={applyFilterDraft}
                        >
                          {t('common.confirm')}
                        </AppButton>
                      </AppFormActions>
                    </div>
                  )}
                </div>
              }
            >
              <AppButton
                type='button'
                className='router-section-button'
                disabled={availableConditionalFilterOptions.length === 0}
                onClick={() => setAddFilterPopupOpen(true)}
              >
                {t('log.filters.add')}
              </AppButton>
            </AppPopover>
        }
        query={
          <>
            <div className='router-log-query-box router-log-query-box-inline'>
              <div className='router-log-query-fields'>
                {visibleFilterConfig.map((item) => (
                  <div key={item.key} className='router-log-filter-chip router-log-filter-chip-static'>
                    <span className='router-log-filter-chip-label'>
                      {item.label}
                    </span>
                    <span className='router-log-filter-chip-value'>
                      {renderFilterSummary(item.key, inputs, t, {
                        resolveOptionLabel,
                        logTypeLabel: getLogTypeLabel(logType),
                      })}
                    </span>
                    <button
                      type='button'
                      className='router-log-filter-chip-remove'
                      onClick={() => removeConditionalFilter(item.key)}
                    >
                      ×
                    </button>
                  </div>
                ))}
                <div className='router-log-search-input'>
                  <input
                    placeholder={t('log.search')}
                    value={searchKeyword}
                    onChange={(e) => setSearchKeyword(e.target.value)}
                  />
                </div>
              </div>
            </div>
            <AppButton
              type='button'
              className='router-section-button router-log-query-button'
              onClick={refresh}
              loading={loading}
            >
              {t('log.buttons.submit')}
            </AppButton>
          </>
        }
        endClassName='router-log-query-wrap'
      />
      <div className='router-table-scroll-x'>
        <AppTable
          className='router-list-table router-table-fit-page router-log-table'
          pagination={false}
          scroll={{ x: LOG_LIST_TABLE_MIN_WIDTH }}
          onChange={handleTableChange}
          rowKey={(log) =>
            log.id ||
            log.trace_id ||
            `${log.timestamp || ''}-${log.type || ''}-${log.token_name || ''}-${log.publicModelName || ''}`
          }
          dataSource={sortedFilteredLogs
            .slice((activePage - 1) * ITEMS_PER_PAGE, activePage * ITEMS_PER_PAGE)
            .filter((log) => !log.deleted)}
          locale={{ emptyText: loading ? t('common.loading') : t('task.empty') }}
          onRow={(log) => ({
            className: 'router-row-clickable',
            onClick: () =>
              log.id
                ? navigate(`${detailBasePath}/${log.id}${location.search || ''}`)
                : undefined,
          })}
          columns={resolveLogColumns([
          {
            title: t('log.table.time'),
            dataIndex: 'created_at',
            key: 'created_at',
            className: 'router-table-col-datetime',
            width: LOG_LIST_COLUMN_WIDTHS.time,
            sorter: true,
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'created_at' ? tableSorter.order : null,
            render: (value) => renderTimestamp(value),
          },
          ...(isAdminScope
            ? [
                {
                  title: t('log.table.channel'),
                  key: 'channel',
                  width: LOG_LIST_COLUMN_WIDTHS.channel,
                  ellipsis: true,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'channel' ? tableSorter.order : null,
                  render: (_, log) =>
                    log.channel ? (
                      <Link
                        to={`/admin/channel/detail/${log.channel}`}
                        state={{ from: currentPagePath }}
                        onClick={(e) => e.stopPropagation()}
                      >
                        <AppTag className='router-tag'>
                          {getLogChannelLabel(log)}
                        </AppTag>
                      </Link>
                    ) : (
                      ''
                    ),
                },
                {
                  title: t('log.table.group'),
                  key: 'group_id',
                  width: LOG_LIST_COLUMN_WIDTHS.group,
                  ellipsis: true,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'group_id' ? tableSorter.order : null,
                  render: (_, log) =>
                    log.group_id ? (
                      <Link
                        to={`/admin/group/detail/${log.group_id}`}
                        state={{ from: currentPagePath }}
                        onClick={(e) => e.stopPropagation()}
                      >
                        <AppTag className='router-tag'>
                          {log.group_name || log.group_id}
                        </AppTag>
                      </Link>
                    ) : (
                      '-'
                    ),
                },
              ]
            : []),
          ...(isAdminScope
            ? [
                {
                  title: t('log.table.type'),
                  dataIndex: 'type',
                  key: 'type',
                  className: 'router-table-col-type-narrow',
                  width: LOG_LIST_COLUMN_WIDTHS.type,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'type' ? tableSorter.order : null,
                  render: (value) => renderType(value),
                },
              ]
            : [
                {
                  title: t('log.table.billing_source'),
                  key: 'billingSource',
                  width: LOG_LIST_COLUMN_WIDTHS.billingSource,
                  ellipsis: true,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'billingSource'
                      ? tableSorter.order
                      : null,
                  render: (_, log) => renderBillingSource(log, t),
                },
              ]),
          {
            title: t('log.table.model'),
            key: 'model_name',
            width: LOG_LIST_COLUMN_WIDTHS.model,
            ellipsis: true,
            sorter: true,
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'model_name' ? tableSorter.order : null,
            render: (_, log) =>
              log?.publicModelName ? renderColorLabel(log.publicModelName) : '',
          },
          ...(showAmountColumns()
            ? [
                ...(isAdminScope
                  ? [
                      {
                        title: t('log.table.username'),
                        key: 'username',
                        width: LOG_LIST_COLUMN_WIDTHS.username,
                        ellipsis: true,
                        sorter: true,
                        sortDirections: ['ascend', 'descend'],
                        sortOrder:
                          tableSorter.columnKey === 'username'
                            ? tableSorter.order
                            : null,
                        render: (_, log) =>
                          log.username ? (
                            <Link
                              to={`/user/detail/${log.user_id}`}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <AppTag className='router-tag'>{log.username}</AppTag>
                            </Link>
                          ) : (
                            ''
                          ),
                      },
                    ]
                  : []),
                {
                  title: t('log.table.token_name'),
                  dataIndex: 'token_name',
                  key: 'token_name',
                  width: LOG_LIST_COLUMN_WIDTHS.tokenName,
                  ellipsis: true,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'token_name'
                      ? tableSorter.order
                      : null,
                  render: (value) => (value ? renderColorLabel(value) : ''),
                },
                {
                  title: t('log.table.prompt_tokens'),
                  dataIndex: 'prompt_tokens',
                  key: 'prompt_tokens',
                  className: 'router-table-col-status-narrow',
                  width: LOG_LIST_COLUMN_WIDTHS.promptTokens,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'prompt_tokens'
                      ? tableSorter.order
                      : null,
                  render: (value) => value || '',
                },
                {
                  title: t('log.table.completion_tokens'),
                  dataIndex: 'completion_tokens',
                  key: 'completion_tokens',
                  className: 'router-table-col-status-narrow',
                  width: LOG_LIST_COLUMN_WIDTHS.completionTokens,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'completion_tokens'
                      ? tableSorter.order
                      : null,
                  render: (value) => value || '',
                },
                {
                  title: t('log.table.cache_tokens'),
                  key: 'cacheQuantity',
                  className: 'router-table-col-status-narrow',
                  width: LOG_LIST_COLUMN_WIDTHS.cacheTokens,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'cacheQuantity'
                      ? tableSorter.order
                      : null,
                  render: (_, log) => {
                    const read = formatCompactNumber(log.cacheReadQuantity);
                    const write = formatCompactNumber(log.cacheWriteQuantity);
                    if (!read && !write) {
                      return '';
                    }
                    return `${read || '0'} / ${write || '0'}`;
                  },
                },
                {
                  title: isAdminScope ? (
                    <div className='router-table-header-with-control'>
                      <span>{t('log.table.quota')}</span>
                      <UnitDropdown
                        variant='header'
                        compact
                        options={displayUnitOptions}
                        value={displayUnit}
                        onClick={(e) => {
                          e.stopPropagation();
                        }}
                        onChange={(_, { value }) => {
                          setDisplayUnit((value || '').toString());
                        }}
                      />
                    </div>
                  ) : (
                    <span>{t('log.table.quota')}</span>
                  ),
                  dataIndex: 'chargeAmount',
                  key: 'chargeAmount',
                  width: LOG_LIST_COLUMN_WIDTHS.quota,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder:
                    tableSorter.columnKey === 'chargeAmount'
                      ? tableSorter.order
                      : null,
                  render: (value) =>
                    isAdminScope
                      ? formatDisplayAmountFromChargeAmount(value, displayUnit, currencyIndex)
                      : value
                        ? formatDisplayAmountFromChargeAmount(value, displayUnit, currencyIndex, {
                            includeSymbol: true,
                            chargeMode: 'compact',
                          })
                        : '',
                },
              ]
            : []),
          ])}
          footer={() => (
            <AppToolbar
              start={
                <AppPagination
                  className='router-page-pagination'
                  activePage={activePage}
                  onPageChange={onPaginationChange}
                  siblingRange={1}
                  totalPages={totalPages}
                />
              }
            />
          )}
        />
      </div>
    </>
  );
};

export default LogsTable;
