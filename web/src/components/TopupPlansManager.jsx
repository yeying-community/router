import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Checkbox,
  Dropdown,
  Form,
  Header,
  Modal,
  Table,
} from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../helpers';

const createEmptyPlan = () => ({
  id: '',
  name: '',
  group_id: '',
  group_name: '',
  amount: 0,
  amount_currency: 'CNY',
  quota_amount: 0,
  quota_currency: 'USD',
  enabled: true,
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
      enabled: Boolean(row?.enabled),
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
        enabled: Boolean(form.enabled),
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

  return (
    <>
      <div className='router-toolbar router-block-gap-sm' style={{ marginBottom: '1rem' }}>
        <div className='router-toolbar-start'>
          <Header as='h3' className='router-section-title' style={{ margin: 0 }}>
            {t('topup.manage.title')}
          </Header>
        </div>
        <div className='router-toolbar-end'>
          <Button className='router-section-button' primary onClick={openCreate}>
            {t('common.add')}
          </Button>
          <Button className='router-section-button' onClick={() => loadPlans()} loading={loading}>
            {t('common.refresh')}
          </Button>
        </div>
      </div>

      <div className='router-table-scroll-x'>
        <Table celled selectable className='router-table router-list-table router-topup-plan-table'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell className='router-topup-plan-name-cell'>
                {t('topup.manage.columns.name')}
              </Table.HeaderCell>
              <Table.HeaderCell>{t('topup.manage.columns.group')}</Table.HeaderCell>
              <Table.HeaderCell className='router-topup-plan-amount-cell'>
                {t('topup.manage.columns.pay_amount')}
              </Table.HeaderCell>
              <Table.HeaderCell className='router-topup-plan-quota-cell'>
                {t('topup.manage.columns.credited_amount')}
              </Table.HeaderCell>
              <Table.HeaderCell className='router-topup-plan-status-cell'>
                {t('topup.manage.columns.enabled')}
              </Table.HeaderCell>
              <Table.HeaderCell className='router-table-action-cell router-topup-plan-action-cell'>
                {t('common.operation')}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {plans.map((row, index) => (
              <Table.Row key={row.id || index}>
                <Table.Cell className='router-topup-plan-name-cell'>{row.name || '-'}</Table.Cell>
                <Table.Cell>{row.group_name || row.group_id || '-'}</Table.Cell>
                <Table.Cell className='router-topup-plan-amount-cell'>{`${row.amount} ${row.amount_currency}`}</Table.Cell>
                <Table.Cell className='router-topup-plan-quota-cell'>{`${row.quota_amount} ${row.quota_currency}`}</Table.Cell>
                <Table.Cell className='router-topup-plan-status-cell'>
                  {row.enabled ? t('common.enabled') : t('common.disabled')}
                </Table.Cell>
                <Table.Cell className='router-topup-plan-action-cell'>
                  <div className='router-action-group-tight'>
                    <Button
                      className='router-inline-button'
                      onClick={() => openEdit(row, index)}
                    >
                      {t('common.edit')}
                    </Button>
                    <Button
                      className='router-inline-button'
                      onClick={() => {
                        setActiveRow(row);
                        setDeleteOpen(true);
                      }}
                    >
                      {t('common.delete')}
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table>
      </div>

      <Modal open={editOpen} size='tiny' onClose={() => setEditOpen(false)}>
        <Modal.Header>
          {isCreating ? t('topup.manage.create_title') : t('topup.manage.edit_title')}
        </Modal.Header>
        <Modal.Content>
          <Form>
            <Form.Input
              label={t('topup.manage.columns.name')}
              value={form.name}
              onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
            />
            <Form.Field>
              <label>{t('topup.manage.columns.group')}</label>
              <Dropdown
                fluid
                search
                selection
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
            </Form.Field>
            <Form.Input
              label={t('topup.manage.columns.pay_amount')}
              type='number'
              min='0'
              step='0.01'
              value={form.amount}
              onChange={(event) =>
                setForm((current) => ({ ...current, amount: Number(event.target.value || 0) }))
              }
            />
            <Form.Input
              label={t('topup.manage.columns.credited_amount')}
              type='number'
              min='0'
              step='0.01'
              value={form.quota_amount}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  quota_amount: Number(event.target.value || 0),
                }))
              }
            />
            <Form.Field>
              <Checkbox
                toggle
                checked={form.enabled}
                label={t('topup.manage.columns.enabled')}
                onChange={(_, data) => setForm((current) => ({ ...current, enabled: Boolean(data.checked) }))}
              />
            </Form.Field>
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button className='router-section-button' onClick={() => setEditOpen(false)}>
            {t('common.cancel')}
          </Button>
          <Button className='router-section-button' primary onClick={savePlan} loading={saving}>
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      <Modal open={deleteOpen} size='tiny' onClose={() => setDeleteOpen(false)}>
        <Modal.Header>{t('topup.manage.delete_title')}</Modal.Header>
        <Modal.Content>
          {t('topup.manage.delete_confirm', {
            name: activeRow?.name || '-',
          })}
        </Modal.Content>
        <Modal.Actions>
          <Button className='router-section-button' onClick={() => setDeleteOpen(false)}>
            {t('common.cancel')}
          </Button>
          <Button className='router-section-button' primary onClick={removePlan} loading={saving}>
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    </>
  );
};

export default TopupPlansManager;
