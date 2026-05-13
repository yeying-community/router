import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, showError, timestamp2string } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import UnitDropdown from './UnitDropdown';
import { buildBillingCurrencyIndex, buildDisplayUnitOptions, formatDisplayAmountFromYYC } from '../helpers/billing';
import { formatAmountWithUnit, renderText } from '../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppPagination,
  AppSelect,
  AppTable,
  AppTag,
  AppTooltip,
} from '../router-ui';

const STATUS_FILTER_ALL_VALUE = '__all_status__';
const BUSINESS_FLOW_HEADER_KEY = {
  topup: 'header.topup',
  'topup-reconcile': 'flow.topup_reconcile.title',
  package: 'header.package',
};

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
      return <AppTag className='router-tag'>{t('topup.external_topup_orders.status.created')}</AppTag>;
    case 'pending':
      return <AppTag color='blue' className='router-tag'>{t('topup.external_topup_orders.status.pending')}</AppTag>;
    case 'paid':
      return <AppTag color='teal' className='router-tag'>{t('topup.external_topup_orders.status.paid')}</AppTag>;
    case 'fulfilled':
      return <AppTag color='green' className='router-tag'>{t('topup.external_topup_orders.status.fulfilled')}</AppTag>;
    case 'failed':
      return <AppTag color='red' className='router-tag'>{t('topup.external_topup_orders.status.failed')}</AppTag>;
    case 'canceled':
      return <AppTag color='grey' className='router-tag'>{t('topup.external_topup_orders.status.canceled')}</AppTag>;
    default:
      return <AppTag color='grey' className='router-tag'>{readOnlyText(status)}</AppTag>;
  }
};

const renderPackageStatus = (status, t) => {
  switch (Number(status)) {
    case 1:
      return <AppTag color='green' className='router-tag'>{t('user.detail.package_status_types.active')}</AppTag>;
    case 2:
      return <AppTag color='grey' className='router-tag'>{t('user.detail.package_status_types.expired')}</AppTag>;
    case 3:
      return <AppTag color='blue' className='router-tag'>{t('user.detail.package_status_types.replaced')}</AppTag>;
    case 4:
      return <AppTag color='red' className='router-tag'>{t('user.detail.package_status_types.canceled')}</AppTag>;
    case 5:
      return <AppTag color='teal' className='router-tag'>{t('user.detail.package_status_types.pending')}</AppTag>;
    default:
      return <AppTag color='grey' className='router-tag'>{t('user.detail.package_status_types.unknown')}</AppTag>;
  }
};

const formatTopupBusinessType = (type, t) => {
  switch ((type || '').toString().trim()) {
    case 'balance_topup':
      return t('topup.business_type.balance_topup');
    case 'package_purchase':
      return t('topup.business_type.package_purchase');
    default:
      return readOnlyText(type);
  }
};

const renderReconcileStage = (row, t) => {
  const status = normalizeTopupStatus(row?.status);
  switch (status) {
    case 'created':
      return <AppTag className='router-tag'>{t('flow.topup_reconcile.stage.awaiting_payment')}</AppTag>;
    case 'pending':
      return <AppTag color='blue' className='router-tag'>{t('flow.topup_reconcile.stage.payment_processing')}</AppTag>;
    case 'paid':
      return <AppTag color='orange' className='router-tag'>{t('flow.topup_reconcile.stage.awaiting_fulfillment')}</AppTag>;
    case 'fulfilled':
      return <AppTag color='green' className='router-tag'>{t('flow.topup_reconcile.stage.done')}</AppTag>;
    case 'failed':
    case 'canceled':
      return <AppTag color='grey' className='router-tag'>{t('flow.topup_reconcile.stage.closed')}</AppTag>;
    default:
      return <AppTag color='grey' className='router-tag'>{readOnlyText(row?.status)}</AppTag>;
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
  const [refreshingRowID, setRefreshingRowID] = useState('');
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
      render: (row) => {
        const userId = readOnlyText(row.user_id || row.redeemed_by_user_id);
        return (
          <AppButton
            type='button'
            basic
            className='router-inline-button'
            onClick={(event) => {
              event.stopPropagation();
              if (userId === '-') {
                return;
              }
              navigate(`/admin/user/detail/${encodeURIComponent(userId)}`, {
                state: { from: currentPagePath },
              });
            }}
          >
            {readOnlyText(row.username || row.redeemed_by_username)}
          </AppButton>
        );
      },
    };
    const compactUserColumn = {
      key: 'username',
      label: t('user.table.username'),
      render: (row) => {
        const userId = readOnlyText(row.user_id || row.redeemed_by_user_id);
        return (
          <AppButton
            type='button'
            basic
            className='router-inline-button'
            onClick={(event) => {
              event.stopPropagation();
              if (userId === '-') {
                return;
              }
              navigate(`/admin/user/detail/${encodeURIComponent(userId)}`, {
                state: { from: currentPagePath },
              });
            }}
          >
            {readOnlyText(row.username || row.redeemed_by_username)}
          </AppButton>
        );
      },
    };

    if (kind === 'topup') {
      return {
        endpoint: '/api/v1/admin/flow/topup-orders',
        searchPlaceholder: t('flow.topup.search_placeholder'),
        emptyText: t('flow.topup.empty'),
        onRowClick: (row) => {
          const rowID = readOnlyText(row?.id);
          if (rowID === '-') {
            return;
          }
          navigate(`/admin/flow/topup/${encodeURIComponent(rowID)}`, {
            state: { from: currentPagePath },
          });
        },
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
            key: 'amount',
            label: t('topup.external_topup_orders.columns.amount'),
            render: (row) => {
              const amountValue = Number(
                row?.amount ?? row?.face_value_amount ?? 0,
              );
              const amountUnit = String(
                row?.currency || row?.face_value_unit || '',
              )
                .trim()
                .toUpperCase();
              return amountValue > 0 && amountUnit
                ? formatAmountWithUnit(amountValue, amountUnit, 6)
                : '-';
            },
          },
          {
            key: 'quota',
            label: (
              <div className='router-table-header-with-control'>
                <span>{t('topup.external_topup_orders.columns.quota')}</span>
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
            render: (row) =>
              formatDisplayAmountFromYYC(
                row?.yyc_value || 0,
                displayUnit,
                currencyIndex,
                { fractionDigits: 6, includeSymbol: false, yycMode: 'fixed' },
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

    if (kind === 'topup-reconcile') {
      return {
        endpoint: '/api/v1/admin/flow/topup-reconcile-records',
        searchPlaceholder: t('flow.topup_reconcile.search_placeholder'),
        emptyText: t('flow.topup_reconcile.empty'),
        onRowClick: (row) => {
          const rowID = readOnlyText(row?.id);
          if (rowID === '-') {
            return;
          }
          navigate(`/admin/flow/topup-reconcile/${encodeURIComponent(rowID)}`, {
            state: { from: currentPagePath },
          });
        },
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
          compactUserColumn,
          {
            key: 'stage',
            label: t('flow.topup_reconcile.columns.stage'),
            render: (row) => renderReconcileStage(row, t),
          },
          {
            key: 'status',
            label: t('topup.external_topup_orders.columns.status'),
            render: (row) => renderTopupStatus(row.status, t),
          },
          {
            key: 'business_type',
            label: t('topup.external_topup_orders.columns.business_type'),
            render: (row) => formatTopupBusinessType(row.business_type, t),
          },
          {
            key: 'amount',
            label: t('topup.external_topup_orders.columns.amount'),
            render: (row) =>
              Number(row.amount || 0) > 0
                ? `${row.currency || 'CNY'} ${Number(row.amount || 0).toFixed(2)}`
                : '-',
          },
          {
            key: 'order',
            label: t('flow.topup_reconcile.columns.order'),
            render: (row) => (
              <div>{renderText(readOnlyText(row.title || formatTopupBusinessType(row.business_type, t)), 28)}</div>
            ),
          },
          {
            key: 'message',
            label: t('flow.topup_reconcile.columns.message'),
            headerClassName: 'router-topup-reconcile-message-cell',
            cellClassName: 'router-topup-reconcile-message-cell',
            render: (row) => {
              const message = readOnlyText(row.status_message);
              if (message === '-') {
                return message;
              }
              return (
                <AppTooltip
                  title={
                    <div className='router-topup-reconcile-message-popup'>
                      {message}
                    </div>
                  }
                >
                  <div className='router-topup-reconcile-message-text'>
                    {message}
                  </div>
                </AppTooltip>
              );
            },
          },
          {
            key: 'updated_at',
            label: t('user.table.updated_at'),
            render: (row) => formatDateTime(row.updated_at),
          },
          {
            key: 'actions',
            label: t('redemption.table.actions'),
            collapsing: true,
            render: (row) => (
              <AppButton
                type='button'
                className='router-inline-button'
                loading={refreshingRowID === row.id}
                disabled={refreshingRowID === row.id}
                onClick={(event) => {
                  event.stopPropagation();
                  handleRefreshReconcileRow(row?.id);
                }}
              >
                {t('flow.topup_reconcile.actions.refresh')}
              </AppButton>
            ),
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
        onRowClick: (row) => {
          const rowID = readOnlyText(row?.id);
          if (rowID === '-') {
            return;
          }
          navigate(`/admin/flow/package/${encodeURIComponent(rowID)}`, {
            state: { from: currentPagePath },
          });
        },
        statusOptions: [
          { key: 'all', value: '', text: t('task.filters.status_all') },
          { key: '1', value: '1', text: t('user.detail.package_status_types.active') },
          { key: '2', value: '2', text: t('user.detail.package_status_types.expired') },
          { key: '3', value: '3', text: t('user.detail.package_status_types.replaced') },
          { key: '4', value: '4', text: t('user.detail.package_status_types.canceled') },
          { key: '5', value: '5', text: t('user.detail.package_status_types.pending') },
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
            key: 'amount',
            label: t('flow.package.columns.amount'),
            render: (row) => {
              const amount = Number(row?.amount || 0);
              const currency = readOnlyText(row?.currency);
              return amount > 0 && currency !== '-'
                ? formatAmountWithUnit(amount, currency, 6)
                : '-';
            },
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
      onRowClick: (row) => {
        const rowID = readOnlyText(row?.id);
        if (rowID === '-') {
          return;
        }
        navigate(`/admin/flow/redemption/${encodeURIComponent(rowID)}`, {
          state: { from: currentPagePath },
        });
      },
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
          label: (
            <div className='router-table-header-with-control'>
              <span>{t('topup.external_topup_orders.columns.quota')}</span>
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
          render: (row) => {
            const unit = (displayUnit || 'USD').toString().trim().toUpperCase();
            const amountText = formatDisplayAmountFromYYC(
              row.yyc_value || 0,
              unit,
              currencyIndex,
              { fractionDigits: 6, includeSymbol: false, yycMode: 'fixed' },
            );
            return amountText === '-' ? '-' : `${amountText} ${unit}`;
          },
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
      ],
    };
  }, [currencyIndex, currentPagePath, displayUnit, displayUnitOptions, kind, navigate, refreshingRowID, t]);

  const statusDropdownOptions = useMemo(
    () =>
      (Array.isArray(config.statusOptions) ? config.statusOptions : []).map((option) => {
        if ((option?.value || '') === '') {
          return {
            ...option,
            value: STATUS_FILTER_ALL_VALUE,
          };
        }
        return option;
      }),
    [config.statusOptions],
  );

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

  async function handleRefreshReconcileRow(rowID) {
    const normalizedRowID = (rowID || '').toString().trim();
    if (!normalizedRowID) {
      return;
    }
    setRefreshingRowID(normalizedRowID);
    try {
      const res = await API.post(
        `/api/v1/admin/flow/topup-reconcile-records/${encodeURIComponent(normalizedRowID)}/refresh`,
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('flow.messages.load_failed'));
        return;
      }
      loadItems(activePage, keyword, statusFilter).then();
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setRefreshingRowID('');
    }
  }

  return (
    <>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'flow', label: t('header.business_flow') },
          {
            key: kind,
            label: t(BUSINESS_FLOW_HEADER_KEY[kind] || 'header.business_flow'),
            active: true,
          },
        ]}
        title={t(BUSINESS_FLOW_HEADER_KEY[kind] || 'header.business_flow')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              loading={loading}
              disabled={loading}
              onClick={onRefresh}
            >
              {t('task.buttons.refresh')}
            </AppButton>
          </div>
        }
        query={
          <div className='router-list-toolbar-query router-list-toolbar-query-compact'>
            {config.statusOptions.length > 0 ? (
              <AppSelect
                className='router-section-dropdown router-flow-filter-dropdown router-dropdown-min-170'
                options={statusDropdownOptions}
                value={statusFilter === '' ? STATUS_FILTER_ALL_VALUE : statusFilter}
                onChange={(event, { value }) => {
                  const normalizedValue = (value || '').toString();
                  setStatusFilter(
                    normalizedValue === STATUS_FILTER_ALL_VALUE ? '' : normalizedValue,
                  );
                }}
              />
            ) : null}
            <div className='router-search-form-xs'>
              <AppInput
                className='router-section-input'
                placeholder={config.searchPlaceholder}
                value={keyword}
                onChange={(event, { value }) => {
                  setKeyword(value);
                }}
              />
            </div>
            <AppButton
              className='router-page-button'
              loading={loading}
              disabled={loading}
              onClick={onSearchSubmit}
            >
              {t('task.buttons.query')}
            </AppButton>
          </div>
        }
      />

      <div className={`router-table-scroll-x ${config.tableWrapperClassName || ''}`.trim()}>
        <AppTable
          className='router-hover-table router-list-table'
          pagination={false}
          rowKey={(row) => row.id || row.transaction_id || row.package_id}
          dataSource={items}
          locale={{ emptyText: config.emptyText }}
          onRow={(row) => ({
            onClick:
              typeof config.onRowClick === 'function'
                ? () => config.onRowClick(row)
                : undefined,
            style:
              typeof config.onRowClick === 'function'
                ? { cursor: 'pointer' }
                : undefined,
          })}
          columns={config.columns.map((column) => ({
            title: column.label,
            key: column.key,
            className: column.cellClassName || '',
            onHeaderCell: () => ({
              className: column.headerClassName || '',
            }),
            render: (_, row) => column.render(row),
          }))}
        />
      </div>

      <div className='router-pagination-wrap'>
        <AppPagination
          activePage={activePage}
          totalPages={totalPages}
          onPageChange={onPageChange}
        />
      </div>
    </>
  );
};

export default BusinessFlowTable;
