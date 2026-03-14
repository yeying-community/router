import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Form,
  Header,
  Icon,
  Label,
  Modal,
  Pagination,
  Table,
} from 'semantic-ui-react';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';

const PROVIDER_DETAIL_MODEL_PAGE_SIZE = 20;
const PROVIDER_CAPABILITY_ORDER = ['text', 'image', 'audio', 'video'];

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
    case 'x-ai':
    case 'xai':
    case 'grok':
      return 'xai';
    case 'meta':
    case 'meta-llama':
    case 'meta_llama':
    case 'metallama':
      return 'meta';
    case 'mistral':
    case 'mistralai':
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
      if (trimmed === '火山' || trimmed === '豆包' || trimmed === '字节')
        return 'volcengine';
      return lower;
  }
};

const inferModelType = (model) => {
  if (typeof model !== 'string') return 'text';
  const lower = model.trim().toLowerCase();
  if (!lower) return 'text';
  if (
    lower.startsWith('veo') ||
    lower.includes('text-to-video') ||
    lower.includes('video-generation') ||
    lower.includes('video_generation') ||
    lower.includes('video')
  ) {
    return 'video';
  }
  if (
    lower.includes('whisper') ||
    lower.startsWith('tts-') ||
    lower.includes('audio')
  ) {
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
  if (type === 'video') return 'per_video';
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

const defaultPriceUnitByComponent = (component) => {
  const normalized = (component || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'image_generation':
      return 'per_image';
    case 'video_generation':
      return 'per_video';
    case 'audio_output':
      return 'per_1k_chars';
    case 'audio_input':
    case 'realtime_audio':
      return 'per_minute';
    case 'realtime_text':
    case 'text':
    default:
      return 'per_1k_tokens';
  }
};

function normalizeProviderCapabilityType(value, model) {
  const normalized = (value || '').toString().trim().toLowerCase();
  if (
    normalized === 'text' ||
    normalized === 'audio' ||
    normalized === 'image' ||
    normalized === 'video'
  ) {
    return normalized;
  }
  return inferModelType(model);
}

const normalizeCapabilities = (capabilities, model) => {
  const values = Array.isArray(capabilities)
    ? capabilities
    : typeof capabilities === 'string'
      ? capabilities.split(',')
      : [];
  const set = new Set();
  values.forEach((item) => {
    const normalized = normalizeProviderCapabilityType(item, model);
    if (normalized) {
      set.add(normalized);
    }
  });
  if (set.size === 0) {
    set.add(inferModelType(model));
  }
  return PROVIDER_CAPABILITY_ORDER.filter((type) => set.has(type));
};

const createEmptyPriceComponent = (component = '') => ({
  component,
  condition: '',
  input_price: 0,
  output_price: 0,
  price_unit: defaultPriceUnitByComponent(component),
  currency: 'USD',
  source: 'manual',
  source_url: '',
  updated_at: 0,
});

const normalizePriceComponents = (components) => {
  if (!Array.isArray(components)) return [];
  const unique = new Map();
  components.forEach((item, index) => {
    if (!item) return;
    const component = (item.component || '').toString().trim().toLowerCase();
    if (!component) return;
    const condition = (item.condition || '').toString().trim();
    const inputPrice = Number(item.input_price || 0);
    const outputPrice = Number(item.output_price || 0);
    const priceUnit =
      typeof item.price_unit === 'string' && item.price_unit.trim() !== ''
        ? item.price_unit.trim().toLowerCase()
        : defaultPriceUnitByComponent(component);
    const currency =
      typeof item.currency === 'string' && item.currency.trim() !== ''
        ? item.currency.trim().toUpperCase()
        : 'USD';
    const source =
      typeof item.source === 'string' && item.source.trim() !== ''
        ? item.source.trim().toLowerCase()
        : 'manual';
    const sourceUrl =
      typeof item.source_url === 'string' && item.source_url.trim() !== ''
        ? item.source_url.trim()
        : '';
    const updatedAt = Number(item.updated_at || 0);
    const sortOrder = Number(item.sort_order || 0);
    unique.set(`${component}\u0000${condition}\u0000${index}`, {
      component,
      condition,
      input_price:
        Number.isFinite(inputPrice) && inputPrice > 0 ? inputPrice : 0,
      output_price:
        Number.isFinite(outputPrice) && outputPrice > 0 ? outputPrice : 0,
      price_unit: priceUnit,
      currency,
      source,
      source_url: sourceUrl,
      sort_order: Number.isFinite(sortOrder) ? sortOrder : 0,
      updated_at: Number.isInteger(updatedAt) && updatedAt > 0 ? updatedAt : 0,
    });
  });
  return Array.from(unique.values()).sort((a, b) => {
    const bySort = Number(a.sort_order || 0) - Number(b.sort_order || 0);
    if (bySort !== 0) return bySort;
    const byComponent = (a.component || '').localeCompare(b.component || '');
    if (byComponent !== 0) return byComponent;
    return (a.condition || '').localeCompare(b.condition || '');
  });
};

const createEmptyModelDetail = (model = '') => {
  const t = inferModelType(model);
  return {
    model,
    type: t,
    capabilities: normalizeCapabilities([], model),
    input_price: 0,
    output_price: 0,
    price_unit: defaultPriceUnitByType(t, model),
    currency: 'USD',
    source: 'manual',
    updated_at: 0,
    price_components: [],
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
      capabilities: normalizeCapabilities(item.capabilities, model),
      input_price:
        Number.isFinite(inputPrice) && inputPrice > 0 ? inputPrice : 0,
      output_price:
        Number.isFinite(outputPrice) && outputPrice > 0 ? outputPrice : 0,
      price_unit: priceUnit,
      currency,
      source,
      updated_at: Number.isInteger(updatedAt) && updatedAt > 0 ? updatedAt : 0,
      price_components: normalizePriceComponents(item.price_components),
    });
  });
  return Array.from(unique.values()).sort((a, b) =>
    a.model.localeCompare(b.model),
  );
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
  official_url: '',
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
    official_url: item?.official_url || '',
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
  baidu: 'https://qianfan.baidubce.com/v2',
  qwen: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  zhipu: 'https://open.bigmodel.cn/api/paas/v4',
  hunyuan: 'https://api.hunyuan.cloud.tencent.com/v1',
  minimax: 'https://api.minimax.io/v1',
  stepfun: 'https://api.stepfun.com/v1',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3',
};

const MODEL_TYPE_OPTIONS = [
  { key: 'text', value: 'text', text: 'text' },
  { key: 'image', value: 'image', text: 'image' },
  { key: 'audio', value: 'audio', text: 'audio' },
  { key: 'video', value: 'video', text: 'video' },
];

const PRICE_UNIT_OPTIONS = [
  { key: 'per_1k_tokens', value: 'per_1k_tokens', text: 'per_1k_tokens' },
  { key: 'per_1k_chars', value: 'per_1k_chars', text: 'per_1k_chars' },
  { key: 'per_image', value: 'per_image', text: 'per_image' },
  { key: 'per_video', value: 'per_video', text: 'per_video' },
  { key: 'per_minute', value: 'per_minute', text: 'per_minute' },
  { key: 'per_second', value: 'per_second', text: 'per_second' },
  { key: 'per_request', value: 'per_request', text: 'per_request' },
  { key: 'per_task', value: 'per_task', text: 'per_task' },
];

const PRICE_COMPONENT_OPTIONS = [
  { key: 'text', value: 'text', text: 'text' },
  {
    key: 'image_generation',
    value: 'image_generation',
    text: 'image_generation',
  },
  { key: 'audio_input', value: 'audio_input', text: 'audio_input' },
  { key: 'audio_output', value: 'audio_output', text: 'audio_output' },
  {
    key: 'video_generation',
    value: 'video_generation',
    text: 'video_generation',
  },
  { key: 'realtime_text', value: 'realtime_text', text: 'realtime_text' },
  { key: 'realtime_audio', value: 'realtime_audio', text: 'realtime_audio' },
];

const SOURCE_OPTIONS = [
  { key: 'manual', value: 'manual', text: 'manual' },
  { key: 'default', value: 'default', text: 'default' },
  { key: 'official', value: 'official', text: 'official' },
  { key: 'imported', value: 'imported', text: 'imported' },
];

const TEXT_ENDPOINT_OPTIONS = [
  { key: '/v1/responses', value: '/v1/responses', text: '/v1/responses' },
  {
    key: '/v1/chat/completions',
    value: '/v1/chat/completions',
    text: '/v1/chat/completions',
  },
];

const IMAGE_QUALITY_OPTIONS = [
  { key: 'standard', value: 'standard', text: 'standard' },
  { key: 'hd', value: 'hd', text: 'hd' },
];

const IMAGE_SIZE_OPTIONS = [
  { key: '1024x1024', value: '1024x1024', text: '1024x1024' },
  { key: '1024x1792', value: '1024x1792', text: '1024x1792' },
  { key: '1792x1024', value: '1792x1024', text: '1792x1024' },
];

const parseConditionString = (condition) => {
  const result = {};
  const normalized = (condition || '').toString().trim();
  if (!normalized) return result;
  normalized.split(';').forEach((part) => {
    const pair = part.trim();
    if (!pair) return;
    const index = pair.indexOf('=');
    if (index <= 0) return;
    const key = pair.slice(0, index).trim().toLowerCase();
    const value = pair
      .slice(index + 1)
      .trim()
      .toLowerCase();
    if (!key) return;
    result[key] = value;
  });
  return result;
};

const buildConditionString = (attrs, orderedKeys) => {
  if (!attrs || typeof attrs !== 'object') return '';
  return orderedKeys
    .map((key) => {
      const normalizedKey = (key || '').toString().trim().toLowerCase();
      const value = (attrs[normalizedKey] || '')
        .toString()
        .trim()
        .toLowerCase();
      if (!normalizedKey || !value) return '';
      return `${normalizedKey}=${value}`;
    })
    .filter(Boolean)
    .join(';');
};

const collectProviderCapabilities = (row) => {
  const capabilitySet = new Set();
  detailsFromCatalogItem(row).forEach((detail) => {
    const type = normalizeProviderCapabilityType(detail?.type, detail?.model);
    capabilitySet.add(type);
  });
  return PROVIDER_CAPABILITY_ORDER.filter((type) => capabilitySet.has(type));
};

const formatProviderPriceCellValue = (value) => {
  const normalized = Number(value || 0);
  return Number.isFinite(normalized) && normalized > 0 ? normalized : '-';
};

const hasComplexInputPricing = (detail) =>
  Array.isArray(detail?.price_components) &&
  detail.price_components.some(
    (component) => Number(component?.input_price || 0) > 0,
  );

const hasComplexOutputPricing = (detail) =>
  Array.isArray(detail?.price_components) &&
  detail.price_components.some(
    (component) => Number(component?.output_price || 0) > 0,
  );

const ProvidersManager = () => {
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
  const [viewModelPage, setViewModelPage] = useState(1);
  const [pricingDetailOpen, setPricingDetailOpen] = useState(false);
  const [pricingDetailModel, setPricingDetailModel] = useState(null);

  const normalizedSearchKeyword = useMemo(
    () => (typeof searchKeyword === 'string' ? searchKeyword.trim() : ''),
    [searchKeyword],
  );

  const totalPages = useMemo(() => {
    if (totalCount <= 0) return 1;
    return Math.ceil(totalCount / ITEMS_PER_PAGE);
  }, [totalCount]);

  const loadCatalog = useCallback(
    async (page, keyword) => {
      setLoading(true);
      try {
        const res = await API.get('/api/v1/admin/providers', {
          params: {
            page: Math.max(page || 1, 1),
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
    },
    [t],
  );

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

  const openCreatePanel = () => {
    if (editing || creating || saving) return;
    setViewingProvider('');
    setViewRow(null);
    setCreateRow(createEmptyRow());
    setCreating(true);
  };

  const closeCreatePanel = () => {
    setCreating(false);
    setCreateRow(createEmptyRow());
  };

  const openViewer = (row) => {
    if (creating || editing || saving) return;
    const normalized = normalizeProvider(row?.id || '');
    if (!normalized) return;
    setViewModelSearchKeyword('');
    setViewModelPage(1);
    setViewingProvider(normalized);
    setViewRow({ ...row });
  };

  const closeViewer = () => {
    setViewModelSearchKeyword('');
    setViewModelPage(1);
    setViewingProvider('');
    setViewRow(null);
  };

  const openPricingDetail = useCallback((detail) => {
    setPricingDetailModel(detail || null);
    setPricingDetailOpen(true);
  }, []);

  const closePricingDetail = useCallback(() => {
    setPricingDetailOpen(false);
    setPricingDetailModel(null);
  }, []);

  const setModelDetailField = (setter, row, index, key, value) => {
    const details = Array.isArray(row.model_details)
      ? [...row.model_details]
      : [];
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
      const normalizedType =
        (value || '').toLowerCase() || inferModelType(next.model || '');
      next.type = normalizedType;
      next.capabilities = normalizeCapabilities(
        next.capabilities,
        next.model || '',
      );
      if (!next.price_unit) {
        next.price_unit = defaultPriceUnitByType(
          normalizedType,
          next.model || '',
        );
      }
    } else if (key === 'model') {
      next.model = value || '';
      if (!next.type) {
        next.type = inferModelType(next.model);
      }
      next.capabilities = normalizeCapabilities(
        next.capabilities,
        next.model || '',
      );
      if (!next.price_unit) {
        next.price_unit = defaultPriceUnitByType(next.type, next.model);
      }
    } else if (key === 'capabilities') {
      next.capabilities = normalizeCapabilities(value, next.model || '');
    } else {
      next[key] = value || '';
    }
    details[index] = next;
    setter('model_details', details);
  };

  const setPriceComponentField = (
    setter,
    row,
    detailIndex,
    componentIndex,
    key,
    value,
  ) => {
    const details = Array.isArray(row.model_details)
      ? [...row.model_details]
      : [];
    if (detailIndex < 0 || detailIndex >= details.length) return;
    const detail = { ...details[detailIndex] };
    const components = Array.isArray(detail.price_components)
      ? [...detail.price_components]
      : [];
    if (componentIndex < 0 || componentIndex >= components.length) return;
    const next = { ...components[componentIndex] };
    if (
      key === 'input_price' ||
      key === 'output_price' ||
      key === 'sort_order'
    ) {
      const parsed = Number(value);
      next[key] = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
    } else if (key === 'currency') {
      next[key] = (value || '').toUpperCase();
    } else if (key === 'source') {
      next[key] = (value || '').toLowerCase();
    } else if (key === 'component') {
      next.component = (value || '').toLowerCase();
      next.condition = '';
      next.price_unit = defaultPriceUnitByComponent(next.component);
    } else {
      next[key] = value || '';
    }
    components[componentIndex] = next;
    detail.price_components = normalizePriceComponents(components);
    details[detailIndex] = detail;
    setter('model_details', details);
  };

  const updatePriceComponentConditionTemplate = (
    setter,
    row,
    detailIndex,
    componentIndex,
    attrs,
    orderedKeys,
  ) => {
    const nextCondition = buildConditionString(attrs, orderedKeys);
    setPriceComponentField(
      setter,
      row,
      detailIndex,
      componentIndex,
      'condition',
      nextCondition,
    );
  };

  const renderPriceComponentConditionTemplate = (
    setter,
    row,
    detailIndex,
    componentIndex,
    component,
    disabled,
  ) => {
    const componentType = (component?.component || '')
      .toString()
      .trim()
      .toLowerCase();
    const attrs = parseConditionString(component?.condition || '');
    if (componentType === 'text') {
      return (
        <div className='router-block-top-sm'>
          <Form.Select
            className='router-inline-dropdown'
            fluid
            options={TEXT_ENDPOINT_OPTIONS}
            placeholder={t(
              'channel.providers.price_component_table.template.endpoint',
            )}
            value={attrs.endpoint || ''}
            disabled={disabled}
            clearable
            onChange={(e, { value }) => {
              updatePriceComponentConditionTemplate(
                setter,
                row,
                detailIndex,
                componentIndex,
                { endpoint: value || '' },
                ['endpoint'],
              );
            }}
          />
        </div>
      );
    }
    if (componentType === 'image_generation') {
      return (
        <div className='router-block-top-sm'>
          <Form.Group widths='equal'>
            <Form.Select
              className='router-inline-dropdown'
              fluid
              options={IMAGE_QUALITY_OPTIONS}
              placeholder={t(
                'channel.providers.price_component_table.template.quality',
              )}
              value={attrs.quality || ''}
              disabled={disabled}
              clearable
              onChange={(e, { value }) => {
                updatePriceComponentConditionTemplate(
                  setter,
                  row,
                  detailIndex,
                  componentIndex,
                  { quality: value || '', size: attrs.size || '' },
                  ['quality', 'size'],
                );
              }}
            />
            <Form.Select
              className='router-inline-dropdown'
              fluid
              options={IMAGE_SIZE_OPTIONS}
              placeholder={t(
                'channel.providers.price_component_table.template.size',
              )}
              value={attrs.size || ''}
              disabled={disabled}
              clearable
              onChange={(e, { value }) => {
                updatePriceComponentConditionTemplate(
                  setter,
                  row,
                  detailIndex,
                  componentIndex,
                  { quality: attrs.quality || '', size: value || '' },
                  ['quality', 'size'],
                );
              }}
            />
          </Form.Group>
        </div>
      );
    }
    if (
      componentType === 'audio_input' ||
      componentType === 'audio_output' ||
      componentType === 'video_generation' ||
      componentType === 'realtime_text' ||
      componentType === 'realtime_audio'
    ) {
      return (
        <div className='router-block-top-sm'>
          {t('channel.providers.price_component_table.template.no_condition')}
        </div>
      );
    }
    return null;
  };

  const addPriceComponentRow = (setter, row, detailIndex) => {
    const details = Array.isArray(row.model_details)
      ? [...row.model_details]
      : [];
    if (detailIndex < 0 || detailIndex >= details.length) return;
    const detail = { ...details[detailIndex] };
    const components = Array.isArray(detail.price_components)
      ? [...detail.price_components]
      : [];
    components.push(createEmptyPriceComponent('text'));
    detail.price_components = normalizePriceComponents(components);
    details[detailIndex] = detail;
    setter('model_details', details);
  };

  const removePriceComponentRow = (
    setter,
    row,
    detailIndex,
    componentIndex,
  ) => {
    const details = Array.isArray(row.model_details)
      ? [...row.model_details]
      : [];
    if (detailIndex < 0 || detailIndex >= details.length) return;
    const detail = { ...details[detailIndex] };
    const components = Array.isArray(detail.price_components)
      ? [...detail.price_components]
      : [];
    if (componentIndex < 0 || componentIndex >= components.length) return;
    components.splice(componentIndex, 1);
    detail.price_components = normalizePriceComponents(components);
    details[detailIndex] = detail;
    setter('model_details', details);
  };

  const addModelDetailRow = (setter, row) => {
    const details = Array.isArray(row.model_details)
      ? [...row.model_details]
      : [];
    details.unshift(createEmptyModelDetail(''));
    setter('model_details', details);
  };

  const removeModelDetailRow = (setter, row, index) => {
    const details = Array.isArray(row.model_details)
      ? [...row.model_details]
      : [];
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
      base_url:
        (row.base_url || '').trim() ||
        OFFICIAL_PROVIDER_BASE_URLS[provider] ||
        '',
      official_url: (row.official_url || '').trim(),
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
      showSuccess(
        options.successMessage || t('channel.providers.messages.save_success'),
      );
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
      const res = await API.delete(`/api/v1/admin/providers/${provider}`);
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
      base_url:
        (editRow.base_url || '').trim() ||
        OFFICIAL_PROVIDER_BASE_URLS[provider] ||
        '',
      official_url: (editRow.official_url || '').trim(),
      model_details: normalizeModelDetails(editRow.model_details || []),
      sort_order: Number(editRow.sort_order || 0),
      source: editRow.source || 'manual',
      updated_at: Math.floor(Date.now() / 1000),
    };
    const saved = await saveProvider(
      'put',
      `/api/v1/admin/providers/${provider}`,
      normalizedRow,
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
      base_url:
        (createRow.base_url || '').trim() ||
        OFFICIAL_PROVIDER_BASE_URLS[provider] ||
        '',
      official_url: (createRow.official_url || '').trim(),
      model_details: normalizeModelDetails(createRow.model_details || []),
      sort_order: Number(createRow.sort_order || 0),
      source: createRow.source || 'manual',
      updated_at: Math.floor(Date.now() / 1000),
    };
    const saved = await saveProvider(
      'post',
      '/api/v1/admin/providers',
      normalizedRow,
    );
    if (saved) {
      closeCreatePanel();
      setViewingProvider(saved.id || '');
      setViewRow(saved);
    }
  };

  const renderModelDetailsTable = (
    row,
    setValueFn,
    disabled = false,
    options = {},
  ) => {
    const details = Array.isArray(row.model_details) ? row.model_details : [];
    const searchable = options.searchable === true;
    const modelSearchKeyword =
      typeof options.searchKeyword === 'string' ? options.searchKeyword : '';
    const normalizedModelSearchKeyword = modelSearchKeyword
      .trim()
      .toLowerCase();
    const detailRows = details.map((detail, index) => ({ detail, index }));
    const visibleDetailRows =
      normalizedModelSearchKeyword === ''
        ? detailRows
        : detailRows.filter(({ detail }) => {
            const haystack = [
              detail.model || '',
              detail.type || '',
              (detail.capabilities || []).join(','),
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
        <div className='router-toolbar router-toolbar-compact'>
          <div className='router-toolbar-title'>
            {t('channel.providers.dialog.model_details')}
          </div>
          <div className='router-toolbar-end'>
            {searchable ? (
              <Form.Input
                className='router-inline-input router-search-form-xs'
                icon='search'
                iconPosition='left'
                placeholder={t(
                  'channel.providers.model_detail_table.search_placeholder',
                )}
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
              className='router-inline-button'
              disabled={disabled}
              onClick={() => addModelDetailRow(setValueFn, row)}
            >
              {t('channel.providers.model_detail_table.add')}
            </Button>
          </div>
        </div>
        <Table compact celled className='router-detail-table'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell width={4}>
                {t('channel.providers.model_detail_table.model')}
              </Table.HeaderCell>
              <Table.HeaderCell width={2}>
                {t('channel.providers.model_detail_table.type')}
              </Table.HeaderCell>
              <Table.HeaderCell width={3}>
                {t('channel.providers.model_detail_table.capabilities')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.input_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.output_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.price_unit')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.currency')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.source')}
              </Table.HeaderCell>
              <Table.HeaderCell width={2}>
                {t('channel.providers.model_detail_table.price_components')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.actions')}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {visibleDetailRows.length === 0 ? (
              <Table.Row>
                <Table.Cell
                  className='router-empty-cell'
                  colSpan={10}
                  textAlign='center'
                >
                  {t('channel.providers.model_detail_table.empty')}
                </Table.Cell>
              </Table.Row>
            ) : (
              visibleDetailRows.map(({ detail, index: detailIndex }) => (
                <React.Fragment
                  key={`${detail.model || 'model'}-${detailIndex}`}
                >
                  <Table.Row>
                    <Table.Cell className='router-cell-min-260'>
                      <Form.Input
                        className='router-inline-input'
                        fluid
                        value={detail.model || ''}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'model',
                            value || '',
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell className='router-cell-min-120'>
                      <Form.Select
                        className='router-inline-dropdown'
                        fluid
                        options={MODEL_TYPE_OPTIONS}
                        value={detail.type || 'text'}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'type',
                            value || 'text',
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell className='router-cell-min-200'>
                      <Form.Input
                        className='router-inline-input'
                        fluid
                        placeholder='text,image,audio'
                        value={
                          Array.isArray(detail.capabilities)
                            ? detail.capabilities.join(', ')
                            : ''
                        }
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'capabilities',
                            value || '',
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Form.Input
                        className='router-inline-input'
                        fluid
                        type='number'
                        step='0.000001'
                        value={detail.input_price || 0}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'input_price',
                            value || 0,
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Form.Input
                        className='router-inline-input'
                        fluid
                        type='number'
                        step='0.000001'
                        value={detail.output_price || 0}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'output_price',
                            value || 0,
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Form.Input
                        className='router-inline-input'
                        fluid
                        value={detail.price_unit || ''}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'price_unit',
                            value || '',
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Form.Input
                        className='router-inline-input'
                        fluid
                        value={detail.currency || 'USD'}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'currency',
                            value || 'USD',
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Form.Select
                        className='router-inline-dropdown'
                        fluid
                        options={SOURCE_OPTIONS}
                        value={detail.source || 'manual'}
                        disabled={disabled}
                        onChange={(e, { value }) =>
                          setModelDetailField(
                            setValueFn,
                            row,
                            detailIndex,
                            'source',
                            value || 'manual',
                          )
                        }
                      />
                    </Table.Cell>
                    <Table.Cell>
                      <Button
                        type='button'
                        className='router-inline-button'
                        disabled={disabled}
                        onClick={() =>
                          addPriceComponentRow(setValueFn, row, detailIndex)
                        }
                      >
                        {t(
                          'channel.providers.model_detail_table.add_price_component',
                        )}
                      </Button>
                    </Table.Cell>
                    <Table.Cell textAlign='center'>
                      <Button
                        type='button'
                        className='router-inline-button'
                        icon
                        color='red'
                        disabled={disabled}
                        onClick={() =>
                          removeModelDetailRow(setValueFn, row, detailIndex)
                        }
                      >
                        <Icon name='trash' />
                      </Button>
                    </Table.Cell>
                  </Table.Row>
                  <Table.Row>
                    <Table.Cell colSpan={10}>
                      <div className='router-block-top-sm'>
                        <div className='router-toolbar router-toolbar-compact'>
                          <div className='router-toolbar-title'>
                            {t(
                              'channel.providers.model_detail_table.price_components',
                            )}
                          </div>
                        </div>
                        <Table
                          compact
                          celled
                          className='router-detail-subtable'
                        >
                          <Table.Header>
                            <Table.Row>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.component',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.condition',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.input_price',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.output_price',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.price_unit',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.currency',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.source',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.source_url',
                                )}
                              </Table.HeaderCell>
                              <Table.HeaderCell>
                                {t(
                                  'channel.providers.price_component_table.actions',
                                )}
                              </Table.HeaderCell>
                            </Table.Row>
                          </Table.Header>
                          <Table.Body>
                            {(detail.price_components || []).length === 0 ? (
                              <Table.Row>
                                <Table.Cell
                                  className='router-empty-cell'
                                  colSpan={9}
                                  textAlign='center'
                                >
                                  {t(
                                    'channel.providers.price_component_table.empty',
                                  )}
                                </Table.Cell>
                              </Table.Row>
                            ) : (
                              (detail.price_components || []).map(
                                (component, componentIndex) => (
                                  <Table.Row
                                    key={`${detail.model || 'model'}-${component.component || 'component'}-${component.condition || 'condition'}-${componentIndex}`}
                                  >
                                    <Table.Cell>
                                      <Form.Select
                                        className='router-inline-dropdown'
                                        fluid
                                        options={PRICE_COMPONENT_OPTIONS}
                                        value={component.component || 'text'}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'component',
                                            value || '',
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Input
                                        className='router-inline-input'
                                        fluid
                                        placeholder='quality=hd;size=1024x1024'
                                        value={component.condition || ''}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'condition',
                                            value || '',
                                          )
                                        }
                                        action={{
                                          icon: 'erase',
                                          type: 'button',
                                          disabled,
                                          onClick: () =>
                                            setPriceComponentField(
                                              setValueFn,
                                              row,
                                              detailIndex,
                                              componentIndex,
                                              'condition',
                                              '',
                                            ),
                                        }}
                                      />
                                      {renderPriceComponentConditionTemplate(
                                        setValueFn,
                                        row,
                                        detailIndex,
                                        componentIndex,
                                        component,
                                        disabled,
                                      )}
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Input
                                        className='router-inline-input'
                                        fluid
                                        type='number'
                                        step='0.000001'
                                        value={component.input_price || 0}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'input_price',
                                            value || 0,
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Input
                                        className='router-inline-input'
                                        fluid
                                        type='number'
                                        step='0.000001'
                                        value={component.output_price || 0}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'output_price',
                                            value || 0,
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Select
                                        className='router-inline-dropdown'
                                        fluid
                                        options={PRICE_UNIT_OPTIONS}
                                        value={
                                          component.price_unit ||
                                          defaultPriceUnitByComponent(
                                            component.component,
                                          )
                                        }
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'price_unit',
                                            value || '',
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Input
                                        className='router-inline-input'
                                        fluid
                                        value={component.currency || 'USD'}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'currency',
                                            value || 'USD',
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Select
                                        className='router-inline-dropdown'
                                        fluid
                                        options={SOURCE_OPTIONS}
                                        value={component.source || 'manual'}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'source',
                                            value || 'manual',
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell>
                                      <Form.Input
                                        className='router-inline-input'
                                        fluid
                                        value={component.source_url || ''}
                                        disabled={disabled}
                                        onChange={(e, { value }) =>
                                          setPriceComponentField(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                            'source_url',
                                            value || '',
                                          )
                                        }
                                      />
                                    </Table.Cell>
                                    <Table.Cell textAlign='center'>
                                      <Button
                                        type='button'
                                        className='router-inline-button'
                                        icon
                                        color='red'
                                        disabled={disabled}
                                        onClick={() =>
                                          removePriceComponentRow(
                                            setValueFn,
                                            row,
                                            detailIndex,
                                            componentIndex,
                                          )
                                        }
                                      >
                                        <Icon name='trash' />
                                      </Button>
                                    </Table.Cell>
                                  </Table.Row>
                                ),
                              )
                            )}
                          </Table.Body>
                        </Table>
                      </div>
                    </Table.Cell>
                  </Table.Row>
                </React.Fragment>
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
    const pageSize =
      Number(options.pageSize || 0) > 0
        ? Number(options.pageSize)
        : PROVIDER_DETAIL_MODEL_PAGE_SIZE;
    const currentPage =
      Number(options.currentPage || 0) > 0 ? Number(options.currentPage) : 1;
    const modelSearchKeyword =
      typeof options.searchKeyword === 'string' ? options.searchKeyword : '';
    const normalizedModelSearchKeyword = modelSearchKeyword
      .trim()
      .toLowerCase();
    const visibleDetailRows =
      normalizedModelSearchKeyword === ''
        ? details
        : details.filter((detail) => {
            const haystack = [
              detail.model || '',
              detail.type || '',
              (detail.capabilities || []).join(','),
              detail.price_unit || '',
              detail.currency || '',
              detail.source || '',
            ]
              .join(' ')
              .toLowerCase();
            return haystack.includes(normalizedModelSearchKeyword);
          });
    const totalPages = Math.max(
      1,
      Math.ceil(visibleDetailRows.length / pageSize),
    );
    const safeCurrentPage = Math.min(currentPage, totalPages);
    const pageRows = visibleDetailRows.slice(
      (safeCurrentPage - 1) * pageSize,
      safeCurrentPage * pageSize,
    );
    return (
      <div className='router-block-top-sm'>
        <div className='router-toolbar router-block-gap-xs'>
          <div className='router-toolbar-title'>
            {t('channel.providers.dialog.model_details')}
          </div>
          {searchable ? (
            <Form.Input
              className='router-inline-input router-search-form-xs'
              icon='search'
              iconPosition='left'
              placeholder={t(
                'channel.providers.model_detail_table.search_placeholder',
              )}
              value={modelSearchKeyword}
              onChange={(e, { value }) => {
                if (typeof options.onSearchChange === 'function') {
                  options.onSearchChange(value || '');
                }
              }}
            />
          ) : null}
        </div>
        <Table compact celled className='router-detail-table'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell width={4}>
                {t('channel.providers.model_detail_table.model')}
              </Table.HeaderCell>
              <Table.HeaderCell width={2}>
                {t('channel.providers.model_detail_table.type')}
              </Table.HeaderCell>
              <Table.HeaderCell width={3}>
                {t('channel.providers.model_detail_table.capabilities')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.input_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.output_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.price_unit')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.currency')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.source')}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {visibleDetailRows.length === 0 ? (
              <Table.Row>
                <Table.Cell
                  className='router-empty-cell'
                  colSpan={8}
                  textAlign='center'
                >
                  {t('channel.providers.model_detail_table.empty')}
                </Table.Cell>
              </Table.Row>
            ) : (
              pageRows.map((detail, index) => {
                const showInputDetail = hasComplexInputPricing(detail);
                const showOutputDetail = hasComplexOutputPricing(detail);
                return (
                  <Table.Row key={`${detail.model || 'model'}-${index}`}>
                    <Table.Cell className='router-cell-min-260'>
                      {detail.model || '-'}
                    </Table.Cell>
                    <Table.Cell className='router-cell-min-120'>
                      {detail.type || 'text'}
                    </Table.Cell>
                    <Table.Cell>
                      {(detail.capabilities || []).length > 0
                        ? detail.capabilities.map((type) => (
                            <Label
                              key={`${detail.model || 'model'}-${type}`}
                              basic
                              className='router-tag'
                            >
                              {t(`channel.model_types.${type}`)}
                            </Label>
                          ))
                        : '-'}
                    </Table.Cell>
                    <Table.Cell>
                      {showInputDetail ? (
                        <Button
                          type='button'
                          basic
                          className='router-inline-button'
                          onClick={() => openPricingDetail(detail)}
                        >
                          {t('channel.providers.model_detail_table.detail')}
                        </Button>
                      ) : (
                        formatProviderPriceCellValue(detail.input_price)
                      )}
                    </Table.Cell>
                    <Table.Cell>
                      {showOutputDetail ? (
                        <Button
                          type='button'
                          basic
                          className='router-inline-button'
                          onClick={() => openPricingDetail(detail)}
                        >
                          {t('channel.providers.model_detail_table.detail')}
                        </Button>
                      ) : (
                        formatProviderPriceCellValue(detail.output_price)
                      )}
                    </Table.Cell>
                    <Table.Cell>{detail.price_unit || '-'}</Table.Cell>
                    <Table.Cell>{detail.currency || 'USD'}</Table.Cell>
                    <Table.Cell>{detail.source || 'manual'}</Table.Cell>
                  </Table.Row>
                );
              })
            )}
          </Table.Body>
        </Table>
        {totalPages > 1 ? (
          <div className='router-pagination-wrap'>
            <Pagination
              className='router-section-pagination'
              activePage={safeCurrentPage}
              totalPages={totalPages}
              onPageChange={(e, { activePage: nextActivePage }) => {
                if (typeof options.onPageChange === 'function') {
                  options.onPageChange(Number(nextActivePage) || 1);
                }
              }}
            />
          </div>
        ) : null}
      </div>
    );
  };

  const renderRows = () => (
    <div>
      <div className='router-toolbar router-block-gap-sm'>
        <div className='router-toolbar-start'>
          <Button
            type='button'
            className='router-page-button'
            disabled={saving || loading}
            onClick={openCreatePanel}
          >
            {t('channel.providers.buttons.add_provider')}
          </Button>
          <Button
            type='button'
            className='router-page-button'
            disabled={saving || loading}
            loading={loading}
            onClick={reloadCurrentPage}
          >
            {t('channel.providers.buttons.refresh')}
          </Button>
        </div>
        <Form className='router-search-form-md'>
          <Form.Input
            className='router-section-input'
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
      <Table
        basic='very'
        compact
        stackable
        className='router-hover-table router-list-table'
      >
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell width={3}>
              {t('channel.providers.table.provider')}
            </Table.HeaderCell>
            <Table.HeaderCell width={4}>
              {t('channel.providers.table.name')}
            </Table.HeaderCell>
            <Table.HeaderCell width={3} textAlign='left'>
              {t('channel.providers.table.capabilities')}
            </Table.HeaderCell>
            <Table.HeaderCell width={3} textAlign='left'>
              {t('channel.providers.table.updated_at')}
            </Table.HeaderCell>
            <Table.HeaderCell width={2} textAlign='left'>
              {t('channel.providers.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {rows.length === 0 ? (
            <Table.Row>
              <Table.Cell
                className='router-empty-cell'
                colSpan={5}
                textAlign='center'
              >
                {loading
                  ? t('common.loading')
                  : t('channel.providers.table.empty')}
              </Table.Cell>
            </Table.Row>
          ) : (
            rows.map((row, index) => {
              const capabilities = collectProviderCapabilities(row);
              return (
                <Table.Row
                  key={`${row.id}-${index}`}
                  onClick={() => {
                    openViewer(row);
                  }}
                  className={
                    creating || editing || saving
                      ? undefined
                      : 'router-row-clickable'
                  }
                >
                  <Table.Cell>{row.id || '-'}</Table.Cell>
                  <Table.Cell>{row.name || row.id || '-'}</Table.Cell>
                  <Table.Cell textAlign='left'>
                    {capabilities.length > 0
                      ? capabilities.map((type) => (
                          <Label
                            key={`${row.id}-${type}`}
                            basic
                            className='router-tag'
                          >
                            {t(`channel.model_types.${type}`)}
                          </Label>
                        ))
                      : '-'}
                  </Table.Cell>
                  <Table.Cell textAlign='left'>
                    {row.updated_at ? timestamp2string(row.updated_at) : '-'}
                  </Table.Cell>
                  <Table.Cell textAlign='left' className='router-nowrap'>
                    <Button
                      type='button'
                      className='router-inline-button'
                      icon
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
                      className='router-inline-button'
                      icon
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
              );
            })
          )}
        </Table.Body>
      </Table>
      {totalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <Pagination
            className='router-section-pagination'
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
      <div className='router-toolbar-start router-block-gap-sm'>
        <Button
          type='button'
          className='router-page-button'
          onClick={rollbackEditor}
          disabled={saving}
        >
          {t('channel.providers.dialog.cancel_create')}
        </Button>
        <Button
          type='button'
          className='router-page-button'
          color='blue'
          loading={saving}
          disabled={saving}
          onClick={applyEditToRows}
        >
          {t('channel.providers.dialog.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Group widths='equal'>
          <Form.Input
            className='router-section-input'
            label={t('channel.providers.dialog.provider')}
            value={editRow.id}
            readOnly
          />
          <Form.Input
            className='router-section-input'
            label={t('channel.providers.dialog.name')}
            placeholder={t('channel.providers.dialog.name_placeholder')}
            value={editRow.name}
            onChange={(e, { value }) => setEditValue('name', value || '')}
          />
        </Form.Group>
        <Form.Input
          className='router-section-input'
          label={t('channel.providers.dialog.base_url')}
          placeholder={t('channel.providers.dialog.base_url_placeholder')}
          value={editRow.base_url}
          onChange={(e, { value }) => setEditValue('base_url', value || '')}
        />
        <Form.Input
          className='router-section-input'
          label={t('channel.providers.dialog.official_url')}
          placeholder={t('channel.providers.dialog.official_url_placeholder')}
          value={editRow.official_url}
          onChange={(e, { value }) => setEditValue('official_url', value || '')}
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
        <Header as='h2' className='router-page-title'>
          {t('channel.providers.dialog.title_detail')}
        </Header>
        <div className='router-toolbar-start router-block-gap-sm'>
          <Button
            type='button'
          className='router-page-button'
          onClick={closeViewer}
          disabled={saving}
        >
            {t('channel.providers.dialog.cancel')}
          </Button>
          <Button
            type='button'
            className='router-page-button'
            color='blue'
            disabled={saving}
            onClick={() => openEditor(viewRow)}
          >
            {t('channel.providers.dialog.edit')}
          </Button>
        </div>
        <Form>
          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('channel.providers.dialog.provider')}
              value={viewRow.id || ''}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('channel.providers.dialog.name')}
              value={viewRow.name || ''}
              readOnly
            />
          </Form.Group>
          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('channel.providers.dialog.base_url')}
              value={viewRow.base_url || ''}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('channel.providers.dialog.official_url')}
              value={viewRow.official_url || ''}
              readOnly
            />
          </Form.Group>
          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('channel.providers.table.source')}
              value={viewRow.source || '-'}
              readOnly
            />
          </Form.Group>
          <Form.Input
            className='router-section-input'
            label={t('channel.providers.table.updated_at')}
            value={
              viewRow.updated_at ? timestamp2string(viewRow.updated_at) : '-'
            }
            readOnly
          />
        </Form>
        {renderModelDetailsReadonly(viewRow, {
          searchable: true,
          searchKeyword: viewModelSearchKeyword,
          currentPage: viewModelPage,
          pageSize: PROVIDER_DETAIL_MODEL_PAGE_SIZE,
          onSearchChange: (value) => {
            setViewModelSearchKeyword(value || '');
            setViewModelPage(1);
          },
          onPageChange: setViewModelPage,
        })}
      </div>
    );
  };

  const renderCreatePanel = () => (
    <div>
      <div className='router-toolbar-start router-block-gap-sm'>
        <Button
          type='button'
          className='router-page-button'
          onClick={closeCreatePanel}
          disabled={saving}
        >
          {t('channel.providers.dialog.cancel_create')}
        </Button>
        <Button
          type='button'
          className='router-page-button'
          color='blue'
          loading={saving}
          disabled={saving}
          onClick={applyCreateToRows}
        >
          {t('channel.providers.dialog.confirm')}
        </Button>
      </div>
      <Form>
        <Form.Group widths='equal'>
          <Form.Input
            className='router-section-input'
            label={t('channel.providers.dialog.provider')}
            placeholder={t('channel.providers.dialog.provider_placeholder')}
            value={createRow.id}
            onChange={(e, { value }) =>
              setCreateValue('id', normalizeProvider(value || ''))
            }
          />
          <Form.Input
            className='router-section-input'
            label={t('channel.providers.dialog.name')}
            placeholder={t('channel.providers.dialog.name_placeholder')}
            value={createRow.name}
            onChange={(e, { value }) => setCreateValue('name', value || '')}
          />
        </Form.Group>
        <Form.Input
          className='router-section-input'
          label={t('channel.providers.dialog.base_url')}
          placeholder={t('channel.providers.dialog.base_url_placeholder')}
          value={createRow.base_url}
          onChange={(e, { value }) => setCreateValue('base_url', value || '')}
        />
        <Form.Input
          className='router-section-input'
          label={t('channel.providers.dialog.official_url')}
          placeholder={t('channel.providers.dialog.official_url_placeholder')}
          value={createRow.official_url}
          onChange={(e, { value }) =>
            setCreateValue('official_url', value || '')
          }
        />
      </Form>

      {renderModelDetailsTable(createRow, setCreateValue, saving)}
    </div>
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
        <Modal.Header>
          {t('channel.providers.dialog.delete_title')}
        </Modal.Header>
        <Modal.Content>
          {t('channel.providers.dialog.delete_content', {
            provider: providerName,
          })}
        </Modal.Content>
        <Modal.Actions>
          <Button
            type='button'
            className='router-modal-button'
            onClick={closeDeleteModal}
            disabled={saving}
          >
            {t('channel.providers.dialog.cancel_create')}
          </Button>
          <Button
            type='button'
            className='router-modal-button'
          color='red'
          loading={saving}
          disabled={saving}
          onClick={confirmDeleteRow}
        >
            {t('channel.providers.dialog.delete_confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    );
  };

  const renderPricingDetailModal = () => (
    <Modal size='large' open={pricingDetailOpen} onClose={closePricingDetail}>
      <Modal.Header>
        {t('channel.providers.dialog.pricing_detail_title')}
      </Modal.Header>
      <Modal.Content scrolling>
        <div className='router-block-gap-sm'>
          <div className='router-toolbar-start router-block-gap-sm'>
            <Label basic className='router-tag'>
              {pricingDetailModel?.model || '-'}
            </Label>
            <Label basic className='router-tag'>
              {pricingDetailModel?.type || 'text'}
            </Label>
          </div>
        </div>
        <Table celled compact className='router-detail-subtable'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.input_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.output_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.price_unit')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.currency')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.model_detail_table.source')}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            <Table.Row>
              <Table.Cell>
                {formatProviderPriceCellValue(pricingDetailModel?.input_price)}
              </Table.Cell>
              <Table.Cell>
                {formatProviderPriceCellValue(pricingDetailModel?.output_price)}
              </Table.Cell>
              <Table.Cell>{pricingDetailModel?.price_unit || '-'}</Table.Cell>
              <Table.Cell>{pricingDetailModel?.currency || 'USD'}</Table.Cell>
              <Table.Cell>{pricingDetailModel?.source || 'manual'}</Table.Cell>
            </Table.Row>
          </Table.Body>
        </Table>
        <Table celled compact className='router-detail-subtable'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.component')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.condition')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.input_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.output_price')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.price_unit')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.currency')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.source')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.providers.price_component_table.source_url')}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {(pricingDetailModel?.price_components || []).length === 0 ? (
              <Table.Row>
                <Table.Cell
                  colSpan={8}
                  className='router-empty-cell'
                  textAlign='center'
                >
                  {t('channel.providers.price_component_table.empty')}
                </Table.Cell>
              </Table.Row>
            ) : (
              (pricingDetailModel?.price_components || []).map(
                (component, componentIndex) => (
                  <Table.Row
                    key={`${pricingDetailModel?.model || 'model'}-${component.component || 'component'}-${component.condition || 'condition'}-${componentIndex}`}
                  >
                    <Table.Cell>{component.component || '-'}</Table.Cell>
                    <Table.Cell>{component.condition || '-'}</Table.Cell>
                    <Table.Cell>
                      {formatProviderPriceCellValue(component.input_price)}
                    </Table.Cell>
                    <Table.Cell>
                      {formatProviderPriceCellValue(component.output_price)}
                    </Table.Cell>
                    <Table.Cell>{component.price_unit || '-'}</Table.Cell>
                    <Table.Cell>{component.currency || 'USD'}</Table.Cell>
                    <Table.Cell>{component.source || 'manual'}</Table.Cell>
                    <Table.Cell>{component.source_url || '-'}</Table.Cell>
                  </Table.Row>
                ),
              )
            )}
          </Table.Body>
        </Table>
      </Modal.Content>
      <Modal.Actions>
        <Button
          type='button'
          className='router-modal-button'
          onClick={closePricingDetail}
        >
          {t('channel.providers.dialog.cancel')}
        </Button>
      </Modal.Actions>
    </Modal>
  );

  return (
    <div>
      {renderDeleteModal()}
      {renderPricingDetailModal()}
      {creating
        ? renderCreatePanel()
        : editing
          ? renderEditor()
          : viewingProvider && viewRow
            ? renderViewer()
            : renderRows()}
    </div>
  );
};

export default ProvidersManager;
