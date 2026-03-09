import React, {
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Icon, Label, Modal, Pagination, Table } from 'semantic-ui-react';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';

const normalizeProvider = (provider) => {
  if (typeof provider !== 'string') return '';
  const trimmed = provider.trim();
  if (!trimmed) return '';
  const lower = trimmed.toLowerCase();
  switch (lower) {
    case 'gpt':
    case 'openai':
      return 'openai';
    case 'gemini':
    case 'google':
      return 'google';
    case 'claude':
    case 'anthropic':
      return 'anthropic';
    case 'xai':
    case 'grok':
      return 'xai';
    case 'mistral':
      return 'mistral';
    case 'cohere':
    case 'command-r':
    case 'commandr':
      return 'cohere';
    case 'deepseek':
      return 'deepseek';
    case 'qwen':
    case 'qwq':
    case 'qvq':
      return 'qwen';
    case 'zhipu':
    case 'glm':
    case 'bigmodel':
      return 'zhipu';
    case 'hunyuan':
    case 'tencent':
      return 'hunyuan';
    case 'volc':
    case 'volcengine':
    case 'doubao':
    case 'ark':
      return 'volcengine';
    case 'minimax':
    case 'abab':
      return 'minimax';
    default:
      if (trimmed === '千问') return 'qwen';
      if (trimmed === '智谱') return 'zhipu';
      if (trimmed === '腾讯' || trimmed === '混元') return 'hunyuan';
      if (trimmed === '火山' || trimmed === '豆包' || trimmed === '字节') return 'volcengine';
      return lower;
  }
};

const inferModelType = (model) => {
  if (typeof model !== 'string') return 'text';
  const lower = model.trim().toLowerCase();
  if (!lower) return 'text';
  if (lower.includes('whisper') || lower.startsWith('tts-') || lower.includes('audio')) {
    return 'audio';
  }
  if (
    lower.startsWith('dall-e') ||
    lower.startsWith('cogview') ||
    lower.includes('stable-diffusion') ||
    lower.startsWith('wanx') ||
    lower.startsWith('step-1x') ||
    lower.includes('flux')
  ) {
    return 'image';
  }
  return 'text';
};

const defaultPriceUnitByType = (type, modelName) => {
  if (type === 'image') return 'per_image';
  if (type === 'audio') {
    if (
      typeof modelName === 'string' &&
      modelName.trim().toLowerCase().startsWith('tts-')
    ) {
      return 'per_1k_chars';
    }
    return 'per_1k_tokens';
  }
  return 'per_1k_tokens';
};

const createEmptyModelDetail = (model = '') => {
  const t = inferModelType(model);
  return {
    model,
    type: t,
    input_price: 0,
    output_price: 0,
    price_unit: defaultPriceUnitByType(t, model),
    currency: 'USD',
    source: 'manual',
    updated_at: 0,
  };
};

const normalizeModelDetails = (details) => {
  if (!Array.isArray(details)) return [];
  const unique = new Map();
  details.forEach((item) => {
    if (!item) return;
    const model =
      typeof item.model === 'string'
        ? item.model.trim()
        : typeof item.id === 'string'
        ? item.id.trim()
        : '';
    if (!model) return;
    const type =
      typeof item.type === 'string' && item.type.trim() !== ''
        ? item.type.trim().toLowerCase()
        : inferModelType(model);
    const inputPrice = Number(item.input_price || 0);
    const outputPrice = Number(item.output_price || 0);
    const currency =
      typeof item.currency === 'string' && item.currency.trim() !== ''
        ? item.currency.trim().toUpperCase()
        : 'USD';
    const priceUnit =
      typeof item.price_unit === 'string' && item.price_unit.trim() !== ''
        ? item.price_unit.trim().toLowerCase()
        : defaultPriceUnitByType(type, model);
    const source =
      typeof item.source === 'string' && item.source.trim() !== ''
        ? item.source.trim().toLowerCase()
        : 'manual';
    const updatedAt = Number(item.updated_at || 0);
    unique.set(model, {
      model,
      type,
      input_price: Number.isFinite(inputPrice) && inputPrice > 0 ? inputPrice : 0,
      output_price: Number.isFinite(outputPrice) && outputPrice > 0 ? outputPrice : 0,
      price_unit: priceUnit,
      currency,
      source,
      updated_at: Number.isInteger(updatedAt) && updatedAt > 0 ? updatedAt : 0,
    });
  });
  return Array.from(unique.values()).sort((a, b) => a.model.localeCompare(b.model));
};

const detailsFromCatalogItem = (item) => {
  if (Array.isArray(item?.model_details) && item.model_details.length > 0) {
    return normalizeModelDetails(item.model_details);
  }
  if (Array.isArray(item?.models) && item.models.length > 0) {
    return normalizeModelDetails(item.models.map((model) => ({ model })));
  }
  return [];
};

const createEmptyRow = () => ({
  id: '',
  name: '',
  base_url: '',
  model_details: [],
  sort_order: 0,
  source: 'manual',
  updated_at: 0,
});

const toEditableRows = (items) => {
  if (!Array.isArray(items)) return [];
  return items.map((item) => ({
    ...createEmptyRow(),
    id: normalizeProvider(item?.id || item?.provider || item?.name || ''),
    name: item?.name || '',
    base_url: item?.base_url || '',
    model_details: detailsFromCatalogItem(item),
    sort_order: Number(item?.sort_order || 0),
    source: item?.source || 'manual',
    updated_at: item?.updated_at || 0,
  }));
};

const OFFICIAL_PROVIDER_BASE_URLS = {
  openai: 'https://api.openai.com',
  google: 'https://generativelanguage.googleapis.com/v1beta/openai',
  anthropic: 'https://api.anthropic.com',
  xai: 'https://api.x.ai',
  mistral: 'https://api.mistral.ai',
  cohere: 'https://api.cohere.com/compatibility/v1',
  deepseek: 'https://api.deepseek.com',
  qwen: 'https://dashscope.aliyuncs.com/compatible-mode',
  zhipu: 'https://open.bigmodel.cn/api/paas/v4',
  hunyuan: 'https://api.hunyuan.cloud.tencent.com/v1',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3',
  minimax: 'https://api.minimax.chat/v1',
};

const MODEL_TYPE_OPTIONS = [
  { key: 'text', value: 'text', text: 'text' },
  { key: 'image', value: 'image', text: 'image' },
  { key: 'audio', value: 'audio', text: 'audio' },
];

const ModelProvidersManager = () => {
  const { t } = useTranslation();
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [deletingRow, setDeletingRow] = useState(null);
  const [creating, setCreating] = useState(false);
  const [createRow, setCreateRow] = useState(createEmptyRow());

  const [editing, setEditing] = useState(false);
  const [editRow, setEditRow] = useState(createEmptyRow());
  const [editModelSearchKeyword, setEditModelSearchKeyword] = useState('');
  const [viewingProvider, setViewingProvider] = useState('');
  const [viewRow, setViewRow] = useState(null);
  const [viewModelSearchKeyword, setViewModelSearchKeyword] = useState('');

  const normalizedSearchKeyword = useMemo(
    () => (typeof searchKeyword === 'string' ? searchKeyword.trim() : ''),
    [searchKeyword]
  );

  const totalPages = useMemo(() => {
    if (totalCount <= 0) return 1;
    return Math.ceil(totalCount / ITEMS_PER_PAGE);
  }, [totalCount]);

  const loadCatalog = useCallback(async (page, keyword) => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/provider', {
        params: {
          p: Math.max((page || 1) - 1, 0),
          page_size: ITEMS_PER_PAGE,
          keyword: keyword || undefined,
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('channel.providers.messages.load_failed'));
        return;
      }
      const items = Array.isArray(data?.items) ? data.items : [];
      setRows(toEditableRows(items));
      setTotalCount(Number(data?.total || 0));
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadCatalog(activePage, normalizedSearchKeyword).then();
  }, [activePage, normalizedSearchKeyword, loadCatalog]);

  useEffect(() => {
    if (activePage > totalPages) {
      setActivePage(totalPages);
    }
  }, [activePage, totalPages]);

  const setEditValue = (key, value) => {
    setEditRow((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const setCreateValue = (key, value) => {
    setCreateRow((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const openEditor = (row) => {
    if (!row || creating) return;
    setViewingProvider('');
    setViewRow(null);
    setEditModelSearchKeyword('');
    setEditRow({ ...row });
    setEditing(true);
  };

  const rollbackEditor = () => {
    setEditing(false);
    setEditModelSearchKeyword('');
    setEditRow(createEmptyRow());
  };

  const openCreateModal = () => {
    if (editing || creating || saving) return;
    setViewingProvider('');
    setViewRow(null);
    setCreateRow(createEmptyRow());
    setCreating(true);
  };

  const closeCreateModal = () => {
    setCreating(false);
    setCreateRow(createEmptyRow());
  };

  const openViewer = (row) => {
    if (creating || editing || saving) return;
    const normalized = normalizeProvider(row?.id || '');
    if (!normalized) return;
    setViewModelSearchKeyword('');
    setViewingProvider(normalized);
    setViewRow({ ...row });
  };

  const closeViewer = () => {
    setViewModelSearchKeyword('');
    setViewingProvider('');
    setViewRow(null);
  };

  const setModelDetailField = (setter, row, index, key, value) => {
    const details = Array.isArray(row.model_details) ? [...row.model_details] : [];
    if (index < 0 || index >= details.length) return;
    const next = { ...details[index] };
    if (key === 'input_price' || key === 'output_price') {
      const parsed = Number(value);
      next[key] = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
    } else if (key === 'currency') {
      next[key] = (value || '').toUpperCase();
    } else if (key === 'source') {
      next[key] = (value || '').toLowerCase();
    } else if (key === 'type') {
      const normalizedType = (value || '').toLowerCase() || inferModelType(next.model || '');
      next.type = normalizedType;
      if (!next.price_unit) {
        next.price_unit = defaultPriceUnitByType(normalizedType, next.model || '');
      }
    } else if (key === 'model') {
      next.model = value || '';
      if (!next.type) {
        next.type = inferModelType(next.model);
      }
      if (!next.price_unit) {
        next.price_unit = defaultPriceUnitByType(next.type, next.model);
      }
    } else {
      next[key] = value || '';
    }
    details[index] = next;
    setter('model_details', details);
  };

  const addModelDetailRow = (setter, row) => {
    const details = Array.isArray(row.model_details) ? [...row.model_details] : [];
    details.unshift(createEmptyModelDetail(''));
    setter('model_details', details);
  };

  const removeModelDetailRow = (setter, row, index) => {
    const details = Array.isArray(row.model_details) ? [...row.model_details] : [];
    if (index < 0 || index >= details.length) return;
    details.splice(index, 1);
    setter('model_details', details);
  };

  const reloadCurrentPage = async () => {
    await loadCatalog(activePage, normalizedSearchKeyword);
  };

  const saveProvider = async (method, url, row, options = {}) => {
    const provider = normalizeProvider(row.id);
    if (!provider) {
      showInfo(t('channel.providers.messages.provider_required'));
      return null;
    }
    const payload = {
      id: provider,
      name: (row.name || '').trim() || provider,
      base_url: (row.base_url || '').trim() || OFFICIAL_PROVIDER_BASE_URLS[provider] || '',
      model_details: normalizeModelDetails(row.model_details || []),
      sort_order: Number(row.sort_order || 0),
      source: row.source || 'manual',
      updated_at: row.updated_at || 0,
    };
    setSaving(true);
    try {
      const res = await API({
        method,
        url,
        data: payload,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('channel.providers.messages.save_failed'));
        return null;
      }
      const savedRow = toEditableRows([data])[0] || null;
      showSuccess(options.successMessage || t('channel.providers.messages.save_success'));
      await reloadCurrentPage();
      return savedRow;
    } catch (error) {
      showError(error);
      return null;
    } finally {
      setSaving(false);
    }
  };

  const openDeleteModal = (row) => {
    if (saving || creating || editing) return;
    if (!row) return;
    setDeletingRow(row);
  };

  const closeDeleteModal = () => {
    if (saving) return;
    setDeletingRow(null);
  };

  const confirmDeleteRow = async () => {
    const provider = normalizeProvider(deletingRow?.id || '');
    if (!provider) {
      setDeletingRow(null);
      return;
    }
    setSaving(true);
    try {
      const res = await API.delete(`/api/v1/admin/provider/${provider}`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('channel.providers.dialog.delete_confirm'));
        return;
      }
      showSuccess(t('channel.providers.dialog.delete_confirm'));
      if (viewingProvider === provider) {
        closeViewer();
      }
      if (rows.length === 1 && activePage > 1) {
        setActivePage((prev) => Math.max(prev - 1, 1));
      } else {
        await reloadCurrentPage();
      }
      setDeletingRow(null);
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  const applyEditToRows = async () => {
    const provider = normalizeProvider(editRow.id);
    if (!provider) {
      showInfo(t('channel.providers.messages.provider_required'));
      return;
    }
    const normalizedRow = {
      ...editRow,
      id: provider,
      name: (editRow.name || '').trim() || provider,
      base_url: (editRow.base_url || '').trim() || OFFICIAL_PROVIDER_BASE_URLS[provider] || '',
      model_details: normalizeModelDetails(editRow.model_details || []),
      sort_order: Number(editRow.sort_order || 0),
      source: editRow.source || 'manual',
      updated_at: Math.floor(Date.now() / 1000),
    };
    const saved = await saveProvider(
      'put',
      `/api/v1/admin/provider/${provider}`,
      normalizedRow
    );
    if (saved) {
      rollbackEditor();
      setViewingProvider(saved.id || '');
      setViewRow(saved);
    }
  };

  const applyCreateToRows = async () => {
    const provider = normalizeProvider(createRow.id);
    if (!provider) {
      showInfo(t('channel.providers.messages.provider_required'));
      return;
    }
    const normalizedRow = {
      ...createRow,
      id: provider,
      name: (createRow.name || '').trim() || provider,
      base_url: (createRow.base_url || '').trim() || OFFICIAL_PROVIDER_BASE_URLS[provider] || '',
      model_details: normalizeModelDetails(createRow.model_details || []),
      sort_order: Number(createRow.sort_order || 0),
      source: createRow.source || 'manual',
      updated_at: Math.floor(Date.now() / 1000),
    };
    const saved = await saveProvider('post', '/api/v1/admin/provider', normalizedRow);
    if (saved) {
      closeCreateModal();
      setViewingProvider(saved.id || '');
      setViewRow(saved);
    }
  };

  const renderModelDetailsTable = (row, setValueFn, disabled = false, options = {}) => {
    const details = Array.isArray(row.model_details) ? row.model_details : [];
    const searchable = options.searchable === true;
    const modelSearchKeyword =
      typeof options.searchKeyword === 'string' ? options.searchKeyword : '';
    const normalizedModelSearchKeyword = modelSearchKeyword.trim().toLowerCase();
    const detailRows = details.map((detail, index) => ({ detail, index }));
    const visibleDetailRows =
      normalizedModelSearchKeyword === ''
        ? detailRows
        : detailRows.filter(({ detail }) => {
            const haystack = [
              detail.model || '',
              detail.type || '',
              detail.price_unit || '',
              detail.currency || '',
              detail.source || '',
            ]
              .join(' ')
              .toLowerCase();
            return haystack.includes(normalizedModelSearchKeyword);
          });
    return (
      <div>
        <div
          style={{
            marginTop: 6,
            marginBottom: 4,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <div style={{ fontWeight: 600 }}>{t('channel.providers.dialog.model_details')}</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {searchable ? (
              <Form.Input
                size='small'
                icon='search'
                iconPosition='left'
                style={{ width: 260 }}
                placeholder={t('channel.providers.model_detail_table.search_placeholder')}
                value={modelSearchKeyword}
                onChange={(e, { value }) => {
                  if (typeof options.onSearchChange === 'function') {
                    options.onSearchChange(value || '');
                  }
                }}
              />
            ) : null}
            <Button
              type='button'
              size='tiny'
              disabled={disabled}
              onClick={() => addModelDetailRow(setValueFn, row)}
            >
              <Icon name='plus' />
              {t('channel.providers.model_detail_table.add')}
            </Button>
          </div>
        </div>
        <Table compact size='small' celled>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell width={4}>{t('channel.providers.model_detail_table.model')}</Table.HeaderCell>
              <Table.HeaderCell width={2}>{t('channel.providers.model_detail_table.type')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.input_price')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.output_price')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.price_unit')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.currency')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.source')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.actions')}</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {visibleDetailRows.length === 0 ? (
                <Table.Row>
                  <Table.Cell colSpan={8} textAlign='center'>
                  {t('channel.providers.model_detail_table.empty')}
                  </Table.Cell>
                </Table.Row>
            ) : (
              visibleDetailRows.map(({ detail, index: detailIndex }) => (
                <Table.Row key={`${detail.model || 'model'}-${detailIndex}`}>
                  <Table.Cell style={{ minWidth: 260 }}>
                    <Form.Input
                      fluid
                      size='small'
                      value={detail.model || ''}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'model', value || '')
                      }
                    />
                  </Table.Cell>
                  <Table.Cell style={{ minWidth: 120 }}>
                    <Form.Select
                      fluid
                      size='small'
                      options={MODEL_TYPE_OPTIONS}
                      value={detail.type || 'text'}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'type', value || 'text')
                      }
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Input
                      fluid
                      size='small'
                      type='number'
                      step='0.000001'
                      value={detail.input_price || 0}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'input_price', value || 0)
                      }
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Input
                      fluid
                      size='small'
                      type='number'
                      step='0.000001'
                      value={detail.output_price || 0}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'output_price', value || 0)
                      }
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Input
                      fluid
                      size='small'
                      value={detail.price_unit || ''}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'price_unit', value || '')
                      }
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Input
                      fluid
                      size='small'
                      value={detail.currency || 'USD'}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'currency', value || 'USD')
                      }
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Form.Input
                      fluid
                      size='small'
                      value={detail.source || 'manual'}
                      disabled={disabled}
                      onChange={(e, { value }) =>
                        setModelDetailField(setValueFn, row, detailIndex, 'source', value || 'manual')
                      }
                    />
                  </Table.Cell>
                  <Table.Cell textAlign='center'>
                    <Button
                      type='button'
                      icon
                      size='tiny'
                      color='red'
                      disabled={disabled}
                      onClick={() => removeModelDetailRow(setValueFn, row, detailIndex)}
                    >
                      <Icon name='trash' />
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

  const renderModelDetailsReadonly = (row, options = {}) => {
    const details = Array.isArray(row?.model_details) ? row.model_details : [];
    const searchable = options.searchable === true;
    const modelSearchKeyword =
      typeof options.searchKeyword === 'string' ? options.searchKeyword : '';
    const normalizedModelSearchKeyword = modelSearchKeyword.trim().toLowerCase();
    const visibleDetailRows =
      normalizedModelSearchKeyword === ''
        ? details
        : details.filter((detail) => {
            const haystack = [
              detail.model || '',
              detail.type || '',
              detail.price_unit || '',
              detail.currency || '',
              detail.source || '',
            ]
              .join(' ')
              .toLowerCase();
            return haystack.includes(normalizedModelSearchKeyword);
          });
    return (
      <div style={{ marginTop: 12 }}>
        <div
          style={{
            marginBottom: 8,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <div style={{ fontWeight: 600 }}>
            {t('channel.providers.dialog.model_details')}
          </div>
          {searchable ? (
            <Form.Input
              size='small'
              icon='search'
              iconPosition='left'
              style={{ width: 260 }}
              placeholder={t('channel.providers.model_detail_table.search_placeholder')}
              value={modelSearchKeyword}
              onChange={(e, { value }) => {
                if (typeof options.onSearchChange === 'function') {
                  options.onSearchChange(value || '');
                }
              }}
            />
          ) : null}
        </div>
        <Table compact size='small' celled>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell width={4}>{t('channel.providers.model_detail_table.model')}</Table.HeaderCell>
              <Table.HeaderCell width={2}>{t('channel.providers.model_detail_table.type')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.input_price')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.output_price')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.price_unit')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.currency')}</Table.HeaderCell>
              <Table.HeaderCell>{t('channel.providers.model_detail_table.source')}</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {visibleDetailRows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan={7} textAlign='center'>
                  {t('channel.providers.model_detail_table.empty')}
                </Table.Cell>
              </Table.Row>
            ) : (
              visibleDetailRows.map((detail, index) => (
                <Table.Row key={`${detail.model || 'model'}-${index}`}>
                  <Table.Cell style={{ minWidth: 260 }}>{detail.model || '-'}</Table.Cell>
                  <Table.Cell style={{ minWidth: 120 }}>{detail.type || 'text'}</Table.Cell>
                  <Table.Cell>{detail.input_price || 0}</Table.Cell>
                  <Table.Cell>{detail.output_price || 0}</Table.Cell>
                  <Table.Cell>{detail.price_unit || '-'}</Table.Cell>
                  <Table.Cell>{detail.currency || 'USD'}</Table.Cell>
                  <Table.Cell>{detail.source || 'manual'}</Table.Cell>
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>
    );
  };

  const renderRows = () => (
    <div>
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
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Button type='button' size='tiny' disabled={saving || loading} onClick={openCreateModal}>
            {t('channel.providers.buttons.add_provider')}
          </Button>
        </div>
        <Form style={{ width: '320px', maxWidth: '100%' }}>
          <Form.Input
            icon='search'
            iconPosition='left'
            placeholder={t('channel.providers.search')}
            value={searchKeyword}
            onChange={(e, { value }) => {
              setSearchKeyword(value || '');
              setActivePage(1);
            }}
          />
        </Form>
      </div>
      <Table basic='very' compact size='small' stackable className='router-hover-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell width={3}>{t('channel.providers.table.provider')}</Table.HeaderCell>
            <Table.HeaderCell width={4}>{t('channel.providers.table.name')}</Table.HeaderCell>
            <Table.HeaderCell width={2} textAlign='left'>{t('channel.providers.table.source')}</Table.HeaderCell>
            <Table.HeaderCell width={3} textAlign='left'>{t('channel.providers.table.updated_at')}</Table.HeaderCell>
            <Table.HeaderCell width={2} textAlign='left'>{t('channel.providers.table.actions')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {rows.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan={5} textAlign='center'>
                {loading ? t('common.loading') : t('channel.providers.table.empty')}
              </Table.Cell>
            </Table.Row>
          ) : (
            rows.map((row, index) => (
              <Table.Row
                key={`${row.id}-${index}`}
                onClick={() => {
                  openViewer(row);
                }}
                style={{
                  cursor: creating || editing || saving ? 'default' : 'pointer',
                }}
              >
                <Table.Cell>{row.id || '-'}</Table.Cell>
                <Table.Cell>{row.name || row.id || '-'}</Table.Cell>
                <Table.Cell textAlign='left'>
                  <Label>{row.source || '-'}</Label>
                </Table.Cell>
                <Table.Cell textAlign='left'>
                  {row.updated_at ? timestamp2string(row.updated_at) : '-'}
                </Table.Cell>
                <Table.Cell textAlign='left' style={{ whiteSpace: 'nowrap' }}>
                  <Button
                    type='button'
                    icon
                    size='tiny'
                    color='blue'
                    disabled={creating || saving}
                    onClick={(e) => {
                      e.stopPropagation();
                      openEditor(row);
                    }}
                  >
                    <Icon name='edit' />
                  </Button>
                  <Button
                    type='button'
                    icon
                    size='tiny'
                    color='red'
                    disabled={creating || saving}
                    onClick={(e) => {
                      e.stopPropagation();
                      openDeleteModal(row);
                    }}
                  >
                    <Icon name='trash' />
                  </Button>
                </Table.Cell>
              </Table.Row>
            ))
          )}
        </Table.Body>
      </Table>
      {totalPages > 1 ? (
        <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
          <Pagination
            activePage={activePage}
            totalPages={totalPages}
            onPageChange={(e, { activePage: nextActivePage }) => {
              setActivePage(Number(nextActivePage) || 1);
            }}
          />
        </div>
      ) : null}
    </div>
  );

  const renderEditor = () => (
    <div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-start',
          gap: '8px',
          flexWrap: 'wrap',
          marginBottom: 12,
        }}
      >
        <Button type='button' onClick={rollbackEditor} disabled={saving}>
          <Icon name='undo' />
          {t('channel.providers.dialog.cancel_create')}
        </Button>
        <Button type='button' color='blue' loading={saving} disabled={saving} onClick={applyEditToRows}>
          <Icon name='check' />
          {t('channel.providers.dialog.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Group widths='equal'>
          <Form.Input
            label={t('channel.providers.dialog.provider')}
            value={editRow.id}
            readOnly
          />
          <Form.Input
            label={t('channel.providers.dialog.name')}
            placeholder={t('channel.providers.dialog.name_placeholder')}
            value={editRow.name}
            onChange={(e, { value }) => setEditValue('name', value || '')}
          />
        </Form.Group>
        <Form.Input
          label={t('channel.providers.dialog.base_url')}
          placeholder={t('channel.providers.dialog.base_url_placeholder')}
          value={editRow.base_url}
          onChange={(e, { value }) => setEditValue('base_url', value || '')}
        />
      </Form>

      {renderModelDetailsTable(editRow, setEditValue, saving, {
        searchable: true,
        searchKeyword: editModelSearchKeyword,
        onSearchChange: setEditModelSearchKeyword,
      })}
    </div>
  );

  const renderViewer = () => {
    if (!viewRow) return null;
    return (
      <div>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'flex-start',
            gap: '8px',
            flexWrap: 'wrap',
            marginBottom: 12,
          }}
        >
          <Button type='button' onClick={closeViewer} disabled={saving}>
            <Icon name='undo' />
            {t('channel.providers.dialog.cancel')}
          </Button>
          <Button
            type='button'
            color='blue'
            disabled={saving}
            onClick={() => openEditor(viewRow)}
          >
            <Icon name='edit' />
            {t('channel.providers.dialog.edit')}
          </Button>
        </div>
        <Form>
          <Form.Group widths='equal'>
            <Form.Input
              label={t('channel.providers.dialog.provider')}
              value={viewRow.id || ''}
              readOnly
            />
            <Form.Input
              label={t('channel.providers.dialog.name')}
              value={viewRow.name || ''}
              readOnly
            />
          </Form.Group>
          <Form.Group widths='equal'>
            <Form.Input
              label={t('channel.providers.dialog.base_url')}
              value={viewRow.base_url || ''}
              readOnly
            />
            <Form.Input
              label={t('channel.providers.table.source')}
              value={viewRow.source || '-'}
              readOnly
            />
          </Form.Group>
          <Form.Input
            label={t('channel.providers.table.updated_at')}
            value={viewRow.updated_at ? timestamp2string(viewRow.updated_at) : '-'}
            readOnly
          />
        </Form>
        {renderModelDetailsReadonly(viewRow, {
          searchable: true,
          searchKeyword: viewModelSearchKeyword,
          onSearchChange: setViewModelSearchKeyword,
        })}
      </div>
    );
  };

  const renderCreateModal = () => (
    <Modal
      open={creating}
      onClose={closeCreateModal}
      size='large'
      closeOnDimmerClick={false}
    >
      <Modal.Header>{t('channel.providers.dialog.title_create')}</Modal.Header>
      <Modal.Content>
        <Form>
          <Form.Group widths='equal'>
            <Form.Input
              label={t('channel.providers.dialog.provider')}
              placeholder={t('channel.providers.dialog.provider_placeholder')}
              value={createRow.id}
              onChange={(e, { value }) => setCreateValue('id', normalizeProvider(value || ''))}
            />
            <Form.Input
              label={t('channel.providers.dialog.name')}
              placeholder={t('channel.providers.dialog.name_placeholder')}
              value={createRow.name}
              onChange={(e, { value }) => setCreateValue('name', value || '')}
            />
          </Form.Group>
          <Form.Input
            label={t('channel.providers.dialog.base_url')}
            placeholder={t('channel.providers.dialog.base_url_placeholder')}
            value={createRow.base_url}
            onChange={(e, { value }) => setCreateValue('base_url', value || '')}
          />
        </Form>

        {renderModelDetailsTable(createRow, setCreateValue, saving)}
      </Modal.Content>
      <Modal.Actions>
        <Button type='button' onClick={closeCreateModal} disabled={saving}>
          <Icon name='undo' />
          {t('channel.providers.dialog.cancel_create')}
        </Button>
        <Button type='button' color='blue' loading={saving} disabled={saving} onClick={applyCreateToRows}>
          <Icon name='check' />
          {t('channel.providers.dialog.confirm')}
        </Button>
      </Modal.Actions>
    </Modal>
  );

  const renderDeleteModal = () => {
    const providerName = deletingRow?.name || deletingRow?.id || '-';
    return (
      <Modal
        open={!!deletingRow}
        onClose={closeDeleteModal}
        size='tiny'
        closeOnDimmerClick={!saving}
      >
        <Modal.Header>{t('channel.providers.dialog.delete_title')}</Modal.Header>
        <Modal.Content>
          {t('channel.providers.dialog.delete_content', { provider: providerName })}
        </Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeDeleteModal} disabled={saving}>
            {t('channel.providers.dialog.cancel_create')}
          </Button>
          <Button
            type='button'
            color='red'
            loading={saving}
            disabled={saving}
            onClick={confirmDeleteRow}
          >
            <Icon name='trash' />
            {t('channel.providers.dialog.delete_confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    );
  };

  return (
    <div>
      {renderCreateModal()}
      {renderDeleteModal()}
      {editing ? renderEditor() : viewingProvider && viewRow ? renderViewer() : renderRows()}
    </div>
  );
};

export default ModelProvidersManager;
