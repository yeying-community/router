import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import {
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

const formatAmount = (row) =>
  Number(row?.amount || 0) > 0
    ? `${readOnlyText(row?.currency)} ${Number(row?.amount || 0).toFixed(6)}`
    : '-';

const formatChargeAmount = (value) => {
  const numericValue = Number(value || 0);
  if (!Number.isFinite(numericValue)) {
    return '-';
  }
  return `${numericValue.toFixed(6)} YYC`;
};

const normalizeTopupStatus = (value) =>
  (value || '').toString().trim().toLowerCase();

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

const resolveListPath = (stateFrom) => {
  if (typeof stateFrom !== 'string') {
    return '/admin/flow/topup';
  }
  const normalized = stateFrom.trim();
  if (!normalized.startsWith('/')) {
    return '/admin/flow/topup';
  }
  if (normalized.startsWith('/admin/flow/topup/')) {
    return '/admin/flow/topup';
  }
  return normalized || '/admin/flow/topup';
};

const TopupDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { id } = useParams();
  const [loading, setLoading] = useState(true);
  const [record, setRecord] = useState(null);

  const listPath = useMemo(
    () => resolveListPath(location.state?.from),
    [location.state?.from],
  );

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(
        `/api/v1/admin/flow/topup-orders/${encodeURIComponent(id)}`,
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('flow.messages.load_failed'));
        return;
      }
      setRecord(data || null);
    } catch (error) {
      showError(error?.message || t('flow.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'flow', label: t('header.business_flow') },
          {
            key: 'flow-topup-list',
            label: t('flow.topup.title'),
            onClick: () => navigate(listPath),
          },
          {
            key: 'flow-topup-current',
            label: readOnlyText(record?.id || id),
            active: true,
          },
        ]}
        title={t('flow.topup.title')}
      />
      <div className='router-entity-detail-page'>
        <AppDetailSection
          title={t('flow.topup.title')}
          titleTag='div'
        >
              {loading ? (
                <div className='router-empty-cell'>{t('common.loading')}</div>
              ) : (
                <div className='router-detail-grid'>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.id')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(record?.id || id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.user')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(record?.username || record?.user_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('topup.external_topup_orders.columns.status')}
                    </div>
                    <div className='router-detail-value'>
                      {renderTopupStatus(record?.status, t)}
                    </div>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup.columns.source')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(record?.provider_name || record?.source)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('topup.external_topup_orders.columns.amount')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatAmount(record)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('topup.external_topup_orders.columns.quota')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatChargeAmount(record?.credit_amount)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('topup.external_topup_orders.columns.transaction_id')}
                    </div>
                    <pre className='router-detail-value router-monospace-value'>
                      {readOnlyText(record?.transaction_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('topup.external_topup_orders.fields.provider_order_id')}
                    </div>
                    <pre className='router-detail-value router-monospace-value'>
                      {readOnlyText(record?.provider_order_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.table.created_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(record?.created_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.table.updated_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(record?.updated_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.paid_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(record?.paid_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.topup_reconcile.detail.fields.redeemed_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(record?.redeemed_at)}
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
                {readOnlyText(record?.status_message)}
              </pre>
        </AppDetailSection>
      </div>
    </div>
  );
};

export default TopupDetail;
