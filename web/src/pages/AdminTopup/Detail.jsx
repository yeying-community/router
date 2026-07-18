import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import { formatAmountWithUnit } from '../../helpers/render';
import { formatPackageConcurrencyLimit } from '../../helpers/package';
import {
  AppDetailSection,
  AppFilterHeader,
  AppTag,
} from '../../router-ui';

const readOnlyText = (value) => {
  const normalized = (value || '').toString().trim();
  return normalized || '-';
};

const formatDateTime = (value) => {
  const normalized = Number(value || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return '-';
  }
  return timestamp2string(normalized);
};

const renderEnabledStatus = (enabled, t) =>
  enabled ? (
    <AppTag color='green' className='router-tag'>
      {t('package_manage.status.enabled')}
    </AppTag>
  ) : (
    <AppTag color='grey' className='router-tag'>
      {t('package_manage.status.disabled')}
    </AppTag>
  );

const renderPublicVisibleStatus = (visible, t) =>
  visible ? (
    <AppTag color='blue' className='router-tag'>
      {t('topup.manage.visibility.visible')}
    </AppTag>
  ) : (
    <AppTag color='grey' className='router-tag'>
      {t('topup.manage.visibility.hidden')}
    </AppTag>
  );

const normalizeModels = (models) =>
  Array.isArray(models)
    ? models
      .map((item) => (item?.model || item?.name || item || '').toString().trim())
      .filter(Boolean)
    : [];

const resolveListPath = (stateFrom) => {
  if (typeof stateFrom !== 'string') {
    return '/admin/entitlement';
  }
  const normalized = stateFrom.trim();
  if (!normalized.startsWith('/')) {
    return '/admin/entitlement';
  }
  if (normalized.startsWith('/admin/entitlement/topup/detail/')) {
    return '/admin/entitlement';
  }
  return normalized || '/admin/entitlement';
};

const TopupPlanDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { id } = useParams();
  const [loading, setLoading] = useState(true);
  const [plan, setPlan] = useState(null);

  const listPath = useMemo(
    () => resolveListPath(location.state?.from),
    [location.state?.from],
  );

  const loadDetail = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/topup/plans');
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.manage.load_failed'));
        return;
      }
      const rows = Array.isArray(data) ? data : [];
      const matched = rows.find(
        (item) => (item?.id || '').toString().trim() === (id || '').toString().trim(),
      );
      if (!matched) {
        showError(t('topup.manage.load_failed'));
        return;
      }
      setPlan(matched);
    } catch (error) {
      showError(error?.message || t('topup.manage.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  const supportedModels = useMemo(
    () => normalizeModels(plan?.supported_models),
    [plan?.supported_models],
  );

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'model', label: t('header.model') },
          {
            key: 'entitlement',
            label: t('header.entitlement'),
            onClick: () => navigate('/admin/entitlement'),
          },
          {
            key: 'topup-list',
            label: t('header.topup'),
            onClick: () => navigate(listPath),
          },
          {
            key: 'topup-current',
            label: readOnlyText(plan?.id || id),
            active: true,
          },
        ]}
        title={t('topup.manage.detail_title')}
      />
      <div className='router-entity-detail-page'>
        <AppDetailSection title={t('common.basic_info')} titleTag='div'>
          {loading ? (
            <div className='router-empty-cell'>{t('common.loading')}</div>
          ) : (
            <div className='router-detail-grid'>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('redemption.table.id')}</div>
                <pre className='router-detail-value router-monospace-value'>
                  {readOnlyText(plan?.id || id)}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('topup.manage.columns.name')}</div>
                <pre className='router-detail-value'>{readOnlyText(plan?.name)}</pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('topup.manage.columns.group')}</div>
                <pre className='router-detail-value'>
                  {readOnlyText(plan?.group_name || plan?.group_id)}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('topup.manage.columns.pay_amount')}</div>
                <pre className='router-detail-value'>
                  {formatAmountWithUnit(plan?.amount || 0, plan?.amount_currency || '', 6)}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>
                  {t('topup.manage.columns.credited_amount')}
                </div>
                <pre className='router-detail-value'>
                  {formatAmountWithUnit(
                    plan?.quota_amount || 0,
                    plan?.quota_currency || 'YYC',
                    6,
                  )}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>
                  {t('topup.manage.columns.concurrency_limit')}
                </div>
                <pre className='router-detail-value'>
                  {formatPackageConcurrencyLimit(plan, t, t('common.unlimited'))}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>
                  {t('topup.manage.columns.validity_days')}
                </div>
                <pre className='router-detail-value'>
                  {Number(plan?.validity_days || 0) > 0
                    ? `${Number(plan?.validity_days || 0)} ${t('common.day')}`
                    : t('common.never')}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('topup.manage.columns.enabled')}</div>
                <div className='router-detail-value'>
                  {renderEnabledStatus(Boolean(plan?.enabled), t)}
                </div>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>
                  {t('topup.manage.columns.public_visible')}
                </div>
                <div className='router-detail-value'>
                  {renderPublicVisibleStatus(plan?.public_visible !== false, t)}
                </div>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>
                  {t('topup.manage.columns.applicable_models')}
                </div>
                <pre className='router-detail-value'>
                  {supportedModels.length > 0 ? supportedModels.join('\n') : '-'}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('topup.manage.columns.created_at')}</div>
                <pre className='router-detail-value'>
                  {formatDateTime(plan?.created_at)}
                </pre>
              </div>
              <div className='router-detail-item'>
                <div className='router-detail-label'>{t('topup.manage.columns.updated_at')}</div>
                <pre className='router-detail-value'>
                  {formatDateTime(plan?.updated_at)}
                </pre>
              </div>
            </div>
          )}
        </AppDetailSection>
      </div>
    </div>
  );
};

export default TopupPlanDetail;
