import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import {
  AppButton,
  AppDetailSection,
  AppFilterHeader,
  AppIcon,
  AppTag,
} from '../../router-ui';

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

const SYNCABLE_TOPUP_RECONCILE_STATUSES = new Set(['created', 'pending', 'paid']);

const renderTopupStatus = (status, t) => {
  switch (normalizeTopupStatus(status)) {
    case 'created':
      return (
        <AppTag className='router-tag'>
          {t('topup.external_topup_orders.status.created')}
        </AppTag>
      );
    case 'pending':
      return (
        <AppTag color='blue' className='router-tag'>
          {t('topup.external_topup_orders.status.pending')}
        </AppTag>
      );
    case 'paid':
      return (
        <AppTag color='teal' className='router-tag'>
          {t('topup.external_topup_orders.status.paid')}
        </AppTag>
      );
    case 'fulfilled':
      return (
        <AppTag color='green' className='router-tag'>
          {t('topup.external_topup_orders.status.fulfilled')}
        </AppTag>
      );
    case 'failed':
      return (
        <AppTag color='red' className='router-tag'>
          {t('topup.external_topup_orders.status.failed')}
        </AppTag>
      );
    case 'canceled':
      return (
        <AppTag color='grey' className='router-tag'>
          {t('topup.external_topup_orders.status.canceled')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='grey' className='router-tag'>
          {readOnlyText(status)}
        </AppTag>
      );
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
      return (
        <AppTag className='router-tag'>
          {t('flow.topup_reconcile.stage.awaiting_payment')}
        </AppTag>
      );
    case 'pending':
      return (
        <AppTag color='blue' className='router-tag'>
          {t('flow.topup_reconcile.stage.payment_processing')}
        </AppTag>
      );
    case 'paid':
      return (
        <AppTag color='orange' className='router-tag'>
          {t('flow.topup_reconcile.stage.awaiting_fulfillment')}
        </AppTag>
      );
    case 'fulfilled':
      return (
        <AppTag color='green' className='router-tag'>
          {t('flow.topup_reconcile.stage.done')}
        </AppTag>
      );
    case 'failed':
    case 'canceled':
      return (
        <AppTag color='grey' className='router-tag'>
          {t('flow.topup_reconcile.stage.closed')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='grey' className='router-tag'>
          {readOnlyText(row?.status)}
        </AppTag>
      );
  }
};

const formatAmount = (row) =>
  Number(row?.amount || 0) > 0
    ? `${readOnlyText(row?.currency || 'CNY')} ${Number(row?.amount || 0).toFixed(2)}`
    : '-';

const resolveListPath = (stateFrom) => {
  if (typeof stateFrom !== 'string') {
    return '/admin/user';
  }
  const normalized = stateFrom.trim();
  if (!normalized.startsWith('/')) {
    return '/admin/user';
  }
  if (normalized.startsWith('/admin/user/detail/')) {
    const cleanPath = normalized.split('?')[0].split('#')[0];
    const segments = cleanPath.split('/').filter(Boolean);
    if (segments.length >= 4) {
      return `/admin/user/detail/${segments[3]}`;
    }
  }
  if (normalized.startsWith('/admin/topup/payment/')) {
    return '/admin/user';
  }
  return normalized || '/admin/user';
};

const PaymentRecordDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { id, paymentId } = useParams();
  const orderID = (paymentId || id || '').toString().trim();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [order, setOrder] = useState(null);

  const listPath = useMemo(
    () => resolveListPath(location.state?.from),
    [location.state?.from],
  );
  const fromUserDetail = listPath.startsWith('/admin/user/detail/');
  const listLabel = useMemo(() => {
    if (fromUserDetail) {
      return t('topup.payment_history.title');
    }
    return t('flow.topup_reconcile.title');
  }, [fromUserDetail, listPath, t]);
  const breadcrumbItems = useMemo(() => {
    if (fromUserDetail) {
      return [
        { key: 'admin', label: t('header.admin_workspace') },
        {
          key: 'user-current',
          label: readOnlyText(order?.username || order?.user_id),
          onClick: () => navigate(listPath),
        },
        {
          key: 'payment-record-current',
          label: readOnlyText(order?.id || id),
          active: true,
        },
      ];
    }
    return [
      { key: 'admin', label: t('header.admin_workspace') },
      { key: 'business', label: t('header.business_operation') },
      {
        key: 'flow-topup-reconcile-list',
        label: listLabel,
        onClick: () => navigate(listPath),
      },
      {
        key: 'flow-topup-reconcile-current',
        label: readOnlyText(order?.id || id),
        active: true,
      },
    ];
  }, [fromUserDetail, id, listLabel, listPath, navigate, order?.id, order?.user_id, order?.username, t]);
  const canSyncPaymentStatus = SYNCABLE_TOPUP_RECONCILE_STATUSES.has(
    normalizeTopupStatus(order?.status),
  );

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(
        `/api/v1/admin/flow/topup-reconcile-records/${encodeURIComponent(orderID)}`,
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('flow.topup_reconcile.detail.messages.load_failed'));
        return;
      }
      setOrder(data || null);
    } catch (error) {
      showError(error?.message || t('flow.topup_reconcile.detail.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [orderID, t]);

  const handleRefresh = useCallback(async () => {
    if (!orderID) {
      return;
    }
    setRefreshing(true);
    try {
      const res = await API.post(
        `/api/v1/admin/flow/topup-reconcile-records/${encodeURIComponent(orderID)}/refresh`,
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('flow.topup_reconcile.detail.messages.refresh_failed'));
        return;
      }
      setOrder(data || null);
    } catch (error) {
      showError(error?.message || t('flow.topup_reconcile.detail.messages.refresh_failed'));
    } finally {
      setRefreshing(false);
    }
  }, [orderID, t]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={breadcrumbItems}
        title={listLabel}
      />
      <div className='router-entity-detail-page'>
        <AppDetailSection
          title={t('common.basic_info')}
          titleTag='div'
          headerEnd={
            <AppButton
              className='router-page-button'
              onClick={loadDetail}
              loading={loading}
              disabled={loading}
            >
              {t('task.buttons.refresh')}
            </AppButton>
          }
        >

              {loading ? (
                <div className='router-empty-cell'>{t('common.loading')}</div>
              ) : (
                <div className='router-detail-grid'>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.id')}
                    </div>
                    <pre className='router-detail-value router-monospace-value'>
                      {readOnlyText(order?.id || id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.user')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(order?.username || order?.user_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.business_type')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatTopupBusinessType(order?.business_type, t)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.amount')}
                    </div>
                    <pre className='router-detail-value'>{formatAmount(order)}</pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.stage')}
                    </div>
                    <div className='router-detail-value'>
                      {renderReconcileStage(order, t)}
                    </div>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.status')}
                    </div>
                    <div className='router-detail-value router-action-group-tight'>
                      {renderTopupStatus(order?.status, t)}
                      {canSyncPaymentStatus ? (
                        <AppButton
                          className='router-inline-button'
                          onClick={handleRefresh}
                          loading={refreshing}
                          disabled={refreshing}
                        >
                          {t('flow.topup_reconcile.actions.refresh')}
                        </AppButton>
                      ) : null}
                    </div>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.title')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(order?.title)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.source')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(order?.provider_name || order?.source)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.transaction_id')}
                    </div>
                    <pre className='router-detail-value router-monospace-value'>
                      {readOnlyText(order?.transaction_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.provider_order_id')}
                    </div>
                    <pre className='router-detail-value router-monospace-value'>
                      {readOnlyText(order?.provider_order_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.created_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(order?.created_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.updated_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(order?.updated_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.paid_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(order?.paid_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.redeemed_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(order?.redeemed_at)}
                    </pre>
                  </div>
                </div>
              )}
        </AppDetailSection>

        <AppDetailSection
          title={t('flow.topup_reconcile.detail.sections.message')}
          titleTag='div'
        >
              <pre className='router-detail-pre'>
                {readOnlyText(order?.status_message)}
              </pre>
        </AppDetailSection>
      </div>
    </div>
  );
};

export default PaymentRecordDetail;
