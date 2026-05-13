import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../helpers';
import {
  AppButton,
  AppDetailSection,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppIcon,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTag,
  AppTabs,
  AppToolbar,
  AppTextarea,
} from '../router-ui';
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

const collectChannelIDsFromGroupModelConfigs = (items) => {
  const ids = [];
  const seen = new Set();
  (Array.isArray(items) ? items : []).forEach((item) => {
    const channelID = (item?.channel_id || '').trim();
    if (channelID === '' || seen.has(channelID)) {
      return;
    }
    seen.add(channelID);
    ids.push(channelID);
  });
  return ids;
};

const toChannelRows = (items) =>
  Array.isArray(items) ? items : [];

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

const summarizeGroupChannels = (items, maxVisible = 2) => {
  const channels = Array.isArray(items) ? items : [];
  const visible = channels.slice(0, Math.max(0, maxVisible));
  const hiddenCount = Math.max(0, channels.length - visible.length);
  return { visible, hiddenCount };
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
  const [activeDetailTab, setActiveDetailTab] = useState('overview');
  const [detailChannelModalOpen, setDetailChannelModalOpen] = useState(false);
  const [detailChannelDraft, setDetailChannelDraft] = useState({ id: '', priority: 0 });
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
  const detailAvailableChannelOptions = useMemo(
    () =>
      (Array.isArray(detailChannelRows) ? detailChannelRows : [])
        .filter((item) => !item?.bound)
        .map((item) => ({
          key: item.id,
          text: formatChannelDisplayName(item),
          value: item.id,
        })),
    [detailChannelRows]
  );
  const detailBasicEditing = detailEditingSection === 'basic';
  const detailSectionLocked = detailEditingSection !== '';
  const detailBasicEditLocked = detailSectionLocked && !detailBasicEditing;
  const detailChannelsEditLocked = detailSectionLocked || detailModelModalOpen;
  const detailModelsEditLocked = detailSectionLocked;
  const isAnyDetailSectionEditing = detailSectionLocked || detailChannelModalOpen || detailModelModalOpen;

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

  const fetchGroupChannelBindings = useCallback(async (groupID) => {
    const encodedID = encodeURIComponent(groupID || '');
    const res = await API.get(`/api/v1/admin/group/${encodedID}/channels`);
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || t('group_manage.messages.bind_load_failed'));
    }
    return Array.isArray(data) ? data : [];
  }, [t]);

  const applyDetailModelConfigPayload = useCallback((payload) => {
    const items = sortGroupModelConfigRows(Array.isArray(payload?.items) ? payload.items : []);
    setDetailModelRows(items);
  }, []);

  const loadDetailModelConfigState = useCallback(async (groupID) => {
    setDetailChannelLoading(true);
    setDetailModelLoading(true);
    try {
      const [payload, channelRows] = await Promise.all([
        fetchGroupModelConfigPayload(groupID),
        fetchGroupChannelBindings(groupID),
      ]);
      setDetailChannelRows(toChannelRows(channelRows));
      applyDetailModelConfigPayload(payload);
      return payload;
    } catch (error) {
      showError(error);
      return null;
    } finally {
      setDetailChannelLoading(false);
      setDetailModelLoading(false);
    }
  }, [applyDetailModelConfigPayload, fetchGroupChannelBindings, fetchGroupModelConfigPayload]);

  const refreshDetailChannelBindings = useCallback(async (groupID) => {
    const normalizedGroupID = (groupID || '').toString().trim();
    if (normalizedGroupID === '') {
      setDetailChannelRows([]);
      return [];
    }
    const rows = await fetchGroupChannelBindings(normalizedGroupID);
    const nextRows = toChannelRows(rows);
    setDetailChannelRows(nextRows);
    return nextRows;
  }, [fetchGroupChannelBindings]);

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
    setDetailChannelLoading(true);
    let channels = [];
    try {
      channels = await fetchGroupChannelBindings(activeGroup.id || '');
    } catch (error) {
      showError(error);
      setDetailChannelLoading(false);
      return;
    }
    setDetailChannelLoading(false);
    setDetailChannelRows(toChannelRows(channels));
    const firstUnbound = channels.find((item) => !item?.bound);
    setDetailChannelDraft({
      id: (firstUnbound?.id || '').toString().trim(),
      priority: toSafePriorityNumber(firstUnbound?.priority, 0),
    });
    setDetailChannelModalOpen(true);
  }, [activeGroup, detailChannelsEditLocked, fetchGroupChannelBindings, submitting]);

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
      await refreshDetailChannelBindings(id);
      showSuccess(t('group_manage.messages.update_success'));
      return true;
    } catch (error) {
      showError(error);
      return false;
    } finally {
      setSubmitting(false);
    }
  }, [activeGroup, applySavedDetailModelState, normalizeModelConfigsPayload, refreshDetailChannelBindings, t]);

  const saveSingleDetailModelConfigs = useCallback(async (modelName, items, channels = []) => {
    const id = (activeGroup?.id || '').toString().trim();
    const normalizedModel = (modelName || '').toString().trim();
    if (id === '' || normalizedModel === '') {
      return false;
    }
    const normalizedModelConfigs = normalizeModelConfigsPayload(items, collectChannelIDsFromGroupModelConfigs(items));
    if (normalizedModelConfigs === null) {
      return false;
    }
    setSubmitting(true);
    try {
      const res = await API.put(
        `/api/v1/admin/group/${encodeURIComponent(id)}/model-configs/${encodeURIComponent(normalizedModel)}`,
        {
          model_configs: normalizedModelConfigs,
        }
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.update_failed'));
        return false;
      }
      const preservedRows = (Array.isArray(detailModelRows) ? detailModelRows : []).filter(
        (item) => (item?.model || '').toString().trim() !== normalizedModel
      );
      applySavedDetailModelState(
        sortGroupModelConfigRows([...preservedRows, ...items]),
        channels,
        toBoundChannelIDs(channels),
      );
      await refreshDetailChannelBindings(id);
      showSuccess(t('group_manage.messages.update_success'));
      return true;
    } catch (error) {
      showError(error);
      return false;
    } finally {
      setSubmitting(false);
    }
  }, [activeGroup, applySavedDetailModelState, detailModelRows, normalizeModelConfigsPayload, refreshDetailChannelBindings, t]);

  const closeDetailModelModal = useCallback(() => {
    if (submitting) {
      return;
    }
    setDetailModelModalOpen(false);
    setDetailModelEditTarget('');
    setDetailModelChannelDrafts([]);
  }, [submitting]);

  const closeDetailChannelModal = useCallback(() => {
    if (submitting) {
      return;
    }
    setDetailChannelModalOpen(false);
    setDetailChannelDraft({ id: '', priority: 0 });
  }, [submitting]);

  const cancelDetailSectionEdit = useCallback(() => {
    if (submitting) {
      return;
    }
    setDetailEditingSection('');
    setDetailChannelModalOpen(false);
    setDetailChannelDraft({ id: '', priority: 0 });
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

  const submitDetailChannels = useCallback(async (channelRows = detailChannelRows) => {
    const id = (activeGroup?.id || '').toString().trim();
    if (id === '') {
      return false;
    }
    setSubmitting(true);
    try {
      const normalizedChannels = (Array.isArray(channelRows) ? channelRows : [])
        .map((item) => ({
          id: (item?.id || '').toString().trim(),
          bound: !!item?.bound,
          priority: toSafePriorityNumber(item?.priority, 0),
        }))
        .filter((item) => item.id !== '');
      const res = await API.put(`/api/v1/admin/group/${encodeURIComponent(id)}/channels`, {
        channel_ids: normalizedChannels
          .filter((item) => item.bound)
          .map((item) => item.id),
        channels: normalizedChannels,
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('group_manage.messages.bind_update_failed'));
        return false;
      }
      showSuccess(t('group_manage.messages.bind_update_success'));
      await refreshGroupDetailState(id);
      return true;
    } catch (error) {
      showError(error);
      return false;
    } finally {
      setSubmitting(false);
    }
  }, [activeGroup, detailChannelRows, refreshGroupDetailState, t]);

  const submitDetailChannelDraft = useCallback(async () => {
    const channelID = (detailChannelDraft.id || '').toString().trim();
    if (channelID === '') {
      showInfo(t('group_manage.messages.channel_required'));
      return;
    }
    const nextRows = (Array.isArray(detailChannelRows) ? detailChannelRows : []).map((item) => {
      if ((item?.id || '').toString().trim() !== channelID) {
        return item;
      }
      return {
        ...item,
        bound: true,
        priority: toSafePriorityNumber(detailChannelDraft.priority, 0),
      };
    });
    setDetailChannelRows(nextRows);
    const ok = await submitDetailChannels(nextRows);
    if (!ok) {
      return;
    }
    closeDetailChannelModal();
  }, [closeDetailChannelModal, detailChannelDraft.id, detailChannelDraft.priority, detailChannelRows, submitDetailChannels, t]);

  const removeDetailChannel = useCallback(async (row) => {
    if (!activeGroup || submitting) {
      return;
    }
    const channelID = (row?.id || '').toString().trim();
    const nextRows = (Array.isArray(detailChannelRows) ? detailChannelRows : []).map((item) => {
      if ((item?.id || '').toString().trim() !== channelID) {
        return item;
      }
      return {
        ...item,
        bound: false,
      };
    });
    setDetailChannelRows(nextRows);
    await submitDetailChannels(nextRows);
  }, [activeGroup, detailChannelRows, submitDetailChannels, submitting]);

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
      <AppTag color='green' className='router-tag'>
        {t('group_manage.status.enabled')}
      </AppTag>
    ) : (
      <AppTag color='grey' className='router-tag'>
        {t('group_manage.status.disabled')}
      </AppTag>
    );

  const renderList = () => (
    <>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'resource', label: t('header.resource') },
          { key: 'group', label: t('header.group'), active: true },
        ]}
        title={t('header.group')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              disabled={submitting}
              onClick={openCreatePanel}
            >
              {t('group_manage.buttons.add')}
            </AppButton>
            <AppButton
              type='button'
              className='router-page-button'
              disabled={submitting}
              loading={loading}
              onClick={loadCatalog}
            >
              {t('group_manage.buttons.refresh')}
            </AppButton>
          </div>
        }
        query={
          <AppInput
            className='router-section-input router-search-form-sm'
            placeholder={t('group_manage.search')}
            value={searchKeyword}
            onChange={(e, { value }) => setSearchKeyword(value || '')}
          />
        }
      />

      <AppTable
        className='router-hover-table router-list-table'
        rowKey='id'
        pagination={false}
        loading={loading}
        locale={{ emptyText: t('group_manage.messages.empty') }}
        dataSource={visibleRows}
        onRow={(row) => ({
          onClick: () => openViewPanel(row),
          className: submitting || loading ? undefined : 'router-row-clickable',
        })}
        columns={[
          {
            title: t('group_manage.table.id'),
            dataIndex: 'name',
            key: 'name',
            render: (value) => value || '-',
          },
          {
            title: t('group_manage.table.description'),
            dataIndex: 'description',
            key: 'description',
            render: (value) => value || '-',
          },
          {
            title: t('group_manage.table.channels'),
            dataIndex: 'channels',
            key: 'channels',
            render: (channels) => {
              const { visible, hiddenCount } = summarizeGroupChannels(channels, 2);
              if (visible.length === 0) {
                return '-';
              }
              return (
                <div className='router-tag-group'>
                  {visible.map((item) => (
                    <AppTag key={item.id} className='router-tag'>
                      {formatChannelDisplayName(item)}
                    </AppTag>
                  ))}
                  {hiddenCount > 0 ? (
                    <AppTag className='router-tag'>... +{hiddenCount}</AppTag>
                  ) : null}
                </div>
              );
            },
          },
          {
            title: t('group_manage.table.billing_ratio'),
            dataIndex: 'billing_ratio',
            key: 'billing_ratio',
            render: (value) => Number(value ?? 1).toFixed(2),
          },
          {
            title: t('group_manage.table.status'),
            dataIndex: 'enabled',
            key: 'enabled',
            render: (value) => renderGroupStatus(value),
          },
          {
            title: t('group_manage.table.created_at'),
            dataIndex: 'created_at',
            key: 'created_at',
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: t('group_manage.table.updated_at'),
            dataIndex: 'updated_at',
            key: 'updated_at',
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: t('group_manage.table.actions'),
            key: 'actions',
            className: 'router-table-action-cell router-group-action-cell',
            render: (_, row) => (
              <div className='router-action-group-tight'>
                <AppButton
                  className='router-inline-button'
                  color={row.enabled ? undefined : 'blue'}
                  disabled={submitting || loading}
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleEnabled(row);
                  }}
                >
                  {row.enabled
                    ? t('group_manage.buttons.disable')
                    : t('group_manage.buttons.enable')}
                </AppButton>
                <AppButton
                  className='router-inline-button'
                  disabled={submitting || loading}
                  onClick={(e) => {
                    e.stopPropagation();
                    openViewPanel(row);
                    startEditPanel(row);
                  }}
                >
                  {t('common.edit')}
                </AppButton>
                <AppButton
                  className='router-inline-button'
                  color='red'
                  disabled={submitting || loading}
                  onClick={(e) => {
                    e.stopPropagation();
                    openDeleteModal(row);
                  }}
                >
                  {t('group_manage.buttons.delete')}
                </AppButton>
              </div>
            ),
          },
        ]}
      />
    </>
  );

  const renderBoundChannelsField = (items, loadingState, options = {}) => (
    <div className={options.hideLabel ? '' : 'router-block-top-sm'}>
      {options.hideLabel ? null : (
        <label>{t('group_manage.detail.bound_channels')}</label>
      )}
      <div className='router-readonly-dropdown'>
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
            <AppTag
              key={item.id}
              className='router-tag'
              onClick={(event) => {
                event.preventDefault();
                openChannelDetailFromCurrentPage(item.id);
              }}
            >
              {formatChannelDisplayName(item)}
            </AppTag>
          ))
        )}
      </div>
    </div>
  );

  const updateDetailChannelDraft = useCallback((channelID, updater) => {
    const normalizedChannelID = (channelID || '').toString().trim();
    if (normalizedChannelID === '') {
      return;
    }
    setDetailChannelRows((prev) => {
      const currentRows = Array.isArray(prev) ? prev : [];
      const nextRows = currentRows.map((item) => {
        if ((item?.id || '').toString().trim() !== normalizedChannelID) {
          return item;
        }
        const nextValue =
          typeof updater === 'function' ? updater(item) : { ...item, ...updater };
        return {
          ...item,
          ...nextValue,
          id: normalizedChannelID,
        };
      });
      return nextRows;
    });
  }, []);

  const renderDetailChannelsTable = (items, loadingState) => {
    const rows = (Array.isArray(items) ? items : []).filter((item) => !!item?.bound);
    return (
      <AppTable
        className='router-detail-table'
        rowKey='id'
        pagination={false}
        loading={loadingState}
        locale={{ emptyText: t('group_manage.detail.empty_channels') }}
        dataSource={rows}
        columns={[
          {
            title: t('group_manage.detail.model_channels'),
            dataIndex: 'name',
            key: 'name',
            width: '44%',
            render: (_, item) => (
              <span className='router-cell-truncate' title={formatChannelDisplayName(item)}>
                {formatChannelDisplayName(item)}
              </span>
            ),
          },
          {
            title: t('group_manage.table.status'),
            dataIndex: 'status',
            key: 'status',
            width: '16%',
            render: (value) => (
              <AppTag color={channelStatusColor(value)} className='router-tag'>
                {value === 1
                  ? t('group_manage.status.enabled')
                  : t('group_manage.status.disabled')}
              </AppTag>
            ),
          },
          {
            title: t('group_manage.detail.priority'),
            dataIndex: 'priority',
            key: 'priority',
            width: '16%',
            render: (_, item) => {
              const channelID = (item?.id || '').toString().trim();
              return (
                <AppInputNumber
                  className='router-inline-input'
                  step={1}
                  precision={0}
                  disabled={submitting || detailChannelsEditLocked || detailChannelModalOpen}
                  value={toSafePriorityNumber(item?.priority, 0)}
                  onChange={(e, { value }) =>
                    updateDetailChannelDraft(channelID, (current) => ({
                      ...current,
                      priority: Number.isFinite(Number(value))
                        ? Math.trunc(Number(value))
                        : 0,
                    }))
                  }
                  onBlur={async () => {
                    const nextRows = (Array.isArray(detailChannelRows) ? detailChannelRows : []).map((row) =>
                      (row?.id || '').toString().trim() === channelID
                        ? {
                            ...row,
                            priority: toSafePriorityNumber(row?.priority, 0),
                          }
                        : row
                    );
                    setDetailChannelRows(nextRows);
                    await submitDetailChannels(nextRows);
                  }}
                />
              );
            },
          },
          {
            title: t('group_manage.table.actions'),
            key: 'actions',
            width: '24%',
            render: (_, item) => {
              const channelID = (item?.id || '').toString().trim();
              return (
                <div className='router-inline-actions'>
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    basic
                    onClick={() => openChannelDetailFromCurrentPage(channelID)}
                  >
                    {t('group_manage.buttons.view_channel')}
                  </AppButton>
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    basic
                    disabled={submitting || detailChannelsEditLocked || detailChannelModalOpen}
                    onClick={() => removeDetailChannel(item)}
                  >
                    {t('group_manage.buttons.remove_channel')}
                  </AppButton>
                </div>
              );
            },
          },
        ]}
      />
    );
  };

  const renderDetailModelConfigTable = (options = {}) => {
    return (
      <div className={options.hideTitle ? '' : 'router-block-top-sm'}>
        <AppFilterHeader
          className={options.hideTitle ? '' : 'router-block-gap-xs'}
          title={
            options.hideTitle
              ? t('group_manage.detail.supported_models')
              : t('group_manage.edit.model_configs')
          }
          titleTag={options.hideTitle ? 'h3' : 'div'}
          titleClassName={
            options.hideTitle
              ? 'router-entity-detail-section-title'
              : 'router-toolbar-title'
          }
          endClassName='router-block-gap-sm router-group-model-toolbar-end'
          actions={
            <AppInput
              className='router-inline-input router-search-form-sm router-group-model-search'
              placeholder={t('group_manage.edit.model_search_placeholder')}
              value={detailModelSearchKeyword}
              onChange={(e, { value }) => setDetailModelSearchKeyword(value || '')}
            />
          }
        />
        <AppTable
          className='router-detail-table router-group-supported-models-table'
          rowKey={(entry) => `group-detail-model-${entry.model || '-'}`}
          pagination={false}
          loading={detailModelLoading}
          locale={{ emptyText: t('group_manage.edit.empty_model_configs') }}
          dataSource={detailModelEntries}
          columns={[
            {
              title: t('group_manage.edit.model'),
              dataIndex: 'model',
              key: 'model',
              className: 'router-group-supported-models-col-model',
              render: (value) => value || '-',
            },
            {
              title: t('group_manage.detail.model_channels'),
              dataIndex: 'rows',
              key: 'rows',
              className: 'router-group-supported-models-col-channels',
              render: (rows) =>
                rows.length > 0 ? (
                  <div className='router-tag-group'>
                    {rows.map((item) => (
                      <AppTag
                        key={`${item?.model || '-'}-${item?.channel_id || '-'}-${item?.upstream_model || '-'}`}
                        className='router-tag'
                        color={channelStatusColor(item?.channel_status)}
                        onClick={(event) => {
                          event.preventDefault();
                          openChannelDetailFromCurrentPage(item.channel_id);
                        }}
                      >
                        {item?.channel_name || item?.channel_id}
                        {` · ${formatPriorityLabel(item?.priority)}`}
                      </AppTag>
                    ))}
                  </div>
                ) : (
                  '-'
                ),
            },
            {
              title: t('group_manage.detail.enabled'),
              key: 'enabled',
              className: 'router-group-supported-models-col-enabled',
              render: (_, entry) => (
                <AppSwitch
                  checked={entry.allEnabled}
                  disabled={submitting || detailModelsEditLocked}
                  onChange={(event, { checked }) => {
                    event?.stopPropagation?.();
                    toggleDetailModelEnabled(entry.model, !!checked);
                  }}
                />
              ),
            },
            {
              title: t('group_manage.table.actions'),
              key: 'actions',
              className: 'router-group-supported-models-col-actions',
              render: (_, entry) => (
                <AppButton
                  type='button'
                  className='router-inline-button'
                  disabled={submitting || detailModelsEditLocked}
                  onClick={() => openDetailModelEdit(entry)}
                >
                  {t('group_manage.buttons.edit')}
                </AppButton>
              ),
            },
          ]}
        />
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
          : null
      )
        .filter(Boolean)
        .map((item) => ({
        ...item,
        enabled: !!item.enabled,
      }))
    );
    await saveSingleDetailModelConfigs(
      normalizedModel,
      nextItems,
      editorState.channels,
    );
  }, [loadDetailModelEditorState, saveSingleDetailModelConfigs]);

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
    const saved = await saveSingleDetailModelConfigs(
      model,
      sortGroupModelConfigRows(nextRows),
      formModelChannels,
    );
    if (saved) {
      closeDetailModelModal();
    }
  }, [
    closeDetailModelModal,
    detailModelChannelDrafts,
    detailModelEditTarget,
    formModelChannels,
    saveSingleDetailModelConfigs,
    t,
  ]);

  const renderEditModelConfigTable = () => (
    <div className='router-block-top-md'>
      <AppFilterHeader
        className='router-block-gap-xs'
        title={t('group_manage.edit.model_configs')}
        actions={
          <>
            <AppButton
              type='button'
              className='router-inline-button'
              disabled={submitting || formModelLoading}
              onClick={addEmptyModelConfigRow}
            >
              {t('group_manage.buttons.add_model')}
            </AppButton>
            <AppInput
              className='router-inline-input router-search-form-sm'
              placeholder={t('group_manage.edit.model_search_placeholder')}
              value={editModelSearchKeyword}
              onChange={(e, { value }) => setEditModelSearchKeyword(value || '')}
            />
          </>
        }
      />
      <AppTable
        className='router-detail-table'
        rowKey={(record) => {
          const item = record?.item || {};
          const channelID = (item?.channel_id || '').toString().trim() || 'channel';
          const model = (item?.model || '').toString().trim() || 'model';
          const upstreamModel =
            (item?.upstream_model || '').toString().trim() || 'upstream';
          return `group-model-config-${record?.index ?? 0}-${channelID}-${model}-${upstreamModel}`;
        }}
        pagination={false}
        loading={formModelLoading}
        locale={{ emptyText: t('group_manage.edit.empty_model_configs') }}
        dataSource={visibleEditModelConfigs}
        columns={[
          {
            title: t('group_manage.edit.model'),
            key: 'model',
            className: 'router-cell-min-260',
            render: (_, record) => (
              <AppInput
                className='router-inline-input'
                placeholder={t('group_manage.edit.model_placeholder')}
                value={record?.item?.model || ''}
                onChange={(e, { value }) =>
                  updateModelConfigRow(record.index, (current) => ({
                    ...current,
                    model: value || '',
                  }))
                }
              />
            ),
          },
          {
            title: t('group_manage.edit.channel'),
            key: 'channel',
            className: 'router-cell-min-240',
            render: (_, record) => (
              <AppSelect
                className='router-inline-dropdown'
                search
                options={selectedFormChannelOptions}
                placeholder={t('group_manage.edit.channel_placeholder')}
                value={record?.item?.channel_id || ''}
                onChange={(e, { value }) => {
                  const nextChannelID = value || '';
                  const nextChannel = formModelChannelLookup[nextChannelID];
                  const nextModels = Array.isArray(nextChannel?.models) ? nextChannel.models : [];
                  const firstModel = nextModels[0] || null;
                  updateModelConfigRow(record.index, (current) => ({
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
            ),
          },
          {
            title: t('group_manage.edit.upstream_model'),
            key: 'upstream_model',
            className: 'router-cell-min-280',
            render: (_, record) => {
              const item = record?.item || {};
              const modelOptions = getChannelModelOptions(item?.channel_id || '');
              return (
                <AppSelect
                  className='router-inline-dropdown'
                  search
                  disabled={(item?.channel_id || '') === '' || modelOptions.length === 0}
                  options={modelOptions}
                  placeholder={t('group_manage.edit.upstream_model_placeholder')}
                  value={resolveChannelModelOptionValue(item)}
                  onChange={(e, { value }) => {
                    const decoded = decodeChannelModelOptionValue(value);
                    updateModelConfigRow(record.index, (current) => ({
                      ...current,
                      upstream_model: decoded.upstream_model || '',
                      model:
                        (current?.model || '').trim() !== ''
                          ? current.model
                          : decoded.model || '',
                    }));
                  }}
                />
              );
            },
          },
          {
            title: t('group_manage.table.actions'),
            key: 'actions',
            width: 120,
            render: (_, record) => (
              <AppButton
                type='button'
                className='router-inline-button'
                color='red'
                disabled={submitting}
                onClick={() => removeModelConfigRow(record.index)}
              >
                {t('group_manage.buttons.delete')}
              </AppButton>
            ),
          },
        ]}
      />
    </div>
  );

  const renderView = () => {
    if (!activeGroup) return null;
    return (
      <>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'resource', label: t('header.resource') },
          {
            key: 'group-list',
            label: t('header.group'),
            onClick: backToList,
          },
          {
            key: 'group-current',
            label: activeGroup.name || activeGroup.id || '-',
            active: true,
          },
        ]}
        title={t('group_manage.detail.title')}
      />
      <div className='router-tab-detail-page router-entity-detail-page'>
        <div className='router-entity-detail-tabs router-block-gap-sm'>
          <AppTabs
            className='router-detail-tab-nav'
            activeKey={activeDetailTab}
            onChange={setActiveDetailTab}
            items={[
              {
                key: 'overview',
                label: t('group_manage.detail_tabs.overview'),
                disabled:
                  isAnyDetailSectionEditing && activeDetailTab !== 'overview',
              },
              {
                key: 'channels',
                label: t('group_manage.detail_tabs.channels'),
                disabled:
                  isAnyDetailSectionEditing && activeDetailTab !== 'channels',
              },
              {
                key: 'models',
                label: t('group_manage.detail_tabs.models'),
                disabled:
                  isAnyDetailSectionEditing && activeDetailTab !== 'models',
              },
            ]}
          />
        </div>
        {activeDetailTab === 'overview' && (
          <AppDetailSection
            title={t('common.basic_info')}
            headerStart={renderGroupStatus(activeGroup.enabled)}
            headerEnd={
              detailBasicEditing ? (
                <>
                  <AppButton
                    type='button'
                    className='router-page-button'
                    disabled={submitting}
                    onClick={cancelDetailSectionEdit}
                  >
                    {t('group_manage.buttons.cancel')}
                  </AppButton>
                  <AppButton
                    type='button'
                    className='router-page-button'
                    color='blue'
                    loading={submitting}
                    disabled={submitting}
                    onClick={submitDetailBasic}
                  >
                    {t('group_manage.buttons.confirm')}
                  </AppButton>
                </>
              ) : (
                <AppButton
                  type='button'
                  className='router-page-button'
                  color='blue'
                  disabled={submitting || detailBasicEditLocked}
                  onClick={startDetailBasicEdit}
                >
                  {t('group_manage.buttons.edit')}
                </AppButton>
              )
            }
          >
              <AppFormRow>
                <AppField label='分组ID' readOnly>
                  <AppInput
                    className='router-section-input'
                    value={activeGroup.id || '-'}
                    readOnly
                  />
                </AppField>
                <AppField
                  label={t('group_manage.form.name')}
                  required={detailBasicEditing}
                  readOnly={!detailBasicEditing}
                >
                  <AppInput
                    className='router-section-input'
                    value={detailBasicEditing ? form.name : activeGroup.name || ''}
                    readOnly={!detailBasicEditing}
                    placeholder={t('group_manage.form.id_placeholder')}
                    onChange={(e, { value }) =>
                      setForm((prev) => ({ ...prev, name: value || '' }))
                    }
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField
                  label={t('group_manage.form.description')}
                  readOnly={!detailBasicEditing}
                >
                  <AppTextarea
                    className='router-section-textarea'
                    value={
                      detailBasicEditing
                        ? form.description
                        : activeGroup.description || ''
                    }
                    readOnly={!detailBasicEditing}
                    placeholder={t('group_manage.form.description_placeholder')}
                    onChange={(e, { value }) =>
                      setForm((prev) => ({
                        ...prev,
                        description: value || '',
                      }))
                    }
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField
                  label={t('group_manage.form.billing_ratio')}
                  hint={t('group_manage.form.billing_ratio_help')}
                  readOnly={!detailBasicEditing}
                >
                  <AppInputNumber
                    className='router-section-input'
                    min={0}
                    step={0.01}
                    precision={2}
                    fluid
                    value={
                      detailBasicEditing
                        ? form.billing_ratio
                        : Number(activeGroup.billing_ratio ?? 1).toFixed(2)
                    }
                    readOnly={!detailBasicEditing}
                    placeholder={t('group_manage.form.billing_ratio_placeholder')}
                    onChange={(e, { value }) =>
                      setForm((prev) => ({
                        ...prev,
                        billing_ratio: value ?? '',
                      }))
                    }
                  />
                </AppField>
              </AppFormRow>
              <AppFormRow>
                <AppField
                  label={t('group_manage.form.sort_order')}
                  readOnly={!detailBasicEditing}
                >
                  <AppInputNumber
                    className='router-section-input'
                    precision={0}
                    fluid
                    value={
                      detailBasicEditing ? form.sort_order : activeGroup.sort_order || 0
                    }
                    readOnly={!detailBasicEditing}
                    onChange={(e, { value }) =>
                      setForm((prev) => ({
                        ...prev,
                        sort_order: Number(value || 0),
                      }))
                    }
                  />
                </AppField>
                <AppField
                  label={t('group_manage.table.created_at')}
                  readOnly
                >
                  <AppInput
                    className='router-section-input'
                    value={
                      activeGroup.created_at
                        ? timestamp2string(activeGroup.created_at)
                        : '-'
                    }
                    readOnly
                  />
                </AppField>
                <AppField
                  label={t('group_manage.table.updated_at')}
                  readOnly
                >
                  <AppInput
                    className='router-section-input'
                    value={
                      activeGroup.updated_at
                        ? timestamp2string(activeGroup.updated_at)
                        : '-'
                    }
                    readOnly
                  />
                </AppField>
              </AppFormRow>
          </AppDetailSection>
        )}
        {activeDetailTab === 'channels' && (
          <AppDetailSection
            title={t('group_manage.detail.bound_channels')}
            headerEnd={
              <AppButton
                type='button'
                className='router-page-button'
                color='blue'
                disabled={submitting || detailChannelsEditLocked || detailAvailableChannelOptions.length === 0}
                onClick={startDetailChannelsEdit}
              >
                {t('group_manage.buttons.add_channel')}
              </AppButton>
            }
          >
            {renderDetailChannelsTable(detailChannelRows, detailChannelLoading)}
          </AppDetailSection>
        )}
        {activeDetailTab === 'models' && (
          <AppDetailSection>
            {renderDetailModelConfigTable({
              hideTitle: true,
            })}
          </AppDetailSection>
        )}
      </div>
      </>
    );
  };

  const renderEdit = () => (
    <div>
      <AppFormActions align='start' className='router-block-gap-sm'>
        <AppButton type='button' className='router-page-button' onClick={() => setMode(MODE_VIEW)} disabled={submitting}>
          {t('group_manage.buttons.cancel')}
        </AppButton>
        <AppButton type='button' className='router-page-button' color='blue' loading={submitting} disabled={submitting} onClick={submitEdit}>
          {t('group_manage.buttons.confirm')}
        </AppButton>
      </AppFormActions>
      <div>
        <AppFormRow>
          <AppField label={t('group_manage.form.id')} required>
            <AppInput
              className='router-section-input'
              placeholder={t('group_manage.form.id_placeholder')}
              value={form.name}
              onChange={(e, { value }) =>
                setForm((prev) => ({ ...prev, name: value || '' }))
              }
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField label={t('group_manage.form.description')}>
            <AppTextarea
              className='router-section-textarea'
              placeholder={t('group_manage.form.description_placeholder')}
              value={form.description}
              onChange={(e, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  description: value || '',
                }))
              }
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField
            label={t('group_manage.form.billing_ratio')}
            hint={t('group_manage.form.billing_ratio_help')}
          >
            <AppInputNumber
              className='router-section-input'
              min={0}
              step={0.01}
              precision={2}
              fluid
              placeholder={t('group_manage.form.billing_ratio_placeholder')}
              value={form.billing_ratio}
              onChange={(e, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  billing_ratio: value ?? '',
                }))
              }
            />
          </AppField>
          <AppField label={t('group_manage.form.sort_order')}>
            <AppInputNumber
              className='router-section-input'
              precision={0}
              fluid
              value={form.sort_order}
              onChange={(e, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  sort_order: Number(value || 0),
                }))
              }
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField label={t('group_manage.form.channels')}>
            <AppSelect
              className='router-section-dropdown'
              multiple
              search
              disabled={formChannelLoading || formModelLoading || submitting}
              options={formChannelOptions}
              value={formChannelIDs}
              placeholder={t('group_manage.form.channels_placeholder')}
              onChange={(e, { value }) =>
                setFormChannelIDs(Array.isArray(value) ? value : [])
              }
            />
          </AppField>
        </AppFormRow>
      </div>
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
      <AppFormActions align='start' className='router-block-gap-sm'>
        <AppButton type='button' className='router-page-button' onClick={backToList} disabled={submitting}>
          {t('group_manage.buttons.cancel')}
        </AppButton>
        <AppButton type='button' className='router-page-button' color='blue' loading={submitting} disabled={submitting} onClick={submitCreate}>
          {t('group_manage.buttons.confirm')}
        </AppButton>
      </AppFormActions>
      <div>
        <AppFormRow>
          <AppField label={t('group_manage.form.id')} required>
            <AppInput
              className='router-section-input'
              placeholder={t('group_manage.form.id_placeholder')}
              value={form.name}
              onChange={(e) =>
                setForm((prev) => ({ ...prev, name: e.target.value }))
              }
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField label={t('group_manage.form.description')}>
            <AppTextarea
              className='router-section-textarea'
              placeholder={t('group_manage.form.description_placeholder')}
              value={form.description}
              onChange={(e, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  description: value || '',
                }))
              }
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField
            label={t('group_manage.form.billing_ratio')}
            hint={t('group_manage.form.billing_ratio_help')}
          >
            <AppInputNumber
              className='router-section-input'
              min={0}
              step={0.01}
              precision={2}
              fluid
              placeholder={t('group_manage.form.billing_ratio_placeholder')}
              value={form.billing_ratio}
              onChange={(e, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  billing_ratio: value ?? '',
                }))
              }
            />
          </AppField>
          <AppField label={t('group_manage.form.channels')}>
            <AppSelect
              className='router-section-dropdown'
              multiple
              search
              disabled={formChannelLoading || submitting}
              options={formChannelOptions}
              value={formChannelIDs}
              placeholder={t('group_manage.form.channels_placeholder')}
              onChange={(e, { value }) =>
                setFormChannelIDs(Array.isArray(value) ? value : [])
              }
            />
          </AppField>
        </AppFormRow>
      </div>
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

      <AppModal
        open={detailModelModalOpen}
        onClose={closeDetailModelModal}
        size='large'
        title={t('group_manage.detail.edit_model_title', {
          model: detailModelEditTarget || '-',
        })}
        footer={null}
      >
        <div className='router-page-stack'>
          <AppTable
            className='router-detail-table'
            rowKey={(item) => `${detailModelEditTarget}-${item.channel_id}`}
            pagination={false}
            rowSelection={{
              selectedRowKeys: detailModelChannelDrafts
                .filter((item) => !!item.selected)
                .map((item) => `${detailModelEditTarget}-${item.channel_id}`),
              columnWidth: 84,
              columnTitle: t('group_manage.detail.selected'),
              onSelect: (record, selected) =>
                updateDetailModelChannelDraft(record.channel_id, (current) => ({
                  ...current,
                  selected: !!selected,
                })),
              onSelectAll: (selected, selectedRows, changeRows) => {
                (Array.isArray(changeRows) ? changeRows : []).forEach((item) => {
                  updateDetailModelChannelDraft(item.channel_id, (current) => ({
                    ...current,
                    selected: !!selected,
                  }));
                });
              },
            }}
            locale={{ emptyText: t('group_manage.detail.empty_model_channels') }}
            dataSource={detailModelChannelDrafts}
            columns={[
              {
                title: t('group_manage.edit.channel'),
                dataIndex: 'channel_name',
                key: 'channel_name',
                render: (_, item) => (
                  <AppTag
                    color={channelStatusColor(item?.channel_status)}
                    className='router-tag'
                  >
                    {item.channel_name || item.channel_id}
                    {item.channel_protocol ? ` · ${item.channel_protocol}` : ''}
                  </AppTag>
                ),
              },
              {
                title: t('group_manage.edit.upstream_model'),
                dataIndex: 'upstream_model',
                key: 'upstream_model',
                render: (value) => value || '-',
              },
              {
                title: t('group_manage.detail.priority'),
                key: 'priority',
                width: 140,
                render: (_, item) => (
                  <AppInputNumber
                    className='router-inline-input'
                    step={1}
                    precision={0}
                    disabled={!item.selected}
                    value={item.priority}
                    onChange={(e, { value }) =>
                      updateDetailModelChannelDraft(item.channel_id, (current) => ({
                        ...current,
                        priority: value ?? '',
                      }))
                    }
                  />
                ),
              },
            ]}
          />
          <AppFormActions>
            <AppButton
              className='router-modal-button'
              onClick={closeDetailModelModal}
              disabled={submitting}
            >
              {t('group_manage.buttons.cancel')}
            </AppButton>
            <AppButton
              className='router-modal-button'
              color='blue'
              onClick={submitDetailModelDraft}
              disabled={submitting}
            >
              {t('group_manage.buttons.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>

      <AppModal
        open={detailChannelModalOpen}
        onClose={closeDetailChannelModal}
        size='small'
        title={t('group_manage.modal.add_channel_title')}
        footer={null}
      >
          <div className='router-page-stack'>
            <AppFormRow>
              <AppField label={t('group_manage.form.channels')} required>
                <AppSelect
                  className='router-section-dropdown'
                  search
                  disabled={submitting}
                  placeholder={t('group_manage.form.channels_placeholder')}
                  options={detailAvailableChannelOptions}
                  value={detailChannelDraft.id || ''}
                  onChange={(e, { value }) =>
                    setDetailChannelDraft((prev) => ({
                      ...prev,
                      id: (value || '').toString(),
                    }))
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('group_manage.detail.priority')}>
                <AppInputNumber
                  className='router-section-input'
                  step={1}
                  precision={0}
                  fluid
                  value={detailChannelDraft.priority}
                  onChange={(e, { value }) =>
                    setDetailChannelDraft((prev) => ({
                      ...prev,
                      priority: Number.isFinite(Number(value))
                        ? Math.trunc(Number(value))
                        : 0,
                    }))
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormActions>
              <AppButton
                className='router-modal-button'
                onClick={closeDetailChannelModal}
                disabled={submitting}
              >
                {t('group_manage.buttons.cancel')}
              </AppButton>
              <AppButton
                className='router-modal-button'
                color='blue'
                onClick={submitDetailChannelDraft}
                loading={submitting}
              >
                {t('group_manage.buttons.confirm')}
              </AppButton>
            </AppFormActions>
          </div>
      </AppModal>

      <AppModal
        open={deleteOpen}
        onClose={closeDeleteModal}
        size='tiny'
        title={t('group_manage.modal.delete_title')}
        footer={[
          <AppButton
            key='cancel'
            className='router-modal-button'
            onClick={closeDeleteModal}
            disabled={submitting}
          >
            {t('group_manage.buttons.cancel')}
          </AppButton>,
          <AppButton
            key='confirm'
            className='router-modal-button'
            color='red'
            onClick={submitDelete}
            loading={submitting}
          >
            {t('group_manage.buttons.confirm')}
          </AppButton>,
        ]}
      >
        <div>
          {t('group_manage.modal.delete_confirm', {
            id: deleteTarget?.id || '',
          })}
        </div>
      </AppModal>
    </>
  );
};

export default GroupsManager;
