import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { Button, Card, Label, Modal, Pagination, Table } from 'semantic-ui-react';
import { API, timestamp2string, showError, showSuccess } from '../../helpers';
import {
  formatTopupBusinessType,
  formatTopupOrderStatusHint,
  normalizeRedemptionRecord,
  useTopUpWorkspace,
  renderTopupOrderStatus,
} from './shared.jsx';

const PAGE_SIZE = 10;

const TopUpRecordsPage = ({ recordKey = 'topup' }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { renderDisplayAmount } = useTopUpWorkspace();
  const isRedemptionRecord = recordKey === 'redeem';
  const isPackageRecord = recordKey === 'package';
  const [orders, setOrders] = useState([]);
  const [ordersPage, setOrdersPage] = useState(1);
  const [ordersTotal, setOrdersTotal] = useState(0);
  const [loadingOrders, setLoadingOrders] = useState(false);
  const [refreshingOrderID, setRefreshingOrderID] = useState('');
  const [orderDetailOpen, setOrderDetailOpen] = useState(false);
  const [orderDetailLoading, setOrderDetailLoading] = useState(false);
  const [activeOrderDetail, setActiveOrderDetail] = useState(null);
  const [redemptionRecords, setRedemptionRecords] = useState([]);
  const [redemptionPage, setRedemptionPage] = useState(1);
  const [redemptionTotal, setRedemptionTotal] = useState(0);
  const [loadingRedemptionRecords, setLoadingRedemptionRecords] = useState(false);

  const currentBusinessType = useMemo(() => {
    if (recordKey === 'package') {
      return 'package_purchase';
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
    [currentBusinessType, t],
  );

  const loadRedemptionRecords = useCallback(
    async (page = 1) => {
      setLoadingRedemptionRecords(true);
      try {
        const res = await API.get('/api/v1/public/log', {
          params: {
            page,
            type: 1,
          },
        });
        const { success, message, data, meta } = res?.data || {};
        if (success) {
          setRedemptionRecords(
            Array.isArray(data)
              ? data.map(normalizeRedemptionRecord).filter(Boolean)
              : [],
          );
          setRedemptionPage(Number(meta?.page || page) || 1);
          setRedemptionTotal(Number(meta?.total || 0) || 0);
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

  const openOrderDetail = useCallback(
    async (order) => {
      const normalizedOrderID = (order?.id || '').trim();
      if (!normalizedOrderID) {
        return;
      }
      setOrderDetailOpen(true);
      setActiveOrderDetail(order || null);
      setOrderDetailLoading(true);
      try {
        const res = await API.get(
          `/api/v1/public/user/topup/orders/${normalizedOrderID}`,
        );
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('topup.external_topup.request_failed'));
          return;
        }
        setActiveOrderDetail(data || order || null);
      } catch (error) {
        showError(error?.message || t('topup.external_topup.request_failed'));
      } finally {
        setOrderDetailLoading(false);
      }
    },
    [t],
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
          onClick: () => navigate('/workspace/topup?tab=balance'),
        };
      case 'topup':
      default:
        return {
          label: t('topup.record_nav.topup'),
          onClick: () => navigate('/workspace/topup?tab=balance'),
        };
    }
  }, [navigate, recordKey, t]);

  return (
    <Card fluid className='router-soft-card'>
      <Card.Content>
        <Card.Header className='router-card-header'>
          <div className='router-toolbar'>
            <div className='router-toolbar-start' />
            <div className='router-toolbar-end'>
              <Button
                primary
                className='router-section-button'
                onClick={actionButton.onClick}
              >
                {actionButton.label}
              </Button>
              <Button
                className='router-section-button'
                onClick={refreshCurrent}
                loading={loadingOrders || loadingRedemptionRecords}
              >
                {t('topup.records.refresh')}
              </Button>
            </div>
          </div>
        </Card.Header>

        {isRedemptionRecord ? (
          <>
            <Table basic='very' compact className='router-list-table'>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell width={3}>
                    {t('topup.redemption_records.columns.time')}
                  </Table.HeaderCell>
                  <Table.HeaderCell width={2}>
                    {t('topup.redemption_records.columns.amount')}
                  </Table.HeaderCell>
                  <Table.HeaderCell>
                    {t('topup.redemption_records.columns.detail')}
                  </Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {redemptionRecords.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan='3' className='router-text-muted'>
                      {loadingRedemptionRecords
                        ? t('common.loading')
                        : t('topup.redemption_records.empty')}
                    </Table.Cell>
                  </Table.Row>
                ) : (
                  redemptionRecords.map((log) => (
                    <Table.Row
                      key={log.trace_id || `${log.created_at}-${log.content}`}
                    >
                      <Table.Cell>
                        {log.created_at ? timestamp2string(log.created_at) : '-'}
                      </Table.Cell>
                      <Table.Cell>
                        {log.yycAmount ? (
                          <Label basic color='green' className='router-tag'>
                            {renderDisplayAmount(log.yycAmount)}
                          </Label>
                        ) : (
                          '-'
                        )}
                      </Table.Cell>
                      <Table.Cell>{log.content || '-'}</Table.Cell>
                    </Table.Row>
                  ))
                )}
              </Table.Body>
            </Table>
            {redemptionTotalPages > 1 ? (
              <div className='router-pagination-wrap-md'>
                <Pagination
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
            <Table basic='very' compact className='router-list-table'>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell width={3}>
                    {t('topup.external_topup_orders.columns.time')}
                  </Table.HeaderCell>
                  <Table.HeaderCell width={2}>
                    {t('topup.external_topup_orders.columns.business_type')}
                  </Table.HeaderCell>
                  <Table.HeaderCell width={2}>
                    {t('topup.external_topup_orders.columns.status')}
                  </Table.HeaderCell>
                  <Table.HeaderCell width={2}>
                    {t('topup.external_topup_orders.columns.amount')}
                  </Table.HeaderCell>
                  <Table.HeaderCell>
                    {isPackageRecord
                      ? t('topup.external_topup_orders.columns.package_name')
                      : t('topup.external_topup_orders.columns.detail')}
                  </Table.HeaderCell>
                  {!isPackageRecord ? (
                    <Table.HeaderCell width={3}>
                      {t('topup.external_topup_orders.columns.action')}
                    </Table.HeaderCell>
                  ) : null}
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {orders.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan={isPackageRecord ? '5' : '6'} className='router-text-muted'>
                      {loadingOrders
                        ? t('common.loading')
                        : t('topup.records.order_empty')}
                    </Table.Cell>
                  </Table.Row>
                ) : (
                  orders.map((order) => (
                    <Table.Row
                      key={order.id}
                      onClick={
                        isPackageRecord ? () => openOrderDetail(order) : undefined
                      }
                      style={isPackageRecord ? { cursor: 'pointer' } : undefined}
                    >
                      <Table.Cell>
                        {order.created_at ? timestamp2string(order.created_at) : '-'}
                      </Table.Cell>
                      <Table.Cell>
                        {formatTopupBusinessType(order.business_type, t)}
                      </Table.Cell>
                      <Table.Cell>
                        <div>{renderTopupOrderStatus(order.status, t)}</div>
                        {!isPackageRecord && formatTopupOrderStatusHint(order.status, t) ? (
                          <div className='router-text-muted' style={{ marginTop: '0.35rem' }}>
                            {formatTopupOrderStatusHint(order.status, t)}
                          </div>
                        ) : null}
                      </Table.Cell>
                      <Table.Cell>
                        {order.amount > 0
                          ? `${order.currency || 'CNY'} ${Number(order.amount || 0).toFixed(2)}`
                          : order.quota > 0
                            ? renderDisplayAmount(order.quota)
                            : '-'}
                      </Table.Cell>
                      <Table.Cell>
                        {isPackageRecord
                          ? order.package_name || '-'
                          : (
                            <>
                              <div>{order.title || order.package_name || order.id || '-'}</div>
                              <div className='router-text-muted'>
                                {order.transaction_id || order.provider_order_id || '-'}
                              </div>
                            </>
                          )}
                      </Table.Cell>
                      {!isPackageRecord ? (
                        <Table.Cell>
                          <Button
                            size='tiny'
                            basic
                            onClick={() => manualRefreshOrder(order.id)}
                            loading={refreshingOrderID === order.id}
                            disabled={refreshingOrderID === order.id}
                          >
                            {t('topup.records.refresh_status')}
                          </Button>
                          {['created', 'pending'].includes(order.status) ? (
                            <Button
                              size='tiny'
                              primary
                              onClick={() => continuePay(order)}
                              loading={refreshingOrderID === order.id}
                              disabled={refreshingOrderID === order.id}
                            >
                              {t('topup.records.continue_pay')}
                            </Button>
                          ) : null}
                        </Table.Cell>
                      ) : null}
                    </Table.Row>
                  ))
                )}
              </Table.Body>
            </Table>
            {ordersTotalPages > 1 ? (
              <div className='router-pagination-wrap-md'>
                <Pagination
                  className='router-section-pagination'
                  activePage={ordersPage}
                  totalPages={ordersTotalPages}
                  onPageChange={(_, { activePage: nextActivePage }) => {
                    setOrdersPage(Number(nextActivePage) || 1);
                  }}
                />
              </div>
            ) : null}
            {isPackageRecord ? (
              <Modal
                open={orderDetailOpen}
                onClose={() => {
                  setOrderDetailOpen(false);
                  setActiveOrderDetail(null);
                  setOrderDetailLoading(false);
                }}
                size='small'
              >
                <Modal.Header>
                  {t('topup.external_topup_orders.detail_title')}
                </Modal.Header>
                <Modal.Content>
                  <Table basic='very' compact='very'>
                    <Table.Body>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.columns.order_id')}
                        </Table.Cell>
                        <Table.Cell>{activeOrderDetail?.id || '-'}</Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.columns.package_name')}
                        </Table.Cell>
                        <Table.Cell>{activeOrderDetail?.package_name || '-'}</Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.columns.status')}
                        </Table.Cell>
                        <Table.Cell>
                          {activeOrderDetail ? renderTopupOrderStatus(activeOrderDetail?.status, t) : '-'}
                        </Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.fields.status_message')}
                        </Table.Cell>
                        <Table.Cell>{activeOrderDetail?.status_message || '-'}</Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.columns.amount')}
                        </Table.Cell>
                        <Table.Cell>
                          {activeOrderDetail?.amount > 0
                            ? `${activeOrderDetail?.currency || 'CNY'} ${Number(activeOrderDetail?.amount || 0).toFixed(2)}`
                            : activeOrderDetail?.quota > 0
                              ? renderDisplayAmount(activeOrderDetail?.quota)
                              : '-'}
                        </Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.fields.title')}
                        </Table.Cell>
                        <Table.Cell>{activeOrderDetail?.title || '-'}</Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.columns.transaction_id')}
                        </Table.Cell>
                        <Table.Cell>{activeOrderDetail?.transaction_id || '-'}</Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.fields.provider_order_id')}
                        </Table.Cell>
                        <Table.Cell>{activeOrderDetail?.provider_order_id || '-'}</Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('topup.external_topup_orders.columns.time')}
                        </Table.Cell>
                        <Table.Cell>
                          {activeOrderDetail?.created_at
                            ? timestamp2string(activeOrderDetail?.created_at)
                            : '-'}
                        </Table.Cell>
                      </Table.Row>
                      <Table.Row>
                        <Table.Cell width={5}>
                          {t('updated_at')}
                        </Table.Cell>
                        <Table.Cell>
                          {activeOrderDetail?.updated_at
                            ? timestamp2string(activeOrderDetail?.updated_at)
                            : '-'}
                        </Table.Cell>
                      </Table.Row>
                    </Table.Body>
                  </Table>
                </Modal.Content>
                <Modal.Actions>
                  <Button
                    className='router-section-button'
                    onClick={() => {
                      setOrderDetailOpen(false);
                      setActiveOrderDetail(null);
                      setOrderDetailLoading(false);
                    }}
                    loading={orderDetailLoading}
                  >
                    {t('common.cancel')}
                  </Button>
                </Modal.Actions>
              </Modal>
            ) : null}
          </>
        )}
      </Card.Content>
    </Card>
  );
};

export default TopUpRecordsPage;
