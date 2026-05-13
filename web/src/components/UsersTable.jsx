import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, copy, isRoot, showError, showSuccess, timestamp2string } from '../helpers';
import { useTranslation } from 'react-i18next';
import UnitDropdown from './UnitDropdown';

import { ITEMS_PER_PAGE } from '../constants';
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
  AppSelect,
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
  const [orderBy, setOrderBy] = useState('');
  const [currencyIndex, setCurrencyIndex] = useState(() =>
    buildPublicDisplayCurrencyIndex([]),
  );
  const [balanceUnit, setBalanceUnit] = useState(() =>
    resolvePreferredDisplayCurrency(buildPublicDisplayCurrencyIndex([]), 'USD'),
  );

  const loadUsers = useCallback(
    async (page) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      const res = await API.get(
        `/api/v1/admin/user/?page=${normalizedPage}&order=${orderBy}`,
      );
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
    [orderBy],
  );

  const refresh = async () => {
    setLoading(true);
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
    loadUsers(1)
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, [loadUsers]);

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
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadUsers(1);
      setActivePage(1);
      setOrderBy('');
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
    setSearchKeyword(value.trim());
  };

  const stopRowClick = (event) => {
    event.stopPropagation();
  };

  const visibleUserCount = users.filter((user) => !user?.deleted).length;
  const totalPages = Math.max(
    Math.ceil((isSearchMode ? visibleUserCount : totalCount) / ITEMS_PER_PAGE),
    1,
  );

  const sortUser = (key) => {
    if (users.length === 0) return;
    setLoading(true);
    let sortedUsers = [...users];
    sortedUsers.sort((a, b) => {
      if (!isNaN(a[key])) {
        // If the value is numeric, subtract to sort
        return a[key] - b[key];
      } else {
        // If the value is not numeric, sort as strings
        return ('' + a[key]).localeCompare(b[key]);
      }
    });
    if (sortedUsers[0].id === users[0].id) {
      sortedUsers.reverse();
    }
    setUsers(sortedUsers);
    setLoading(false);
  };

  const handleOrderByChange = (e, { value }) => {
    setOrderBy(value);
    setActivePage(1);
  };

  const renderCountValue = (value) => (
    <AppTooltip title={formatFullNumber(value)}>
      <span>{formatCompactNumber(value)}</span>
    </AppTooltip>
  );

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
              onClick={() => navigate('/user/add')}
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
          </div>
        }
        query={
          <div className='router-list-toolbar-query router-list-toolbar-query-compact'>
            <AppSelect
              className='router-section-dropdown router-dropdown-min-170'
              placeholder={t('user.table.sort_by')}
              options={[
                { key: '', text: t('user.table.sort.default'), value: '' },
                {
                  key: 'request_count',
                  text: t('user.table.sort.by_request_count'),
                  value: 'request_count',
                },
              ]}
              value={orderBy}
              onChange={handleOrderByChange}
            />
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
          </div>
        }
      />

      <AppTable
        className='router-hover-table router-list-table'
        pagination={false}
        rowKey={(user) => user.id}
        dataSource={users
          .slice(
            (activePage - 1) * ITEMS_PER_PAGE,
            activePage * ITEMS_PER_PAGE,
          )
          .filter((user) => !user?.deleted)}
        onRow={(user, idx) => ({
          className: 'router-row-clickable',
          onClick: () => navigate(`/user/detail/${user.id}`),
        })}
        columns={[
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('username');
                }}
              >
                {t('user.table.username')}
              </span>
            ),
            dataIndex: 'username',
            key: 'username',
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
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('active_package_name');
                }}
              >
                {t('user.table.package')}
              </span>
            ),
            dataIndex: 'active_package_name',
            key: 'active_package_name',
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
            render: (_, user) =>
              yycToBillingInputValue(
                user.yyc_balance ?? user.quota,
                balanceUnit,
                currencyIndex,
              ),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('request_count');
                }}
              >
                {t('user.table.request_count')}
              </span>
            ),
            dataIndex: 'request_count',
            key: 'request_count',
            render: (value) => renderCountValue(value),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('created_at');
                }}
              >
                {t('user.table.created_at')}
              </span>
            ),
            dataIndex: 'created_at',
            key: 'created_at',
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('updated_at');
                }}
              >
                {t('user.table.updated_at')}
              </span>
            ),
            dataIndex: 'updated_at',
            key: 'updated_at',
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('role');
                }}
              >
                {t('user.table.role_text')}
              </span>
            ),
            dataIndex: 'role',
            key: 'role',
            render: (value) => renderRole(value, t),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortUser('status');
                }}
              >
                {t('user.table.status_text')}
              </span>
            ),
            dataIndex: 'status',
            key: 'status',
            render: (value) => renderStatus(value),
          },
          {
            title: t('user.table.actions'),
            key: 'actions',
            render: (_, user, idx) => {
              const isAdminUser = Number(user.role) >= 10;
              const canManageAdminUser = !isAdminUser || isRoot();
              return (
                <div className='router-action-group' onClick={stopRowClick}>
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
