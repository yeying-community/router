import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Icon, Label, Modal, Table } from 'semantic-ui-react';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../helpers';

const MODE_LIST = 'list';
const MODE_CREATE = 'create';
const MODE_VIEW = 'view';
const MODE_EDIT = 'edit';

const createEmptyForm = () => ({
  id: '',
  name: '',
  description: '',
  billing_ratio: 1,
  sort_order: 0,
});

const createEmptyModelConfig = () => ({
  model: '',
  channel_id: '',
  upstream_model: '',
});

const sortCatalogRows = (items) =>
  [...items].sort((a, b) => {
    const aOrder = Number(a.sort_order || 0);
    const bOrder = Number(b.sort_order || 0);
    if (aOrder !== bOrder) {
      return aOrder - bOrder;
    }
    return (a.id || '').localeCompare(b.id || '');
  });

const buildFormFromRow = (row) => ({
  id: row?.id || '',
  name: row?.name || '',
  description: row?.description || '',
  billing_ratio: Number(row?.billing_ratio ?? 1),
  sort_order: Number(row?.sort_order || 0),
});

const toChannelOptions = (items) =>
  (Array.isArray(items) ? items : []).map((item) => ({
    key: item.id,
    text: `${item.name || item.id} (${item.id})`,
    value: item.id,
  }));

const toBoundChannelIDs = (items) =>
  (Array.isArray(items) ? items : [])
    .filter((item) => !!item.bound)
    .map((item) => item.id);

const toBoundChannelRows = (items) =>
  (Array.isArray(items) ? items : []).filter((item) => !!item.bound);

const encodeChannelModelOptionValue = (item) =>
  JSON.stringify({
    model: item?.model || '',
    upstream_model: item?.upstream_model || '',
  });

const decodeChannelModelOptionValue = (value) => {
  if (typeof value !== 'string' || value.trim() === '') {
    return { model: '', upstream_model: '' };
  }
  try {
    const parsed = JSON.parse(value);
    return {
      model: parsed?.model || '',
      upstream_model: parsed?.upstream_model || '',
    };
  } catch (error) {
    return { model: '', upstream_model: '' };
  }
};

const buildChannelLookup = (items) => {
  const lookup = {};
  (Array.isArray(items) ? items : []).forEach((item) => {
    if (!item?.id) return;
    lookup[item.id] = item;
  });
  return lookup;
};

const ensureSelectedChannelsHaveModelRows = (rows, selectedChannelIDs, channelLookup) => {
  const currentRows = Array.isArray(rows) ? rows : [];
  const selectedIDs = Array.isArray(selectedChannelIDs) ? selectedChannelIDs : [];
  if (selectedIDs.length === 0) {
    return currentRows;
  }
  const existingChannelIDs = new Set(
    currentRows
      .map((item) => (typeof item?.channel_id === 'string' ? item.channel_id.trim() : ''))
      .filter((item) => item !== '')
  );
  const additions = [];
  selectedIDs.forEach((channelID) => {
    if (!channelID || existingChannelIDs.has(channelID)) {
      return;
    }
    const channel = channelLookup[channelID];
    const models = Array.isArray(channel?.models) ? channel.models : [];
    models.forEach((item) => {
      additions.push({
        model: item?.model || item?.upstream_model || '',
        channel_id: channelID,
        upstream_model: item?.upstream_model || item?.model || '',
      });
    });
  });
  if (additions.length === 0) {
    return currentRows;
  }
  return [...currentRows, ...additions];
};

const formatChannelDisplayName = (item) => {
  if (!item) return '-';
  return item.name || item.id || '-';
};

const channelStatusColor = (status) => {
  const normalized = Number(status || 0);
  if (normalized === 1) return 'green';
  if (normalized === 4) return 'blue';
  return 'grey';
};

const actionBarStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'flex-start',
  gap: '8px',
  flexWrap: 'wrap',
  marginBottom: 12,
};

const GroupsManager = () => {
  const { t } = useTranslation();
  const [mode, setMode] = useState(MODE_LIST);
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');

  const [activeGroup, setActiveGroup] = useState(null);
  const [form, setForm] = useState(createEmptyForm());
  const [formChannelOptions, setFormChannelOptions] = useState([]);
  const [formChannelIDs, setFormChannelIDs] = useState([]);
  const [formChannelLoading, setFormChannelLoading] = useState(false);
  const [formModelChannels, setFormModelChannels] = useState([]);
  const [formModelConfigs, setFormModelConfigs] = useState([]);
  const [formModelLoading, setFormModelLoading] = useState(false);
  const [editModelSearchKeyword, setEditModelSearchKeyword] = useState('');

  const [detailChannelRows, setDetailChannelRows] = useState([]);
  const [detailChannelLoading, setDetailChannelLoading] = useState(false);
  const [detailModelRows, setDetailModelRows] = useState([]);
  const [detailModelLoading, setDetailModelLoading] = useState(false);
  const [detailModelSearchKeyword, setDetailModelSearchKeyword] = useState('');

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);

  const formModelChannelLookup = useMemo(
    () => buildChannelLookup(formModelChannels),
    [formModelChannels]
  );

  const selectedFormChannelOptions = useMemo(
    () => toChannelOptions(formModelChannels.filter((item) => formChannelIDs.includes(item.id))),
    [formModelChannels, formChannelIDs]
  );

  const loadCatalog = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/group/catalog');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.load_failed'));
        return;
      }
      setRows(sortCatalogRows(Array.isArray(data) ? data : []));
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadCatalog().then();
  }, [loadCatalog]);

  const visibleRows = useMemo(() => {
    const keyword = typeof searchKeyword === 'string' ? searchKeyword.trim().toLowerCase() : '';
    if (!keyword) {
      return rows;
    }
    return rows.filter((row) => {
      const channelNames = Array.isArray(row.channels)
        ? row.channels.map((item) => formatChannelDisplayName(item)).join(' ')
        : '';
      const haystacks = [row.id, row.name, row.description, channelNames];
      return haystacks.some((item) =>
        typeof item === 'string' ? item.toLowerCase().includes(keyword) : false
      );
    });
  }, [rows, searchKeyword]);

  useEffect(() => {
    if (mode !== MODE_EDIT) {
      return;
    }
    setFormModelConfigs((prev) =>
      ensureSelectedChannelsHaveModelRows(prev, formChannelIDs, formModelChannelLookup)
    );
  }, [mode, formChannelIDs, formModelChannelLookup]);

  const resetFormState = () => {
    setForm(createEmptyForm());
    setFormChannelOptions([]);
    setFormChannelIDs([]);
    setFormChannelLoading(false);
    setFormModelChannels([]);
    setFormModelConfigs([]);
    setFormModelLoading(false);
    setEditModelSearchKeyword('');
  };

  const resetDetailState = () => {
    setDetailChannelRows([]);
    setDetailChannelLoading(false);
    setDetailModelRows([]);
    setDetailModelLoading(false);
    setDetailModelSearchKeyword('');
  };

  const clearDeleteState = () => {
    setDeleteOpen(false);
    setDeleteTarget(null);
  };

  const closeDeleteModal = () => {
    if (submitting) return;
    clearDeleteState();
  };

  const fetchCreateChannelOptions = useCallback(async () => {
    setFormChannelLoading(true);
    try {
      const res = await API.get('/api/v1/admin/group/channel-options');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.bind_load_failed'));
        return;
      }
      setFormChannelOptions(toChannelOptions(data));
      setFormChannelIDs([]);
    } catch (error) {
      showError(error);
    } finally {
      setFormChannelLoading(false);
    }
  }, [t]);

  const fetchGroupChannels = useCallback(async (groupID) => {
    const encodedID = encodeURIComponent(groupID || '');
    const res = await API.get(`/api/v1/admin/group/${encodedID}/channels`);
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || t('group_manage.messages.bind_load_failed'));
    }
    return Array.isArray(data) ? data : [];
  }, [t]);

  const loadViewChannelRows = useCallback(async (groupID) => {
    setDetailChannelLoading(true);
    try {
      const rows = await fetchGroupChannels(groupID);
      setDetailChannelRows(toBoundChannelRows(rows));
    } catch (error) {
      showError(error);
    } finally {
      setDetailChannelLoading(false);
    }
  }, [fetchGroupChannels]);

  const loadViewModelRows = useCallback(async (groupID) => {
    setDetailModelLoading(true);
    try {
      const encodedID = encodeURIComponent(groupID || '');
      const res = await API.get(`/api/v1/admin/group/${encodedID}/models`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.model_load_failed'));
        return;
      }
      setDetailModelRows(Array.isArray(data) ? data : []);
    } catch (error) {
      showError(error);
    } finally {
      setDetailModelLoading(false);
    }
  }, [t]);

  const loadEditModelConfigs = useCallback(async (groupID) => {
    setFormChannelLoading(true);
    setFormModelLoading(true);
    try {
      const encodedID = encodeURIComponent(groupID || '');
      const res = await API.get(`/api/v1/admin/group/${encodedID}/model-configs`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.model_config_load_failed'));
        return;
      }
      const channels = Array.isArray(data?.channels) ? data.channels : [];
      const items = Array.isArray(data?.items) ? data.items : [];
      setFormModelChannels(channels);
      setFormChannelOptions(toChannelOptions(channels));
      setFormChannelIDs(toBoundChannelIDs(channels));
      setFormModelConfigs(items);
    } catch (error) {
      showError(error);
    } finally {
      setFormChannelLoading(false);
      setFormModelLoading(false);
    }
  }, [t]);

  const resetToList = () => {
    setMode(MODE_LIST);
    setActiveGroup(null);
    resetFormState();
    resetDetailState();
  };

  const backToList = () => {
    if (submitting) return;
    resetToList();
  };

  const openCreatePanel = () => {
    if (submitting) return;
    setMode(MODE_CREATE);
    setActiveGroup(null);
    resetDetailState();
    resetFormState();
    fetchCreateChannelOptions().then();
  };

  const openViewPanel = (row) => {
    if (!row || submitting) return;
    setMode(MODE_VIEW);
    setActiveGroup(row);
    resetFormState();
    resetDetailState();
    loadViewChannelRows(row.id || '').then();
    loadViewModelRows(row.id || '').then();
  };

  const openEditPanel = (row = activeGroup) => {
    if (!row || submitting) return;
    setMode(MODE_EDIT);
    setActiveGroup(row);
    setForm(buildFormFromRow(row));
    setFormChannelOptions([]);
    setFormChannelIDs([]);
    setFormModelChannels([]);
    setFormModelConfigs([]);
    setEditModelSearchKeyword('');
    loadEditModelConfigs(row.id || '').then();
  };

  const submitCreate = async () => {
    const id = (form.id || '').trim();
    if (id === '') {
      showInfo(t('group_manage.messages.id_required'));
      return;
    }
    const billingRatio = Number(form.billing_ratio ?? 1);
    if (!Number.isFinite(billingRatio) || billingRatio < 0) {
      showInfo(t('group_manage.messages.billing_ratio_invalid'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/v1/admin/group/', {
        id,
        name: (form.name || '').trim(),
        description: (form.description || '').trim(),
        billing_ratio: billingRatio,
        channel_ids: formChannelIDs,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.create_failed'));
        return;
      }
      await loadCatalog();
      showSuccess(t('group_manage.messages.create_success'));
      resetToList();
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const submitEdit = async () => {
    const id = (form.id || '').trim();
    if (id === '') {
      showInfo(t('group_manage.messages.id_required'));
      return;
    }
    const billingRatio = Number(form.billing_ratio ?? 1);
    if (!Number.isFinite(billingRatio) || billingRatio < 0) {
      showInfo(t('group_manage.messages.billing_ratio_invalid'));
      return;
    }
    const selectedChannelIDSet = new Set(formChannelIDs);
    const normalizedModelConfigs = [];
    const seenModelConfigKeys = new Set();
    for (const item of Array.isArray(formModelConfigs) ? formModelConfigs : []) {
      const model = (item?.model || '').trim();
      const channelID = (item?.channel_id || '').trim();
      const upstreamModel = (item?.upstream_model || '').trim();
      if (model === '' && channelID === '' && upstreamModel === '') {
        continue;
      }
      if (!selectedChannelIDSet.has(channelID)) {
        continue;
      }
      if (model === '' || channelID === '' || upstreamModel === '') {
        showInfo(t('group_manage.messages.model_config_incomplete'));
        return;
      }
      const dedupeKey = `${model}::${channelID}`;
      if (seenModelConfigKeys.has(dedupeKey)) {
        showInfo(t('group_manage.messages.model_config_duplicate'));
        return;
      }
      seenModelConfigKeys.add(dedupeKey);
      normalizedModelConfigs.push({
        model,
        channel_id: channelID,
        upstream_model: upstreamModel,
      });
    }
    setSubmitting(true);
    try {
      const res = await API.put('/api/v1/admin/group/', {
        id,
        name: (form.name || '').trim(),
        description: (form.description || '').trim(),
        billing_ratio: billingRatio,
        sort_order: Number(form.sort_order || 0),
        channel_ids: formChannelIDs,
        model_configs: normalizedModelConfigs,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.update_failed'));
        return;
      }
      await loadCatalog();
      setActiveGroup(data);
      showSuccess(t('group_manage.messages.update_success'));
      setMode(MODE_VIEW);
      resetFormState();
      loadViewChannelRows(data.id || '').then();
      loadViewModelRows(data.id || '').then();
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const toggleEnabled = async (row) => {
    if (!row || submitting) return;
    setSubmitting(true);
    try {
      const res = await API.put('/api/v1/admin/group/', {
        id: row.id,
        name: row.name || '',
        description: row.description || '',
        billing_ratio: Number(row.billing_ratio ?? 1),
        sort_order: Number(row.sort_order || 0),
        enabled: !row.enabled,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.update_failed'));
        return;
      }
      await loadCatalog();
      if (activeGroup?.id === data.id) {
        setActiveGroup(data);
      }
      showSuccess(t('group_manage.messages.update_success'));
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const openDeleteModal = (row) => {
    if (!row || submitting) return;
    setDeleteTarget(row);
    setDeleteOpen(true);
  };

  const submitDelete = async () => {
    if (!deleteTarget || submitting) return;
    setSubmitting(true);
    try {
      const encodedID = encodeURIComponent(deleteTarget.id || '');
      const res = await API.delete(`/api/v1/admin/group/${encodedID}`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.delete_failed'));
        return;
      }
      await loadCatalog();
      showSuccess(t('group_manage.messages.delete_success'));
      clearDeleteState();
      if (activeGroup?.id === deleteTarget.id) {
        resetToList();
      }
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const renderGroupStatus = (enabled) =>
    enabled ? (
      <Label basic color='green'>
        {t('group_manage.status.enabled')}
      </Label>
    ) : (
      <Label basic color='grey'>
        {t('group_manage.status.disabled')}
      </Label>
    );

  const renderChannelStatus = (status) => {
    const normalized = Number(status || 0);
    if (normalized === 1) {
      return (
        <Label basic color='green'>
          {t('channel.table.status_enabled')}
        </Label>
      );
    }
    if (normalized === 4) {
      return (
        <Label basic color='blue'>
          {t('channel.table.status_creating')}
        </Label>
      );
    }
    return (
      <Label basic color='grey'>
        {t('channel.table.status_disabled')}
      </Label>
    );
  };

  const renderList = () => (
    <>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '12px',
          flexWrap: 'wrap',
          marginBottom: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          <Button
            type='button'
            size='tiny'
            disabled={submitting || loading}
            onClick={openCreatePanel}
          >
            {t('group_manage.buttons.add')}
          </Button>
          <Button
            type='button'
            size='tiny'
            disabled={submitting}
            loading={loading}
            onClick={loadCatalog}
          >
            {t('group_manage.buttons.refresh')}
          </Button>
        </div>
        <Form style={{ width: '320px', maxWidth: '100%' }}>
          <Form.Input
            icon='search'
            iconPosition='left'
            placeholder={t('group_manage.search')}
            value={searchKeyword}
            onChange={(e, { value }) => setSearchKeyword(value || '')}
          />
        </Form>
      </div>

      <Table basic='very' compact size='small' className='router-hover-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>{t('group_manage.table.id')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.name')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.description')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.channels')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.billing_ratio')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.status')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.updated_at')}</Table.HeaderCell>
            <Table.HeaderCell style={{ width: '220px' }}>
              {t('group_manage.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {visibleRows.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan={8} textAlign='center'>
                {loading
                  ? t('group_manage.messages.loading')
                  : t('group_manage.messages.empty')}
              </Table.Cell>
            </Table.Row>
          ) : (
            visibleRows.map((row) => (
              <Table.Row
                key={row.id}
                onClick={() => openViewPanel(row)}
                style={{ cursor: submitting || loading ? 'default' : 'pointer' }}
              >
                <Table.Cell>{row.id}</Table.Cell>
                <Table.Cell>{row.name || '-'}</Table.Cell>
                <Table.Cell>{row.description || '-'}</Table.Cell>
                <Table.Cell>
                  {Array.isArray(row.channels) && row.channels.length > 0 ? (
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '6px',
                        flexWrap: 'wrap',
                      }}
                    >
                      {row.channels.map((item) => (
                        <Label key={item.id} size='tiny'>
                          {formatChannelDisplayName(item)}
                        </Label>
                      ))}
                    </div>
                  ) : (
                    '-'
                  )}
                </Table.Cell>
                <Table.Cell>{Number(row.billing_ratio ?? 1).toFixed(2)}</Table.Cell>
                <Table.Cell>{renderGroupStatus(row.enabled)}</Table.Cell>
                <Table.Cell>{row.updated_at ? timestamp2string(row.updated_at) : '-'}</Table.Cell>
                <Table.Cell>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '6px',
                      flexWrap: 'wrap',
                    }}
                  >
                    <Button
                      size='tiny'
                      color={row.enabled ? 'orange' : 'green'}
                      disabled={submitting || loading}
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleEnabled(row);
                      }}
                    >
                      {row.enabled
                        ? t('group_manage.buttons.disable')
                        : t('group_manage.buttons.enable')}
                    </Button>
                    <Button
                      size='tiny'
                      negative
                      disabled={submitting || loading}
                      onClick={(e) => {
                        e.stopPropagation();
                        openDeleteModal(row);
                      }}
                    >
                      {t('group_manage.buttons.delete')}
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>
            ))
          )}
        </Table.Body>
      </Table>
    </>
  );

  const renderBoundChannelsTable = (items, loadingState) => (
    <div style={{ marginTop: 12 }}>
      <div style={{ fontWeight: 600, marginBottom: 8 }}>
        {t('group_manage.detail.bound_channels')}
      </div>
      <Table compact size='small' celled>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>{t('channel.table.id')}</Table.HeaderCell>
            <Table.HeaderCell>{t('channel.table.name')}</Table.HeaderCell>
            <Table.HeaderCell>{t('channel.table.type')}</Table.HeaderCell>
            <Table.HeaderCell>{t('channel.table.status')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {loadingState ? (
            <Table.Row>
              <Table.Cell colSpan={4} textAlign='center'>
                {t('group_manage.messages.loading')}
              </Table.Cell>
            </Table.Row>
          ) : items.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan={4} textAlign='center'>
                {t('group_manage.detail.empty_channels')}
              </Table.Cell>
            </Table.Row>
          ) : (
            items.map((item) => (
              <Table.Row key={item.id}>
                <Table.Cell>{item.id}</Table.Cell>
                <Table.Cell>{item.name || '-'}</Table.Cell>
                <Table.Cell>{item.protocol || '-'}</Table.Cell>
                <Table.Cell>{renderChannelStatus(item.status)}</Table.Cell>
              </Table.Row>
            ))
          )}
        </Table.Body>
      </Table>
    </div>
  );

  const renderModelSummaryTable = (items, loadingState) => {
    const keyword = detailModelSearchKeyword.trim().toLowerCase();
    const visibleItems =
      keyword === ''
        ? items
        : items.filter((item) => {
            const modelName = (item.model || '').toLowerCase();
            const channels = Array.isArray(item.channels)
              ? item.channels
                  .map((channel) =>
                    `${formatChannelDisplayName(channel)} ${channel.protocol || ''}`.toLowerCase()
                  )
                  .join(' ')
              : '';
            return modelName.includes(keyword) || channels.includes(keyword);
          });

    return (
      <div style={{ marginTop: 12 }}>
        <div
          style={{
            marginBottom: 8,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 8,
            flexWrap: 'wrap',
          }}
        >
          <div style={{ fontWeight: 600 }}>
            {t('group_manage.detail.supported_models')}
          </div>
          <Form.Input
            size='small'
            icon='search'
            iconPosition='left'
            style={{ width: 280, maxWidth: '100%' }}
            placeholder={t('group_manage.detail.model_search_placeholder')}
            value={detailModelSearchKeyword}
            onChange={(e, { value }) => setDetailModelSearchKeyword(value || '')}
          />
        </div>
        <Table compact size='small' celled>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>{t('group_manage.detail.model')}</Table.HeaderCell>
              <Table.HeaderCell collapsing>{t('group_manage.detail.channel_count')}</Table.HeaderCell>
              <Table.HeaderCell>{t('group_manage.detail.model_channels')}</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {loadingState ? (
              <Table.Row>
                <Table.Cell colSpan={3} textAlign='center'>
                  {t('group_manage.messages.loading')}
                </Table.Cell>
              </Table.Row>
            ) : visibleItems.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan={3} textAlign='center'>
                  {t('group_manage.detail.empty_models')}
                </Table.Cell>
              </Table.Row>
            ) : (
              visibleItems.map((item) => (
                <Table.Row key={item.model}>
                  <Table.Cell style={{ minWidth: 240 }}>{item.model || '-'}</Table.Cell>
                  <Table.Cell textAlign='center'>
                    {Array.isArray(item.channels) ? item.channels.length : 0}
                  </Table.Cell>
                  <Table.Cell>
                    {Array.isArray(item.channels) && item.channels.length > 0 ? (
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '6px',
                          flexWrap: 'wrap',
                        }}
                      >
                        {item.channels.map((channel) => (
                          <Label
                            key={`${item.model}-${channel.id}`}
                            size='tiny'
                            basic
                            color={channelStatusColor(channel.status)}
                          >
                            {formatChannelDisplayName(channel)}
                            {` · ${channel.protocol || '-'}`}
                          </Label>
                        ))}
                      </div>
                    ) : (
                      '-'
                    )}
                  </Table.Cell>
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>
    );
  };

  const getChannelModelOptions = (channelID) => {
    const channel = formModelChannelLookup[channelID];
    const models = Array.isArray(channel?.models) ? channel.models : [];
    return models.map((item) => ({
      key: `${channelID}-${item.upstream_model}-${item.model}`,
      text: item.label || item.model || item.upstream_model || '-',
      value: encodeChannelModelOptionValue(item),
    }));
  };

  const resolveChannelModelOptionValue = (item) => {
    const channelID = (item?.channel_id || '').trim();
    if (!channelID) {
      return '';
    }
    const channel = formModelChannelLookup[channelID];
    const models = Array.isArray(channel?.models) ? channel.models : [];
    const upstreamModel = (item?.upstream_model || '').trim();
    if (upstreamModel !== '') {
      const matched = models.find((row) => (row?.upstream_model || '') === upstreamModel);
      if (matched) {
        return encodeChannelModelOptionValue(matched);
      }
    }
    const modelName = (item?.model || '').trim();
    if (modelName !== '') {
      const matched = models.find((row) => (row?.model || '') === modelName);
      if (matched) {
        return encodeChannelModelOptionValue(matched);
      }
    }
    return '';
  };

  const visibleEditModelConfigs = useMemo(() => {
    const keyword = editModelSearchKeyword.trim().toLowerCase();
    const selectedChannelIDSet = new Set(formChannelIDs);
    const entries = (Array.isArray(formModelConfigs) ? formModelConfigs : [])
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => {
        const channelID = (item?.channel_id || '').trim();
        return channelID === '' || selectedChannelIDSet.has(channelID);
      });
    if (keyword === '') {
      return entries;
    }
    return entries.filter(({ item }) => {
      const channel = formModelChannelLookup[item?.channel_id || ''];
      const haystacks = [
        item?.model || '',
        item?.upstream_model || '',
        channel?.name || '',
        channel?.id || '',
        channel?.protocol || '',
      ];
      return haystacks.some((value) => value.toLowerCase().includes(keyword));
    });
  }, [editModelSearchKeyword, formChannelIDs, formModelConfigs, formModelChannelLookup]);

  const addEmptyModelConfigRow = () => {
    setFormModelConfigs((prev) => [createEmptyModelConfig(), ...(Array.isArray(prev) ? prev : [])]);
  };

  const updateModelConfigRow = (index, updater) => {
    setFormModelConfigs((prev) =>
      (Array.isArray(prev) ? prev : []).map((item, itemIndex) => {
        if (itemIndex !== index) {
          return item;
        }
        return typeof updater === 'function' ? updater(item) : item;
      })
    );
  };

  const removeModelConfigRow = (index) => {
    setFormModelConfigs((prev) =>
      (Array.isArray(prev) ? prev : []).filter((item, itemIndex) => itemIndex !== index)
    );
  };

  const renderEditModelConfigTable = () => (
    <div style={{ marginTop: 18 }}>
      <div
        style={{
          marginBottom: 8,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 8,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <div style={{ fontWeight: 600 }}>{t('group_manage.edit.model_configs')}</div>
          <Button
            type='button'
            size='tiny'
            disabled={submitting || formModelLoading}
            onClick={addEmptyModelConfigRow}
          >
            {t('group_manage.buttons.add_model')}
          </Button>
        </div>
        <Form.Input
          size='small'
          icon='search'
          iconPosition='left'
          style={{ width: 280, maxWidth: '100%' }}
          placeholder={t('group_manage.edit.model_search_placeholder')}
          value={editModelSearchKeyword}
          onChange={(e, { value }) => setEditModelSearchKeyword(value || '')}
        />
      </div>
      <Table compact size='small' celled>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell style={{ minWidth: 260 }}>
              {t('group_manage.edit.model')}
            </Table.HeaderCell>
            <Table.HeaderCell style={{ minWidth: 240 }}>
              {t('group_manage.edit.channel')}
            </Table.HeaderCell>
            <Table.HeaderCell style={{ minWidth: 280 }}>
              {t('group_manage.edit.upstream_model')}
            </Table.HeaderCell>
            <Table.HeaderCell collapsing>
              {t('group_manage.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {formModelLoading ? (
            <Table.Row>
              <Table.Cell colSpan={4} textAlign='center'>
                {t('group_manage.messages.loading')}
              </Table.Cell>
            </Table.Row>
          ) : visibleEditModelConfigs.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan={4} textAlign='center'>
                {t('group_manage.edit.empty_model_configs')}
              </Table.Cell>
            </Table.Row>
          ) : (
            visibleEditModelConfigs.map(({ item, index }) => {
              const modelOptions = getChannelModelOptions(item?.channel_id || '');
              return (
                <Table.Row key={`group-model-config-${index}`}>
                  <Table.Cell>
                    <Form.Input
                      size='small'
                      fluid
                      placeholder={t('group_manage.edit.model_placeholder')}
                      value={item?.model || ''}
                      onChange={(e, { value }) =>
                        updateModelConfigRow(index, (current) => ({
                          ...current,
                          model: value || '',
                        }))
                      }
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Dropdown
                      fluid
                      selection
                      search
                      size='small'
                      options={selectedFormChannelOptions}
                      placeholder={t('group_manage.edit.channel_placeholder')}
                      value={item?.channel_id || ''}
                      onChange={(e, { value }) => {
                        const nextChannelID = value || '';
                        const nextChannel = formModelChannelLookup[nextChannelID];
                        const nextModels = Array.isArray(nextChannel?.models) ? nextChannel.models : [];
                        const firstModel = nextModels[0] || null;
                        updateModelConfigRow(index, (current) => ({
                          ...current,
                          channel_id: nextChannelID,
                          upstream_model: firstModel?.upstream_model || '',
                          model:
                            (current?.model || '').trim() !== ''
                              ? current.model
                              : firstModel?.model || '',
                        }));
                      }}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Dropdown
                      fluid
                      selection
                      search
                      size='small'
                      disabled={(item?.channel_id || '') === '' || modelOptions.length === 0}
                      options={modelOptions}
                      placeholder={t('group_manage.edit.upstream_model_placeholder')}
                      value={resolveChannelModelOptionValue(item)}
                      onChange={(e, { value }) => {
                        const decoded = decodeChannelModelOptionValue(value);
                        updateModelConfigRow(index, (current) => ({
                          ...current,
                          upstream_model: decoded.upstream_model || '',
                          model:
                            (current?.model || '').trim() !== ''
                              ? current.model
                              : decoded.model || '',
                        }));
                      }}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Button
                      type='button'
                      size='tiny'
                      negative
                      disabled={submitting}
                      onClick={() => removeModelConfigRow(index)}
                    >
                      {t('group_manage.buttons.delete')}
                    </Button>
                  </Table.Cell>
                </Table.Row>
              );
            })
          )}
        </Table.Body>
      </Table>
    </div>
  );

  const renderView = () => {
    if (!activeGroup) return null;
    return (
      <div>
        <div style={actionBarStyle}>
          <Button type='button' onClick={backToList} disabled={submitting}>
            <Icon name='undo' />
            {t('group_manage.buttons.back')}
          </Button>
          <Button type='button' color='blue' disabled={submitting} onClick={() => openEditPanel()}>
            <Icon name='edit' />
            {t('group_manage.buttons.edit')}
          </Button>
        </div>
        <Form>
          <Form.Group widths='equal'>
            <Form.Input
              label={t('group_manage.form.id')}
              value={activeGroup.id || ''}
              readOnly
            />
            <Form.Input
              label={t('group_manage.form.name')}
              value={activeGroup.name || ''}
              readOnly
            />
          </Form.Group>
          <Form.TextArea
            label={t('group_manage.form.description')}
            value={activeGroup.description || ''}
            readOnly
          />
          <Form.Group widths='equal'>
            <Form.Input
              label={t('group_manage.form.billing_ratio')}
              value={Number(activeGroup.billing_ratio ?? 1).toFixed(2)}
              readOnly
            />
            <Form.Input
              label={t('group_manage.table.status')}
              value={
                activeGroup.enabled
                  ? t('group_manage.status.enabled')
                  : t('group_manage.status.disabled')
              }
              readOnly
            />
          </Form.Group>
          <Form.Group widths='equal'>
            <Form.Input
              label={t('group_manage.form.sort_order')}
              value={activeGroup.sort_order || 0}
              readOnly
            />
            <Form.Input
              label={t('group_manage.table.updated_at')}
              value={activeGroup.updated_at ? timestamp2string(activeGroup.updated_at) : '-'}
              readOnly
            />
          </Form.Group>
        </Form>
        {renderModelSummaryTable(detailModelRows, detailModelLoading)}
        {renderBoundChannelsTable(detailChannelRows, detailChannelLoading)}
      </div>
    );
  };

  const renderEdit = () => (
    <div>
      <div style={actionBarStyle}>
        <Button type='button' onClick={() => setMode(MODE_VIEW)} disabled={submitting}>
          {t('group_manage.buttons.cancel')}
        </Button>
        <Button type='button' color='blue' loading={submitting} disabled={submitting} onClick={submitEdit}>
          <Icon name='check' />
          {t('group_manage.buttons.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Group widths='equal'>
          <Form.Input
            label={t('group_manage.form.id')}
            value={form.id}
            readOnly
          />
          <Form.Input
            label={t('group_manage.form.name')}
            placeholder={t('group_manage.form.name_placeholder')}
            value={form.name}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, name: value || '' }))
            }
          />
        </Form.Group>
        <Form.TextArea
          label={t('group_manage.form.description')}
          placeholder={t('group_manage.form.description_placeholder')}
          value={form.description}
          onChange={(e) =>
            setForm((prev) => ({
              ...prev,
              description: e.target.value,
            }))
          }
        />
        <Form.Group widths='equal'>
          <Form.Input
            type='number'
            min='0'
            step='0.01'
            label={t('group_manage.form.billing_ratio')}
            placeholder={t('group_manage.form.billing_ratio_placeholder')}
            value={form.billing_ratio}
            onChange={(e) =>
              setForm((prev) => ({
                ...prev,
                billing_ratio: e.target.value,
              }))
            }
          />
          <Form.Input
            type='number'
            label={t('group_manage.form.sort_order')}
            value={form.sort_order}
            onChange={(e) =>
              setForm((prev) => ({
                ...prev,
                sort_order: Number(e.target.value || 0),
              }))
            }
          />
        </Form.Group>
        <Form.Dropdown
          fluid
          multiple
          search
          selection
          loading={formChannelLoading || formModelLoading}
          disabled={formChannelLoading || formModelLoading || submitting}
          label={t('group_manage.form.channels')}
          placeholder={t('group_manage.form.channels_placeholder')}
          options={formChannelOptions}
          value={formChannelIDs}
          onChange={(e, { value }) =>
            setFormChannelIDs(Array.isArray(value) ? value : [])
          }
        />
      </Form>
      {renderEditModelConfigTable()}
    </div>
  );

  const renderCreate = () => (
    <div>
      <div style={actionBarStyle}>
        <Button type='button' onClick={backToList} disabled={submitting}>
          {t('group_manage.buttons.cancel')}
        </Button>
        <Button type='button' color='blue' loading={submitting} disabled={submitting} onClick={submitCreate}>
          <Icon name='check' />
          {t('group_manage.buttons.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Group widths='equal'>
          <Form.Input
            required
            label={t('group_manage.form.id')}
            placeholder={t('group_manage.form.id_placeholder')}
            value={form.id}
            onChange={(e) =>
              setForm((prev) => ({ ...prev, id: e.target.value }))
            }
          />
          <Form.Input
            label={t('group_manage.form.name')}
            placeholder={t('group_manage.form.name_placeholder')}
            value={form.name}
            onChange={(e) =>
              setForm((prev) => ({ ...prev, name: e.target.value }))
            }
          />
        </Form.Group>
        <Form.TextArea
          label={t('group_manage.form.description')}
          placeholder={t('group_manage.form.description_placeholder')}
          value={form.description}
          onChange={(e) =>
            setForm((prev) => ({
              ...prev,
              description: e.target.value,
            }))
          }
        />
        <Form.Group widths='equal'>
          <Form.Input
            type='number'
            min='0'
            step='0.01'
            label={t('group_manage.form.billing_ratio')}
            placeholder={t('group_manage.form.billing_ratio_placeholder')}
            value={form.billing_ratio}
            onChange={(e) =>
              setForm((prev) => ({
                ...prev,
                billing_ratio: e.target.value,
              }))
            }
          />
          <Form.Dropdown
            fluid
            multiple
            search
            selection
            loading={formChannelLoading}
            disabled={formChannelLoading || submitting}
            label={t('group_manage.form.channels')}
            placeholder={t('group_manage.form.channels_placeholder')}
            options={formChannelOptions}
            value={formChannelIDs}
            onChange={(e, { value }) =>
              setFormChannelIDs(Array.isArray(value) ? value : [])
            }
          />
        </Form.Group>
      </Form>
    </div>
  );

  return (
    <>
      {mode === MODE_CREATE
        ? renderCreate()
        : mode === MODE_EDIT
          ? renderEdit()
          : mode === MODE_VIEW
            ? renderView()
            : renderList()}

      <Modal open={deleteOpen} onClose={closeDeleteModal} size='tiny'>
        <Modal.Header>{t('group_manage.modal.delete_title')}</Modal.Header>
        <Modal.Content>
          {t('group_manage.modal.delete_confirm', {
            id: deleteTarget?.id || '',
          })}
        </Modal.Content>
        <Modal.Actions>
          <Button onClick={closeDeleteModal} disabled={submitting}>
            {t('group_manage.buttons.cancel')}
          </Button>
          <Button negative onClick={submitDelete} loading={submitting}>
            {t('group_manage.buttons.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    </>
  );
};

export default GroupsManager;
