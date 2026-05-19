import React, {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import {
  getChannelProtocolOptions,
  loadChannelProtocolOptions,
} from '../../helpers/helper';
import ChannelDetailEndpointsTab from './components/ChannelDetailEndpointsTab';
import ChannelDetailModelsTab from './components/ChannelDetailModelsTab';
import ChannelDetailOverviewTab from './components/ChannelDetailOverviewTab';
import ChannelDetailTestsTab from './components/ChannelDetailTestsTab';
import ChannelAppendProviderModal from './components/ChannelAppendProviderModal';
import ChannelComplexPricingModal from './components/ChannelComplexPricingModal';
import ChannelModelEditorModal from './components/ChannelModelEditorModal';
import ChannelEndpointPolicyEditorModal from './components/ChannelEndpointPolicyEditorModal';
import {
  AppAlert,
  AppButton,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppIcon,
  AppInput,
  AppModal,
  AppSelect,
  AppSpin,
  AppTabs,
} from '../../router-ui';
import { CHANNEL_DETAIL_MODEL_COLUMN_WIDTHS } from '../../constants/tableWidthPresets';

const normalizeModelId = (model) => {
  if (typeof model === 'string') return model;
  if (model && typeof model === 'object') {
    if (typeof model.id === 'string') return model.id;
    if (typeof model.name === 'string') return model.name;
    if (typeof model.model === 'string') return model.model;
  }
  return null;
};

const buildModelIDs = (models) => {
  const seen = new Set();
  const ids = [];
  models.forEach((model) => {
    const id = normalizeModelId(model);
    if (!id || seen.has(id)) return;
    seen.add(id);
    ids.push(id);
  });
  return ids;
};

const normalizeModelIDs = (models) => {
  if (!Array.isArray(models)) {
    return [];
  }
  const seen = new Set();
  const result = [];
  models.forEach((item) => {
    const id = (item || '').toString().trim();
    if (!id || seen.has(id)) return;
    seen.add(id);
    result.push(id);
  });
  result.sort();
  return result;
};

const normalizeBaseURL = (baseURL) =>
  (baseURL || '').trim().replace(/\/+$/, '');

const resolveEffectiveAPIBaseURL = (inputs, config) =>
  normalizeBaseURL(config?.api_base_url || inputs?.base_url || '');

const CHANNEL_IDENTIFIER_PATTERN = /^[a-z0-9-]+$/;
const CHANNEL_IDENTIFIER_MAX_LENGTH = 64;
const CHANNEL_ENDPOINT_COLUMN_WIDTHS = [
  '14%',
  '14%',
  '18%',
  '8%',
  '20%',
  '14%',
  '12%',
  '8%',
];
const CHANNEL_MODEL_TEST_GROUP_COLUMN_WIDTHS = [
  '4%',
  '15%',
  '23%',
  '8%',
  '16%',
  '12%',
  '18%',
];

const supportsModelTestStream = (row) =>
  normalizeChannelModelType(row?.type) === 'text';

const resolveModelTestStreamEnabled = (row) => {
  if (!supportsModelTestStream(row)) {
    return false;
  }
  if (
    Object.prototype.hasOwnProperty.call(row || {}, 'is_stream') ||
    Object.prototype.hasOwnProperty.call(row || {}, 'isStream')
  ) {
    return row?.is_stream === true || row?.isStream === true;
  }
  return true;
};
const normalizeChannelIdentifier = (value) =>
  (value || '').toString().trim().toLowerCase();

const validateChannelIdentifier = (value, t) => {
  const normalized = normalizeChannelIdentifier(value);
  if (normalized === '') {
    return t('channel.edit.messages.identifier_required');
  }
  if (
    normalized.length > CHANNEL_IDENTIFIER_MAX_LENGTH ||
    !CHANNEL_IDENTIFIER_PATTERN.test(normalized)
  ) {
    return t('channel.edit.messages.identifier_invalid');
  }
  return '';
};

const parseJSONObject = (value) => {
  if (typeof value !== 'string') {
    return {};
  }
  const trimmed = value.trim();
  if (trimmed === '') {
    return {};
  }
  try {
    const parsed = JSON.parse(trimmed);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {};
    }
    return parsed;
  } catch {
    return {};
  }
};

const normalizePriceOverrideValue = (value) => {
  if (value === null || value === undefined || value === '') {
    return null;
  }
  const price = Number(value);
  if (!Number.isFinite(price) || price < 0) {
    return null;
  }
  return price;
};

const normalizePriceUnitValue = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  return normalized || 'per_1k_tokens';
};

const normalizeCurrencyValue = (value) => {
  const normalized = (value || '').toString().trim().toUpperCase();
  return normalized || 'USD';
};

const normalizeChannelModelType = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'image':
    case 'audio':
    case 'video':
    case 'embedding':
      return normalized;
    default:
      return 'text';
  }
};

const normalizeChannelProtocol = (value) =>
  (value || '').toString().trim().toLowerCase();

const defaultChannelModelEndpoint = (type, protocol) => {
  switch (normalizeChannelModelType(type)) {
    case 'image':
      return '/v1/images/generations';
    case 'audio':
      return '/v1/audio/speech';
    case 'video':
      return '/v1/videos';
    case 'embedding':
      return '/v1/embeddings';
    default:
      return normalizeChannelProtocol(protocol) === 'anthropic'
        ? '/v1/messages'
        : '/v1/responses';
  }
};

const normalizeChannelModelEndpoint = (type, value, protocol) => {
  const normalizedType = normalizeChannelModelType(type);
  const normalized = (value || '').toString().trim().toLowerCase();
  if (normalizedType === 'image') {
    switch (normalized) {
      case '/v1/responses':
        return '/v1/responses';
      case '/v1/batches':
        return '/v1/batches';
      case '/v1/images/edits':
        return '/v1/images/edits';
      default:
        return '/v1/images/generations';
    }
  }
  if (normalizedType === 'text') {
    switch (normalized) {
      case '/v1/chat/completions':
        return '/v1/chat/completions';
      case '/v1/messages':
        return '/v1/messages';
      case '/v1/responses':
        return '/v1/responses';
      default:
        return defaultChannelModelEndpoint(normalizedType, protocol);
    }
  }
  if (normalizedType === 'audio') {
    switch (normalized) {
      case '/v1/realtime':
        return '/v1/realtime';
      case '/v1/audio/speech':
        return '/v1/audio/speech';
      default:
        return defaultChannelModelEndpoint(normalizedType, protocol);
    }
  }
  if (normalizedType === 'embedding') {
    switch (normalized) {
      case '/v1/embeddings':
        return '/v1/embeddings';
      default:
        return defaultChannelModelEndpoint(normalizedType, protocol);
    }
  }
  return defaultChannelModelEndpoint(normalizedType, protocol);
};

const normalizeChannelModelEndpoints = (type, endpoints, endpoint, protocol) => {
  const candidates = [];
  if (Array.isArray(endpoints)) {
    endpoints.forEach((item) => {
      candidates.push(item);
    });
  }
  if ((endpoint || '').toString().trim() !== '') {
    candidates.push(endpoint);
  }
  if (candidates.length === 0) {
    candidates.push(defaultChannelModelEndpoint(type, protocol));
  }
  const seen = new Set();
  const result = [];
  candidates.forEach((item) => {
    const normalized = normalizeChannelModelEndpoint(type, item, protocol);
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    result.push(normalized);
  });
  if (result.length === 0) {
    result.push(defaultChannelModelEndpoint(type, protocol));
  }
  return result;
};

const DETAIL_TAB_KEYS = [
  'overview',
  'models',
  'endpoints',
  'tests',
];

const normalizeDetailTab = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  return DETAIL_TAB_KEYS.includes(normalized) ? normalized : 'overview';
};

const CHANNEL_MODEL_TYPE_OPTIONS = [
  { key: 'text', value: 'text', text: 'text' },
  { key: 'image', value: 'image', text: 'image' },
  { key: 'audio', value: 'audio', text: 'audio' },
  { key: 'video', value: 'video', text: 'video' },
  { key: 'embedding', value: 'embedding', text: 'embedding' },
];

const TEXT_MODEL_ENDPOINT_OPTIONS = [
  { key: 'responses', value: '/v1/responses', text: '/v1/responses' },
  { key: 'chat', value: '/v1/chat/completions', text: '/v1/chat/completions' },
  { key: 'messages', value: '/v1/messages', text: '/v1/messages' },
];

const AUDIO_MODEL_ENDPOINT_OPTIONS = [
  { key: 'audio_speech', value: '/v1/audio/speech', text: '/v1/audio/speech' },
  { key: 'realtime', value: '/v1/realtime', text: '/v1/realtime' },
];

const EMBEDDING_MODEL_ENDPOINT_OPTIONS = [
  { key: 'embeddings', value: '/v1/embeddings', text: '/v1/embeddings' },
];

const IMAGE_MODEL_ENDPOINT_OPTIONS = [
  { key: 'responses', value: '/v1/responses', text: '/v1/responses' },
  {
    key: 'images_generations',
    value: '/v1/images/generations',
    text: '/v1/images/generations',
  },
  { key: 'images_edits', value: '/v1/images/edits', text: '/v1/images/edits' },
  { key: 'batches', value: '/v1/batches', text: '/v1/batches' },
];

const CHANNEL_ENDPOINT_SORT_ORDER = {
  '/v1/chat/completions': 10,
  '/v1/responses': 20,
  '/v1/messages': 30,
  '/v1/images/generations': 40,
  '/v1/images/edits': 50,
  '/v1/batches': 60,
  '/v1/embeddings': 65,
  '/v1/audio/speech': 70,
  '/v1/realtime': 80,
  '/v1/videos': 90,
};

const endpointOptionsForModelType = (type) => {
  const normalizedType = normalizeChannelModelType(type);
  if (normalizedType === 'image') {
    return IMAGE_MODEL_ENDPOINT_OPTIONS;
  }
  if (normalizedType === 'audio') {
    return AUDIO_MODEL_ENDPOINT_OPTIONS;
  }
  if (normalizedType === 'embedding') {
    return EMBEDDING_MODEL_ENDPOINT_OPTIONS;
  }
  if (normalizedType === 'text') {
    return TEXT_MODEL_ENDPOINT_OPTIONS;
  }
  return [];
};

const buildEndpointOptionsFromValues = (type, values, protocol) => {
  const labelByValue = new Map(
    endpointOptionsForModelType(type).map((option) => [
      normalizeChannelModelEndpoint(type, option.value, protocol),
      option.text || option.value,
    ]),
  );
  return normalizeChannelModelEndpoints(type, values, '', protocol).map(
    (endpoint) => ({
      key: endpoint,
      value: endpoint,
      text: labelByValue.get(endpoint) || endpoint,
    }),
  );
};

const buildChannelEndpointKey = (modelName, endpoint) =>
  `${(modelName || '').toString().trim()}::${(endpoint || '')
    .toString()
    .trim()}`;

const normalizeChannelEndpointRows = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  const seen = new Set();
  const rows = [];
  items.forEach((item) => {
    if (!item || typeof item !== 'object') {
      return;
    }
    const model = (item.model || '').toString().trim();
    const endpoint = (item.endpoint || '').toString().trim();
    if (model === '' || endpoint === '') {
      return;
    }
    const key = buildChannelEndpointKey(model, endpoint);
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    rows.push({
      channel_id: (item.channel_id || '').toString().trim(),
      model,
      endpoint,
      base_url: normalizeBaseURL(item.base_url),
      enabled: item.enabled === true,
      updated_at: Number(item.updated_at || 0),
      last_test_status: (item.last_test_status || '').toString().trim(),
      last_tested_at: Number(item.last_tested_at || 0),
      last_test_error: (item.last_test_error || '').toString().trim(),
      enable_block_reason: (item.enable_block_reason || '').toString().trim(),
    });
  });
  rows.sort((left, right) => {
    const modelOrder = left.model.localeCompare(right.model);
    if (modelOrder !== 0) {
      return modelOrder;
    }
    const leftOrder =
      CHANNEL_ENDPOINT_SORT_ORDER[left.endpoint] || Number.MAX_SAFE_INTEGER;
    const rightOrder =
      CHANNEL_ENDPOINT_SORT_ORDER[right.endpoint] || Number.MAX_SAFE_INTEGER;
    if (leftOrder !== rightOrder) {
      return leftOrder - rightOrder;
    }
    return left.endpoint.localeCompare(right.endpoint);
  });
  return rows;
};

const prettyJSONString = (value) => {
  const trimmed = (value || '').toString().trim();
  if (trimmed === '') {
    return '';
  }
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return trimmed;
  }
};

const normalizeChannelEndpointPolicyRows = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  const seen = new Set();
  const rows = [];
  items.forEach((item) => {
    if (!item || typeof item !== 'object') {
      return;
    }
    const model = (item.model || '').toString().trim();
    const endpoint = (item.endpoint || '').toString().trim();
    if (model === '' || endpoint === '') {
      return;
    }
    const key = buildChannelEndpointKey(model, endpoint);
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    rows.push({
      id: (item.id || '').toString().trim(),
      channel_id: (item.channel_id || '').toString().trim(),
      model,
      endpoint,
      enabled: item.enabled === true,
      template_key: (item.template_key || '').toString().trim(),
      capabilities: prettyJSONString(item.capabilities),
      request_policy: prettyJSONString(item.request_policy),
      response_policy: prettyJSONString(item.response_policy),
      reason: (item.reason || '').toString(),
      source: (item.source || '').toString().trim() || 'manual',
      last_verified_at: Number(item.last_verified_at || 0),
      updated_at: Number(item.updated_at || 0),
    });
  });
  rows.sort((left, right) => {
    const modelOrder = left.model.localeCompare(right.model);
    if (modelOrder !== 0) {
      return modelOrder;
    }
    const leftOrder =
      CHANNEL_ENDPOINT_SORT_ORDER[left.endpoint] || Number.MAX_SAFE_INTEGER;
    const rightOrder =
      CHANNEL_ENDPOINT_SORT_ORDER[right.endpoint] || Number.MAX_SAFE_INTEGER;
    if (leftOrder !== rightOrder) {
      return leftOrder - rightOrder;
    }
    return left.endpoint.localeCompare(right.endpoint);
  });
  return rows;
};

const buildEmptyEndpointPolicyDraft = (channelId, modelName, endpoint) => ({
  id: '',
  channel_id: (channelId || '').toString().trim(),
  model: (modelName || '').toString().trim(),
  endpoint: (endpoint || '').toString().trim(),
  enabled: true,
  template_key: '',
  capabilities: '',
  request_policy: '',
  response_policy: '',
  reason: '',
  source: 'manual',
  last_verified_at: 0,
  updated_at: 0,
});

const ENDPOINT_POLICY_TEMPLATES = [
  {
    key: 'ANTHROPIC_IMAGE_URL_TO_BASE64',
    value: 'ANTHROPIC_IMAGE_URL_TO_BASE64',
    text: 'ANTHROPIC_IMAGE_URL_TO_BASE64',
    buildDraft: () => ({
      template_key: 'ANTHROPIC_IMAGE_URL_TO_BASE64',
      capabilities: JSON.stringify(
        {
          input_image_url: false,
          input_image_base64: true,
        },
        null,
        2,
      ),
      request_policy: JSON.stringify(
        {
          actions: [
            {
              type: 'image_url_to_base64',
              input_types: ['anthropic.image_url'],
              reason: 'convert image url to base64 for upstream compatibility',
              limits: {
                max_bytes: 5242880,
                timeout_ms: 10000,
                allowed_content_types: [
                  'image/png',
                  'image/jpeg',
                  'image/webp',
                  'image/gif',
                ],
              },
            },
          ],
        },
        null,
        2,
      ),
      response_policy: '',
      reason: '该上游只稳定支持 base64 图片输入，需要在 Router 侧转换',
      source: 'manual',
    }),
  },
];

const CHANNEL_MODEL_PAGE_SIZE = 10;

const buildProviderOptionText = (item) => {
  const id = (item?.id || item?.provider || '').toString().trim();
  const name = (item?.name || '').toString().trim();
  if (id === '') {
    return '';
  }
  if (name !== '' && name !== id) {
    return `${name} (${id})`;
  }
  return id;
};

const normalizeSearchKeyword = (value) =>
  (value || '')
    .toString()
    .trim()
    .toLowerCase()
    .replace(/[\s/_-]+/g, '');

const filterProviderOptionsByQuery = (options, query) => {
  const normalizedQuery = normalizeSearchKeyword(query);
  if (normalizedQuery === '') {
    return options;
  }
  return (Array.isArray(options) ? options : []).filter((option) => {
    const candidates = [option?.text, option?.value, option?.key].map(
      normalizeSearchKeyword,
    );
    return candidates.some((candidate) => candidate.includes(normalizedQuery));
  });
};

const normalizeComplexPriceComponents = (components) => {
  if (!Array.isArray(components)) {
    return [];
  }
  const unique = new Map();
  components.forEach((item, index) => {
    if (!item) {
      return;
    }
    const component = (item.component || '').toString().trim().toLowerCase();
    if (component === '') {
      return;
    }
    const condition = (item.condition || '').toString().trim();
    const inputPrice = Number(item.input_price || 0);
    const outputPrice = Number(item.output_price || 0);
    const priceUnit =
      typeof item.price_unit === 'string' && item.price_unit.trim() !== ''
        ? item.price_unit.trim().toLowerCase()
        : '';
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
    });
  });
  return Array.from(unique.values()).sort((a, b) => {
    const byComponent = (a.component || '').localeCompare(b.component || '');
    if (byComponent !== 0) {
      return byComponent;
    }
    return (a.condition || '').localeCompare(b.condition || '');
  });
};

const mergePriceComponentOverrides = (baseComponents, overrideComponents) => {
  const merged = normalizeComplexPriceComponents(baseComponents);
  const indexByKey = new Map(
    merged.map((component, index) => [
      `${component.component || ''}\u0000${component.condition || ''}`,
      index,
    ]),
  );
  normalizeComplexPriceComponents(overrideComponents).forEach((component) => {
    const key = `${component.component || ''}\u0000${component.condition || ''}`;
    const nextComponent = {
      ...component,
      source: component.source || 'channel_override',
    };
    if (indexByKey.has(key)) {
      merged[indexByKey.get(key)] = nextComponent;
      return;
    }
    indexByKey.set(key, merged.length);
    merged.push(nextComponent);
  });
  return normalizeComplexPriceComponents(merged).filter(
    (component) =>
      Number(component.input_price || 0) > 0 ||
      Number(component.output_price || 0) > 0,
  );
};

const buildProviderIndex = (items) => {
  const providerOptions = [];
  const modelOwners = {};
  const providerModelDetails = {};
  const providerSeen = new Set();
  (Array.isArray(items) ? items : []).forEach((item) => {
    const providerId = (item?.id || item?.provider || '').toString().trim();
    if (providerId === '') {
      return;
    }
    if (!providerSeen.has(providerId)) {
      providerSeen.add(providerId);
      providerOptions.push({
        key: providerId,
        value: providerId,
        text: buildProviderOptionText(item),
      });
    }
    const details = Array.isArray(item?.model_details)
      ? item.model_details
      : [];
    if (!providerModelDetails[providerId]) {
      providerModelDetails[providerId] = {};
    }
    details.forEach((detail) => {
      const modelName = (detail?.model || '').toString().trim();
      if (modelName === '') {
        return;
      }
      if (!Array.isArray(modelOwners[modelName])) {
        modelOwners[modelName] = [];
      }
      if (!modelOwners[modelName].includes(providerId)) {
        modelOwners[modelName].push(providerId);
      }
      providerModelDetails[providerId][modelName] = {
        model: modelName,
        type: normalizeChannelModelType(detail?.type, modelName),
        input_price: Number(detail?.input_price || 0) || 0,
        output_price: Number(detail?.output_price || 0) || 0,
        price_unit:
          typeof detail?.price_unit === 'string'
            ? detail.price_unit.toString().trim().toLowerCase()
            : '',
        currency:
          typeof detail?.currency === 'string' &&
          detail.currency.toString().trim() !== ''
            ? detail.currency.toString().trim().toUpperCase()
            : 'USD',
        supported_endpoints: Array.isArray(detail?.supported_endpoints)
          ? detail.supported_endpoints
              .map((endpoint) => (endpoint || '').toString().trim())
              .filter(Boolean)
          : [],
        source:
          typeof detail?.source === 'string' &&
          detail.source.toString().trim() !== ''
            ? detail.source.toString().trim().toLowerCase()
            : 'manual',
        price_components: normalizeComplexPriceComponents(
          detail?.price_components,
        ),
      };
    });
  });
  providerOptions.sort((a, b) => a.value.localeCompare(b.value));
  Object.keys(modelOwners).forEach((modelName) => {
    modelOwners[modelName].sort((a, b) => a.localeCompare(b));
  });
  return { providerOptions, modelOwners, providerModelDetails };
};

const buildProviderLookupKeys = (row) => {
  const keys = new Set();
  [row?.upstream_model, row?.model].forEach((value) => {
    const normalized = (value || '').toString().trim();
    if (normalized === '') {
      return;
    }
    keys.add(normalized);
    if (normalized.includes('/')) {
      const parts = normalized.split('/');
      if (parts.length > 1) {
        const suffix = parts.slice(1).join('/').trim();
        if (suffix !== '') {
          keys.add(suffix);
        }
      }
    }
  });
  return Array.from(keys);
};

const normalizeChannelModelProviderValue = (value) =>
  (value || '').toString().trim().toLowerCase();

const inferAssignableProviderForRowWithOptions = (row, providerOptions) => {
  const candidates = buildProviderLookupKeys(row);
  const providerValues = new Set(
    (Array.isArray(providerOptions) ? providerOptions : []).map((item) =>
      normalizeProviderIdentifier(item?.value || ''),
    ),
  );
  for (const candidate of candidates) {
    const resolvedProvider = resolveProviderIdentifierFromModelName(candidate);
    if (
      resolvedProvider !== '' &&
      resolvedProvider !== 'unknown' &&
      providerValues.has(resolvedProvider)
    ) {
      return resolvedProvider;
    }
  }
  return '';
};

const normalizeProviderIdentifier = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  if (normalized === '') {
    return '';
  }
  switch (normalized) {
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
    case 'qwen':
    case 'qwq':
    case 'qvq':
    case '千问':
      return 'qwen';
    case 'zhipu':
    case 'glm':
    case '智谱':
    case 'bigmodel':
      return 'zhipu';
    case 'hunyuan':
    case 'tencent':
    case '腾讯':
    case '混元':
      return 'hunyuan';
    case 'volc':
    case 'volcengine':
    case 'doubao':
    case 'ark':
    case '火山':
    case '豆包':
    case '字节':
      return 'volcengine';
    case 'minimax':
    case 'abab':
      return 'minimax';
    case 'black-forest-labs':
    case 'blackforestlabs':
    case 'bfl':
      return 'black-forest-labs';
    default:
      return normalized;
  }
};

const resolveProviderIdentifierFromModelName = (modelName) => {
  const name = (modelName || '').toString().trim();
  if (name === '') {
    return '';
  }
  if (name.includes('/')) {
    const [prefix] = name.split('/', 1);
    const normalizedPrefix = normalizeProviderIdentifier(prefix);
    if (normalizedPrefix !== '') {
      return normalizedPrefix;
    }
  }
  const lower = name.toLowerCase();
  if (
    lower.startsWith('gpt-') ||
    lower.startsWith('o1') ||
    lower.startsWith('o3') ||
    lower.startsWith('o4') ||
    lower.startsWith('chatgpt-')
  ) {
    return 'openai';
  }
  if (lower.startsWith('claude-')) return 'anthropic';
  if (lower.startsWith('gemini-') || lower.startsWith('veo')) return 'google';
  if (lower.startsWith('grok-')) return 'xai';
  if (
    lower.startsWith('mistral-') ||
    lower.startsWith('mixtral-') ||
    lower.startsWith('pixtral-') ||
    lower.startsWith('ministral-') ||
    lower.startsWith('codestral-') ||
    lower.startsWith('open-mistral-') ||
    lower.startsWith('devstral-') ||
    lower.startsWith('magistral-')
  ) {
    return 'mistral';
  }
  if (lower.startsWith('command-r') || lower.startsWith('cohere-'))
    return 'cohere';
  if (lower.startsWith('deepseek-')) return 'deepseek';
  if (
    lower.startsWith('qwen') ||
    lower.startsWith('qwq-') ||
    lower.startsWith('qvq-')
  ) {
    return 'qwen';
  }
  if (lower.startsWith('glm-') || lower.startsWith('cogview-')) return 'zhipu';
  if (lower.startsWith('hunyuan-')) return 'hunyuan';
  if (lower.startsWith('doubao-') || lower.startsWith('ark-'))
    return 'volcengine';
  if (lower.startsWith('abab') || lower.startsWith('minimax-'))
    return 'minimax';
  if (lower.startsWith('ernie-')) return 'baidu';
  if (lower.startsWith('spark-')) return 'xunfei';
  if (lower.startsWith('moonshot-') || lower.startsWith('kimi-'))
    return 'moonshot';
  if (lower.startsWith('llama')) return 'meta';
  if (lower.startsWith('flux')) return 'black-forest-labs';
  if (lower.startsWith('baichuan-')) return 'baichuan';
  if (lower.startsWith('yi-')) return 'lingyiwanwu';
  if (lower.startsWith('step-')) return 'stepfun';
  if (lower.startsWith('ollama-')) return 'ollama';
  return '';
};

const normalizeChannelModelConfigRow = (row, protocol) => {
  if (!row || typeof row !== 'object') {
    return null;
  }
  const upstreamModel = (
    row.upstream_model ||
    row.upstreamModel ||
    row.name ||
    row.model ||
    ''
  )
    .toString()
    .trim();
  const alias = (row.model || row.alias || row.display_model || upstreamModel)
    .toString()
    .trim();
  const model = alias || upstreamModel;
  if (!model) {
    return null;
  }
  const normalizedEndpoints = normalizeChannelModelEndpoints(
    row.type,
    row.endpoints || row.endpoint_list || [],
    row.endpoint,
    protocol,
  );
  const normalizedEndpointCandidate = normalizeChannelModelEndpoint(
    row.type,
    row.endpoint,
    protocol,
  );
  return {
    model,
    upstream_model: upstreamModel || model,
    provider: normalizeChannelModelProviderValue(row.provider),
    type: normalizeChannelModelType(row.type),
    endpoint: normalizedEndpoints.includes(normalizedEndpointCandidate)
      ? normalizedEndpointCandidate
      : normalizedEndpoints[0],
    endpoints: normalizedEndpoints,
    inactive: row.inactive === true,
    selected: row.selected === true,
    is_stream: resolveModelTestStreamEnabled(row),
    input_price: normalizePriceOverrideValue(row.input_price),
    output_price: normalizePriceOverrideValue(row.output_price),
    price_unit: normalizePriceUnitValue(row.price_unit),
    currency: normalizeCurrencyValue(row.currency),
    price_components: normalizeComplexPriceComponents(row.price_components),
    sync_status: (row.sync_status || 'unknown').toString().trim(),
    last_synced_at: Number(row.last_synced_at || 0),
    enable_block_reason: (row.enable_block_reason || '').toString().trim(),
  };

};

const normalizeChannelModels = (rows, protocol) => {
  if (!Array.isArray(rows)) {
    return [];
  }
  const seen = new Set();
  const result = [];
  rows.forEach((row) => {
    const normalized = normalizeChannelModelConfigRow(row, protocol);
    if (!normalized) {
      return;
    }
    if (seen.has(normalized.model)) {
      return;
    }
    seen.add(normalized.model);
    result.push(normalized);
  });
  return result;
};

const buildChannelModelsFromLegacyFields = ({
  channelModels,
  availableModels,
  selectedModels,
  modelMapping,
  inputPrice,
  outputPrice,
  priceUnit,
  currency,
  protocol,
}) => {
  const normalizedChannelModels = normalizeChannelModels(channelModels, protocol);
  if (normalizedChannelModels.length > 0) {
    return normalizedChannelModels;
  }
  const orderedModels = [];
  const seen = new Set();
  const appendModel = (modelId) => {
    const normalized = (modelId || '').toString().trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    orderedModels.push(normalized);
  };
  (Array.isArray(availableModels) ? availableModels : []).forEach(appendModel);
  (Array.isArray(selectedModels) ? selectedModels : []).forEach(appendModel);

  const selectedSet = new Set(
    normalizeModelIDs(Array.isArray(selectedModels) ? selectedModels : []),
  );
  const modelMappingMap = parseJSONObject(modelMapping);
  const inputPriceMap = parseJSONObject(inputPrice);
  const outputPriceMap = parseJSONObject(outputPrice);
  const priceUnitMap = parseJSONObject(priceUnit);
  const currencyMap = parseJSONObject(currency);

  return orderedModels.map((modelId) => ({
    model: modelId,
    upstream_model:
      (modelMappingMap[modelId] || '').toString().trim() || modelId,
    type: 'text',
    selected: selectedSet.has(modelId),
    input_price: normalizePriceOverrideValue(inputPriceMap[modelId]),
    output_price: normalizePriceOverrideValue(outputPriceMap[modelId]),
    price_unit: normalizePriceUnitValue(priceUnitMap[modelId]),
    currency: normalizeCurrencyValue(currencyMap[modelId]),
  }));
};

const buildChannelModelState = (channelModels, protocol) => {
  const normalizedChannelModels = normalizeChannelModels(channelModels, protocol);
  const selectedModels = normalizedChannelModels
    .filter((row) => row.selected && row.inactive !== true)
    .map((row) => row.model);
  return {
    channelModels: normalizedChannelModels,
    selectedModels,
  };
};

const buildNextInputsWithChannelModels = (previousInputs, channelModels, protocol) => {
  const { channelModels: normalizedChannelModels, selectedModels } =
    buildChannelModelState(
      channelModels,
      protocol ?? previousInputs?.protocol,
    );
  const currentTestModel = (previousInputs.test_model || '').toString().trim();
  const nextTestModel =
    currentTestModel !== '' && selectedModels.includes(currentTestModel)
      ? currentTestModel
      : selectedModels[0] || '';
  return {
    ...previousInputs,
    channel_models: normalizedChannelModels,
    models: selectedModels,
    test_model: nextTestModel,
  };
};

const getChannelModelsFromInputs = (inputs) =>
  normalizeChannelModels(inputs?.channel_models, inputs?.protocol);

const getBlockedSelectedChannelModels = (rows, protocol) => {
  return normalizeChannelModels(rows, protocol).filter((row) => {
    if (row.inactive === true || row.selected !== true) {
      return false;
    }
    return ((row.enable_block_reason || '').toString().trim()) !== '';
  });
};

const buildBlockedSelectedModelsMessage = (rows, protocol, t) => {
  const blockedRows = getBlockedSelectedChannelModels(rows, protocol);
  if (blockedRows.length === 0) {
    return '';
  }
  const labels = blockedRows.map((row) => {
    const modelName =
      (row.upstream_model || row.model || '').toString().trim() || '-';
    const reason = (row.enable_block_reason || '').toString().trim();
    return reason ? `${modelName}（${reason}）` : modelName;
  });
  return t('channel.edit.messages.blocked_selected_models', {
    models: labels.join('；'),
  });
};

const extractChannelModelListItems = (payload) => {
  if (Array.isArray(payload?.items)) {
    return payload.items;
  }
  if (Array.isArray(payload?.channel_models)) {
    return payload.channel_models;
  }
  return [];
};

const fetchAllChannelModels = async (channelId, protocol) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const items = [];
  let page = 1;
  while (page < 50) {
    const res = await API.get(
      `/api/v1/admin/channel/${normalizedChannelId}/models`,
      {
        params: {
          page,
          page_size: 100,
        },
      },
    );
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || 'fetch channel models failed');
    }
    const pageItems = normalizeChannelModels(
      extractChannelModelListItems(data),
      protocol,
    );
    items.push(...pageItems);
    const total = Number(data?.total || pageItems.length || 0);
    if (
      pageItems.length === 0 ||
      items.length >= total ||
      pageItems.length < 100
    ) {
      break;
    }
    page += 1;
  }
  return normalizeChannelModels(items, protocol);
};

const fetchChannelTests = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return {
      items: [],
      lastTestedAt: 0,
    };
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/tests`,
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel tests failed');
  }
  return {
    items: normalizeModelTestResults(data?.items),
    lastTestedAt: Number(data?.last_tested_at || 0),
  };
};

const fetchChannelEndpoints = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/endpoints`,
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel endpoints failed');
  }
  return normalizeChannelEndpointRows(data?.items);
};

const fetchChannelEndpointPolicies = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/policies`,
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel endpoint policies failed');
  }
  return normalizeChannelEndpointPolicyRows(data?.items);
};

const fetchActiveChannelTasks = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const res = await API.get('/api/v1/admin/tasks', {
    params: {
      page: 1,
      page_size: 100,
      channel_id: normalizedChannelId,
      status: 'pending,running',
    },
  });
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel tasks failed');
  }
  return normalizeAsyncTasks(data?.items);
};

const fetchTaskById = async (taskId) => {
  const normalizedTaskId = (taskId || '').toString().trim();
  if (normalizedTaskId === '') {
    throw new Error('fetch task failed');
  }
  const res = await API.get(`/api/v1/admin/tasks/${normalizedTaskId}`);
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch task failed');
  }
  return normalizeAsyncTasks([data])[0] || null;
};

const validateChannelModels = (channelModels, t) => {
  const seen = new Set();
  for (const row of Array.isArray(channelModels) ? channelModels : []) {
    const alias = (row?.model || '').toString().trim();
    const upstreamModel = (row?.upstream_model || '').toString().trim();
    if (alias === '' || upstreamModel === '') {
      return t('channel.edit.messages.model_config_invalid');
    }
    if (seen.has(alias)) {
      return t('channel.edit.messages.model_config_invalid');
    }
    seen.add(alias);
    if (
      row?.input_price !== null &&
      normalizePriceOverrideValue(row?.input_price) === null
    ) {
      return t('channel.edit.messages.model_config_invalid');
    }
    if (
      row?.output_price !== null &&
      normalizePriceOverrideValue(row?.output_price) === null
    ) {
      return t('channel.edit.messages.model_config_invalid');
    }
  }
  return '';
};

const buildChannelConnectionSignature = ({
  protocol,
  key,
  baseURL,
  channelID,
}) => {
  const normalizedKey = (key || '').trim();
  const normalizedChannelID = (channelID || '').trim();
  const keyPart =
    normalizedKey !== '' ? normalizedKey : `@channel:${normalizedChannelID}`;
  return `${protocol}|${normalizeBaseURL(baseURL)}|${keyPart}`;
};

const buildChannelModelTestSignature = ({
  protocol,
  key,
  baseURL,
  channelID,
  models,
  channelModels,
}) =>
  `${buildChannelConnectionSignature({
    protocol,
    key,
    baseURL,
    channelID,
  })}|${normalizeModelIDs(models).join(',')}|${normalizeChannelModels(
    channelModels,
    protocol,
  )
    .filter((row) => row.selected)
    .map(
      (row) =>
        `${row.model}:${row.type}:${normalizeChannelModelEndpoints(
          row.type,
          row.endpoints,
          row.endpoint,
          protocol,
        ).join('|')}`,
    )
    .join(',')}`;

const normalizeModelTestResults = (results) => {
  if (!Array.isArray(results)) {
    return [];
  }
  return results
    .filter(
      (item) =>
        item && typeof item === 'object' && typeof item.model === 'string',
    )
    .map((item) => {
      const hasIsStream =
        Object.prototype.hasOwnProperty.call(item, 'is_stream') ||
        Object.prototype.hasOwnProperty.call(item, 'isStream');
      const rawIsStream = hasIsStream
        ? item.is_stream ?? item.isStream
        : null;
      return {
        channel_id: (item.channel_id || '').toString().trim(),
        model: item.model || '',
        upstream_model: item.upstream_model || '',
        type: normalizeChannelModelType(item.type),
        endpoint: item.endpoint || '',
        is_stream:
          rawIsStream === true
            ? true
            : rawIsStream === false
              ? false
              : null,
        status: item.status || 'unsupported',
        supported: !!item.supported,
        message: item.message || '',
        latency_ms: Number(item.latency_ms || 0),
        tested_at: Number(item.tested_at || 0),
        artifact_path: (item.artifact_path || '').toString().trim(),
        artifact_name: (item.artifact_name || '').toString().trim(),
        artifact_content_type: (item.artifact_content_type || '')
          .toString()
          .trim(),
        artifact_size: Number(item.artifact_size || 0),
      };
    });
};

const buildModelTestResultKey = (modelName, endpoint) =>
  `${(modelName || '').toString().trim()}::${(endpoint || '')
    .toString()
    .trim()}`;

const normalizeAsyncTaskStatus = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'pending':
    case 'running':
    case 'succeeded':
    case 'failed':
    case 'canceled':
      return normalized;
    default:
      return 'pending';
  }
};

const isActiveAsyncTaskStatus = (value) => {
  const normalized = normalizeAsyncTaskStatus(value);
  return normalized === 'pending' || normalized === 'running';
};

const normalizeAsyncTasks = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .filter((item) => item && typeof item === 'object' && item.id)
    .map((item) => ({
      id: (item.id || '').toString().trim(),
      type: (item.type || '').toString().trim(),
      status: normalizeAsyncTaskStatus(item.status),
      channel_id: (item.channel_id || '').toString().trim(),
      channel_name: (item.channel_name || '').toString().trim(),
      model: (item.model || '').toString().trim(),
      endpoint: (item.endpoint || '').toString().trim(),
      error_message: (item.error_message || '').toString().trim(),
      result: (item.result || '').toString().trim(),
      created_at: Number(item.created_at || 0),
      finished_at: Number(item.finished_at || 0),
    }));
};

const sanitizeCreateInputsForLocalStorage = (inputs) => {
  if (!inputs || typeof inputs !== 'object') {
    return CHANNEL_ORIGIN_INPUTS;
  }
  return {
    ...inputs,
    key: '',
  };
};

const sanitizeCreateConfigForLocalStorage = (config) => {
  if (!config || typeof config !== 'object') {
    return CHANNEL_DEFAULT_CONFIG;
  }
  return {
    ...config,
    ak: '',
    sk: '',
    vertex_ai_adc: '',
  };
};

const CHANNEL_ORIGIN_INPUTS = {
  id: '',
  name: '',
  protocol: 'openai',
  key: '',
  base_url: '',
  other: '',
  channel_models: [],
  models: [],
  test_model: '',
  created_time: 0,
  updated_at: 0,
};

const CHANNEL_DEFAULT_CONFIG = {
  region: '',
  sk: '',
  ak: '',
  user_id: '',
  api_base_url: '',
  account_base_url: '',
  vertex_ai_project_id: '',
  vertex_ai_adc: '',
};

function protocol2secretPrompt(protocol, t) {
  switch (protocol) {
    case 'zhipu':
      return t('channel.edit.key_prompts.zhipu');
    case 'xunfei':
      return t('channel.edit.key_prompts.spark');
    case 'fastgpt':
      return t('channel.edit.key_prompts.fastgpt');
    case 'tencent':
      return t('channel.edit.key_prompts.tencent');
    default:
      return t('channel.edit.key_prompts.default');
  }
}

function protocolSelectionHint(t) {
  return t('channel.edit.protocol_hint');
}

const resolveProtocolFromChannelPayload = (payload) => {
  const protocol = (payload?.protocol || '').toString().trim().toLowerCase();
  if (protocol !== '') {
    return protocol;
  }
  return 'openai';
};

const ChannelForm = ({ mode = 'auto' } = {}) => {
  const { t } = useTranslation();
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const channelId = params.id;
  const normalizedMode = (mode || 'auto').toString().trim().toLowerCase();
  const forceCreateMode = normalizedMode === 'add' || normalizedMode === 'create';
  const forceDetailMode = normalizedMode === 'edit' || normalizedMode === 'detail';
  const hasChannelID = !forceCreateMode && channelId !== undefined;
  const isDetailMode =
    forceDetailMode ||
    (hasChannelID && location.pathname.includes('/channel/detail/'));
  const isCreateMode = !hasChannelID;
  const returnPath = useMemo(() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);
  const returnChannelLabel = useMemo(() => {
    const raw = location.state?.channelLabel;
    if (typeof raw !== 'string') {
      return '';
    }
    return raw.trim();
  }, [location.state]);
  const copyFromId = useMemo(() => {
    if (hasChannelID) return '';
    const query = new URLSearchParams(location.search);
    return (query.get('copy_from') || '').trim();
  }, [hasChannelID, location.search]);
  const [loading, setLoading] = useState(
    hasChannelID || copyFromId !== '',
  );
  const activeDetailTab = useMemo(() => {
    if (!isDetailMode) {
      return 'overview';
    }
    const query = new URLSearchParams(location.search);
    return normalizeDetailTab(query.get('tab'));
  }, [isDetailMode, location.search]);
  const [channelKeySet, setChannelKeySet] = useState(false);
  const handleBackToChannelList = useCallback(() => {
    navigate('/admin/channel');
  }, [navigate]);
  const handleCancel = () => {
    if (isDetailMode && returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/admin/channel');
  };
  const goToDetailTab = useCallback(
    (nextTab) => {
      if (!isDetailMode) {
        return;
      }
      const normalizedTab = normalizeDetailTab(nextTab);
      const query = new URLSearchParams(location.search);
      if (normalizedTab === 'overview') {
        query.delete('tab');
      } else {
        query.set('tab', normalizedTab);
      }
      const nextSearch = query.toString();
      navigate(
        {
          pathname: location.pathname,
          search: nextSearch ? `?${nextSearch}` : '',
        },
        { replace: false, state: location.state },
      );
    },
    [isDetailMode, location.pathname, location.search, location.state, navigate],
  );
  const [inputs, setInputs] = useState(CHANNEL_ORIGIN_INPUTS);
  const detailChannelLabel = useMemo(() => {
    const currentName = (inputs.name || '').toString().trim();
    if (currentName !== '') {
      return currentName;
    }
    if (returnChannelLabel !== '') {
      return returnChannelLabel;
    }
    return '';
  }, [inputs.name, returnChannelLabel]);
  const [channelProtocolOptions, setChannelProtocolOptions] = useState(() =>
    getChannelProtocolOptions(),
  );
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [modelsSyncError, setModelsSyncError] = useState('');
  const [modelsLastSyncedAt, setModelsLastSyncedAt] = useState(0);
  const [verifiedModelSignature, setVerifiedModelSignature] = useState('');
  const [modelTestResults, setModelTestResults] = useState([]);
  const [channelEndpoints, setChannelEndpoints] = useState([]);
  const [channelEndpointsLoading, setChannelEndpointsLoading] = useState(false);
  const [channelEndpointsError, setChannelEndpointsError] = useState('');
  const [endpointMutatingKey, setEndpointMutatingKey] = useState('');
  const [channelEndpointPolicies, setChannelEndpointPolicies] = useState([]);
  const [channelEndpointPoliciesLoading, setChannelEndpointPoliciesLoading] =
    useState(false);
  const [channelEndpointPoliciesError, setChannelEndpointPoliciesError] =
    useState('');
  const [endpointEnableConfirmOpen, setEndpointEnableConfirmOpen] =
    useState(false);
  const [endpointEnableConfirmLoading, setEndpointEnableConfirmLoading] =
    useState(false);
  const [pendingEndpointEnableRow, setPendingEndpointEnableRow] = useState(null);
  const [policyEditorOpen, setPolicyEditorOpen] = useState(false);
  const [policyEditorSaving, setPolicyEditorSaving] = useState(false);
  const [selectedPolicyTemplate, setSelectedPolicyTemplate] = useState('');
  const [policyDraft, setPolicyDraft] = useState(
    buildEmptyEndpointPolicyDraft('', '', ''),
  );
  const [modelTesting, setModelTesting] = useState(false);
  const [modelTestingScope, setModelTestingScope] = useState('');
  const [modelTestingTargets, setModelTestingTargets] = useState([]);
  const [channelTasks, setChannelTasks] = useState([]);
  const [modelTestError, setModelTestError] = useState('');
  const [audioTestLanguage, setAudioTestLanguage] = useState('zh-CN');
  const openChannelTaskView = useCallback(
    (extraParams = {}) => {
      const targetChannelId = (channelId || '')
        .toString()
        .trim();
      const query = new URLSearchParams();
      if (targetChannelId !== '') {
        query.set('channel_id', targetChannelId);
      }
      Object.entries(extraParams || {}).forEach(([key, value]) => {
        const normalizedValue = (value || '').toString().trim();
        if (normalizedValue !== '') {
          query.set(key, normalizedValue);
        }
      });
      const search = query.toString();
      navigate(`/admin/channel/tasks${search ? `?${search}` : ''}`, {
        state: {
          from: `${location.pathname}${location.search}${location.hash}`,
          fromLabel: (inputs.name || channelId || '').toString().trim(),
          contextType: 'channel_test_history',
          contextLabel: (inputs.name || channelId || '').toString().trim(),
        },
      });
    },
    [
      channelId,
      inputs.name,
      location.hash,
      location.pathname,
      location.search,
      navigate,
    ],
  );
  const [modelTestedAt, setModelTestedAt] = useState(0);
  const [modelTestedSignature, setModelTestedSignature] = useState('');
  const [modelTestTargetModels, setModelTestTargetModels] = useState([]);
  const [detailModelMutating, setDetailModelMutating] = useState(false);
  const [detailBasicEditing, setDetailBasicEditing] = useState(false);
  const [detailEditingModelKey, setDetailEditingModelKey] = useState('');
  const [detailEditingModelSnapshot, setDetailEditingModelSnapshot] =
    useState(null);
  const [detailBasicSaving, setDetailBasicSaving] = useState(false);
  const [config, setConfig] = useState(CHANNEL_DEFAULT_CONFIG);
  const [providerOptions, setProviderOptions] = useState([]);
  const [providerModelOwners, setProviderModelOwners] = useState({});
  const [providerModelDetailsIndex, setProviderModelDetailsIndex] = useState(
    {},
  );
  const [providerDataLoading, setProviderDataLoading] = useState(false);
  const [providerDataLoaded, setProviderDataLoaded] = useState(false);
  const [appendProviderModalOpen, setAppendProviderModalOpen] = useState(false);
  const [appendingProviderModel, setAppendingProviderModel] = useState(false);
  const [complexPricingModalOpen, setComplexPricingModalOpen] = useState(false);
  const [complexPricingModalData, setComplexPricingModalData] = useState(null);
  const [appendProviderForm, setAppendProviderForm] = useState({
    provider: '',
    model: '',
    type: 'text',
  });
  const [modelSearchKeyword, setModelSearchKeyword] = useState('');
  const [detailModelFilter, setDetailModelFilter] = useState('all');
  const [detailModelPage, setDetailModelPage] = useState(1);
  const fetchingModelsRef = useRef(false);
  const pendingRefreshTaskIdRef = useRef('');
  const pendingRefreshSignatureRef = useRef('');
  const deferredModelSearchKeyword = useDeferredValue(modelSearchKeyword);
  const currentProtocolOption = useMemo(() => {
    const normalizedProtocol = (inputs.protocol || '')
      .toString()
      .trim()
      .toLowerCase();
    if (normalizedProtocol === '') {
      return null;
    }
    return (
      channelProtocolOptions.find(
        (option) =>
          (option?.value || '').toString().trim().toLowerCase() ===
          normalizedProtocol,
      ) || null
    );
  }, [channelProtocolOptions, inputs.protocol]);

  const buildEffectiveKey = useCallback(() => {
    let effectiveKey = inputs.key || '';
    if (effectiveKey === '') {
      if (config.ak !== '' && config.sk !== '' && config.region !== '') {
        effectiveKey = `${config.ak}|${config.sk}|${config.region}`;
      } else if (
        config.region !== '' &&
        config.vertex_ai_project_id !== '' &&
        config.vertex_ai_adc !== ''
      ) {
        effectiveKey = `${config.region}|${config.vertex_ai_project_id}|${config.vertex_ai_adc}`;
      }
    }
    return effectiveKey;
  }, [
    config.ak,
    config.region,
    config.sk,
    config.vertex_ai_adc,
    config.vertex_ai_project_id,
    inputs.key,
  ]);

  const effectivePreviewKey = useMemo(
    () => buildEffectiveKey().trim(),
    [buildEffectiveKey],
  );
  const effectiveAPIBaseURL = useMemo(
    () => resolveEffectiveAPIBaseURL(inputs, config),
    [config, inputs],
  );
  const previewChannelID = useMemo(
    () => ((hasChannelID ? channelId : '') || '').trim(),
    [channelId, hasChannelID],
  );
  const currentModelSignature = useMemo(
    () =>
      buildChannelConnectionSignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: effectiveAPIBaseURL,
        channelID: previewChannelID,
      }),
    [effectiveAPIBaseURL, effectivePreviewKey, inputs.protocol, previewChannelID],
  );
  const requiresConnectionVerification = false;
  const showStepOne = isDetailMode ? activeDetailTab === 'overview' : true;
  const showStepTwo =
    isDetailMode &&
    (activeDetailTab === 'models' || activeDetailTab === 'endpoints');
  const showDetailOverviewTab = isDetailMode && activeDetailTab === 'overview';
  const showDetailModelsTab = isDetailMode && activeDetailTab === 'models';
  const showDetailEndpointsTab = isDetailMode && activeDetailTab === 'endpoints';
  const showDetailTestsTab = isDetailMode && activeDetailTab === 'tests';
  const detailBasicReadonly = isDetailMode && !detailBasicEditing;
  const detailModelsEditing =
    isDetailMode && detailEditingModelKey.toString().trim() !== '';
  const isAnyDetailSectionEditing = detailBasicEditing || detailModelsEditing;
  const detailTabItems = [
    {
      key: 'overview',
      label: t('channel.edit.detail_tabs.overview'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'overview',
    },
    {
      key: 'models',
      label: t('channel.edit.detail_tabs.models'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'models',
    },
    {
      key: 'tests',
      label: t('channel.edit.detail_tabs.tests'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'tests',
    },
    {
      key: 'endpoints',
      label: t('channel.edit.detail_tabs.endpoints'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'endpoints',
    },
  ];
  const detailBasicEditLocked =
    isDetailMode &&
    !detailBasicEditing &&
    detailModelsEditing;
  const detailModelsEditLocked = isDetailMode && detailBasicEditing;
  const detailTestingReadonly = isDetailMode && isAnyDetailSectionEditing;
  const inputReadonlyProps = detailBasicReadonly ? { readOnly: true } : {};
  const visibleChannelModels = useMemo(
    () => normalizeChannelModels(inputs.channel_models, inputs.protocol),
    [inputs.channel_models, inputs.protocol],
  );
  const detailEditingModelRow = useMemo(() => {
    if (!detailModelsEditing) {
      return null;
    }
    return (
      visibleChannelModels.find(
        (row) => row.upstream_model === detailEditingModelKey,
      ) || null
    );
  }, [detailEditingModelKey, detailModelsEditing, visibleChannelModels]);
  const modelTestResultsByKey = useMemo(() => {
    const index = new Map();
    normalizeModelTestResults(modelTestResults).forEach((item) => {
      const key = buildModelTestResultKey(item.model, item.endpoint);
      if (!item.model || !item.endpoint || key === '::') {
        return;
      }
      const existing = index.get(key);
      if (!existing) {
        index.set(key, item);
        return;
      }
      if (Number(item.tested_at || 0) > Number(existing.tested_at || 0)) {
        index.set(key, item);
        return;
      }
      if (
        Number(item.tested_at || 0) === Number(existing.tested_at || 0) &&
        Number(item.latency_ms || 0) >= Number(existing.latency_ms || 0)
      ) {
        index.set(key, item);
      }
    });
    return index;
  }, [modelTestResults]);
  const modelTestRows = useMemo(() => {
    return visibleChannelModels.filter(
      (row) => row.inactive !== true && row.selected === true,
    );
  }, [visibleChannelModels]);
  const modelTestingTargetSet = useMemo(
    () => new Set(modelTestingTargets),
    [modelTestingTargets],
  );
  const activeChannelTasksByModel = useMemo(() => {
    const index = new Map();
    normalizeAsyncTasks(channelTasks)
      .filter(
        (item) =>
          item.type === 'channel_model_test' &&
          isActiveAsyncTaskStatus(item.status),
      )
      .forEach((item) => {
        if (!item.model) {
          return;
        }
        const existing = index.get(item.model);
        if (!existing || existing.status === 'pending') {
          index.set(item.model, item);
        }
      });
    return index;
  }, [channelTasks]);
  const activeRefreshModelsTask = useMemo(
    () =>
      normalizeAsyncTasks(channelTasks).find(
        (item) =>
          item.type === 'channel_refresh_models' &&
          isActiveAsyncTaskStatus(item.status),
      ) || null,
    [channelTasks],
  );
  const selectedModelTestHasActiveTasks = useMemo(
    () =>
      modelTestTargetModels.some((modelName) =>
        activeChannelTasksByModel.has(modelName),
      ),
    [activeChannelTasksByModel, modelTestTargetModels],
  );
  useEffect(() => {
    const visibleModelSet = new Set(modelTestRows.map((row) => row.model));
    setModelTestTargetModels((previous) => {
      const next = previous.filter((modelName) => visibleModelSet.has(modelName));
      if (next.length === previous.length) {
        return previous;
      }
      return next;
    });
  }, [modelTestRows]);
  const getProviderOwnersForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider,
      );
      const owners = new Set();
      buildProviderLookupKeys(row).forEach((key) => {
        (providerModelOwners[key] || []).forEach((providerId) => {
          owners.add(providerId);
        });
      });
      const sortedOwners = Array.from(owners).sort((a, b) =>
        a.localeCompare(b),
      );
      if (selectedProvider === '' || !sortedOwners.includes(selectedProvider)) {
        return sortedOwners;
      }
      return [
        selectedProvider,
        ...sortedOwners.filter((item) => item !== selectedProvider),
      ];
    },
    [providerModelOwners],
  );
  const getProviderSelectOptionsForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider,
      );
      const providerOptionById = new Map(
        (Array.isArray(providerOptions) ? providerOptions : []).map((option) => [
          normalizeProviderIdentifier(option?.value || ''),
          option,
        ]),
      );
      const allOptions = Array.from(providerOptionById.values());
      if (selectedProvider === '') {
        return allOptions;
      }
      const normalizedSelectedProvider =
        normalizeProviderIdentifier(selectedProvider);
      const matchedOption = providerOptionById.get(normalizedSelectedProvider);
      if (!matchedOption) {
        return [
          {
            key: normalizedSelectedProvider,
            value: selectedProvider,
            text: selectedProvider || '-',
          },
          ...allOptions,
        ];
      }
      return [
        matchedOption,
        ...allOptions.filter(
          (option) =>
            normalizeProviderIdentifier(option?.value || '') !==
            normalizedSelectedProvider,
        ),
      ];
    },
    [providerOptions],
  );
  const resolvePreferredProviderForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider,
      );
      const providerOwners = getProviderOwnersForModel(row);
      if (
        selectedProvider !== '' &&
        providerOwners.includes(selectedProvider)
      ) {
        return selectedProvider;
      }
      if (providerOwners.length === 1) {
        return providerOwners[0];
      }
      return '';
    },
    [getProviderOwnersForModel],
  );
  const getSelectedProviderDisplayItems = useCallback(
    (row) => {
      const selectedProvider = resolvePreferredProviderForModel(row);
      if (selectedProvider === '') {
        return [];
      }
      const providerOptionById = new Map(
        (Array.isArray(providerOptions) ? providerOptions : []).map((option) => [
          normalizeProviderIdentifier(option?.value || ''),
          option,
        ]),
      );
      const normalizedSelectedProvider =
        normalizeProviderIdentifier(selectedProvider);
      const matchedOption = providerOptionById.get(normalizedSelectedProvider);
      return [
        {
          key: normalizedSelectedProvider || selectedProvider,
          value: selectedProvider,
          text: matchedOption?.text || selectedProvider,
        },
      ];
    },
    [providerOptions, resolvePreferredProviderForModel],
  );
  const getComplexPricingDetailsForModel = useCallback(
    (row) => {
      const owners = getProviderOwnersForModel(row);
      const keys = buildProviderLookupKeys(row);
      const details = [];
      const seen = new Set();
      owners.forEach((providerId) => {
        const providerDetails = providerModelDetailsIndex[providerId] || {};
        keys.forEach((key) => {
          const detail = providerDetails[key];
          if (!detail || (detail.price_components || []).length === 0) {
            return;
          }
          const uniqueKey = `${providerId}\u0000${detail.model}`;
          if (seen.has(uniqueKey)) {
            return;
          }
          seen.add(uniqueKey);
          const priceComponents = mergePriceComponentOverrides(
            detail.price_components,
            row?.price_components,
          );
          if (priceComponents.length === 0) {
            return;
          }
          details.push({
            provider: providerId,
            ...detail,
            price_components: priceComponents,
            source:
              (row?.price_components || []).length > 0
                ? 'channel_override'
                : detail.source,
          });
        });
      });
      return details.sort((a, b) => {
        const byProvider = (a.provider || '').localeCompare(b.provider || '');
        if (byProvider !== 0) {
          return byProvider;
        }
        return (a.model || '').localeCompare(b.model || '');
      });
    },
    [getProviderOwnersForModel, providerModelDetailsIndex],
  );
  const getProviderCandidateEndpointsForModel = useCallback(
    (row) => {
      const providerId = resolvePreferredProviderForModel(row);
      const providerDetails = providerModelDetailsIndex[providerId] || {};
      const candidates = [];
      normalizeChannelModelEndpoints(
        row?.type,
        row?.endpoints || row?.endpoint_list || [],
        row?.endpoint,
        inputs.protocol,
      ).forEach((endpoint) => {
        candidates.push(endpoint);
      });
      buildProviderLookupKeys(row).forEach((key) => {
        const detail = providerDetails[key];
        if (!detail || !Array.isArray(detail.supported_endpoints)) {
          return;
        }
        detail.supported_endpoints.forEach((endpoint) => {
          candidates.push(endpoint);
        });
      });
      const seen = new Set();
      const result = [];
      candidates.forEach((endpoint) => {
        const normalized = normalizeChannelModelEndpoint(
          row?.type,
          endpoint,
          inputs.protocol,
        );
        if (normalized === '' || seen.has(normalized)) {
          return;
        }
        seen.add(normalized);
        result.push(normalized);
      });
      return result;
    },
    [inputs.protocol, providerModelDetailsIndex, resolvePreferredProviderForModel],
  );
  const getEndpointOptionsForModel = useCallback(
    (row) => {
      const providerEndpoints = getProviderCandidateEndpointsForModel(row);
      return buildEndpointOptionsFromValues(
        row?.type,
        providerEndpoints,
        inputs.protocol,
      );
    },
    [getProviderCandidateEndpointsForModel, inputs.protocol],
  );
  const getEffectiveModelEndpoint = useCallback(
    (row) => {
      const normalizedCurrent = normalizeChannelModelEndpoint(
        row?.type,
        row?.endpoint,
        inputs.protocol,
      );
      const providerEndpoints = getProviderCandidateEndpointsForModel(row);
      if (normalizedCurrent !== '') {
        return normalizedCurrent;
      }
      return providerEndpoints[0] || normalizedCurrent;
    },
    [getProviderCandidateEndpointsForModel, inputs.protocol],
  );
  const modelTestGroups = useMemo(() => {
    const groups = new Map();
    modelTestRows.forEach((row) => {
      const provider = resolvePreferredProviderForModel(row);
      if (provider === '') {
        return;
      }
      const type = normalizeChannelModelType(row.type);
      const key = `${provider}::${type}`;
      if (!groups.has(key)) {
        groups.set(key, {
          key,
          provider,
          type,
          rows: [],
        });
      }
      groups.get(key).rows.push(row);
    });
    return Array.from(groups.values())
      .map((group) => {
        let commonEndpoints = null;
        const labelByValue = new Map();
        group.rows.forEach((row) => {
          const options = getEndpointOptionsForModel(row);
          const rowEndpointSet = new Set(options.map((option) => option.value));
          options.forEach((option) => {
            if (!labelByValue.has(option.value)) {
              labelByValue.set(option.value, option.text || option.value);
            }
          });
          if (commonEndpoints === null) {
            commonEndpoints = rowEndpointSet;
            return;
          }
          commonEndpoints = new Set(
            Array.from(commonEndpoints).filter((endpoint) =>
              rowEndpointSet.has(endpoint),
            ),
          );
        });
        const endpointOptions = Array.from(commonEndpoints || [])
          .sort((a, b) => a.localeCompare(b))
          .map((endpoint) => ({
            key: endpoint,
            value: endpoint,
            text: labelByValue.get(endpoint) || endpoint,
          }));
        const endpointSet = new Set(
          group.rows.map((row) => getEffectiveModelEndpoint(row)),
        );
        return {
          ...group,
          endpointOptions,
          endpointValue:
            endpointSet.size === 1 ? Array.from(endpointSet)[0] || '' : '',
        };
      })
      .sort((left, right) => {
        const providerOrder = (left.provider || '').localeCompare(
          right.provider || '',
        );
        if (providerOrder !== 0) {
          return providerOrder;
        }
        return (left.type || '').localeCompare(right.type || '');
      });
  }, [
    getEffectiveModelEndpoint,
    getEndpointOptionsForModel,
    modelTestRows,
    resolvePreferredProviderForModel,
  ]);
  const openComplexPricingModal = useCallback(
    (row) => {
      const details = getComplexPricingDetailsForModel(row);
      setComplexPricingModalData({
        model: row?.upstream_model || row?.model || '',
        alias: row?.model || '',
        details,
      });
      setComplexPricingModalOpen(true);
    },
    [getComplexPricingDetailsForModel],
  );
  const closeComplexPricingModal = useCallback(() => {
    setComplexPricingModalOpen(false);
    setComplexPricingModalData(null);
  }, []);
  const hasProviderConfiguredForModel = useCallback(
    (row) => getProviderOwnersForModel(row).length > 0,
    [getProviderOwnersForModel],
  );
  const canSelectChannelModel = useCallback(
    (row) =>
      row?.inactive !== true &&
      hasProviderConfiguredForModel(row) &&
      !((row?.enable_block_reason || '').toString().trim()),
    [hasProviderConfiguredForModel],
  );
  const activeChannelModels = useMemo(
    () => visibleChannelModels.filter((row) => row.inactive !== true),
    [visibleChannelModels],
  );
  const detailFilteredChannelModels = useMemo(() => {
    if (!isDetailMode) {
      return visibleChannelModels;
    }
    return visibleChannelModels.filter((row) => {
      if (detailModelFilter === 'enabled') {
        return row.selected === true;
      }
      if (detailModelFilter === 'disabled') {
        return row.selected !== true;
      }
      return true;
    });
  }, [
    detailModelFilter,
    isDetailMode,
    visibleChannelModels,
  ]);
  const searchedChannelModels = useMemo(() => {
    const keyword = normalizeSearchKeyword(deferredModelSearchKeyword);
    if (keyword === '') {
      return detailFilteredChannelModels;
    }
    return detailFilteredChannelModels.filter((row) => {
      const syncStatus = (row?.sync_status || 'unknown').toString().trim();
      const providerOwners = getProviderOwnersForModel(row).join(' ');
      const selectedProviderText = getSelectedProviderDisplayItems(row)
        .map((item) => item.text || item.value || '')
        .join(' ');
      const candidates = [
        row?.upstream_model,
        row?.model,
        row?.type,
        syncStatus,
        t(`channel.edit.model_selector.upstream_return_status.${syncStatus}`),
        providerOwners,
        selectedProviderText,
      ].map(normalizeSearchKeyword);
      return candidates.some((candidate) => candidate.includes(keyword));
    });
  }, [
    deferredModelSearchKeyword,
    detailFilteredChannelModels,
    getProviderOwnersForModel,
    getSelectedProviderDisplayItems,
    t,
  ]);
  const detailModelTotalPages = useMemo(() => {
    return Math.max(
      1,
      Math.ceil(searchedChannelModels.length / CHANNEL_MODEL_PAGE_SIZE),
    );
  }, [searchedChannelModels.length]);
  const renderedChannelModels = useMemo(() => {
    const offset = (detailModelPage - 1) * CHANNEL_MODEL_PAGE_SIZE;
    return searchedChannelModels.slice(offset, offset + CHANNEL_MODEL_PAGE_SIZE);
  }, [searchedChannelModels, detailModelPage]);
  const detailCurrentPageSelectableModels = useMemo(
    () => renderedChannelModels.filter((row) => canSelectChannelModel(row)),
    [canSelectChannelModel, renderedChannelModels],
  );
  const detailCurrentPageBlockedModels = useMemo(
    () =>
      renderedChannelModels.filter(
        (row) => row.inactive !== true && !canSelectChannelModel(row),
      ),
    [canSelectChannelModel, renderedChannelModels],
  );
  const detailCurrentPageAllSelected = useMemo(
    () =>
      detailCurrentPageSelectableModels.length > 0 &&
      detailCurrentPageSelectableModels.every((row) => row.selected === true),
    [detailCurrentPageSelectableModels],
  );
  const detailCurrentPagePartiallySelected = useMemo(
    () =>
      !detailCurrentPageAllSelected &&
      detailCurrentPageSelectableModels.some((row) => row.selected === true),
    [detailCurrentPageAllSelected, detailCurrentPageSelectableModels],
  );
  const modelSelectionSummaryText = useMemo(
    () =>
      t('channel.edit.model_selector.summary', {
        selected: inputs.models.length,
        total: activeChannelModels.length,
      }),
    [activeChannelModels.length, inputs.models.length, t],
  );
  const modelSectionMetaText = useMemo(
    () => modelSelectionSummaryText,
    [modelSelectionSummaryText],
  );
  const endpointCapabilityStats = useMemo(() => {
    return channelEndpoints.reduce(
      (acc, row) => {
        acc.total += 1;
        if (row.enabled) {
          acc.enabled += 1;
        } else {
          acc.disabled += 1;
        }
        return acc;
      },
      {
        total: 0,
        enabled: 0,
        disabled: 0,
      },
    );
  }, [channelEndpoints]);
  const endpointSummaryText = useMemo(
    () =>
      t('channel.edit.endpoint_capabilities.summary', {
        total: endpointCapabilityStats.total,
        configured: channelEndpointPolicies.length,
        capability_enabled: endpointCapabilityStats.enabled,
        policy_enabled: channelEndpointPolicies.filter((row) => row.enabled)
          .length,
      }),
    [channelEndpointPolicies, endpointCapabilityStats.enabled, endpointCapabilityStats.total, t],
  );
  const endpointCapabilityReadonly =
    !isDetailMode ||
    isAnyDetailSectionEditing ||
    channelEndpointsLoading ||
    endpointMutatingKey !== '';
  const endpointPolicyReadonly =
    !isDetailMode || isAnyDetailSectionEditing || policyEditorSaving;

  const handleInputChange = (e, { name, value }) => {
    const nextValue = name === 'id' ? normalizeChannelIdentifier(value) : value;
    setInputs((inputs) => ({ ...inputs, [name]: nextValue }));
  };

  const handleConfigChange = (e, { name, value }) => {
    setConfig((inputs) => ({ ...inputs, [name]: value }));
  };

  const keyField = useMemo(() => {
    if (inputs.protocol === 'awsclaude' || inputs.protocol === 'vertexai') {
      return null;
    }
    return (
      <AppFormRow>
        <AppField label={t('channel.edit.key')} required={isCreateMode}>
          <AppInput
            className='router-section-input'
            name='key'
            type='password'
            required={isCreateMode}
            placeholder={
              channelKeySet && (inputs.key || '').trim() === ''
                ? '********'
                : protocol2secretPrompt(inputs.protocol, t)
            }
            onChange={handleInputChange}
            value={inputs.key}
            autoComplete='new-password'
            {...inputReadonlyProps}
          />
        </AppField>
      </AppFormRow>
    );
  }, [
    channelKeySet,
    handleInputChange,
    inputReadonlyProps,
    inputs.key,
    inputs.protocol,
    isCreateMode,
    t,
  ]);

  const buildChannelPayloadFromState = useCallback(
    (baseInputs, baseConfig, options = {}) => {
      const { includeModelState = true } = options;
      const effectiveKey = buildEffectiveKey();
      let localInputs = { ...baseInputs, key: effectiveKey };
      localInputs.id = (localInputs.id || '').toString().trim();
      localInputs.name = normalizeChannelIdentifier(localInputs.name);
      if (localInputs.key === 'undefined|undefined|undefined') {
        localInputs.key = '';
      }
      if (localInputs.base_url && localInputs.base_url.endsWith('/')) {
        localInputs.base_url = localInputs.base_url.slice(
          0,
          localInputs.base_url.length - 1,
        );
      }
      if (localInputs.protocol === 'azure' && localInputs.other === '') {
        localInputs.other = '2024-03-01-preview';
      }
      if (includeModelState) {
        const derivedModelState = buildChannelModelState(
          baseInputs.channel_models,
          baseInputs.protocol,
        );
        localInputs.channel_models = derivedModelState.channelModels;
        localInputs.models = derivedModelState.selectedModels.join(',');
      } else {
        delete localInputs.channel_models;
        delete localInputs.models;
      }
      const submitConfig = {
        ...baseConfig,
        api_base_url: normalizeBaseURL(baseConfig.api_base_url),
        account_base_url: normalizeBaseURL(baseConfig.account_base_url),
      };
      localInputs.config = JSON.stringify(submitConfig);
      return localInputs;
    },
    [buildEffectiveKey],
  );

  const buildChannelPayload = useCallback(
    (options = {}) => buildChannelPayloadFromState(inputs, config, options),
    [buildChannelPayloadFromState, config, inputs],
  );

  const persistDetailChannelModels = useCallback(
    async (nextChannelModels) => {
      if (!isDetailMode) {
        return true;
      }
      const targetChannelID = (channelId || '').toString().trim();
      if (targetChannelID === '') {
        return false;
      }
      const blockedMessage = buildBlockedSelectedModelsMessage(
        nextChannelModels,
        inputs.protocol,
        t,
      );
      if (blockedMessage !== '') {
        showError(blockedMessage);
        return false;
      }
      const nextInputs = buildNextInputsWithChannelModels(
        inputs,
        nextChannelModels,
        inputs.protocol,
      );
      setDetailModelMutating(true);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelID}/models`,
          {
            channel_models: getChannelModelsFromInputs(nextInputs),
          },
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(message || t('channel.edit.messages.save_channel_failed'));
          return false;
        }
        setInputs((prev) => ({
          ...prev,
          channel_models: getChannelModelsFromInputs(nextInputs),
          models: nextInputs.models,
          test_model: nextInputs.test_model,
        }));
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.messages.save_channel_failed'),
        );
        return false;
      } finally {
        setDetailModelMutating(false);
      }
    },
    [
      channelId,
      inputs,
      isDetailMode,
      t,
    ],
  );

  const persistDetailChannel = useCallback(
    async ({
      loadingSetter = null,
      successMessage = '',
      validateBasic = false,
      includeModelState = true,
    } = {}) => {
      if (!isDetailMode) {
        return false;
      }
      const targetChannelID = (channelId || '').toString().trim();
      if (targetChannelID === '') {
        return false;
      }
      if (validateBasic) {
        const identifierError = validateChannelIdentifier(inputs.name, t);
        if (identifierError !== '') {
          showInfo(identifierError);
          return false;
        }
        if (buildEffectiveKey().trim() === '' && !channelKeySet) {
          showInfo(t('channel.edit.messages.key_required'));
          return false;
        }
      }
      if (includeModelState) {
        const blockedMessage = buildBlockedSelectedModelsMessage(
          inputs.channel_models,
          inputs.protocol,
          t,
        );
        if (blockedMessage !== '') {
          showError(blockedMessage);
          return false;
        }
      }
      if (typeof loadingSetter === 'function') {
        loadingSetter(true);
      }
      try {
        const payload = buildChannelPayload({ includeModelState });
        const res = await API.put('/api/v1/admin/channel/', {
          ...payload,
          id: targetChannelID,
        });
        const { success, message } = res.data || {};
        if (!success) {
          showError(message || t('channel.edit.messages.save_channel_failed'));
          return false;
        }
        if ((payload.key || '').trim() !== '') {
          setChannelKeySet(true);
        }
        if (successMessage) {
          showSuccess(successMessage);
        }
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.messages.save_channel_failed'),
        );
        return false;
      } finally {
        if (typeof loadingSetter === 'function') {
          loadingSetter(false);
        }
      }
    },
    [
      buildChannelPayload,
      buildEffectiveKey,
      channelId,
      channelKeySet,
      inputs.name,
      isDetailMode,
      t,
    ],
  );

  const saveDetailBasicInfo = useCallback(async () => {
    const ok = await persistDetailChannel({
      loadingSetter: setDetailBasicSaving,
      successMessage: t('channel.edit.messages.update_success'),
      validateBasic: true,
      includeModelState: false,
    });
    if (ok) {
      setDetailBasicEditing(false);
    }
  }, [persistDetailChannel, t]);

  const saveDetailModelsConfig = useCallback(async () => {
    if (!detailModelsEditing) {
      return;
    }
    const ok = await persistDetailChannelModels(visibleChannelModels);
    if (ok) {
      setDetailEditingModelKey('');
      setDetailEditingModelSnapshot(null);
      showSuccess(t('channel.edit.messages.update_success'));
    }
  }, [detailModelsEditing, persistDetailChannelModels, t, visibleChannelModels]);

  const loadChannelModelsFromServer = useCallback(
    async (targetChannelId, protocol) => {
      try {
        return await fetchAllChannelModels(targetChannelId, protocol);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.messages.fetch_models_failed'),
        );
      }
    },
    [t],
  );

  const loadChannelTestsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelTests(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.model_tester.test_failed'),
        );
      }
    },
    [t],
  );

  const loadChannelEndpointsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelEndpoints(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message ||
            t('channel.edit.endpoint_capabilities.load_failed'),
        );
      }
    },
    [t],
  );

  const loadChannelEndpointPoliciesFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelEndpointPolicies(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.endpoint_policies.load_failed'),
        );
      }
    },
    [t],
  );

  const loadChannelTasksFromServer = useCallback(async (targetChannelId) => {
    try {
      return await fetchActiveChannelTasks(targetChannelId);
    } catch (error) {
      throw new Error(error?.message || 'fetch channel tasks failed');
    }
  }, []);

  const refreshChannelRuntimeState = useCallback(
    async (targetChannelId) => {
      const normalizedChannelId = (targetChannelId || '').toString().trim();
      if (normalizedChannelId === '') {
        return;
      }
      const [
        nextChannelModels,
        nextTests,
        nextTasks,
        nextEndpoints,
        nextPolicies,
      ] =
        await Promise.all([
        loadChannelModelsFromServer(normalizedChannelId, inputs.protocol),
        loadChannelTestsFromServer(normalizedChannelId),
        loadChannelTasksFromServer(normalizedChannelId),
        loadChannelEndpointsFromServer(normalizedChannelId),
        loadChannelEndpointPoliciesFromServer(normalizedChannelId),
      ]);
      const nextInputs = buildNextInputsWithChannelModels(
        inputs,
        nextChannelModels,
        inputs.protocol,
      );
      const nextSignature = buildChannelModelTestSignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: effectiveAPIBaseURL,
        channelID: normalizedChannelId,
        models: nextInputs.models,
        channelModels: nextInputs.channel_models,
      });
      setInputs(nextInputs);
      setModelTestResults(normalizeModelTestResults(nextTests.items));
      setModelTestError('');
      setModelTestedAt(
        Number(nextTests.lastTestedAt || 0) > 0
          ? Number(nextTests.lastTestedAt) * 1000
          : 0,
      );
      setModelTestedSignature(
        Number(nextTests.lastTestedAt || 0) > 0 ? nextSignature : '',
      );
      setChannelTasks(normalizeAsyncTasks(nextTasks));
      setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
      setChannelEndpointsError('');
      setChannelEndpointPolicies(
        normalizeChannelEndpointPolicyRows(nextPolicies),
      );
      setChannelEndpointPoliciesError('');
    },
    [
      effectiveAPIBaseURL,
      effectivePreviewKey,
      inputs,
      inputs.protocol,
      loadChannelEndpointPoliciesFromServer,
      loadChannelEndpointsFromServer,
      loadChannelModelsFromServer,
      loadChannelTasksFromServer,
      loadChannelTestsFromServer,
    ],
  );

  const loadChannelById = useCallback(
    async (targetId, forCopy = false, fromCreating = false) => {
      try {
        let res = await API.get(`/api/v1/admin/channel/${targetId}`);
        const { success, message, data } = res.data;
        if (success) {
          const [remoteChannelModels, channelTestsData, activeTasks] =
            await Promise.all([
              loadChannelModelsFromServer(
                data.id || targetId,
                resolveProtocolFromChannelPayload(data),
              ),
              forCopy
                ? Promise.resolve({ items: [], lastTestedAt: 0 })
                : loadChannelTestsFromServer(data.id || targetId),
              forCopy
                ? Promise.resolve([])
                : loadChannelTasksFromServer(data.id || targetId),
            ]);
          const storedModelTestResults = normalizeModelTestResults(
            channelTestsData.items,
          );
          const storedModelTestedAt =
            Number(channelTestsData.lastTestedAt || 0) > 0
              ? Number(channelTestsData.lastTestedAt) * 1000
              : 0;
          let parsedConfig = {};
          if (data.config !== '') {
            parsedConfig = JSON.parse(data.config);
          }
          const normalizedProtocol = resolveProtocolFromChannelPayload(data);
          const modelState = buildChannelModelState(
            remoteChannelModels,
            normalizedProtocol,
          );
          const loadedModelTestSignature = buildChannelModelTestSignature({
            protocol: normalizedProtocol,
            key: '',
            baseURL: resolveEffectiveAPIBaseURL(data, parsedConfig),
            channelID: data.id || targetId,
            models: modelState.selectedModels,
            channelModels: modelState.channelModels,
          });

          if (forCopy) {
            pendingRefreshTaskIdRef.current = '';
            pendingRefreshSignatureRef.current = '';
            setInputs({
              id: '',
              name: '',
              protocol: normalizedProtocol,
              key: '',
              base_url: data.base_url || '',
              other: data.other || '',
              channel_models: modelState.channelModels,
              models: modelState.selectedModels,
              test_model: data.test_model || modelState.selectedModels[0] || '',
              created_time: 0,
              updated_at: 0,
            });
            setModelTestResults([]);
            setModelTestError('');
            setModelTestedAt(0);
            setModelTestedSignature('');
            setModelTestTargetModels([]);
            setChannelTasks([]);
          } else {
            pendingRefreshTaskIdRef.current = '';
            pendingRefreshSignatureRef.current = '';
            setInputs({
              id: data.id,
              name: data.name || '',
              protocol: normalizedProtocol,
              key: '',
              base_url: data.base_url || '',
              other: data.other || '',
              channel_models: modelState.channelModels,
              models: modelState.selectedModels,
              test_model: data.test_model || modelState.selectedModels[0] || '',
              status: data.status,
              weight: data.weight,
              priority: data.priority,
              created_time: Number(data.created_time || 0),
              updated_at: Number(data.updated_at || 0),
            });
            setModelTestResults(storedModelTestResults);
            setModelTestError('');
            setModelTestedAt(storedModelTestedAt);
            setModelTestedSignature(
              storedModelTestResults.length > 0 && storedModelTestedAt > 0
                ? loadedModelTestSignature
                : '',
            );
            setModelTestTargetModels([]);
            setChannelTasks(normalizeAsyncTasks(activeTasks));
          }
          setConfig({
            ...CHANNEL_DEFAULT_CONFIG,
            ...parsedConfig,
            api_base_url: normalizeBaseURL(parsedConfig.api_base_url),
            account_base_url: normalizeBaseURL(parsedConfig.account_base_url),
          });
          if (hasChannelID) {
            setChannelKeySet(!!data.key_set);
          } else {
            setChannelKeySet(false);
          }
        } else {
          showError(message);
        }
      } finally {
        setLoading(false);
      }
    },
    [
      hasChannelID,
      loadChannelModelsFromServer,
      loadChannelTasksFromServer,
      loadChannelTestsFromServer,
    ],
  );

  const cancelDetailBasicEdit = useCallback(async () => {
    if (!isDetailMode || !channelId) {
      setDetailBasicEditing(false);
      return;
    }
    setLoading(true);
    setDetailBasicEditing(false);
    await loadChannelById(channelId, false, false);
  }, [channelId, isDetailMode, loadChannelById]);

  const cancelDetailModelsEdit = useCallback(() => {
    if (!detailModelsEditing) {
      setDetailEditingModelKey('');
      setDetailEditingModelSnapshot(null);
      return;
    }
    if (detailEditingModelSnapshot) {
      setInputs((prev) =>
        buildNextInputsWithChannelModels(
          prev,
          visibleChannelModels.map((row) =>
            row.upstream_model === detailEditingModelKey
              ? { ...detailEditingModelSnapshot }
              : row,
          ),
          prev.protocol,
        ),
      );
    }
    setDetailEditingModelKey('');
    setDetailEditingModelSnapshot(null);
  }, [
    detailEditingModelKey,
    detailEditingModelSnapshot,
    detailModelsEditing,
    visibleChannelModels,
  ]);

  const handleFetchModels = useCallback(
    async ({ silent = false } = {}) => {
      if (!isDetailMode) {
        return false;
      }
      if (fetchingModelsRef.current) {
        return false;
      }
      fetchingModelsRef.current = true;
      setFetchModelsLoading(true);
      try {
        const targetChannelId = (channelId || '').toString().trim();
        if (targetChannelId === '') {
          return false;
        }
        const key = buildEffectiveKey().trim();
        const requestSignature = buildChannelConnectionSignature({
          protocol: inputs.protocol,
          key,
          baseURL: effectiveAPIBaseURL,
          channelID: targetChannelId,
        });
        const res = await API.post(
          `/api/v1/admin/channel/${targetChannelId}/refresh`,
        );
        const { success, message, data } = res.data || {};
        if (!success) {
          const errorMessage =
            message || t('channel.edit.messages.fetch_models_failed');
          setModelsSyncError(errorMessage);
          setVerifiedModelSignature('');
          if (!silent) {
            showError(errorMessage);
          }
          return false;
        }
        const refreshTask = normalizeAsyncTasks([data?.task])[0];
        if (!refreshTask?.id) {
          const errorMessage = t('channel.edit.messages.fetch_models_failed');
          setModelsSyncError(errorMessage);
          setVerifiedModelSignature('');
          if (!silent) {
            showError(errorMessage);
          }
          return false;
        }
        setChannelTasks((prev) =>
          normalizeAsyncTasks([...normalizeAsyncTasks(prev), refreshTask]),
        );
        pendingRefreshTaskIdRef.current = refreshTask.id;
        pendingRefreshSignatureRef.current = requestSignature;
        setModelsSyncError('');
        if (!silent) {
          showSuccess(t('channel.messages.operation_success'));
        }
        return true;
      } catch (error) {
        const errorMessage =
          error?.message || t('channel.edit.messages.fetch_models_failed');
        setModelsSyncError(errorMessage);
        setVerifiedModelSignature('');
        if (!silent) {
          showError(errorMessage);
        }
        return false;
      } finally {
        fetchingModelsRef.current = false;
        setFetchModelsLoading(false);
      }
    },
    [
      buildEffectiveKey,
      channelId,
      effectiveAPIBaseURL,
      inputs,
      inputs.protocol,
      isDetailMode,
      t,
    ],
  );

  const fetchChannelTypes = useCallback(async () => {
    const options = await loadChannelProtocolOptions();
    if (Array.isArray(options) && options.length > 0) {
      setChannelProtocolOptions(options);
    }
  }, []);

  const loadProviderIndex = useCallback(
    async ({ silent = true, force = false } = {}) => {
      if (providerDataLoading) {
        return null;
      }
      if (providerDataLoaded && !force && providerOptions.length > 0) {
        return {
          providerOptions,
          modelOwners: providerModelOwners,
          providerModelDetails: providerModelDetailsIndex,
        };
      }
      setProviderDataLoading(true);
      try {
        const items = [];
        let page = 0;
        let total = 0;
        while (page < 20) {
          const res = await API.get('/api/v1/admin/providers', {
            params: {
              page: page + 1,
              page_size: 100,
            },
          });
          const { success, message, data } = res.data || {};
          if (!success) {
            if (!silent) {
              showError(
                message ||
                  t('channel.edit.model_selector.provider_load_failed'),
              );
            }
            return null;
          }
          const pageItems = Array.isArray(data?.items) ? data.items : [];
          items.push(...pageItems);
          total = Number(data?.total || pageItems.length || 0);
          if (
            pageItems.length === 0 ||
            items.length >= total ||
            pageItems.length < 100
          ) {
            break;
          }
          page += 1;
        }
        const nextProviderIndex = buildProviderIndex(items);
        setProviderOptions(nextProviderIndex.providerOptions);
        setProviderModelOwners(nextProviderIndex.modelOwners);
        setProviderModelDetailsIndex(nextProviderIndex.providerModelDetails);
        setProviderDataLoaded(true);
        return nextProviderIndex;
      } catch (error) {
        if (!silent) {
          showError(
            error?.message ||
              t('channel.edit.model_selector.provider_load_failed'),
          );
        }
        return null;
      } finally {
        setProviderDataLoading(false);
      }
    },
    [
      providerDataLoaded,
      providerModelDetailsIndex,
      providerDataLoading,
      providerModelOwners,
      providerOptions,
      t,
    ],
  );

  const startDetailModelEdit = useCallback(
    (upstreamModel) => {
      const targetModel = (upstreamModel || '').toString().trim();
      if (targetModel === '') {
        return;
      }
      const currentRow =
        visibleChannelModels.find(
          (row) => row.upstream_model === targetModel,
        ) || null;
      if (!currentRow) {
        return;
      }
      if (
        providerOptions.length === 0 &&
        !providerDataLoaded &&
        !providerDataLoading
      ) {
        loadProviderIndex({ silent: true }).then();
      }
      setDetailEditingModelKey(targetModel);
      setDetailEditingModelSnapshot({ ...currentRow });
    },
    [
      loadProviderIndex,
      providerDataLoaded,
      providerDataLoading,
      providerOptions.length,
      visibleChannelModels,
    ],
  );

  const openAppendProviderModal = useCallback(
    async (row) => {
      const providerIndex = await loadProviderIndex({
        silent: false,
        force: true,
      });
      if (!providerIndex) {
        return;
      }
      if (providerIndex.providerOptions.length === 0) {
        showInfo(t('channel.edit.model_selector.provider_no_options'));
        return;
      }
      setAppendProviderForm({
        provider: inferAssignableProviderForRowWithOptions(
          row,
          providerIndex.providerOptions,
        ),
        model: (row?.upstream_model || row?.model || '').toString().trim(),
        type: normalizeChannelModelType(row?.type),
      });
      setAppendProviderModalOpen(true);
    },
    [loadProviderIndex, t],
  );

  const closeAppendProviderModal = useCallback(() => {
    if (appendingProviderModel) {
      return;
    }
    setAppendProviderModalOpen(false);
    setAppendProviderForm({
      provider: '',
      model: '',
      type: 'text',
    });
  }, [appendingProviderModel]);

  const handleAppendModelToProvider = useCallback(async () => {
    const providerId = (appendProviderForm.provider || '').toString().trim();
    const modelName = (appendProviderForm.model || '').toString().trim();
    if (providerId === '' || modelName === '') {
      showInfo(t('channel.edit.model_selector.provider_append_invalid'));
      return;
    }
    setAppendingProviderModel(true);
    try {
      const res = await API.post(
        `/api/v1/admin/providers/${providerId}/model`,
        {
          model: modelName,
          type: normalizeChannelModelType(appendProviderForm.type),
        },
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(
          message || t('channel.edit.model_selector.provider_append_failed'),
        );
        return;
      }
      await loadProviderIndex({ silent: true, force: true });
      showSuccess(t('channel.edit.model_selector.provider_append_success'));
      closeAppendProviderModal();
    } catch (error) {
      showError(
        error?.message ||
          t('channel.edit.model_selector.provider_append_failed'),
      );
    } finally {
      setAppendingProviderModel(false);
    }
  }, [
    appendProviderForm,
    closeAppendProviderModal,
    loadProviderIndex,
    t,
  ]);

  const handleRunModelTests = useCallback(
    async ({ targetModels = [], scope = 'batch' } = {}) => {
      if (!isDetailMode) {
        return;
      }
      if (detailTestingReadonly) {
        return;
      }
      if (inputs.protocol === 'proxy') {
        return;
      }
      const normalizedTargets = normalizeModelIDs(
        Array.isArray(targetModels) && targetModels.length > 0
          ? targetModels
          : modelTestTargetModels,
      );
      if (normalizedTargets.length === 0) {
        showInfo(t('channel.edit.messages.models_required'));
        return;
      }
      const targetChannelId = (channelId || '').toString().trim();
      if (targetChannelId === '') {
        return;
      }
      const targetConfigs = visibleChannelModels
        .filter((row) => normalizedTargets.includes(row.model))
        .map((row) => {
          const targetConfig = {
            model: row.model,
            endpoint: getEffectiveModelEndpoint(row),
          };
          if (supportsModelTestStream(row)) {
            targetConfig.is_stream = !!row.is_stream;
          }
          return targetConfig;
        });
      setModelTesting(true);
      setModelTestingScope(scope === 'single' ? 'single' : 'batch');
      setModelTestingTargets(normalizedTargets);
      try {
        const res = await API.post(
          `/api/v1/admin/channel/${targetChannelId}/tests`,
          {
            test_model: inputs.test_model || '',
            target_models: normalizedTargets,
            target_configs: targetConfigs,
            audio_language: audioTestLanguage,
          },
        );
        const { success, message, data, meta } = res.data || {};
        if (!success) {
          const errorMessage =
            message || t('channel.edit.model_tester.test_failed');
          setModelTestError(errorMessage);
          showError(errorMessage);
          return;
        }
        const nextTasks = normalizeAsyncTasks(data?.tasks);
        setChannelTasks((prev) =>
          normalizeAsyncTasks([...normalizeAsyncTasks(prev), ...nextTasks]),
        );
        setModelTestError('');
        showSuccess(
          t('channel.edit.model_tester.task_created', {
            count: Number(meta?.created || nextTasks.length || 0),
            reused: Number(meta?.reused || 0),
          }),
        );
      } catch (error) {
        const errorMessage =
          error?.message || t('channel.edit.model_tester.test_failed');
        setModelTestError(errorMessage);
        showError(errorMessage);
      } finally {
        setModelTesting(false);
        setModelTestingScope('');
        setModelTestingTargets([]);
      }
    },
    [
      channelId,
      modelTestTargetModels,
      getEffectiveModelEndpoint,
      inputs,
      inputs.protocol,
      inputs.test_model,
      detailTestingReadonly,
      isDetailMode,
      t,
      visibleChannelModels,
      audioTestLanguage,
    ],
  );

  const toggleModelTestTarget = useCallback((modelName, checked) => {
    if (detailTestingReadonly) {
      return;
    }
    setModelTestTargetModels((prev) => {
      const normalized = (modelName || '').toString().trim();
      if (normalized === '') {
        return prev;
      }
      if (checked) {
        return normalizeModelIDs([...prev, normalized]);
      }
      return prev.filter((item) => item !== normalized);
    });
  }, [detailTestingReadonly]);

  const toggleModelTestGroupTargets = useCallback(
    (rows, checked) => {
      if (detailTestingReadonly) {
        return;
      }
      const modelIDs = normalizeModelIDs(
        (Array.isArray(rows) ? rows : []).map((row) => row.model),
      );
      if (modelIDs.length === 0) {
        return;
      }
      setModelTestTargetModels((prev) => {
        if (checked) {
          return normalizeModelIDs([...prev, ...modelIDs]);
        }
        const removeSet = new Set(modelIDs);
        return prev.filter((item) => !removeSet.has(item));
      });
    },
    [detailTestingReadonly],
  );

  const updateModelTestEndpoint = useCallback(
    async (modelName, endpoint) => {
      if (detailTestingReadonly) {
        return;
      }
      const nextConfigs = visibleChannelModels.map((row) => {
        if (row.model !== modelName) {
          return row;
        }
        const nextEndpoint = normalizeChannelModelEndpoint(
          row.type,
          endpoint,
          inputs.protocol,
        );
        return {
          ...row,
          endpoint: nextEndpoint,
          endpoints: normalizeChannelModelEndpoints(
            row.type,
            row.endpoints,
            nextEndpoint,
            inputs.protocol,
          ),
        };
      });
      if (isDetailMode) {
        await persistDetailChannelModels(nextConfigs);
        return;
      }
      setInputs((prev) =>
        buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol),
      );
    },
    [
      detailTestingReadonly,
      isDetailMode,
      persistDetailChannelModels,
      visibleChannelModels,
    ],
  );
  const updateAllModelTestEndpoints = useCallback(
    async (endpoint, targetModels) => {
      if (detailTestingReadonly) {
        return;
      }
      const targetEndpoint = (endpoint || '').toString().trim();
      const targetSet = new Set(normalizeModelIDs(targetModels));
      if (targetEndpoint === '' || targetSet.size === 0) {
        return;
      }
      const nextConfigs = visibleChannelModels.map((row) => {
        if (!targetSet.has(row.model)) {
          return row;
        }
        const nextEndpoint = normalizeChannelModelEndpoint(
          row.type,
          targetEndpoint,
          inputs.protocol,
        );
        return {
          ...row,
          endpoint: nextEndpoint,
          endpoints: normalizeChannelModelEndpoints(
            row.type,
            row.endpoints,
            nextEndpoint,
            inputs.protocol,
          ),
        };
      });
      if (isDetailMode) {
        await persistDetailChannelModels(nextConfigs);
        return;
      }
      setInputs((prev) =>
        buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol),
      );
    },
    [
      detailTestingReadonly,
      inputs.protocol,
      isDetailMode,
      persistDetailChannelModels,
      visibleChannelModels,
    ],
  );
  const updateAllModelTestStreams = useCallback(
    async (isStream, modelNames = []) => {
      if (detailTestingReadonly) {
        return;
      }
      const targetSet = new Set(
        (Array.isArray(modelNames) ? modelNames : [])
          .map((item) => (item || '').toString().trim())
          .filter(Boolean),
      );
      if (targetSet.size === 0) {
        return;
      }
      const nextConfigs = visibleChannelModels.map((row) => {
        if (!targetSet.has(row.model) || !supportsModelTestStream(row)) {
          return row;
        }
        return { ...row, is_stream: !!isStream };
      });
      if (isDetailMode) {
        await persistDetailChannelModels(nextConfigs);
        return;
      }
      setInputs((prev) =>
        buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol),
      );
    },
    [
      detailTestingReadonly,
      isDetailMode,
      persistDetailChannelModels,
      visibleChannelModels,
    ],
  );

  const updateChannelEndpointCapability = useCallback(
    async (row, nextValues = {}, options = {}) => {
      const { skipConfirm = false } = options;
      if (!isDetailMode || endpointCapabilityReadonly) {
        return;
      }
      const targetChannelId = (row?.channel_id || channelId || '')
        .toString()
        .trim();
      const modelName = (row?.model || '').toString().trim();
      const endpoint = (row?.endpoint || '').toString().trim();
      if (targetChannelId === '' || modelName === '' || endpoint === '') {
        return;
      }
      const enabled =
        typeof nextValues.enabled === 'boolean'
          ? nextValues.enabled
          : row?.enabled === true;
      const baseURL = normalizeBaseURL(nextValues.base_url ?? row?.base_url);
      const endpointKey = buildChannelEndpointKey(modelName, endpoint);
      const latestResult = modelTestResultsByKey.get(endpointKey) || null;
      const hasSuccessfulTest =
        latestResult?.status === 'supported' && latestResult?.supported === true;
      if (enabled && !skipConfirm && !hasSuccessfulTest) {
        setPendingEndpointEnableRow(row);
        setEndpointEnableConfirmOpen(true);
        return;
      }
      setEndpointMutatingKey(endpointKey);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/endpoints`,
          {
            model: modelName,
            endpoint,
            base_url: baseURL,
            enabled: !!enabled,
          },
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(
            message || t('channel.edit.endpoint_capabilities.update_failed'),
          );
          return;
        }
        const nextEndpoints = await loadChannelEndpointsFromServer(
          targetChannelId,
        );
        setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
        setChannelEndpointsError('');
        showSuccess(
          t(
            enabled
              ? 'channel.edit.endpoint_capabilities.enable_success'
              : 'channel.edit.endpoint_capabilities.disable_success',
          ),
        );
      } catch (error) {
        showError(
          error?.message || t('channel.edit.endpoint_capabilities.update_failed'),
        );
      } finally {
        setEndpointMutatingKey('');
      }
    },
    [
      channelId,
      endpointCapabilityReadonly,
      isDetailMode,
      modelTestResultsByKey,
      loadChannelEndpointsFromServer,
      t,
    ],
  );

  const closeEndpointEnableConfirm = useCallback(() => {
    if (endpointEnableConfirmLoading) {
      return;
    }
    setEndpointEnableConfirmOpen(false);
    setPendingEndpointEnableRow(null);
  }, [endpointEnableConfirmLoading]);

  const confirmEnableEndpointWithoutSuccessfulTest = useCallback(async () => {
    if (!pendingEndpointEnableRow) {
      return;
    }
    setEndpointEnableConfirmLoading(true);
    try {
      await updateChannelEndpointCapability(
        pendingEndpointEnableRow,
        { enabled: true },
        {
          skipConfirm: true,
        },
      );
      setEndpointEnableConfirmOpen(false);
      setPendingEndpointEnableRow(null);
    } finally {
      setEndpointEnableConfirmLoading(false);
    }
  }, [pendingEndpointEnableRow, updateChannelEndpointCapability]);

  const openEndpointPolicyEditor = useCallback(
    (row) => {
      const targetChannelId = (row?.channel_id || channelId || '')
        .toString()
        .trim();
      const modelName = (row?.model || '').toString().trim();
      const endpoint = (row?.endpoint || '').toString().trim();
      if (targetChannelId === '' || modelName === '' || endpoint === '') {
        return;
      }
      const existingPolicy =
        channelEndpointPolicies.find(
          (item) => item.model === modelName && item.endpoint === endpoint,
        ) || null;
      setPolicyDraft(
        existingPolicy
          ? {
              ...existingPolicy,
              channel_id: targetChannelId,
              capabilities: prettyJSONString(existingPolicy.capabilities),
              request_policy: prettyJSONString(existingPolicy.request_policy),
              response_policy: prettyJSONString(existingPolicy.response_policy),
            }
          : buildEmptyEndpointPolicyDraft(targetChannelId, modelName, endpoint),
      );
      setSelectedPolicyTemplate(existingPolicy?.template_key || '');
      setPolicyEditorOpen(true);
    },
    [channelEndpointPolicies, channelId],
  );

  const closeEndpointPolicyEditor = useCallback(() => {
    if (policyEditorSaving) {
      return;
    }
    setPolicyEditorOpen(false);
    setSelectedPolicyTemplate('');
    setPolicyDraft(buildEmptyEndpointPolicyDraft('', '', ''));
  }, [policyEditorSaving]);

  const applyEndpointPolicyTemplate = useCallback((templateValue) => {
    const template = ENDPOINT_POLICY_TEMPLATES.find(
      (item) => item.value === templateValue,
    );
    if (!template) {
      return;
    }
    const patch = template.buildDraft();
    setPolicyDraft((prev) => ({
      ...prev,
      ...patch,
    }));
    setSelectedPolicyTemplate(templateValue);
  }, []);

  const saveEndpointPolicy = useCallback(async () => {
    if (policyEditorSaving) {
      return;
    }
    const targetChannelId = (policyDraft.channel_id || '').toString().trim();
    const modelName = (policyDraft.model || '').toString().trim();
    const endpoint = (policyDraft.endpoint || '').toString().trim();
    if (targetChannelId === '' || modelName === '' || endpoint === '') {
      showError(t('channel.edit.endpoint_policies.invalid'));
      return;
    }
    setPolicyEditorSaving(true);
    try {
      const res = await API.put(
        `/api/v1/admin/channel/${targetChannelId}/policies`,
        {
          id: (policyDraft.id || '').toString().trim(),
          model: modelName,
          endpoint,
          enabled: !!policyDraft.enabled,
          template_key: (policyDraft.template_key || '').toString().trim(),
          capabilities: (policyDraft.capabilities || '').toString().trim(),
          request_policy: (policyDraft.request_policy || '')
            .toString()
            .trim(),
          response_policy: (policyDraft.response_policy || '')
            .toString()
            .trim(),
          reason: (policyDraft.reason || '').toString(),
          source: 'manual',
          last_verified_at: Number(policyDraft.last_verified_at || 0),
        },
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('channel.edit.endpoint_policies.update_failed'));
        return;
      }
      const nextPolicies = await loadChannelEndpointPoliciesFromServer(
        targetChannelId,
      );
      setChannelEndpointPolicies(
        normalizeChannelEndpointPolicyRows(nextPolicies),
      );
      setChannelEndpointPoliciesError('');
      showSuccess(t('channel.edit.endpoint_policies.update_success'));
      closeEndpointPolicyEditor();
    } catch (error) {
      showError(
        error?.message || t('channel.edit.endpoint_policies.update_failed'),
      );
    } finally {
      setPolicyEditorSaving(false);
    }
  }, [
    closeEndpointPolicyEditor,
    loadChannelEndpointPoliciesFromServer,
    policyDraft,
    policyEditorSaving,
    t,
  ]);

  const toggleModelSelection = useCallback(
    async (upstreamModel, checked) => {
      const nextConfigs = visibleChannelModels.map((row) =>
        row.upstream_model === upstreamModel && canSelectChannelModel(row)
          ? {
              ...row,
              selected: !!checked,
            }
          : row,
      );
      if (isDetailMode) {
        if (
          detailModelsEditing &&
          detailEditingModelKey === (upstreamModel || '').toString().trim()
        ) {
          setInputs((prev) =>
            buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol),
          );
          return;
        }
        await persistDetailChannelModels(nextConfigs);
        return;
      }
      setInputs((prev) =>
        buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol),
      );
    },
    [
      canSelectChannelModel,
      detailEditingModelKey,
      detailModelsEditing,
      isDetailMode,
      persistDetailChannelModels,
      visibleChannelModels,
    ],
  );
  const toggleDetailCurrentPageSelections = useCallback(
    async (checked) => {
      if (checked && detailCurrentPageBlockedModels.length > 0) {
        showInfo(
          t('channel.edit.model_selector.selection_skipped_unassigned', {
            count: detailCurrentPageBlockedModels.length,
          }),
        );
      }
      const targetIDs = new Set(
        detailCurrentPageSelectableModels.map((row) => row.upstream_model),
      );
      if (targetIDs.size === 0) {
        return;
      }
      const nextConfigs = visibleChannelModels.map((row) =>
        targetIDs.has(row.upstream_model)
          ? {
              ...row,
              selected: !!checked,
            }
          : row,
      );
      await persistDetailChannelModels(nextConfigs);
    },
    [
      detailCurrentPageBlockedModels.length,
      detailCurrentPageSelectableModels,
      persistDetailChannelModels,
      t,
      visibleChannelModels,
    ],
  );

  const handleDeleteDetailModel = useCallback(
    async (row) => {
      if (!isDetailMode || detailModelMutating || detailModelsEditing) {
        return;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const modelName = (row?.model || '').toString().trim();
      const upstreamModel = (row?.upstream_model || '').toString().trim();
      if (targetChannelId === '' || (modelName === '' && upstreamModel === '')) {
        return;
      }
      setDetailModelMutating(true);
      try {
        const res = await API.delete(
          `/api/v1/admin/channel/${targetChannelId}/models`,
          {
            params: {
              model: modelName,
              upstream_model: upstreamModel,
            },
          },
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(message || t('channel.edit.model_selector.delete_failed'));
          return;
        }
        await refreshChannelRuntimeState(targetChannelId);
        setDetailEditingModelKey('');
        setDetailEditingModelSnapshot(null);
        showSuccess(t('channel.edit.model_selector.delete_success'));
      } catch (error) {
        showError(
          error?.message || t('channel.edit.model_selector.delete_failed'),
        );
      } finally {
        setDetailModelMutating(false);
      }
    },
    [
      channelId,
      detailModelMutating,
      detailModelsEditing,
      isDetailMode,
      refreshChannelRuntimeState,
      t,
    ],
  );

  const updateModelConfigField = useCallback(
    (upstreamModel, field, value) => {
      const targetModel = (upstreamModel || '').toString().trim();
      if (
        isDetailMode &&
        (!detailModelsEditing || detailEditingModelKey !== targetModel)
      ) {
        return;
      }
      setInputs((prev) =>
        buildNextInputsWithChannelModels(
          prev,
          visibleChannelModels.map((row) => {
            if (row.upstream_model !== targetModel) {
              return row;
            }
            if (field === 'model') {
              const alias = (value || '').toString().trim();
              const targetAlias = alias || row.upstream_model;
              const duplicated = visibleChannelModels.some(
                (item) =>
                  item.upstream_model !== targetModel &&
                  item.model === targetAlias,
              );
              if (duplicated) {
                return row;
              }
              return {
                ...row,
                model: targetAlias,
              };
            }
            if (field === 'input_price' || field === 'output_price') {
              return {
                ...row,
                [field]: normalizePriceOverrideValue(value),
              };
            }
            if (field === 'price_unit') {
              return {
                ...row,
                price_unit: normalizePriceUnitValue(value),
              };
            }
            if (field === 'price_components') {
              return {
                ...row,
                price_components: normalizeComplexPriceComponents(value),
              };
            }
            if (field === 'provider') {
              return {
                ...row,
                provider: normalizeChannelModelProviderValue(value),
              };
            }
            if (field === 'endpoint') {
              const nextEndpoint = normalizeChannelModelEndpoint(
                row.type,
                value,
                prev.protocol,
              );
              const nextEndpoints = normalizeChannelModelEndpoints(
                row.type,
                row.endpoints,
                nextEndpoint,
                prev.protocol,
              );
              return {
                ...row,
                endpoint: nextEndpoint,
                endpoints: nextEndpoints,
              };
            }
            if (field === 'endpoints') {
              const nextEndpoints = normalizeChannelModelEndpoints(
                row.type,
                Array.isArray(value) ? value : [],
                row.endpoint,
                prev.protocol,
              );
              const nextEndpoint = nextEndpoints.includes(row.endpoint)
                ? row.endpoint
                : nextEndpoints[0];
              return {
                ...row,
                endpoint: nextEndpoint,
                endpoints: nextEndpoints,
              };
            }
            return {
              ...row,
              [field]: value,
            };
          }),
          prev.protocol,
        ),
      );
    },
    [detailEditingModelKey, detailModelsEditing, isDetailMode, visibleChannelModels],
  );

  useEffect(() => {
    const selectedModels = visibleChannelModels
      .filter((row) => row.selected)
      .map((row) => row.model);
    const currentTestModel = (inputs.test_model || '').toString().trim();
    if (currentTestModel === '' || selectedModels.includes(currentTestModel)) {
      return;
    }
    setInputs((prev) => ({
      ...prev,
      test_model: selectedModels[0] || '',
    }));
  }, [inputs.test_model, visibleChannelModels]);

  useEffect(() => {
    if (!isDetailMode) {
      setDetailBasicEditing(false);
      setDetailEditingModelKey('');
      setDetailEditingModelSnapshot(null);
    }
  }, [isDetailMode]);

  useEffect(() => {
    if (hasChannelID) {
      setLoading(true);
      loadChannelById(channelId, false, false).then();
      return;
    }
    if (copyFromId !== '') {
      setLoading(true);
      loadChannelById(copyFromId, true, false).then();
      return;
    }
    setChannelKeySet(false);
    setConfig(CHANNEL_DEFAULT_CONFIG);
    setLoading(false);
  }, [
    channelId,
    copyFromId,
    hasChannelID,
    loadChannelById,
  ]);

  useEffect(() => {
    if (!isDetailMode || !channelId) {
      setChannelEndpoints([]);
      setChannelEndpointsError('');
      setChannelEndpointsLoading(false);
      return undefined;
    }
    let disposed = false;
    setChannelEndpointsLoading(true);
    loadChannelEndpointsFromServer(channelId)
      .then((items) => {
        if (disposed) {
          return;
        }
        setChannelEndpoints(normalizeChannelEndpointRows(items));
        setChannelEndpointsError('');
      })
      .catch((error) => {
        if (disposed) {
          return;
        }
        setChannelEndpoints([]);
        setChannelEndpointsError(
          error?.message || t('channel.edit.endpoint_capabilities.load_failed'),
        );
      })
      .finally(() => {
        if (disposed) {
          return;
        }
        setChannelEndpointsLoading(false);
      });
    return () => {
      disposed = true;
    };
  }, [channelId, isDetailMode, loadChannelEndpointsFromServer, t]);

  useEffect(() => {
    if (!isDetailMode || !channelId) {
      setChannelEndpointPolicies([]);
      setChannelEndpointPoliciesError('');
      setChannelEndpointPoliciesLoading(false);
      return undefined;
    }
    let disposed = false;
    setChannelEndpointPoliciesLoading(true);
    loadChannelEndpointPoliciesFromServer(channelId)
      .then((items) => {
        if (disposed) {
          return;
        }
        setChannelEndpointPolicies(normalizeChannelEndpointPolicyRows(items));
        setChannelEndpointPoliciesError('');
      })
      .catch((error) => {
        if (disposed) {
          return;
        }
        setChannelEndpointPolicies([]);
        setChannelEndpointPoliciesError(
          error?.message || t('channel.edit.endpoint_policies.load_failed'),
        );
      })
      .finally(() => {
        if (disposed) {
          return;
        }
        setChannelEndpointPoliciesLoading(false);
      });
    return () => {
      disposed = true;
    };
  }, [channelId, isDetailMode, loadChannelEndpointPoliciesFromServer, t]);

  useEffect(() => {
    const targetChannelId = ((hasChannelID ? channelId : '') || '')
      .toString()
      .trim();
    if (targetChannelId === '') {
      return undefined;
    }
    const hasActiveTasks = channelTasks.some((item) =>
      isActiveAsyncTaskStatus(item?.status),
    );
    if (!hasActiveTasks) {
      return undefined;
    }
    const timer = window.setInterval(async () => {
      try {
        const nextTasks = await loadChannelTasksFromServer(targetChannelId);
        const stillActive = nextTasks.some((item) =>
          isActiveAsyncTaskStatus(item?.status),
        );
        setChannelTasks(normalizeAsyncTasks(nextTasks));
        if (stillActive) {
          try {
            const nextTests = await loadChannelTestsFromServer(targetChannelId);
            if (Array.isArray(nextTests?.items) && nextTests.items.length > 0) {
              setModelTestResults(normalizeModelTestResults(nextTests.items));
            }
            const nextLastTestedAt = Number(nextTests?.lastTestedAt || 0);
            if (nextLastTestedAt > 0) {
              setModelTestedAt(nextLastTestedAt * 1000);
            }
          } catch {
            // keep polling tasks; test results will be retried on next tick
          }
          return;
        }
        if (!stillActive) {
          const refreshTaskId = pendingRefreshTaskIdRef.current;
          let completedRefreshTask = null;
          if (refreshTaskId !== '') {
            try {
              completedRefreshTask = await fetchTaskById(refreshTaskId);
            } catch {
              completedRefreshTask = null;
            }
          }
          await refreshChannelRuntimeState(targetChannelId);
          if (refreshTaskId !== '') {
            pendingRefreshTaskIdRef.current = '';
            if (
              completedRefreshTask &&
              normalizeAsyncTaskStatus(completedRefreshTask.status) ===
                'succeeded'
            ) {
              setModelsSyncError('');
              setModelsLastSyncedAt(Date.now());
              if (pendingRefreshSignatureRef.current !== '') {
                setVerifiedModelSignature(pendingRefreshSignatureRef.current);
              }
            } else {
              setVerifiedModelSignature('');
              setModelsSyncError(
                completedRefreshTask?.error_message ||
                  t('channel.edit.messages.fetch_models_failed'),
              );
            }
            pendingRefreshSignatureRef.current = '';
          }
        }
      } catch {
        // keep current local state and retry on next tick
      }
    }, 2500);
    return () => window.clearInterval(timer);
  }, [
    channelId,
    channelTasks,
    hasChannelID,
    loadChannelTasksFromServer,
    loadChannelTestsFromServer,
    refreshChannelRuntimeState,
  ]);

  useEffect(() => {
    if (!requiresConnectionVerification) {
      return;
    }
    if (verifiedModelSignature === '') {
      return;
    }
    if (verifiedModelSignature === currentModelSignature) {
      return;
    }
    setModelsLastSyncedAt(0);
    setModelsSyncError(t('channel.edit.model_selector.verify_stale'));
  }, [
    currentModelSignature,
    requiresConnectionVerification,
    t,
    verifiedModelSignature,
  ]);

  useEffect(() => {
    if (requiresConnectionVerification) {
      return;
    }
    if (verifiedModelSignature === '') {
      return;
    }
    setVerifiedModelSignature('');
  }, [requiresConnectionVerification, verifiedModelSignature]);

  useEffect(() => {
    fetchChannelTypes().then();
  }, [fetchChannelTypes]);

  useEffect(() => {
    if (
      !showStepTwo &&
      !showDetailTestsTab
    ) {
      return;
    }
    loadProviderIndex({ silent: true }).then();
  }, [loadProviderIndex, showDetailTestsTab, showStepTwo]);

  useEffect(() => {
    if (detailModelPage <= detailModelTotalPages) {
      return;
    }
    setDetailModelPage(detailModelTotalPages);
  }, [detailModelPage, detailModelTotalPages]);

  useEffect(() => {
    setDetailModelPage(1);
  }, [detailModelFilter, modelSearchKeyword]);

  useEffect(() => {
    if (modelTestRows.length === 0) {
      setModelTestTargetModels([]);
      return;
    }
    setModelTestTargetModels((prev) => {
      const available = modelTestRows.map((row) => row.model);
      return prev.filter((item) => available.includes(item));
    });
  }, [modelTestRows]);

  const submit = async () => {
    const effectiveKey = buildEffectiveKey();
    const identifierError = validateChannelIdentifier(inputs.name, t);
    if (!isDetailMode && identifierError !== '') {
      showInfo(identifierError);
      return;
    }
    if (isCreateMode && effectiveKey.trim() === '') {
      showInfo(t('channel.edit.messages.key_required'));
      return;
    }
    let localInputs = buildChannelPayload();
    const res = await API.post(`/api/v1/admin/channel/`, localInputs);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(t('channel.edit.messages.create_success'));
      const targetChannelID = (
        data?.id ||
        localInputs.id ||
        ''
      ).toString().trim();
      if (targetChannelID !== '') {
        navigate(`/admin/channel/detail/${targetChannelID}`, {
          replace: true,
        });
        return;
      }
      navigate('/admin/channel', { replace: true });
      return;
    } else {
      showError(message);
    }
  };

  const renderCreateStepNavigation = () => {
    return null;
  };

  const renderConnectionFields = () => {
    return keyField || null;
  };

  const renderAddressRoutingFields = () => {
    return (
      <>
        <AppFormRow>
          <AppField label={t('channel.edit.api_base_url')}>
            <AppInput
              className='router-section-input'
              name='api_base_url'
              placeholder={t('channel.edit.api_base_url_placeholder')}
              onChange={handleConfigChange}
              value={config.api_base_url || ''}
              autoComplete='new-password'
              {...inputReadonlyProps}
            />
          </AppField>
          <AppField label={t('channel.edit.account_base_url')}>
            <AppInput
              className='router-section-input'
              name='account_base_url'
              placeholder={t('channel.edit.account_base_url_placeholder')}
              onChange={handleConfigChange}
              value={config.account_base_url || ''}
              autoComplete='new-password'
              {...inputReadonlyProps}
            />
          </AppField>
        </AppFormRow>
        <div className='router-form-hint router-form-hint-tight'>
          {t('channel.edit.address_routing_hint')}
        </div>
      </>
    );
  };

  const renderProtocolSpecificFields = () => {
    return (
      <>
        {inputs.protocol === 'azure' && (
          <>
            <AppAlert
              type='info'
              showIcon
              className='router-section-message'
              title={
                <span>
              注意，<strong>模型部署名称必须和模型名称保持一致</strong>
              ，因为 Router 会把请求体中的 model
              参数替换为你的部署名称（模型名称中的点会被剔除），
              <a
                target='_blank'
                rel='noreferrer'
                href='https://github.com/yeying-community/router/issues/133?notification_referrer_id=NT_kwDOAmJSYrM2NjIwMzI3NDgyOjM5OTk4MDUw#issuecomment-1571602271'
              >
                图片演示
              </a>
              。
                </span>
              }
            />
            <AppFormRow>
              <AppField label='默认 API 版本'>
                <AppInput
                  className='router-section-input'
                  name='other'
                  placeholder='请输入默认 API 版本，例如：2024-03-01-preview，该配置可以被实际的请求查询参数所覆盖'
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                  {...inputReadonlyProps}
                />
              </AppField>
            </AppFormRow>
          </>
        )}
        {inputs.protocol === 'xunfei' && (
          <AppFormRow>
            <AppField label={t('channel.edit.spark_version')}>
              <AppInput
                className='router-section-input'
                name='other'
                placeholder={t('channel.edit.spark_version_placeholder')}
                onChange={handleInputChange}
                value={inputs.other}
                autoComplete='new-password'
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
        {inputs.protocol === 'aiproxy-library' && (
          <AppFormRow>
            <AppField label={t('channel.edit.knowledge_id')}>
              <AppInput
                className='router-section-input'
                name='other'
                placeholder={t('channel.edit.knowledge_id_placeholder')}
                onChange={handleInputChange}
                value={inputs.other}
                autoComplete='new-password'
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
        {inputs.protocol === 'ali' && (
          <AppFormRow>
            <AppField label={t('channel.edit.plugin_param')}>
              <AppInput
                className='router-section-input'
                name='other'
                placeholder={t('channel.edit.plugin_param_placeholder')}
                onChange={handleInputChange}
                value={inputs.other}
                autoComplete='new-password'
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
        {inputs.protocol === 'coze' && (
          <AppAlert
            type='info'
            showIcon
            className='router-section-message'
            title={t('channel.edit.coze_notice')}
          />
        )}
        {inputs.protocol === 'doubao' && (
          <AppAlert
            type='info'
            showIcon
            className='router-section-message'
            title={
              <span>
                {t('channel.edit.douban_notice')}
                <a
                  target='_blank'
                  rel='noreferrer'
                  href='https://console.volcengine.com/ark/region:ark+cn-beijing/endpoint'
                >
                  {t('channel.edit.douban_notice_link')}
                </a>
                {t('channel.edit.douban_notice_2')}
              </span>
            }
          />
        )}
        {inputs.protocol === 'awsclaude' && (
          <AppFormRow>
            <AppField label='Region' required>
              <AppInput
                className='router-section-input'
                name='region'
                required
                placeholder={t('channel.edit.aws_region_placeholder')}
                onChange={handleConfigChange}
                value={config.region}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
            <AppField label='AK' required>
              <AppInput
                className='router-section-input'
                name='ak'
                required
                placeholder={t('channel.edit.aws_ak_placeholder')}
                onChange={handleConfigChange}
                value={config.ak}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
            <AppField label='SK' required>
              <AppInput
                className='router-section-input'
                name='sk'
                required
                placeholder={t('channel.edit.aws_sk_placeholder')}
                onChange={handleConfigChange}
                value={config.sk}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
        {inputs.protocol === 'vertexai' && (
          <AppFormRow>
            <AppField label='Region' required>
              <AppInput
                className='router-section-input'
                name='region'
                required
                placeholder={t('channel.edit.vertex_region_placeholder')}
                onChange={handleConfigChange}
                value={config.region}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
            <AppField label={t('channel.edit.vertex_project_id')} required>
              <AppInput
                className='router-section-input'
                name='vertex_ai_project_id'
                required
                placeholder={t('channel.edit.vertex_project_id_placeholder')}
                onChange={handleConfigChange}
                value={config.vertex_ai_project_id}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
            <AppField label={t('channel.edit.vertex_credentials')} required>
              <AppInput
                className='router-section-input'
                name='vertex_ai_adc'
                required
                placeholder={t('channel.edit.vertex_credentials_placeholder')}
                onChange={handleConfigChange}
                value={config.vertex_ai_adc}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
        {inputs.protocol === 'coze' && (
          <AppFormRow>
            <AppField label={t('channel.edit.user_id')} required>
              <AppInput
                className='router-section-input'
                name='user_id'
                required
                placeholder={t('channel.edit.user_id_placeholder')}
                onChange={handleConfigChange}
                value={config.user_id}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
        {inputs.protocol === 'cloudflare' && (
          <AppFormRow>
            <AppField label='Account ID' required>
              <AppInput
                className='router-section-input'
                name='user_id'
                required
                placeholder='请输入 Account ID，例如：d8d7c61dbc334c32d3ced580e4bf42b4'
                onChange={handleConfigChange}
                value={config.user_id}
                autoComplete=''
                {...inputReadonlyProps}
              />
            </AppField>
          </AppFormRow>
        )}
      </>
    );
  };

  const renderBasicInfoSection = () => {
    if (!showStepOne) {
      return null;
    }
    return !isDetailMode ? (
      <>
        <AppFormRow>
          <AppField label={t('channel.edit.identifier')} required>
            <AppInput
              className='router-section-input'
              name='name'
              placeholder={t('channel.edit.identifier_placeholder')}
              onChange={handleInputChange}
              value={inputs.name}
              required
              maxLength={CHANNEL_IDENTIFIER_MAX_LENGTH}
              readOnly={detailBasicReadonly}
            />
          </AppField>
          <AppField label={t('channel.edit.type')}>
            {detailBasicReadonly ? (
              <AppInput
                className='router-section-input'
                value={currentProtocolOption?.text || inputs.protocol || '-'}
                readOnly
              />
            ) : (
              <AppSelect
                className='router-section-dropdown'
                name='protocol'
                required
                search
                options={channelProtocolOptions}
                value={inputs.protocol}
                onChange={handleInputChange}
              />
            )}
          </AppField>
        </AppFormRow>
        {!detailBasicReadonly && (
          <div className='router-form-hint router-form-hint-section'>
            {protocolSelectionHint(t)}
          </div>
        )}
        {renderConnectionFields()}
        {renderAddressRoutingFields()}
        {renderProtocolSpecificFields()}
      </>
    ) : null;
  };

  return (
    <div className='dashboard-container'>
      <ChannelModelEditorModal
        t={t}
        open={detailModelsEditing}
        onClose={cancelDetailModelsEdit}
        detailModelMutating={detailModelMutating}
        detailEditingModelRow={detailEditingModelRow}
        normalizeChannelModelType={normalizeChannelModelType}
        updateModelConfigField={updateModelConfigField}
        providerDataLoading={providerDataLoading}
        getProviderSelectOptionsForModel={getProviderSelectOptionsForModel}
        resolvePreferredProviderForModel={resolvePreferredProviderForModel}
        openAppendProviderModal={openAppendProviderModal}
        canSelectChannelModel={canSelectChannelModel}
        toggleModelSelection={toggleModelSelection}
        getComplexPricingDetailsForModel={getComplexPricingDetailsForModel}
        saveDetailModelsConfig={saveDetailModelsConfig}
      />
      <ChannelComplexPricingModal
        t={t}
        open={complexPricingModalOpen}
        onClose={closeComplexPricingModal}
        data={complexPricingModalData}
        normalizeChannelModelType={normalizeChannelModelType}
      />
      <ChannelEndpointPolicyEditorModal
        t={t}
        open={policyEditorOpen}
        onClose={closeEndpointPolicyEditor}
        policyEditorSaving={policyEditorSaving}
        endpointPolicyTemplates={ENDPOINT_POLICY_TEMPLATES}
        selectedPolicyTemplate={selectedPolicyTemplate}
        setSelectedPolicyTemplate={setSelectedPolicyTemplate}
        applyEndpointPolicyTemplate={applyEndpointPolicyTemplate}
        policyDraft={policyDraft}
        setPolicyDraft={setPolicyDraft}
        saveEndpointPolicy={saveEndpointPolicy}
      />
      <AppModal
        size='tiny'
        open={endpointEnableConfirmOpen}
        onClose={closeEndpointEnableConfirm}
        title={t('channel.edit.endpoint_capabilities.enable_confirm_title')}
        footer={
          <AppFormActions>
            <AppButton
              type='button'
              onClick={closeEndpointEnableConfirm}
              disabled={endpointEnableConfirmLoading}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              color='blue'
              loading={endpointEnableConfirmLoading}
              disabled={endpointEnableConfirmLoading}
              onClick={confirmEnableEndpointWithoutSuccessfulTest}
            >
              {t('channel.edit.endpoint_capabilities.enable_confirm_action')}
            </AppButton>
          </AppFormActions>
        }
      >
        <div>
          <p>{t('channel.edit.endpoint_capabilities.enable_confirm_content')}</p>
          <p className='router-muted-text'>
            {pendingEndpointEnableRow
              ? t('channel.edit.endpoint_capabilities.enable_confirm_target', {
                  model: pendingEndpointEnableRow.model || '-',
                  endpoint: pendingEndpointEnableRow.endpoint || '-',
                })
              : ''}
          </p>
        </div>
      </AppModal>
      <ChannelAppendProviderModal
        t={t}
        open={appendProviderModalOpen}
        onClose={closeAppendProviderModal}
        appendingProviderModel={appendingProviderModel}
        filterProviderOptionsByQuery={filterProviderOptionsByQuery}
        providerOptions={providerOptions}
        appendProviderForm={appendProviderForm}
        setAppendProviderForm={setAppendProviderForm}
        channelModelTypeOptions={CHANNEL_MODEL_TYPE_OPTIONS}
        normalizeChannelModelType={normalizeChannelModelType}
        handleAppendModelToProvider={handleAppendModelToProvider}
      />
      {isDetailMode ? (
        <AppFilterHeader
          breadcrumbs={[
            { key: 'admin', label: t('header.admin_workspace') },
            { key: 'resource', label: t('header.resource') },
            {
              key: 'channel-list',
              label: t('header.channel'),
              onClick: handleBackToChannelList,
            },
            {
              key: 'channel-current',
              label: detailChannelLabel || t('channel.edit.title_detail'),
              active: true,
            },
          ]}
          title={inputs.name || t('channel.edit.title_detail')}
        />
      ) : null}
      <div
        className={
          isDetailMode
            ? 'router-tab-detail-page router-entity-detail-page'
            : 'router-tab-detail-page'
        }
      >
          {isDetailMode && (
            <div className='router-entity-detail-tabs router-block-gap-sm'>
              <AppTabs
                className='router-detail-tab-menu'
                activeKey={activeDetailTab}
                items={detailTabItems}
                onChange={goToDetailTab}
              />
            </div>
          )}
          {isCreateMode && (
            <AppFormActions align='start' className='router-block-gap-sm'>
              <AppButton
                className='router-page-button'
                onClick={handleCancel}
              >
                {t('channel.edit.buttons.cancel')}
              </AppButton>
              <AppButton
                className='router-page-button'
                color='blue'
                onClick={submit}
              >
                {t('channel.edit.buttons.submit')}
              </AppButton>
            </AppFormActions>
          )}
          <AppSpin spinning={loading}>
            <div>
            {renderCreateStepNavigation()}
            {renderBasicInfoSection()}
            {showDetailOverviewTab && showStepOne && (
              <ChannelDetailOverviewTab
                t={t}
                inputs={inputs}
                currentProtocolOption={currentProtocolOption}
                channelProtocolOptions={channelProtocolOptions}
                detailBasicEditing={detailBasicEditing}
                detailBasicSaving={detailBasicSaving}
                detailBasicEditLocked={detailBasicEditLocked}
                detailBasicReadonly={detailBasicReadonly}
                channelIdentifierMaxLength={CHANNEL_IDENTIFIER_MAX_LENGTH}
                handleInputChange={handleInputChange}
                cancelDetailBasicEdit={cancelDetailBasicEdit}
                saveDetailBasicInfo={saveDetailBasicInfo}
                setDetailBasicEditing={setDetailBasicEditing}
                basicConnectionFields={renderConnectionFields()}
                addressRoutingFields={renderAddressRoutingFields()}
                protocolSelectionHintContent={
                  !detailBasicReadonly ? (
                    <div className='router-form-hint router-form-hint-section'>
                      {protocolSelectionHint(t)}
                    </div>
                  ) : null
                }
                protocolSpecificFields={renderProtocolSpecificFields()}
                timestamp2string={timestamp2string}
              />
            )}
            {showStepTwo && inputs.protocol !== 'proxy' && (
              <>
                {showDetailModelsTab && (
                  <ChannelDetailModelsTab
                    t={t}
                    columnWidths={CHANNEL_DETAIL_MODEL_COLUMN_WIDTHS}
                    modelSectionMetaText={modelSectionMetaText}
                    detailModelFilter={detailModelFilter}
                    setDetailModelFilter={setDetailModelFilter}
                    detailModelsEditing={detailModelsEditing}
                    modelSearchKeyword={modelSearchKeyword}
                    setModelSearchKeyword={setModelSearchKeyword}
                    fetchModelsLoading={fetchModelsLoading}
                    activeRefreshModelsTask={activeRefreshModelsTask}
                    detailModelMutating={detailModelMutating}
                    handleFetchModels={handleFetchModels}
                    searchedChannelModels={searchedChannelModels}
                    visibleChannelModels={visibleChannelModels}
                    renderedChannelModels={renderedChannelModels}
                    getComplexPricingDetailsForModel={
                      getComplexPricingDetailsForModel
                    }
                    openComplexPricingModal={openComplexPricingModal}
                    detailModelsEditLocked={detailModelsEditLocked}
                    providerDataLoading={providerDataLoading}
                    toggleModelSelection={toggleModelSelection}
                    canSelectChannelModel={canSelectChannelModel}
                    detailCurrentPageAllSelected={
                      detailCurrentPageAllSelected
                    }
                    detailCurrentPagePartiallySelected={
                      detailCurrentPagePartiallySelected
                    }
                    detailCurrentPageSelectableCount={
                      detailCurrentPageSelectableModels.length
                    }
                    toggleDetailCurrentPageSelections={
                      toggleDetailCurrentPageSelections
                    }
                    normalizeChannelModelType={normalizeChannelModelType}
                    startDetailModelEdit={startDetailModelEdit}
                    handleDeleteDetailModel={handleDeleteDetailModel}
                    detailModelTotalPages={detailModelTotalPages}
                    detailModelPage={detailModelPage}
                    setDetailModelPage={setDetailModelPage}
                    modelsSyncError={modelsSyncError}
                  />
                )}
                {showDetailEndpointsTab && (
                  <ChannelDetailEndpointsTab
                    t={t}
                    columnWidths={CHANNEL_ENDPOINT_COLUMN_WIDTHS}
                    endpointSummaryText={endpointSummaryText}
                    channelEndpoints={channelEndpoints}
                    channelEndpointsLoading={channelEndpointsLoading}
                    channelEndpointsError={channelEndpointsError}
                    buildChannelEndpointKey={buildChannelEndpointKey}
                    modelTestResultsByKey={modelTestResultsByKey}
                    endpointCapabilityReadonly={endpointCapabilityReadonly}
                    endpointMutatingKey={endpointMutatingKey}
                    updateChannelEndpointCapability={
                      updateChannelEndpointCapability
                    }
                    channelEndpointPoliciesLoading={
                      channelEndpointPoliciesLoading
                    }
                    channelEndpointPolicies={channelEndpointPolicies}
                    channelEndpointPoliciesError={
                      channelEndpointPoliciesError
                    }
                    endpointPolicyReadonly={endpointPolicyReadonly}
                    openEndpointPolicyEditor={openEndpointPolicyEditor}
                    timestamp2string={timestamp2string}
                  />
                )}
              </>
            )}
            {showDetailTestsTab && (
              <ChannelDetailTestsTab
                t={t}
                channelId={channelId}
                inputs={inputs}
                columnWidths={CHANNEL_MODEL_TEST_GROUP_COLUMN_WIDTHS}
                modelTestResults={modelTestResults}
                modelTestRows={modelTestRows}
                modelTestTargetModels={modelTestTargetModels}
                detailModelMutating={detailModelMutating}
                toggleModelTestTarget={toggleModelTestTarget}
                getEffectiveModelEndpoint={getEffectiveModelEndpoint}
                modelTestResultsByKey={modelTestResultsByKey}
                buildModelTestResultKey={buildModelTestResultKey}
                activeChannelTasksByModel={activeChannelTasksByModel}
                getEndpointOptionsForModel={getEndpointOptionsForModel}
                updateModelTestEndpoint={updateModelTestEndpoint}
                modelTesting={modelTesting}
                modelTestingScope={modelTestingScope}
                modelTestingTargetSet={modelTestingTargetSet}
                handleRunModelTests={handleRunModelTests}
                detailTestingReadonly={detailTestingReadonly}
                modelTestError={modelTestError}
                openChannelTaskView={openChannelTaskView}
                selectedModelTestHasActiveTasks={
                  selectedModelTestHasActiveTasks
                }
                timestamp2string={timestamp2string}
                updateAllModelTestEndpoints={updateAllModelTestEndpoints}
                updateAllModelTestStreams={updateAllModelTestStreams}
                resolvePreferredProviderForModel={
                  resolvePreferredProviderForModel
                }
                normalizeChannelModelType={normalizeChannelModelType}
                audioTestLanguage={audioTestLanguage}
                setAudioTestLanguage={setAudioTestLanguage}
              />
            )}
            </div>
          </AppSpin>
      </div>
    </div>
  );
};

export default ChannelForm;
