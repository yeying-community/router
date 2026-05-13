import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import {
  AppButton,
  AppDetailSection,
  AppEmpty,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppIcon,
  AppInput,
  AppInputNumber,
  AppModal,
  AppPagination,
  AppSelect,
  AppTable,
  AppTabs,
  AppTag,
  AppTextarea,
  AppToolbar,
} from '../router-ui';

const PROVIDER_DETAIL_MODEL_PAGE_SIZE = 20;
const PROVIDER_ENDPOINT_SORT_ORDER = {
  '/v1/chat/completions': 10,
  '/v1/responses': 20,
  '/v1/messages': 30,
  '/v1/images/generations': 40,
  '/v1/images/edits': 50,
  '/v1/batches': 60,
  '/v1/audio/speech': 70,
  '/v1/realtime': 80,
  '/v1/videos': 90,
};

const formatProviderModelUsageError = (message, t) => {
  if (typeof message !== 'string') return '';
  const marker = ' is still in use: ';
  const markerIndex = message.indexOf(marker);
  if (markerIndex < 0) return '';
  const detailPart = message.slice(markerIndex + marker.length).trim();
  if (!detailPart) return '';

  const blocks = detailPart
    .split(';')
    .map((item) => item.trim())
    .filter(Boolean);
  if (blocks.length === 0) return '';

  const lines = [t('channel.providers.messages.model_in_use')];
  blocks.forEach((block) => {
    const [rawKey, rawValue] = block.split('=');
    const key = (rawKey || '').trim();
    const value = (rawValue || '').trim();
    if (!key || !value) return;
    const labelKey = `channel.providers.messages.model_usage_${key}`;
    const label = t(labelKey, { defaultValue: key });
    lines.push(`${label}: ${value}`);
  });
  return lines.join('\n');
};

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

function normalizeProviderModelType(value, model) {
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

const normalizeProviderEndpoint = (endpoint) => {
  const normalized = (endpoint || '').toString().trim().toLowerCase();
  if (normalized.startsWith('/v1/chat/completions')) {
    return '/v1/chat/completions';
  }
  if (normalized.startsWith('/v1/responses')) {
    return '/v1/responses';
  }
  if (normalized.startsWith('/v1/messages')) {
    return '/v1/messages';
  }
  if (normalized.startsWith('/v1/images/generations')) {
    return '/v1/images/generations';
  }
  if (normalized.startsWith('/v1/images/edits')) {
    return '/v1/images/edits';
  }
  if (normalized.startsWith('/v1/batches')) {
    return '/v1/batches';
  }
  if (normalized.startsWith('/v1/audio/')) {
    return '/v1/audio/speech';
  }
  if (normalized.startsWith('/v1/realtime')) {
    return '/v1/realtime';
  }
  if (normalized.startsWith('/v1/videos')) {
    return '/v1/videos';
  }
  return '';
};

const isProviderEndpointAllowedForType = (type, endpoint) => {
  const normalizedType = normalizeProviderModelType(type, '');
  switch (normalizedType) {
    case 'image':
      return [
        '/v1/responses',
        '/v1/images/generations',
        '/v1/images/edits',
        '/v1/batches',
      ].includes(endpoint);
    case 'audio':
      return endpoint === '/v1/audio/speech' || endpoint === '/v1/realtime';
    case 'video':
      return endpoint === '/v1/videos';
    case 'text':
    default:
      return ['/v1/chat/completions', '/v1/responses', '/v1/messages'].includes(
        endpoint,
      );
  }
};

const normalizeSupportedEndpoints = (endpoints, type) => {
  const values = Array.isArray(endpoints)
    ? endpoints
    : typeof endpoints === 'string'
      ? endpoints.split(',')
      : [];
  const seen = new Set();
  const result = [];
  values.forEach((item) => {
    const endpoint = normalizeProviderEndpoint(item);
    if (!endpoint || !isProviderEndpointAllowedForType(type, endpoint)) {
      return;
    }
    if (seen.has(endpoint)) {
      return;
    }
    seen.add(endpoint);
    result.push(endpoint);
  });
  return result.sort(
    (a, b) =>
      (PROVIDER_ENDPOINT_SORT_ORDER[a] || 1000) -
        (PROVIDER_ENDPOINT_SORT_ORDER[b] || 1000) || a.localeCompare(b),
  );
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
    status: 'active',
    description: '',
    is_deleted: false,
    supported_endpoints: [],
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
    const status =
      typeof item.status === 'string' && item.status.trim() !== ''
        ? item.status.trim().toLowerCase()
        : 'active';
    const description =
      typeof item.description === 'string' ? item.description.trim() : '';
    const isDeleted = item.is_deleted === true;
    const updatedAt = Number(item.updated_at || 0);
    unique.set(model, {
      model,
      type,
      status,
      description,
      is_deleted: isDeleted,
      supported_endpoints: normalizeSupportedEndpoints(
        item.supported_endpoints,
        type,
      ),
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
  return Array.from(unique.values())
    .filter((item) => item.is_deleted !== true)
    .sort((a, b) => a.model.localeCompare(b.model));
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
  created_at: 0,
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
    created_at: item?.created_at || 0,
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

const cloneEditableRow = (row) => toEditableRows([row])[0] || createEmptyRow();
const cloneModelDetail = (detail) =>
  normalizeModelDetails([detail])[0] || createEmptyModelDetail('');

const MODEL_TYPE_OPTIONS = [
  { key: 'text', value: 'text', text: 'text' },
  { key: 'image', value: 'image', text: 'image' },
  { key: 'audio', value: 'audio', text: 'audio' },
  { key: 'video', value: 'video', text: 'video' },
];

const PROVIDER_MODEL_STATUS_OPTIONS = [
  { key: 'active', value: 'active', text: 'active' },
  { key: 'deprecated', value: 'deprecated', text: 'deprecated' },
];

const PROVIDER_ENDPOINT_OPTIONS = [
  {
    key: '/v1/chat/completions',
    value: '/v1/chat/completions',
    text: '/v1/chat/completions',
  },
  { key: '/v1/responses', value: '/v1/responses', text: '/v1/responses' },
  { key: '/v1/messages', value: '/v1/messages', text: '/v1/messages' },
  {
    key: '/v1/images/generations',
    value: '/v1/images/generations',
    text: '/v1/images/generations',
  },
  {
    key: '/v1/images/edits',
    value: '/v1/images/edits',
    text: '/v1/images/edits',
  },
  { key: '/v1/batches', value: '/v1/batches', text: '/v1/batches' },
  {
    key: '/v1/audio/speech',
    value: '/v1/audio/speech',
    text: '/v1/audio/speech',
  },
  { key: '/v1/videos', value: '/v1/videos', text: '/v1/videos' },
];

const providerEndpointOptionsForType = (type) =>
  PROVIDER_ENDPOINT_OPTIONS.filter((option) =>
    isProviderEndpointAllowedForType(type, option.value),
  );

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

const formatProviderPriceCellValue = (value) => {
  const normalized = Number(value || 0);
  return Number.isFinite(normalized) && normalized > 0 ? normalized : '-';
};

const isComponentBasedPricing = (detail) =>
  Array.isArray(detail?.price_components) && detail.price_components.length > 0;

const summarizeModelPriceUnit = (detail, t) => {
  if (isComponentBasedPricing(detail)) {
    return '-';
  }
  return detail?.price_unit || '-';
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
  const [refreshing, setRefreshing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [deletingRow, setDeletingRow] = useState(null);
  const [creating, setCreating] = useState(false);
  const [createRow, setCreateRow] = useState(createEmptyRow());
  const [viewingProvider, setViewingProvider] = useState('');
  const [viewRow, setViewRow] = useState(null);
  const [activeProviderDetailTab, setActiveProviderDetailTab] = useState('basic');
  const [viewModelSearchKeyword, setViewModelSearchKeyword] = useState('');
  const [viewModelPage, setViewModelPage] = useState(1);
  const [detailEditingSection, setDetailEditingSection] = useState('');
  const [detailBasicDraft, setDetailBasicDraft] = useState(createEmptyRow());
  const [detailModelsDraft, setDetailModelsDraft] = useState(createEmptyRow());
  const [detailEditingModelIndex, setDetailEditingModelIndex] = useState(-1);
  const [modelDetailEditorOpen, setModelDetailEditorOpen] = useState(false);
  const [modelDetailEditorMode, setModelDetailEditorMode] = useState('edit');
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
    async (page, keyword, options = {}) => {
      const withRefreshIndicator = options.withRefreshIndicator === true;
      setLoading(true);
      if (withRefreshIndicator) {
        setRefreshing(true);
      }
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
        if (withRefreshIndicator) {
          setRefreshing(false);
        }
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

  const setCreateValue = (key, value) => {
    setCreateRow((prev) => ({
      ...prev,
      [key]: typeof value === 'function' ? value(prev[key], prev) : value,
    }));
  };

  const resetDetailEditingState = useCallback(() => {
    setDetailEditingSection('');
    setDetailBasicDraft(createEmptyRow());
    setDetailModelsDraft(createEmptyRow());
    setDetailEditingModelIndex(-1);
    setModelDetailEditorOpen(false);
    setModelDetailEditorMode('edit');
  }, []);

  const openCreatePanel = () => {
    if (creating || saving) return;
    setViewingProvider('');
    setViewRow(null);
    resetDetailEditingState();
    setCreateRow(createEmptyRow());
    setCreating(true);
  };

  const closeCreatePanel = () => {
    setCreating(false);
    setCreateRow(createEmptyRow());
  };

  const openViewer = (row) => {
    if (creating || saving) return;
    const normalized = normalizeProvider(row?.id || '');
    if (!normalized) return;
    setViewModelSearchKeyword('');
    setViewModelPage(1);
    setActiveProviderDetailTab('basic');
    resetDetailEditingState();
    setViewingProvider(normalized);
    setViewRow(cloneEditableRow(row));
  };

  const closeViewer = () => {
    setViewModelSearchKeyword('');
    setViewModelPage(1);
    setActiveProviderDetailTab('basic');
    setViewingProvider('');
    setViewRow(null);
    resetDetailEditingState();
  };

  const startDetailSectionEdit = useCallback(
    (section, row = null) => {
      const sourceRow = cloneEditableRow(row || viewRow);
      if (!sourceRow?.id || saving || creating) {
        return;
      }
      setDetailEditingSection(section);
      if (section === 'basic') {
        setDetailBasicDraft(sourceRow);
      }
    },
    [creating, saving, viewRow],
  );

  const cancelDetailSectionEdit = useCallback(() => {
    resetDetailEditingState();
  }, [resetDetailEditingState]);

  const setDetailBasicValue = (key, value) => {
    setDetailBasicDraft((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const setDetailModelsValue = (key, value) => {
    setDetailModelsDraft((prev) => ({
      ...prev,
      [key]: typeof value === 'function' ? value(prev[key], prev) : value,
    }));
  };

  const openPricingDetail = useCallback((detail) => {
    setPricingDetailModel(detail || null);
    setPricingDetailOpen(true);
  }, []);

  const closePricingDetail = useCallback(() => {
    setPricingDetailOpen(false);
    setPricingDetailModel(null);
  }, []);

  const setModelDetailField = (setter, _row, index, key, value) => {
    setter('model_details', (currentDetails) => {
      const details = Array.isArray(currentDetails) ? [...currentDetails] : [];
      if (index < 0 || index >= details.length) return details;
      const next = { ...details[index] };
      if (key === 'input_price' || key === 'output_price') {
        next[key] = value === null || value === undefined ? '' : `${value}`;
      } else if (key === 'currency') {
        next[key] = (value || '').toUpperCase();
      } else if (key === 'source') {
        next[key] = (value || '').toLowerCase();
      } else if (key === 'type') {
        const normalizedType =
          (value || '').toLowerCase() || inferModelType(next.model || '');
        next.type = normalizedType;
        next.supported_endpoints = normalizeSupportedEndpoints(
          next.supported_endpoints,
          normalizedType,
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
        next.supported_endpoints = normalizeSupportedEndpoints(
          next.supported_endpoints,
          next.type,
        );
        if (!next.price_unit) {
          next.price_unit = defaultPriceUnitByType(next.type, next.model);
        }
      } else if (key === 'supported_endpoints') {
        next.supported_endpoints = normalizeSupportedEndpoints(
          value,
          next.type,
        );
      } else {
        next[key] = value || '';
      }
      details[index] = next;
      return details;
    });
  };

  const setPriceComponentField = (
    setter,
    row,
    detailIndex,
    componentIndex,
    key,
    value,
  ) => {
    setter('model_details', (currentDetails) => {
      const details = Array.isArray(currentDetails) ? [...currentDetails] : [];
      if (detailIndex < 0 || detailIndex >= details.length) return details;
      const detail = { ...details[detailIndex] };
      const components = Array.isArray(detail.price_components)
        ? [...detail.price_components]
        : [];
      if (componentIndex < 0 || componentIndex >= components.length) {
        return details;
      }
      const next = { ...components[componentIndex] };
      if (key === 'input_price' || key === 'output_price') {
        next[key] = value === null || value === undefined ? '' : `${value}`;
      } else if (key === 'sort_order') {
        next[key] = value === null || value === undefined ? '' : `${value}`;
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
      detail.price_components = components;
      details[detailIndex] = detail;
      return details;
    });
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
          <AppSelect
            className='router-inline-dropdown'
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
          <div className='router-provider-inline-grid'>
            <AppSelect
              className='router-inline-dropdown'
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
            <AppSelect
              className='router-inline-dropdown'
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
          </div>
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

  const addPriceComponentRow = (setter, _row, detailIndex) => {
    setter('model_details', (currentDetails) => {
      const details = Array.isArray(currentDetails) ? [...currentDetails] : [];
      if (detailIndex < 0 || detailIndex >= details.length) return details;
      const detail = { ...details[detailIndex] };
      const components = Array.isArray(detail.price_components)
        ? [...detail.price_components]
        : [];
      components.push(createEmptyPriceComponent('text'));
      detail.price_components = components;
      details[detailIndex] = detail;
      return details;
    });
  };

  const removePriceComponentRow = (
    setter,
    _row,
    detailIndex,
    componentIndex,
  ) => {
    setter('model_details', (currentDetails) => {
      const details = Array.isArray(currentDetails) ? [...currentDetails] : [];
      if (detailIndex < 0 || detailIndex >= details.length) return details;
      const detail = { ...details[detailIndex] };
      const components = Array.isArray(detail.price_components)
        ? [...detail.price_components]
        : [];
      if (componentIndex < 0 || componentIndex >= components.length) {
        return details;
      }
      components.splice(componentIndex, 1);
      detail.price_components = components;
      details[detailIndex] = detail;
      return details;
    });
  };

  const addModelDetailRow = (setter, _row) => {
    setter('model_details', (currentDetails) => {
      const details = Array.isArray(currentDetails) ? [...currentDetails] : [];
      details.unshift(createEmptyModelDetail(''));
      return details;
    });
  };

  const removeModelDetailRow = (setter, _row, index) => {
    setter('model_details', (currentDetails) => {
      const details = Array.isArray(currentDetails) ? [...currentDetails] : [];
      if (index < 0 || index >= details.length) return details;
      details.splice(index, 1);
      return details;
    });
  };

  const reloadCurrentPage = async () => {
    await loadCatalog(activePage, normalizedSearchKeyword, {
      withRefreshIndicator: true,
    });
  };

  const persistViewerModelDetails = useCallback(
    async (modelDetails) => {
      const sourceRow = cloneEditableRow(viewRow);
      const provider = normalizeProvider(sourceRow.id);
      if (!provider) {
        showInfo(t('channel.providers.messages.provider_required'));
        return null;
      }
      const saved = await saveProvider(
        'put',
        `/api/v1/admin/providers/${provider}`,
        {
          ...sourceRow,
          model_details: normalizeModelDetails(modelDetails || []),
          updated_at: Math.floor(Date.now() / 1000),
        },
      );
      if (saved) {
        setViewingProvider(saved.id || '');
        setViewRow(saved);
        resetDetailEditingState();
      }
      return saved;
    },
    [resetDetailEditingState, saveProvider, t, viewRow],
  );

  const closeModelDetailEditor = useCallback(() => {
    if (saving) {
      return;
    }
    resetDetailEditingState();
  }, [resetDetailEditingState, saving]);

  const startDetailModelEdit = useCallback(
    (index) => {
      const sourceRow = cloneEditableRow(viewRow);
      const details = Array.isArray(sourceRow.model_details)
        ? sourceRow.model_details
        : [];
      if (
        saving ||
        creating ||
        !sourceRow?.id ||
        index < 0 ||
        index >= details.length
      ) {
        return;
      }
      setDetailEditingSection('models');
      setDetailModelsDraft(sourceRow);
      setDetailEditingModelIndex(index);
      setModelDetailEditorMode('edit');
      setModelDetailEditorOpen(true);
    },
    [creating, saving, viewRow],
  );

  const startDetailModelCreate = useCallback(() => {
    const sourceRow = cloneEditableRow(viewRow);
    if (saving || creating || !sourceRow?.id) {
      return;
    }
    const nextDetails = [
      createEmptyModelDetail(''),
      ...(Array.isArray(sourceRow.model_details)
        ? sourceRow.model_details
        : []),
    ];
    setDetailEditingSection('models');
    setDetailModelsDraft({
      ...sourceRow,
      model_details: nextDetails,
    });
    setDetailEditingModelIndex(0);
    setModelDetailEditorMode('create');
    setModelDetailEditorOpen(true);
    setViewModelSearchKeyword('');
    setViewModelPage(1);
  }, [creating, saving, viewRow]);

  const saveDetailModelEdit = useCallback(async () => {
    const currentDetails = Array.isArray(detailModelsDraft.model_details)
      ? detailModelsDraft.model_details
      : [];
    const currentDetail =
      detailEditingModelIndex >= 0 &&
      detailEditingModelIndex < currentDetails.length
        ? cloneModelDetail(currentDetails[detailEditingModelIndex])
        : null;
    if (!currentDetail?.model) {
      showInfo(t('channel.providers.messages.model_required'));
      return;
    }
    await persistViewerModelDetails(currentDetails);
  }, [
    detailEditingModelIndex,
    detailModelsDraft.model_details,
    persistViewerModelDetails,
    t,
  ]);

  const deleteDetailModel = useCallback(
    async (index) => {
      const sourceRow = cloneEditableRow(viewRow);
      const details = Array.isArray(sourceRow.model_details)
        ? [...sourceRow.model_details]
        : [];
      if (saving || creating || index < 0 || index >= details.length) {
        return;
      }
      if (
        typeof window !== 'undefined' &&
        !window.confirm(
          t('channel.providers.model_detail_table.delete_confirm'),
        )
      ) {
        return;
      }
      details.splice(index, 1);
      await persistViewerModelDetails(details);
    },
    [creating, persistViewerModelDetails, saving, t, viewRow],
  );

  async function saveProvider(method, url, row, options = {}) {
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
        skipErrorHandler: true,
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
      const message =
        error?.response?.data?.message ||
        error?.message ||
        t('channel.providers.messages.save_failed');
      const formattedUsageError = formatProviderModelUsageError(message, t);
      showError(formattedUsageError || message);
      return null;
    } finally {
      setSaving(false);
    }
  }

  const openDeleteModal = (row) => {
    if (saving || creating) return;
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

  const saveViewerSection = async (section) => {
    const sourceRow = cloneEditableRow(viewRow);
    const provider = normalizeProvider(sourceRow.id);
    if (!provider) {
      showInfo(t('channel.providers.messages.provider_required'));
      return;
    }
    let normalizedRow = {
      ...sourceRow,
      id: provider,
      name: (sourceRow.name || '').trim() || provider,
      base_url:
        (sourceRow.base_url || '').trim() ||
        OFFICIAL_PROVIDER_BASE_URLS[provider] ||
        '',
      official_url: (sourceRow.official_url || '').trim(),
      model_details: normalizeModelDetails(sourceRow.model_details || []),
      sort_order: Number(sourceRow.sort_order || 0),
      source: sourceRow.source || 'manual',
      updated_at: Math.floor(Date.now() / 1000),
    };
    if (section === 'basic') {
      normalizedRow = {
        ...normalizedRow,
        name: (detailBasicDraft.name || '').trim() || provider,
        base_url:
          (detailBasicDraft.base_url || '').trim() ||
          OFFICIAL_PROVIDER_BASE_URLS[provider] ||
          '',
        official_url: (detailBasicDraft.official_url || '').trim(),
      };
    }
    if (section === 'models') {
      normalizedRow = {
        ...normalizedRow,
        model_details: normalizeModelDetails(
          detailModelsDraft.model_details || [],
        ),
      };
    }
    const saved = await saveProvider(
      'put',
      `/api/v1/admin/providers/${provider}`,
      normalizedRow,
    );
    if (saved) {
      setViewingProvider(saved.id || '');
      setViewRow(saved);
      resetDetailEditingState();
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
    const hideTitle = options.hideTitle === true;
    const showToolbar = options.showToolbar !== false;
    const normalizedModelSearchKeyword = modelSearchKeyword
      .trim()
      .toLowerCase();
    const detailRows = details.map((detail, index) => ({ detail, index }));
    let visibleDetailRows =
      normalizedModelSearchKeyword === ''
        ? detailRows
        : detailRows.filter(({ detail }) => {
            const haystack = [
              detail.model || '',
              detail.description || '',
              detail.status || '',
              detail.type || '',
              (detail.supported_endpoints || []).join(','),
              detail.price_unit || '',
              detail.currency || '',
            ]
              .join(' ')
              .toLowerCase();
            return haystack.includes(normalizedModelSearchKeyword);
          });
    const renderEditablePriceComponentsTable = (
      detail,
      detailIndex,
      setFieldValue,
      targetRow,
      isDisabled,
    ) => (
      <AppTable
        className='router-detail-subtable'
        size='small'
        pagination={false}
        rowKey={(component, componentIndex) =>
          `${detail.model || 'model'}-${component.component || 'component'}-${component.condition || 'condition'}-${componentIndex}`
        }
        dataSource={detail.price_components || []}
        locale={{
          emptyText: t('channel.providers.price_component_table.empty'),
        }}
        scroll={{ x: 1320 }}
        columns={[
          {
            title: t('channel.providers.price_component_table.component'),
            key: 'component',
            width: 180,
            render: (_, component, componentIndex) => (
              <AppSelect
                className='router-inline-dropdown'
                options={PRICE_COMPONENT_OPTIONS}
                value={component.component || 'text'}
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'component',
                    value || '',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.condition'),
            key: 'condition',
            width: 320,
            render: (_, component, componentIndex) => (
              <div>
                <div className='router-provider-inline-field'>
                  <AppInput
                    className='router-inline-input'
                    placeholder='quality=hd;size=1024x1024'
                    value={component.condition || ''}
                    disabled={isDisabled}
                    onChange={(e, { value }) =>
                      setPriceComponentField(
                        setFieldValue,
                        targetRow,
                        detailIndex,
                        componentIndex,
                        'condition',
                        value || '',
                      )
                    }
                  />
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    basic
                    disabled={isDisabled}
                    onClick={() =>
                      setPriceComponentField(
                        setFieldValue,
                        targetRow,
                        detailIndex,
                        componentIndex,
                        'condition',
                        '',
                      )
                    }
                  >
                    {t('common.clear')}
                  </AppButton>
                </div>
                {renderPriceComponentConditionTemplate(
                  setFieldValue,
                  targetRow,
                  detailIndex,
                  componentIndex,
                  component,
                  isDisabled,
                )}
              </div>
            ),
          },
          {
            title: t('channel.providers.price_component_table.input_price'),
            key: 'input_price',
            width: 180,
            render: (_, component, componentIndex) => (
              <AppInputNumber
                className='router-inline-input'
                step='0.000001'
                min={0}
                precision={6}
                value={component.input_price ?? ''}
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'input_price',
                    value ?? '',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.output_price'),
            key: 'output_price',
            width: 180,
            render: (_, component, componentIndex) => (
              <AppInputNumber
                className='router-inline-input'
                step='0.000001'
                min={0}
                precision={6}
                value={component.output_price ?? ''}
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'output_price',
                    value ?? '',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.price_unit'),
            key: 'price_unit',
            width: 180,
            render: (_, component, componentIndex) => (
              <AppSelect
                className='router-inline-dropdown'
                options={PRICE_UNIT_OPTIONS}
                value={
                  component.price_unit ??
                  defaultPriceUnitByComponent(component.component)
                }
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'price_unit',
                    value || '',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.currency'),
            key: 'currency',
            width: 120,
            render: (_, component, componentIndex) => (
              <AppInput
                className='router-inline-input'
                value={component.currency ?? ''}
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'currency',
                    value ?? '',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.source'),
            key: 'source',
            width: 160,
            render: (_, component, componentIndex) => (
              <AppSelect
                className='router-inline-dropdown'
                options={SOURCE_OPTIONS}
                value={component.source || 'manual'}
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'source',
                    value || 'manual',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.source_url'),
            key: 'source_url',
            width: 220,
            render: (_, component, componentIndex) => (
              <AppInput
                className='router-inline-input'
                value={component.source_url || ''}
                disabled={isDisabled}
                onChange={(e, { value }) =>
                  setPriceComponentField(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                    'source_url',
                    value || '',
                  )
                }
              />
            ),
          },
          {
            title: t('channel.providers.price_component_table.actions'),
            key: 'actions',
            width: 120,
            render: (_, component, componentIndex) => (
              <AppButton
                type='button'
                className='router-inline-button'
                color='red'
                disabled={isDisabled}
                onClick={() =>
                  removePriceComponentRow(
                    setFieldValue,
                    targetRow,
                    detailIndex,
                    componentIndex,
                  )
                }
              >
                <AppIcon name='trash' />
              </AppButton>
            ),
          },
        ]}
      />
    );

    return (
      <div>
        {showToolbar ? (
          <AppFilterHeader
            className='router-toolbar-compact'
            title={
              hideTitle ? null : t('channel.providers.dialog.model_details')
            }
            actions={
              <>
                {searchable ? (
                  <AppInput
                    className='router-inline-input router-search-form-xs'
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
                <AppButton
                  type='button'
                  className='router-inline-button'
                  disabled={disabled}
                  onClick={() => addModelDetailRow(setValueFn, row)}
                >
                  {t('channel.providers.model_detail_table.add')}
                </AppButton>
              </>
            }
          />
        ) : null}
        <AppTable
          className='router-detail-table'
          size='small'
          pagination={false}
          rowKey={(record) =>
            `${record?.detail?.model || 'model'}-${record?.index ?? '0'}`
          }
          dataSource={visibleDetailRows}
          locale={{
            emptyText: t('channel.providers.model_detail_table.empty'),
          }}
          scroll={{ x: 1480 }}
          expandable={{
            expandedRowKeys: visibleDetailRows.map(
              ({ detail, index }) => `${detail?.model || 'model'}-${index}`,
            ),
            showExpandColumn: false,
            expandedRowRender: ({ detail, index: detailIndex }) => (
              <div className='router-block-top-sm'>
                <AppFilterHeader
                  className='router-toolbar-compact'
                  title={t('channel.providers.model_detail_table.price_components')}
                />
                {renderEditablePriceComponentsTable(
                  detail,
                  detailIndex,
                  setValueFn,
                  row,
                  disabled,
                )}
              </div>
            ),
          }}
          columns={[
            {
              title: t('channel.providers.model_detail_table.model'),
              key: 'model',
              width: 160,
              render: (_, { detail, index: detailIndex }) => (
                <div className='router-cell-min-130'>
                  <AppInput
                    className='router-inline-input'
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
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.description'),
              key: 'description',
              width: 240,
              render: (_, { detail, index: detailIndex }) => (
                <div className='router-cell-min-220'>
                  <AppTextarea
                    className='router-inline-input'
                    rows={2}
                    value={detail.description || ''}
                    disabled={disabled}
                    placeholder={t(
                      'channel.providers.model_detail_table.description',
                    )}
                    onChange={(e, { value }) =>
                      setModelDetailField(
                        setValueFn,
                        row,
                        detailIndex,
                        'description',
                        value || '',
                      )
                    }
                  />
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.status'),
              key: 'status',
              width: 140,
              render: (_, { detail, index: detailIndex }) => (
                <div className='router-cell-min-120'>
                  <AppSelect
                    className='router-inline-dropdown'
                    options={PROVIDER_MODEL_STATUS_OPTIONS}
                    value={detail.status || 'active'}
                    disabled={disabled}
                    onChange={(e, { value }) =>
                      setModelDetailField(
                        setValueFn,
                        row,
                        detailIndex,
                        'status',
                        value || 'active',
                      )
                    }
                  />
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.type'),
              key: 'type',
              width: 140,
              render: (_, { detail, index: detailIndex }) => (
                <div className='router-cell-min-120'>
                  <AppSelect
                    className='router-inline-dropdown'
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
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.supported_endpoints'),
              key: 'supported_endpoints',
              width: 360,
              render: (_, { detail, index: detailIndex }) => (
                <div className='router-cell-min-360'>
                  <AppSelect
                    className='router-inline-dropdown'
                    multiple
                    clearable
                    options={providerEndpointOptionsForType(detail.type)}
                    placeholder={t(
                      'channel.providers.model_detail_table.supported_endpoints',
                    )}
                    value={detail.supported_endpoints || []}
                    disabled={disabled}
                    onChange={(e, { value }) =>
                      setModelDetailField(
                        setValueFn,
                        row,
                        detailIndex,
                        'supported_endpoints',
                        Array.isArray(value) ? value : [],
                      )
                    }
                  />
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.price_compact'),
              key: 'price_compact',
              width: 220,
              render: (_, { detail, index: detailIndex }) => (
                <div className='router-block-gap-xs'>
                  <div className='router-muted'>
                    {t('channel.providers.model_detail_table.input_price')}
                  </div>
                  <AppInputNumber
                    className='router-inline-input'
                    step='0.000001'
                    min={0}
                    precision={6}
                    value={detail.input_price ?? ''}
                    disabled={disabled}
                    onChange={(e, { value }) =>
                      setModelDetailField(
                        setValueFn,
                        row,
                        detailIndex,
                        'input_price',
                        value ?? '',
                      )
                    }
                  />
                  <div className='router-muted router-block-top-xs'>
                    {t('channel.providers.model_detail_table.output_price')}
                  </div>
                  <AppInputNumber
                    className='router-inline-input'
                    step='0.000001'
                    min={0}
                    precision={6}
                    value={detail.output_price ?? ''}
                    disabled={disabled}
                    onChange={(e, { value }) =>
                      setModelDetailField(
                        setValueFn,
                        row,
                        detailIndex,
                        'output_price',
                        value ?? '',
                      )
                    }
                  />
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.price_unit'),
              key: 'price_unit',
              width: 160,
              render: (_, { detail, index: detailIndex }) => (
                <AppInput
                  className='router-inline-input'
                  value={detail.price_unit ?? ''}
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
              ),
            },
            {
              title: t('channel.providers.model_detail_table.currency'),
              key: 'currency',
              width: 120,
              render: (_, { detail, index: detailIndex }) => (
                <AppInput
                  className='router-inline-input'
                  value={detail.currency ?? ''}
                  disabled={disabled}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setValueFn,
                      row,
                      detailIndex,
                      'currency',
                      value ?? '',
                    )
                  }
                />
              ),
            },
            {
              title: t('channel.providers.model_detail_table.price_components'),
              key: 'price_components',
              width: 180,
              render: (_, { index: detailIndex }) => (
                <AppButton
                  type='button'
                  className='router-inline-button'
                  disabled={disabled}
                  onClick={() => addPriceComponentRow(setValueFn, row, detailIndex)}
                >
                  {t(
                    'channel.providers.model_detail_table.add_price_component',
                  )}
                </AppButton>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.actions'),
              key: 'actions',
              width: 120,
              render: (_, { index: detailIndex }) => (
                <AppButton
                  type='button'
                  className='router-inline-button'
                  color='red'
                  disabled={disabled}
                  onClick={() =>
                    removeModelDetailRow(setValueFn, row, detailIndex)
                  }
                >
                  <AppIcon name='trash' />
                </AppButton>
              ),
            },
          ]}
        />
      </div>
    );
  };

  const renderModelDetailsReadonly = (row, options = {}) => {
    const details = Array.isArray(row?.model_details) ? row.model_details : [];
    const searchable = options.searchable === true;
    const hideTitle = options.hideTitle === true;
    const showToolbar = options.showToolbar !== false;
    const actions = options.actions || {};
    const actionsDisabled = Boolean(options.actionsDisabled) || saving;
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
    const detailRows = details.map((detail, index) => ({ detail, index }));
    const visibleDetailRows =
      normalizedModelSearchKeyword === ''
        ? detailRows
        : detailRows.filter(({ detail }) => {
            const haystack = [
              detail.model || '',
              detail.description || '',
              detail.status || '',
              detail.type || '',
              (detail.supported_endpoints || []).join(','),
              detail.price_unit || '',
              detail.currency || '',
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
        {showToolbar ? (
          <AppFilterHeader
            className='router-block-gap-xs'
            title={hideTitle ? '' : t('channel.providers.dialog.model_details')}
            actions={
              searchable ? (
                <AppInput
                  className='router-inline-input router-search-form-xs'
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
              ) : null
            }
            />
        ) : null}
        <AppTable
          className='router-detail-table'
          size='small'
          pagination={false}
          rowKey={(record) =>
            `${record?.detail?.model || 'model'}-${record?.index ?? '0'}`
          }
          dataSource={pageRows}
          locale={{
            emptyText: t('channel.providers.model_detail_table.empty'),
          }}
          scroll={{ x: 1280 }}
          columns={[
            {
              title: t('channel.providers.model_detail_table.model'),
              dataIndex: ['detail', 'model'],
              key: 'model',
              width: 150,
              render: (value) => (
                <div className='router-model-title router-cell-min-130'>
                  {value || '-'}
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.description'),
              dataIndex: ['detail', 'description'],
              key: 'description',
              width: 240,
              render: (value) =>
                value ? (
                  <div className='router-model-description router-cell-min-220'>
                    {value}
                  </div>
                ) : (
                  '-'
                ),
            },
            {
              title: t('channel.providers.model_detail_table.status'),
              dataIndex: ['detail', 'status'],
              key: 'status',
              width: 96,
              render: (value) => (
                <div className='router-cell-min-90'>{value || 'active'}</div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.type'),
              dataIndex: ['detail', 'type'],
              key: 'type',
              width: 88,
              render: (value) => (
                <div className='router-cell-min-80'>{value || 'text'}</div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.supported_endpoints'),
              dataIndex: ['detail', 'supported_endpoints'],
              key: 'supported_endpoints',
              width: 320,
              render: (endpoints, record) =>
                Array.isArray(endpoints) && endpoints.length > 0 ? (
                  <div className='router-cell-min-300'>
                    {endpoints.map((endpoint) => (
                      <AppTag
                        key={`${record.detail?.model || 'model'}-${endpoint}`}
                        className='router-tag router-provider-endpoint-tag'
                      >
                        {endpoint}
                      </AppTag>
                    ))}
                  </div>
                ) : (
                  '-'
                ),
            },
            {
              title: t('channel.providers.model_detail_table.price_compact'),
              key: 'price_compact',
              render: (_, { detail }) => {
                const showInputDetail = hasComplexInputPricing(detail);
                const showOutputDetail = hasComplexOutputPricing(detail);
                const inputPriceText = formatProviderPriceCellValue(
                  detail.input_price,
                );
                const outputPriceText = formatProviderPriceCellValue(
                  detail.output_price,
                );
                const compactPriceText =
                  inputPriceText === '-' && outputPriceText === '-'
                    ? '-'
                    : `${inputPriceText}｜${outputPriceText}`;
                return showInputDetail || showOutputDetail ? (
                  <AppButton
                    type='button'
                    basic
                    className='router-inline-button'
                    onClick={() => openPricingDetail(detail)}
                  >
                    {t('channel.providers.model_detail_table.detail')}
                  </AppButton>
                ) : (
                  compactPriceText
                );
              },
            },
            {
              title: t('channel.providers.model_detail_table.price_unit'),
              key: 'price_unit',
              width: 132,
              render: (_, { detail }) => (
                <div className='router-cell-min-120'>
                  {summarizeModelPriceUnit(detail, t)}
                </div>
              ),
            },
            {
              title: t('channel.providers.model_detail_table.currency'),
              dataIndex: ['detail', 'currency'],
              key: 'currency',
              width: 96,
              render: (value) => value || 'USD',
            },
            {
              title: t('channel.providers.model_detail_table.actions'),
              key: 'actions',
              width: 160,
              render: (_, { index: detailIndex }) => (
                <div className='router-nowrap'>
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    disabled={actionsDisabled}
                    onClick={() =>
                      typeof actions.onStartEdit === 'function'
                        ? actions.onStartEdit(detailIndex)
                        : null
                    }
                  >
                    {t('common.edit')}
                  </AppButton>
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    disabled={actionsDisabled}
                    onClick={() =>
                      typeof actions.onDelete === 'function'
                        ? actions.onDelete(detailIndex)
                        : null
                    }
                  >
                    {t('common.delete')}
                  </AppButton>
                </div>
              ),
            },
          ]}
        />
        {totalPages > 1 ? (
          <div className='router-pagination-wrap'>
            <AppPagination
              className='router-section-pagination'
              current={safeCurrentPage}
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

  const renderModelDetailEditorModal = () => {
    const details = Array.isArray(detailModelsDraft.model_details)
      ? detailModelsDraft.model_details
      : [];
    if (
      !modelDetailEditorOpen ||
      detailEditingModelIndex < 0 ||
      detailEditingModelIndex >= details.length
    ) {
      return null;
    }
    const detail = details[detailEditingModelIndex];
    return (
      <AppModal
        size='large'
        open={modelDetailEditorOpen}
        onClose={closeModelDetailEditor}
        closeOnDimmerClick={!saving}
        title={
          modelDetailEditorMode === 'create'
            ? t('channel.providers.model_detail_table.create_title')
            : t('channel.providers.model_detail_table.edit_title')
        }
        footer={null}
      >
        <div className='router-modal-scroll-body router-page-stack'>
          <div>
            <AppFormRow>
              <AppField
                label={t('channel.providers.model_detail_table.model')}
                required
              >
                <AppInput
                  className='router-section-input'
                  value={detail.model || ''}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'model',
                      value || '',
                    )
                  }
                />
              </AppField>
              <AppField
                label={t('channel.providers.model_detail_table.status')}
                required
              >
                <AppSelect
                  className='router-section-dropdown'
                  options={PROVIDER_MODEL_STATUS_OPTIONS}
                  value={detail.status || 'active'}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'status',
                      value || 'active',
                    )
                  }
                />
              </AppField>
              <AppField
                label={t('channel.providers.model_detail_table.type')}
                required
              >
                <AppSelect
                  className='router-section-dropdown'
                  options={MODEL_TYPE_OPTIONS}
                  value={detail.type || 'text'}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'type',
                      value || 'text',
                    )
                  }
                />
              </AppField>
              <AppField
                label={t('channel.providers.model_detail_table.source')}
                required
              >
                <AppSelect
                  className='router-section-dropdown'
                  options={SOURCE_OPTIONS}
                  value={detail.source || 'manual'}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'source',
                      value || 'manual',
                    )
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField
                label={t(
                  'channel.providers.model_detail_table.supported_endpoints',
                )}
              >
                <AppSelect
                  className='router-section-dropdown'
                  multiple
                  clearable
                  options={providerEndpointOptionsForType(detail.type)}
                  value={detail.supported_endpoints || []}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'supported_endpoints',
                      Array.isArray(value) ? value : [],
                    )
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField
                label={t('channel.providers.model_detail_table.description')}
              >
                <AppTextarea
                  className='router-section-input'
                  value={detail.description || ''}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'description',
                      value || '',
                    )
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('channel.providers.model_detail_table.input_price')}>
                <AppInputNumber
                  className='router-section-input'
                  min={0}
                  step={0.000001}
                  precision={6}
                  fluid
                  value={detail.input_price ?? ''}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'input_price',
                      value ?? '',
                    )
                  }
                />
              </AppField>
              <AppField label={t('channel.providers.model_detail_table.output_price')}>
                <AppInputNumber
                  className='router-section-input'
                  min={0}
                  step={0.000001}
                  precision={6}
                  fluid
                  value={detail.output_price ?? ''}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'output_price',
                      value ?? '',
                    )
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('channel.providers.model_detail_table.price_unit')}>
                <AppInput
                  className='router-section-input'
                  value={detail.price_unit ?? ''}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'price_unit',
                      value || '',
                    )
                  }
                />
              </AppField>
              <AppField label={t('channel.providers.model_detail_table.currency')}>
                <AppInput
                  className='router-section-input'
                  value={detail.currency ?? ''}
                  onChange={(e, { value }) =>
                    setModelDetailField(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                      'currency',
                      value ?? '',
                    )
                  }
                />
              </AppField>
            </AppFormRow>
          </div>
          <div className='router-block-top-md'>
            <AppFilterHeader
              className='router-toolbar-compact'
              title={t('channel.providers.model_detail_table.price_components')}
              actions={
                <AppButton
                  type='button'
                  className='router-inline-button'
                  disabled={saving}
                  onClick={() =>
                    addPriceComponentRow(
                      setDetailModelsValue,
                      detailModelsDraft,
                      detailEditingModelIndex,
                    )
                  }
                >
                  {t(
                    'channel.providers.model_detail_table.add_price_component',
                  )}
                </AppButton>
              }
            />
            <AppTable
              className='router-detail-subtable'
              size='small'
              pagination={false}
              rowKey={(component, componentIndex) =>
                `${detail.model || 'model'}-${component.component || 'component'}-${component.condition || 'condition'}-${componentIndex}`
              }
              dataSource={detail.price_components || []}
              locale={{
                emptyText: t('channel.providers.price_component_table.empty'),
              }}
              scroll={{ x: 1320 }}
              columns={[
                {
                  title: t('channel.providers.price_component_table.component'),
                  key: 'component',
                  width: 180,
                  render: (_, component, componentIndex) => (
                    <AppSelect
                      className='router-inline-dropdown'
                      options={PRICE_COMPONENT_OPTIONS}
                      value={component.component || 'text'}
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'component',
                          value || '',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.condition'),
                  key: 'condition',
                  width: 320,
                  render: (_, component, componentIndex) => (
                    <div>
                      <div className='router-provider-inline-field'>
                        <AppInput
                          className='router-inline-input'
                          placeholder='quality=hd;size=1024x1024'
                          value={component.condition || ''}
                          disabled={saving}
                          onChange={(e, { value }) =>
                            setPriceComponentField(
                              setDetailModelsValue,
                              detailModelsDraft,
                              detailEditingModelIndex,
                              componentIndex,
                              'condition',
                              value || '',
                            )
                          }
                        />
                        <AppButton
                          type='button'
                          className='router-inline-button'
                          basic
                          disabled={saving}
                          onClick={() =>
                            setPriceComponentField(
                              setDetailModelsValue,
                              detailModelsDraft,
                              detailEditingModelIndex,
                              componentIndex,
                              'condition',
                              '',
                            )
                          }
                        >
                          {t('common.clear')}
                        </AppButton>
                      </div>
                      {renderPriceComponentConditionTemplate(
                        setDetailModelsValue,
                        detailModelsDraft,
                        detailEditingModelIndex,
                        componentIndex,
                        component,
                        saving,
                      )}
                    </div>
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.input_price'),
                  key: 'input_price',
                  width: 180,
                  render: (_, component, componentIndex) => (
                    <AppInputNumber
                      className='router-inline-input'
                      step='0.000001'
                      min={0}
                      precision={6}
                      value={component.input_price ?? ''}
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'input_price',
                          value ?? '',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.output_price'),
                  key: 'output_price',
                  width: 180,
                  render: (_, component, componentIndex) => (
                    <AppInputNumber
                      className='router-inline-input'
                      step='0.000001'
                      min={0}
                      precision={6}
                      value={component.output_price ?? ''}
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'output_price',
                          value ?? '',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.price_unit'),
                  key: 'price_unit',
                  width: 180,
                  render: (_, component, componentIndex) => (
                    <AppSelect
                      className='router-inline-dropdown'
                      options={PRICE_UNIT_OPTIONS}
                      value={
                        component.price_unit ??
                        defaultPriceUnitByComponent(component.component)
                      }
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'price_unit',
                          value || '',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.currency'),
                  key: 'currency',
                  width: 120,
                  render: (_, component, componentIndex) => (
                    <AppInput
                      className='router-inline-input'
                      value={component.currency ?? ''}
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'currency',
                          value ?? '',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.source'),
                  key: 'source',
                  width: 160,
                  render: (_, component, componentIndex) => (
                    <AppSelect
                      className='router-inline-dropdown'
                      options={SOURCE_OPTIONS}
                      value={component.source || 'manual'}
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'source',
                          value || 'manual',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.source_url'),
                  key: 'source_url',
                  width: 220,
                  render: (_, component, componentIndex) => (
                    <AppInput
                      className='router-inline-input'
                      value={component.source_url || ''}
                      disabled={saving}
                      onChange={(e, { value }) =>
                        setPriceComponentField(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                          'source_url',
                          value || '',
                        )
                      }
                    />
                  ),
                },
                {
                  title: t('channel.providers.price_component_table.actions'),
                  key: 'actions',
                  width: 120,
                  render: (_, component, componentIndex) => (
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      disabled={saving}
                      onClick={() =>
                        removePriceComponentRow(
                          setDetailModelsValue,
                          detailModelsDraft,
                          detailEditingModelIndex,
                          componentIndex,
                        )
                      }
                    >
                      {t('common.delete')}
                    </AppButton>
                  ),
                },
              ]}
            />
          </div>
          <AppFormActions>
            <AppButton
              type='button'
              className='router-page-button'
              onClick={closeModelDetailEditor}
              disabled={saving}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              loading={saving}
              disabled={saving}
              onClick={saveDetailModelEdit}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>
    );
  };

  const renderRows = () => (
    <div>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'resource', label: t('header.resource') },
          { key: 'providers', label: t('header.providers'), active: true },
        ]}
        title={t('header.providers')}
        actions={
          <div className='router-list-toolbar-actions'>
          <AppButton
            type='button'
            className='router-page-button'
            color='blue'
            disabled={saving}
            onClick={openCreatePanel}
          >
            {t('channel.providers.buttons.add_provider')}
          </AppButton>
          <AppButton
            type='button'
            className='router-page-button'
            disabled={saving || refreshing}
            loading={refreshing}
            onClick={reloadCurrentPage}
          >
            {t('channel.providers.buttons.refresh')}
          </AppButton>
          </div>
        }
        query={
          <AppInput
            className='router-section-input router-search-form-sm'
            placeholder={t('channel.providers.search')}
            value={searchKeyword}
            onChange={(e, { value }) => {
              setSearchKeyword(value || '');
              setActivePage(1);
            }}
          />
        }
      />
      <AppTable
        className='router-hover-table router-list-table'
        size='small'
        pagination={false}
        rowKey={(row) =>
          row?.id ||
          `${row?.name || 'provider'}-${row?.created_at || 0}-${row?.updated_at || 0}`
        }
        dataSource={rows}
        locale={{
          emptyText: (
            <AppEmpty>
              {loading ? t('common.loading') : t('channel.providers.table.empty')}
            </AppEmpty>
          ),
        }}
        onRow={(row) =>
          creating || saving
            ? {}
            : {
                onClick: () => {
                  openViewer(row);
                },
              }
        }
        rowClassName={() =>
          creating || saving ? '' : 'router-row-clickable'
        }
        columns={[
          {
            title: t('channel.providers.table.provider'),
            dataIndex: 'id',
            key: 'id',
            width: '22%',
            render: (value) => value || '-',
          },
          {
            title: t('channel.providers.table.name'),
            key: 'name',
            width: '28%',
            render: (_, row) => row.name || row.id || '-',
          },
          {
            title: t('channel.providers.table.created_at'),
            dataIndex: 'created_at',
            key: 'created_at',
            width: '20%',
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: t('channel.providers.table.updated_at'),
            dataIndex: 'updated_at',
            key: 'updated_at',
            width: '20%',
            render: (value) => (value ? timestamp2string(value) : '-'),
          },
          {
            title: t('channel.providers.table.actions'),
            key: 'actions',
            width: '10%',
            render: (_, row) => (
              <div className='router-action-group-tight'>
                <AppButton
                  type='button'
                  className='router-inline-button'
                  color='blue'
                  disabled={creating || saving}
                  onClick={(e) => {
                    e.stopPropagation();
                    openViewer(row);
                    startDetailSectionEdit('basic', row);
                  }}
                >
                  <AppIcon name='edit' />
                </AppButton>
              </div>
            ),
          },
        ]}
      />
      {totalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <AppPagination
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

  const renderViewer = () => {
    if (!viewRow) return null;
    const basicEditing = detailEditingSection === 'basic';
    const modelsEditing = detailEditingSection === 'models';
    const basicSourceRow = basicEditing ? detailBasicDraft : viewRow;
    const providerDetailTabItems = [
      {
        key: 'basic',
        label: t('channel.providers.dialog.detail_basic_title'),
        disabled: modelsEditing,
      },
      {
        key: 'models',
        label: t('channel.providers.dialog.model_details'),
        disabled: basicEditing,
      },
    ];
    return (
      <>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'admin', label: t('header.admin_workspace') },
            { key: 'resource', label: t('header.resource') },
            {
              key: 'provider-list',
              label: t('header.providers'),
              onClick: closeViewer,
            },
            {
              key: 'provider-current',
              label: viewRow.name || viewRow.id || '-',
              active: true,
            },
          ]}
          title={t('header.providers')}
        />
        <div className='router-tab-detail-page router-provider-detail-page'>
          <div className='router-entity-detail-tabs router-block-gap-sm'>
            <AppTabs
              className='router-detail-tab-menu'
              activeKey={activeProviderDetailTab}
              items={providerDetailTabItems}
              onChange={setActiveProviderDetailTab}
            />
          </div>
          {activeProviderDetailTab === 'basic' ? (
          <AppDetailSection
            className='router-provider-detail-section'
            title={t('channel.providers.dialog.detail_basic_title')}
            titleClassName='router-provider-detail-section-title'
            headerEnd={
              basicEditing ? (
                <>
                  <AppButton
                    type='button'
                    className='router-page-button'
                    onClick={cancelDetailSectionEdit}
                    disabled={saving}
                  >
                    {t('common.cancel')}
                  </AppButton>
                  <AppButton
                    type='button'
                    className='router-page-button'
                    color='blue'
                    loading={saving}
                    disabled={saving}
                    onClick={() => saveViewerSection('basic')}
                  >
                    {t('common.save')}
                  </AppButton>
                </>
              ) : (
                <AppButton
                  type='button'
                  className='router-page-button'
                  disabled={saving || modelsEditing}
                  onClick={() => startDetailSectionEdit('basic')}
                >
                  {t('common.edit')}
                </AppButton>
              )
            }
          >
              <AppFormRow>
                <AppField
                  label={t('channel.providers.dialog.provider')}
                  readOnly
                >
                  <AppInput
                    className='router-section-input'
                    value={basicSourceRow.id || ''}
                    readOnly
                  />
                </AppField>
                {basicEditing ? (
                  <AppField
                    label={t('channel.providers.dialog.name')}
                    required
                  >
                    <AppInput
                      className='router-section-input'
                      placeholder={t(
                        'channel.providers.dialog.name_placeholder',
                      )}
                      value={detailBasicDraft.name}
                      onChange={(e, { value }) =>
                        setDetailBasicValue('name', value || '')
                      }
                    />
                  </AppField>
                ) : (
                  <AppField
                    label={t('channel.providers.dialog.name')}
                    readOnly
                  >
                    <AppInput
                      className='router-section-input'
                      value={basicSourceRow.name || ''}
                      readOnly
                    />
                  </AppField>
                )}
              </AppFormRow>
              <AppFormRow>
                {basicEditing ? (
                  <AppField
                    label={t('channel.providers.dialog.base_url')}
                    required
                  >
                    <AppInput
                      className='router-section-input'
                      placeholder={t(
                        'channel.providers.dialog.base_url_placeholder',
                      )}
                      value={detailBasicDraft.base_url}
                      onChange={(e, { value }) =>
                        setDetailBasicValue('base_url', value || '')
                      }
                    />
                  </AppField>
                ) : (
                  <AppField
                    label={t('channel.providers.dialog.base_url')}
                    readOnly
                  >
                    <AppInput
                      className='router-section-input'
                      value={basicSourceRow.base_url || ''}
                      readOnly
                    />
                  </AppField>
                )}
                {basicEditing ? (
                  <AppField label={t('channel.providers.dialog.official_url')}>
                    <AppInput
                      className='router-section-input'
                      placeholder={t(
                        'channel.providers.dialog.official_url_placeholder',
                      )}
                      value={detailBasicDraft.official_url}
                      onChange={(e, { value }) =>
                        setDetailBasicValue('official_url', value || '')
                      }
                    />
                  </AppField>
                ) : (
                  <AppField
                    label={t('channel.providers.dialog.official_url')}
                    readOnly
                  >
                    <AppInput
                      className='router-section-input'
                      value={basicSourceRow.official_url || ''}
                      readOnly
                    />
                  </AppField>
                )}
              </AppFormRow>
              <AppFormRow>
                <AppField label={t('channel.providers.table.source')} readOnly>
                  <AppInput
                    className='router-section-input'
                    value={viewRow.source || '-'}
                    readOnly
                  />
                </AppField>
                <AppField
                  label={t('channel.providers.table.created_at')}
                  readOnly
                >
                  <AppInput
                    className='router-section-input'
                    value={
                      viewRow.created_at
                        ? timestamp2string(viewRow.created_at)
                        : '-'
                    }
                    readOnly
                  />
                </AppField>
                <AppField
                  label={t('channel.providers.table.updated_at')}
                  readOnly
                >
                  <AppInput
                    className='router-section-input'
                    value={
                      viewRow.updated_at
                        ? timestamp2string(viewRow.updated_at)
                        : '-'
                    }
                    readOnly
                  />
                </AppField>
              </AppFormRow>
          </AppDetailSection>
          ) : null}
          {activeProviderDetailTab === 'models' ? (
          <AppDetailSection
            className='router-provider-detail-section'
            title={t('channel.providers.dialog.model_details')}
            titleClassName='router-provider-detail-section-title'
            headerEnd={
              <>
                <AppInput
                  className='router-inline-input router-search-form-xs router-section-header-search'
                  placeholder={t(
                    'channel.providers.model_detail_table.search_placeholder',
                  )}
                  value={viewModelSearchKeyword}
                  onChange={(e, { value }) => {
                    setViewModelSearchKeyword(value || '');
                    setViewModelPage(1);
                  }}
                />
                <AppButton
                  type='button'
                  className='router-page-button'
                  disabled={saving || basicEditing || modelsEditing}
                  onClick={startDetailModelCreate}
                >
                  {t('channel.providers.model_detail_table.add')}
                </AppButton>
              </>
            }
          >
            {renderModelDetailsReadonly(viewRow, {
              searchable: false,
              hideTitle: true,
              showToolbar: false,
              searchKeyword: viewModelSearchKeyword,
              currentPage: viewModelPage,
              pageSize: PROVIDER_DETAIL_MODEL_PAGE_SIZE,
              onPageChange: setViewModelPage,
              actions: {
                onStartEdit: startDetailModelEdit,
                onDelete: deleteDetailModel,
              },
              actionsDisabled: basicEditing || modelsEditing,
            })}
          </AppDetailSection>
          ) : null}
        </div>
      </>
    );
  };

  const renderCreatePanel = () => (
    <div>
      <AppFormActions align='start' className='router-block-gap-sm'>
        <AppButton
          type='button'
          className='router-page-button'
          onClick={closeCreatePanel}
          disabled={saving}
        >
          {t('channel.providers.dialog.cancel_create')}
        </AppButton>
        <AppButton
          type='button'
          className='router-page-button'
          color='blue'
          loading={saving}
          disabled={saving}
          onClick={applyCreateToRows}
        >
          {t('channel.providers.dialog.confirm')}
        </AppButton>
      </AppFormActions>
      <div>
        <AppFormRow>
          <AppField
            label={t('channel.providers.dialog.provider')}
            required
          >
            <AppInput
              className='router-section-input'
              placeholder={t('channel.providers.dialog.provider_placeholder')}
              value={createRow.id}
              onChange={(e, { value }) =>
                setCreateValue('id', normalizeProvider(value || ''))
              }
            />
          </AppField>
          <AppField label={t('channel.providers.dialog.name')} required>
            <AppInput
              className='router-section-input'
              placeholder={t('channel.providers.dialog.name_placeholder')}
              value={createRow.name}
              onChange={(e, { value }) => setCreateValue('name', value || '')}
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField label={t('channel.providers.dialog.base_url')} required>
            <AppInput
              className='router-section-input'
              placeholder={t('channel.providers.dialog.base_url_placeholder')}
              value={createRow.base_url}
              onChange={(e, { value }) =>
                setCreateValue('base_url', value || '')
              }
            />
          </AppField>
          <AppField label={t('channel.providers.dialog.official_url')}>
            <AppInput
              className='router-section-input'
              placeholder={t(
                'channel.providers.dialog.official_url_placeholder',
              )}
              value={createRow.official_url}
              onChange={(e, { value }) =>
                setCreateValue('official_url', value || '')
              }
            />
          </AppField>
        </AppFormRow>
      </div>

      {renderModelDetailsTable(createRow, setCreateValue, saving)}
    </div>
  );

  const renderDeleteModal = () => {
    const providerName = deletingRow?.name || deletingRow?.id || '-';
    return (
      <AppModal
        open={!!deletingRow}
        onClose={closeDeleteModal}
        size='tiny'
        closeOnDimmerClick={!saving}
        title={t('channel.providers.dialog.delete_title')}
        footer={[
          <AppButton
            key='cancel'
            type='button'
            className='router-modal-button'
            onClick={closeDeleteModal}
            disabled={saving}
          >
            {t('channel.providers.dialog.cancel_create')}
          </AppButton>,
          <AppButton
            key='confirm'
            type='button'
            className='router-modal-button'
            color='red'
            loading={saving}
            disabled={saving}
            onClick={confirmDeleteRow}
          >
            {t('channel.providers.dialog.delete_confirm')}
          </AppButton>,
        ]}
      >
        <div>
          {t('channel.providers.dialog.delete_content', {
            provider: providerName,
          })}
        </div>
      </AppModal>
    );
  };

  const renderPricingDetailModal = () => (
    <AppModal
      size='large'
      open={pricingDetailOpen}
      onClose={closePricingDetail}
      title={t('channel.providers.dialog.pricing_detail_title')}
      footer={[
        <AppButton
          key='close'
          type='button'
          className='router-modal-button'
          onClick={closePricingDetail}
        >
          {t('channel.providers.dialog.cancel')}
        </AppButton>,
      ]}
    >
      <div className='router-modal-scroll-body'>
        <div className='router-block-gap-sm'>
          <AppToolbar
            className='router-block-gap-sm'
            start={
              <>
                <AppTag className='router-tag'>
                  {pricingDetailModel?.model || '-'}
                </AppTag>
                <AppTag className='router-tag'>
                  {pricingDetailModel?.type || 'text'}
                </AppTag>
              </>
            }
          />
        </div>
        <AppTable
          className='router-detail-subtable'
          size='small'
          pagination={false}
          rowKey={() => `${pricingDetailModel?.model || 'model'}-summary`}
          dataSource={[pricingDetailModel || {}]}
          columns={[
            {
              title: t('channel.providers.model_detail_table.input_price'),
              key: 'input_price',
              render: (_, record) =>
                isComponentBasedPricing(record)
                  ? t('channel.providers.model_detail_table.component_based')
                  : formatProviderPriceCellValue(record?.input_price),
            },
            {
              title: t('channel.providers.model_detail_table.output_price'),
              key: 'output_price',
              render: (_, record) =>
                isComponentBasedPricing(record)
                  ? t('channel.providers.model_detail_table.component_based')
                  : formatProviderPriceCellValue(record?.output_price),
            },
            {
              title: t('channel.providers.model_detail_table.price_unit'),
              key: 'price_unit',
              render: (_, record) => summarizeModelPriceUnit(record, t),
            },
            {
              title: t('channel.providers.model_detail_table.currency'),
              dataIndex: 'currency',
              key: 'currency',
              render: (value) => value || 'USD',
            },
          ]}
        />
        <AppTable
          className='router-detail-subtable'
          size='small'
          pagination={false}
          rowKey={(component, componentIndex) =>
            `${pricingDetailModel?.model || 'model'}-${component.component || 'component'}-${component.condition || 'condition'}-${componentIndex}`
          }
          dataSource={pricingDetailModel?.price_components || []}
          locale={{
            emptyText: t('channel.providers.price_component_table.empty'),
          }}
          scroll={{ x: 1080 }}
          columns={[
            {
              title: t('channel.providers.price_component_table.component'),
              dataIndex: 'component',
              key: 'component',
              render: (value) => value || '-',
            },
            {
              title: t('channel.providers.price_component_table.condition'),
              dataIndex: 'condition',
              key: 'condition',
              render: (value) => value || '-',
            },
            {
              title: t('channel.providers.price_component_table.input_price'),
              dataIndex: 'input_price',
              key: 'input_price',
              render: (value) => formatProviderPriceCellValue(value),
            },
            {
              title: t('channel.providers.price_component_table.output_price'),
              dataIndex: 'output_price',
              key: 'output_price',
              render: (value) => formatProviderPriceCellValue(value),
            },
            {
              title: t('channel.providers.price_component_table.price_unit'),
              dataIndex: 'price_unit',
              key: 'price_unit',
              render: (value) => value || '-',
            },
            {
              title: t('channel.providers.price_component_table.currency'),
              dataIndex: 'currency',
              key: 'currency',
              render: (value) => value || 'USD',
            },
            {
              title: t('channel.providers.price_component_table.source'),
              dataIndex: 'source',
              key: 'source',
              render: (value) => value || 'manual',
            },
            {
              title: t('channel.providers.price_component_table.source_url'),
              dataIndex: 'source_url',
              key: 'source_url',
              render: (value) => value || '-',
            },
          ]}
        />
      </div>
    </AppModal>
  );

  return (
    <div>
      {renderDeleteModal()}
      {renderModelDetailEditorModal()}
      {renderPricingDetailModal()}
      {creating
        ? renderCreatePanel()
        : viewingProvider && viewRow
          ? renderViewer()
          : renderRows()}
    </div>
  );
};

export default ProvidersManager;
