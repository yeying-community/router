import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { Breadcrumb, Button, Card, Popup, Table } from 'semantic-ui-react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import TopUpWorkspaceProvider from './provider.jsx';
import {
  formatTopupBusinessType,
  formatTopupOrderStatusHint,
  renderTopupOrderStatus,
  useTopUpWorkspace,
} from './shared.jsx';

const resolveRecordKeyFromBusinessType = (businessType = '') => {
  return String(businessType || '').trim() === 'package_purchase'
    ? 'package'
    : 'topup';
};

const normalizeRecordKey = (value = '') => {
  const normalized = String(value || '').trim();
  return normalized === 'package' ? 'package' : 'topup';
};

const TopUpOrderDetailInner = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { id } = useParams();
  const { renderDisplayAmount } = useTopUpWorkspace();
  const [loading, setLoading] = useState(false);
  const [order, setOrder] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const [canceling, setCanceling] = useState(false);

  const loadDetail = useCallback(async () => {
    const normalizedOrderID = String(id || '').trim();
    if (normalizedOrderID === '') {
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(
        `/api/v1/public/user/topup/orders/${encodeURIComponent(normalizedOrderID)}`,
      );
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.external_topup.request_failed'));
        return;
      }
      setOrder(data || null);
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  const recordKey = useMemo(() => {
    const stateRecordKey = normalizeRecordKey(location.state?.recordKey || '');
    if (location.state?.recordKey) {
      return stateRecordKey;
    }
    return resolveRecordKeyFromBusinessType(order?.business_type || '');
  }, [location.state?.recordKey, order?.business_type]);

  const listPath = useMemo(() => {
    const from = String(location.state?.from || '').trim();
    if (from.startsWith('/workspace/topup')) {
      return from;
    }
    return `/workspace/topup?tab=records&record=${recordKey}`;
  }, [location.state?.from, recordKey]);

  const refreshOrderStatus = useCallback(async () => {
    const normalizedOrderID = String(order?.id || '').trim();
    if (normalizedOrderID === '') {
      return null;
    }
    setRefreshing(true);
    try {
      const res = await API.post(
        `/api/v1/public/user/topup/orders/${encodeURIComponent(normalizedOrderID)}/refresh`,
      );
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.external_topup.request_failed'));
        return null;
      }
      setOrder(data || null);
      return data || null;
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
      return null;
    } finally {
      setRefreshing(false);
    }
  }, [order?.id, t]);

  const continuePay = useCallback(async () => {
    const refreshed = await refreshOrderStatus();
    const targetOrder = refreshed || order;
    if (!targetOrder) {
      return;
    }
    if (['paid', 'fulfilled'].includes(String(targetOrder.status || '').trim())) {
      showSuccess(t('topup.records.order_paid'));
      return;
    }
    const redirectURL = String(targetOrder.redirect_url || '').trim();
    if (redirectURL === '') {
      showError(t('topup.records.redirect_missing'));
      return;
    }
    const popup = window.open(redirectURL, '_blank');
    if (!popup) {
      showError(t('topup.external_topup.popup_blocked'));
    }
  }, [order, refreshOrderStatus, t]);

  const cancelPay = useCallback(async () => {
    const normalizedOrderID = String(order?.id || '').trim();
    if (normalizedOrderID === '') {
      return;
    }
    setCanceling(true);
    try {
      const res = await API.post(
        `/api/v1/public/user/topup/orders/${encodeURIComponent(normalizedOrderID)}/cancel`,
      );
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.external_topup.request_failed'));
        return;
      }
      setOrder(data || null);
      showSuccess(t('topup.records.order_canceled'));
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
    } finally {
      setCanceling(false);
    }
  }, [order?.id, t]);

  const statusHint = useMemo(
    () => formatTopupOrderStatusHint(order?.status, t),
    [order?.status, t],
  );

  const detailTitle =
    recordKey === 'package'
      ? t('topup.external_topup_orders.detail_title_package')
      : t('topup.external_topup_orders.detail_title_topup');

  const detailPathLabel =
    recordKey === 'package'
      ? t('topup.external_topup_orders.detail_path_package')
      : t('topup.external_topup_orders.detail_path_topup');

  const detailPathOrderID = String(order?.id || id || '').trim() || '-';

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-entity-detail-page'>
            <div className='router-entity-detail-breadcrumb'>
              <Breadcrumb size='small'>
                <Breadcrumb.Section link onClick={() => navigate(listPath)}>
                  {detailPathLabel}
                </Breadcrumb.Section>
                <Breadcrumb.Divider icon='right chevron' />
                <Breadcrumb.Section active>
                  {detailPathOrderID}
                </Breadcrumb.Section>
              </Breadcrumb>
            </div>

            <div className='router-toolbar'>
              <div className='router-toolbar-start'>
                <div className='router-detail-section-title'>{detailTitle}</div>
              </div>
              <div className='router-toolbar-end'>
                <Button
                  className='router-section-button'
                  onClick={refreshOrderStatus}
                  loading={refreshing}
                  disabled={!order}
                >
                  {t('topup.records.refresh_status')}
                </Button>
                {['created', 'pending'].includes(String(order?.status || '').trim()) ? (
                  <>
                    <Button
                      primary
                      className='router-section-button'
                      onClick={continuePay}
                      loading={refreshing}
                      disabled={!order}
                    >
                      {t('topup.records.continue_pay')}
                    </Button>
                    <Button
                      className='router-section-button'
                      onClick={cancelPay}
                      loading={canceling}
                      disabled={!order}
                    >
                      {t('topup.records.cancel_pay')}
                    </Button>
                  </>
                ) : null}
              </div>
            </div>

            {loading ? (
              <div className='router-empty-cell'>{t('common.loading')}</div>
            ) : (
              <Table basic='very' compact='very'>
                <Table.Body>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.columns.order_id')}
                    </Table.Cell>
                    <Table.Cell>{order?.id || '-'}</Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.columns.business_type')}
                    </Table.Cell>
                    <Table.Cell>
                      {formatTopupBusinessType(order?.business_type, t)}
                    </Table.Cell>
                  </Table.Row>
                  {recordKey === 'package' ? (
                    <Table.Row>
                      <Table.Cell width={5}>
                        {t('topup.external_topup_orders.columns.package_name')}
                      </Table.Cell>
                      <Table.Cell>{order?.package_name || '-'}</Table.Cell>
                    </Table.Row>
                  ) : null}
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.columns.status')}
                    </Table.Cell>
                    <Table.Cell>
                      {statusHint ? (
                        <Popup
                          content={statusHint}
                          trigger={
                            <span style={{ display: 'inline-block', cursor: 'help' }}>
                              {renderTopupOrderStatus(order?.status, t)}
                            </span>
                          }
                        />
                      ) : (
                        renderTopupOrderStatus(order?.status, t)
                      )}
                    </Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.fields.status_message')}
                    </Table.Cell>
                    <Table.Cell>{order?.status_message || '-'}</Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.columns.amount')}
                    </Table.Cell>
                    <Table.Cell>
                      {Number(order?.amount || 0) > 0
                        ? `${order?.currency || 'CNY'} ${Number(order?.amount || 0).toFixed(2)}`
                        : Number(order?.quota || 0) > 0
                          ? renderDisplayAmount(order?.quota)
                          : '-'}
                    </Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.fields.title')}
                    </Table.Cell>
                    <Table.Cell>{order?.title || '-'}</Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.columns.transaction_id')}
                    </Table.Cell>
                    <Table.Cell>{order?.transaction_id || '-'}</Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.fields.provider_order_id')}
                    </Table.Cell>
                    <Table.Cell>{order?.provider_order_id || '-'}</Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.columns.time')}
                    </Table.Cell>
                    <Table.Cell>
                      {order?.created_at ? timestamp2string(order?.created_at) : '-'}
                    </Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell width={5}>
                      {t('topup.external_topup_orders.fields.updated_at')}
                    </Table.Cell>
                    <Table.Cell>
                      {order?.updated_at ? timestamp2string(order?.updated_at) : '-'}
                    </Table.Cell>
                  </Table.Row>
                </Table.Body>
              </Table>
            )}
          </div>
        </Card.Content>
      </Card>
    </div>
  );
};

const TopUpOrderDetail = () => {
  return (
    <TopUpWorkspaceProvider>
      <TopUpOrderDetailInner />
    </TopUpWorkspaceProvider>
  );
};

export default TopUpOrderDetail;
