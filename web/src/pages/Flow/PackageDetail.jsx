import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import { formatAmountWithUnit } from '../../helpers/render';
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

const formatChargeAmount = (value) => {
  const numericValue = Number(value || 0);
  if (!Number.isFinite(numericValue)) {
    return '-';
  }
  return `${numericValue.toFixed(6)} YYC`;
};

const renderPackageStatus = (status, t) => {
  switch (Number(status)) {
    case 1:
      return (
        <AppTag color='green' className='router-tag'>
          {t('user.detail.package_status_types.active')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.detail.package_status_types.expired')}
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='blue' className='router-tag'>
          {t('user.detail.package_status_types.replaced')}
        </AppTag>
      );
    case 4:
      return (
        <AppTag color='red' className='router-tag'>
          {t('user.detail.package_status_types.canceled')}
        </AppTag>
      );
    case 5:
      return (
        <AppTag color='teal' className='router-tag'>
          {t('user.detail.package_status_types.pending')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('user.detail.package_status_types.unknown')}
        </AppTag>
      );
  }
};

const resolveListPath = (stateFrom) => {
  if (typeof stateFrom !== 'string') {
    return '/admin/flow/package';
  }
  const normalized = stateFrom.trim();
  if (!normalized.startsWith('/')) {
    return '/admin/flow/package';
  }
  if (normalized.startsWith('/admin/flow/package/')) {
    return '/admin/flow/package';
  }
  return normalized || '/admin/flow/package';
};

const PackageDetail = () => {
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
        `/api/v1/admin/flow/package-records/${encodeURIComponent(id)}`,
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
            key: 'flow-package-list',
            label: t('flow.package.title'),
            onClick: () => navigate(listPath),
          },
          {
            key: 'flow-package-current',
            label: readOnlyText(record?.id || id),
            active: true,
          },
        ]}
        title={t('flow.package.title')}
      />
      <div className='router-entity-detail-page'>
        <AppDetailSection
          title={t('flow.package.title')}
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
                      {t('user.detail.package_name')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(record?.package_name)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.package_group')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(record?.group_name || record?.group_id)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('flow.package.columns.amount')}
                    </div>
                    <pre className='router-detail-value'>
                      {Number(record?.amount || 0) > 0
                        ? formatAmountWithUnit(
                            record?.amount || 0,
                            record?.currency || '',
                            6,
                          )
                        : '-'}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.package_daily_limit')}
                    </div>
                    <pre className='router-detail-value'>
                      {Number(record?.daily_quota_limit || 0) > 0
                        ? formatChargeAmount(record?.daily_quota_limit)
                        : t('common.unlimited')}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.package_emergency_limit')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatChargeAmount(record?.package_emergency_quota_limit)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.package_status')}
                    </div>
                    <div className='router-detail-value'>
                      {renderPackageStatus(record?.status, t)}
                    </div>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.package_started_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {formatDateTime(record?.started_at)}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.package_expires_at')}
                    </div>
                    <pre className='router-detail-value'>
                      {Number(record?.expires_at || 0) > 0
                        ? formatDateTime(record?.expires_at)
                        : t('common.unlimited')}
                    </pre>
                  </div>
                  <div className='router-detail-item'>
                    <div className='router-detail-label'>
                      {t('user.detail.quota_reset_timezone')}
                    </div>
                    <pre className='router-detail-value'>
                      {readOnlyText(record?.quota_reset_timezone)}
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
                </div>
              )}
        </AppDetailSection>
      </div>
    </div>
  );
};

export default PackageDetail;
