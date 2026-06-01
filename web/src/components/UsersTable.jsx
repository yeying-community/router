import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  downloadTextAsFile,
  isRoot,
  showError,
  showSuccess,
  timestamp2string,
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
  yycToBillingInputValue,
} from '../helpers/billing';
import {
  AppButton,
  AppFilterHeader,
  AppIcon,
  AppInput,
  AppPagination,
  AppTable,
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
          setUsers((prev) => {
            const next = [...prev];
            next.splice(
              (normalizedPage - 1) * ITEMS_PER_PAGE,
              data.length,
              ...data,
            );
            return next;
          });
        }
      } else {
        showError(message);
      }
      setLoading(false);
    },
    [],
  );

  const loadUsersByIDs = useCallback(async (userIDs, label = '') => {
    const normalizedIDs = [...new Set(
      (Array.isArray(userIDs) ? userIDs : [])
        .map((item) => (item || '').toString().trim())
        .filter(Boolean),
    )];
    if (normalizedIDs.length === 0) {
      setFocusLabel('');
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
    if (focusIDs.length > 0) {
      await loadUsersByIDs(focusIDs, focusName);
      return;
    }
    setIsFocusMode(false);
    await loadUsers(activePage);
  };

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
    setLoading(true);
    if (focusIDs.length > 0) {
      loadUsersByIDs(focusIDs, focusName).catch((reason) => {
        showError(reason?.message || reason);
        setLoading(false);
      });
      return;
    }
    setFocusLabel('');
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

  const searchUsers = async () => {
    setFocusLabel('');
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
    setIsFocusMode(false);
    setSearchKeyword(value.trim());
  };

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
      'yyc_balance',
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
          user?.yyc_balance ?? user?.quota,
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
          { key: 'business', label: t('header.business_operation') },
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

      <div className='router-table-scroll-x'>
        <AppTable
          className='router-hover-table router-list-table router-table-fit-page router-user-list-table'
          pagination={false}
          scroll={{ x: USER_LIST_TABLE_MIN_WIDTH }}
          rowKey={(user) => user.id}
          onChange={handleTableChange}
          dataSource={users
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE,
            )
            .filter((user) => !user?.deleted)}
          onRow={(user, idx) => ({
            className: 'router-row-clickable',
            onClick: () => navigate(`/admin/user/detail/${user.id}`),
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
                yycToBillingInputValue(
                  user.yyc_balance ?? user.quota,
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
            className: 'router-table-col-actions-compact',
            width: USER_LIST_COLUMN_WIDTHS.actions,
            render: (_, user, idx) => {
              const isAdminUser = Number(user.role) >= 10;
              const canManageAdminUser = !isAdminUser || isRoot();
              return (
                <div
                  className='router-action-group router-table-actions-compact'
                  onClick={stopRowClick}
                >
                  <AppButton
                    className='router-inline-button'
                    color={user.status === 1 ? undefined : 'blue'}
                    onClick={() => {
                      manageUser(
                        user.username,
                        user.status === 1 ? 'disable' : 'enable',
                        idx,
                      );
                    }}
                    disabled={!canManageAdminUser}
                  >
                    {user.status === 1
                      ? t('user.buttons.disable')
                      : t('user.buttons.enable')}
                  </AppButton>
                  <AppButton
                    className='router-inline-button'
                    color='red'
                    disabled={!canManageAdminUser}
                    onClick={() => {
                      manageUser(user.username, 'delete', idx);
                    }}
                  >
                    {t('user.buttons.delete')}
                  </AppButton>
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
    </>
  );
};

export default UsersTable;
