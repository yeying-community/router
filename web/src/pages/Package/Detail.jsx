import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import { Breadcrumb, Card, Form } from 'semantic-ui-react';
import { API, showError, timestamp2string } from '../../helpers';
import {
  buildBillingCurrencyIndex,
  buildDisplayUnitOptions,
} from '../../helpers/billing';
import { formatDecimalNumber } from '../../helpers/render';
import UnitDropdown from '../../components/UnitDropdown';

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
    return Number(row?.yyc_daily_limit ?? row?.daily_quota_limit ?? 0);
  }
  return Number(
    row?.yyc_package_emergency_limit ?? row?.package_emergency_quota_limit ?? 0,
  );
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
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-entity-detail-page'>
            <div className='router-entity-detail-breadcrumb'>
              <Breadcrumb size='small'>
                <Breadcrumb.Section
                  link
                  onClick={() => navigate('/admin/package')}
                >
                  {t('header.package')}
                </Breadcrumb.Section>
                <Breadcrumb.Divider icon='right chevron' />
                <Breadcrumb.Section active>
                  {normalizedId || '-'}
                </Breadcrumb.Section>
              </Breadcrumb>
            </div>

            <div className='router-toolbar'>
              <div className='router-toolbar-start'>
                <div className='router-detail-section-title'>
                  {t('package_manage.dialog.detail_title')}
                </div>
              </div>
            </div>

            {loading ? (
              <div className='router-empty-cell'>{t('common.loading')}</div>
            ) : (
              <Form>
                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.form.id')}
                    value={detail?.id || '-'}
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.table.name')}
                    value={detail?.name || '-'}
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.table.group')}
                    value={detail?.group_name || detail?.group_id || '-'}
                    readOnly
                  />
                </Form.Group>

                <Form.TextArea
                  className='router-section-input'
                  label={t('package_manage.form.description')}
                  value={detail?.description || '-'}
                  readOnly
                />

                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.form.sale_price')}
                    value={`${detail?.sale_currency || 'CNY'} ${detail?.sale_price ?? 0}`}
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.form.sale_currency')}
                    value={detail?.sale_currency || 'CNY'}
                    readOnly
                  />
                </Form.Group>

                <Form.Group widths='equal'>
                  <Form.Field>
                    <label>{t('package_manage.table.daily_quota_limit')}</label>
                    <div className='router-section-input-with-unit'>
                      <Form.Input
                        className='router-section-input router-section-input-with-unit-field'
                        value={renderPackageAmountValue(
                          resolvePackageYYCAmount(detail, 'daily'),
                          displayUnit,
                          currencyIndex,
                        )}
                        readOnly
                      />
                      <UnitDropdown
                        variant='inputUnit'
                        options={displayUnitOptions}
                        value={displayUnit}
                        onChange={(_, { value }) =>
                          setDisplayUnit((value || '').toString().trim().toUpperCase())
                        }
                        aria-label={t('package_manage.table.daily_quota_limit')}
                      />
                    </div>
                  </Form.Field>
                  <Form.Field>
                    <label>{t('package_manage.table.package_emergency_quota_limit')}</label>
                    <div className='router-section-input-with-unit'>
                      <Form.Input
                        className='router-section-input router-section-input-with-unit-field'
                        value={renderPackageAmountValue(
                          resolvePackageYYCAmount(detail, 'emergency'),
                          displayUnit,
                          currencyIndex,
                        )}
                        readOnly
                      />
                      <UnitDropdown
                        variant='inputUnit'
                        options={displayUnitOptions}
                        value={displayUnit}
                        onChange={(_, { value }) =>
                          setDisplayUnit((value || '').toString().trim().toUpperCase())
                        }
                        aria-label={t('package_manage.table.package_emergency_quota_limit')}
                      />
                    </div>
                  </Form.Field>
                </Form.Group>

                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.table.duration_days')}
                    value={Number(detail?.duration_days || 0) || '-'}
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.table.status')}
                    value={
                      detail?.enabled
                        ? t('package_manage.status.enabled')
                        : t('package_manage.status.disabled')
                    }
                    readOnly
                  />
                </Form.Group>

                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.form.quota_reset_timezone')}
                    value={detail?.quota_reset_timezone || '-'}
                    readOnly
                  />
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.table.created_at')}
                    value={detail?.created_at ? timestamp2string(detail.created_at) : '-'}
                    readOnly
                  />
                </Form.Group>

                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-section-input'
                    label={t('package_manage.table.updated_at')}
                    value={detail?.updated_at ? timestamp2string(detail.updated_at) : '-'}
                    readOnly
                  />
                </Form.Group>
              </Form>
            )}
          </div>
        </Card.Content>
      </Card>
    </div>
  );
};

export default PackageDetail;
