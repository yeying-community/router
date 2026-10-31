import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  API,
  downloadTextAsFile,
  isRoot,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
  hasLoadedPagedRows,
  writePagedRows,
  withCardLabels,
} from '../helpers';
import { useTranslation } from 'react-i18next';
import useUrlState, { parsePageParam, parseListPageSize } from '../hooks/useUrlState';
import UnitDropdown from './UnitDropdown';
import UserSectionTabs from './UserSectionTabs';
import { buildLogDrilldownPath } from './LogsTable.helpers';

import { ITEMS_PER_PAGE } from '../constants';
import { resolveListEffectPage } from '../helpers/listState.mjs';
import {
  USER_LIST_COLUMN_WIDTHS,
  USER_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import {
  formatCompactNumber,
  formatIdentifierPreview,
  renderText,
} from '../helpers/render';
import {
  buildDisplayUnitOptions,
  buildPublicDisplayCurrencyIndex,
  loadPublicDisplayCurrencyCatalog,
  resolvePreferredDisplayCurrency,
  chargeAmountToBillingInputValue,
} from '../helpers/billing';
import useBatchRowActions from '../hooks/useBatchRowActions';
import {
  AppButton,
  AppEmpty,
  AppErrorState,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppInput,
  AppModal,
  AppPagination,
  AppPopconfirm,
  AppSelect,
  AppSpin,
  AppTable,
  AppTableActionButton,
  AppTag,
  AppTooltip,
} from '../router-ui';

function renderRole(role, t) {
  switch (role) {
    case 1:
      return (
        <AppTag className='router-tag'>
          {t('user.table.role_types.normal')}
        </AppTag>
      );
    case 10:
      return (
        <AppTag color='yellow' className='router-tag'>
          {t('user.table.role_types.admin')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='red' className='router-tag'>
          {t('user.table.role_types.unknown')}
        </AppTag>
      );
  }
}

const formatFullNumber = (value) => {
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) {
    return '0';
  }
  return numericValue.toLocaleString();
};

const formatUserBalanceValue = (value) => {
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) {
    return '0.00';
  }
  return numericValue.toFixed(2);
};

const formatPlanNumber = (value) => {
  const numeric = Number(value || 0);
  if (!Number.isFinite(numeric)) {
    return '0';
  }
  if (Math.abs(numeric - Math.round(numeric)) < 0.000001) {
    return `${Math.round(numeric)}`;
  }
  return numeric.toFixed(6).replace(/\.?0+$/, '');
};

const toTopupPlanOptions = (rows, t) =>
  (Array.isArray(rows) ? rows : [])
    .filter((item) => Boolean(item?.enabled))
    .map((item) => {
      const id = (item?.id || '').toString().trim();
      const amount = formatPlanNumber(item?.amount ?? item?.sale_price ?? 0);
      const amountCurrency = (item?.amount_currency || item?.sale_currency || '').toString().trim().toUpperCase();
      const quotaAmount = formatPlanNumber(item?.quota_amount || 0);
      const quotaCurrency = (item?.quota_currency || '').toString().trim().toUpperCase();
      const validityDays = Number(item?.validity_days || item?.duration_days || 0);
      const labelParts = [`${amount} ${amountCurrency}`, `${quotaAmount} ${quotaCurrency}`];
      if (validityDays > 0) {
        labelParts.push(`${validityDays}${t('common.day')}`);
      } else {
        labelParts.push(t('common.never'));
      }
      return {
        key: id,
        value: id,
        text: labelParts.join(' / '),
      };
    })
    .filter((option) => option.value);

const loadAllEntitlementProducts = async (kind) => {
  const items = [];
  let page = 1;
  while (page <= 50) {
    const res = await API.get('/api/v1/admin/entitlement/products', {
      params: {
        kind,
        page,
        page_size: 100,
      },
    });
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || '');
    }
    const pageItems = Array.isArray(data?.items) ? data.items : [];
    items.push(...pageItems);
    const total = Number(data?.total || pageItems.length || 0);
    if (pageItems.length === 0 || items.length >= total || pageItems.length < 100) {
      break;
    }
    page += 1;
  }
  return items;
};

const compareTextValue = (left, right) =>
  String(left || '').localeCompare(String(right || ''));

const compareNumberValue = (left, right) =>
  Number(left || 0) - Number(right || 0);

const UsersTable = ({ embedded = false }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const isAdminScope = location.pathname.startsWith('/admin/');
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  // page/pageSize 也入 URL:列表↔详情往返(from=pathname+search)后回到原页原尺寸,
  // 而非退回第 1 页。仅挂载时读一次 URL 播种,之后以组件内 state 为准,URL 由
  // 翻页/改尺寸/换筛选主动镜像回写(见 onPaginationChange 与筛选 handler)。
  const initialListQuery = useMemo(() => {
    const params = new URLSearchParams(location.search);
    return {
      page: parsePageParam(params.get('page')),
      pageSize: parseListPageSize(params.get('page_size')),
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const [activePage, setActivePage] = useState(initialListQuery.page);
  const [pageSize, setPageSize] = useState(initialListQuery.pageSize);
  const pageSizeRef = useRef(initialListQuery.pageSize);
  pageSizeRef.current = pageSize;
  const [totalCount, setTotalCount] = useState(0);
  const [isSearchMode, setIsSearchMode] = useState(false);
  const [
    { status: statusFilter, role: roleFilter, keyword: searchKeyword, group: groupFilter, groupName: groupFilterName },
    patchQuery,
  ] = useUrlState({
    status: { param: 'status', default: 'all' },
    role: { param: 'role', default: 'all' },
    keyword: { param: 'q', default: '' },
    group: { param: 'group', default: '' },
    groupName: { param: 'group_name', default: '' },
    page: { param: 'page', default: 1, parse: parsePageParam },
    pageSize: {
      param: 'page_size',
      default: ITEMS_PER_PAGE,
      parse: parseListPageSize,
    },
  });
  const setSearchKeyword = useCallback(
    (value) => patchQuery({ keyword: (value || '').toString() }),
    [patchQuery],
  );
  const [searching, setSearching] = useState(false);
  const [focusLabel, setFocusLabel] = useState('');
  const [focusTotal, setFocusTotal] = useState(0);
  const [isFocusMode, setIsFocusMode] = useState(false);
  const [tableSorter, setTableSorter] = useState({
    columnKey: 'created_at',
    order: 'descend',
  });
  const previousSearchKeywordRef = useRef(searchKeyword);
  const [currencyIndex, setCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([]),
  );
  const [balanceUnit, setBalanceUnit] = useState(() =>
    resolvePreferredDisplayCurrency(buildPublicDisplayCurrencyIndex([]), 'USD'),
  );
  const batchActions = useBatchRowActions();
  const {
    isSelecting: batchSelectionMode,
    selectedRowKeys,
    setSelectedRowKeys,
  } = batchActions;
  const [topupPlanOptions, setTopupPlanOptions] = useState([]);
  const [topupPlanOptionsLoading, setTopupPlanOptionsLoading] = useState(false);
  const [batchTopupOpen, setBatchTopupOpen] = useState(false);
  const [batchTopupForm, setBatchTopupForm] = useState({
    plan_id: '',
  });
  const [batchTopupSubmitting, setBatchTopupSubmitting] = useState(false);
  const [batchTopupResult, setBatchTopupResult] = useState(null);
  const [batchManageSubmitting, setBatchManageSubmitting] = useState(false);

  const loadUsers = useCallback(
    async (page, { status = 'all', role = 'all', group = '' } = {}) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      try {
        const params = new URLSearchParams();
        params.set('page', String(normalizedPage));
        const size = pageSizeRef.current;
        params.set('page_size', String(size));
        const normalizedStatus = (status || 'all').toString();
        const normalizedRole = (role || 'all').toString();
        const normalizedGroup = (group || '').toString().trim();
        if (normalizedStatus !== 'all') params.set('status', normalizedStatus);
        if (normalizedRole !== 'all') params.set('role', normalizedRole);
        if (normalizedGroup !== '') params.set('group', normalizedGroup);
        const res = await API.get(`/api/v1/admin/user/?${params.toString()}`);
        const { success, message, data, meta } = res.data;
        if (success) {
          setLoadError(false);
          setIsSearchMode(false);
          setTotalCount(Number(meta?.total || data?.length || 0));
          if (normalizedPage === 1) {
            setUsers(data);
          } else {
            setUsers((prev) => writePagedRows(prev, normalizedPage, size, data));
          }
        } else {
          if (normalizedPage === 1) setLoadError(true);
          showError(message);
        }
      } catch (error) {
        if (normalizedPage === 1) setLoadError(true);
        showError(error?.message || error);
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  const loadUsersByIDs = useCallback(async (userIDs, label = '', totalHint = 0) => {
    const normalizedIDs = [...new Set(
      (Array.isArray(userIDs) ? userIDs : [])
        .map((item) => (item || '').toString().trim())
        .filter(Boolean),
    )];
    if (normalizedIDs.length === 0) {
      setFocusLabel('');
      setFocusTotal(0);
      setIsSearchMode(false);
      setTotalCount(0);
      setUsers([]);
      setActivePage(1);
      setLoading(false);
      return;
    }
    const responses = await Promise.all(
      normalizedIDs.map(async (userID) => {
        try {
          const res = await API.get(`/api/v1/admin/user/${encodeURIComponent(userID)}`);
          const { success, data } = res.data || {};
          return success && data ? data : null;
        } catch (error) {
          return null;
        }
      }),
    );
    const matchedUsers = responses.filter(Boolean);
    setFocusLabel(label);
    setFocusTotal(Number(totalHint) > 0 ? Number(totalHint) : matchedUsers.length);
    setIsFocusMode(true);
    setSearchKeyword('');
    setIsSearchMode(true);
    setTotalCount(matchedUsers.length);
    setUsers(matchedUsers);
    setActivePage(1);
    setLoading(false);
  }, []);

  const locationSearch = location.search || '';
  const focusParams = useMemo(() => {
    const params = new URLSearchParams(locationSearch);
    return {
      ids: (params.get('focus_ids') || '')
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean),
      name: (params.get('focus_name') || '').trim(),
      total: Number(params.get('focus_total') || 0),
    };
  }, [locationSearch]);
  const focusKey = focusParams.ids.join(',');
  const userListRequestKey = useMemo(
    () =>
      JSON.stringify({
        focusKey,
        focusName: focusParams.name,
        focusTotal: focusParams.total,
        status: statusFilter,
        role: roleFilter,
        group: groupFilter,
      }),
    [
      focusKey,
      focusParams.name,
      focusParams.total,
      groupFilter,
      roleFilter,
      statusFilter,
    ],
  );
  const previousUserListRequestKeyRef = useRef(userListRequestKey);

  const refresh = async () => {
    setLoading(true);
    if (focusParams.ids.length > 0) {
      await loadUsersByIDs(focusParams.ids, focusParams.name, focusParams.total);
      return;
    }
    setIsFocusMode(false);
    setFocusTotal(0);
    await loadUsers(activePage, { status: statusFilter, role: roleFilter, group: groupFilter });
  };

  const loadTopupPlanOptions = useCallback(async () => {
    if (topupPlanOptions.length > 0) {
      return;
    }
    setTopupPlanOptionsLoading(true);
    try {
      const items = await loadAllEntitlementProducts('balance');
      setTopupPlanOptions(toTopupPlanOptions(items, t));
    } catch (error) {
      showError(
        error?.message ||
          t('user.messages.load_entitlement_products_failed'),
      );
    } finally {
      setTopupPlanOptionsLoading(false);
    }
  }, [topupPlanOptions.length, t]);

  useEffect(() => {
    if (!batchTopupOpen) {
      return;
    }
    loadTopupPlanOptions().then();
  }, [batchTopupOpen, loadTopupPlanOptions]);

  const onPaginationChange = (e, { activePage, pageSize: nextPageSize }) => {
    (async () => {
      const size = Number(nextPageSize) > 0 ? Number(nextPageSize) : pageSize;
      if (size !== pageSize) {
        pageSizeRef.current = size;
        setPageSize(size);
        setActivePage(1);
        patchQuery({ pageSize: size, page: 1 });
        if (!isSearchMode) {
          // 每页条数变了,按旧尺寸建立的行缓存已失效,重建
          setUsers([]);
          await loadUsers(1, { status: statusFilter, role: roleFilter, group: groupFilter });
        }
        return;
      }
      const nextPage = Number(activePage) > 0 ? Number(activePage) : 1;
      const hasLoadedPageRows = hasLoadedPagedRows(users, nextPage, size);
      if (!isSearchMode && !hasLoadedPageRows) {
        await loadUsers(nextPage, { status: statusFilter, role: roleFilter, group: groupFilter });
      }
      setActivePage(nextPage);
      patchQuery({ page: nextPage });
    })();
  };

  useEffect(() => {
    setLoading(true);
    if (focusParams.ids.length > 0) {
      loadUsersByIDs(focusParams.ids, focusParams.name, focusParams.total).catch(
        (reason) => {
          setLoadError(true);
          showError(reason?.message || reason);
          setLoading(false);
        },
      );
      return;
    }
    setFocusLabel('');
    setFocusTotal(0);
    setIsFocusMode(false);
    // StrictMode 会重放挂载 effect；不能用 didMount 布尔值判断首次加载，
    // 否则第二次挂载执行会被误判成筛选变化并把 URL 页码重置为 1。
    const startPage = resolveListEffectPage(
      previousUserListRequestKeyRef.current,
      userListRequestKey,
      initialListQuery.page,
    );
    previousUserListRequestKeyRef.current = userListRequestKey;
    setActivePage(startPage);
    loadUsers(startPage, { status: statusFilter, role: roleFilter, group: groupFilter })
      .then()
      .catch((reason) => {
        setLoadError(true);
        showError(reason);
        setLoading(false);
      });
  }, [
    loadUsers,
    loadUsersByIDs,
    focusKey,
    focusParams.name,
    focusParams.total,
    statusFilter,
    roleFilter,
    groupFilter,
    initialListQuery,
    userListRequestKey,
  ]);

  useEffect(() => {
    let disposed = false;
    loadPublicDisplayCurrencyCatalog().then(({ currencyIndex: nextIndex, defaultCurrency }) => {
      if (disposed) {
        return;
      }
      setCurrencyIndex(nextIndex);
      setBalanceUnit((current) =>
        resolvePreferredDisplayCurrency(
          nextIndex,
          current || defaultCurrency || 'USD',
        ),
      );
    });
    return () => {
      disposed = true;
    };
  }, []);

  const manageUser = async (targetUser, action) => {
    const targetID = (targetUser?.id || '').toString();
    const targetUsername = (targetUser?.username || '').toString();
    const isTargetUser = (item) =>
      (Boolean(targetID) && item?.id === targetID) ||
      (Boolean(targetUsername) && item?.username === targetUsername);
    const res = await API.post('/api/v1/admin/user/manage', {
      username: targetUsername,
      action,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('user.messages.operation_success'));
      let user = res.data.data;
      if (action === 'delete') {
        setUsers((currentUsers) =>
          currentUsers.map((item) =>
            isTargetUser(item) ? { ...item, deleted: true } : item,
          ),
        );
        setTotalCount((prev) => Math.max(prev - 1, 0));
        setSelectedRowKeys((prev) => prev.filter((key) => key !== targetID));
      } else {
        setUsers((currentUsers) =>
          currentUsers.map((item) =>
            isTargetUser(item)
              ? { ...item, status: user.status, role: user.role }
              : item,
          ),
        );
      }
      return user;
    }
    showError(message);
    return null;
  };

  const renderStatus = (status) => {
    switch (status) {
      case 1:
        return (
          <AppTag className='router-tag'>
            {t('user.table.status_types.activated')}
          </AppTag>
        );
      case 2:
        return (
          <AppTag color='red' className='router-tag'>
            {t('user.table.status_types.banned')}
          </AppTag>
        );
      default:
        return (
          <AppTag color='grey' className='router-tag'>
            {t('user.table.status_types.unknown')}
          </AppTag>
        );
    }
  };

  const openBatchTopupModal = useCallback(() => {
    if (selectedRowKeys.length === 0) {
      showInfo(t('user.batch.topup_select_required'));
      return;
    }
    setBatchTopupResult(null);
    setBatchTopupOpen(true);
  }, [selectedRowKeys.length, t]);

  const enterBatchSelectionMode = useCallback(() => {
    batchActions.enter();
  }, [batchActions]);

  const cancelBatchSelectionMode = useCallback(() => {
    if (batchTopupSubmitting || batchManageSubmitting) {
      return;
    }
    batchActions.exit();
    setBatchTopupResult(null);
  }, [batchActions, batchTopupSubmitting, batchManageSubmitting]);

  const closeBatchTopupModal = useCallback(() => {
    if (batchTopupSubmitting) {
      return;
    }
    setBatchTopupOpen(false);
    setBatchTopupResult(null);
  }, [batchTopupSubmitting]);

  const submitBatchTopup = useCallback(async () => {
    const userIDs = selectedRowKeys
      .map((item) => (item || '').toString().trim())
      .filter(Boolean);
    if (userIDs.length === 0) {
      showInfo(t('user.batch.topup_select_required'));
      return;
    }
    const normalizedPlanID = (batchTopupForm.plan_id || '').toString().trim();
    if (normalizedPlanID === '') {
      showInfo(t('user.detail.assign.topup_plan_required'));
      return;
    }
    setBatchTopupSubmitting(true);
    try {
      const res = await API.post('/api/v1/admin/user/batch/topup/grant', {
        user_ids: userIDs,
        plan_id: normalizedPlanID,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.operation_failed'));
        return;
      }
      const result = {
        total: Number(data?.total || userIDs.length),
        succeeded: Number(data?.succeeded || 0),
        failed: Number(data?.failed || 0),
        items: Array.isArray(data?.items) ? data.items : [],
      };
      setBatchTopupResult(result);
      showSuccess(
        t('user.batch.topup_done', {
          success: result.succeeded,
          failed: result.failed,
        }),
      );
      const failedIDs = result.items
        .filter((item) => !item?.success)
        .map((item) => (item?.user_id || '').toString().trim())
        .filter(Boolean);
      setSelectedRowKeys(failedIDs);
      if (result.failed === 0) {
        setBatchTopupForm({ plan_id: '' });
        setBatchTopupOpen(false);
        batchActions.exit();
      }
      await refresh();
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setBatchTopupSubmitting(false);
    }
  }, [batchTopupForm.plan_id, refresh, selectedRowKeys, t]);

  const runBatchManage = useCallback(
    async (action) => {
      const idSet = new Set(
        selectedRowKeys
          .map((item) => (item || '').toString().trim())
          .filter(Boolean),
      );
      if (idSet.size === 0) {
        showInfo(t('user.batch.topup_select_required'));
        return;
      }
      // selectedRowKeys 保存的是用户 id，manage 接口需要 username，先按 id 解析出实时用户对象。
      const targets = users.filter(
        (user) => idSet.has((user?.id || '').toString()) && !user?.deleted,
      );
      // 跳过状态已符合的行（启用时跳过已启用、停用时跳过已停用）。
      const actionable = targets.filter((user) => {
        if (action === 'enable') return user.status !== 1;
        if (action === 'disable') return user.status === 1;
        return true;
      });
      if (actionable.length === 0) {
        showInfo(
          action === 'delete'
            ? t('user.batch.manage_no_deletable')
            : t('user.batch.manage_no_active_change'),
        );
        return;
      }
      setBatchManageSubmitting(true);
      let succeeded = 0;
      let failed = 0;
      const failedIDs = [];
      for (const user of actionable) {
        const username = (user?.username || '').toString();
        try {
          const res = await API.post('/api/v1/admin/user/manage', {
            username,
            action,
          });
          if (res?.data?.success) {
            succeeded += 1;
          } else {
            failed += 1;
            failedIDs.push((user?.id || '').toString());
          }
        } catch (error) {
          failed += 1;
          failedIDs.push((user?.id || '').toString());
        }
      }
      const skipped = targets.length - actionable.length;
      showSuccess(
        t('user.batch.manage_done', { success: succeeded, failed }),
      );
      if (skipped > 0) {
        showInfo(t('user.batch.manage_skipped', { skipped }));
      }
      setSelectedRowKeys(failedIDs);
      if (failed === 0) {
        batchActions.exit();
      }
      await refresh();
      setBatchManageSubmitting(false);
    },
    [batchActions, refresh, selectedRowKeys, t, users],
  );

  const searchUsers = async () => {
    setFocusLabel('');
    setFocusTotal(0);
    setIsFocusMode(false);
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadUsers(1, { status: statusFilter, role: roleFilter });
      setActivePage(1);
      patchQuery({ page: 1 });
      return;
    }
    setSearching(true);
    const res = await API.get(
      `/api/v1/admin/user/search?keyword=${searchKeyword}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      setIsSearchMode(true);
      setTotalCount(Array.isArray(data) ? data.length : 0);
      setUsers(data);
      setActivePage(1);
      patchQuery({ page: 1 });
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const handleKeywordChange = async (e, { value }) => {
    setFocusLabel('');
    setFocusTotal(0);
    setIsFocusMode(false);
    setSearchKeyword(value.trim());
  };

  const clearFocusMode = useCallback(() => {
    setFocusLabel('');
    setFocusTotal(0);
    setIsFocusMode(false);
    setSearchKeyword('');
    navigate('/admin/user');
  }, [navigate]);

  useEffect(() => {
    const keywordChanged = previousSearchKeywordRef.current !== searchKeyword;
    previousSearchKeywordRef.current = searchKeyword;
    if (!keywordChanged && searchKeyword === '') {
      // Initial list load is owned by the filter effect; only auto-run search
      // on mount when a keyword was restored from the URL. This semantic check
      // is safe when StrictMode replays the mount effect.
      return undefined;
    }
    if (isFocusMode && searchKeyword === '') {
      return undefined;
    }
    const timer = window.setTimeout(() => {
      searchUsers().catch((error) => {
        showError(error?.message || error);
      });
    }, 250);
    return () => {
      window.clearTimeout(timer);
    };
  }, [isFocusMode, searchKeyword]);

  const stopRowClick = (event) => {
    event.stopPropagation();
  };

  const visibleUserCount = users.filter((user) => !user?.deleted).length;
  const focusMatchedCount = isFocusMode
    ? Math.max(Number(focusTotal || 0), visibleUserCount)
    : 0;
  const paginationTotal = isSearchMode ? visibleUserCount : totalCount;

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

  const renderCountValue = (value) => (
    <AppTooltip title={formatFullNumber(value)}>
      <span>{formatCompactNumber(value)}</span>
    </AppTooltip>
  );

  const [exporting, setExporting] = useState(false);

  const buildExportCsvText = useCallback((exportRows) => {
    const escapeCSV = (value) => {
      const normalized = String(value ?? '');
      if (/[",\n]/.test(normalized)) {
        return `"${normalized.replace(/"/g, '""')}"`;
      }
      return normalized;
    };
    const headers = [
      'id',
      'username',
      'email',
      'display_name',
      'avatar_url',
      'wallet_identity_did',
      'wallet_address',
      'active_package_name',
      'balance_amount',
      'request_count',
      'role',
      'status',
      'created_at',
      'updated_at',
    ];
    const lines = [
      headers.join(','),
      ...exportRows.map((user) =>
        [
          user?.id,
          user?.username,
          user?.email,
          user?.display_name,
          user?.avatar_url,
          user?.wallet_identity_did,
          user?.wallet_address,
          user?.active_package_name,
          user?.balance_amount ?? 0,
          user?.request_count,
          user?.role,
          user?.status,
          user?.created_at ? timestamp2string(user.created_at) : '',
          user?.updated_at ? timestamp2string(user.updated_at) : '',
        ]
          .map(escapeCSV)
          .join(','),
      ),
    ];
    return lines.join('\n');
  }, []);

  const handleExportUsers = useCallback(async () => {
    if (exporting) return;
    setExporting(true);
    try {
      // 覆盖式分页下当前可见用户不等于筛选后全量;后端无专门的"导出"端点,
      // 故拉一次 page_size=10000 的全集,参数与列表一致(包括 status / role)。
      const params = new URLSearchParams();
      params.set('page', '1');
      params.set('page_size', '10000');
      const normalizedStatus = (statusFilter || 'all').toString();
      const normalizedRole = (roleFilter || 'all').toString();
      const normalizedGroup = (groupFilter || '').toString().trim();
      if (normalizedStatus !== 'all') params.set('status', normalizedStatus);
      if (normalizedRole !== 'all') params.set('role', normalizedRole);
      if (normalizedGroup !== '') params.set('group', normalizedGroup);
      const res = await API.get(`/api/v1/admin/user/?${params.toString()}`);
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('common.load_failed'));
        return;
      }
      const exportRows = (Array.isArray(data) ? data : []).filter(
        (user) => !user?.deleted,
      );
      if (exportRows.length === 0) {
        showError(t('user.export.empty'));
        return;
      }
      const csvText = buildExportCsvText(exportRows);
      const stamp = new Date().toISOString().replace(/[:.]/g, '-');
      const focusSuffix = focusLabel ? `-${focusLabel}` : '';
      downloadTextAsFile(csvText, `users${focusSuffix}-${stamp}.csv`);
      showSuccess(t('user.export.success', { count: exportRows.length }));
    } catch (error) {
      showError(error?.message || t('common.load_failed'));
    } finally {
      setExporting(false);
    }
  }, [
    buildExportCsvText,
    exporting,
    focusLabel,
    roleFilter,
    statusFilter,
    t,
  ]);

  const balanceUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex),
    [currencyIndex],
  );
  const selectedUserCount = selectedRowKeys.length;
  const batchTopupFailedItems = (batchTopupResult?.items || []).filter(
    (item) => !item?.success,
  );
  const userRowSelection = batchSelectionMode
    ? {
        selectedRowKeys,
        preserveSelectedRowKeys: true,
        renderCell: (_, __, ___, originNode) => (
          <span onClick={stopRowClick}>{originNode}</span>
        ),
        onChange: (nextSelectedRowKeys) => {
          setSelectedRowKeys(
            nextSelectedRowKeys
              .map((item) => (item || '').toString().trim())
              .filter(Boolean),
          );
        },
        getCheckboxProps: (record) => ({
          disabled: record?.deleted === true,
        }),
      }
    : undefined;

  return (
    <>
      <AppFilterHeader
        breadcrumbs={
          embedded
            ? undefined
            : [
                {
                  key: 'workspace',
                  label: isAdminScope
                    ? t('header.admin_workspace')
                    : t('header.user_workspace'),
                },
                { key: 'user', label: t('header.user'), active: true },
              ]
        }
        title={embedded ? undefined : t('header.user')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              color='blue'
              onClick={() => navigate('/admin/user/add')}
            >
              {t('user.buttons.add')}
            </AppButton>
            {batchSelectionMode ? (
              <>
                <AppButton
                  className='router-page-button'
                  onClick={openBatchTopupModal}
                  disabled={batchManageSubmitting}
                >
                  {t('user.batch.grant_topup_selected', {
                    count: selectedUserCount,
                  })}
                </AppButton>
                <AppPopconfirm
                  title={t('user.batch.manage_confirm_enable', {
                    count: selectedUserCount,
                  })}
                  okText={t('common.confirm')}
                  cancelText={t('common.cancel')}
                  disabled={selectedUserCount === 0 || batchManageSubmitting}
                  onConfirm={() => runBatchManage('enable')}
                >
                  <AppButton
                    className='router-page-button'
                    disabled={selectedUserCount === 0 || batchManageSubmitting}
                    loading={batchManageSubmitting}
                  >
                    {t('user.batch.enable_selected', {
                      count: selectedUserCount,
                    })}
                  </AppButton>
                </AppPopconfirm>
                <AppPopconfirm
                  title={t('user.batch.manage_confirm_disable', {
                    count: selectedUserCount,
                  })}
                  okText={t('common.confirm')}
                  cancelText={t('common.cancel')}
                  disabled={selectedUserCount === 0 || batchManageSubmitting}
                  onConfirm={() => runBatchManage('disable')}
                >
                  <AppButton
                    className='router-page-button'
                    disabled={selectedUserCount === 0 || batchManageSubmitting}
                    loading={batchManageSubmitting}
                  >
                    {t('user.batch.disable_selected', {
                      count: selectedUserCount,
                    })}
                  </AppButton>
                </AppPopconfirm>
                <AppPopconfirm
                  title={t('user.batch.manage_confirm_delete', {
                    count: selectedUserCount,
                  })}
                  okText={t('common.confirm')}
                  cancelText={t('common.cancel')}
                  disabled={selectedUserCount === 0 || batchManageSubmitting}
                  onConfirm={() => runBatchManage('delete')}
                >
                  <AppButton
                    className='router-page-button'
                    color='red'
                    disabled={selectedUserCount === 0 || batchManageSubmitting}
                    loading={batchManageSubmitting}
                  >
                    {t('user.batch.delete_selected', {
                      count: selectedUserCount,
                    })}
                  </AppButton>
                </AppPopconfirm>
                <AppButton
                  className='router-page-button'
                  onClick={cancelBatchSelectionMode}
                  disabled={batchTopupSubmitting || batchManageSubmitting}
                >
                  {t('user.batch.cancel_selection')}
                </AppButton>
              </>
            ) : (
              <AppButton
                className='router-page-button'
                onClick={enterBatchSelectionMode}
              >
                {t('user.batch.enter_selection')}
              </AppButton>
            )}
            <AppButton
              className='router-page-button'
              loading={loading}
              disabled={loading}
              onClick={refresh}
            >
              {t('user.buttons.refresh')}
            </AppButton>
            <AppButton
              className='router-page-button'
              disabled={exporting}
              loading={exporting}
              onClick={handleExportUsers}
            >
              {t('common.download')}
            </AppButton>
          </div>
        }
        query={
          <div className='router-list-toolbar-query router-list-toolbar-query-compact'>
            <div className='router-search-form-xs'>
              <AppInput
                className='router-section-input'
                icon='search'
                iconPosition='left'
                fluid
                placeholder={t('user.search')}
                value={searchKeyword}
                loading={searching}
                onChange={handleKeywordChange}
              />
            </div>
            <AppSelect
              className='router-section-select'
              value={statusFilter}
              onChange={(_, { value }) => patchQuery({ status: value, page: 1 })}
              options={[
                { value: 'all', label: t('user.filter.status_all') },
                { value: '1', label: t('user.table.status_types.activated') },
                { value: '2', label: t('user.table.status_types.banned') },
              ]}
            />
            <AppSelect
              className='router-section-select'
              value={roleFilter}
              onChange={(_, { value }) => patchQuery({ role: value, page: 1 })}
              options={[
                { value: 'all', label: t('user.filter.role_all') },
                { value: '1', label: t('user.table.role_types.normal') },
                { value: '10', label: t('user.table.role_types.admin') },
              ]}
            />
            <AppButton
              className='router-section-button'
              disabled={
                statusFilter === 'all' &&
                roleFilter === 'all' &&
                searchKeyword === ''
              }
              onClick={() =>
                patchQuery({ status: 'all', role: 'all', keyword: '', page: 1 })
              }
            >
              {t('common.clear_filters')}
            </AppButton>
            {focusLabel ? (
              <AppTag className='router-tag'>{focusLabel}</AppTag>
            ) : null}
          </div>
        }
      />

      {isAdminScope && !embedded ? <UserSectionTabs active='list' /> : null}

      {isFocusMode ? (
        <div className='router-user-focus-summary'>
          <div className='router-user-focus-summary-main'>
            <div className='router-user-focus-summary-title'>
              {focusLabel || t('user.focus.title')}
            </div>
            <div className='router-user-focus-summary-text'>
              {t('user.focus.summary', {
                count: visibleUserCount,
                total: focusMatchedCount,
              })}
            </div>
          </div>
          <AppButton
            className='router-inline-button'
            type='button'
            onClick={clearFocusMode}
          >
            {t('user.focus.clear')}
          </AppButton>
        </div>
      ) : null}

      {groupFilter && !isFocusMode ? (
        <div className='router-user-focus-summary'>
          <div className='router-user-focus-summary-main'>
            <div className='router-user-focus-summary-title'>
              {t('user.group_filter.title', {
                group: groupFilterName || groupFilter,
              })}
            </div>
            <div className='router-user-focus-summary-text'>
              {t('user.group_filter.summary', { total: totalCount })}
            </div>
          </div>
          <AppButton
            className='router-inline-button'
            type='button'
            onClick={() => patchQuery({ group: '', groupName: '', page: 1 })}
          >
            {t('user.group_filter.clear')}
          </AppButton>
        </div>
      ) : null}

      <div className='router-table-scroll-x'>
        <AppSpin spinning={loading}>
          <AppTable
            className='router-hover-table router-list-table router-table-fit-page router-user-list-table router-table-cardify'
            pagination={false}
            scroll={{ x: USER_LIST_TABLE_MIN_WIDTH }}
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
                      onClick={() =>
                        navigate('/admin/user/add', {
                          state: {
                            from: `${location.pathname}${location.search}`,
                          },
                        })
                      }
                    >
                      {t('user.buttons.add')}
                    </AppButton>
                  }
                >
                  {t('user.table.empty_cta')}
                </AppEmpty>
              ),
            }}
            rowKey={(user) => user.id}
            rowSelection={userRowSelection}
            onChange={handleTableChange}
            dataSource={users
              .slice(
              (activePage - 1) * pageSize,
              activePage * pageSize,
            )
            .filter((user) => !user?.deleted)}
          onRow={(user, idx) => ({
            className: 'router-row-clickable',
            onClick: () => {
              if (batchSelectionMode) {
                const userID = (user?.id || '').toString().trim();
                if (!userID || user?.deleted === true) {
                  return;
                }
                setSelectedRowKeys((previous) =>
                  previous.includes(userID)
                    ? previous.filter((item) => item !== userID)
                    : [...previous, userID],
                );
                return;
              }
              navigate(`/admin/user/detail/${user.id}`, {
                state: { from: `${location.pathname}${location.search}` },
              });
            },
          })}
          columns={withCardLabels([
          {
            title: t('user.table.username'),
            dataIndex: 'username',
            key: 'username',
            width: USER_LIST_COLUMN_WIDTHS.username,
            ellipsis: true,
            sorter: (a, b) => compareTextValue(a.username, b.username),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'username' ? tableSorter.order : null,
            render: (_, user) => (
              <AppTooltip
                title={
                  <div>
                    <div>{user.username}</div>
                    <div>{user.email ? user.email : t('user.no_email')}</div>
                  </div>
                }
              >
                <span>{renderText(user.username, 15)}</span>
              </AppTooltip>
            ),
          },
          {
            title: t('user.table.wallet_identity'),
            dataIndex: 'wallet_identity_did',
            key: 'wallet_identity_did',
            width: USER_LIST_COLUMN_WIDTHS.identity,
            ellipsis: true,
            render: (value) =>
              value ? (
                <AppTooltip title={value}>
                  <span className='router-monospace-value'>
                    {formatIdentifierPreview(value, 18, 8)}
                  </span>
                </AppTooltip>
              ) : (
                '-'
              ),
          },
          {
            title: (
              <div className='router-table-header-with-control'>
                <span>{t('user.table.balance')}</span>
                <UnitDropdown
                  variant='header'
                  compact
                  options={balanceUnitOptions}
                  value={balanceUnit}
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                  onChange={(_, { value }) => {
                    setBalanceUnit((value || '').toString());
                  }}
                />
              </div>
            ),
            key: 'balance',
            className: 'router-redemption-face-value-header',
            width: USER_LIST_COLUMN_WIDTHS.balance,
            render: (_, user) =>
              formatUserBalanceValue(
                chargeAmountToBillingInputValue(
                  user.balance_amount ?? 0,
                  balanceUnit,
                  currencyIndex,
                ),
              ),
          },
          {
            title: t('user.table.request_count'),
            dataIndex: 'request_count',
            key: 'request_count',
            className: 'router-table-col-status-narrow',
            width: USER_LIST_COLUMN_WIDTHS.requestCount,
            sorter: (a, b) => compareNumberValue(a.request_count, b.request_count),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'request_count'
                ? tableSorter.order
                : null,
            render: (value) => renderCountValue(value),
          },
          {
            title: t('user.table.created_at'),
            dataIndex: 'created_at',
            key: 'created_at',
            className: 'router-table-col-datetime',
            width: USER_LIST_COLUMN_WIDTHS.createdAt,
            sorter: (a, b) => compareNumberValue(a.created_at, b.created_at),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'created_at'
                ? tableSorter.order
                : null,
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: t('user.table.role_text'),
            dataIndex: 'role',
            key: 'role',
            className: 'router-table-col-status-compact',
            width: USER_LIST_COLUMN_WIDTHS.role,
            sorter: (a, b) => compareNumberValue(a.role, b.role),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'role' ? tableSorter.order : null,
            render: (value) => renderRole(value, t),
          },
          {
            title: t('user.table.status_text'),
            dataIndex: 'status',
            key: 'status',
            className: 'router-table-col-status-compact',
            width: USER_LIST_COLUMN_WIDTHS.status,
            sorter: (a, b) => compareNumberValue(a.status, b.status),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'status' ? tableSorter.order : null,
            render: (value) => renderStatus(value),
          },
          {
            title: t('user.table.actions'),
            key: 'actions',
            className: 'router-table-col-actions-icon',
            width: 112,
            render: (_, user) => {
              const isAdminUser = Number(user.role) >= 10;
              const canManageAdminUser = !isAdminUser || isRoot();
              return (
                <div
                  className='router-action-group router-table-actions-icon-compact'
                  onClick={stopRowClick}
                >
                  <AppTableActionButton
                    icon='book'
                    title={t('log.drilldown.view')}
                    onClick={() => {
                      navigate(
                        buildLogDrilldownPath('admin', {
                          username: user.username,
                        }),
                      );
                    }}
                  />
                  <AppPopconfirm
                    title={t('user.buttons.confirm_change_status')}
                    onConfirm={() => {
                      manageUser(
                        user,
                        user.status === 1 ? 'disable' : 'enable',
                      );
                    }}
                    disabled={!canManageAdminUser}
                  >
                    <span>
                      <AppTableActionButton
                        icon={user.status === 1 ? 'close' : 'check'}
                        title={
                          user.status === 1
                            ? t('user.buttons.disable')
                            : t('user.buttons.enable')
                        }
                        color={user.status === 1 ? undefined : 'blue'}
                        disabled={!canManageAdminUser}
                      />
                    </span>
                  </AppPopconfirm>
                  <AppPopconfirm
                    title={t('user.buttons.confirm_delete')}
                    onConfirm={() => {
                      manageUser(user, 'delete');
                    }}
                    disabled={!canManageAdminUser}
                  >
                    <span>
                      <AppTableActionButton
                        icon='trash'
                        title={t('user.buttons.delete')}
                        color='red'
                        disabled={!canManageAdminUser}
                      />
                    </span>
                  </AppPopconfirm>
                </div>
              );
            },
          },
          ])}
          />
        </AppSpin>
      </div>
      <div className='router-pagination-wrap'>
        <AppPagination
          className='router-page-pagination'
          activePage={activePage}
          onPageChange={onPaginationChange}
          siblingRange={1}
          total={paginationTotal}
          pageSize={pageSize}
        />
      </div>
      <AppModal
        open={batchTopupOpen}
        onClose={closeBatchTopupModal}
        size='small'
        title={t('user.batch.grant_topup')}
        footer={
          <AppFormActions>
            <AppButton
              type='button'
              onClick={closeBatchTopupModal}
              disabled={batchTopupSubmitting}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              color='blue'
              loading={batchTopupSubmitting}
              onClick={submitBatchTopup}
            >
              {t('user.batch.confirm_grant')}
            </AppButton>
          </AppFormActions>
        }
      >
        <div className='router-page-stack'>
          <div className='router-form-hint'>
            {t('user.batch.topup_confirm_hint', {
              count: selectedUserCount,
            })}
          </div>
          <AppField label={t('user.detail.assign.topup_plan')} required>
            <AppSelect
              className='router-section-input'
              fluid
              search
              clearable
              loading={topupPlanOptionsLoading}
              placeholder={t('user.detail.assign.topup_plan_placeholder')}
              options={topupPlanOptions}
              value={batchTopupForm.plan_id}
              onChange={(e, { value }) =>
                setBatchTopupForm((prev) => ({
                  ...prev,
                  plan_id: (value || '').toString(),
                }))
              }
            />
          </AppField>
          {batchTopupResult ? (
            <div className='router-batch-action-result'>
              <div className='router-batch-action-result-summary'>
                {t('user.batch.topup_result_summary', {
                  total: batchTopupResult.total,
                  success: batchTopupResult.succeeded,
                  failed: batchTopupResult.failed,
                })}
              </div>
              {batchTopupFailedItems.length > 0 ? (
                <div className='router-batch-action-failed-list'>
                  <div className='router-batch-action-failed-title'>
                    {t('user.batch.failed_users')}
                  </div>
                  {batchTopupFailedItems.slice(0, 10).map((item) => (
                    <div
                      className='router-batch-action-failed-item'
                      key={item?.user_id}
                    >
                      <span>{item?.username || item?.user_id}</span>
                      <span>{item?.message || '-'}</span>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </AppModal>
    </>
  );
};

export default UsersTable;
