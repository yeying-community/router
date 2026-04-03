import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Breadcrumb, Button, Checkbox, Form, Header, Label, Modal, Table } from 'semantic-ui-react';
import { useLocation, useNavigate } from 'react-router-dom';
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
  enabled: true,
  priority: null,
});

const sortGroupModelConfigRows = (items) =>
  [...(Array.isArray(items) ? items : [])].sort((a, b) => {
    const modelDiff = (a?.model || '').localeCompare(b?.model || '');
    if (modelDiff !== 0) {
      return modelDiff;
    }
    const priorityDiff =
      toSafePriorityNumber(b?.priority, 0) - toSafePriorityNumber(a?.priority, 0);
    if (priorityDiff !== 0) {
      return priorityDiff;
    }
    const channelNameDiff = (a?.channel_name || '').localeCompare(b?.channel_name || '');
    if (channelNameDiff !== 0) {
      return channelNameDiff;
    }
    return (a?.channel_id || '').localeCompare(b?.channel_id || '');
  });

const sortCatalogRows = (items) =>
  [...items].sort((a, b) => {
    const aOrder = Number(a.sort_order || 0);
    const bOrder = Number(b.sort_order || 0);
    if (aOrder !== bOrder) {
      return aOrder - bOrder;
    }
    return (a.name || '').localeCompare(b.name || '');
  });

const buildFormFromRow = (row) => ({
  id: row?.id || '',
  name: row?.name || '',
  description: row?.description || '',
  billing_ratio: Number(row?.billing_ratio ?? 1),
  sort_order: Number(row?.sort_order || 0),
});

const toChannelOptions = (items) =>
  (Array.isArray(items) ? items : [])
    .filter((item) => Number(item?.status || 0) === 1)
    .map((item) => ({
      key: item.id,
      text: item.name || item.id,
      value: item.id,
    }));

const toBoundChannelIDs = (items) =>
  (Array.isArray(items) ? items : [])
    .filter((item) => !!item.bound && Number(item?.status || 0) === 1)
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
        priority: channel?.priority ?? null,
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

const toSafePriorityNumber = (value, fallback = 0) => {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return Math.trunc(Number(fallback) || 0);
  }
  return Math.trunc(parsed);
};

const formatPriorityLabel = (value) => `P${toSafePriorityNumber(value, 0)}`;

const GroupsManager = ({ detailGroupId = '' }) => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
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
  const [detailEditingSection, setDetailEditingSection] = useState('');
  const [detailModelModalOpen, setDetailModelModalOpen] = useState(false);
  const [detailModelEditTarget, setDetailModelEditTarget] = useState('');
  const [detailModelChannelDrafts, setDetailModelChannelDrafts] = useState([]);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);

  const normalizedDetailGroupId = (detailGroupId || '').toString().trim();
  const isDetailRoute = normalizedDetailGroupId !== '';

  const currentPagePath = useMemo(
    () => `${location.pathname}${location.search}${location.hash}`,
    [location.hash, location.pathname, location.search]
  );
  const returnPath = useMemo(() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);

  const formModelChannelLookup = useMemo(
    () => buildChannelLookup(formModelChannels),
    [formModelChannels]
  );

  const selectedFormChannelOptions = useMemo(
    () => toChannelOptions(formModelChannels.filter((item) => formChannelIDs.includes(item.id))),
    [formModelChannels, formChannelIDs]
  );
  const detailBasicEditing = detailEditingSection === 'basic';
  const detailChannelsEditing = detailEditingSection === 'channels';
  const detailSectionLocked = detailEditingSection !== '';
  const detailBasicEditLocked = detailSectionLocked && !detailBasicEditing;
  const detailChannelsEditLocked = detailSectionLocked && !detailChannelsEditing;
  const detailModelsEditLocked = detailSectionLocked;

  const fetchAllGroups = useCallback(async () => {
    const items = [];
    let page = 1;
    while (page <= 50) {
      const res = await API.get('/api/v1/admin/groups', {
        params: {
          page,
          page_size: 100,
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        throw new Error(message || t('group_manage.messages.load_failed'));
      }
      const pageItems = Array.isArray(data?.items) ? data.items : [];
      items.push(...pageItems);
      const total = Number(data?.total || pageItems.length || 0);
      if (pageItems.length === 0 || items.length >= total || pageItems.length < 100) {
        break;
      }
      page += 1;
    }
    return items;
  }, [t]);

  const loadCatalog = useCallback(async () => {
    setLoading(true);
    try {
      const items = await fetchAllGroups();
      setRows(sortCatalogRows(Array.isArray(items) ? items : []));
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, [fetchAllGroups]);

  useEffect(() => {
    if (isDetailRoute) {
      return;
    }
    loadCatalog().then();
  }, [isDetailRoute, loadCatalog]);

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
    setDetailEditingSection('');
    setDetailModelModalOpen(false);
    setDetailModelEditTarget('');
    setDetailModelChannelDrafts([]);
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
      const rows = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/channels', {
          params: {
            page,
            page_size: 100,
            compact: 1,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('group_manage.messages.bind_load_failed'));
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        rows.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (pageItems.length === 0 || rows.length >= total || pageItems.length < 100) {
          break;
        }
        page += 1;
      }
      setFormChannelOptions(toChannelOptions(rows));
      setFormChannelIDs([]);
    } catch (error) {
      showError(error);
    } finally {
      setFormChannelLoading(false);
    }
  }, [t]);

  const fetchGroupModelConfigPayload = useCallback(async (groupID) => {
    const encodedID = encodeURIComponent(groupID || '');
    const res = await API.get(`/api/v1/admin/group/${encodedID}/model-configs`);
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || t('group_manage.messages.model_config_load_failed'));
    }
    return {
      channels: Array.isArray(data?.channels) ? data.channels : [],
      items: sortGroupModelConfigRows(Array.isArray(data?.items) ? data.items : []),
    };
  }, [t]);

  const applyDetailModelConfigPayload = useCallback((payload) => {
    const channels = Array.isArray(payload?.channels) ? payload.channels : [];
    const items = sortGroupModelConfigRows(Array.isArray(payload?.items) ? payload.items : []);
    setDetailChannelRows(toBoundChannelRows(channels));
    setDetailModelRows(items);
  }, []);

  const loadDetailModelConfigState = useCallback(async (groupID) => {
    setDetailChannelLoading(true);
    setDetailModelLoading(true);
    try {
      const payload = await fetchGroupModelConfigPayload(groupID);
      applyDetailModelConfigPayload(payload);
      return payload;
    } catch (error) {
      showError(error);
      return null;
    } finally {
      setDetailChannelLoading(false);
      setDetailModelLoading(false);
    }
  }, [applyDetailModelConfigPayload, fetchGroupModelConfigPayload]);

  const loadEditModelConfigs = useCallback(async (groupID) => {
    setFormChannelLoading(true);
    setFormModelLoading(true);
    try {
      const payload = await fetchGroupModelConfigPayload(groupID);
      const channels = Array.isArray(payload?.channels) ? payload.channels : [];
      const items = sortGroupModelConfigRows(Array.isArray(payload?.items) ? payload.items : []);
      setFormModelChannels(channels);
      setFormChannelOptions(toChannelOptions(channels));
      setFormChannelIDs(toBoundChannelIDs(channels));
      setFormModelConfigs(items);
      return payload;
    } catch (error) {
      showError(error);
      return null;
    } finally {
      setFormChannelLoading(false);
      setFormModelLoading(false);
    }
  }, [fetchGroupModelConfigPayload]);

  const applySavedDetailModelState = useCallback((items, channels, selectedChannelIDs) => {
    const nextItems = sortGroupModelConfigRows(Array.isArray(items) ? items : []);
    const nextChannels = Array.isArray(channels) ? channels : [];
    const normalizedSelectedChannelIDs = Array.isArray(selectedChannelIDs)
      ? selectedChannelIDs
      : toBoundChannelIDs(nextChannels);
    setDetailModelRows(nextItems);
    setDetailChannelRows(toBoundChannelRows(nextChannels));
    setFormModelChannels(nextChannels);
    setFormChannelOptions(toChannelOptions(nextChannels));
    setFormChannelIDs(normalizedSelectedChannelIDs);
    setFormModelConfigs(nextItems);
  }, []);

  const loadGroupDetail = useCallback(
    async (groupID) => {
      const normalizedGroupID = (groupID || '').toString().trim();
      if (normalizedGroupID === '') {
        navigate('/admin/group', { replace: true });
        return;
      }
      setLoading(true);
      try {
        const encodedID = encodeURIComponent(normalizedGroupID);
        const res = await API.get(`/api/v1/admin/group/${encodedID}`);
        const { success, message, data } = res.data || {};
        if (!success || !data?.id) {
          showError(
            message || `${t('group_manage.messages.load_failed')}: ${normalizedGroupID}`,
          );
          navigate('/admin/group', { replace: true });
          return;
        }
        setMode(MODE_VIEW);
        setActiveGroup(data);
        resetFormState();
        resetDetailState();
        loadDetailModelConfigState(data.id || '').then();
      } catch (error) {
        showError(error?.message || error);
        navigate('/admin/group', { replace: true });
      } finally {
        setLoading(false);
      }
    },
    [loadDetailModelConfigState, navigate, t]
  );

  const resetToList = () => {
    setMode(MODE_LIST);
    setActiveGroup(null);
    resetFormState();
    resetDetailState();
  };

  const backToList = () => {
    if (submitting) return;
    if (isDetailRoute) {
      if (returnPath !== '') {
        navigate(-1);
        return;
      }
      navigate('/admin/group');
      return;
    }
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

  const openViewPanel = (row, options = {}) => {
    if (!row || submitting) return;
    const { syncRoute = true, replace = false } = options;
    if (syncRoute) {
      navigate(`/admin/group/detail/${encodeURIComponent(row.id || '')}`, {
        replace,
      });
      return;
    }
    setMode(MODE_VIEW);
    setActiveGroup(row);
    resetFormState();
    resetDetailState();
    loadDetailModelConfigState(row.id || '').then();
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

  const openChannelDetailFromCurrentPage = useCallback(
    (channelID) => {
      const normalizedChannelID = (channelID || '').toString().trim();
      if (normalizedChannelID === '') {
        return;
      }
      navigate(`/channel/detail/${normalizedChannelID}`, {
        state: {
          from: currentPagePath,
        },
      });
    },
    [currentPagePath, navigate]
  );

  const normalizeModelConfigsPayload = useCallback((items, selectedChannelIDs) => {
    const selectedChannelIDSet = new Set(
      Array.isArray(selectedChannelIDs) ? selectedChannelIDs : []
    );
    const normalizedModelConfigs = [];
    const seenModelConfigKeys = new Set();
    for (const item of Array.isArray(items) ? items : []) {
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
        return null;
      }
      const dedupeKey = `${model}::${channelID}`;
      if (seenModelConfigKeys.has(dedupeKey)) {
        showInfo(t('group_manage.messages.model_config_duplicate'));
        return null;
      }
      seenModelConfigKeys.add(dedupeKey);
      const normalizedItem = {
        model,
        channel_id: channelID,
        upstream_model: upstreamModel,
        enabled: item?.enabled !== false,
      };
      const rawPriority = item?.priority;
      if (rawPriority !== '' && rawPriority !== null && typeof rawPriority !== 'undefined') {
        normalizedItem.priority = toSafePriorityNumber(rawPriority, 0);
      }
      normalizedModelConfigs.push(normalizedItem);
    }
    return normalizedModelConfigs;
  }, [t]);

  const buildNormalizedModelConfigs = useCallback(() => {
    return normalizeModelConfigsPayload(formModelConfigs, formChannelIDs);
  }, [formChannelIDs, formModelConfigs, normalizeModelConfigsPayload]);

  const submitCreate = async () => {
    const name = (form.name || '').trim();
    if (name === '') {
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
        name,
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
    const name = (form.name || '').trim();
    if (id === '' || name === '') {
      showInfo(t('group_manage.messages.id_required'));
      return;
    }
    const billingRatio = Number(form.billing_ratio ?? 1);
    if (!Number.isFinite(billingRatio) || billingRatio < 0) {
      showInfo(t('group_manage.messages.billing_ratio_invalid'));
      return;
    }
    const normalizedModelConfigs = buildNormalizedModelConfigs();
    if (normalizedModelConfigs === null) {
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.put('/api/v1/admin/group/', {
        id,
        name,
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
      loadDetailModelConfigState(data.id || '').then();
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

  const startDetailBasicEdit = useCallback(() => {
    if (!activeGroup || submitting || detailBasicEditLocked) {
      return;
    }
    setForm(buildFormFromRow(activeGroup));
    setDetailEditingSection('basic');
  }, [activeGroup, detailBasicEditLocked, submitting]);

  const startDetailChannelsEdit = useCallback(async () => {
    if (!activeGroup || submitting || detailChannelsEditLocked) {
      return;
    }
    setForm(buildFormFromRow(activeGroup));
    const payload = await loadEditModelConfigs(activeGroup.id || '');
    if (!payload) {
      return;
    }
    setDetailEditingSection('channels');
  }, [activeGroup, detailChannelsEditLocked, loadEditModelConfigs, submitting]);

  const loadDetailModelEditorState = useCallback(async () => {
    if (!activeGroup || submitting || detailModelsEditLocked) {
      return null;
    }
    const payload = await loadEditModelConfigs(activeGroup.id || '');
    if (!payload) {
      return null;
    }
    return {
      channels: Array.isArray(payload?.channels) ? payload.channels : [],
      items: Array.isArray(payload?.items) ? payload.items : [],
      selectedChannelIDs: toBoundChannelIDs(payload?.channels),
    };
  }, [
    activeGroup,
    detailModelsEditLocked,
    loadEditModelConfigs,
    submitting,
  ]);

  const refreshGroupDetailState = useCallback(async (groupID) => {
    const normalizedGroupID = (groupID || '').toString().trim();
    if (normalizedGroupID === '') {
      return;
    }
    await loadCatalog();
    await loadGroupDetail(normalizedGroupID);
  }, [loadCatalog, loadGroupDetail]);

  const saveDetailModelConfigs = useCallback(async (items, selectedChannelIDs, channels = []) => {
    const id = (activeGroup?.id || '').toString().trim();
    if (id === '') {
      return false;
    }
    const normalizedModelConfigs = normalizeModelConfigsPayload(items, selectedChannelIDs);
    if (normalizedModelConfigs === null) {
      return false;
    }
    setSubmitting(true);
    try {
      const res = await API.put(
        `/api/v1/admin/group/${encodeURIComponent(id)}/model-configs`,
        {
          channel_ids: selectedChannelIDs,
          model_configs: normalizedModelConfigs,
        }
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.update_failed'));
        return false;
      }
      applySavedDetailModelState(items, channels, selectedChannelIDs);
      showSuccess(t('group_manage.messages.update_success'));
      return true;
    } catch (error) {
      showError(error);
      return false;
    } finally {
      setSubmitting(false);
    }
  }, [activeGroup, applySavedDetailModelState, normalizeModelConfigsPayload, t]);

  const closeDetailModelModal = useCallback(() => {
    if (submitting) {
      return;
    }
    setDetailModelModalOpen(false);
    setDetailModelEditTarget('');
    setDetailModelChannelDrafts([]);
  }, [submitting]);

  const cancelDetailSectionEdit = useCallback(() => {
    if (submitting) {
      return;
    }
    setDetailEditingSection('');
    closeDetailModelModal();
    resetFormState();
  }, [closeDetailModelModal, submitting]);

  const submitDetailBasic = useCallback(async () => {
    const id = (activeGroup?.id || '').toString().trim();
    const name = (form.name || '').trim();
    if (id === '' || name === '') {
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
      const res = await API.put('/api/v1/admin/group/', {
        id,
        name,
        description: (form.description || '').trim(),
        billing_ratio: billingRatio,
        sort_order: Number(form.sort_order || 0),
        enabled: !!activeGroup?.enabled,
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.update_failed'));
        return;
      }
      showSuccess(t('group_manage.messages.update_success'));
      await refreshGroupDetailState(id);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  }, [activeGroup, form.billing_ratio, form.description, form.name, form.sort_order, refreshGroupDetailState, t]);

  const submitDetailChannels = useCallback(async () => {
    const id = (activeGroup?.id || '').toString().trim();
    if (id === '') {
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.put(`/api/v1/admin/group/${encodeURIComponent(id)}/channels`, {
        channel_ids: formChannelIDs,
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.bind_update_failed'));
        return;
      }
      showSuccess(t('group_manage.messages.bind_update_success'));
      await refreshGroupDetailState(id);
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  }, [activeGroup, formChannelIDs, refreshGroupDetailState, t]);

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
      <Label basic color='green' className='router-tag'>
        {t('group_manage.status.enabled')}
      </Label>
    ) : (
      <Label basic color='grey' className='router-tag'>
        {t('group_manage.status.disabled')}
      </Label>
    );

  const renderList = () => (
    <>
      <div
        className='router-toolbar router-block-gap-sm'
      >
        <div className='router-toolbar-start'>
          <Button
            type='button'
            className='router-page-button'
            disabled={submitting}
            onClick={openCreatePanel}
          >
            {t('group_manage.buttons.add')}
          </Button>
          <Button
            type='button'
            className='router-page-button'
            disabled={submitting}
            loading={loading}
            onClick={loadCatalog}
          >
            {t('group_manage.buttons.refresh')}
          </Button>
        </div>
        <Form className='router-search-form-md'>
          <Form.Input
            className='router-section-input'
            icon='search'
            iconPosition='left'
            placeholder={t('group_manage.search')}
            value={searchKeyword}
            onChange={(e, { value }) => setSearchKeyword(value || '')}
          />
        </Form>
      </div>

      <Table basic='very' compact className='router-hover-table router-list-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>{t('group_manage.table.id')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.description')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.channels')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.billing_ratio')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.status')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.created_at')}</Table.HeaderCell>
            <Table.HeaderCell>{t('group_manage.table.updated_at')}</Table.HeaderCell>
            <Table.HeaderCell className='router-table-action-cell router-group-action-cell'>
              {t('group_manage.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {visibleRows.length === 0 ? (
            <Table.Row>
              <Table.Cell className='router-empty-cell' colSpan={8} textAlign='center'>
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
                className={submitting || loading ? undefined : 'router-row-clickable'}
              >
                <Table.Cell>{row.name || '-'}</Table.Cell>
                <Table.Cell>{row.description || '-'}</Table.Cell>
                <Table.Cell>
                  {Array.isArray(row.channels) && row.channels.length > 0 ? (
                    <div className='router-tag-group'>
                      {row.channels.map((item) => (
                        <Label key={item.id} className='router-tag'>
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
                <Table.Cell>{row.created_at ? timestamp2string(row.created_at) : '-'}</Table.Cell>
                <Table.Cell>{row.updated_at ? timestamp2string(row.updated_at) : '-'}</Table.Cell>
                <Table.Cell className='router-table-action-cell router-group-action-cell'>
                  <div className='router-action-group-tight'>
                    <Button
                      className='router-inline-button'
                      color={row.enabled ? 'orange' : 'green'}
                      size='mini'
                      compact
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
                      className='router-inline-button'
                      negative
                      size='mini'
                      compact
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

  const renderBoundChannelsField = (items, loadingState, options = {}) => (
    <Form.Field className={options.hideLabel ? '' : 'router-block-top-sm'}>
      {options.hideLabel ? null : (
        <label>{t('group_manage.detail.bound_channels')}</label>
      )}
      <div className='ui fluid multiple selection dropdown router-section-dropdown router-readonly-dropdown'>
        {loadingState ? (
          <div className='router-readonly-dropdown-empty'>
            {t('group_manage.messages.loading')}
          </div>
        ) : items.length === 0 ? (
          <div className='router-readonly-dropdown-empty'>
            {t('group_manage.detail.empty_channels')}
          </div>
        ) : (
          items.map((item) => (
            <Label
              as='a'
              key={item.id}
              className='router-tag'
              onClick={(event) => {
                event.preventDefault();
                openChannelDetailFromCurrentPage(item.id);
              }}
            >
              {formatChannelDisplayName(item)}
            </Label>
          ))
        )}
      </div>
    </Form.Field>
  );

  const renderDetailModelConfigTable = (options = {}) => {
    return (
      <div className={options.hideTitle ? '' : 'router-block-top-sm'}>
        <div
          className={
            options.hideTitle
              ? 'router-entity-detail-section-header'
              : 'router-toolbar router-block-gap-xs'
          }
        >
          {options.hideTitle ? (
            <Header as='h3' className='router-entity-detail-section-title'>
              {t('group_manage.detail.supported_models')}
            </Header>
          ) : (
            <div className='router-toolbar-title'>
              {t('group_manage.edit.model_configs')}
            </div>
          )}
          <div className='router-toolbar-start router-block-gap-sm'>
            <Form.Input
              className='router-inline-input router-search-form-sm'
              icon='search'
              iconPosition='left'
              placeholder={t('group_manage.edit.model_search_placeholder')}
              value={detailModelSearchKeyword}
              onChange={(e, { value }) => setDetailModelSearchKeyword(value || '')}
            />
          </div>
        </div>
        <Table compact celled className='router-detail-table'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>{t('group_manage.edit.model')}</Table.HeaderCell>
              <Table.HeaderCell>{t('group_manage.detail.model_channels')}</Table.HeaderCell>
              <Table.HeaderCell collapsing>{t('group_manage.detail.enabled')}</Table.HeaderCell>
              <Table.HeaderCell collapsing>{t('group_manage.table.actions')}</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {detailModelLoading ? (
              <Table.Row>
                <Table.Cell className='router-empty-cell' colSpan={4} textAlign='center'>
                  {t('group_manage.messages.loading')}
                </Table.Cell>
              </Table.Row>
            ) : detailModelEntries.length === 0 ? (
              <Table.Row>
                <Table.Cell className='router-empty-cell' colSpan={4} textAlign='center'>
                  {t('group_manage.edit.empty_model_configs')}
                </Table.Cell>
              </Table.Row>
            ) : (
              detailModelEntries.map((entry) => (
                <Table.Row key={`group-detail-model-${entry.model || '-'}`}>
                  <Table.Cell className='router-cell-min-240'>{entry.model || '-'}</Table.Cell>
                  <Table.Cell>
                    {entry.rows.length > 0 ? (
                      <div className='router-tag-group'>
                        {entry.rows.map((item) => (
                          <Label
                            as='a'
                            key={`${entry.model}-${item?.channel_id || '-'}-${item?.upstream_model || '-'}`}
                            className='router-tag'
                            basic
                            color={channelStatusColor(item?.channel_status)}
                            onClick={(event) => {
                              event.preventDefault();
                              openChannelDetailFromCurrentPage(item.channel_id);
                            }}
                          >
                            {item?.channel_name || item?.channel_id}
                            {` · ${formatPriorityLabel(item?.priority)}`}
                          </Label>
                        ))}
                      </div>
                    ) : (
                      '-'
                    )}
                  </Table.Cell>
                  <Table.Cell collapsing>
                    <Checkbox
                      toggle
                      checked={entry.allEnabled}
                      indeterminate={entry.partiallyEnabled}
                      disabled={submitting || detailModelsEditLocked}
                      onChange={(event, { checked }) => {
                        event.stopPropagation();
                        toggleDetailModelEnabled(entry.model, !!checked);
                      }}
                    />
                  </Table.Cell>
                  <Table.Cell collapsing>
                    <Button
                      type='button'
                      className='router-inline-button'
                      disabled={submitting || detailModelsEditLocked}
                      onClick={() => openDetailModelEdit(entry)}
                    >
                      {t('group_manage.buttons.edit')}
                    </Button>
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

  const detailModelSourceRows = useMemo(() => {
    return (Array.isArray(detailModelRows) ? detailModelRows : []).map((item) => ({
      ...item,
      enabled: item?.enabled !== false,
      priority: item?.priority ?? 0,
    }));
  }, [detailModelRows]);

  const detailModelEntries = useMemo(() => {
    const grouped = new Map();
    detailModelSourceRows.forEach((item) => {
      const model = (item?.model || '').trim();
      if (model === '') {
        return;
      }
      if (!grouped.has(model)) {
        grouped.set(model, {
          model,
          rows: [],
        });
      }
      grouped.get(model).rows.push(item);
    });

    const entries = Array.from(grouped.values())
      .map((entry) => {
        const rows = [...entry.rows].sort((left, right) => {
          const priorityDiff =
            toSafePriorityNumber(right?.priority, 0) - toSafePriorityNumber(left?.priority, 0);
          if (priorityDiff !== 0) {
            return priorityDiff;
          }
          const nameDiff = (left?.channel_name || '').localeCompare(right?.channel_name || '');
          if (nameDiff !== 0) {
            return nameDiff;
          }
          return (left?.channel_id || '').localeCompare(right?.channel_id || '');
        });
        const enabledCount = rows.filter((row) => row?.enabled !== false).length;
        return {
          model: entry.model,
          rows,
          enabledCount,
          allEnabled: rows.length > 0 && enabledCount === rows.length,
          partiallyEnabled: enabledCount > 0 && enabledCount < rows.length,
        };
      })
      .sort((left, right) => left.model.localeCompare(right.model));

    const keyword = detailModelSearchKeyword.trim().toLowerCase();
    if (keyword === '') {
      return entries;
    }
    return entries.filter((entry) => {
      const haystacks = [
        entry.model,
        ...entry.rows.flatMap((row) => [
          row?.channel_name || '',
          row?.channel_id || '',
          row?.channel_protocol || '',
          row?.upstream_model || '',
        ]),
      ];
      return haystacks.some((value) => value.toLowerCase().includes(keyword));
    });
  }, [detailModelSearchKeyword, detailModelSourceRows]);

  const openDetailModelEdit = useCallback(async (entry) => {
    const editorState = await loadDetailModelEditorState();
    if (!editorState) {
      return;
    }
    const targetModel = (entry?.model || '').toString().trim();
    if (targetModel === '') {
      return;
    }
    const selectedChannelIDSet = new Set(editorState.selectedChannelIDs);
    const sourceItems = Array.isArray(editorState?.items) ? editorState.items : [];
    const existingRows = sourceItems.filter(
      (item) => (item?.model || '').toString().trim() === targetModel
    );
    const existingByChannelID = new Map();
    existingRows.forEach((item) => {
      const channelID = (item?.channel_id || '').toString().trim();
      if (channelID === '' || existingByChannelID.has(channelID)) {
        return;
      }
      existingByChannelID.set(channelID, item);
    });

    const drafts = [];
    (Array.isArray(editorState?.channels) ? editorState.channels : []).forEach((channel) => {
      const channelID = (channel?.id || '').toString().trim();
      if (channelID === '' || !selectedChannelIDSet.has(channelID)) {
        return;
      }
      const matchedModel = (Array.isArray(channel?.models) ? channel.models : []).find(
        (item) => (item?.model || '').toString().trim() === targetModel
      );
      const existing = existingByChannelID.get(channelID);
      if (!matchedModel && !existing) {
        return;
      }
      drafts.push({
        channel_id: channelID,
        channel_name: channel?.name || channelID,
        channel_protocol: channel?.protocol || '',
        channel_status: Number(channel?.status || 0),
        selected: !!existing,
        enabled: existing?.enabled !== false,
        upstream_model:
          (existing?.upstream_model || '').toString().trim() ||
          (matchedModel?.upstream_model || matchedModel?.model || '').toString().trim(),
        priority: String(
          toSafePriorityNumber(existing?.priority ?? channel?.priority, 0)
        ),
      });
    });

    drafts.sort((left, right) => {
      if (left.selected !== right.selected) {
        return left.selected ? -1 : 1;
      }
      const priorityDiff =
        toSafePriorityNumber(right.priority, 0) - toSafePriorityNumber(left.priority, 0);
      if (priorityDiff !== 0) {
        return priorityDiff;
      }
      return (left.channel_name || '').localeCompare(right.channel_name || '');
    });

    if (drafts.length === 0) {
      showInfo(t('group_manage.messages.model_channel_empty'));
      return;
    }
    setDetailModelEditTarget(targetModel);
    setDetailModelChannelDrafts(drafts);
    setDetailModelModalOpen(true);
  }, [loadDetailModelEditorState, t]);

  const updateDetailModelChannelDraft = useCallback((channelID, updater) => {
    setDetailModelChannelDrafts((prev) =>
      (Array.isArray(prev) ? prev : []).map((item) => {
        if ((item?.channel_id || '') !== channelID) {
          return item;
        }
        return typeof updater === 'function' ? updater(item) : item;
      })
    );
  }, []);

  const toggleDetailModelEnabled = useCallback(async (modelName, nextEnabled) => {
    const editorState = await loadDetailModelEditorState();
    if (!editorState) {
      return;
    }
    const normalizedModel = (modelName || '').toString().trim();
    if (normalizedModel === '') {
      return;
    }
    const nextItems = sortGroupModelConfigRows(
      (Array.isArray(editorState.items) ? editorState.items : []).map((item) =>
        (item?.model || '').toString().trim() === normalizedModel
          ? { ...item, enabled: !!nextEnabled }
          : item
      )
    );
    await saveDetailModelConfigs(
      nextItems,
      editorState.selectedChannelIDs,
      editorState.channels,
    );
  }, [loadDetailModelEditorState, saveDetailModelConfigs]);

  const submitDetailModelDraft = useCallback(async () => {
    const model = detailModelEditTarget.trim();
    if (model === '') {
      showInfo(t('group_manage.messages.model_config_incomplete'));
      return;
    }
    const selectedDrafts = (Array.isArray(detailModelChannelDrafts) ? detailModelChannelDrafts : []).filter(
      (item) => !!item?.selected
    );
    if (selectedDrafts.length === 0) {
      showInfo(t('group_manage.messages.model_channel_required'));
      return;
    }
    const selectedChannelIDs =
      Array.isArray(formChannelIDs) && formChannelIDs.length > 0
        ? formChannelIDs
        : toBoundChannelIDs(formModelChannels);
    const baseRows = (Array.isArray(formModelConfigs) ? formModelConfigs : []).filter(
      (item) => (item?.model || '').toString().trim() !== model
    );
    const nextRows = selectedDrafts.map((item) => ({
      model,
      channel_id: item.channel_id,
      upstream_model: item.upstream_model,
      enabled: item.enabled !== false,
      priority: toSafePriorityNumber(item.priority, 0),
      channel_name: item.channel_name,
      channel_protocol: item.channel_protocol,
      channel_status: item.channel_status,
    }));
    const saved = await saveDetailModelConfigs(
      sortGroupModelConfigRows([...baseRows, ...nextRows]),
      selectedChannelIDs,
      formModelChannels,
    );
    if (saved) {
      closeDetailModelModal();
    }
  }, [
    closeDetailModelModal,
    detailModelChannelDrafts,
    detailModelEditTarget,
    formChannelIDs,
    formModelChannels,
    formModelConfigs,
    saveDetailModelConfigs,
    t,
  ]);

  const renderEditModelConfigTable = () => (
    <div className='router-block-top-md'>
      <div className='router-toolbar router-block-gap-xs'>
        <div className='router-toolbar-start'>
          <div className='router-toolbar-title'>{t('group_manage.edit.model_configs')}</div>
          <Button
            type='button'
            className='router-inline-button'
            disabled={submitting || formModelLoading}
            onClick={addEmptyModelConfigRow}
          >
            {t('group_manage.buttons.add_model')}
          </Button>
        </div>
        <Form.Input
          className='router-inline-input router-search-form-sm'
          icon='search'
          iconPosition='left'
          placeholder={t('group_manage.edit.model_search_placeholder')}
          value={editModelSearchKeyword}
          onChange={(e, { value }) => setEditModelSearchKeyword(value || '')}
        />
      </div>
      <Table compact celled className='router-detail-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell className='router-cell-min-260'>
              {t('group_manage.edit.model')}
            </Table.HeaderCell>
            <Table.HeaderCell className='router-cell-min-240'>
              {t('group_manage.edit.channel')}
            </Table.HeaderCell>
            <Table.HeaderCell className='router-cell-min-280'>
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
              <Table.Cell className='router-empty-cell' colSpan={4} textAlign='center'>
                {t('group_manage.messages.loading')}
              </Table.Cell>
            </Table.Row>
          ) : visibleEditModelConfigs.length === 0 ? (
            <Table.Row>
              <Table.Cell className='router-empty-cell' colSpan={4} textAlign='center'>
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
                      className='router-inline-input'
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
                      className='router-inline-dropdown'
                      fluid
                      selection
                      search
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
                      className='router-inline-dropdown'
                      fluid
                      selection
                      search
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
                      className='router-inline-button'
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
      <div className='router-entity-detail-page'>
        <div className='router-entity-detail-breadcrumb'>
          <Breadcrumb size='small'>
            <Breadcrumb.Section link onClick={backToList}>
              {t('header.group')}
            </Breadcrumb.Section>
            <Breadcrumb.Divider icon='right chevron' />
            <Breadcrumb.Section active>
              {activeGroup.name || activeGroup.id || '-'}
            </Breadcrumb.Section>
          </Breadcrumb>
        </div>
        <section className='router-entity-detail-section'>
          <div className='router-entity-detail-section-header'>
            <Header as='h3' className='router-entity-detail-section-title'>
              {t('common.basic_info')}
            </Header>
            <div className='router-toolbar-start'>
              {renderGroupStatus(activeGroup.enabled)}
              {detailBasicEditing ? (
                <>
                  <Button
                    type='button'
                    className='router-page-button'
                    disabled={submitting}
                    onClick={cancelDetailSectionEdit}
                  >
                    {t('group_manage.buttons.cancel')}
                  </Button>
                  <Button
                    type='button'
                    className='router-page-button'
                    color='blue'
                    loading={submitting}
                    disabled={submitting}
                    onClick={submitDetailBasic}
                  >
                    {t('group_manage.buttons.confirm')}
                  </Button>
                </>
              ) : (
                <Button
                  type='button'
                  className='router-page-button'
                  color='blue'
                  disabled={submitting || detailBasicEditLocked}
                  onClick={startDetailBasicEdit}
                >
                  {t('group_manage.buttons.edit')}
                </Button>
              )}
            </div>
          </div>
          <Form>
            <Form.Input
              className='router-section-input'
              label={t('group_manage.form.id')}
              value={detailBasicEditing ? form.name : activeGroup.name || ''}
              readOnly={!detailBasicEditing}
              placeholder={t('group_manage.form.id_placeholder')}
              onChange={(e, { value }) =>
                setForm((prev) => ({ ...prev, name: value || '' }))
              }
            />
            <Form.TextArea
              className='router-section-textarea'
              label={t('group_manage.form.description')}
              value={detailBasicEditing ? form.description : activeGroup.description || ''}
              readOnly={!detailBasicEditing}
              placeholder={t('group_manage.form.description_placeholder')}
              onChange={(e) =>
                setForm((prev) => ({
                  ...prev,
                  description: e.target.value,
                }))
              }
            />
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                type='number'
                min='0'
                step='0.01'
                label={t('group_manage.form.billing_ratio')}
                value={
                  detailBasicEditing
                    ? form.billing_ratio
                    : Number(activeGroup.billing_ratio ?? 1).toFixed(2)
                }
                readOnly={!detailBasicEditing}
                placeholder={t('group_manage.form.billing_ratio_placeholder')}
                onChange={(e) =>
                  setForm((prev) => ({
                    ...prev,
                    billing_ratio: e.target.value,
                }))
              }
            />
            </Form.Group>
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                type='number'
                label={t('group_manage.form.sort_order')}
                value={detailBasicEditing ? form.sort_order : activeGroup.sort_order || 0}
                readOnly={!detailBasicEditing}
                onChange={(e) =>
                  setForm((prev) => ({
                    ...prev,
                    sort_order: Number(e.target.value || 0),
                  }))
                }
              />
              <Form.Input
                className='router-section-input'
                label={t('group_manage.table.created_at')}
                value={activeGroup.created_at ? timestamp2string(activeGroup.created_at) : '-'}
                readOnly
              />
              <Form.Input
                className='router-section-input'
                label={t('group_manage.table.updated_at')}
                value={activeGroup.updated_at ? timestamp2string(activeGroup.updated_at) : '-'}
                readOnly
              />
            </Form.Group>
          </Form>
        </section>
        <section className='router-entity-detail-section'>
          <div className='router-entity-detail-section-header'>
            <Header as='h3' className='router-entity-detail-section-title'>
              {t('group_manage.detail.bound_channels')}
            </Header>
            <div className='router-toolbar-start'>
              {detailChannelsEditing ? (
                <>
                  <Button
                    type='button'
                    className='router-page-button'
                    disabled={submitting}
                    onClick={cancelDetailSectionEdit}
                  >
                    {t('group_manage.buttons.cancel')}
                  </Button>
                  <Button
                    type='button'
                    className='router-page-button'
                    color='blue'
                    loading={submitting}
                    disabled={submitting}
                    onClick={submitDetailChannels}
                  >
                    {t('group_manage.buttons.confirm')}
                  </Button>
                </>
              ) : (
                <Button
                  type='button'
                  className='router-page-button'
                  color='blue'
                  disabled={submitting || detailChannelsEditLocked}
                  onClick={startDetailChannelsEdit}
                >
                  {t('group_manage.buttons.edit')}
                </Button>
              )}
            </div>
          </div>
          {detailChannelsEditing ? (
            <Form>
              <Form.Dropdown
                className='router-section-dropdown'
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
          ) : (
            renderBoundChannelsField(detailChannelRows, detailChannelLoading, {
              hideLabel: true,
            })
          )}
        </section>
        <section className='router-entity-detail-section'>
          {renderDetailModelConfigTable({
            hideTitle: true,
          })}
        </section>
      </div>
    );
  };

  const renderEdit = () => (
    <div>
      <div className='router-toolbar-start router-block-gap-sm'>
        <Button type='button' className='router-page-button' onClick={() => setMode(MODE_VIEW)} disabled={submitting}>
          {t('group_manage.buttons.cancel')}
        </Button>
        <Button type='button' className='router-page-button' color='blue' loading={submitting} disabled={submitting} onClick={submitEdit}>
          {t('group_manage.buttons.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Input
          className='router-section-input'
          label={t('group_manage.form.id')}
          placeholder={t('group_manage.form.id_placeholder')}
          value={form.name}
          onChange={(e, { value }) =>
            setForm((prev) => ({ ...prev, name: value || '' }))
          }
        />
        <Form.TextArea
          className='router-section-textarea'
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
            className='router-section-input'
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
            className='router-section-input'
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
          className='router-section-dropdown'
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

  useEffect(() => {
    if (!isDetailRoute) {
      if (mode === MODE_VIEW && activeGroup) {
        resetToList();
      }
      return;
    }
    const activeGroupID = (activeGroup?.id || '').toString().trim();
    if (
      activeGroupID === normalizedDetailGroupId &&
      (mode === MODE_VIEW || mode === MODE_EDIT)
    ) {
      return;
    }
    loadGroupDetail(normalizedDetailGroupId).then();
  }, [
    activeGroup,
    isDetailRoute,
    loadGroupDetail,
    mode,
    normalizedDetailGroupId,
  ]);

  const renderCreate = () => (
    <div>
      <div className='router-toolbar-start router-block-gap-sm'>
        <Button type='button' className='router-page-button' onClick={backToList} disabled={submitting}>
          {t('group_manage.buttons.cancel')}
        </Button>
        <Button type='button' className='router-page-button' color='blue' loading={submitting} disabled={submitting} onClick={submitCreate}>
          {t('group_manage.buttons.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Input
          className='router-section-input'
          required
          label={t('group_manage.form.id')}
          placeholder={t('group_manage.form.id_placeholder')}
          value={form.name}
          onChange={(e) =>
            setForm((prev) => ({ ...prev, name: e.target.value }))
          }
        />
        <Form.TextArea
          className='router-section-textarea'
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
            className='router-section-input'
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
            className='router-section-dropdown'
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

      <Modal open={detailModelModalOpen} onClose={closeDetailModelModal} size='large'>
        <Modal.Header>
          {t('group_manage.detail.edit_model_title', {
            model: detailModelEditTarget || '-',
          })}
        </Modal.Header>
        <Modal.Content>
          <Table compact celled className='router-detail-table'>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell collapsing>{t('group_manage.detail.selected')}</Table.HeaderCell>
                <Table.HeaderCell>{t('group_manage.edit.channel')}</Table.HeaderCell>
                <Table.HeaderCell>{t('group_manage.edit.upstream_model')}</Table.HeaderCell>
                <Table.HeaderCell collapsing>{t('group_manage.detail.priority')}</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {detailModelChannelDrafts.length === 0 ? (
                <Table.Row>
                  <Table.Cell className='router-empty-cell' colSpan={4} textAlign='center'>
                    {t('group_manage.detail.empty_model_channels')}
                  </Table.Cell>
                </Table.Row>
              ) : (
                detailModelChannelDrafts.map((item) => (
                  <Table.Row key={`${detailModelEditTarget}-${item.channel_id}`}>
                    <Table.Cell collapsing>
                      <Checkbox
                        checked={!!item.selected}
                        onChange={(event, { checked }) =>
                          updateDetailModelChannelDraft(item.channel_id, (current) => ({
                            ...current,
                            selected: !!checked,
                          }))
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Label
                        basic
                        color={channelStatusColor(item?.channel_status)}
                        className='router-tag'
                      >
                        {item.channel_name || item.channel_id}
                        {item.channel_protocol ? ` · ${item.channel_protocol}` : ''}
                      </Label>
                    </Table.Cell>
                    <Table.Cell>{item.upstream_model || '-'}</Table.Cell>
                    <Table.Cell collapsing>
                      <Form.Input
                        className='router-inline-input'
                        type='number'
                        step='1'
                        disabled={!item.selected}
                        value={item.priority}
                        onChange={(e, { value }) =>
                          updateDetailModelChannelDraft(item.channel_id, (current) => ({
                            ...current,
                            priority: value ?? '',
                          }))
                        }
                      />
                    </Table.Cell>
                  </Table.Row>
                ))
              )}
            </Table.Body>
          </Table>
        </Modal.Content>
        <Modal.Actions>
          <Button className='router-modal-button' onClick={closeDetailModelModal} disabled={submitting}>
            {t('group_manage.buttons.cancel')}
          </Button>
          <Button className='router-modal-button' color='blue' onClick={submitDetailModelDraft} disabled={submitting}>
            {t('group_manage.buttons.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      <Modal open={deleteOpen} onClose={closeDeleteModal} size='tiny'>
        <Modal.Header>{t('group_manage.modal.delete_title')}</Modal.Header>
        <Modal.Content>
          {t('group_manage.modal.delete_confirm', {
            id: deleteTarget?.id || '',
          })}
        </Modal.Content>
        <Modal.Actions>
          <Button className='router-modal-button' onClick={closeDeleteModal} disabled={submitting}>
            {t('group_manage.buttons.cancel')}
          </Button>
          <Button className='router-modal-button' negative onClick={submitDelete} loading={submitting}>
            {t('group_manage.buttons.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    </>
  );
};

export default GroupsManager;
