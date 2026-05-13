import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import {
  buildBillingCurrencyIndex,
  buildDisplayUnitOptions,
} from '../../helpers/billing';
import { formatDecimalNumber } from '../../helpers/render';
import UnitDropdown from '../../components/UnitDropdown';
import {
  AppField,
  AppFilterHeader,
  AppFormRow,
  AppIcon,
  AppInput,
  AppSection,
  AppTextarea,
} from '../../router-ui';

const formatByCurrencyMinorUnit = (amount, currency) => {
  const normalizedAmount = Number(amount || 0);
  if (!Number.isFinite(normalizedAmount)) {
    return '-';
  }
  const minorUnit = Number(currency?.minor_unit);
  const maximumFractionDigits =
    Number.isInteger(minorUnit) && minorUnit >= 0 ? minorUnit : 8;
  const unit = (currency?.code || '').toString().trim().toUpperCase();
  if (unit === 'YYC') {
    return formatDecimalNumber(Math.round(normalizedAmount), 0);
  }
  return formatDecimalNumber(normalizedAmount, maximumFractionDigits);
};

const renderPackageAmountValue = (yycAmount, displayUnit, currencyIndex) => {
  const normalizedYYCAmount = Number(yycAmount || 0);
  if (!Number.isFinite(normalizedYYCAmount)) {
    return '-';
  }
  const targetCurrency = currencyIndex[displayUnit] || currencyIndex.YYC;
  const rate = Number(targetCurrency?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatByCurrencyMinorUnit(normalizedYYCAmount / rate, targetCurrency);
};

const resolvePackageYYCAmount = (row, type) => {
  if (type === 'daily') {
    return Number(row?.daily_quota_limit ?? 0);
  }
  return Number(row?.package_emergency_quota_limit ?? 0);
};

const PackageDetail = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState(null);
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true }),
  );

  const normalizedId = useMemo(() => (id || '').toString().trim(), [id]);

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'yyc-first' }),
    [currencyIndex],
  );

  const loadDisplayUnits = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      const next = buildBillingCurrencyIndex(Array.isArray(data) ? data : [], {
        activeOnly: true,
      });
      setCurrencyIndex(next);
      setDisplayUnit((current) => {
        const normalizedCurrent = (current || '').toString().trim().toUpperCase();
        if (normalizedCurrent && next[normalizedCurrent]) {
          return normalizedCurrent;
        }
        if (next.USD) {
          return 'USD';
        }
        const fallbackUnit = Object.keys(next)
          .filter((code) => code)
          .sort((a, b) => a.localeCompare(b))[0];
        return fallbackUnit || 'YYC';
      });
    } catch (error) {
      showError(error?.message || error);
    }
  }, []);

  const loadDetail = useCallback(async () => {
    if (normalizedId === '') {
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/package/${encodeURIComponent(normalizedId)}`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.load_failed'));
        return;
      }
      setDetail(data || null);
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setLoading(false);
    }
  }, [normalizedId, t]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  useEffect(() => {
    loadDetail().then();
  }, [loadDetail]);

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          {
            key: 'package-list',
            label: t('header.package'),
            onClick: () => navigate('/admin/package'),
          },
          { key: 'package-current', label: normalizedId || '-', active: true },
        ]}
        title={t('package_manage.dialog.detail_title')}
      />
      <AppSection
        extra={
          <UnitDropdown
            variant='section'
            options={displayUnitOptions}
            value={displayUnit}
            onChange={(_, { value }) =>
              setDisplayUnit((value || '').toString().trim().toUpperCase())
            }
            aria-label={t('package_manage.table.daily_quota_limit')}
          />
        }
      >
        <div className='router-entity-detail-page'>
            {loading ? (
              <div className='router-empty-cell'>{t('common.loading')}</div>
            ) : (
              <>
                <AppFormRow>
                  <AppField label={t('package_manage.form.id')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.id || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.name')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.name || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.group')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.group_name || detail?.group_id || '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.form.description')} readOnly>
                    <AppTextarea
                      className='router-section-input'
                      value={detail?.description || '-'}
                      readOnly
                      rows={3}
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.form.sale_price')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={`${detail?.sale_currency || 'CNY'} ${detail?.sale_price ?? 0}`}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.form.sale_currency')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.sale_currency || 'CNY'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.table.daily_quota_limit')} readOnly>
                    <div className='router-section-input-with-unit'>
                      <AppInput
                        className='router-section-input router-section-input-with-unit-field'
                        value={renderPackageAmountValue(
                          resolvePackageYYCAmount(detail, 'daily'),
                          displayUnit,
                          currencyIndex,
                        )}
                        readOnly
                      />
                    </div>
                  </AppField>
                  <AppField
                    label={t('package_manage.table.package_emergency_quota_limit')}
                    readOnly
                  >
                    <div className='router-section-input-with-unit'>
                      <AppInput
                        className='router-section-input router-section-input-with-unit-field'
                        value={renderPackageAmountValue(
                          resolvePackageYYCAmount(detail, 'emergency'),
                          displayUnit,
                          currencyIndex,
                        )}
                        readOnly
                      />
                    </div>
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.table.duration_days')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={Number(detail?.duration_days || 0) || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.status')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={
                        detail?.enabled
                          ? t('package_manage.status.enabled')
                          : t('package_manage.status.disabled')
                      }
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.form.quota_reset_timezone')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.quota_reset_timezone || '-'}
                      readOnly
                    />
                  </AppField>
                  <AppField label={t('package_manage.table.created_at')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.created_at ? timestamp2string(detail.created_at) : '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>

                <AppFormRow>
                  <AppField label={t('package_manage.table.updated_at')} readOnly>
                    <AppInput
                      className='router-section-input'
                      value={detail?.updated_at ? timestamp2string(detail.updated_at) : '-'}
                      readOnly
                    />
                  </AppField>
                </AppFormRow>
              </>
            )}
        </div>
      </AppSection>
    </div>
  );
};

export default PackageDetail;
