import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  downloadTextAsFile,
  isRoot,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
  writePagedRows,
} from '../helpers';
import { useTranslation } from 'react-i18next';
import UnitDropdown from './UnitDropdown';

import { ITEMS_PER_PAGE } from '../constants';
import {
  USER_LIST_COLUMN_WIDTHS,
  USER_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import {
  formatCompactNumber,
  renderText,
} from '../helpers/render';
import {
  buildDisplayUnitOptions,
  buildPublicDisplayCurrencyIndex,
  loadPublicDisplayCurrencyCatalog,
  resolvePreferredDisplayCurrency,
  chargeAmountToBillingInputValue,
} from '../helpers/billing';
import {
  AppButton,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppIcon,
  AppInput,
  AppModal,
  AppPagination,
  AppSelect,
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

const maskWalletAddress = (walletAddress) => {
  if (typeof walletAddress !== 'string') return '';
  const trimmedWallet = walletAddress.trim();
  if (trimmedWallet.length < 7) return trimmedWallet;
  return `${trimmedWallet.slice(0, 3)}...${trimmedWallet.slice(-3)}`;
};

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
      throw new Error(message || '权益商品加载失败');
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

const UsersTable = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const isAdminScope = location.pathname.startsWith('/admin/');
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [isSearchMode, setIsSearchMode] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [focusLabel, setFocusLabel] = useState('');
  const [focusTotal, setFocusTotal] = useState(0);
  const [isFocusMode, setIsFocusMode] = useState(false);
  const [tableSorter, setTableSorter] = useState({
    columnKey: 'created_at',
    order: 'descend',
  });
  const initializedSearchRef = useRef(false);
  const [currencyIndex, setCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([]),
  );
  const [balanceUnit, setBalanceUnit] = useState(() =>
    resolvePreferredDisplayCurrency(buildPublicDisplayCurrencyIndex([]), 'USD'),
  );
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [batchSelectionMode, setBatchSelectionMode] = useState(false);
  const [topupPlanOptions, setTopupPlanOptions] = useState([]);
  const [topupPlanOptionsLoading, setTopupPlanOptionsLoading] = useState(false);
  const [batchTopupOpen, setBatchTopupOpen] = useState(false);
  const [batchTopupForm, setBatchTopupForm] = useState({
    plan_id: '',
  });
  const [batchTopupSubmitting, setBatchTopupSubmitting] = useState(false);
  const [batchTopupResult, setBatchTopupResult] = useState(null);

  const loadUsers = useCallback(
    async (page) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      const res = await API.get(`/api/v1/admin/user/?page=${normalizedPage}`);
      const { success, message, data, meta } = res.data;
      if (success) {
        setIsSearchMode(false);
        setTotalCount(Number(meta?.total || data?.length || 0));
        if (normalizedPage === 1) {
          setUsers(data);
        } else {
          setUsers((prev) => writePagedRows(prev, normalizedPage, ITEMS_PER_PAGE, data));
        }
      } else {
        showError(message);
      }
      setLoading(false);
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

  const refresh = async () => {
    setLoading(true);
    const params = new URLSearchParams(location.search || '');
    const focusIDs = (params.get('focus_ids') || '')
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
    const focusName = (params.get('focus_name') || '').trim();
    const focusTotalHint = Number(params.get('focus_total') || 0);
    if (focusIDs.length > 0) {
      await loadUsersByIDs(focusIDs, focusName, focusTotalHint);
      return;
    }
    setIsFocusMode(false);
    setFocusTotal(0);
    await loadUsers(activePage);
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
      showError(error?.message || error);
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

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      const nextPage = Number(activePage) > 0 ? Number(activePage) : 1;
      const hasLoadedPageRows = users
        .slice((nextPage - 1) * ITEMS_PER_PAGE, nextPage * ITEMS_PER_PAGE)
        .some(Boolean);
      if (!isSearchMode && !hasLoadedPageRows) {
        await loadUsers(nextPage);
      }
      setActivePage(nextPage);
    })();
  };

  useEffect(() => {
    const params = new URLSearchParams(location.search || '');
    const focusIDs = (params.get('focus_ids') || '')
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
    const focusName = (params.get('focus_name') || '').trim();
    const focusTotalHint = Number(params.get('focus_total') || 0);
    setLoading(true);
    if (focusIDs.length > 0) {
      loadUsersByIDs(focusIDs, focusName, focusTotalHint).catch((reason) => {
        showError(reason?.message || reason);
        setLoading(false);
      });
      return;
    }
    setFocusLabel('');
    setFocusTotal(0);
    setIsFocusMode(false);
    loadUsers(1)
      .then()
      .catch((reason) => {
        showError(reason);
        setLoading(false);
      });
  }, [loadUsers, loadUsersByIDs, location.search]);

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

  const manageUser = async (username, action, idx) => {
    const res = await API.post('/api/v1/admin/user/manage', {
      username,
      action,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('user.messages.operation_success'));
      let user = res.data.data;
      let newUsers = [...users];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      if (action === 'delete') {
        newUsers[realIdx].deleted = true;
        setTotalCount((prev) => Math.max(prev - 1, 0));
        setSelectedRowKeys((prev) => prev.filter((key) => key !== user.id));
      } else {
        newUsers[realIdx].status = user.status;
        newUsers[realIdx].role = user.role;
      }
      setUsers(newUsers);
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

  const copyWalletAddress = async (walletAddress) => {
    if (!walletAddress) return;
    if (await copy(walletAddress)) {
      showSuccess(t('user.messages.wallet_copy_success'));
      return;
    }
    showError(t('user.messages.wallet_copy_failed'));
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
    setBatchSelectionMode(true);
  }, []);

  const cancelBatchSelectionMode = useCallback(() => {
    if (batchTopupSubmitting) {
      return;
    }
    setBatchSelectionMode(false);
    setSelectedRowKeys([]);
    setBatchTopupResult(null);
  }, [batchTopupSubmitting]);

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
        setBatchSelectionMode(false);
      }
      await refresh();
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setBatchTopupSubmitting(false);
    }
  }, [batchTopupForm.plan_id, refresh, selectedRowKeys, t]);

  const searchUsers = async () => {
    setFocusLabel('');
    setFocusTotal(0);
    setIsFocusMode(false);
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadUsers(1);
      setActivePage(1);
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
    if (!initializedSearchRef.current) {
      initializedSearchRef.current = true;
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
  const totalPages = Math.max(
    Math.ceil((isSearchMode ? visibleUserCount : totalCount) / ITEMS_PER_PAGE),
    1,
  );

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

  const exportCurrentUsers = useCallback(() => {
    const exportRows = (Array.isArray(users) ? users : []).filter((user) => !user?.deleted);
    if (exportRows.length === 0) {
      return;
    }
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
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    const focusSuffix = focusLabel ? `-${focusLabel}` : '';
    downloadTextAsFile(lines.join('\n'), `users${focusSuffix}-${timestamp}.csv`);
  }, [focusLabel, users]);

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
        breadcrumbs={[
          {
            key: 'workspace',
            label: isAdminScope
              ? t('header.admin_workspace')
              : t('header.user_workspace'),
          },
          { key: 'business', label: t('header.operation') },
          { key: 'user', label: t('header.user'), active: true },
        ]}
        title={t('header.user')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              color='blue'
              onClick={() => navigate('/admin/user/add')}
            >
              {t('user.buttons.add')}
            </AppButton>
            <AppButton
              className='router-page-button'
              onClick={
                batchSelectionMode ? openBatchTopupModal : enterBatchSelectionMode
              }
            >
              {batchSelectionMode
                ? t('user.batch.grant_topup_selected', {
                    count: selectedUserCount,
                  })
                : t('user.batch.grant_topup')}
            </AppButton>
            {batchSelectionMode ? (
              <AppButton
                className='router-page-button'
                onClick={cancelBatchSelectionMode}
                disabled={batchTopupSubmitting}
              >
                {t('user.batch.cancel_selection')}
              </AppButton>
            ) : null}
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
              disabled={users.filter((user) => !user?.deleted).length === 0}
              onClick={exportCurrentUsers}
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
            {focusLabel ? (
              <AppTag className='router-tag'>{focusLabel}</AppTag>
            ) : null}
          </div>
        }
      />

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

      <div className='router-table-scroll-x'>
        <AppTable
          className='router-hover-table router-list-table router-table-fit-page router-user-list-table'
          pagination={false}
          scroll={{ x: USER_LIST_TABLE_MIN_WIDTH }}
          rowKey={(user) => user.id}
          rowSelection={userRowSelection}
          onChange={handleTableChange}
          dataSource={users
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE,
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
              navigate(`/admin/user/detail/${user.id}`);
            },
          })}
          columns={[
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
                    <div>{user.email ? user.email : '未绑定邮箱地址'}</div>
                  </div>
                }
              >
                <span>{renderText(user.username, 15)}</span>
              </AppTooltip>
            ),
          },
          {
            title: t('user.table.wallet'),
            dataIndex: 'wallet_address',
            key: 'wallet_address',
            width: USER_LIST_COLUMN_WIDTHS.wallet,
            render: (value) =>
              value ? (
                <span className='router-action-group'>
                  <AppTooltip title={value}>
                    <span>{maskWalletAddress(value)}</span>
                  </AppTooltip>
                  <button
                    type='button'
                    className='router-icon-button'
                    onClick={(event) => {
                      stopRowClick(event);
                      copyWalletAddress(value);
                    }}
                  >
                    <AppIcon name='copy outline' />
                  </button>
                </span>
              ) : (
                '-'
              ),
          },
          {
            title: t('user.table.package'),
            dataIndex: 'active_package_name',
            key: 'active_package_name',
            width: USER_LIST_COLUMN_WIDTHS.package,
            ellipsis: true,
            sorter: (a, b) =>
              compareTextValue(a.active_package_name, b.active_package_name),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'active_package_name'
                ? tableSorter.order
                : null,
            render: (value) => (value ? renderText(value, 18) : '-'),
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
            title: t('user.table.updated_at'),
            dataIndex: 'updated_at',
            key: 'updated_at',
            className: 'router-table-col-datetime',
            width: USER_LIST_COLUMN_WIDTHS.updatedAt,
            sorter: (a, b) => compareNumberValue(a.updated_at, b.updated_at),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'updated_at'
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
            width: 84,
            render: (_, user, idx) => {
              const isAdminUser = Number(user.role) >= 10;
              const canManageAdminUser = !isAdminUser || isRoot();
              return (
                <div
                  className='router-action-group router-table-actions-icon-compact'
                  onClick={stopRowClick}
                >
                  <AppTableActionButton
                    icon={user.status === 1 ? 'close' : 'check'}
                    title={
                      user.status === 1
                        ? t('user.buttons.disable')
                        : t('user.buttons.enable')
                    }
                    color={user.status === 1 ? undefined : 'blue'}
                    onClick={() => {
                      manageUser(
                        user.username,
                        user.status === 1 ? 'disable' : 'enable',
                        idx,
                      );
                    }}
                    disabled={!canManageAdminUser}
                  />
                  <AppTableActionButton
                    icon='trash'
                    title={t('user.buttons.delete')}
                    color='red'
                    disabled={!canManageAdminUser}
                    onClick={() => {
                      manageUser(user.username, 'delete', idx);
                    }}
                  />
                </div>
              );
            },
          },
          ]}
        />
      </div>
      <div className='router-pagination-wrap'>
        <AppPagination
          className='router-page-pagination'
          activePage={activePage}
          onPageChange={onPaginationChange}
          siblingRange={1}
          totalPages={totalPages}
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
