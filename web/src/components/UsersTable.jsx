import React, { useCallback, useEffect, useState } from 'react';
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
import { API, copy, isRoot, showError, showSuccess } from '../helpers';
import { useTranslation } from 'react-i18next';

import { ITEMS_PER_PAGE } from '../constants';
import {
  renderGroup,
  formatCompactNumber,
  renderText,
} from '../helpers/render';

function renderRole(role, t) {
  switch (role) {
    case 1:
      return <Label className='router-tag'>{t('user.table.role_types.normal')}</Label>;
    case 10:
      return <Label color='yellow' className='router-tag'>{t('user.table.role_types.admin')}</Label>;
    case 100:
      return (
        <Label color='orange' className='router-tag'>{t('user.table.role_types.super_admin')}</Label>
      );
    default:
      return <Label color='red' className='router-tag'>{t('user.table.role_types.unknown')}</Label>;
  }
}

const ROLE_OPTIONS = (t) => [
  { key: 1, value: 1, text: t('user.table.role_types.normal') },
  { key: 10, value: 10, text: t('user.table.role_types.admin') },
];

const maskWalletAddress = (walletAddress) => {
  if (typeof walletAddress !== 'string') return '';
  const trimmedWallet = walletAddress.trim();
  if (trimmedWallet.length < 7) return trimmedWallet;
  return `${trimmedWallet.slice(0, 3)}...${trimmedWallet.slice(-3)}`;
};

const UsersTable = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [users, setUsers] = useState([]);
  const [groupMap, setGroupMap] = useState({});
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [orderBy, setOrderBy] = useState('');
  const [roleUpdatingUsername, setRoleUpdatingUsername] = useState('');

  const loadUsers = useCallback(async (page) => {
    const normalizedPage = Number(page) > 0 ? Number(page) : 1;
    const res = await API.get(`/api/v1/admin/user/?page=${normalizedPage}&order=${orderBy}`);
    const { success, message, data } = res.data;
    if (success) {
      if (normalizedPage === 1) {
        setUsers(data);
      } else {
        setUsers((prev) => [...prev, ...data]);
      }
    } else {
      showError(message);
    }
    setLoading(false);
  }, [orderBy]);

  const loadGroups = useCallback(async () => {
    try {
      const rows = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/groups', {
          params: {
            page,
            page_size: 100,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('user.messages.operation_failed'));
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        rows.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (pageItems.length === 0 || rows.length >= total || pageItems.length < 100) {
          break;
        }
        page += 1;
      }
      const nextMap = {};
      rows.forEach((group) => {
        const id = (group?.id || '').toString().trim();
        if (id === '') {
          return;
        }
        nextMap[id] = (group?.name || '').toString().trim() || id;
      });
      setGroupMap(nextMap);
    } catch (error) {
      showError(error?.message || error);
    }
  }, [t]);

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      if (activePage === Math.ceil(users.length / ITEMS_PER_PAGE) + 1) {
        // In this case we have to load more data and then append them.
        await loadUsers(activePage);
      }
      setActivePage(activePage);
    })();
  };

  useEffect(() => {
    loadUsers(1)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    loadGroups().then();
  }, [loadGroups, loadUsers]);

  const renderUserGroup = useCallback(
    (value) => {
      const raw = (value || '').toString().trim();
      if (raw === '') {
        return renderGroup(raw);
      }
      const mapped = raw
        .split(',')
        .map((item) => item.trim())
        .filter((item) => item !== '')
        .map((item) => groupMap[item] || item)
        .join(',');
      return renderGroup(mapped);
    },
    [groupMap],
  );

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
        return <Label basic className='router-tag'>{t('user.table.status_types.activated')}</Label>;
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
    const res = await API.get(`/api/v1/admin/user/search?keyword=${searchKeyword}`);
    const { success, message, data } = res.data;
    if (success) {
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

  const updateUserRole = async (user, idx, nextRole) => {
    if (!user || user.role === nextRole) return;
    if (user.role === 100 || nextRole === 100) return;

    let action = '';
    if (nextRole === 10) {
      action = 'promote';
    } else if (nextRole === 1) {
      action = 'demote';
    }
    if (!action) return;

    setRoleUpdatingUsername(user.username);
    try {
      await manageUser(user.username, action, idx);
    } finally {
      setRoleUpdatingUsername('');
    }
  };

  const quotaPerUnit = parseFloat(
    localStorage.getItem('quota_per_unit') || '1'
  );

  const formatFullNumber = (value) => {
    const num = Number(value);
    if (!Number.isFinite(num)) return '0';
    return num.toLocaleString();
  };

  const formatFullAmount = (value) => {
    const num = Number(value);
    if (!Number.isFinite(num)) return '0';
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    });
  };

  const renderQuotaValue = (value) => (
    (() => {
      const numericValue = Number(value);
      const base = Number.isFinite(numericValue) ? numericValue : 0;
      const amount = quotaPerUnit > 0 ? base / quotaPerUnit : base;
      return (
        <Popup
          content={`$${formatFullAmount(amount)}`}
          trigger={<span>{formatCompactNumber(amount)}</span>}
        />
      );
    })()
  );

  const renderCountValue = (value) => (
    <Popup
      content={formatFullNumber(value)}
      trigger={<span>{formatCompactNumber(value)}</span>}
    />
  );

  return (
    <>
      <div
        className='router-toolbar router-block-gap-sm'
      >
        <div className='router-toolbar-start'>
          <Button className='router-page-button' as={Link} to='/user/add' loading={loading}>
            {t('user.buttons.add')}
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
                key: 'quota',
                text: t('user.table.sort.by_quota'),
                value: 'quota',
              },
              {
                key: 'used_quota',
                text: t('user.table.sort.by_used_quota'),
                value: 'used_quota',
              },
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

      <Table basic={'very'} compact className='router-hover-table router-list-table'>
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
                sortUser('group');
              }}
            >
              {t('user.table.group')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('quota');
              }}
            >
              {t('user.table.remaining_quota')}
              <span className='router-inline-unit'>
                $
              </span>
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortUser('used_quota');
              }}
            >
              {t('user.table.used_quota')}
              <span className='router-inline-unit'>
                $
              </span>
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
              activePage * ITEMS_PER_PAGE
            )
            .map((user, idx) => {
              if (user.deleted) return <></>;
              return (
                <Table.Row
                  key={user.id}
                  className='router-row-clickable'
                  onClick={() => navigate(`/user/edit/${user.id}`)}
                >
                  <Table.Cell>
                    <Popup
                      content={user.email ? user.email : '未绑定邮箱地址'}
                      key={user.username}
                      header={
                        user.display_name ? user.display_name : user.username
                      }
                      trigger={<span>{renderText(user.username, 15)}</span>}
                      hoverable
                    />
                  </Table.Cell>
                  <Table.Cell onClick={stopRowClick}>
                    {user.wallet_address ? (
                      <span className='router-action-group'>
                        <Popup
                          content={user.wallet_address}
                          trigger={<span>{maskWalletAddress(user.wallet_address)}</span>}
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
                  <Table.Cell>{renderUserGroup(user.group)}</Table.Cell>
                  {/*<Table.Cell>*/}
                  {/*  {user.email ? <Popup hoverable content={user.email} trigger={<span>{renderText(user.email, 24)}</span>} /> : '无'}*/}
                  {/*</Table.Cell>*/}
                  <Table.Cell>{renderQuotaValue(user.quota)}</Table.Cell>
                  <Table.Cell>{renderQuotaValue(user.used_quota)}</Table.Cell>
                  <Table.Cell>{renderCountValue(user.request_count)}</Table.Cell>
                  <Table.Cell>
                    {user.role === 100 ? (
                      renderRole(user.role, t)
                    ) : (
                      <Dropdown
                        className='router-inline-dropdown router-role-dropdown'
                        selection
                        compact
                        options={ROLE_OPTIONS(t)}
                        value={user.role}
                        disabled={!isRoot() || roleUpdatingUsername === user.username}
                        loading={roleUpdatingUsername === user.username}
                        onChange={(e, { value }) => updateUserRole(user, idx, Number(value))}
                      />
                    )}
                  </Table.Cell>
                  <Table.Cell>{renderStatus(user.status)}</Table.Cell>
                  <Table.Cell onClick={stopRowClick}>
                    <div className='router-action-group'>
                      <Popup
                        trigger={
                          <Button
                            className='router-inline-button'
                            negative
                            disabled={user.role === 100}
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
                            idx
                          );
                        }}
                        disabled={user.role === 100}
                      >
                        {user.status === 1
                          ? t('user.buttons.disable')
                          : t('user.buttons.enable')}
                      </Button>
                      <Button
                        className='router-inline-button'
                        as={Link}
                        to={'/user/edit/' + user.id}
                      >
                        {t('user.buttons.edit')}
                      </Button>
                    </div>
                  </Table.Cell>
                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan='9'>
              <Pagination
                className='router-page-pagination'
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                siblingRange={1}
                totalPages={
                  Math.ceil(users.length / ITEMS_PER_PAGE) +
                  (users.length % ITEMS_PER_PAGE === 0 ? 1 : 0)
                }
              />
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default UsersTable;
