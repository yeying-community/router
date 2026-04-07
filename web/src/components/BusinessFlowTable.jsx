import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Form, Label, Pagination, Table } from 'semantic-ui-react';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, showError, timestamp2string } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import UnitDropdown from './UnitDropdown';
import { buildBillingCurrencyIndex, buildDisplayUnitOptions, formatDisplayAmountFromYYC } from '../helpers/billing';
import { formatAmountWithUnit, formatYYCValue, renderText } from '../helpers/render';

const readOnlyText = (value) => {
  const normalized = (value || '').toString().trim();
  return normalized || '-';
};

const formatDateTime = (value) => {
  const numericValue = Number(value || 0);
  if (!Number.isFinite(numericValue) || numericValue <= 0) {
    return '-';
  }
  return timestamp2string(numericValue);
};

const normalizeTopupStatus = (value) =>
  (value || '').toString().trim().toLowerCase();

const renderTopupStatus = (status, t) => {
  switch (normalizeTopupStatus(status)) {
    case 'created':
      return <Label basic className='router-tag'>{t('topup.external_topup_orders.status.created')}</Label>;
    case 'pending':
      return <Label basic color='blue' className='router-tag'>{t('topup.external_topup_orders.status.pending')}</Label>;
    case 'paid':
      return <Label basic color='teal' className='router-tag'>{t('topup.external_topup_orders.status.paid')}</Label>;
    case 'fulfilled':
      return <Label basic color='green' className='router-tag'>{t('topup.external_topup_orders.status.fulfilled')}</Label>;
    case 'failed':
      return <Label basic color='red' className='router-tag'>{t('topup.external_topup_orders.status.failed')}</Label>;
    case 'canceled':
      return <Label basic color='grey' className='router-tag'>{t('topup.external_topup_orders.status.canceled')}</Label>;
    default:
      return <Label basic color='grey' className='router-tag'>{readOnlyText(status)}</Label>;
  }
};

const renderPackageStatus = (status, t) => {
  switch (Number(status)) {
    case 1:
      return <Label basic color='green' className='router-tag'>{t('user.detail.package_status_types.active')}</Label>;
    case 2:
      return <Label basic color='grey' className='router-tag'>{t('user.detail.package_status_types.expired')}</Label>;
    case 3:
      return <Label basic color='blue' className='router-tag'>{t('user.detail.package_status_types.replaced')}</Label>;
    case 4:
      return <Label basic color='red' className='router-tag'>{t('user.detail.package_status_types.canceled')}</Label>;
    default:
      return <Label basic color='grey' className='router-tag'>{t('user.detail.package_status_types.unknown')}</Label>;
  }
};

const BusinessFlowTable = ({ kind }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );

  const currentPagePath = useMemo(
    () => `${location.pathname}${location.search}${location.hash}`,
    [location.hash, location.pathname, location.search],
  );

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'yyc-first' }),
    [currencyIndex],
  );

  const config = useMemo(() => {
    const commonUserColumn = {
      key: 'username',
      label: t('user.table.username'),
      render: (row) => (
        <div>
          <Button
            type='button'
            basic
            compact
            size='mini'
            className='router-inline-button'
            onClick={() => {
              const userId = readOnlyText(row.user_id || row.redeemed_by_user_id);
              if (userId === '-') {
                return;
              }
              navigate(`/admin/user/detail/${encodeURIComponent(userId)}`, {
                state: { from: currentPagePath },
              });
            }}
          >
            {readOnlyText(row.username || row.redeemed_by_username)}
          </Button>
          <div className='router-text-muted'>
            {readOnlyText(row.user_id || row.redeemed_by_user_id)}
          </div>
        </div>
      ),
    };
    const compactUserColumn = {
      key: 'username',
      label: t('user.table.username'),
      render: (row) => {
        const userId = readOnlyText(row.user_id || row.redeemed_by_user_id);
        return (
          <Button
            type='button'
            basic
            compact
            size='mini'
            className='router-inline-button'
            onClick={() => {
              if (userId === '-') {
                return;
              }
              navigate(`/admin/user/detail/${encodeURIComponent(userId)}`, {
                state: { from: currentPagePath },
              });
            }}
          >
            {readOnlyText(row.username || row.redeemed_by_username)}
          </Button>
        );
      },
    };

    if (kind === 'topup') {
      return {
        endpoint: '/api/v1/admin/flow/topup-orders',
        searchPlaceholder: t('flow.topup.search_placeholder'),
        emptyText: t('flow.topup.empty'),
        statusOptions: [
          { key: 'all', value: '', text: t('task.filters.status_all') },
          { key: 'created', value: 'created', text: t('topup.external_topup_orders.status.created') },
          { key: 'pending', value: 'pending', text: t('topup.external_topup_orders.status.pending') },
          { key: 'paid', value: 'paid', text: t('topup.external_topup_orders.status.paid') },
          { key: 'fulfilled', value: 'fulfilled', text: t('topup.external_topup_orders.status.fulfilled') },
          { key: 'failed', value: 'failed', text: t('topup.external_topup_orders.status.failed') },
          { key: 'canceled', value: 'canceled', text: t('topup.external_topup_orders.status.canceled') },
        ],
        columns: [
          commonUserColumn,
          {
            key: 'status',
            label: t('topup.external_topup_orders.columns.status'),
            render: (row) => renderTopupStatus(row.status, t),
          },
          {
            key: 'source',
            label: t('flow.topup.columns.source'),
            render: (row) => (
              <div>
                <div>{readOnlyText(row.provider_name || row.source)}</div>
                <div className='router-text-muted'>{readOnlyText(row.source)}</div>
              </div>
            ),
          },
          {
            key: 'redemption',
            label: t('flow.topup.columns.redemption'),
            render: (row) => (
              <div>
                <div>{readOnlyText(row.redemption_name)}</div>
                <div className='router-text-muted'>
                  {readOnlyText(row.group_name || row.group_id)}
                </div>
              </div>
            ),
          },
          {
            key: 'amount',
            label: t('redemption.table.face_value'),
            render: (row) => (
              <div>
                <div>
                  {row.face_value_unit
                    ? formatAmountWithUnit(row.face_value_amount, row.face_value_unit, 6)
                    : '-'}
                </div>
                <div className='router-text-muted'>{formatYYCValue(row.yyc_value || 0)}</div>
              </div>
            ),
          },
          {
            key: 'order',
            label: t('flow.topup.columns.order'),
            render: (row) => (
              <div>
                <div>{renderText(readOnlyText(row.transaction_id), 20)}</div>
                <div className='router-text-muted'>{renderText(readOnlyText(row.id), 16)}</div>
              </div>
            ),
          },
          {
            key: 'created_at',
            label: t('user.table.created_at'),
            render: (row) => formatDateTime(row.created_at),
          },
          {
            key: 'updated_at',
            label: t('user.table.updated_at'),
            render: (row) => formatDateTime(row.updated_at),
          },
        ],
      };
    }

    if (kind === 'package') {
      return {
        endpoint: '/api/v1/admin/flow/package-records',
        tableWrapperClassName: 'router-business-flow-package-scroll',
        searchPlaceholder: t('flow.package.search_placeholder'),
        emptyText: t('flow.package.empty'),
        statusOptions: [
          { key: 'all', value: '', text: t('task.filters.status_all') },
          { key: '1', value: '1', text: t('user.detail.package_status_types.active') },
          { key: '2', value: '2', text: t('user.detail.package_status_types.expired') },
          { key: '3', value: '3', text: t('user.detail.package_status_types.replaced') },
          { key: '4', value: '4', text: t('user.detail.package_status_types.canceled') },
        ],
        columns: [
          compactUserColumn,
          {
            key: 'package_name',
            label: t('user.detail.package_name'),
            render: (row) => readOnlyText(row.package_name),
          },
          {
            key: 'group',
            label: t('user.detail.package_group'),
            render: (row) => readOnlyText(row.group_name || row.group_id),
          },
          {
            key: 'daily_quota_limit',
            label: (
              <div className='router-table-header-with-control'>
                <span>{t('user.detail.package_daily_limit')}</span>
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
            render: (row) => (
              Number(row.daily_quota_limit || 0) > 0
                ? formatDisplayAmountFromYYC(
                    row.daily_quota_limit,
                    displayUnit,
                    currencyIndex,
                    { fractionDigits: 6, includeSymbol: false, yycMode: 'fixed' },
                  )
                : t('common.unlimited')
            ),
          },
          {
            key: 'package_emergency_quota_limit',
            label: (
              <div className='router-table-header-with-control'>
                <span>{t('user.detail.package_emergency_limit')}</span>
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
            render: (row) => formatDisplayAmountFromYYC(
              row.package_emergency_quota_limit || 0,
              displayUnit,
              currencyIndex,
              { fractionDigits: 6, includeSymbol: false, yycMode: 'fixed' },
            ),
          },
          {
            key: 'status',
            label: t('user.detail.package_status'),
            render: (row) => renderPackageStatus(row.status, t),
          },
          {
            key: 'started_at',
            label: t('user.detail.package_started_at'),
            render: (row) => formatDateTime(row.started_at),
          },
          {
            key: 'expires_at',
            label: t('user.detail.package_expires_at'),
            render: (row) => (
              Number(row.expires_at || 0) > 0 ? formatDateTime(row.expires_at) : t('common.unlimited')
            ),
          },
          {
            key: 'updated_at',
            label: t('user.table.updated_at'),
            render: (row) => formatDateTime(row.updated_at),
          },
        ],
      };
    }

    return {
      endpoint: '/api/v1/admin/flow/redemption-records',
      searchPlaceholder: t('flow.redemption.search_placeholder'),
      emptyText: t('flow.redemption.empty'),
      statusOptions: [],
      columns: [
        commonUserColumn,
        {
          key: 'name',
          label: t('redemption.table.name'),
          render: (row) => readOnlyText(row.name),
        },
        {
          key: 'group',
          label: t('redemption.table.group'),
          render: (row) => readOnlyText(row.group_name || row.group_id),
        },
        {
          key: 'face_value',
          label: t('redemption.table.face_value'),
          render: (row) => formatAmountWithUnit(row.face_value_amount, row.face_value_unit, 6),
        },
        {
          key: 'credited_yyc',
          label: t('redemption.table.credited_yyc'),
          render: (row) => formatYYCValue(row.yyc_value || 0),
        },
        {
          key: 'redeemed_time',
          label: t('redemption.table.redeemed_time'),
          render: (row) => formatDateTime(row.redeemed_time),
        },
        {
          key: 'created_time',
          label: t('redemption.table.created_time'),
          render: (row) => formatDateTime(row.created_time),
        },
        {
          key: 'actions',
          label: t('redemption.table.actions'),
          collapsing: true,
          render: (row) => (
            <Button
              type='button'
              className='router-inline-button'
              onClick={() => {
                navigate(`/admin/redemption/${encodeURIComponent(row.id)}`, {
                  state: { from: currentPagePath },
                });
              }}
            >
              {t('task.buttons.view')}
            </Button>
          ),
        },
      ],
    };
  }, [currencyIndex, currentPagePath, displayUnit, displayUnitOptions, kind, navigate, t]);

  const loadCurrencyCatalog = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, data } = res.data || {};
      if (!success) {
        return;
      }
      const next = buildBillingCurrencyIndex(Array.isArray(data) ? data : [], {
        activeOnly: true,
        placeholderCodes: ['USD', 'CNY'],
      });
      setCurrencyIndex(next);
      if (!next[displayUnit]) {
        setDisplayUnit(next.USD ? 'USD' : Object.keys(next)[0] || 'USD');
      }
    } catch {
      // Keep the placeholder index when the catalog cannot be loaded.
    }
  }, [displayUnit]);

  const totalPages = useMemo(
    () => Math.max(Math.ceil(totalCount / ITEMS_PER_PAGE), 1),
    [totalCount],
  );

  const loadItems = useCallback(
    async (page = 1, nextKeyword = '', nextStatus = '') => {
      setLoading(true);
      try {
        const res = await API.get(config.endpoint, {
          params: {
            page,
            page_size: ITEMS_PER_PAGE,
            keyword: (nextKeyword || '').toString().trim(),
            status: (nextStatus || '').toString().trim(),
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('flow.messages.load_failed'));
          return;
        }
        setItems(Array.isArray(data?.items) ? data.items : []);
        setTotalCount(Number(data?.total || 0));
        setActivePage(Number(data?.page || page || 1));
      } catch (error) {
        showError(error?.message || error);
      } finally {
        setLoading(false);
      }
    },
    [config.endpoint, t],
  );

  useEffect(() => {
    loadCurrencyCatalog().then();
  }, [loadCurrencyCatalog]);

  useEffect(() => {
    setKeyword('');
    setStatusFilter('');
    setItems([]);
    setTotalCount(0);
    setActivePage(1);
    loadItems(1, '', '').then();
  }, [kind, loadItems]);

  const onSearchSubmit = useCallback(() => {
    loadItems(1, keyword, statusFilter).then();
  }, [keyword, statusFilter, loadItems]);

  const onRefresh = useCallback(() => {
    loadItems(activePage, keyword, statusFilter).then();
  }, [activePage, keyword, statusFilter, loadItems]);

  const onPageChange = useCallback(
    (e, { activePage: nextPage }) => {
      loadItems(Number(nextPage) || 1, keyword, statusFilter).then();
    },
    [keyword, statusFilter, loadItems],
  );

  return (
    <>
      <div className='router-toolbar router-block-gap-sm'>
        <div className='router-toolbar-start'>
          <Button
            className='router-page-button'
            loading={loading}
            disabled={loading}
            onClick={onRefresh}
          >
            {t('task.buttons.refresh')}
          </Button>
        </div>
        <div className='router-toolbar-end'>
          {config.statusOptions.length > 0 ? (
            <Dropdown
              className='router-section-dropdown router-dropdown-min-170'
              selection
              options={config.statusOptions}
              value={statusFilter}
              onChange={(event, { value }) => {
                setStatusFilter((value || '').toString());
              }}
            />
          ) : null}
          <Form
            className='router-search-form-xs'
            onSubmit={(event) => {
              event.preventDefault();
              onSearchSubmit();
            }}
          >
            <Form.Input
              className='router-section-input'
              placeholder={config.searchPlaceholder}
              value={keyword}
              onChange={(event, { value }) => {
                setKeyword(value);
              }}
            />
          </Form>
          <Button
            className='router-page-button'
            loading={loading}
            disabled={loading}
            onClick={onSearchSubmit}
          >
            {t('task.buttons.query')}
          </Button>
        </div>
      </div>

      <div className={`router-table-scroll-x ${config.tableWrapperClassName || ''}`.trim()}>
        <Table basic='very' compact className='router-hover-table router-list-table'>
          <Table.Header>
            <Table.Row>
              {config.columns.map((column) => (
                <Table.HeaderCell key={column.key} collapsing={column.collapsing === true}>
                  {column.label}
                </Table.HeaderCell>
              ))}
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {items.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan={config.columns.length} className='router-table-empty-cell'>
                  {config.emptyText}
                </Table.Cell>
              </Table.Row>
            ) : (
              items.map((row) => (
                <Table.Row key={row.id || row.transaction_id || row.package_id}>
                  {config.columns.map((column) => (
                    <Table.Cell key={column.key} collapsing={column.collapsing === true}>
                      {column.render(row)}
                    </Table.Cell>
                  ))}
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>

      <div className='table-footer'>
        <Pagination
          activePage={activePage}
          totalPages={totalPages}
          onPageChange={onPageChange}
        />
      </div>
    </>
  );
};

export default BusinessFlowTable;
