import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import UnitDropdown from './UnitDropdown';
import {
  API,
  copy,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../helpers';

import { ITEMS_PER_PAGE } from '../constants';
import {
  buildDisplayUnitOptions,
  buildPublicDisplayCurrencyIndex,
  formatDisplayAmountFromYYC,
  loadPublicDisplayCurrencyCatalog,
  resolvePreferredDisplayCurrency,
} from '../helpers/billing';
import {
  AppButton,
  AppFilterHeader,
  AppIcon,
  AppInput,
  AppMenuDropdown,
  AppPagination,
  AppPopconfirm,
  AppSelect,
  AppTable,
  AppTag,
  AppToolbar,
} from '../router-ui';

const normalizeTokenRow = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  return {
    ...raw,
    yycUsed: Number(raw?.yyc_used ?? raw?.used_quota ?? 0) || 0,
    yycRemaining: Number(raw?.yyc_remain ?? raw?.remain_quota ?? 0) || 0,
    hasUnlimitedYYCLimit: raw?.unlimited_quota === true,
    createdTime: Number(raw?.created_time ?? 0) || 0,
    expiredTime: Number(raw?.expired_time ?? 0) || 0,
  };
};

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

function renderStatus(status, t) {
  switch (status) {
    case 1:
      return (
        <AppTag color='green' className='router-tag'>
          {t('token.table.status_enabled')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='red' className='router-tag'>
          {t('token.table.status_disabled')}
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='yellow' className='router-tag'>
          {t('token.table.status_expired')}
        </AppTag>
      );
    case 4:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('token.table.status_depleted')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='black' className='router-tag'>
          {t('token.table.status_unknown')}
        </AppTag>
      );
  }
}

function renderShortToken(key) {
  const raw = typeof key === 'string' ? key.trim() : '';
  if (raw === '') {
    return '-';
  }
  const withPrefix = raw.startsWith('sk-') ? raw : `sk-${raw}`;
  if (withPrefix.length <= 16) {
    return withPrefix;
  }
  return `${withPrefix.slice(0, 8)}...${withPrefix.slice(-6)}`;
}

function resolveTokenSortValue(token, key) {
  switch (key) {
    case 'name':
      return (token?.name || '').toString();
    case 'status':
      return Number(token?.status || 0);
    case 'usedAmount':
      return Number(token?.yycUsed || 0);
    case 'remainingAmount':
      return Number(token?.yycRemaining || 0);
    case 'createdTime':
      return Number(token?.createdTime || 0);
    case 'expiredTime':
      return Number(token?.expiredTime || 0);
    default:
      return '';
  }
}

const TokensTable = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;

  const OPEN_LINK_OPTIONS = [
    { key: 'next', text: t('token.copy_options.next'), value: 'next' },
    { key: 'ama', text: t('token.copy_options.ama'), value: 'ama' },
    { key: 'opencat', text: t('token.copy_options.opencat'), value: 'opencat' },
    { key: 'lobe', text: t('token.copy_options.lobe'), value: 'lobechat' },
  ];

  const [tokens, setTokens] = useState([]);
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
  const [displayUnit, setDisplayUnit] = useState(() =>
    resolvePreferredDisplayCurrency(buildPublicDisplayCurrencyIndex([]), 'USD'),
  );

  const loadTokens = useCallback(
    async (page) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      const res = await API.get(
        `/api/v1/public/token/?page=${normalizedPage}&order=${orderBy}`,
      );
      const { success, message, data, meta } = res.data;
      if (success) {
        const normalizedRows = Array.isArray(data)
          ? data.map(normalizeTokenRow).filter(Boolean)
          : [];
        setIsSearchMode(false);
        setTotalCount(Number(meta?.total || normalizedRows?.length || 0));
        if (normalizedPage === 1) {
          setTokens(normalizedRows);
        } else {
          setTokens((prev) => {
            let next = [...prev];
            next.splice(
              (normalizedPage - 1) * ITEMS_PER_PAGE,
              normalizedRows.length,
              ...normalizedRows,
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

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      const nextPage = Number(activePage) > 0 ? Number(activePage) : 1;
      const hasLoadedPageRows = tokens
        .slice((nextPage - 1) * ITEMS_PER_PAGE, nextPage * ITEMS_PER_PAGE)
        .some(Boolean);
      if (!isSearchMode && !hasLoadedPageRows) {
        await loadTokens(nextPage);
      }
      setActivePage(nextPage);
    })();
  };

  const refresh = async () => {
    setLoading(true);
    await loadTokens(activePage);
  };

  const onCopy = async (type, key) => {
    let status = localStorage.getItem('status');
    let serverAddress = '';
    if (status) {
      status = JSON.parse(status);
      serverAddress = status.server_address;
    }
    if (serverAddress === '') {
      serverAddress = window.location.origin;
    }
    let encodedServerAddress = encodeURIComponent(serverAddress);
    const nextLink = localStorage.getItem('chat_link');
    let nextUrl;

    if (nextLink) {
      nextUrl =
        nextLink + `/#/?settings={"key":"sk-${key}","url":"${serverAddress}"}`;
    } else {
      nextUrl = `https://app.nextchat.dev/#/?settings={"key":"sk-${key}","url":"${serverAddress}"}`;
    }

    let url;
    switch (type) {
      case 'ama':
        url = `ama://set-api-key?server=${encodedServerAddress}&key=sk-${key}`;
        break;
      case 'opencat':
        url = `opencat://team/join?domain=${encodedServerAddress}&token=sk-${key}`;
        break;
      case 'next':
        url = nextUrl;
        break;
      case 'lobechat':
        url =
          nextLink +
          `/?settings={"keyVaults":{"openai":{"apiKey":"sk-${key}","baseURL":"${serverAddress}/v1"}}}`;
        break;
      default:
        url = `sk-${key}`;
    }
    if (await copy(url)) {
      showSuccess(t('token.messages.copy_success'));
    } else {
      showWarning(t('token.messages.copy_failed'));
      setSearchKeyword(url);
    }
  };

  const onOpenLink = async (type, key) => {
    let status = localStorage.getItem('status');
    let serverAddress = '';
    if (status) {
      status = JSON.parse(status);
      serverAddress = status.server_address;
    }
    if (serverAddress === '') {
      serverAddress = window.location.origin;
    }
    let encodedServerAddress = encodeURIComponent(serverAddress);
    const chatLink = localStorage.getItem('chat_link');
    let defaultUrl;

    if (chatLink) {
      defaultUrl =
        chatLink + `/#/?settings={"key":"sk-${key}","url":"${serverAddress}"}`;
    } else {
      defaultUrl = `https://app.nextchat.dev/#/?settings={"key":"sk-${key}","url":"${serverAddress}"}`;
    }
    let url;
    switch (type) {
      case 'ama':
        url = `ama://set-api-key?server=${encodedServerAddress}&key=sk-${key}`;
        break;

      case 'opencat':
        url = `opencat://team/join?domain=${encodedServerAddress}&token=sk-${key}`;
        break;

      case 'lobechat':
        url =
          chatLink +
          `/?settings={"keyVaults":{"openai":{"apiKey":"sk-${key}","baseURL":"${serverAddress}/v1"}}}`;
        break;

      default:
        url = defaultUrl;
    }

    window.open(url, '_blank');
  };

  useEffect(() => {
    loadTokens(1)
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, [loadTokens]);

  useEffect(() => {
    let disposed = false;
    loadPublicDisplayCurrencyCatalog().then(
      ({ currencyIndex: nextIndex, defaultCurrency }) => {
        if (disposed) {
          return;
        }
        setCurrencyIndex(nextIndex);
        setDisplayUnit((current) =>
          resolvePreferredDisplayCurrency(
            nextIndex,
            current || defaultCurrency || 'USD',
          ),
        );
      },
    );
    return () => {
      disposed = true;
    };
  }, []);

  const manageToken = async (id, action, idx) => {
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/v1/public/token/${id}/`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/v1/public/token/?status_only=true', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/v1/public/token/?status_only=true', data);
        break;
      default:
        return;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('token.messages.operation_success'));
      let token = res.data.data;
      let newTokens = [...tokens];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      if (action === 'delete') {
        newTokens[realIdx].deleted = true;
        setTotalCount((prev) => Math.max(prev - 1, 0));
      } else {
        newTokens[realIdx].status = token.status;
      }
      setTokens(newTokens);
    } else {
      showError(message);
    }
  };

  const searchTokens = async () => {
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadTokens(1);
      setActivePage(1);
      setOrderBy('');
      return;
    }
    setSearching(true);
    const res = await API.get(
      `/api/v1/public/token/search?keyword=${searchKeyword}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      const normalizedRows = Array.isArray(data)
        ? data.map(normalizeTokenRow).filter(Boolean)
        : [];
      setIsSearchMode(true);
      setTotalCount(normalizedRows.length);
      setTokens(normalizedRows);
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

  const sortToken = (key) => {
    if (tokens.length === 0) return;
    setLoading(true);
    let sortedTokens = [...tokens];
    sortedTokens.sort((a, b) => {
      const leftValue = resolveTokenSortValue(a, key);
      const rightValue = resolveTokenSortValue(b, key);
      if (
        typeof leftValue === 'number' &&
        typeof rightValue === 'number' &&
        !Number.isNaN(leftValue) &&
        !Number.isNaN(rightValue)
      ) {
        return leftValue - rightValue;
      } else {
        return String(leftValue).localeCompare(String(rightValue));
      }
    });
    if (sortedTokens[0].id === tokens[0].id) {
      sortedTokens.reverse();
    }
    setTokens(sortedTokens);
    setLoading(false);
  };

  const handleOrderByChange = (e, { value }) => {
    setOrderBy(value);
    setActivePage(1);
  };

  const visibleTokenCount = tokens.filter((token) => !token?.deleted).length;
  const totalPages = Math.max(
    Math.ceil((isSearchMode ? visibleTokenCount : totalCount) / ITEMS_PER_PAGE),
    1,
  );
  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex),
    [currencyIndex],
  );

  return (
    <>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'mine', label: t('header.mine') },
          { key: 'token', label: t('header.token'), active: true },
        ]}
        title={t('header.token')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              color='blue'
              as={Link}
              to='/token/add'
            >
              {t('token.buttons.add')}
            </AppButton>
            <AppButton
              className='router-page-button'
              onClick={refresh}
              loading={loading}
            >
              {t('token.buttons.refresh')}
            </AppButton>
          </div>
        }
        query={
          <div className='router-list-toolbar-query router-list-toolbar-query-compact'>
            <AppSelect
              className='router-section-dropdown router-dropdown-min-170'
              placeholder={t('token.sort.placeholder')}
              options={[
                { key: '', text: t('token.sort.default'), value: '' },
                {
                  key: 'remaining_amount',
                  text: t('token.sort.by_remain'),
                  value: 'remain_quota',
                },
                {
                  key: 'used_amount',
                  text: t('token.sort.by_used'),
                  value: 'used_quota',
                },
              ]}
              value={orderBy}
              onChange={handleOrderByChange}
            />
            <form
              className='router-search-form-xs'
              onSubmit={(event) => {
                event.preventDefault();
                searchTokens();
              }}
            >
              <AppInput
                className='router-section-input'
                icon='search'
                fluid
                iconPosition='left'
                placeholder={t('token.search')}
                value={searchKeyword}
                loading={searching}
                onChange={handleKeywordChange}
              />
            </form>
          </div>
        }
      />

      <AppTable
        className='router-list-table'
        pagination={false}
        rowKey='id'
        dataSource={tokens
          .slice((activePage - 1) * ITEMS_PER_PAGE, activePage * ITEMS_PER_PAGE)
          .filter((token) => !token?.deleted)}
        scroll={{ x: 1200 }}
        locale={{ emptyText: t('common.no_data', '暂无数据') }}
        onRow={(token) => ({
          className: 'router-row-clickable',
          onClick: () =>
            navigate(`/token/${token.id}`, {
              state: {
                from: currentPagePath,
              },
            }),
        })}
        columns={[
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortToken('name');
                }}
              >
                {t('token.table.name')}
              </span>
            ),
            dataIndex: 'name',
            key: 'name',
            render: (value) => value || t('token.table.no_name'),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortToken('status');
                }}
              >
                {t('token.table.status')}
              </span>
            ),
            dataIndex: 'status',
            key: 'status',
            render: (value) => renderStatus(value, t),
          },
          {
            title: t('token.table.token'),
            dataIndex: 'key',
            key: 'token',
            render: (value, token) => (
              <span
                className='router-action-group'
                onClick={(event) => stopRowClick(event)}
              >
                <span
                  role='button'
                  tabIndex={0}
                  className='router-text-link'
                  onClick={() =>
                    navigate(`/token/${token.id}`, {
                      state: {
                        from: currentPagePath,
                      },
                    })
                  }
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      navigate(`/token/${token.id}`, {
                        state: {
                          from: currentPagePath,
                        },
                      });
                    }
                  }}
                >
                  {renderShortToken(value)}
                </span>
                <button
                  type='button'
                  className='router-icon-button'
                  onClick={async (event) => {
                    stopRowClick(event);
                    await onCopy('', token.key);
                  }}
                >
                  <AppIcon name='copy outline' />
                </button>
              </span>
            ),
          },
          {
            title: (
              <div className='router-table-header-with-control'>
                <span
                  className='router-sortable-header'
                  onClick={() => {
                    sortToken('usedAmount');
                  }}
                >
                  {t('token.table.used_amount')}
                </span>
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
            ),
            dataIndex: 'yycUsed',
            key: 'usedAmount',
            render: (value) =>
              formatDisplayAmountFromYYC(value, displayUnit, currencyIndex),
          },
          {
            title: (
              <div className='router-table-header-with-control'>
                <span
                  className='router-sortable-header'
                  onClick={() => {
                    sortToken('remainingAmount');
                  }}
                >
                  {t('token.table.remain_amount')}
                </span>
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
            ),
            dataIndex: 'yycRemaining',
            key: 'remainingAmount',
            render: (value, token) =>
              token.hasUnlimitedYYCLimit
                ? t('token.table.unlimited')
                : formatDisplayAmountFromYYC(value, displayUnit, currencyIndex),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortToken('createdTime');
                }}
              >
                {t('token.table.created_time')}
              </span>
            ),
            dataIndex: 'createdTime',
            key: 'createdTime',
            render: (value) => renderTimestamp(value),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortToken('expiredTime');
                }}
              >
                {t('token.table.expired_time')}
              </span>
            ),
            dataIndex: 'expiredTime',
            key: 'expiredTime',
            render: (value) =>
              value === -1 ? t('token.table.never_expire') : renderTimestamp(value),
          },
          {
            title: t('token.table.actions'),
            key: 'actions',
            render: (_, token) => {
              const realIdx = tokens.findIndex((item) => item?.id === token?.id);
              const openLinkOptionsWithHandlers = OPEN_LINK_OPTIONS.map((option) => ({
                ...option,
                onClick: async () => {
                  await onOpenLink(option.value, token.key);
                },
              }));

              return (
                <div
                  className='router-action-group'
                  onClick={(event) => stopRowClick(event)}
                >
                  <div className='router-action-group-tight'>
                    <AppButton
                      className='router-inline-button'
                      color='blue'
                      type='button'
                      onClick={() => onOpenLink('', token.key)}
                    >
                      {t('token.buttons.chat')}
                    </AppButton>
                    <AppMenuDropdown
                      className='router-token-action-menu'
                      menuClassName='router-token-action-menu-overlay'
                      items={openLinkOptionsWithHandlers.map((option) => ({
                        key: option.value,
                        label: option.text,
                        onClick: option.onClick,
                      }))}
                    >
                      <AppButton className='router-inline-button' type='button'>
                        <AppIcon name='right chevron' />
                      </AppButton>
                    </AppMenuDropdown>
                  </div>
                  <AppPopconfirm
                    title={`${t('token.buttons.confirm_delete')} ${token.name || ''}`}
                    onConfirm={() => {
                      manageToken(token.id, 'delete', realIdx);
                    }}
                    okText={t('common.confirm')}
                    cancelText={t('common.cancel')}
                  >
                    <AppButton className='router-inline-button' color='red' type='button'>
                      {t('token.buttons.delete')}
                    </AppButton>
                  </AppPopconfirm>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      manageToken(
                        token.id,
                        token.status === 1 ? 'disable' : 'enable',
                        realIdx,
                      );
                    }}
                  >
                    {token.status === 1
                      ? t('token.buttons.disable')
                      : t('token.buttons.enable')}
                  </AppButton>
                </div>
              );
            },
          },
        ]}
        footer={() => (
          <AppToolbar
            className='router-toolbar-compact'
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
    </>
  );
};

export default TokensTable;
