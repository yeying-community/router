import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Icon,
  Form,
  Label,
  Pagination,
  Popup,
  Table,
  Dropdown,
} from 'semantic-ui-react';
import { Link, useNavigate } from 'react-router-dom';
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

function renderRole(role, t) {
  switch (role) {
    case 1:
      return (
        <Label className='router-tag'>
          {t('user.table.role_types.normal')}
        </Label>
      );
    case 10:
      return (
        <Label color='yellow' className='router-tag'>
          {t('user.table.role_types.admin')}
        </Label>
      );
    default:
      return (
        <Label color='red' className='router-tag'>
          {t('user.table.role_types.unknown')}
        </Label>
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
          <Label basic className='router-tag'>
            {t('user.table.status_types.activated')}
          </Label>
        );
      case 2:
        return (
          <Label basic color='red' className='router-tag'>
            {t('user.table.status_types.banned')}
          </Label>
        );
      default:
        return (
          <Label basic color='grey' className='router-tag'>
            {t('user.table.status_types.unknown')}
          </Label>
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
    <Popup
      content={formatFullNumber(value)}
      trigger={<span>{formatCompactNumber(value)}</span>}
    />
  );

  const balanceUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex),
    [currencyIndex],
  );

  return (
    <>
      <div className='router-toolbar router-block-gap-sm'>
        <div className='router-toolbar-start'>
          <Button
            className='router-page-button'
            as={Link}
            to='/user/add'
          >
            {t('user.buttons.add')}
          </Button>
          <Button
            className='router-page-button'
            loading={loading}
            disabled={loading}
            onClick={refresh}
          >
            {t('user.buttons.refresh')}
          </Button>
        </div>
        <div className='router-toolbar-end'>
          <Dropdown
            className='router-section-dropdown router-dropdown-min-170'
            placeholder={t('user.table.sort_by')}
            selection
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
          <Form onSubmit={searchUsers} className='router-search-form-xs'>
            <Form.Input
              className='router-section-input'
              icon='search'
              iconPosition='left'
              placeholder={t('user.search')}
              value={searchKeyword}
              loading={searching}
              onChange={handleKeywordChange}
            />
          </Form>
        </div>
      </div>

      <Table
        basic={'very'}
        compact
        className='router-hover-table router-list-table'
      >
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('username');
              }}
            >
              {t('user.table.username')}
            </Table.HeaderCell>
            <Table.HeaderCell>{t('user.table.wallet')}</Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('active_package_name');
              }}
            >
              {t('user.table.package')}
            </Table.HeaderCell>
            <Table.HeaderCell className='router-redemption-face-value-header'>
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
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('request_count');
              }}
            >
              {t('user.table.request_count')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('created_at');
              }}
            >
              {t('user.table.created_at')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('updated_at');
              }}
            >
              {t('user.table.updated_at')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('role');
              }}
            >
              {t('user.table.role_text')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('status');
              }}
            >
              {t('user.table.status_text')}
            </Table.HeaderCell>
            <Table.HeaderCell>{t('user.table.actions')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {users
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE,
            )
            .map((user, idx) => {
              if (user.deleted) return <></>;
              const isAdminUser = Number(user.role) >= 10;
              const canManageAdminUser = !isAdminUser || isRoot();
              return (
                <Table.Row
                  key={user.id}
                  className='router-row-clickable'
                  onClick={() => navigate(`/user/detail/${user.id}`)}
                >
                  <Table.Cell>
                    <Popup
                      content={user.email ? user.email : '未绑定邮箱地址'}
                      key={user.username}
                      header={user.username}
                      trigger={<span>{renderText(user.username, 15)}</span>}
                      hoverable
                    />
                  </Table.Cell>
                  <Table.Cell onClick={stopRowClick}>
                    {user.wallet_address ? (
                      <span className='router-action-group'>
                        <Popup
                          content={user.wallet_address}
                          trigger={
                            <span>
                              {maskWalletAddress(user.wallet_address)}
                            </span>
                          }
                        />
                        <Icon
                          name='copy outline'
                          link
                          onClick={() => copyWalletAddress(user.wallet_address)}
                        />
                      </span>
                    ) : (
                      '-'
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    {user.active_package_name
                      ? renderText(user.active_package_name, 18)
                      : '-'}
                  </Table.Cell>
                  <Table.Cell>
                    {yycToBillingInputValue(
                      user.yyc_balance ?? user.quota,
                      balanceUnit,
                      currencyIndex,
                    )}
                  </Table.Cell>
                  <Table.Cell>
                    {renderCountValue(user.request_count)}
                  </Table.Cell>
                  <Table.Cell>{user.created_at ? timestamp2string(user.created_at) : '-'}</Table.Cell>
                  <Table.Cell>{user.updated_at ? timestamp2string(user.updated_at) : '-'}</Table.Cell>
                  <Table.Cell>
                    {renderRole(user.role, t)}
                  </Table.Cell>
                  <Table.Cell>{renderStatus(user.status)}</Table.Cell>
                  <Table.Cell onClick={stopRowClick}>
                    <div className='router-action-group'>
                      <Popup
                        trigger={
                          <Button
                            className='router-inline-button'
                            negative
                            disabled={!canManageAdminUser}
                          >
                            {t('user.buttons.delete')}
                          </Button>
                        }
                        on='click'
                        flowing
                        hoverable
                      >
                        <Button
                          className='router-inline-button'
                          negative
                          disabled={!canManageAdminUser}
                          onClick={() => {
                            manageUser(user.username, 'delete', idx);
                          }}
                        >
                          {t('user.buttons.delete_user')} {user.username}
                        </Button>
                      </Popup>
                      <Button
                        className='router-inline-button'
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
                      </Button>
                    </div>
                  </Table.Cell>
                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan='10'>
              <Pagination
                className='router-page-pagination'
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                siblingRange={1}
                totalPages={totalPages}
              />
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default UsersTable;
