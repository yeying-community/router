import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../helpers';
import {
  TOPUP_PLAN_LIST_COLUMN_WIDTHS,
  TOPUP_PLAN_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import UnitDropdown from './UnitDropdown';
import {
  billingInputValueToChargeAmount,
  buildBillingCurrencyIndex,
  buildDisplayUnitOptions,
  formatDisplayAmountFromChargeAmount,
} from '../helpers/billing';
import {
  AppButton,
  AppCompact,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTableActionButton,
} from '../router-ui';

const createEmptyPlan = () => ({
  id: '',
  name: '',
  group_id: '',
  group_name: '',
  amount: 0,
  amount_currency: 'CNY',
  quota_amount: 0,
  quota_currency: 'USD',
  validity_days: 0,
  enabled: true,
  public_visible: true,
});

const ensureUnitOption = (options, value) => {
  const normalized = (value || '').toString().trim().toUpperCase();
  const items = Array.isArray(options) ? options : [];
  if (!normalized || items.some((item) => item?.value === normalized)) {
    return items;
  }
  return [...items, { value: normalized, label: normalized }];
};

const appendGroupOptionIfMissing = (options, groupID, groupName) => {
  const normalizedGroupID = (groupID || '').toString().trim();
  if (!normalizedGroupID) {
    return Array.isArray(options) ? options : [];
  }
  const currentOptions = Array.isArray(options) ? options : [];
  if (currentOptions.some((item) => (item?.value || '').toString().trim() === normalizedGroupID)) {
    return currentOptions;
  }
  return [
    ...currentOptions,
    {
      key: normalizedGroupID,
      value: normalizedGroupID,
      text: (groupName || '').toString().trim() || normalizedGroupID,
    },
  ];
};

const mergeGroupOptions = (currentOptions, nextOptions) => {
  const merged = new Map();
  const appendOption = (option) => {
    const value = (option?.value || '').toString().trim();
    if (!value) {
      return;
    }
    const current = merged.get(value);
    const nextText = (option?.text || option?.label || '').toString().trim();
    if (!current) {
      merged.set(value, {
        key: option?.key || value,
        value,
        text: nextText || value,
      });
      return;
    }
    merged.set(value, {
      ...current,
      key: option?.key || current.key || value,
      value,
      text: nextText || current.text || value,
    });
  };
  (Array.isArray(currentOptions) ? currentOptions : []).forEach(appendOption);
  (Array.isArray(nextOptions) ? nextOptions : []).forEach(appendOption);
  return Array.from(merged.values()).sort((a, b) =>
    (a.text || '').localeCompare(b.text || '', 'zh-Hans-CN', {
      sensitivity: 'base',
      numeric: true,
    }),
  );
};

const resolveGroupOptionLabel = (options, groupID, fallbackName = '') => {
  const normalizedGroupID = (groupID || '').toString().trim();
  if (!normalizedGroupID) {
    return '';
  }
  const matchedOption = (Array.isArray(options) ? options : []).find(
    (item) => (item?.value || '').toString().trim() === normalizedGroupID,
  );
  return (
    (matchedOption?.text || matchedOption?.label || fallbackName || normalizedGroupID)
      .toString()
      .trim()
  );
};

const TopupPlansManager = () => {
  const { t } = useTranslation();
  const [plans, setPlans] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [activeRow, setActiveRow] = useState(null);
  const [form, setForm] = useState(createEmptyPlan());
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'charge-first' }),
    [currencyIndex]
  );

  const payCurrencyOptions = useMemo(
    () => ensureUnitOption(displayUnitOptions, form.amount_currency || 'CNY'),
    [displayUnitOptions, form.amount_currency]
  );

  const quotaCurrencyOptions = useMemo(
    () => ensureUnitOption(displayUnitOptions, form.quota_currency || 'USD'),
    [displayUnitOptions, form.quota_currency]
  );

  const selectedGroupValue = useMemo(() => {
    const groupID = (form.group_id || '').toString().trim();
    if (!groupID) {
      return undefined;
    }
    return {
      value: groupID,
      label: resolveGroupOptionLabel(groupOptions, groupID, form.group_name),
    };
  }, [form.group_id, form.group_name, groupOptions]);

  const loadPlans = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/topup/plans');
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.manage.load_failed'));
        return;
      }
      setPlans(Array.isArray(data) ? data : []);
    } catch (error) {
      showError(error?.message || t('topup.manage.load_failed'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadPlans().then();
  }, [loadPlans]);

  const loadGroups = useCallback(async () => {
    setGroupLoading(true);
    try {
      const items = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/groups', {
          params: {
            page,
            page_size: 100,
          },
        });
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('topup.manage.load_failed'));
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        items.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (pageItems.length === 0 || items.length >= total || pageItems.length < 100) {
          break;
        }
        page += 1;
      }
      setGroupOptions((current) =>
        mergeGroupOptions(
          current,
          items.map((item) => ({
            key: item.id,
            value: item.id,
            text: item.name || item.id,
          })),
        ),
      );
    } catch (error) {
      showError(error?.message || t('topup.manage.load_failed'));
    } finally {
      setGroupLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadGroups().then();
  }, [loadGroups]);

  const loadDisplayUnits = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('topup.manage.load_failed'));
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
  }, [t]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  const openCreate = () => {
    setIsCreating(true);
    setActiveRow(null);
    setForm(createEmptyPlan());
    setEditOpen(true);
  };

  const openEdit = (row) => {
    setIsCreating(false);
    setActiveRow(row || null);
    setGroupOptions((current) =>
      appendGroupOptionIfMissing(current, row?.group_id, row?.group_name),
    );
    setForm({
      id: row?.id || '',
      name: row?.name || '',
      group_id: row?.group_id || '',
      group_name: row?.group_name || '',
      amount: Number(row?.amount || 0),
      amount_currency: row?.amount_currency || 'CNY',
      quota_amount: Number(row?.quota_amount || 0),
      quota_currency: row?.quota_currency || 'USD',
      validity_days: Number(row?.validity_days || 0),
      enabled: Boolean(row?.enabled),
      public_visible: row?.public_visible !== false,
    });
    setEditOpen(true);
  };

  const savePlan = async () => {
    setSaving(true);
    try {
      const payload = {
        id: form.id,
        name: form.name,
        group_id: form.group_id,
        amount: Number(form.amount || 0),
        amount_currency: form.amount_currency || 'CNY',
        quota_amount: Number(form.quota_amount || 0),
        quota_currency: form.quota_currency || 'USD',
        validity_days: Number(form.validity_days || 0),
        enabled: Boolean(form.enabled),
        public_visible: Boolean(form.public_visible),
        sort_order: Number(activeRow?.sort_order || 0),
      };
      const res = isCreating
        ? await API.post('/api/v1/admin/topup/plan', payload)
        : await API.put('/api/v1/admin/topup/plan', payload);
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.manage.save_failed'));
        return;
      }
      if (isCreating) {
        setPlans((current) => [...current, data]);
      } else {
        setPlans((current) =>
          current.map((item) => (((item?.id || '') === (data?.id || '')) ? data : item)),
        );
      }
      setEditOpen(false);
      showSuccess(isCreating ? t('topup.manage.create_success') : t('topup.manage.save_success'));
    } catch (error) {
      showError(
        error?.message ||
          (isCreating ? t('topup.manage.create_failed') : t('topup.manage.save_failed')),
      );
    } finally {
      setSaving(false);
    }
  };

  const removePlan = async () => {
    const id = (activeRow?.id || '').toString().trim();
    if (!id) {
      return;
    }
    setSaving(true);
    try {
      const res = await API.delete(`/api/v1/admin/topup/plan/${encodeURIComponent(id)}`);
      const { success, message } = res?.data || {};
      if (!success) {
        showError(message || t('topup.manage.delete_failed'));
        return;
      }
      setPlans((current) => current.filter((item) => (item?.id || '') !== id));
      setDeleteOpen(false);
      setActiveRow(null);
      showSuccess(t('topup.manage.delete_success'));
    } catch (error) {
      showError(error?.message || t('topup.manage.delete_failed'));
    } finally {
      setSaving(false);
    }
  };

  const togglePublicVisible = async (row, checked) => {
    const id = (row?.id || '').toString().trim();
    if (!id) {
      return;
    }
    setSaving(true);
    try {
      const payload = {
        id,
        name: row?.name || '',
        group_id: row?.group_id || '',
        amount: Number(row?.amount || 0),
        amount_currency: row?.amount_currency || 'CNY',
        quota_amount: Number(row?.quota_amount || 0),
        quota_currency: row?.quota_currency || 'USD',
        validity_days: Number(row?.validity_days || 0),
        enabled: Boolean(row?.enabled),
        public_visible: Boolean(checked),
        sort_order: Number(row?.sort_order || 0),
      };
      const res = await API.put('/api/v1/admin/topup/plan', payload);
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.manage.save_failed'));
        return;
      }
      setPlans((current) =>
        current.map((item) => (((item?.id || '') === (data?.id || '')) ? data : item)),
      );
      showSuccess(t('topup.manage.save_success'));
    } catch (error) {
      showError(error?.message || t('topup.manage.save_failed'));
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (row, checked) => {
    const id = (row?.id || '').toString().trim();
    if (!id) {
      return;
    }
    setSaving(true);
    try {
      const payload = {
        id,
        name: row?.name || '',
        group_id: row?.group_id || '',
        amount: Number(row?.amount || 0),
        amount_currency: row?.amount_currency || 'CNY',
        quota_amount: Number(row?.quota_amount || 0),
        quota_currency: row?.quota_currency || 'USD',
        validity_days: Number(row?.validity_days || 0),
        enabled: Boolean(checked),
        public_visible: row?.public_visible !== false,
        sort_order: Number(row?.sort_order || 0),
      };
      const res = await API.put('/api/v1/admin/topup/plan', payload);
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.manage.save_failed'));
        return;
      }
      setPlans((current) =>
        current.map((item) => (((item?.id || '') === (data?.id || '')) ? data : item)),
      );
      showSuccess(t('topup.manage.save_success'));
    } catch (error) {
      showError(error?.message || t('topup.manage.save_failed'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <AppFilterHeader
        className='router-block-gap-md'
        breadcrumbs={[
          { key: 'workspace', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          { key: 'topup', label: t('header.topup'), active: true },
        ]}
        title={t('topup.manage.title')}
        titleClassName='router-ui-section-title'
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton className='router-page-button' color='blue' onClick={openCreate}>
              {t('common.add')}
            </AppButton>
            <AppButton className='router-page-button' onClick={() => loadPlans()} loading={loading}>
              {t('common.refresh')}
            </AppButton>
          </div>
        }
      />

      <div className='router-table-scroll-x'>
        <AppTable
          className='router-table router-list-table router-table-fit-page router-topup-plan-table'
          dataSource={plans}
          scroll={{ x: TOPUP_PLAN_LIST_TABLE_MIN_WIDTH }}
          rowKey={(row) => row?.id || [row?.group_id || 'group', row?.name || 'plan', row?.amount || 0].join('-')}
          pagination={false}
          locale={{ emptyText: t('common.no_data', '暂无数据') }}
          columns={[
            {
              title: t('topup.manage.columns.name'),
              dataIndex: 'name',
              key: 'name',
              className: 'router-topup-plan-name-cell',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.name,
              ellipsis: true,
              render: (value) => value || '-',
            },
            {
              title: t('topup.manage.columns.group'),
              dataIndex: 'group_name',
              key: 'group',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.group,
              ellipsis: true,
              render: (_, row) => row.group_name || row.group_id || '-',
            },
            {
              title: t('topup.manage.columns.pay_amount'),
              dataIndex: 'amount',
              key: 'amount',
              className: 'router-topup-plan-amount-cell',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.payAmount,
              render: (_, row) => `${row.amount} ${row.amount_currency}`,
            },
            {
              title: (
                <div className='router-table-header-with-control'>
                  <span>{t('topup.manage.columns.credited_amount')}</span>
                  <UnitDropdown
                    variant='header'
                    compact
                    options={displayUnitOptions}
                    value={displayUnit}
                    onClick={(e) => {
                      e.stopPropagation();
                    }}
                    onChange={(_, { value }) => {
                      setDisplayUnit((value || '').toString());
                    }}
                  />
                </div>
              ),
              dataIndex: 'quota_amount',
              key: 'quota_amount',
              className: 'router-topup-plan-quota-cell',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.creditedAmount,
              render: (_, row) => {
                const storedChargeAmount = billingInputValueToChargeAmount(
                  row?.quota_amount || 0,
                  row?.quota_currency || 'USD',
                  currencyIndex,
                );
                if (!Number.isFinite(storedChargeAmount)) {
                  return '-';
                }
                return formatDisplayAmountFromChargeAmount(
                  storedChargeAmount,
                  displayUnit,
                  currencyIndex,
                  {
                    fractionDigits: 6,
                    includeSymbol: false,
                    chargeMode: 'fixed',
                  },
                );
              },
            },
            {
              title: t('topup.manage.columns.enabled'),
              dataIndex: 'enabled',
              key: 'enabled',
              className: 'router-table-col-status-narrow',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.enabled,
              render: (_, row) => (
                <AppSwitch
                  checked={Boolean(row.enabled)}
                  disabled={saving}
                  onChange={(_, { checked }) => toggleEnabled(row, Boolean(checked))}
                />
              ),
            },
            {
              title: t('topup.manage.columns.public_visible'),
              dataIndex: 'public_visible',
              key: 'public_visible',
              className: 'router-table-col-status-narrow',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.publicVisible,
              render: (_, row) => (
                <AppSwitch
                  checked={row.public_visible !== false}
                  disabled={saving}
                  onChange={(_, { checked }) => togglePublicVisible(row, Boolean(checked))}
                />
              ),
            },
            {
              title: t('topup.manage.columns.validity_days'),
              dataIndex: 'validity_days',
              key: 'validity_days',
              className: 'router-table-col-status-narrow',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.validityDays,
              render: (value) =>
                Number(value || 0) > 0
                  ? `${Number(value || 0)} ${t('common.day')}`
                  : t('common.never'),
            },
            {
              title: t('topup.manage.columns.created_at', t('common.created_at', '创建时间')),
              dataIndex: 'created_at',
              key: 'created_at',
              className: 'router-table-col-datetime',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.createdAt,
              sorter: (a, b) => Number(a.created_at || 0) - Number(b.created_at || 0),
              defaultSortOrder: 'descend',
              render: (value) => (value ? timestamp2string(value) : '-'),
            },
            {
              title: t('topup.manage.columns.updated_at', t('common.updated_at', '更新时间')),
              dataIndex: 'updated_at',
              key: 'updated_at',
              className: 'router-table-col-datetime',
              width: TOPUP_PLAN_LIST_COLUMN_WIDTHS.updatedAt,
              sorter: (a, b) => Number(a.updated_at || 0) - Number(b.updated_at || 0),
              render: (value) => (value ? timestamp2string(value) : '-'),
            },
            {
              title: t('common.operation'),
              key: 'action',
              className: 'router-table-col-actions-icon router-topup-plan-action-cell',
              width: 84,
              render: (_, row) => (
                <div className='router-action-group-tight router-table-actions-icon-compact'>
                  <AppTableActionButton
                    icon='edit'
                    title={t('common.edit')}
                    onClick={() => openEdit(row)}
                  />
                  <AppTableActionButton
                    icon='trash'
                    title={t('common.delete')}
                    onClick={() => {
                      setActiveRow(row);
                      setDeleteOpen(true);
                    }}
                  />
                </div>
              ),
            },
          ]}
        />
      </div>

      <AppModal
        className='router-topup-plan-editor-modal'
        open={editOpen}
        size='tiny'
        onClose={() => setEditOpen(false)}
        title={isCreating ? t('topup.manage.create_title') : t('topup.manage.edit_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          <AppFormRow className='router-topup-plan-form-row'>
            <AppField label={t('topup.manage.columns.name')} required>
              <AppInput
                value={form.name}
                onChange={(event, { value }) =>
                  setForm((current) => ({ ...current, name: value || '' }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow className='router-topup-plan-form-row'>
            <AppField label={t('topup.manage.columns.group')}>
              <AppSelect
                labelInValue
                search
                loading={groupLoading}
                options={groupOptions}
                placeholder={t('topup.manage.group_placeholder')}
                value={selectedGroupValue}
                onChange={(_, data) => {
                  const value =
                    typeof data?.value === 'object'
                      ? (data?.value?.value || '').toString()
                      : (data?.value || '').toString();
                  const label =
                    typeof data?.value === 'object'
                      ? (data?.value?.label || '').toString().trim()
                      : '';
                  setForm((current) => ({
                    ...current,
                    group_id: value,
                    group_name:
                      (
                        label ||
                        resolveGroupOptionLabel(groupOptions, value, current.group_name)
                      )
                        .toString()
                        .trim(),
                  }));
                }}
              />
            </AppField>
          </AppFormRow>
          <AppFormRow className='router-topup-plan-form-row'>
            <AppField label={t('topup.manage.columns.pay_amount')}>
              <AppCompact className='router-section-input-with-unit' block>
                <AppInputNumber
                  className='router-section-input router-section-input-with-unit-field'
                  min={0}
                  step={0.01}
                  precision={2}
                  fluid
                  value={form.amount}
                  onChange={(_, { value }) =>
                    setForm((current) => ({
                      ...current,
                      amount: Number(value || 0),
                    }))
                  }
                />
                <UnitDropdown
                  variant='inputUnit'
                  options={payCurrencyOptions}
                  value={form.amount_currency || 'CNY'}
                  onChange={(_, { value }) =>
                    setForm((current) => ({
                      ...current,
                      amount_currency: (value || 'CNY').toString().trim().toUpperCase(),
                    }))
                  }
                  aria-label={t('topup.manage.columns.pay_amount')}
                />
              </AppCompact>
            </AppField>
            <AppField label={t('topup.manage.columns.credited_amount')}>
              <AppCompact className='router-section-input-with-unit' block>
                <AppInputNumber
                  className='router-section-input router-section-input-with-unit-field'
                  min={0}
                  step={0.01}
                  precision={2}
                  fluid
                  value={form.quota_amount}
                  onChange={(_, { value }) =>
                    setForm((current) => ({
                      ...current,
                      quota_amount: Number(value || 0),
                    }))
                  }
                />
                <UnitDropdown
                  variant='inputUnit'
                  options={quotaCurrencyOptions}
                  value={form.quota_currency || 'USD'}
                  onChange={(_, { value }) =>
                    setForm((current) => ({
                      ...current,
                      quota_currency: (value || 'USD').toString().trim().toUpperCase(),
                    }))
                  }
                  aria-label={t('topup.manage.columns.credited_amount')}
                />
              </AppCompact>
            </AppField>
          </AppFormRow>
          <AppFormRow className='router-topup-plan-form-row'>
            <AppField label={t('topup.manage.columns.validity_days')}>
              <AppInputNumber
                min={0}
                step={1}
                precision={0}
                fluid
                value={form.validity_days}
                onChange={(_, { value }) =>
                  setForm((current) => ({
                    ...current,
                    validity_days: Number(value || 0),
                  }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow className='router-topup-plan-form-row'>
            <AppField label={t('topup.manage.columns.enabled')}>
              <AppSwitch
                checked={form.enabled}
                onChange={(_, { checked }) =>
                  setForm((current) => ({
                    ...current,
                    enabled: Boolean(checked),
                  }))
                }
              />
            </AppField>
            <AppField label={t('topup.manage.columns.public_visible')}>
              <AppSwitch
                checked={form.public_visible}
                onChange={(_, { checked }) =>
                  setForm((current) => ({
                    ...current,
                    public_visible: Boolean(checked),
                  }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormActions>
            <AppButton
              className='router-section-button'
              onClick={() => setEditOpen(false)}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              className='router-section-button'
              color='blue'
              onClick={savePlan}
              loading={saving}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>

      <AppModal
        open={deleteOpen}
        size='tiny'
        onClose={() => setDeleteOpen(false)}
        title={t('topup.manage.delete_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          <div>
            {t('topup.manage.delete_confirm', {
              name: activeRow?.name || '-',
            })}
          </div>
          <AppFormActions>
            <AppButton
              className='router-section-button'
              onClick={() => setDeleteOpen(false)}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              className='router-section-button'
              color='blue'
              onClick={removePlan}
              loading={saving}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>
    </>
  );
};

export default TopupPlansManager;
