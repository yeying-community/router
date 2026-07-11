import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, timestamp2string, showError, showSuccess } from '../../helpers';
import {
  TOPUP_RECORD_COLUMN_WIDTHS,
  TOPUP_RECORD_TABLE_MIN_WIDTH,
  TOPUP_REDEMPTION_RECORD_TABLE_MIN_WIDTH,
} from '../../constants/tableWidthPresets';
import {
  AppButton,
  AppPagination,
  AppSection,
  AppTable,
  AppTag,
  AppTooltip,
} from '../../router-ui';
import {
  formatTopupBusinessType,
  formatTopupOrderStatusHint,
  normalizeRedemptionRecord,
  useTopUpWorkspace,
  renderTopupOrderStatus,
} from './shared.jsx';
import RedeemCodePage from './RedeemCodePage';

const PAGE_SIZE = 10;

const TopUpRecordsPage = ({ recordKey = 'topup', embedded = false }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { renderDisplayAmount } = useTopUpWorkspace();
  const isRedemptionRecord = recordKey === 'redeem';
  const isPackageRecord = recordKey === 'package';
  const isGiftRecord = recordKey === 'gift';
  const isPaymentRecord = recordKey === 'payment';
  const [orders, setOrders] = useState([]);
  const [ordersPage, setOrdersPage] = useState(1);
  const [ordersTotal, setOrdersTotal] = useState(0);
  const [loadingOrders, setLoadingOrders] = useState(false);
  const [refreshingOrderID, setRefreshingOrderID] = useState('');
  const [redemptionRecords, setRedemptionRecords] = useState([]);
  const [redemptionPage, setRedemptionPage] = useState(1);
  const [redemptionTotal, setRedemptionTotal] = useState(0);
  const [loadingRedemptionRecords, setLoadingRedemptionRecords] = useState(false);
  const [redeemModalOpen, setRedeemModalOpen] = useState(false);

  const currentBusinessType = useMemo(() => {
    if (recordKey === 'package') {
      return 'package_purchase';
    }
    if (recordKey === 'payment') {
      return '';
    }
    return 'balance_topup';
  }, [recordKey]);

  const loadOrders = useCallback(
    async (page = 1) => {
      setLoadingOrders(true);
      try {
        const res = await API.get('/api/v1/public/user/topup/orders', {
          params: {
            page,
            page_size: PAGE_SIZE,
            business_type: currentBusinessType,
            credit_origin: isGiftRecord
              ? 'gift'
              : recordKey === 'topup' || isPaymentRecord
                ? 'paid'
                : undefined,
          },
        });
        const { success, message, data } = res?.data || {};
        if (success) {
          setOrders(Array.isArray(data?.items) ? data.items : []);
          setOrdersPage(Number(data?.page || page) || 1);
          setOrdersTotal(Number(data?.total || 0) || 0);
          return;
        }
        showError(message || t('topup.external_topup.request_failed'));
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
      } finally {
        setLoadingOrders(false);
      }
    },
    [currentBusinessType, isGiftRecord, isPaymentRecord, recordKey, t],
  );

  const loadRedemptionRecords = useCallback(
    async (page = 1) => {
      setLoadingRedemptionRecords(true);
      try {
        const res = await API.get('/api/v1/public/user/topup/redemptions', {
          params: {
            page,
            page_size: PAGE_SIZE,
          },
        });
        const { success, message, data, meta } = res?.data || {};
        if (success) {
          const items = Array.isArray(data)
            ? data
            : Array.isArray(data?.items)
              ? data.items
              : [];
          setRedemptionRecords(
            items.map(normalizeRedemptionRecord).filter(Boolean),
          );
          setRedemptionPage(
            Number(data?.page || meta?.page || page) || 1,
          );
          setRedemptionTotal(Number(data?.total || meta?.total || 0) || 0);
          return;
        }
        showError(message || t('topup.redeem.request_failed'));
      } catch (error) {
        showError(error?.message || t('topup.redeem.request_failed'));
      } finally {
        setLoadingRedemptionRecords(false);
      }
    },
    [t],
  );

  const refreshCurrent = useCallback(async () => {
    if (isRedemptionRecord) {
      await loadRedemptionRecords(redemptionPage);
      return;
    }
    await loadOrders(ordersPage);
  }, [
    isRedemptionRecord,
    loadOrders,
    loadRedemptionRecords,
    ordersPage,
    redemptionPage,
  ]);

  useEffect(() => {
    if (isRedemptionRecord) {
      loadRedemptionRecords(redemptionPage).then();
      return;
    }
    loadOrders(ordersPage).then();
  }, [
    isRedemptionRecord,
    loadOrders,
    loadRedemptionRecords,
    ordersPage,
    redemptionPage,
  ]);

  useEffect(() => {
    setOrdersPage(1);
    setRedemptionPage(1);
  }, [recordKey]);

  const ordersTotalPages = Math.max(1, Math.ceil(ordersTotal / PAGE_SIZE));
  const redemptionTotalPages = Math.max(
    1,
    Math.ceil(redemptionTotal / PAGE_SIZE),
  );

  const refreshOrderStatus = useCallback(
    async (orderID) => {
      const normalizedOrderID = (orderID || '').trim();
      if (!normalizedOrderID) {
        return null;
      }
      setRefreshingOrderID(normalizedOrderID);
      try {
        const res = await API.post(
          `/api/v1/public/user/topup/orders/${normalizedOrderID}/refresh`,
        );
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('topup.external_topup.request_failed'));
          return null;
        }
        setOrders((previous) =>
          previous.map((item) =>
            item.id === normalizedOrderID ? { ...item, ...data } : item,
          ),
        );
        return data || null;
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
        return null;
      } finally {
        setRefreshingOrderID('');
      }
    },
    [t],
  );

  const continuePay = useCallback(
    async (order) => {
      const refreshedOrder = await refreshOrderStatus(order?.id);
      const targetOrder = refreshedOrder || order;
      if (!targetOrder) {
        return;
      }
      if (['paid', 'fulfilled'].includes(targetOrder.status)) {
        showSuccess(t('topup.records.order_paid'));
        loadOrders(ordersPage).then();
        return;
      }
      const redirectURL = (targetOrder.redirect_url || '').trim();
      if (redirectURL === '') {
        showError(t('topup.records.redirect_missing'));
        return;
      }
      const popup = window.open(redirectURL, '_blank');
      if (!popup) {
        showError(t('topup.external_topup.popup_blocked'));
      }
    },
    [loadOrders, ordersPage, refreshOrderStatus, t],
  );

  const manualRefreshOrder = useCallback(
    async (orderID) => {
      const refreshedOrder = await refreshOrderStatus(orderID);
      if (refreshedOrder && ['paid', 'fulfilled'].includes(refreshedOrder.status)) {
        showSuccess(t('topup.records.order_paid'));
      }
    },
    [refreshOrderStatus, t],
  );

  const cancelPay = useCallback(
    async (orderID) => {
      const normalizedOrderID = (orderID || '').trim();
      if (!normalizedOrderID) {
        return;
      }
      setRefreshingOrderID(normalizedOrderID);
      try {
        const res = await API.post(
          `/api/v1/public/user/topup/orders/${normalizedOrderID}/cancel`,
        );
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('topup.external_topup.request_failed'));
          return;
        }
        setOrders((previous) =>
          previous.map((item) =>
            item.id === normalizedOrderID ? { ...item, ...(data || {}) } : item,
          ),
        );
        showSuccess(t('topup.records.order_canceled'));
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
      } finally {
        setRefreshingOrderID('');
      }
    },
    [t],
  );

  const openOrderDetailPage = useCallback(
    (order) => {
      const normalizedOrderID = (order?.id || '').trim();
      if (!normalizedOrderID) {
        return;
      }
      const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
      const orderRecordKey = isGiftRecord
        ? 'gift'
        : order?.business_type === 'package_purchase'
          ? 'package'
          : 'topup';
      navigate(`/workspace/topup/orders/${encodeURIComponent(normalizedOrderID)}`, {
        state: {
          from: currentPagePath,
          recordKey: orderRecordKey,
        },
      });
    },
    [
      isGiftRecord,
      location.hash,
      location.pathname,
      location.search,
      navigate,
    ],
  );

  const actionButton = useMemo(() => {
    switch (recordKey) {
      case 'package':
        return {
          label: t('topup.record_nav.package'),
          onClick: () => navigate('/workspace/service/pricing'),
        };
      case 'redeem':
        return {
          label: t('topup.record_nav.redeem'),
          onClick: () => setRedeemModalOpen(true),
        };
      case 'gift':
        return null;
      case 'payment':
        return {
          label: t('topup.record_nav.package'),
          onClick: () => navigate('/workspace/service/pricing'),
        };
      case 'topup':
      default:
        return {
          label: t('topup.record_nav.topup'),
          onClick: () => navigate('/workspace/service/pricing'),
        };
    }
  }, [navigate, recordKey, t]);
  const redemptionColumns = useMemo(
    () => [
      {
        title: t('topup.redemption_records.columns.time'),
        dataIndex: 'created_at',
        key: 'created_at',
        className: 'router-table-col-datetime',
        width: TOPUP_RECORD_COLUMN_WIDTHS.time,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('topup.redemption_records.columns.amount'),
        dataIndex: 'chargeAmount',
        key: 'chargeAmount',
        width: TOPUP_RECORD_COLUMN_WIDTHS.amount,
        render: (value) =>
          value ? (
            <AppTag color='green' className='router-tag'>
              {renderDisplayAmount(value)}
            </AppTag>
          ) : (
            '-'
          ),
      },
      {
        title: t('topup.redemption_records.columns.redemption_code'),
        dataIndex: 'redemptionCode',
        key: 'redemptionCode',
        width: TOPUP_RECORD_COLUMN_WIDTHS.redemptionCode,
        ellipsis: true,
        render: (value) => value || '-',
      },
    ],
    [renderDisplayAmount, t],
  );
  const orderColumns = useMemo(
    () => [
      {
        title: t('topup.external_topup_orders.columns.time'),
        dataIndex: 'created_at',
        key: 'created_at',
        className: 'router-table-col-datetime',
        width: TOPUP_RECORD_COLUMN_WIDTHS.time,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('topup.external_topup_orders.columns.business_type'),
        dataIndex: 'business_type',
        key: 'business_type',
        className: 'router-table-col-type-narrow',
        width: TOPUP_RECORD_COLUMN_WIDTHS.businessType,
        render: (value) => formatTopupBusinessType(value, t),
      },
      {
        title: t('topup.external_topup_orders.columns.status'),
        dataIndex: 'status',
        key: 'status',
        className: 'router-table-col-status-compact',
        width: TOPUP_RECORD_COLUMN_WIDTHS.status,
        render: (value, order) => {
          const statusNode = renderTopupOrderStatus(value, t);
          const statusHint = order?.business_type !== 'package_purchase'
            ? formatTopupOrderStatusHint(value, t)
            : '';
          if (!statusHint) {
            return statusNode;
          }
          return (
            <AppTooltip title={statusHint}>
              <span className='router-help-trigger'>
                {statusNode}
              </span>
            </AppTooltip>
          );
        },
      },
      {
        title: t('topup.external_topup_orders.columns.amount'),
        dataIndex: 'amount',
        key: 'amount',
        width: TOPUP_RECORD_COLUMN_WIDTHS.amount,
        render: (_, order) =>
          order.amount > 0
            ? `${order.currency || 'CNY'} ${Number(order.amount || 0).toFixed(2)}`
            : order.quota > 0
              ? renderDisplayAmount(order.quota)
              : '-',
      },
      {
        title: isPaymentRecord
          ? t('topup.external_topup_orders.columns.name')
          : isPackageRecord
            ? t('topup.external_topup_orders.columns.package_name')
            : t('topup.external_topup_orders.columns.quota'),
        dataIndex: isPackageRecord ? 'package_name' : 'quota',
        key: isPaymentRecord
          ? 'name'
          : isPackageRecord
            ? 'package_name'
            : 'quota',
        width: TOPUP_RECORD_COLUMN_WIDTHS.quotaOrPackage,
        ellipsis: true,
        render: (_, order) => {
          if (isPaymentRecord) {
            return order.business_type === 'package_purchase'
              ? order.package_name || order.title || '-'
              : order.quota > 0
                ? renderDisplayAmount(order.quota)
                : order.title || '-';
          }
          return isPackageRecord
            ? order.package_name || '-'
            : order.quota > 0
              ? renderDisplayAmount(order.quota)
              : '-';
        },
      },
      isGiftRecord
        ? null
        : {
        title: t('topup.external_topup_orders.columns.action'),
        key: 'action',
        className: 'router-table-col-actions-token',
        width: TOPUP_RECORD_COLUMN_WIDTHS.actions,
        render: (_, order) => (
          <div className='router-action-group-tight router-table-actions-wide'>
            <AppButton
              className='router-inline-button'
              onClick={(event) => {
                event.stopPropagation();
                manualRefreshOrder(order.id);
              }}
              loading={refreshingOrderID === order.id}
              disabled={refreshingOrderID === order.id}
            >
              {t('topup.records.refresh_status')}
            </AppButton>
            {['created', 'pending'].includes(order.status) ? (
              <>
                <AppButton
                  className='router-inline-button'
                  color='blue'
                  onClick={(event) => {
                    event.stopPropagation();
                    continuePay(order);
                  }}
                  loading={refreshingOrderID === order.id}
                  disabled={refreshingOrderID === order.id}
                >
                  {t('topup.records.continue_pay')}
                </AppButton>
                <AppButton
                  className='router-inline-button'
                  onClick={(event) => {
                    event.stopPropagation();
                    cancelPay(order.id);
                  }}
                  loading={refreshingOrderID === order.id}
                  disabled={refreshingOrderID === order.id}
                >
                  {t('topup.records.cancel_pay')}
                </AppButton>
              </>
            ) : null}
          </div>
        ),
        },
    ].filter(Boolean),
    [
      cancelPay,
      continuePay,
      formatTopupBusinessType,
      isGiftRecord,
      isPackageRecord,
      isPaymentRecord,
      manualRefreshOrder,
      refreshingOrderID,
      renderDisplayAmount,
      t,
    ],
  );

  const sectionTitle = isRedemptionRecord
    ? t('topup.redemption_records.title', '兑换记录')
    : isPaymentRecord
      ? t('topup.payment_history.title')
    : isPackageRecord
      ? t('topup.records.package_title', '套餐订单')
      : isGiftRecord
        ? t('topup.records.gift_title', '赠送记录')
        : t('topup.records.title', '充值订单');
  const shouldShowSectionExtra = !(embedded && isPaymentRecord);
  const sectionExtra = shouldShowSectionExtra ? (
    <>
      {isPaymentRecord ? (
        <AppButton
          className='router-section-button'
          onClick={() => navigate('/workspace/service/pricing')}
        >
          {t('topup.payment_history.back_to_pricing')}
        </AppButton>
      ) : null}
      {actionButton ? (
        <AppButton
          color='blue'
          className='router-section-button'
          onClick={actionButton.onClick}
        >
          {actionButton.label}
        </AppButton>
      ) : null}
      <AppButton
        className='router-section-button'
        onClick={refreshCurrent}
        loading={loadingOrders || loadingRedemptionRecords}
      >
        {t('topup.records.refresh')}
      </AppButton>
    </>
  ) : null;
  const recordsBody = isRedemptionRecord ? (
    <>
      <div className='router-table-scroll-x'>
        <AppTable
          className='router-list-table router-table-fit-page'
          rowKey={(log) =>
            log.id || log.trace_id || `${log.created_at}-${log.content}`
          }
          pagination={false}
          scroll={{ x: TOPUP_REDEMPTION_RECORD_TABLE_MIN_WIDTH }}
          loading={loadingRedemptionRecords}
          locale={{ emptyText: t('topup.redemption_records.empty') }}
          dataSource={redemptionRecords}
          columns={redemptionColumns}
        />
      </div>
      {redemptionTotalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <AppPagination
            className='router-section-pagination'
            activePage={redemptionPage}
            totalPages={redemptionTotalPages}
            onPageChange={(_, { activePage: nextActivePage }) => {
              setRedemptionPage(Number(nextActivePage) || 1);
            }}
          />
        </div>
      ) : null}
    </>
  ) : (
    <>
      <div className='router-table-scroll-x'>
        <AppTable
          className='router-list-table router-table-fit-page'
          rowKey='id'
          pagination={false}
          scroll={{ x: TOPUP_RECORD_TABLE_MIN_WIDTH }}
          loading={loadingOrders}
          locale={{ emptyText: t('topup.records.order_empty') }}
          dataSource={orders}
          columns={orderColumns}
          onRow={(order) => ({
            onClick: () => openOrderDetailPage(order),
            style: { cursor: 'pointer' },
          })}
        />
      </div>
      {ordersTotalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <AppPagination
            className='router-section-pagination'
            activePage={ordersPage}
            totalPages={ordersTotalPages}
            onPageChange={(_, { activePage: nextActivePage }) => {
              setOrdersPage(Number(nextActivePage) || 1);
            }}
          />
        </div>
      ) : null}
    </>
  );

  return (
    <>
      {embedded ? (
        <div className='router-topup-history-panel'>
          {sectionExtra ? (
            <div className='router-topup-history-toolbar'>{sectionExtra}</div>
          ) : null}
          {recordsBody}
        </div>
      ) : (
        <AppSection title={sectionTitle} extra={sectionExtra}>
          {recordsBody}
        </AppSection>
      )}
      <RedeemCodePage
        open={redeemModalOpen}
        onClose={() => setRedeemModalOpen(false)}
        onRedeemed={refreshCurrent}
      />
    </>
  );
};

export default TopUpRecordsPage;
