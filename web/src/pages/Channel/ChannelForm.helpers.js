// Pure logic helpers, constants and API-fetch wrappers extracted from
// ChannelForm.jsx to shrink the container. No JSX, no React, no component
// state here; the only external dependency is the API client.
import { API } from '../../helpers';

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

const normalizeChannelBillingSourceValue = (source) => {
  const normalizedSource = (source || '').toString().trim().toLowerCase();
  return normalizedSource === '' ||
    normalizedSource === 'unsupported' ||
    normalizedSource.startsWith('builtin_')
    ? 'manual'
    : normalizedSource;
};

const resolveChannelBillingSourceValue = (source, adapters = []) => {
  const normalizedSource = normalizeChannelBillingSourceValue(source);
  if (normalizedSource === 'manual') {
    return normalizedSource;
  }
  const adapterNames = new Set(
    (Array.isArray(adapters) ? adapters : [])
      .map((adapter) => (adapter?.name || '').toString().trim().toLowerCase())
      .filter(Boolean)
  );
  return adapterNames.has(normalizedSource) ? normalizedSource : 'manual';
};

const normalizeBillingCredentialFieldName = (name) =>
  (name || '').toString().trim().toLowerCase();

const normalizeBillingCredentialFields = (fields) => {
  if (!Array.isArray(fields)) {
    return [];
  }
  const seen = new Set();
  return fields
    .filter((field) => field && typeof field === 'object')
    .map((field) => ({
      name: normalizeBillingCredentialFieldName(field.name),
      label: (field.label || '').toString().trim(),
      required: field.required === true,
      secret: field.secret !== false,
    }))
    .filter((field) => {
      if (field.name === '' || seen.has(field.name)) {
        return false;
      }
      seen.add(field.name);
      return true;
    });
};

const normalizeBillingCredentials = (credentials) => {
  if (!credentials || typeof credentials !== 'object') {
    return {};
  }
  const result = {};
  Object.entries(credentials).forEach(([key, value]) => {
    const normalizedKey = normalizeBillingCredentialFieldName(key);
    const normalizedValue = (value || '').toString().trim();
    if (normalizedKey === '' || normalizedValue === '') {
      return;
    }
    result[normalizedKey] = normalizedValue;
  });
  return result;
};

const resolveBillingAdapterCredentialFields = (source, adapters = []) => {
  const billingSource = resolveChannelBillingSourceValue(source, adapters);
  if (billingSource === 'manual') {
    return [];
  }
  const adapter = (Array.isArray(adapters) ? adapters : []).find(
    (item) =>
      (item?.name || '').toString().trim().toLowerCase() === billingSource
  );
  return normalizeBillingCredentialFields(adapter?.credential_fields);
};

const filterBillingCredentialsByFields = (credentials, fields) => {
  const normalizedCredentials = normalizeBillingCredentials(credentials);
  if (!Array.isArray(fields) || fields.length === 0) {
    return {};
  }
  const allowedFields = new Set(
    fields.map((field) => normalizeBillingCredentialFieldName(field.name))
  );
  const result = {};
  Object.entries(normalizedCredentials).forEach(([key, value]) => {
    if (allowedFields.has(key)) {
      result[key] = value;
    }
  });
  return result;
};

const missingRequiredBillingCredentialField = (fields, credentials) => {
  const normalizedCredentials = normalizeBillingCredentials(credentials);
  const credentialFields = normalizeBillingCredentialFields(fields);
  const missingField = credentialFields.find(
    (field) => field.required && !normalizedCredentials[field.name]
  );
  if (!missingField) {
    return '';
  }
  return missingField.label || missingField.name;
};

const resolveEffectiveAPIBaseURL = (inputs, config) =>
  normalizeBaseURL(config?.api_base_url || inputs?.base_url || '');

const CHANNEL_IDENTIFIER_PATTERN = /^[a-z0-9-]+$/;
const CHANNEL_IDENTIFIER_MAX_LENGTH = 64;
const CHANNEL_ENDPOINT_COLUMN_WIDTHS = ['28%', '18%', '10%', '34%', '10%'];
const CHANNEL_MODEL_TEST_GROUP_COLUMN_WIDTHS = [
  '4%',
  '15%',
  '22%',
  '8%',
  '12%',
  '15%',
  '16%',
  '8%',
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

const VOLCENGINE_STANDARD_PROTOCOL = 'volcengine';

function isVolcengineDisplayProtocol(protocol) {
  return normalizeChannelProtocol(protocol) === VOLCENGINE_STANDARD_PROTOCOL;
}

function hasVolcengineRealtimeChannelModel(channelModels, protocol) {
  return normalizeChannelModels(channelModels, protocol).some((row) => {
    const endpoints = Array.isArray(row?.endpoints)
      ? row.endpoints
      : row?.endpoint
      ? [row.endpoint]
      : [];
    return endpoints.some(
      (endpoint) =>
        (endpoint || '').toString().trim().toLowerCase() === '/v1/realtime'
    );
  });
}

function resolveDisplayProtocolFromChannelPayload(payload) {
  const protocol = normalizeChannelProtocol(payload?.protocol);
  if (protocol !== '') {
    return protocol;
  }
  return 'openai';
}

function resolveEffectiveProtocolFromInputs(inputs) {
  const protocol = normalizeChannelProtocol(inputs?.protocol);
  if (!isVolcengineDisplayProtocol(protocol)) {
    return protocol || 'openai';
  }
  return VOLCENGINE_STANDARD_PROTOCOL;
}

function isEffectiveVolcengineRealtimeProtocol(inputs) {
  // The Realtime App ID / Resource ID block (volc.speech.dialog) is specific to
  // the Volcengine voice WebSocket API. Other providers (e.g. Aliyun/QianWen) may
  // also expose a /v1/realtime model, so gate on the effective protocol too —
  // otherwise their realtime models wrongly surface the Volcengine-only fields.
  if (resolveEffectiveProtocolFromInputs(inputs) !== VOLCENGINE_STANDARD_PROTOCOL) {
    return false;
  }
  return hasVolcengineRealtimeChannelModel(inputs?.channel_models, inputs?.protocol);
}

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

const validateProtocolSpecificChannelConfig = (inputs, config, t) => {
  const protocol = resolveEffectiveProtocolFromInputs(inputs);
  if (protocol !== 'deepseek') {
    return '';
  }
  const baseURL = normalizeBaseURL(inputs?.base_url || '');
  if (baseURL.toLowerCase().endsWith('/v1')) {
    return t('channel.edit.deepseek_base_url_no_v1');
  }
  const apiBaseURL = normalizeBaseURL(config?.api_base_url || '');
  if (apiBaseURL.toLowerCase().endsWith('/v1')) {
    return t('channel.edit.deepseek_api_base_url_no_v1');
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

// Per-model target margin override is a fraction (0..0.95); null/empty clears the
// override so the global policy applies. See 成本与盈利核算标准 §5.
const normalizeTargetMarginOverride = (value) => {
  if (value === null || value === undefined || value === '') {
    return null;
  }
  const margin = Number(value);
  if (!Number.isFinite(margin) || margin <= 0) {
    return null;
  }
  return margin > 0.95 ? 0.95 : margin;
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
      switch (normalizeChannelProtocol(protocol)) {
        case 'anthropic':
          return '/v1/messages';
        case 'zhipu':
          return '/v1/chat/completions';
        default:
          return '/v1/responses';
      }
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

const normalizeChannelModelEndpoints = (
  type,
  endpoints,
  endpoint,
  protocol
) => {
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

const normalizeExplicitChannelModelEndpoint = (type, value, protocol) => {
  if ((value || '').toString().trim() === '') {
    return '';
  }
  return normalizeChannelModelEndpoint(type, value, protocol);
};

const normalizeExplicitChannelModelEndpoints = (
  type,
  endpoints,
  endpoint,
  protocol
) => {
  const candidates = [];
  if (Array.isArray(endpoints)) {
    endpoints.forEach((item) => {
      candidates.push(item);
    });
  }
  if ((endpoint || '').toString().trim() !== '') {
    candidates.push(endpoint);
  }
  const seen = new Set();
  const result = [];
  candidates.forEach((item) => {
    const normalized = normalizeExplicitChannelModelEndpoint(
      type,
      item,
      protocol
    );
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    result.push(normalized);
  });
  return result;
};

const DETAIL_TAB_KEYS = [
  'overview',
  'models',
  'endpoints',
  'tests',
  'publish',
  'procurement',
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

const PROVIDER_MODEL_TAG_OPTIONS = [
  'text',
  'image',
  'audio',
  'video',
  'embedding',
  'tool_calling',
  'reasoning',
  'vision',
  'realtime',
  'structured_output',
].map((tag) => ({ key: tag, value: tag, text: tag }));

const providerModelTypeFromTags = (tags) => {
  const values = Array.isArray(tags)
    ? tags
    : typeof tags === 'string'
    ? tags.split(',')
    : [];
  for (const item of values) {
    const tag = (item || '').toString().trim().toLowerCase();
    if (['text', 'image', 'audio', 'video', 'embedding'].includes(tag)) {
      return tag;
    }
  }
  return '';
};

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
    ])
  );
  return normalizeChannelModelEndpoints(type, values, '', protocol).map(
    (endpoint) => ({
      key: endpoint,
      value: endpoint,
      text: labelByValue.get(endpoint) || endpoint,
    })
  );
};

const buildChannelEndpointKey = (modelName, endpoint) =>
  `${(modelName || '').toString().trim()}::${(endpoint || '')
    .toString()
    .trim()}`;

const buildChannelEndpointPolicyKey = (modelName, endpoint, templateKey) =>
  `${buildChannelEndpointKey(modelName, endpoint)}::${(templateKey || '')
    .toString()
    .trim()}`;

const ENDPOINT_POLICY_TEMPLATE_CUSTOM_REQUEST_POLICY = 'CUSTOM_REQUEST_POLICY';
const ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL = 'OVERRIDE_ENDPOINT_BASE_URL';

const parseEndpointAccessPolicyBaseURL = (requestPolicy) => {
  const raw = (requestPolicy || '').toString().trim();
  if (raw === '') {
    return '';
  }
  try {
    const parsed = JSON.parse(raw);
    return normalizeBaseURL(parsed?.base_url || '');
  } catch {
    return '';
  }
};

const buildEndpointAccessPolicyJSON = (baseURL) =>
  JSON.stringify(
    {
      base_url: normalizeBaseURL(baseURL || ''),
    },
    null,
    2
  );

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
      disabled_reason: (item.disabled_reason || '').toString().trim(),
      disabled_at: Number(item.disabled_at || 0),
      disabled_by: (item.disabled_by || '').toString().trim(),
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

const normalizeChannelBillingSummary = (item) => {
  if (!item || typeof item !== 'object') {
    return null;
  }
  const billingSource = normalizeChannelBillingSourceValue(item.billing_source);
  return {
    channel_id: (item.channel_id || '').toString().trim(),
    billing_source: billingSource,
    action_capabilities: Array.isArray(item.action_capabilities)
      ? item.action_capabilities
      : [],
    refresh_supported: item.refresh_supported === true,
    latest_snapshot_at: Number(item.latest_snapshot_at || 0),
    latest_snapshot_status: (item.latest_snapshot_status || '')
      .toString()
      .trim(),
    latest_snapshot_message: (item.latest_snapshot_message || '')
      .toString()
      .trim(),
    quota_items: Array.isArray(item.quota_items)
      ? item.quota_items.map((quotaItem) => ({
          resource_type: (quotaItem?.resource_type || '').toString().trim(),
          quota_type: (quotaItem?.quota_type || '').toString().trim(),
          quota_label: (quotaItem?.quota_label || '').toString().trim(),
          amount: Number(quotaItem?.amount || 0),
          limit_amount: Number(quotaItem?.limit_amount || 0),
          used_amount: Number(quotaItem?.used_amount || 0),
          remaining_amount: Number(quotaItem?.remaining_amount || 0),
          currency: (quotaItem?.currency || '').toString().trim(),
          reset_at: Number(quotaItem?.reset_at || 0),
          expires_at: Number(quotaItem?.expires_at || 0),
          status: (quotaItem?.status || '').toString().trim(),
          source_ref: (quotaItem?.source_ref || '').toString().trim(),
        }))
      : [],
  };
};

const normalizeChannelCostTrackingModeValue = (mode) => {
  const normalized = (mode || '').toString().trim().toLowerCase();
  return normalized === 'free' || normalized === 'actual'
    ? normalized
    : 'untracked';
};

const normalizeChannelBillingProfile = (item) => {
  if (!item || typeof item !== 'object') {
    return null;
  }
  const billingSource = normalizeChannelBillingSourceValue(item.billing_source);
  return {
    channel_id: (item.channel_id || '').toString().trim(),
    billing_source: billingSource,
    cost_tracking_mode: normalizeChannelCostTrackingModeValue(
      item.cost_tracking_mode
    ),
    billing_credentials: normalizeBillingCredentials(item.billing_credentials),
    action_capabilities: Array.isArray(item.action_capabilities)
      ? item.action_capabilities
      : [],
    cost_missing_model_count: Number(item.cost_missing_model_count || 0),
  };
};

const normalizeChannelBillingAdapters = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  const seen = new Set();
  return items
    .filter((item) => item && typeof item === 'object')
    .map((item) => ({
      name: (item.name || '').toString().trim().toLowerCase(),
      capabilities: Array.isArray(item.capabilities) ? item.capabilities : [],
      credential_fields: normalizeBillingCredentialFields(
        item.credential_fields
      ),
    }))
    .filter((item) => {
      if (item.name === '' || seen.has(item.name)) {
        return false;
      }
      seen.add(item.name);
      return true;
    });
};

const normalizeChannelBillingSnapshots = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .filter((item) => item && typeof item === 'object')
    .map((item) => ({
      id: (item.id || '').toString().trim(),
      source_type: (item.source_type || '').toString().trim(),
      message: (item.message || '').toString().trim(),
      purchase_at: Number(item.purchase_at || 0),
      purchase_currency: (item.purchase_currency || '').toString().trim(),
      purchase_amount: Number(item.purchase_amount || 0),
      purchase_fx_rate: Number(item.purchase_fx_rate || 0),
      purchase_cost_amount: Number(item.purchase_cost_amount || 0),
      entitlement_name: (item.entitlement_name || '').toString(),
      event_type: (item.event_type || 'purchase').toString(),
      parent_snapshot_id: (item.parent_snapshot_id || '').toString(),
      old_batch_disposition: (item.old_batch_disposition || 'keep').toString(),
      valid_from: Number(item.valid_from || 0),
      valid_until: Number(item.valid_until || 0),
      created_at: Number(item.created_at || 0),
      items: Array.isArray(item.items)
        ? item.items.map((quotaItem) => ({
            id: (quotaItem?.id || '').toString().trim(),
            resource_type: (quotaItem?.resource_type || '').toString().trim(),
            quota_type: (quotaItem?.quota_type || '').toString().trim(),
            quota_label: (quotaItem?.quota_label || '').toString().trim(),
            amount: Number(quotaItem?.amount || 0),
            limit_amount: Number(quotaItem?.limit_amount || 0),
            used_amount: Number(quotaItem?.used_amount || 0),
            remaining_amount: Number(quotaItem?.remaining_amount || 0),
            currency: (quotaItem?.currency || '').toString().trim(),
            reset_at: Number(quotaItem?.reset_at || 0),
            expires_at: Number(quotaItem?.expires_at || 0),
            status: (quotaItem?.status || '').toString().trim(),
            source_ref: (quotaItem?.source_ref || '').toString().trim(),
          }))
        : [],
    }));
};

const normalizeChannelBillingActions = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .filter((item) => item && typeof item === 'object')
    .map((item) => ({
      id: (item.id || '').toString().trim(),
      action_type: (item.action_type || '').toString().trim(),
      status: (item.status || '').toString().trim(),
      message: (item.message || '').toString().trim(),
      created_at: Number(item.created_at || 0),
    }));
};

const normalizeChannelProcurementBatches = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .filter((item) => item && typeof item === 'object')
    .map((item) => ({
      id: (item.id || '').toString().trim(),
      channel_id: (item.channel_id || '').toString().trim(),
      resource_type: (item.resource_type || '').toString().trim(),
      quota_type: (item.quota_type || '').toString().trim(),
      capacity_unit: (item.capacity_unit || '').toString().trim(),
      capacity_total: Number(item.capacity_total || 0),
      capacity_effective: Number(item.capacity_effective || 0),
      capacity_remaining: Number(item.capacity_remaining || 0),
      purchase_currency: (item.purchase_currency || '').toString().trim(),
      purchase_amount: Number(item.purchase_amount || 0),
      purchase_fx_rate: Number(item.purchase_fx_rate || 0),
      purchase_cost_amount: Number(item.purchase_cost_amount || 0),
      cost_per_unit_amount: Number(item.cost_per_unit_amount || 0),
      cost_source: (item.cost_source || '').toString().trim(),
      cost_status: (item.cost_status || '').toString().trim(),
      expire_at: Number(item.expire_at || 0),
      reset_cycle: (item.reset_cycle || '').toString().trim(),
      source_snapshot_id: (item.source_snapshot_id || '').toString().trim(),
      source_ref: (item.source_ref || '').toString().trim(),
      created_at: Number(item.created_at || 0),
      updated_at: Number(item.updated_at || 0),
    }));
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
    const templateKey = (item.template_key || '').toString().trim();
    const key = buildChannelEndpointPolicyKey(model, endpoint, templateKey);
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
      template_key: templateKey,
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
    const endpointOrder = left.endpoint.localeCompare(right.endpoint);
    if (endpointOrder !== 0) {
      return endpointOrder;
    }
    const templateOrder = left.template_key.localeCompare(right.template_key);
    if (templateOrder !== 0) {
      return templateOrder;
    }
    return left.id.localeCompare(right.id);
  });
  return rows;
};

const buildEmptyEndpointPolicyDraft = (channelId, modelName, endpoint) => ({
  id: '',
  channel_id: (channelId || '').toString().trim(),
  model: (modelName || '').toString().trim(),
  endpoint: (endpoint || '').toString().trim(),
  endpoint_enabled: false,
  endpoint_legacy_base_url: '',
  access_base_url: '',
  endpoint_enable_block_reason: '',
  enabled: true,
  template_key: '',
  original_template_key: '',
  capabilities: '',
  request_policy: '',
  response_policy: '',
  reason: '',
  source: 'manual',
  endpoint_policy_rows: [],
  last_verified_at: 0,
  updated_at: 0,
});

const ENDPOINT_POLICY_TEMPLATES = [
  {
    key: ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL,
    value: ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL,
    text: 'OVERRIDE_ENDPOINT_BASE_URL',
    buildDraft: (prev = {}) => {
      const baseURL = normalizeBaseURL(
        prev.access_base_url || parseEndpointAccessPolicyBaseURL(prev.request_policy)
      );
      return {
        template_key: ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL,
        access_base_url: baseURL,
        capabilities: '',
        request_policy: buildEndpointAccessPolicyJSON(baseURL),
        response_policy: '',
        reason: '该端点使用独立访问地址',
        source: 'manual',
      };
    },
  },
  {
    key: ENDPOINT_POLICY_TEMPLATE_CUSTOM_REQUEST_POLICY,
    value: ENDPOINT_POLICY_TEMPLATE_CUSTOM_REQUEST_POLICY,
    text: 'CUSTOM_REQUEST_POLICY',
    buildDraft: () => ({
      template_key: ENDPOINT_POLICY_TEMPLATE_CUSTOM_REQUEST_POLICY,
      capabilities: '',
      request_policy: '',
      response_policy: '',
      reason: '',
      source: 'manual',
    }),
  },
  {
    key: 'IMAGE_URL_TO_BASE64',
    value: 'IMAGE_URL_TO_BASE64',
    text: 'IMAGE_URL_TO_BASE64',
    buildDraft: () => ({
      template_key: 'IMAGE_URL_TO_BASE64',
      capabilities: JSON.stringify(
        {
          input_image_url: false,
          input_image_base64: true,
        },
        null,
        2
      ),
      request_policy: JSON.stringify(
        {
          actions: [
            {
              type: 'image_url_to_base64',
              input_types: [
                'anthropic.image_url',
                'openai.image_url',
                'openai.input_image',
              ],
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
        2
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
      normalizeSearchKeyword
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
    ])
  );
  normalizeComplexPriceComponents(overrideComponents).forEach((component) => {
    const key = `${component.component || ''}\u0000${
      component.condition || ''
    }`;
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
      Number(component.output_price || 0) > 0
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
      const providerModelType = providerModelTypeFromTags(detail?.tags);
      providerModelDetails[providerId][modelName] = {
        model: modelName,
        type: providerModelType
          ? normalizeChannelModelType(providerModelType)
          : '',
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
          detail?.price_components
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

const canonicalProviderModelSnapshotPattern =
  /^(.+?)(?:(?:-(?:preview|latest))|(?:-\d{6})|(?:-\d{4}-\d{2}-\d{2}))+$/i;

const canonicalProviderModelLookupKey = (value) => {
  const normalized = (value || '').toString().trim();
  if (normalized === '') {
    return '';
  }
  const match = normalized.match(canonicalProviderModelSnapshotPattern);
  const canonical = (match?.[1] || '').toString().trim();
  if (canonical === '' || canonical === normalized) {
    return '';
  }
  return canonical;
};

const addProviderLookupKey = (keys, value) => {
  const normalized = (value || '').toString().trim();
  if (normalized === '') {
    return;
  }
  keys.add(normalized);
  const canonical = canonicalProviderModelLookupKey(normalized);
  if (canonical !== '') {
    keys.add(canonical);
  }
};

const buildProviderLookupKeys = (row) => {
  const keys = new Set();
  [row?.upstream_model, row?.model].forEach((value) => {
    const normalized = (value || '').toString().trim();
    if (normalized === '') {
      return;
    }
    addProviderLookupKey(keys, normalized);
    if (normalized.includes('/')) {
      const parts = normalized.split('/');
      if (parts.length > 1) {
        const suffix = parts.slice(1).join('/').trim();
        addProviderLookupKey(keys, suffix);
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
      normalizeProviderIdentifier(item?.value || '')
    )
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
    case 'moonshot':
    case 'moonshotai':
    case 'kimi':
      return 'moonshot';
    case 'amazon-nova':
    case 'amazon_nova':
      return 'amazon-nova';
    case 'black-forest-labs':
    case 'blackforestlabs':
    case 'bfl':
      return 'black-forest-labs';
    case 'perplexity':
      return 'perplexity';
    case 'voyage':
    case 'voyageai':
    case 'voyage-ai':
    case 'voyage ai':
      return 'voyageai';
    case 'deepgram':
      return 'deepgram';
    case 'assemblyai':
    case 'assembly-ai':
      return 'assemblyai';
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
    const [prefix, suffix] = name.split('/', 2);
    if (
      prefix.toLowerCase() === 'amazon' &&
      suffix?.toLowerCase().startsWith('nova')
    ) {
      return 'amazon-nova';
    }
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
  if (
    lower.startsWith('glm-') ||
    lower.startsWith('cogview-') ||
    lower.startsWith('cogvideox-')
  )
    return 'zhipu';
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
  if (lower.startsWith('sonar')) return 'perplexity';
  if (lower.startsWith('voyage')) return 'voyageai';
  if (lower.startsWith('universal-')) return 'assemblyai';
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
  const modelName = (row.model || row.alias || row.display_model || upstreamModel)
    .toString()
    .trim();
  const model = modelName || upstreamModel;
  if (!model) {
    return null;
  }
  const normalizedEndpoints = normalizeChannelModelEndpoints(
    row.type,
    row.endpoints || row.endpoint_list || [],
    row.endpoint,
    protocol
  );
  const normalizedEndpointCandidate = normalizeChannelModelEndpoint(
    row.type,
    row.endpoint,
    protocol
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
    selected: row.selected === true,
    disabled_reason: (row.disabled_reason || '').toString().trim(),
    disabled_at: Number(row.disabled_at || 0),
    disabled_by: (row.disabled_by || '').toString().trim(),
    is_stream: resolveModelTestStreamEnabled(row),
    input_price: normalizePriceOverrideValue(row.input_price),
    output_price: normalizePriceOverrideValue(row.output_price),
    target_margin: normalizeTargetMarginOverride(row.target_margin),
    price_unit: normalizePriceUnitValue(row.price_unit),
    currency: normalizeCurrencyValue(row.currency),
    price_components: normalizeComplexPriceComponents(row.price_components),
    publish_status: (row.publish_status || '').toString().trim(),
    publish_enabled: row.publish_enabled === true,
    published_model: (
      Object.prototype.hasOwnProperty.call(row, 'published_model')
        ? row.published_model
        : row.model || upstreamModel || ''
    ).toString().trim(),
    published_model_original: (
      row.published_model_original ||
      row.published_model ||
      row.model ||
      upstreamModel ||
      ''
    ).toString().trim(),
    published_at: Number(row.published_at || 0),
    published_by: (row.published_by || '').toString().trim(),
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
  const normalizedChannelModels = normalizeChannelModels(
    channelModels,
    protocol
  );
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
    normalizeModelIDs(Array.isArray(selectedModels) ? selectedModels : [])
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
  const normalizedChannelModels = normalizeChannelModels(
    channelModels,
    protocol
  );
  const selectedModels = normalizedChannelModels
    .filter((row) => row.selected)
    .map((row) => row.model);
  return {
    channelModels: normalizedChannelModels,
    selectedModels,
  };
};

const buildNextInputsWithChannelModels = (
  previousInputs,
  channelModels,
  protocol
) => {
  const { channelModels: normalizedChannelModels, selectedModels } =
    buildChannelModelState(channelModels, protocol ?? previousInputs?.protocol);
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

const getChangedSelectedChannelModels = (rows, previousRows, protocol) => {
  const previousByModel = new Map(
    normalizeChannelModels(previousRows, protocol).map((row) => [
      row.model,
      row,
    ])
  );
  return normalizeChannelModels(rows, protocol).filter((row) => {
    if (row.selected !== true) {
      return false;
    }
    const previous = previousByModel.get(row.model);
    if (!previous || previous.selected !== true) {
      return true;
    }
    return (
      previous.upstream_model !== row.upstream_model ||
      previous.provider !== row.provider ||
      previous.type !== row.type
    );
  });
};

const buildBlockedSelectedModelsMessage = (rows, previousRows, protocol, t) => {
  const blockedRows = getChangedSelectedChannelModels(
    rows,
    previousRows,
    protocol
  ).filter((row) => (row.enable_block_reason || '').toString().trim() !== '');
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
      }
    );
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || 'fetch channel models failed');
    }
    const pageItems = normalizeChannelModels(
      extractChannelModelListItems(data),
      protocol
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
    `/api/v1/admin/channel/${normalizedChannelId}/tests`
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
    `/api/v1/admin/channel/${normalizedChannelId}/endpoints`
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
    `/api/v1/admin/channel/${normalizedChannelId}/policies`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel endpoint policies failed');
  }
  return normalizeChannelEndpointPolicyRows(data?.items);
};

const deleteChannelEndpointPolicy = async (channelId, policyId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  const normalizedPolicyId = (policyId || '').toString().trim();
  if (normalizedChannelId === '' || normalizedPolicyId === '') {
    throw new Error('delete channel endpoint policy failed');
  }
  const res = await API.delete(
    `/api/v1/admin/channel/${normalizedChannelId}/policies/${normalizedPolicyId}`
  );
  const { success, message } = res.data || {};
  if (!success) {
    throw new Error(message || 'delete channel endpoint policy failed');
  }
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

const fetchChannelBillingAdapters = async () => {
  const res = await API.get('/api/v1/admin/channel/billing/adapters');
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch billing adapters failed');
  }
  return normalizeChannelBillingAdapters(data?.items);
};

const fetchChannelBillingSummary = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return null;
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel billing failed');
  }
  return normalizeChannelBillingSummary(data);
};

const fetchChannelBillingProfile = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return null;
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/profile`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel billing profile failed');
  }
  return normalizeChannelBillingProfile(data);
};

const fetchChannelBillingSnapshots = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/snapshots`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel billing snapshots failed');
  }
  return normalizeChannelBillingSnapshots(data?.items);
};

const fetchChannelBillingActions = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/actions`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel billing actions failed');
  }
  return normalizeChannelBillingActions(data?.items);
};

const fetchChannelProcurementBatches = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/procurement-batches`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel procurement batches failed');
  }
  return normalizeChannelProcurementBatches(data?.items);
};

// Read-only cost-quote reconciliation: billing service's normalized unit cost per
// model (YYC) vs local procurement readiness. Never affects online charging.
const fetchChannelCostQuotes = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return { service_available: false, reason: '', rows: [] };
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/cost-quotes`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel cost quotes failed');
  }
  return {
    service_available: data?.service_available === true,
    reason: (data?.reason || '').toString(),
    rows: Array.isArray(data?.rows)
      ? data.rows.map((row) => ({
          model: (row?.model || '').toString(),
          capacity_unit: (row?.capacity_unit || '').toString(),
          service_unit_cost: Number(row?.service_unit_cost || 0),
          service_currency: (row?.service_currency || '').toString(),
          service_unit_cost_yyc: Number(row?.service_unit_cost_yyc || 0),
          service_confidence: (row?.service_confidence || '').toString(),
          local_readiness: (row?.local_readiness || '').toString(),
        }))
      : [],
  };
};

// Sync service cost quotes into the local rate cache (admin-only). Returns the
// counts of newly cached quotes vs skipped ones (non-actual, unparseable currency).
const syncChannelCostQuotes = async (channelId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return { cached: 0, skipped: 0 };
  }
  const res = await API.post(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/cost-quotes/sync`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'sync channel cost quotes failed');
  }
  return {
    cached: Number(data?.cached || 0),
    skipped: Number(data?.skipped || 0),
  };
};

const fetchChannelProcurementBatchConsumptions = async (channelId, batchId) => {
  const normalizedChannelId = (channelId || '').toString().trim();
  const normalizedBatchId = (batchId || '').toString().trim();
  if (normalizedChannelId === '' || normalizedBatchId === '') {
    return [];
  }
  const res = await API.get(
    `/api/v1/admin/channel/${normalizedChannelId}/billing/procurement-batches/${normalizedBatchId}/consumptions`
  );
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch procurement consumptions failed');
  }
  return Array.isArray(data?.items) ? data.items : [];
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
    const modelName = (row?.model || '').toString().trim();
    const upstreamModel = (row?.upstream_model || '').toString().trim();
    if (modelName === '' || upstreamModel === '') {
      return t('channel.edit.messages.model_config_invalid');
    }
    if (seen.has(modelName)) {
      return t('channel.edit.messages.model_config_invalid');
    }
    seen.add(modelName);
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
    protocol
  )
    .filter((row) => row.selected)
    .map(
      (row) =>
        `${row.model}:${row.type}:${normalizeChannelModelEndpoints(
          row.type,
          row.endpoints,
          row.endpoint,
          protocol
        ).join('|')}`
    )
    .join(',')}`;

const normalizeModelTestResults = (results) => {
  if (!Array.isArray(results)) {
    return [];
  }
  return results
    .filter(
      (item) =>
        item && typeof item === 'object' && typeof item.model === 'string'
    )
    .map((item) => {
      const hasIsStream =
        Object.prototype.hasOwnProperty.call(item, 'is_stream') ||
        Object.prototype.hasOwnProperty.call(item, 'isStream');
      const rawIsStream = hasIsStream ? item.is_stream ?? item.isStream : null;
      return {
        channel_id: (item.channel_id || '').toString().trim(),
        model: item.model || '',
        upstream_model: item.upstream_model || '',
        type: normalizeChannelModelType(item.type),
        endpoint: item.endpoint || '',
        is_stream:
          rawIsStream === true ? true : rawIsStream === false ? false : null,
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
  key_preview: '',
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
  app_id: '',
  user_id: '',
  resource_id: '',
  api_base_url: '',
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
  return resolveDisplayProtocolFromChannelPayload(payload);
};

const isRecordNotFoundMessage = (value) =>
  (value || '').toString().trim().toLowerCase() === 'record not found';

const maskChannelKeyPreview = (value) => {
  const normalized = (value || '').toString().trim();
  if (normalized === '') {
    return '';
  }
  if (normalized.length <= 6) {
    return '*'.repeat(normalized.length);
  }
  return `${normalized.slice(0, 3)}${'*'.repeat(
    normalized.length - 6
  )}${normalized.slice(-3)}`;
};

export {
  addProviderLookupKey,
  AUDIO_MODEL_ENDPOINT_OPTIONS,
  buildBlockedSelectedModelsMessage,
  buildChannelConnectionSignature,
  buildChannelEndpointKey,
  buildChannelEndpointPolicyKey,
  buildChannelModelsFromLegacyFields,
  buildChannelModelState,
  buildChannelModelTestSignature,
  buildEmptyEndpointPolicyDraft,
  buildEndpointAccessPolicyJSON,
  buildEndpointOptionsFromValues,
  buildModelIDs,
  buildModelTestResultKey,
  buildNextInputsWithChannelModels,
  buildProviderIndex,
  buildProviderLookupKeys,
  buildProviderOptionText,
  canonicalProviderModelLookupKey,
  canonicalProviderModelSnapshotPattern,
  CHANNEL_DEFAULT_CONFIG,
  CHANNEL_ENDPOINT_COLUMN_WIDTHS,
  CHANNEL_ENDPOINT_SORT_ORDER,
  CHANNEL_IDENTIFIER_MAX_LENGTH,
  CHANNEL_IDENTIFIER_PATTERN,
  CHANNEL_MODEL_PAGE_SIZE,
  CHANNEL_MODEL_TEST_GROUP_COLUMN_WIDTHS,
  CHANNEL_MODEL_TYPE_OPTIONS,
  CHANNEL_ORIGIN_INPUTS,
  defaultChannelModelEndpoint,
  deleteChannelEndpointPolicy,
  DETAIL_TAB_KEYS,
  EMBEDDING_MODEL_ENDPOINT_OPTIONS,
  ENDPOINT_POLICY_TEMPLATE_CUSTOM_REQUEST_POLICY,
  ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL,
  ENDPOINT_POLICY_TEMPLATES,
  endpointOptionsForModelType,
  extractChannelModelListItems,
  fetchActiveChannelTasks,
  fetchAllChannelModels,
  fetchChannelBillingActions,
  fetchChannelBillingAdapters,
  fetchChannelBillingProfile,
  fetchChannelBillingSnapshots,
  fetchChannelBillingSummary,
  fetchChannelEndpointPolicies,
  fetchChannelEndpoints,
  fetchChannelProcurementBatchConsumptions,
  fetchChannelProcurementBatches,
  fetchChannelCostQuotes,
  syncChannelCostQuotes,
  fetchChannelTests,
  fetchTaskById,
  filterBillingCredentialsByFields,
  filterProviderOptionsByQuery,
  getChangedSelectedChannelModels,
  getChannelModelsFromInputs,
  hasVolcengineRealtimeChannelModel,
  IMAGE_MODEL_ENDPOINT_OPTIONS,
  inferAssignableProviderForRowWithOptions,
  isActiveAsyncTaskStatus,
  isEffectiveVolcengineRealtimeProtocol,
  isRecordNotFoundMessage,
  isVolcengineDisplayProtocol,
  maskChannelKeyPreview,
  mergePriceComponentOverrides,
  missingRequiredBillingCredentialField,
  normalizeAsyncTasks,
  normalizeAsyncTaskStatus,
  normalizeBaseURL,
  normalizeBillingCredentialFieldName,
  normalizeBillingCredentialFields,
  normalizeBillingCredentials,
  normalizeChannelBillingActions,
  normalizeChannelBillingAdapters,
  normalizeChannelBillingProfile,
  normalizeChannelBillingSnapshots,
  normalizeChannelBillingSourceValue,
  normalizeChannelBillingSummary,
  normalizeChannelCostTrackingModeValue,
  normalizeChannelEndpointPolicyRows,
  normalizeChannelEndpointRows,
  normalizeChannelIdentifier,
  normalizeChannelModelConfigRow,
  normalizeChannelModelEndpoint,
  normalizeChannelModelEndpoints,
  normalizeChannelModelProviderValue,
  normalizeChannelModels,
  normalizeChannelModelType,
  normalizeChannelProcurementBatches,
  normalizeChannelProtocol,
  normalizeComplexPriceComponents,
  normalizeCurrencyValue,
  normalizeDetailTab,
  normalizeExplicitChannelModelEndpoint,
  normalizeExplicitChannelModelEndpoints,
  normalizeModelId,
  normalizeModelIDs,
  normalizeModelTestResults,
  normalizePriceOverrideValue,
  normalizePriceUnitValue,
  normalizeProviderIdentifier,
  normalizeSearchKeyword,
  parseEndpointAccessPolicyBaseURL,
  parseJSONObject,
  prettyJSONString,
  protocol2secretPrompt,
  protocolSelectionHint,
  PROVIDER_MODEL_TAG_OPTIONS,
  providerModelTypeFromTags,
  resolveBillingAdapterCredentialFields,
  resolveChannelBillingSourceValue,
  resolveDisplayProtocolFromChannelPayload,
  resolveEffectiveAPIBaseURL,
  resolveEffectiveProtocolFromInputs,
  resolveModelTestStreamEnabled,
  resolveProtocolFromChannelPayload,
  resolveProviderIdentifierFromModelName,
  sanitizeCreateConfigForLocalStorage,
  sanitizeCreateInputsForLocalStorage,
  supportsModelTestStream,
  TEXT_MODEL_ENDPOINT_OPTIONS,
  validateChannelIdentifier,
  validateChannelModels,
  validateProtocolSpecificChannelConfig,
  VOLCENGINE_STANDARD_PROTOCOL,
};
