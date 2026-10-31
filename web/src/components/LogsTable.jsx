import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';
import { useTranslation } from 'react-i18next';
import UnitDropdown from './UnitDropdown';
import useList, { sorterToSort, sortOrderForColumn } from '../hooks/useList';
import { parseListPageSize, parsePageParam } from '../hooks/useUrlState';
import {
  buildQueryFilterSignature,
  resolveListEffectPage,
} from '../helpers/listState.mjs';

import { LIST_PAGE_SIZE } from '../constants';
import { exportCSV } from '../helpers/csv';
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
  AppEmpty,
  AppErrorState,
  AppFilterHeader,
  AppFormActions,
  AppModal,
  AppPagination,
  AppPopconfirm,
  AppTable,
  AppTag,
  AppToolbar,
} from '../router-ui';
import ListFilterBar from './ListFilterBar';
import {
  DEFAULT_LOG_COLUMN_WIDTHS,
  LOG_COLUMN_MAX_WIDTH,
  LOG_COLUMN_MIN_WIDTH,
  LOG_SORT_FIELD_MAP,
  cleanupDatetimeLocalValue,
  currentDatetimeLocalValue,
  formatCompactNumber,
  getLogChannelLabel,
  getLogColumnOrderStorageKey,
  getLogColumnWidthStorageKey,
  loadLogColumnOrder,
  loadLogColumnWidths,
  normalizeLogColumnOrder,
  normalizeLogEntry,
  parseDatetimeInput,
  parseLogFiltersFromSearch,
  renderBillingSource,
  renderFilterSummary,
  renderTimestamp,
  renderType,
  startOfTodayDatetimeLocalValue,
  toDatetimeLocalValue,
  toTokenFilterOption,
  toUserFilterOption,
} from './LogsTable.helpers';

const COMMON_LOG_FILTER_QUERY_KEYS = [
  'log_type',
  'type',
  'start_timestamp',
  'end_timestamp',
  'token_name',
  'model_name',
];
const ADMIN_LOG_FILTER_QUERY_KEYS = [
  ...COMMON_LOG_FILTER_QUERY_KEYS,
  'channel',
  'group_id',
  'username',
];

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
  // Keep URL-driven filter hydration separate from pagination/sort/navigation
  // params. If this depended on the whole location.search, writing `page=2`
  // would create fresh filter objects, rebuild fetchLogs, and reset page to 1.
  const urlFilterSignature = useMemo(
    () =>
      buildQueryFilterSignature(
        location.search,
        isAdminScope
          ? ADMIN_LOG_FILTER_QUERY_KEYS
          : COMMON_LOG_FILTER_QUERY_KEYS,
      ),
    [isAdminScope, location.search],
  );
  const urlSearchFilters = useMemo(
    () => parseLogFiltersFromSearch(urlFilterSignature, isAdminScope),
    [isAdminScope, urlFilterSignature],
  );
  const initialPageSize = useMemo(() => {
    const raw = new URLSearchParams(location.search).get('page_size');
    return raw ? parseListPageSize(raw) : LIST_PAGE_SIZE;
    // Seed once from the URL; later changes flow through setPageSize.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  // Seed the page from the URL once so returning from a log detail (or a shared
  // link) restores the same page; filter/sort/size changes reset back to page 1.
  const initialPage = useMemo(
    () => parsePageParam(new URLSearchParams(location.search).get('page')),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  const [logType, setLogType] = useState(urlSearchFilters.logType);
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
  const [inputs, setInputs] = useState(urlSearchFilters.inputs);
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
    urlSearchFilters.activeFilterKeys
  );
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([])
  );
  const [logColumnOrder, setLogColumnOrder] = useState(() =>
    loadLogColumnOrder(isAdminScope),
  );
  const [logColumnWidths, setLogColumnWidths] = useState(() =>
    loadLogColumnWidths(isAdminScope),
  );
  const [draggingColumnKey, setDraggingColumnKey] = useState('');
  const [dragOverColumnKey, setDragOverColumnKey] = useState('');
  const [resizingColumnKey, setResizingColumnKey] = useState('');
  const [cleanupTimestamp, setCleanupTimestamp] = useState(
    cleanupDatetimeLocalValue(),
  );
  const [cleanupModalOpen, setCleanupModalOpen] = useState(false);
  const [cleaningLogs, setCleaningLogs] = useState(false);
  const draggingColumnKeyRef = useRef('');
  const resizingColumnRef = useRef(null);

  // Seed the sort from the URL once (frontend column key + direction), defaulting
  // to created-at descending to match the backend default.
  const initialSort = useMemo(() => {
    const params = new URLSearchParams(location.search);
    const field = (params.get('order_by') || '').trim();
    const order = (params.get('order') || '').trim();
    if (LOG_SORT_FIELD_MAP[field] && (order === 'asc' || order === 'desc')) {
      return { field, order };
    }
    return { field: 'created_at', order: 'desc' };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Backend-paged fetcher: page-overwrite + true global sort. Frontend column
  // keys map to whitelisted backend columns; structured filters are applied
  // server-side. Filter identity changes reset to page 1 via the effect below.
  const fetchLogs = useCallback(
    async ({ page, pageSize, orderBy, order }) => {
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
      const params = new URLSearchParams();
      params.set('page', String(page));
      params.set('page_size', String(pageSize));
      const backendOrderBy = LOG_SORT_FIELD_MAP[orderBy] || '';
      if (backendOrderBy) {
        params.set('order_by', backendOrderBy);
        params.set('order', order === 'asc' ? 'asc' : 'desc');
      }
      params.set('type', String(queryLogType));
      params.set('token_name', queryTokenName);
      params.set('model_name', queryModelName);
      params.set('start_timestamp', String(localStartTimestamp));
      params.set('end_timestamp', String(localEndTimestamp));
      if (isAdminScope) {
        params.set('username', queryUsername);
        params.set('group_id', queryGroupID);
        params.set('channel', queryChannel);
      }
      const base = isAdminScope ? '/api/v1/admin/log/' : '/api/v1/public/log';
      const res = await API.get(`${base}?${params.toString()}`);
      const { success, message, data, meta } = res.data;
      if (!success) {
        showError(message);
        throw new Error(message || 'load logs failed');
      }
      const rows = Array.isArray(data) ? data.map(normalizeLogEntry) : [];
      return { rows, total: Number(meta?.total || rows.length || 0) };
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
    ],
  );

  // This key describes only the fields that alter the backend log query. It is
  // deliberately independent from page, page size and sort, whose handlers
  // already perform their own loads. Sorting the active keys makes the key
  // insensitive to array identity/order and prevents reference-only reloads.
  const logRequestFilterKey = useMemo(
    () =>
      JSON.stringify({
        scope: isAdminScope ? 'admin' : 'workspace',
        active: [...activeFilterKeys].sort(),
        logType,
        username,
        token_name,
        model_name,
        start_timestamp,
        end_timestamp,
        channel,
        group_id,
      }),
    [
      activeFilterKeys,
      channel,
      end_timestamp,
      group_id,
      isAdminScope,
      logType,
      model_name,
      start_timestamp,
      token_name,
      username,
    ],
  );

  const {
    rows: logs,
    total: totalCount,
    loading,
    loadError,
    page: activePage,
    pageSize,
    sort,
    load: loadLogs,
    setPageSize,
    setSort,
  } = useList({
    fetcher: fetchLogs,
    pageSize: initialPageSize,
    initialPage,
    initialSort,
  });

  // Write active filters back to the URL so a refresh or shared link restores
  // them. The written set mirrors parseLogFiltersFromSearch exactly, so the
  // round-trip is symmetric; `source` (breadcrumb origin) is preserved and
  // navigate replace avoids spamming history. A diff guard prevents loops.
  useEffect(() => {
    const query = new URLSearchParams();
    const currentParams = new URLSearchParams(location.search || '');
    // `tab` is owned by the embedding layout (e.g. /workspace/topup?tab=logs);
    // rebuilding the query purely from LogsTable state would drop it and bounce
    // the parent back to its default tab, so carry it through unchanged.
    const tab = currentParams.get('tab');
    if (tab) {
      query.set('tab', tab);
    }
    const source = currentParams.get('source');
    if (source) {
      query.set('source', source);
    }
    if (activeFilterKeys.includes('log_type') && Number(logType) > 0) {
      query.set('log_type', String(logType));
    }
    if (activeFilterKeys.includes('time_range')) {
      if ((inputs.start_timestamp || '').trim() !== '') {
        query.set('start_timestamp', inputs.start_timestamp);
      }
      if ((inputs.end_timestamp || '').trim() !== '') {
        query.set('end_timestamp', inputs.end_timestamp);
      }
    }
    const textFilterKeys = ['token_name', 'model_name'];
    if (isAdminScope) {
      textFilterKeys.push('channel', 'group_id', 'username');
    }
    textFilterKeys.forEach((key) => {
      if (activeFilterKeys.includes(key) && (inputs[key] || '').trim() !== '') {
        query.set(key, inputs[key].trim());
      }
    });
    if (sort?.field) {
      query.set('order_by', sort.field);
      query.set('order', sort.order === 'asc' ? 'asc' : 'desc');
    }
    if (Number(pageSize) !== LIST_PAGE_SIZE) {
      query.set('page_size', String(pageSize));
    }
    // Page position (default 1 is stripped). Folded into this rebuild-from-state
    // effect so it stays consistent with the filters/sort/size it writes; any of
    // those resets the page to 1 via loadLogs(1), which drops the param here.
    if (Number(activePage) > 1) {
      query.set('page', String(activePage));
    }
    const nextSearch = query.toString();
    const currentSearch = location.search.startsWith('?')
      ? location.search.slice(1)
      : location.search;
    if (nextSearch === currentSearch) {
      return;
    }
    navigate(
      {
        pathname: location.pathname,
        search: nextSearch ? `?${nextSearch}` : '',
      },
      { replace: true },
    );
  }, [
    activeFilterKeys,
    inputs,
    logType,
    sort,
    pageSize,
    activePage,
    isAdminScope,
    location.pathname,
    location.search,
    navigate,
  ]);

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
        type:
          isAdminScope || filterOptions.tokenNames.length > 0 ? 'select' : 'text',
        options: filterOptions.tokenNames,
      },
      {
        key: 'model_name',
        label: t('log.table.model_name'),
        placeholder: t('log.table.model_name_placeholder'),
        type:
          isAdminScope || filterOptions.modelNames.length > 0 ? 'select' : 'text',
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
    setInputs(urlSearchFilters.inputs);
    setLogType(urlSearchFilters.logType);
    setActiveFilterKeys(urlSearchFilters.activeFilterKeys);
    // The request-key effect below owns the page-1 reset. Page/sort-only URL
    // changes leave urlFilterSignature unchanged, so they never enter here.
  }, [urlSearchFilters]);

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
                ? data.token_names.map((item) => ({
                    key: item,
                    text: item,
                    value: item,
                  }))
                : []
              : prev.tokenNames,
          modelNames:
            !normalizedFilterKey || normalizedFilterKey === 'model_name'
              ? Array.isArray(data?.model_names)
                ? data.model_names.map((item) => ({
                    key: item,
                    text: item,
                    value: item,
                  }))
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

  const getLogFilterConfig = useCallback(
    (filterKey) =>
      conditionalFilterConfig.find((item) => item.key === filterKey) || null,
    [conditionalFilterConfig]
  );

  const getLogInitialDraft = useCallback(
    (filterKey) => {
      if (filterKey === 'time_range') {
        return {
          value: '',
          start_timestamp:
            toDatetimeLocalValue(inputs.start_timestamp) ||
            startOfTodayDatetimeLocalValue(),
          end_timestamp:
            toDatetimeLocalValue(inputs.end_timestamp) ||
            currentDatetimeLocalValue(),
        };
      }
      if (filterKey === 'log_type') {
        return { value: logType > 0 ? logType : '' };
      }
      return { value: (inputs[filterKey] || '').toString() };
    },
    [inputs, logType]
  );

  const onLogDraftOpen = useCallback(
    (filterKey) => {
      if (['channel', 'group_id'].includes(filterKey)) {
        loadFilterOptions(filterKey).then();
      } else if (
        !isAdminScope &&
        ['token_name', 'model_name'].includes(filterKey)
      ) {
        // Normal users get a dropdown of the token/model names they have
        // actually used (from /api/v1/public/log/options) instead of typing
        // blind. Options load lazily on first open, mirroring channel/group.
        loadFilterOptions(filterKey).then();
      }
    },
    [isAdminScope, loadFilterOptions]
  );

  const applyLogFilterDraft = useCallback(
    (filterKey, draft) => {
      if (filterKey === '') {
        return false;
      }
      const config = conditionalFilterConfig.find(
        (item) => item.key === filterKey
      );
      if (!config) {
        return false;
      }
      if (config.type === 'time_range') {
        const nextStart = (draft.start_timestamp || '').trim();
        const nextEnd = (draft.end_timestamp || '').trim();
        if (nextStart === '' && nextEnd === '') {
          showError(t('log.filters.empty'));
          return false;
        }
        setInputs((prev) => ({
          ...prev,
          start_timestamp: nextStart,
          end_timestamp: nextEnd,
        }));
      } else if (filterKey === 'log_type') {
        const nextValue = Number(draft.value || 0);
        if (!Number.isFinite(nextValue) || nextValue <= 0) {
          showError(t('log.filters.empty'));
          return false;
        }
        setLogType(nextValue);
      } else {
        const nextValue = (draft.value || '').toString().trim();
        if (nextValue === '') {
          showError(t('log.filters.empty'));
          return false;
        }
        setInputs((prev) => ({
          ...prev,
          [filterKey]: nextValue,
        }));
      }
      setActiveFilterKeys((prev) =>
        prev.includes(filterKey) ? prev : [...prev, filterKey]
      );
      return true;
    },
    [conditionalFilterConfig, t]
  );

  const getLogSelectLoading = useCallback(
    (filterKey) =>
      filterKey === 'username'
        ? userFilterSearchLoading
        : filterKey === 'token_name'
          ? tokenFilterSearchLoading
          : filterKey === 'model_name'
            ? modelFilterSearchLoading
            : loadingFilterKeys.includes(filterKey),
    [
      loadingFilterKeys,
      modelFilterSearchLoading,
      tokenFilterSearchLoading,
      userFilterSearchLoading,
    ]
  );

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

  const clearAllFilters = useCallback(() => {
    setActiveFilterKeys([]);
    setLogType(0);
    setInputs((prev) => ({
      ...prev,
      username: '',
      token_name: '',
      model_name: '',
      start_timestamp: '',
      end_timestamp: '',
      channel: '',
      group_id: '',
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

  const onPaginationChange = (e, { activePage: nextActivePage, pageSize: nextSize }) => {
    const size = Number(nextSize) > 0 ? Number(nextSize) : pageSize;
    if (size !== pageSize) {
      // Page-size change reloads page 1 at the new size; the URL effect mirrors it.
      setPageSize(size);
      return;
    }
    const nextPage = Number(nextActivePage) > 0 ? Number(nextActivePage) : 1;
    loadLogs(nextPage);
  };

  const refresh = useCallback(() => loadLogs(1), [loadLogs]);

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

  // React StrictMode replays mount effects. A boolean "did mount" ref would
  // misclassify that replay as a filter update and load page 1. Compare the
  // semantic request key instead: equal means mount/replay, changed means a
  // real filter/scope change.
  const previousLogRequestFilterKeyRef = useRef(logRequestFilterKey);
  useEffect(() => {
    const targetPage = resolveListEffectPage(
      previousLogRequestFilterKeyRef.current,
      logRequestFilterKey,
      initialPage,
    );
    previousLogRequestFilterKeyRef.current = logRequestFilterKey;
    loadLogs(targetPage);
  }, [initialPage, loadLogs, logRequestFilterKey]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  useEffect(() => {
    setLogColumnOrder(loadLogColumnOrder(isAdminScope));
    setLogColumnWidths(loadLogColumnWidths(isAdminScope));
    draggingColumnKeyRef.current = '';
    setDraggingColumnKey('');
    setDragOverColumnKey('');
    setResizingColumnKey('');
  }, [isAdminScope]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    try {
      window.localStorage.setItem(
        getLogColumnOrderStorageKey(isAdminScope),
        JSON.stringify(logColumnOrder),
      );
    } catch (error) {
      console.warn('Failed to persist log column order:', error);
    }
  }, [isAdminScope, logColumnOrder]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    try {
      window.localStorage.setItem(
        getLogColumnWidthStorageKey(isAdminScope),
        JSON.stringify(logColumnWidths),
      );
    } catch (error) {
      console.warn('Failed to persist log column widths:', error);
    }
  }, [isAdminScope, logColumnWidths]);

  const handleTableChange = (_, __, sorter) => {
    setSort(sorterToSort(sorter));
  };

  const [exportingFull, setExportingFull] = useState(false);

  const handleExportCsv = useCallback(() => {
    const stamp = timestamp2string(Math.floor(Date.now() / 1000)).replace(
      /[^0-9]/g,
      '',
    );
    const columns = [
      {
        key: 'created_at',
        label: t('log.table.time'),
        format: (v) => (v ? timestamp2string(v) : ''),
      },
    ];
    if (isAdminScope) {
      columns.push({ key: 'channel', label: t('log.table.channel') });
      columns.push({ key: 'username', label: t('log.table.username') });
    }
    columns.push(
      { key: 'token_name', label: t('log.table.token_name') },
      { key: 'publicModelName', label: t('log.table.model') },
      { key: 'prompt_tokens', label: t('log.table.prompt_tokens') },
      { key: 'completion_tokens', label: t('log.table.completion_tokens') },
      { key: 'chargeAmount', label: t('log.table.quota') },
      { key: 'content', label: t('log.table.detail') },
    );
    // 拉一份全量筛选结果(覆盖式分页下当前页不代表完整集合),
    // 再就地 exportCSV;与现有 fetchLogs 走同一后端参数,避免重复拼接。
    setExportingFull(true);
    const params = new URLSearchParams();
    params.set('page', '1');
    // 10k 仍是 100 的整数倍;后端若有上限会失败由 try/catch 兜底。
    params.set('page_size', '10000');
    if (sort?.orderBy) {
      const backendOrderBy = LOG_SORT_FIELD_MAP[sort.orderBy] || '';
      if (backendOrderBy) {
        params.set('order_by', backendOrderBy);
        params.set('order', sort.order === 'asc' ? 'asc' : 'desc');
      }
    }
    const queryLogType = activeFilterKeys.includes('log_type')
      ? logType
      : 0;
    params.set('type', String(queryLogType));
    params.set('token_name', activeFilterKeys.includes('token_name') ? token_name : '');
    params.set('model_name', activeFilterKeys.includes('model_name') ? model_name : '');
    params.set('start_timestamp', String(start_timestamp || 0));
    params.set('end_timestamp', String(end_timestamp || 0));
    if (isAdminScope) {
      params.set('username', activeFilterKeys.includes('username') ? username : '');
      params.set('group_id', activeFilterKeys.includes('group_id') ? group_id : '');
      params.set('channel', activeFilterKeys.includes('channel') ? channel : '');
    }
    const base = isAdminScope ? '/api/v1/admin/log/' : '/api/v1/public/log';
    API.get(`${base}?${params.toString()}`)
      .then((res) => {
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('log.messages.load_failed'));
          return;
        }
        const rows = (Array.isArray(data) ? data : []).map((log) => ({
          ...log,
          channel: getLogChannelLabel(log),
        }));
        if (rows.length === 0) {
          showError(t('log.export.empty'));
          return;
        }
        exportCSV(
          `logs-${isAdminScope ? 'admin' : 'mine'}-${stamp}.csv`,
          columns,
          rows,
        );
        showSuccess(t('log.export.success', { count: rows.length }));
      })
      .catch((error) => {
        showError(error?.message || t('log.messages.load_failed'));
      })
      .finally(() => {
        setExportingFull(false);
      });
  }, [
    activeFilterKeys,
    channel,
    end_timestamp,
    getLogChannelLabel,
    group_id,
    isAdminScope,
    logType,
    model_name,
    sort,
    start_timestamp,
    t,
    token_name,
    username,
  ]);

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


  const detailBasePath = isAdminScope ? '/admin/log' : '/workspace/log';
  const logTableScrollWidth = Math.max(
    LOG_LIST_TABLE_MIN_WIDTH,
    Object.values(logColumnWidths).reduce((total, width) => total + Number(width || 0), 0),
  );

  const moveLogColumn = useCallback((sourceKey, targetKey, placeAfter) => {
    if (!sourceKey || !targetKey || sourceKey === targetKey) {
      return;
    }
    setLogColumnOrder((currentOrder) => {
      const normalizedOrder = normalizeLogColumnOrder(currentOrder, isAdminScope);
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
  }, [isAdminScope]);

  const clearColumnDragState = useCallback(() => {
    draggingColumnKeyRef.current = '';
    setDraggingColumnKey('');
    setDragOverColumnKey('');
  }, []);

  const handleColumnResizeStart = useCallback((event, columnKey) => {
    event.preventDefault();
    event.stopPropagation();
    const startWidth = Number(logColumnWidths[columnKey] || DEFAULT_LOG_COLUMN_WIDTHS[columnKey]);
    resizingColumnRef.current = {
      key: columnKey,
      startX: event.clientX,
      startWidth: Number.isFinite(startWidth) ? startWidth : LOG_COLUMN_MIN_WIDTH,
    };
    setResizingColumnKey(columnKey);
  }, [logColumnWidths]);

  useEffect(() => {
    if (!resizingColumnKey) {
      return undefined;
    }
    const handleMouseMove = (event) => {
      const resizingColumn = resizingColumnRef.current;
      if (!resizingColumn?.key) {
        return;
      }
      const nextWidth = Math.min(
        LOG_COLUMN_MAX_WIDTH,
        Math.max(
          LOG_COLUMN_MIN_WIDTH,
          resizingColumn.startWidth + event.clientX - resizingColumn.startX,
        ),
      );
      setLogColumnWidths((currentWidths) => ({
        ...currentWidths,
        [resizingColumn.key]: nextWidth,
      }));
    };
    const handleMouseUp = () => {
      resizingColumnRef.current = null;
      setResizingColumnKey('');
    };
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };
  }, [resizingColumnKey]);

  const resolveLogColumns = (columns) => {
    const columnsByKey = new Map(
      columns.map((column) => [String(column.key || column.dataIndex), column]),
    );
    const visibleOrder = [
      ...logColumnOrder.filter((key) => columnsByKey.has(key)),
      ...columns
        .map((column) => String(column.key || column.dataIndex))
        .filter((key) => !logColumnOrder.includes(key)),
    ];
    return visibleOrder.map((columnKey) => {
      const column = columnsByKey.get(columnKey);
      const originalTitle = column.title;
      const columnWidth = logColumnWidths[columnKey] || column.width || DEFAULT_LOG_COLUMN_WIDTHS[columnKey];
      return {
        ...column,
        width: columnWidth,
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
            <span
              className='router-log-column-resize-handle'
              role='separator'
              aria-label={t('log.table.resize_column')}
              title={t('log.table.resize_column')}
              onMouseDown={(event) => handleColumnResizeStart(event, columnKey)}
              onClick={(event) => event.stopPropagation()}
            />
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
            resizingColumnKey === columnKey
              ? 'router-log-column-resizing'
              : '',
          ]
            .filter(Boolean)
            .join(' '),
          style: { width: columnWidth, minWidth: columnWidth },
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
            moveLogColumn(sourceKey, columnKey, placeAfter);
            clearColumnDragState();
          },
        }),
        onCell: (record, index) => {
          const previous =
            typeof column.onCell === 'function'
              ? column.onCell(record, index) || {}
              : {};
          return typeof originalTitle === 'string'
            ? { ...previous, 'data-label': originalTitle }
            : previous;
        },
      };
    });
  };

  return (
    <>
      <AppFilterHeader
        breadcrumbs={breadcrumbs}
        title={t('header.log')}
        actions={
          <div className='router-log-cleanup-actions'>
            <AppButton
              type='button'
              className='router-section-button'
              onClick={handleExportCsv}
              disabled={loading || exportingFull}
              loading={exportingFull}
            >
              {t('common.export_csv')}
            </AppButton>
            {isAdminScope ? (
              <AppButton
                type='button'
                className='router-section-button router-danger-button'
                onClick={() => setCleanupModalOpen(true)}
                disabled={loading}
              >
                {t('log.cleanup.button')}
              </AppButton>
            ) : null}
          </div>
        }
        query={
          <ListFilterBar
            availableOptions={availableConditionalFilterOptions}
            visibleFilters={visibleFilterConfig}
            getFilterConfig={getLogFilterConfig}
            getInitialDraft={getLogInitialDraft}
            onDraftOpen={onLogDraftOpen}
            getSelectLoading={getLogSelectLoading}
            onSelectOpen={(filterKey, currentValue) => {
              if (filterKey === 'username') {
                searchAdminUsers(currentValue).then();
              } else if (filterKey === 'token_name' && isAdminScope) {
                searchAdminTokens(currentValue).then();
              } else if (filterKey === 'model_name' && isAdminScope) {
                searchAdminModels(currentValue).then();
              }
            }}
            onSelectSearch={(filterKey, keyword) => {
              if (filterKey === 'username') {
                searchAdminUsers(keyword).then();
              } else if (filterKey === 'token_name' && isAdminScope) {
                searchAdminTokens(keyword).then();
              } else if (filterKey === 'model_name' && isAdminScope) {
                searchAdminModels(keyword).then();
              }
            }}
            onApplyDraft={applyLogFilterDraft}
            onRemoveFilter={removeConditionalFilter}
            renderSummary={(key) =>
              renderFilterSummary(key, inputs, t, {
                resolveOptionLabel,
                logTypeLabel: getLogTypeLabel(logType),
              })
            }
            onQuery={refresh}
            queryLoading={loading}
            onClearFilters={clearAllFilters}
            clearDisabled={activeFilterKeys.length === 0}
            addButtonText={t('log.filters.add')}
            addButtonClassName='router-section-button'
            queryButtonText={t('log.buttons.submit')}
            queryButtonClassName='router-section-button router-log-query-button'
            pickerOptionBasic
          />
        }
        endClassName='router-log-query-wrap'
      />

      <AppModal
        open={cleanupModalOpen}
        title={t('log.cleanup.confirm_title')}
        onCancel={() => setCleanupModalOpen(false)}
        onClose={() => setCleanupModalOpen(false)}
        footer={
          <AppFormActions>
            <AppButton type='button' onClick={() => setCleanupModalOpen(false)}>
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              className='router-danger-button'
              loading={cleaningLogs}
              onClick={async () => {
                await deleteHistoryLogs();
                setCleanupModalOpen(false);
              }}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        }
      >
        <div className='router-settings-page-block'>
          <label className='router-log-cleanup-label'>
            <span>{t('log.cleanup.label')}</span>
            <input
              className='router-log-cleanup-input'
              type='datetime-local'
              value={cleanupTimestamp}
              onChange={(e) => setCleanupTimestamp(e.target.value)}
            />
          </label>
          <div className='router-text-muted'>{t('log.cleanup.confirm_description')}</div>
        </div>
      </AppModal>
      <div className='router-table-scroll-x'>
        <AppTable
          className='router-list-table router-table-fit-page router-log-table router-table-cardify'
          pagination={false}
          scroll={{ x: logTableScrollWidth }}
          onChange={handleTableChange}
          rowKey={(log) =>
            log.id ||
            log.trace_id ||
            `${log.timestamp || ''}-${log.type || ''}-${log.token_name || ''}-${log.publicModelName || ''}`
          }
          dataSource={logs.filter((log) => !log.deleted)}
          locale={{
            emptyText: loading ? (
              t('common.loading')
            ) : loadError ? (
              <AppErrorState
                message={t('common.load_failed')}
                onRetry={refresh}
                retryText={t('common.retry')}
              />
            ) : (
              <AppEmpty
                action={
                  <AppButton
                    color='blue'
                    onClick={() => navigate('/workspace/models')}
                  >
                    {t('log.empty_cta_action')}
                  </AppButton>
                }
              >
                {t('log.empty_cta')}
              </AppEmpty>
            ),
          }}
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
            width: LOG_LIST_COLUMN_WIDTHS.time,
            sorter: true,
            sortDirections: ['ascend', 'descend'],
            sortOrder: sortOrderForColumn(sort, 'created_at'),
            render: (value) => renderTimestamp(value),
          },
          ...(isAdminScope
            ? [
                {
                  title: t('log.table.channel'),
                  key: 'channel',
                  width: LOG_LIST_COLUMN_WIDTHS.channel,
                  ellipsis: true,
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
                  width: LOG_LIST_COLUMN_WIDTHS.type,
                  render: (value) => renderType(value, t),
                },
              ]
            : [
                {
                  title: t('log.table.billing_source'),
                  key: 'billingSource',
                  width: LOG_LIST_COLUMN_WIDTHS.billingSource,
                  ellipsis: true,
                  render: (_, log) => renderBillingSource(log, t),
                },
              ]),
          {
            title: t('log.table.model'),
            key: 'model_name',
            width: LOG_LIST_COLUMN_WIDTHS.model,
            ellipsis: true,
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
                  render: (value) => (value ? renderColorLabel(value) : ''),
                },
                {
                  title: t('log.table.prompt_tokens'),
                  dataIndex: 'prompt_tokens',
                  key: 'prompt_tokens',
                  width: LOG_LIST_COLUMN_WIDTHS.promptTokens,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder: sortOrderForColumn(sort, 'prompt_tokens'),
                  render: (value) => value || '',
                },
                {
                  title: t('log.table.completion_tokens'),
                  dataIndex: 'completion_tokens',
                  key: 'completion_tokens',
                  width: LOG_LIST_COLUMN_WIDTHS.completionTokens,
                  sorter: true,
                  sortDirections: ['ascend', 'descend'],
                  sortOrder: sortOrderForColumn(sort, 'completion_tokens'),
                  render: (value) => value || '',
                },
                {
                  title: t('log.table.cache_tokens'),
                  key: 'cacheQuantity',
                  width: LOG_LIST_COLUMN_WIDTHS.cacheTokens,
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
                  sortOrder: sortOrderForColumn(sort, 'chargeAmount'),
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
                  total={totalCount}
                  pageSize={pageSize}
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
