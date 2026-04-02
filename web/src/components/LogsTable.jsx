import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Dropdown,
  Form,
  Label,
  Pagination,
  Popup,
  Table,
} from 'semantic-ui-react';
import {
  API,
  copy,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../helpers';
import { useTranslation } from 'react-i18next';

import { ITEMS_PER_PAGE } from '../constants';
import {
  renderColorLabel,
  renderQuota,
  YYC_SYMBOL,
} from '../helpers/render';
import { Link, useLocation, useNavigate } from 'react-router-dom';

function renderTimestamp(timestamp, trace_id) {
  return (
    <code
      onClick={async (e) => {
        e.stopPropagation();
        if (await copy(trace_id)) {
          showSuccess(`已复制 Trace ID：${trace_id}`);
        } else {
          showWarning(`Trace ID 复制失败：${trace_id}`);
        }
      }}
      className='router-row-clickable'
    >
      {timestamp2string(timestamp)}
    </code>
  );
}

function renderType(type) {
  switch (type) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          充值
        </Label>
      );
    case 2:
      return (
        <Label basic color='olive' className='router-tag'>
          消费
        </Label>
      );
    case 3:
      return (
        <Label basic color='orange' className='router-tag'>
          管理
        </Label>
      );
    case 4:
      return (
        <Label basic color='purple' className='router-tag'>
          系统
        </Label>
      );
    case 5:
      return (
        <Label basic color='violet' className='router-tag'>
          测试
        </Label>
      );
    default:
      return (
        <Label basic color='black' className='router-tag'>
          未知
        </Label>
      );
  }
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
        <Label
          basic
          className='router-tag'
          color={getColorByElapsedTime(log.elapsed_time)}
        >
          {log.elapsed_time} ms
        </Label>
      )}
      {log.is_stream && (
        <>
          <Label className='router-tag' color='pink'>
            Stream
          </Label>
        </>
      )}
      {log.system_prompt_reset && (
        <>
          <Label basic className='router-tag' color='red'>
            System Prompt Reset
          </Label>
        </>
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

function normalizeLogEntry(log) {
  return {
    ...(log || {}),
    quota: Number(log?.yyc_amount ?? log?.quota ?? 0),
    user_daily_quota: Number(log?.yyc_user_daily ?? log?.user_daily_quota ?? 0),
    user_emergency_quota: Number(log?.yyc_user_emergency ?? log?.user_emergency_quota ?? 0),
  };
}

function buildDisplayCurrencyIndex(rows) {
  const next = {
    YYC: {
      code: 'YYC',
      symbol: YYC_SYMBOL,
      minor_unit: 0,
      yyc_per_unit: 1,
    },
  };
  (Array.isArray(rows) ? rows : [])
    .filter((item) => Number(item?.status || 0) === 1)
    .forEach((item) => {
      const code = (item?.code || '').toString().trim().toUpperCase();
      if (!code) {
        return;
      }
      next[code] = {
        ...item,
        code,
      };
    });
  return next;
}

function formatLogQuotaAmount(amount, fractionDigits = 6) {
  const normalizedAmount = Number(amount || 0);
  if (!Number.isFinite(normalizedAmount)) {
    return '-';
  }
  return normalizedAmount.toFixed(fractionDigits);
}

function renderLogQuotaValue(quota, displayUnit, currencyIndex) {
  const yycValue = Number(quota || 0);
  if (!Number.isFinite(yycValue)) {
    return '-';
  }
  const targetCurrency = currencyIndex?.[displayUnit] || currencyIndex?.YYC;
  const rate = Number(targetCurrency?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatLogQuotaAmount(yycValue / rate, 6);
}

function toDatetimeLocalValue(value) {
  const raw = (value || '').toString().trim();
  if (raw === '') {
    return '';
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

function formatFilterDisplayValue(value) {
  return (value || '').toString().trim().replace('T', ' ');
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
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [logType, setLogType] = useState(0);
  const [filterOptions, setFilterOptions] = useState({
    tokenNames: [],
    modelNames: [],
    usernames: [],
    channels: [],
    groups: [],
  });
  const [inputs, setInputs] = useState({
    username: '',
    token_name: '',
    model_name: '',
    start_timestamp: '',
    end_timestamp: '',
    channel: '',
    group_id: '',
  });
  const {
    username,
    token_name,
    model_name,
    start_timestamp,
    end_timestamp,
    channel,
    group_id,
  } = inputs;
  const [activeFilterKeys, setActiveFilterKeys] = useState([]);
  const [addFilterPopupOpen, setAddFilterPopupOpen] = useState(false);
  const [draftFilterKey, setDraftFilterKey] = useState('');
  const [draftFilterInputs, setDraftFilterInputs] = useState({
    value: '',
    start_timestamp: '',
    end_timestamp: '',
  });
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(buildDisplayCurrencyIndex([]));

  const LOG_OPTIONS = [
    { key: '0', text: t('log.type.all'), value: 0 },
    { key: '1', text: t('log.type.topup'), value: 1 },
    { key: '2', text: t('log.type.usage'), value: 2 },
    { key: '3', text: t('log.type.admin'), value: 3 },
    { key: '4', text: t('log.type.system'), value: 4 },
    { key: '5', text: t('log.type.test'), value: 5 },
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
        type: filterOptions.tokenNames.length > 0 ? 'select' : 'text',
        options: filterOptions.tokenNames.map((item) => ({
          key: item,
          text: item,
          value: item,
        })),
      },
      {
        key: 'model_name',
        label: t('log.table.model_name'),
        placeholder: t('log.table.model_name_placeholder'),
        type: filterOptions.modelNames.length > 0 ? 'select' : 'text',
        options: filterOptions.modelNames.map((item) => ({
          key: item,
          text: item,
          value: item,
        })),
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
          type: filterOptions.usernames.length > 0 ? 'select' : 'text',
          options: filterOptions.usernames.map((item) => ({
            key: item,
            text: item,
            value: item,
          })),
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

  const displayUnitOptions = useMemo(() => {
    const items = [
      {
        value: 'YYC',
        label: YYC_SYMBOL,
      },
    ];
    Object.values(currencyIndex)
      .filter((item) => item && item.code && item.code !== 'YYC')
      .sort((a, b) => `${a.code}`.localeCompare(`${b.code}`))
      .forEach((item) => {
        const symbol = (item?.symbol || '').toString().trim();
        items.push({
          value: item.code,
          label: symbol || item.code,
        });
      });
    return items;
  }, [currencyIndex]);

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
      setDraftFilterKey(filterKey);
      setAddFilterPopupOpen(true);
    },
    [conditionalFilterConfig, inputs]
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

  const loadFilterOptions = useCallback(async () => {
    const url = isAdminScope ? '/api/v1/admin/log/options' : '/api/v1/public/log/options';
    const res = await API.get(url);
    const { success, message, data } = res.data || {};
    if (!success) {
      showError(message || t('log.messages.load_failed'));
      return;
    }
    setFilterOptions({
      tokenNames: Array.isArray(data?.token_names) ? data.token_names : [],
      modelNames: Array.isArray(data?.model_names) ? data.model_names : [],
      usernames: Array.isArray(data?.usernames) ? data.usernames : [],
      channels: Array.isArray(data?.channels) ? data.channels : [],
      groups: Array.isArray(data?.groups) ? data.groups : [],
    });
  }, [isAdminScope, t]);

  const loadDisplayUnits = useCallback(async () => {
    if (!isAdminScope) {
      return;
    }
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('log.messages.load_failed'));
        return;
      }
      const next = buildDisplayCurrencyIndex(Array.isArray(data) ? data : []);
      setCurrencyIndex(next);
      setDisplayUnit((current) => {
        const normalizedCurrent = (current || '').toString().trim().toUpperCase();
        if (normalizedCurrent && next[normalizedCurrent]) {
          return normalizedCurrent;
        }
        if (next.USD) {
          return 'USD';
        }
        const fallbackUnit = Object.keys(next)
          .filter((code) => code)
          .sort((a, b) => a.localeCompare(b))[0];
        return fallbackUnit || 'YYC';
      });
    } catch (error) {
      showError(error?.message || error);
    }
  }, [isAdminScope, t]);

  const showUserTokenQuota = () => {
    const effectiveLogType = activeFilterKeys.includes('log_type') ? logType : 0;
    return effectiveLogType !== 5;
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
          setLogs((prev) => {
            let next = [...prev];
            next.splice((normalizedPage - 1) * ITEMS_PER_PAGE, normalizedRows.length, ...normalizedRows);
            return next;
          });
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

  useEffect(() => {
    refresh().then();
  }, [refresh]);

  useEffect(() => {
    loadFilterOptions().then();
  }, [loadFilterOptions]);

  useEffect(() => {
    if (!isAdminScope) {
      return;
    }
    loadDisplayUnits().then();
  }, [isAdminScope, loadDisplayUnits]);

  useEffect(() => {
    setActivePage(1);
  }, [searchKeyword, activeFilterKeys, username, token_name, model_name, channel, group_id, start_timestamp, end_timestamp]);

  const sortLog = (key) => {
    if (logs.length === 0) return;
    setLoading(true);
    let sortedLogs = [...logs];
    if (typeof sortedLogs[0][key] === 'string') {
      sortedLogs.sort((a, b) => {
        return ('' + a[key]).localeCompare(b[key]);
      });
    } else {
      sortedLogs.sort((a, b) => {
        if (a[key] === b[key]) return 0;
        if (a[key] > b[key]) return -1;
        if (a[key] < b[key]) return 1;
        return 0;
      });
    }
    if (sortedLogs[0].id === logs[0].id) {
      sortedLogs.reverse();
    }
    setLogs(sortedLogs);
    setLoading(false);
  };

  const filteredLogs = useMemo(() => {
    const keyword = (searchKeyword || '').toString().trim().toLowerCase();
    if (keyword === '') {
      return logs;
    }
    return logs.filter((log) => {
      const haystacks = [
        log?.content,
        log?.model_name,
        log?.token_name,
        log?.username,
        log?.channel_name,
        log?.channel,
        log?.group_name,
        log?.group_id,
        log?.trace_id,
      ]
        .map((item) => (item || '').toString().toLowerCase())
        .filter((item) => item !== '');
      return haystacks.some((item) => item.includes(keyword));
    });
  }, [logs, searchKeyword]);

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
    ? showUserTokenQuota()
      ? 10
      : 5
    : showUserTokenQuota()
      ? 7
      : 3;

  return (
    <>
      <Form>
        <div className='router-toolbar router-log-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start router-log-toolbar-start'>
            <Popup
              open={addFilterPopupOpen}
              on='click'
              position='bottom left'
              onClose={closeFilterDraft}
              trigger={
                <Button
                  type='button'
                  className='router-section-button'
                  disabled={availableConditionalFilterOptions.length === 0}
                  onClick={() => setAddFilterPopupOpen(true)}
                >
                  {t('log.filters.add')}
                </Button>
              }
              content={
                <div className='router-log-filter-picker'>
                  <div className='router-log-filter-picker-options'>
                    {availableConditionalFilterOptions.map((item) => (
                      <Button
                        key={item.value}
                        type='button'
                        size='mini'
                        className='router-inline-button'
                        primary={draftFilterKey === item.value}
                        onClick={() => openFilterDraft(item.value)}
                      >
                        {item.text}
                      </Button>
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
                        <Dropdown
                          className='router-section-dropdown router-log-filter-select'
                          fluid
                          search
                          selection
                          clearable
                          options={
                            conditionalFilterConfig.find((item) => item.key === draftFilterKey)
                              ?.options || []
                          }
                          value={draftFilterInputs.value}
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
                      <div className='router-log-filter-editor-actions'>
                        <Button
                          type='button'
                          size='mini'
                          className='router-inline-button'
                          onClick={closeFilterDraft}
                        >
                          {t('common.cancel')}
                        </Button>
                        <Button
                          type='button'
                          size='mini'
                          className='router-inline-button'
                          primary
                          onClick={applyFilterDraft}
                        >
                          {t('common.confirm')}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              }
            />
          </div>
          <div className='router-toolbar-end router-log-query-wrap'>
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
            <Button
              type='button'
              className='router-section-button router-log-query-button'
              onClick={refresh}
              loading={loading}
            >
              {t('log.buttons.submit')}
            </Button>
          </div>
        </div>
      </Form>
      <Table basic={'very'} compact className='router-list-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortLog('created_time');
              }}
              width={3}
            >
              {t('log.table.time')}
            </Table.HeaderCell>
            {isAdminScope && (
              <Table.HeaderCell
                className='router-sortable-header'
                onClick={() => {
                  sortLog('channel');
                }}
                width={2}
              >
                {t('log.table.channel')}
              </Table.HeaderCell>
            )}
            {isAdminScope && (
              <Table.HeaderCell
                className='router-sortable-header'
                onClick={() => {
                  sortLog('group_id');
                }}
                width={1}
              >
                {t('log.table.group')}
              </Table.HeaderCell>
            )}
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortLog('type');
              }}
              width={1}
            >
              {t('log.table.type')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortLog('model_name');
              }}
              width={2}
            >
              {t('log.table.model')}
            </Table.HeaderCell>
            {showUserTokenQuota() && (
              <>
                {isAdminScope && (
                  <Table.HeaderCell
                    className='router-sortable-header'
                    onClick={() => {
                      sortLog('username');
                    }}
                    width={2}
                  >
                    {t('log.table.username')}
                  </Table.HeaderCell>
                )}
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('token_name');
                  }}
                  width={1}
                >
                  {t('log.table.token_name')}
                </Table.HeaderCell>
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('prompt_tokens');
                  }}
                  width={1}
                >
                  {t('log.table.prompt_tokens')}
                </Table.HeaderCell>
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('completion_tokens');
                  }}
                  width={1}
                >
                  {t('log.table.completion_tokens')}
                </Table.HeaderCell>
                <Table.HeaderCell
                  className={isAdminScope ? 'router-redemption-face-value-header' : 'router-sortable-header'}
                  width={1}
                >
                  {isAdminScope ? (
                    <div className='router-table-header-with-control'>
                      <span
                        className='router-sortable-header'
                        onClick={() => {
                          sortLog('quota');
                        }}
                      >
                        {t('log.table.quota')}
                      </span>
                      <select
                        className='router-table-header-select'
                        value={displayUnit}
                        onClick={(e) => {
                          e.stopPropagation();
                        }}
                        onChange={(e) => {
                          setDisplayUnit(e.target.value);
                        }}
                      >
                        {displayUnitOptions.map((item) => (
                          <option key={item.value} value={item.value}>
                            {item.label}
                          </option>
                        ))}
                      </select>
                    </div>
                  ) : (
                    <span
                      onClick={() => {
                        sortLog('quota');
                      }}
                    >
                      {t('log.table.quota')}
                    </span>
                  )}
                </Table.HeaderCell>
              </>
            )}
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {filteredLogs
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE
            )
            .map((log, idx) => {
              if (log.deleted) return null;
              return (
                <Table.Row
                  key={log.id || idx}
                  className='router-row-clickable'
                  onClick={() =>
                    log.id
                      ? navigate(
                          `${detailBasePath}/${log.id}${location.search || ''}`
                        )
                      : undefined
                  }
                >
                  <Table.Cell>
                    {renderTimestamp(log.created_at, log.trace_id)}
                  </Table.Cell>
                  {isAdminScope && (
                    <Table.Cell>
                      {log.channel ? (
                        <Label
                          basic
                          className='router-tag'
                          as={Link}
                          to={`/channel/detail/${log.channel}`}
                          state={{ from: currentPagePath }}
                          onClick={(e) => e.stopPropagation()}
                        >
                          {getLogChannelLabel(log)}
                        </Label>
                      ) : (
                        ''
                      )}
                    </Table.Cell>
                  )}
                  {isAdminScope && (
                    <Table.Cell>
                      {log.group_id ? (
                        <Label
                          basic
                          className='router-tag'
                          as={Link}
                          to={`/admin/group/detail/${log.group_id}`}
                          state={{ from: currentPagePath }}
                          onClick={(e) => e.stopPropagation()}
                        >
                          {log.group_name || log.group_id}
                        </Label>
                      ) : (
                        '-'
                      )}
                    </Table.Cell>
                  )}
                  <Table.Cell>{renderType(log.type)}</Table.Cell>
                  <Table.Cell>
                    {log.model_name ? renderColorLabel(log.model_name) : ''}
                  </Table.Cell>
                  {showUserTokenQuota() && (
                    <>
                      {isAdminScope && (
                        <Table.Cell>
                          {log.username ? (
                            <Label
                              basic
                              className='router-tag'
                              as={Link}
                              to={`/user/detail/${log.user_id}`}
                              onClick={(e) => e.stopPropagation()}
                            >
                              {log.username}
                            </Label>
                          ) : (
                            ''
                          )}
                        </Table.Cell>
                      )}
                      <Table.Cell>
                        {log.token_name ? renderColorLabel(log.token_name) : ''}
                      </Table.Cell>

                      <Table.Cell>
                        {log.prompt_tokens ? log.prompt_tokens : ''}
                      </Table.Cell>
                      <Table.Cell>
                        {log.completion_tokens ? log.completion_tokens : ''}
                      </Table.Cell>
                      <Table.Cell>
                        {isAdminScope
                          ? renderLogQuotaValue(log.quota, displayUnit, currencyIndex)
                          : log.quota
                            ? renderQuota(log.quota, t, 6)
                            : ''}
                      </Table.Cell>
                    </>
                  )}

                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan={tableColSpan}>
              <div className='router-toolbar'>
                <Pagination
                  className='router-page-pagination'
                  activePage={activePage}
                  onPageChange={onPaginationChange}
                  siblingRange={1}
                  totalPages={totalPages}
                />
              </div>
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default LogsTable;
