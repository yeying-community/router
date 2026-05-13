import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
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
import {
  AppButton,
  AppDescriptions,
  AppFilterHeader,
  AppSection,
  AppTooltip,
} from '../../router-ui';

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
  const detailRows = useMemo(() => {
    const rows = [
      {
        key: 'order_id',
        label: t('topup.external_topup_orders.columns.order_id'),
        value: order?.id || '-',
      },
      {
        key: 'business_type',
        label: t('topup.external_topup_orders.columns.business_type'),
        value: formatTopupBusinessType(order?.business_type, t),
      },
      {
        key: 'status',
        label: t('topup.external_topup_orders.columns.status'),
        value: statusHint ? (
          <AppTooltip title={statusHint}>
            <span className='router-help-trigger'>
              {renderTopupOrderStatus(order?.status, t)}
            </span>
          </AppTooltip>
        ) : (
          renderTopupOrderStatus(order?.status, t)
        ),
      },
      {
        key: 'status_message',
        label: t('topup.external_topup_orders.fields.status_message'),
        value: order?.status_message || '-',
      },
      {
        key: 'amount',
        label: t('topup.external_topup_orders.columns.amount'),
        value:
          Number(order?.amount || 0) > 0
            ? `${order?.currency || 'CNY'} ${Number(order?.amount || 0).toFixed(2)}`
            : Number(order?.quota || 0) > 0
              ? renderDisplayAmount(order?.quota)
              : '-',
      },
      {
        key: 'title',
        label: t('topup.external_topup_orders.fields.title'),
        value: order?.title || '-',
      },
      {
        key: 'transaction_id',
        label: t('topup.external_topup_orders.columns.transaction_id'),
        value: order?.transaction_id || '-',
      },
      {
        key: 'provider_order_id',
        label: t('topup.external_topup_orders.fields.provider_order_id'),
        value: order?.provider_order_id || '-',
      },
      {
        key: 'created_at',
        label: t('topup.external_topup_orders.columns.time'),
        value: order?.created_at ? timestamp2string(order?.created_at) : '-',
      },
      {
        key: 'updated_at',
        label: t('topup.external_topup_orders.fields.updated_at'),
        value: order?.updated_at ? timestamp2string(order?.updated_at) : '-',
      },
    ];
    if (recordKey === 'package') {
      rows.splice(2, 0, {
        key: 'package_name',
        label: t('topup.external_topup_orders.columns.package_name'),
        value: order?.package_name || '-',
      });
    }
    return rows;
  }, [order, recordKey, renderDisplayAmount, statusHint, t]);

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'records', label: t('header.records') },
          {
            key: 'topup-order-list',
            label: detailPathLabel,
            onClick: () => navigate(listPath),
          },
          {
            key: 'topup-order-current',
            label: detailPathOrderID,
            active: true,
          },
        ]}
        title={detailTitle}
        actions={
          <>
          <AppButton
            className='router-section-button'
            onClick={refreshOrderStatus}
            loading={refreshing}
            disabled={!order}
          >
            {t('topup.records.refresh_status')}
          </AppButton>
          {['created', 'pending'].includes(String(order?.status || '').trim()) ? (
            <>
              <AppButton
                color='blue'
                className='router-section-button'
                onClick={continuePay}
                loading={refreshing}
                disabled={!order}
              >
                {t('topup.records.continue_pay')}
              </AppButton>
              <AppButton
                className='router-section-button'
                onClick={cancelPay}
                loading={canceling}
                disabled={!order}
              >
                {t('topup.records.cancel_pay')}
              </AppButton>
            </>
          ) : null}
          </>
        }
      />
      <AppSection>
        <div className='router-entity-detail-page'>
            {loading ? (
              <div className='router-empty-cell'>{t('common.loading')}</div>
            ) : (
              <AppDescriptions items={detailRows} />
            )}
        </div>
      </AppSection>
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
