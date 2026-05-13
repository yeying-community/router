import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../helpers';
import {
  AppButton,
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
  sort_order: 0,
});

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
      setGroupOptions(
        items.map((item) => ({
          key: item.id,
          value: item.id,
          text: item.name || item.id,
        })),
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

  const openCreate = () => {
    setIsCreating(true);
    setActiveRow(null);
    setForm(createEmptyPlan());
    setEditOpen(true);
  };

  const openEdit = (row, index = 0) => {
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
      sort_order: Number(row?.sort_order || index + 1),
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
        sort_order: Number(form.sort_order || 0),
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
        setPlans((current) => [...current, data].sort((a, b) => Number(a?.sort_order || 0) - Number(b?.sort_order || 0)));
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

  return (
    <>
      <AppFilterHeader
        className='router-block-gap-md'
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
          className='router-table router-list-table router-topup-plan-table'
          dataSource={plans}
          rowKey={(row) =>
            row?.id ||
            [
              row?.group_id || 'group',
              row?.name || 'plan',
              row?.sort_order || 0,
              row?.amount || 0,
            ].join('-')
          }
          pagination={false}
          scroll={{ x: 920 }}
          locale={{ emptyText: t('common.no_data', '暂无数据') }}
          columns={[
            {
              title: t('topup.manage.columns.name'),
              dataIndex: 'name',
              key: 'name',
              className: 'router-topup-plan-name-cell',
              render: (value) => value || '-',
            },
            {
              title: t('topup.manage.columns.group'),
              dataIndex: 'group_name',
              key: 'group',
              render: (_, row) => row.group_name || row.group_id || '-',
            },
            {
              title: t('topup.manage.columns.pay_amount'),
              dataIndex: 'amount',
              key: 'amount',
              className: 'router-topup-plan-amount-cell',
              render: (_, row) => `${row.amount} ${row.amount_currency}`,
            },
            {
              title: t('topup.manage.columns.credited_amount'),
              dataIndex: 'quota_amount',
              key: 'quota_amount',
              className: 'router-topup-plan-quota-cell',
              render: (_, row) => `${row.quota_amount} ${row.quota_currency}`,
            },
            {
              title: t('package_manage.form.sort_order'),
              dataIndex: 'sort_order',
              key: 'sort_order',
              className: 'router-topup-plan-status-cell',
              render: (value) => Number(value || 0),
            },
            {
              title: t('topup.manage.columns.enabled'),
              dataIndex: 'enabled',
              key: 'enabled',
              className: 'router-topup-plan-status-cell',
              render: (value) => (
                <AppSwitch checked={Boolean(value)} disabled size='small' />
              ),
            },
            {
              title: t('topup.manage.columns.public_visible'),
              dataIndex: 'public_visible',
              key: 'public_visible',
              className: 'router-topup-plan-status-cell',
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
              className: 'router-topup-plan-status-cell',
              render: (value) =>
                Number(value || 0) > 0
                  ? `${Number(value || 0)} ${t('common.day')}`
                  : t('common.never'),
            },
            {
              title: t('common.operation'),
              key: 'action',
              className: 'router-table-action-cell router-topup-plan-action-cell',
              render: (_, row, index) => (
                <div className='router-action-group-tight'>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => openEdit(row, index)}
                  >
                    {t('common.edit')}
                  </AppButton>
                  <AppButton
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setActiveRow(row);
                      setDeleteOpen(true);
                    }}
                  >
                    {t('common.delete')}
                  </AppButton>
                </div>
              ),
            },
          ]}
        />
      </div>

      <AppModal
        open={editOpen}
        size='tiny'
        onClose={() => setEditOpen(false)}
        title={isCreating ? t('topup.manage.create_title') : t('topup.manage.edit_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          <AppFormRow>
            <AppField label={t('topup.manage.columns.name')} required>
              <AppInput
                value={form.name}
                onChange={(event, { value }) =>
                  setForm((current) => ({ ...current, name: value || '' }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField label={t('topup.manage.columns.group')}>
              <AppSelect
                search
                loading={groupLoading}
                options={groupOptions}
                placeholder={t('topup.manage.group_placeholder')}
                value={form.group_id}
                onChange={(_, data) => {
                  const value = (data?.value || '').toString();
                  const option = groupOptions.find((item) => item.value === value);
                  setForm((current) => ({
                    ...current,
                    group_id: value,
                    group_name: option?.text || '',
                  }));
                }}
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField label={t('topup.manage.columns.pay_amount')}>
              <AppInputNumber
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
            </AppField>
            <AppField label={t('topup.manage.columns.credited_amount')}>
              <AppInputNumber
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
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField label={t('package_manage.form.sort_order')}>
              <AppInputNumber
                min={0}
                step={1}
                precision={0}
                fluid
                value={form.sort_order}
                onChange={(_, { value }) =>
                  setForm((current) => ({
                    ...current,
                    sort_order: Number(value || 0),
                  }))
                }
              />
            </AppField>
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
          <AppFormRow>
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
