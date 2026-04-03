import React, {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import {
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Dropdown,
  Form,
  Icon,
  Label,
  Message,
  Modal,
  Pagination,
  Table,
} from 'semantic-ui-react';
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

const CHANNEL_IDENTIFIER_PATTERN = /^[a-z0-9-]+$/;
const CHANNEL_IDENTIFIER_MAX_LENGTH = 64;

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
      return normalized;
    default:
      return 'text';
  }
};

const defaultChannelModelEndpoint = (type) => {
  switch (normalizeChannelModelType(type)) {
    case 'image':
      return '/v1/images/generations';
    case 'audio':
      return '/v1/audio/speech';
    case 'video':
      return '/v1/videos';
    default:
      return '/v1/responses';
  }
};

const normalizeChannelModelEndpoint = (type, value) => {
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
    return normalized === '/v1/chat/completions'
      ? '/v1/chat/completions'
      : '/v1/responses';
  }
  return defaultChannelModelEndpoint(normalizedType);
};

const CHANNEL_MODEL_TYPE_OPTIONS = [
  { key: 'text', value: 'text', text: 'text' },
  { key: 'image', value: 'image', text: 'image' },
  { key: 'audio', value: 'audio', text: 'audio' },
  { key: 'video', value: 'video', text: 'video' },
];

const TEXT_MODEL_ENDPOINT_OPTIONS = [
  { key: 'responses', value: '/v1/responses', text: '/v1/responses' },
  { key: 'chat', value: '/v1/chat/completions', text: '/v1/chat/completions' },
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

const buildProviderCatalogIndex = (items) => {
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

const normalizeChannelModelConfigRow = (row) => {
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
  return {
    model,
    upstream_model: upstreamModel || model,
    provider: normalizeChannelModelProviderValue(row.provider),
    type: normalizeChannelModelType(row.type),
    endpoint: normalizeChannelModelEndpoint(row.type, row.endpoint),
    inactive: row.inactive === true,
    selected: row.selected === true,
    input_price: normalizePriceOverrideValue(row.input_price),
    output_price: normalizePriceOverrideValue(row.output_price),
    price_unit: normalizePriceUnitValue(row.price_unit),
    currency: normalizeCurrencyValue(row.currency),
  };
};

const normalizeChannelModelConfigs = (rows) => {
  if (!Array.isArray(rows)) {
    return [];
  }
  const seen = new Set();
  const result = [];
  rows.forEach((row) => {
    const normalized = normalizeChannelModelConfigRow(row);
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

const buildModelConfigsFromLegacyFields = ({
  modelConfigs,
  availableModels,
  selectedModels,
  modelMapping,
  inputPrice,
  outputPrice,
  priceUnit,
  currency,
}) => {
  const normalizedConfigs = normalizeChannelModelConfigs(modelConfigs);
  if (normalizedConfigs.length > 0) {
    return normalizedConfigs;
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

const buildChannelModelState = (modelConfigs) => {
  const normalizedConfigs = normalizeChannelModelConfigs(modelConfigs);
  const selectedModels = normalizedConfigs
    .filter((row) => row.selected && row.inactive !== true)
    .map((row) => row.model);
  return {
    modelConfigs: normalizedConfigs,
    selectedModels,
  };
};

const buildNextInputsWithModelConfigs = (previousInputs, modelConfigs) => {
  const { modelConfigs: normalizedConfigs, selectedModels } =
    buildChannelModelState(modelConfigs);
  const currentTestModel = (previousInputs.test_model || '').toString().trim();
  const nextTestModel =
    currentTestModel !== '' && selectedModels.includes(currentTestModel)
      ? currentTestModel
      : selectedModels[0] || '';
  return {
    ...previousInputs,
    model_configs: normalizedConfigs,
    models: selectedModels,
    test_model: nextTestModel,
  };
};

const extractChannelModelListItems = (payload) => {
  if (Array.isArray(payload?.items)) {
    return payload.items;
  }
  if (Array.isArray(payload?.model_configs)) {
    return payload.model_configs;
  }
  return [];
};

const fetchAllChannelModelConfigs = async (channelId) => {
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
    const pageItems = normalizeChannelModelConfigs(
      extractChannelModelListItems(data),
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
  return normalizeChannelModelConfigs(items);
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

const validateModelConfigs = (modelConfigs, t) => {
  const seen = new Set();
  for (const row of Array.isArray(modelConfigs) ? modelConfigs : []) {
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
  modelConfigs,
}) =>
  `${buildChannelConnectionSignature({
    protocol,
    key,
    baseURL,
    channelID,
  })}|${normalizeModelIDs(models).join(',')}|${normalizeChannelModelConfigs(
    modelConfigs,
  )
    .filter((row) => row.selected)
    .map((row) => `${row.model}:${row.type}:${row.endpoint || ''}`)
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
    .map((item) => ({
      channel_id: (item.channel_id || '').toString().trim(),
      model: item.model || '',
      upstream_model: item.upstream_model || '',
      type: normalizeChannelModelType(item.type),
      endpoint: item.endpoint || '',
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
    }));
};

const buildModelTestResultKey = (modelName, endpoint) =>
  `${(modelName || '').toString().trim()}::${(endpoint || '')
    .toString()
    .trim()}`;

const parseDownloadFilename = (contentDisposition, fallbackName) => {
  const raw = (contentDisposition || '').toString();
  const utf8Match = raw.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const plainMatch = raw.match(/filename=\"?([^\";]+)\"?/i);
  if (plainMatch?.[1]) {
    return plainMatch[1];
  }
  return fallbackName;
};

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

const mergeModelTestResults = (previousResults, nextResults) => {
  const merged = new Map();
  normalizeModelTestResults(previousResults).forEach((item) => {
    const key = buildModelTestResultKey(item.model, item.endpoint);
    if (!item.model || !item.endpoint || key === '::') {
      return;
    }
    merged.set(key, item);
  });
  normalizeModelTestResults(nextResults).forEach((item) => {
    const key = buildModelTestResultKey(item.model, item.endpoint);
    if (!item.model || !item.endpoint || key === '::') {
      return;
    }
    merged.set(key, item);
  });
  return Array.from(merged.values()).sort((a, b) =>
    buildModelTestResultKey(a.model, a.endpoint).localeCompare(
      buildModelTestResultKey(b.model, b.endpoint),
    ),
  );
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

const CHANNEL_CREATE_CACHE_KEY = 'router.channel.create.v3';
const CREATE_CHANNEL_STEP_MIN = 1;
const CREATE_CHANNEL_STEP_MAX = 4;

const parseCreateStep = (rawStep) => {
  const step = Number(rawStep);
  if (!Number.isInteger(step)) {
    return CREATE_CHANNEL_STEP_MIN;
  }
  if (step < CREATE_CHANNEL_STEP_MIN) {
    return CREATE_CHANNEL_STEP_MIN;
  }
  if (step > CREATE_CHANNEL_STEP_MAX) {
    return CREATE_CHANNEL_STEP_MAX;
  }
  return step;
};

const CHANNEL_ORIGIN_INPUTS = {
  id: '',
  name: '',
  protocol: 'openai',
  key: '',
  base_url: '',
  other: '',
  model_configs: [],
  system_prompt: '',
  models: [],
  test_model: '',
  created_time: 0,
};

const CHANNEL_DEFAULT_CONFIG = {
  region: '',
  sk: '',
  ak: '',
  user_id: '',
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

const resolveProtocolFromChannelPayload = (payload) => {
  const protocol = (payload?.protocol || '').toString().trim().toLowerCase();
  if (protocol !== '') {
    return protocol;
  }
  return 'openai';
};

const inferCreatingChannelStepFromPayload = (payload) => {
  const protocol = resolveProtocolFromChannelPayload(payload);
  if (protocol === 'proxy') {
    return 1;
  }
  const selectedModels = Array.isArray(payload?.model_configs)
    ? payload.model_configs
        .filter((row) => row && row.selected === true)
        .map((row) => (row.model || '').toString().trim())
        .filter((row) => row !== '')
    : (payload?.models || '')
        .toString()
        .split(',')
        .map((item) => item.trim())
        .filter((item) => item !== '');
  if (selectedModels.length === 0) {
    return 2;
  }
  const testedAt = Number(payload?.channel_tests_last_tested_at || 0);
  const results = Array.isArray(payload?.channel_tests)
    ? payload.channel_tests
    : [];
  if (testedAt > 0 || results.length > 0) {
    return 4;
  }
  return 3;
};

const EditChannel = () => {
  const { t } = useTranslation();
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const channelId = params.id;
  const hasChannelID = channelId !== undefined;
  const isDetailMode =
    hasChannelID && location.pathname.includes('/channel/detail/');
  const isCreateMode = !hasChannelID;
  const returnPath = useMemo(() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  }, [location.state]);
  const copyFromId = useMemo(() => {
    if (hasChannelID) return '';
    const query = new URLSearchParams(location.search);
    return (query.get('copy_from') || '').trim();
  }, [hasChannelID, location.search]);
  const creatingChannelIdFromQuery = useMemo(() => {
    if (hasChannelID) return '';
    const query = new URLSearchParams(location.search);
    return (query.get('channel_id') || '').trim();
  }, [hasChannelID, location.search]);
  const [loading, setLoading] = useState(
    hasChannelID || copyFromId !== '' || creatingChannelIdFromQuery !== '',
  );
  const [createStep, setCreateStep] = useState(() => {
    const query = new URLSearchParams(location.search);
    return parseCreateStep(query.get('step'));
  });
  const [creatingChannelId, setCreatingChannelId] = useState(
    creatingChannelIdFromQuery,
  );
  const [channelKeySet, setChannelKeySet] = useState(false);
  const handleCancel = () => {
    if (isDetailMode && returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/admin/channel');
  };
  const openChannelTaskView = useCallback(
    (extraParams = {}) => {
      const targetChannelId = (channelId || creatingChannelId || '')
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
      navigate(`/admin/channel/tasks${search ? `?${search}` : ''}`);
    },
    [channelId, creatingChannelId, navigate],
  );
  const [inputs, setInputs] = useState(CHANNEL_ORIGIN_INPUTS);
  const [channelProtocolOptions, setChannelProtocolOptions] = useState(() =>
    getChannelProtocolOptions(),
  );
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [modelsSyncError, setModelsSyncError] = useState('');
  const [modelsLastSyncedAt, setModelsLastSyncedAt] = useState(0);
  const [verifiedModelSignature, setVerifiedModelSignature] = useState('');
  const [modelTestResults, setModelTestResults] = useState([]);
  const [modelTesting, setModelTesting] = useState(false);
  const [modelTestingScope, setModelTestingScope] = useState('');
  const [modelTestingTargets, setModelTestingTargets] = useState([]);
  const [channelTasks, setChannelTasks] = useState([]);
  const [modelTestError, setModelTestError] = useState('');
  const [modelTestedAt, setModelTestedAt] = useState(0);
  const [modelTestedSignature, setModelTestedSignature] = useState('');
  const [modelTestTargetModels, setModelTestTargetModels] = useState([]);
  const [detailModelMutating, setDetailModelMutating] = useState(false);
  const [detailBasicEditing, setDetailBasicEditing] = useState(false);
  const [detailEditingModelKey, setDetailEditingModelKey] = useState('');
  const [detailEditingModelSnapshot, setDetailEditingModelSnapshot] =
    useState(null);
  const [detailBasicSaving, setDetailBasicSaving] = useState(false);
  const [detailAdvancedEditing, setDetailAdvancedEditing] = useState(false);
  const [detailAdvancedSaving, setDetailAdvancedSaving] = useState(false);
  const [config, setConfig] = useState(CHANNEL_DEFAULT_CONFIG);
  const [providerOptions, setProviderOptions] = useState([]);
  const [providerModelOwners, setProviderModelOwners] = useState({});
  const [providerModelDetailsIndex, setProviderModelDetailsIndex] = useState(
    {},
  );
  const [providerCatalogLoading, setProviderCatalogLoading] = useState(false);
  const [providerCatalogLoaded, setProviderCatalogLoaded] = useState(false);
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
  const creatingChannelIdRef = useRef(creatingChannelIdFromQuery);
  const creatingStepProvidedRef = useRef(false);
  const skipNextCreatingReloadRef = useRef('');
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
  const previewChannelID = useMemo(
    () => ((hasChannelID ? channelId : creatingChannelId) || '').trim(),
    [channelId, creatingChannelId, hasChannelID],
  );
  const hasModelPreviewCredentials =
    effectivePreviewKey !== '' || (previewChannelID !== '' && channelKeySet);
  const canReuseStoredKeyForCreate =
    isCreateMode && previewChannelID !== '' && channelKeySet;
  const currentModelSignature = useMemo(
    () =>
      buildChannelConnectionSignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: inputs.base_url,
        channelID: previewChannelID,
      }),
    [effectivePreviewKey, inputs.base_url, inputs.protocol, previewChannelID],
  );
  const currentModelTestSignature = useMemo(
    () =>
      buildChannelModelTestSignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: inputs.base_url,
        channelID: previewChannelID,
        models: inputs.models,
        modelConfigs: inputs.model_configs,
      }),
    [
      effectivePreviewKey,
      inputs.base_url,
      inputs.model_configs,
      inputs.models,
      inputs.protocol,
      previewChannelID,
    ],
  );
  const requiresConnectionVerification =
    isCreateMode && inputs.protocol !== 'proxy';
  const showAllSections = hasChannelID;
  const showStepOne = showAllSections || createStep === 1;
  const showStepTwo = showAllSections || createStep === 2;
  const showStepThree = showAllSections || createStep === 3;
  const showStepFour = showAllSections || createStep === 4;
  const isCurrentSignatureVerified =
    requiresConnectionVerification &&
    verifiedModelSignature !== '' &&
    currentModelSignature === verifiedModelSignature;
  const requireVerificationBeforeProceed =
    requiresConnectionVerification && inputs.models.length === 0;
  const fetchModelsButtonText = t('channel.edit.buttons.fetch_models');
  const detailBasicReadonly = isDetailMode && !detailBasicEditing;
  const detailModelsEditing =
    isDetailMode && detailEditingModelKey.toString().trim() !== '';
  const detailAdvancedReadonly = isDetailMode && !detailAdvancedEditing;
  const isAnyDetailSectionEditing =
    detailBasicEditing || detailModelsEditing || detailAdvancedEditing;
  const detailBasicEditLocked =
    isDetailMode &&
    !detailBasicEditing &&
    (detailModelsEditing || detailAdvancedEditing);
  const detailModelsEditLocked =
    isDetailMode &&
    (detailBasicEditing || detailAdvancedEditing);
  const detailAdvancedEditLocked =
    isDetailMode &&
    !detailAdvancedEditing &&
    (detailBasicEditing || detailModelsEditing);
  const detailTestingReadonly = isDetailMode && isAnyDetailSectionEditing;
  const inputReadonlyProps = detailBasicReadonly ? { readOnly: true } : {};
  const visibleModelConfigs = useMemo(
    () => normalizeChannelModelConfigs(inputs.model_configs),
    [inputs.model_configs],
  );
  const detailEditingModelRow = useMemo(() => {
    if (!detailModelsEditing) {
      return null;
    }
    return (
      visibleModelConfigs.find(
        (row) => row.upstream_model === detailEditingModelKey,
      ) || null
    );
  }, [detailEditingModelKey, detailModelsEditing, visibleModelConfigs]);
  const modelTestResultsByKey = useMemo(() => {
    const index = new Map();
    normalizeModelTestResults(modelTestResults).forEach((item) => {
      const key = buildModelTestResultKey(item.model, item.endpoint);
      if (!item.model || !item.endpoint || key === '::') {
        return;
      }
      index.set(key, item);
    });
    return index;
  }, [modelTestResults]);
  const testedEndpointsByModel = useMemo(() => {
    const index = new Map();
    normalizeModelTestResults(modelTestResults).forEach((item) => {
      const modelName = (item.model || '').toString().trim();
      const endpoint = (item.endpoint || '').toString().trim();
      if (modelName === '' || endpoint === '') {
        return;
      }
      const existing = index.get(modelName) || new Set();
      existing.add(endpoint);
      index.set(modelName, existing);
    });
    return index;
  }, [modelTestResults]);
  const modelTestRows = useMemo(() => {
    return visibleModelConfigs.filter((row) => {
      const endpoint = normalizeChannelModelEndpoint(row.type, row.endpoint);
      const resultKey = buildModelTestResultKey(row.model, endpoint);
      if (row.inactive) {
        return false;
      }
      if (row.selected) {
        return true;
      }
      return modelTestResultsByKey.has(resultKey);
    });
  }, [modelTestResultsByKey, visibleModelConfigs]);
  const allModelTestTargetsSelected = useMemo(() => {
    if (modelTestRows.length === 0) {
      return false;
    }
    return modelTestRows.every((row) =>
      modelTestTargetModels.includes(row.model),
    );
  }, [modelTestRows, modelTestTargetModels]);
  const isModelTestSignatureFresh =
    modelTestedSignature !== '' &&
    modelTestedSignature === currentModelTestSignature;
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
  const getProviderOwnersForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider,
      );
      const owners = new Set();
      if (selectedProvider !== '') {
        owners.add(selectedProvider);
      }
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
      if (selectedProvider !== '') {
        return selectedProvider;
      }
      const providerOwners = getProviderOwnersForModel(row);
      if (providerOwners.length === 1) {
        return providerOwners[0];
      }
      return inferAssignableProviderForRowWithOptions(row, providerOptions);
    },
    [getProviderOwnersForModel, providerOptions],
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
          details.push({
            provider: providerId,
            ...detail,
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
  const canSelectChannelModel = useCallback(
    (row) =>
      row?.inactive !== true && getProviderOwnersForModel(row).length > 0,
    [getProviderOwnersForModel],
  );
  const activeModelConfigs = useMemo(
    () => visibleModelConfigs.filter((row) => row.inactive !== true),
    [visibleModelConfigs],
  );
  const detailModelStats = useMemo(() => {
    return visibleModelConfigs.reduce(
      (acc, row) => {
        const owners = getProviderOwnersForModel(row);
        if (owners.length > 0) {
          acc.assigned += 1;
        } else {
          acc.unassigned += 1;
        }
        return acc;
      },
      {
        assigned: 0,
        unassigned: 0,
      },
    );
  }, [getProviderOwnersForModel, visibleModelConfigs]);
  const detailFilteredModelConfigs = useMemo(() => {
    if (!isDetailMode) {
      return visibleModelConfigs;
    }
    return visibleModelConfigs.filter((row) => {
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
    visibleModelConfigs,
  ]);
  const searchedModelConfigs = useMemo(() => {
    const keyword = normalizeSearchKeyword(deferredModelSearchKeyword);
    if (keyword === '') {
      return detailFilteredModelConfigs;
    }
    return detailFilteredModelConfigs.filter((row) => {
      const providerOwners = getProviderOwnersForModel(row).join(' ');
      const selectedProviderText = getSelectedProviderDisplayItems(row)
        .map((item) => item.text || item.value || '')
        .join(' ');
      const candidates = [
        row?.upstream_model,
        row?.model,
        row?.type,
        providerOwners,
        selectedProviderText,
      ].map(normalizeSearchKeyword);
      return candidates.some((candidate) => candidate.includes(keyword));
    });
  }, [
    deferredModelSearchKeyword,
    detailFilteredModelConfigs,
    getProviderOwnersForModel,
    getSelectedProviderDisplayItems,
  ]);
  const detailModelTotalPages = useMemo(() => {
    return Math.max(
      1,
      Math.ceil(searchedModelConfigs.length / CHANNEL_MODEL_PAGE_SIZE),
    );
  }, [searchedModelConfigs.length]);
  const renderedModelConfigs = useMemo(() => {
    const offset = (detailModelPage - 1) * CHANNEL_MODEL_PAGE_SIZE;
    return searchedModelConfigs.slice(offset, offset + CHANNEL_MODEL_PAGE_SIZE);
  }, [searchedModelConfigs, detailModelPage]);
  const modelSelectionSummaryText = useMemo(
    () =>
      t('channel.edit.model_selector.summary', {
        selected: inputs.models.length,
        total: activeModelConfigs.length,
      }),
    [activeModelConfigs.length, inputs.models.length, t],
  );
  const modelAssignmentSummaryText = useMemo(() => {
    if (!isDetailMode) {
      return '';
    }
    return t('channel.edit.model_selector.assignment_summary', {
      assigned: detailModelStats.assigned,
      unassigned: detailModelStats.unassigned,
    });
  }, [detailModelStats, isDetailMode, t]);
  const modelSectionMetaText = useMemo(() => {
    const parts = [modelSelectionSummaryText];
    if (modelAssignmentSummaryText) {
      parts.push(modelAssignmentSummaryText);
    }
    return parts.filter(Boolean).join(' · ');
  }, [modelAssignmentSummaryText, modelSelectionSummaryText]);

  const handleInputChange = (e, { name, value }) => {
    const nextValue = name === 'id' ? normalizeChannelIdentifier(value) : value;
    setInputs((inputs) => ({ ...inputs, [name]: nextValue }));
  };

  const handleConfigChange = (e, { name, value }) => {
    setConfig((inputs) => ({ ...inputs, [name]: value }));
  };

  const baseURLField = useMemo(() => {
    if (inputs.protocol === 'azure') {
      return (
        <Form.Field>
          <Form.Input
            className='router-section-input'
            label={t('channel.edit.base_url')}
            name='base_url'
            placeholder='请输入 AZURE_OPENAI_ENDPOINT，例如：https://docs-test-001.openai.azure.com'
            onChange={handleInputChange}
            value={inputs.base_url}
            autoComplete='new-password'
            {...inputReadonlyProps}
          />
        </Form.Field>
      );
    }
    if (inputs.protocol === 'custom') {
      return (
        <Form.Field>
          <Form.Input
            className='router-section-input'
            required
            label={t('channel.edit.proxy_url')}
            name='base_url'
            placeholder={t('channel.edit.proxy_url_placeholder')}
            onChange={handleInputChange}
            value={inputs.base_url}
            autoComplete='new-password'
            {...inputReadonlyProps}
          />
        </Form.Field>
      );
    }
    if (inputs.protocol === 'openai') {
      return (
        <Form.Field>
          <Form.Input
            className='router-section-input'
            label={t('channel.edit.base_url')}
            name='base_url'
            placeholder={t('channel.edit.base_url_placeholder')}
            onChange={handleInputChange}
            value={inputs.base_url}
            autoComplete='new-password'
            {...inputReadonlyProps}
          />
        </Form.Field>
      );
    }
    if (inputs.protocol === 'fastgpt') {
      return (
        <Form.Field>
          <Form.Input
            className='router-section-input'
            label={t('channel.edit.base_url')}
            name='base_url'
            placeholder={
              '请输入私有部署地址，格式为：https://fastgpt.run' +
              '/api' +
              '/openapi'
            }
            onChange={handleInputChange}
            value={inputs.base_url}
            autoComplete='new-password'
            {...inputReadonlyProps}
          />
        </Form.Field>
      );
    }
    if (inputs.protocol !== 'awsclaude') {
      return (
        <Form.Field>
          <Form.Input
            className='router-section-input'
            label={t('channel.edit.proxy_url')}
            name='base_url'
            placeholder={t('channel.edit.proxy_url_placeholder')}
            onChange={handleInputChange}
            value={inputs.base_url}
            autoComplete='new-password'
            {...inputReadonlyProps}
          />
        </Form.Field>
      );
    }
    return null;
  }, [
    handleInputChange,
    inputReadonlyProps,
    inputs.base_url,
    inputs.protocol,
    t,
  ]);

  const keyField = useMemo(() => {
    if (inputs.protocol === 'awsclaude' || inputs.protocol === 'vertexai') {
      return null;
    }
    return (
      <Form.Field>
        <Form.Input
          className='router-section-input'
          label={t('channel.edit.key')}
          name='key'
          type='password'
          required={isCreateMode && !canReuseStoredKeyForCreate}
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
      </Form.Field>
    );
  }, [
    canReuseStoredKeyForCreate,
    channelKeySet,
    handleInputChange,
    inputReadonlyProps,
    inputs.key,
    inputs.protocol,
    isCreateMode,
    t,
  ]);

  const clearCreateChannelCache = useCallback(() => {
    if (typeof window === 'undefined') {
      return;
    }
    localStorage.removeItem(CHANNEL_CREATE_CACHE_KEY);
  }, []);

  const restoreCreateChannelCache = useCallback(() => {
    if (typeof window === 'undefined') {
      return false;
    }
    const raw = localStorage.getItem(CHANNEL_CREATE_CACHE_KEY);
    if (!raw) {
      return false;
    }
    try {
      const cachedState = JSON.parse(raw);
      if (!cachedState || typeof cachedState !== 'object') {
        return false;
      }
      if (!cachedState.inputs || typeof cachedState.inputs !== 'object') {
        return false;
      }

      setInputs({
        ...CHANNEL_ORIGIN_INPUTS,
        ...sanitizeCreateInputsForLocalStorage(cachedState.inputs),
      });
      if (cachedState.config && typeof cachedState.config === 'object') {
        setConfig({
          ...CHANNEL_DEFAULT_CONFIG,
          ...sanitizeCreateConfigForLocalStorage(cachedState.config),
        });
      }
      if (typeof cachedState.modelsSyncError === 'string') {
        setModelsSyncError(cachedState.modelsSyncError);
      }
      if (Number.isFinite(cachedState.modelsLastSyncedAt)) {
        setModelsLastSyncedAt(cachedState.modelsLastSyncedAt);
      }
      if (typeof cachedState.verifiedModelSignature === 'string') {
        setVerifiedModelSignature(cachedState.verifiedModelSignature);
      }
      const restoredModelTestResults = Array.isArray(
        cachedState.modelTestResults,
      )
        ? cachedState.modelTestResults
        : Array.isArray(cachedState.capabilityResults)
          ? cachedState.capabilityResults
          : [];
      const restoredModelTestTargetModels = Array.isArray(
        cachedState.modelTestTargetModels,
      )
        ? cachedState.modelTestTargetModels
        : Array.isArray(cachedState.capabilityTargetModels)
          ? cachedState.capabilityTargetModels
          : [];
      const restoredModelTestError =
        typeof cachedState.modelTestError === 'string'
          ? cachedState.modelTestError
          : typeof cachedState.capabilityTestError === 'string'
            ? cachedState.capabilityTestError
            : '';
      const restoredModelTestedAt = Number.isFinite(cachedState.modelTestedAt)
        ? cachedState.modelTestedAt
        : Number.isFinite(cachedState.capabilityTestedAt)
          ? cachedState.capabilityTestedAt
          : 0;
      const restoredModelTestedSignature =
        typeof cachedState.modelTestedSignature === 'string'
          ? cachedState.modelTestedSignature
          : typeof cachedState.capabilityTestedSignature === 'string'
            ? cachedState.capabilityTestedSignature
            : '';
      if (restoredModelTestResults.length > 0) {
        setModelTestResults(
          normalizeModelTestResults(restoredModelTestResults),
        );
      }
      if (restoredModelTestTargetModels.length > 0) {
        setModelTestTargetModels(
          normalizeModelIDs(restoredModelTestTargetModels),
        );
      }
      if (restoredModelTestError !== '') {
        setModelTestError(restoredModelTestError);
      }
      if (restoredModelTestedAt > 0) {
        setModelTestedAt(restoredModelTestedAt);
      }
      if (restoredModelTestedSignature !== '') {
        setModelTestedSignature(restoredModelTestedSignature);
      }
      if (typeof cachedState.channel_id === 'string') {
        const restoredChannelID = cachedState.channel_id.trim();
        setCreatingChannelId(restoredChannelID);
        creatingChannelIdRef.current = restoredChannelID;
        skipNextCreatingReloadRef.current = restoredChannelID;
      }
      if (typeof cachedState.channel_key_set === 'boolean') {
        setChannelKeySet(cachedState.channel_key_set);
      } else {
        setChannelKeySet(false);
      }
      setCreateStep(parseCreateStep(cachedState.step));
      return true;
    } catch {
      return false;
    }
  }, []);

  const goToCreateStep = useCallback(
    (targetStep) => {
      if (!isCreateMode) {
        return;
      }
      setCreateStep(parseCreateStep(targetStep));
    },
    [isCreateMode],
  );

  const moveToPreviousCreateStep = useCallback(() => {
    goToCreateStep(createStep - 1);
  }, [createStep, goToCreateStep]);

  const buildChannelPayloadFromState = useCallback(
    (baseInputs, baseConfig) => {
      const effectiveKey = buildEffectiveKey();
      const derivedModelState = buildChannelModelState(
        baseInputs.model_configs,
      );
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
      localInputs.model_configs = derivedModelState.modelConfigs;
      localInputs.models = derivedModelState.selectedModels.join(',');
      const submitConfig = { ...baseConfig };
      localInputs.config = JSON.stringify(submitConfig);
      return localInputs;
    },
    [buildEffectiveKey],
  );

  const buildChannelPayload = useCallback(() => {
    return buildChannelPayloadFromState(inputs, config);
  }, [buildChannelPayloadFromState, config, inputs]);

  const createChannelRecord = useCallback(async () => {
    const payload = buildChannelPayload();
    const res = await API.post('/api/v1/admin/channel/create', {
      name: payload.name,
      protocol: payload.protocol,
      key: payload.key,
      base_url: payload.base_url,
      config: payload.config,
    });
    const { success, message, data } = res.data || {};
    if (!success) {
      showError(message || t('channel.edit.messages.create_channel_failed'));
      return '';
    }
    const id = (data?.id || '').toString();
    if (id === '') {
      showError(t('channel.edit.messages.create_channel_failed'));
      return '';
    }
    setCreatingChannelId(id);
    creatingChannelIdRef.current = id;
    skipNextCreatingReloadRef.current = id;
    if ((payload.key || '').trim() !== '') {
      setChannelKeySet(true);
    }
    return id;
  }, [buildChannelPayload, t]);

  const persistWorkingChannel = useCallback(
    async ({ status } = {}) => {
      let targetChannelID = (
        (hasChannelID
          ? channelId
          : creatingChannelIdRef.current || creatingChannelId) || ''
      ).trim();
      if (targetChannelID === '') {
        if (!isCreateMode) {
          return '';
        }
        const createdID = await createChannelRecord();
        if (createdID === '') {
          return '';
        }
        targetChannelID = createdID;
      }
      const payload = buildChannelPayload();
      const requestBody = {
        ...payload,
        id: targetChannelID,
      };
      if (typeof status === 'number') {
        requestBody.status = status;
      } else if (isCreateMode) {
        requestBody.status = 4;
      }
      const res = await API.put('/api/v1/admin/channel/', requestBody);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('channel.edit.messages.save_channel_failed'));
        return '';
      }
      if ((payload.key || '').trim() !== '') {
        setChannelKeySet(true);
      }
      return targetChannelID;
    },
    [
      buildChannelPayload,
      channelId,
      createChannelRecord,
      creatingChannelId,
      hasChannelID,
      isCreateMode,
      t,
    ],
  );

  const saveCreatingChannel = useCallback(async () => {
    const targetChannelID = await persistWorkingChannel({ status: 4 });
    return targetChannelID !== '';
  }, [persistWorkingChannel]);

  const persistDetailModelConfigs = useCallback(
    async (nextModelConfigs) => {
      if (!isDetailMode) {
        return true;
      }
      const targetChannelID = (channelId || creatingChannelId || '')
        .toString()
        .trim();
      if (targetChannelID === '') {
        return false;
      }
      const nextInputs = buildNextInputsWithModelConfigs(
        inputs,
        nextModelConfigs,
      );
      const payload = buildChannelPayloadFromState(nextInputs, config);
      setDetailModelMutating(true);
      try {
        const res = await API.put('/api/v1/admin/channel/', {
          ...payload,
          id: targetChannelID,
        });
        const { success, message } = res.data || {};
        if (!success) {
          showError(message || t('channel.edit.messages.save_channel_failed'));
          return false;
        }
        setInputs(nextInputs);
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
      buildChannelPayloadFromState,
      channelId,
      config,
      creatingChannelId,
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
      if (typeof loadingSetter === 'function') {
        loadingSetter(true);
      }
      try {
        const payload = buildChannelPayload();
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
    });
    if (ok) {
      setDetailBasicEditing(false);
    }
  }, [persistDetailChannel, t]);

  const saveDetailModelsConfig = useCallback(async () => {
    if (!detailModelsEditing) {
      return;
    }
    const ok = await persistDetailModelConfigs(visibleModelConfigs);
    if (ok) {
      setDetailEditingModelKey('');
      setDetailEditingModelSnapshot(null);
      showSuccess(t('channel.edit.messages.update_success'));
    }
  }, [detailModelsEditing, persistDetailModelConfigs, t, visibleModelConfigs]);

  const saveDetailAdvancedConfig = useCallback(async () => {
    const ok = await persistDetailChannel({
      loadingSetter: setDetailAdvancedSaving,
      successMessage: t('channel.edit.messages.update_success'),
    });
    if (ok) {
      setDetailAdvancedEditing(false);
    }
  }, [persistDetailChannel, t]);

  const verifyChannelModelsPersisted = useCallback(
    async (expectedModels) => {
      const targetChannelID = (
        creatingChannelIdRef.current ||
        creatingChannelId ||
        ''
      ).trim();
      if (targetChannelID === '') {
        return false;
      }
      try {
        const remoteConfigs =
          await fetchAllChannelModelConfigs(targetChannelID);
        const remoteModels = normalizeModelIDs(
          remoteConfigs
            .filter((row) => row && row.selected === true)
            .map((row) => row.model),
        );
        const localModels = normalizeModelIDs(expectedModels);
        if (remoteModels.length !== localModels.length) {
          return false;
        }
        for (let i = 0; i < localModels.length; i += 1) {
          if (localModels[i] !== remoteModels[i]) {
            return false;
          }
        }
        return true;
      } catch {
        return false;
      }
    },
    [creatingChannelId],
  );

  const ensureCreatingChannel = useCallback(async () => {
    if (!isCreateMode) {
      return true;
    }
    if (creatingChannelId) {
      return saveCreatingChannel();
    }
    const createdID = await createChannelRecord();
    return createdID !== '';
  }, [
    createChannelRecord,
    creatingChannelId,
    isCreateMode,
    saveCreatingChannel,
  ]);

  const moveToStepTwo = useCallback(async () => {
    const effectiveKey = buildEffectiveKey();
    const identifierError = validateChannelIdentifier(inputs.name, t);
    if (identifierError) {
      showInfo(identifierError);
      return;
    }
    if (effectiveKey.trim() === '' && !canReuseStoredKeyForCreate) {
      showInfo(t('channel.edit.messages.key_required'));
      return;
    }
    if (isCreateMode) {
      const ok = await ensureCreatingChannel();
      if (!ok) {
        return;
      }
    }
    goToCreateStep(2);
  }, [
    buildEffectiveKey,
    canReuseStoredKeyForCreate,
    ensureCreatingChannel,
    goToCreateStep,
    inputs.name,
    isCreateMode,
    t,
  ]);

  const ensureModelsStepCompleted = useCallback(async () => {
    const modelConfigError = validateModelConfigs(visibleModelConfigs, t);
    if (modelConfigError) {
      showInfo(modelConfigError);
      return false;
    }
    if (requireVerificationBeforeProceed) {
      if (!hasModelPreviewCredentials) {
        showInfo(t('channel.edit.model_selector.verify_prerequisite'));
        return false;
      }
      if (!isCurrentSignatureVerified) {
        showInfo(t('channel.edit.model_selector.verify_required'));
        return false;
      }
    }
    if (inputs.protocol !== 'proxy' && inputs.models.length === 0) {
      showInfo(t('channel.edit.messages.models_required'));
      return false;
    }
    if (isCreateMode) {
      const ok = await saveCreatingChannel();
      if (!ok) {
        return false;
      }
      const expectedModels = [...inputs.models];
      const persisted = await verifyChannelModelsPersisted(expectedModels);
      if (!persisted) {
        showError(t('channel.edit.messages.save_channel_failed'));
        return false;
      }
    }
    return true;
  }, [
    creatingChannelId,
    hasModelPreviewCredentials,
    inputs.models.length,
    inputs.models,
    inputs.protocol,
    isCreateMode,
    isCurrentSignatureVerified,
    requireVerificationBeforeProceed,
    requiresConnectionVerification,
    saveCreatingChannel,
    t,
    verifyChannelModelsPersisted,
    visibleModelConfigs,
  ]);

  const moveToStepThree = useCallback(async () => {
    const ok = await ensureModelsStepCompleted();
    if (!ok) {
      return;
    }
    goToCreateStep(3);
  }, [ensureModelsStepCompleted, goToCreateStep]);

  const moveToStepFour = useCallback(async () => {
    if (createStep <= 2) {
      const ok = await ensureModelsStepCompleted();
      if (!ok) {
        return;
      }
    }
    goToCreateStep(4);
  }, [createStep, ensureModelsStepCompleted, goToCreateStep]);

  const loadChannelModelConfigsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchAllChannelModelConfigs(targetChannelId);
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
      const [nextModelConfigs, nextTests, nextTasks] = await Promise.all([
        loadChannelModelConfigsFromServer(normalizedChannelId),
        loadChannelTestsFromServer(normalizedChannelId),
        loadChannelTasksFromServer(normalizedChannelId),
      ]);
      const nextInputs = buildNextInputsWithModelConfigs(
        inputs,
        nextModelConfigs,
      );
      const nextSignature = buildChannelModelTestSignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: inputs.base_url,
        channelID: normalizedChannelId,
        models: nextInputs.models,
        modelConfigs: nextInputs.model_configs,
      });
      setInputs(nextInputs);
      setModelTestResults((prev) =>
        mergeModelTestResults(prev, nextTests.items),
      );
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
    },
    [
      effectivePreviewKey,
      inputs,
      inputs.base_url,
      inputs.protocol,
      loadChannelModelConfigsFromServer,
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
          const [remoteModelConfigs, channelTestsData, activeTasks] =
            await Promise.all([
              loadChannelModelConfigsFromServer(data.id || targetId),
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
          const modelState = buildChannelModelState(remoteModelConfigs);
          const loadedModelTestSignature = buildChannelModelTestSignature({
            protocol: normalizedProtocol,
            key: '',
            baseURL: data.base_url || '',
            channelID: data.id || targetId,
            models: modelState.selectedModels,
            modelConfigs: modelState.modelConfigs,
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
              model_configs: modelState.modelConfigs,
              system_prompt: data.system_prompt || '',
              models: modelState.selectedModels,
              test_model: data.test_model || modelState.selectedModels[0] || '',
              created_time: 0,
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
              model_configs: modelState.modelConfigs,
              system_prompt: data.system_prompt || '',
              models: modelState.selectedModels,
              test_model: data.test_model || modelState.selectedModels[0] || '',
              status: data.status,
              weight: data.weight,
              priority: data.priority,
              created_time: Number(data.created_time || 0),
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
          setConfig((prev) => ({
            ...prev,
            ...parsedConfig,
          }));
          if (fromCreating || hasChannelID) {
            setChannelKeySet(!!data.key_set);
          } else {
            setChannelKeySet(false);
          }
          if (fromCreating && !creatingStepProvidedRef.current) {
            setCreateStep(
              inferCreatingChannelStepFromPayload({
                ...data,
                model_configs: modelState.modelConfigs,
                channel_tests: storedModelTestResults,
                channel_tests_last_tested_at: channelTestsData.lastTestedAt,
              }),
            );
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
      loadChannelModelConfigsFromServer,
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
        buildNextInputsWithModelConfigs(
          prev,
          visibleModelConfigs.map((row) =>
            row.upstream_model === detailEditingModelKey
              ? { ...detailEditingModelSnapshot }
              : row,
          ),
        ),
      );
    }
    setDetailEditingModelKey('');
    setDetailEditingModelSnapshot(null);
  }, [
    detailEditingModelKey,
    detailEditingModelSnapshot,
    detailModelsEditing,
    visibleModelConfigs,
  ]);

  const cancelDetailAdvancedEdit = useCallback(async () => {
    if (!isDetailMode || !channelId) {
      setDetailAdvancedEditing(false);
      return;
    }
    setLoading(true);
    setDetailAdvancedEditing(false);
    await loadChannelById(channelId, false, false);
  }, [channelId, isDetailMode, loadChannelById]);

  const handleFetchModels = useCallback(
    async ({ silent = false } = {}) => {
      if (fetchingModelsRef.current) {
        return false;
      }
      fetchingModelsRef.current = true;
      setFetchModelsLoading(true);
      try {
        const persistedChannelId = await persistWorkingChannel({
          status: isCreateMode ? 4 : undefined,
        });
        if (persistedChannelId === '') {
          return false;
        }
        const targetChannelId = (
          persistedChannelId ||
          previewChannelID ||
          creatingChannelIdRef.current ||
          creatingChannelId ||
          ''
        ).trim();
        if (targetChannelId === '') {
          const errorMessage = t('channel.edit.messages.save_channel_failed');
          setModelsSyncError(errorMessage);
          if (!silent) {
            showError(errorMessage);
          }
          return false;
        }
        const normalizedBaseURL = normalizeBaseURL(inputs.base_url);
        const key = buildEffectiveKey().trim();
        const requestSignature = buildChannelConnectionSignature({
          protocol: inputs.protocol,
          key,
          baseURL: normalizedBaseURL,
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
      creatingChannelId,
      inputs.base_url,
      inputs,
      inputs.protocol,
      isCreateMode,
      loadChannelModelConfigsFromServer,
      loadChannelTasksFromServer,
      persistWorkingChannel,
      previewChannelID,
      t,
    ],
  );

  const fetchChannelTypes = useCallback(async () => {
    const options = await loadChannelProtocolOptions();
    if (Array.isArray(options) && options.length > 0) {
      setChannelProtocolOptions(options);
    }
  }, []);

  const loadProviderCatalogIndex = useCallback(
    async ({ silent = true, force = false } = {}) => {
      if (providerCatalogLoading) {
        return null;
      }
      if (providerCatalogLoaded && !force && providerOptions.length > 0) {
        return {
          providerOptions,
          modelOwners: providerModelOwners,
          providerModelDetails: providerModelDetailsIndex,
        };
      }
      setProviderCatalogLoading(true);
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
        const nextCatalog = buildProviderCatalogIndex(items);
        setProviderOptions(nextCatalog.providerOptions);
        setProviderModelOwners(nextCatalog.modelOwners);
        setProviderModelDetailsIndex(nextCatalog.providerModelDetails);
        setProviderCatalogLoaded(true);
        return nextCatalog;
      } catch (error) {
        if (!silent) {
          showError(
            error?.message ||
              t('channel.edit.model_selector.provider_load_failed'),
          );
        }
        return null;
      } finally {
        setProviderCatalogLoading(false);
      }
    },
    [
      providerCatalogLoaded,
      providerModelDetailsIndex,
      providerCatalogLoading,
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
        visibleModelConfigs.find(
          (row) => row.upstream_model === targetModel,
        ) || null;
      if (!currentRow) {
        return;
      }
      if (
        providerOptions.length === 0 &&
        !providerCatalogLoaded &&
        !providerCatalogLoading
      ) {
        loadProviderCatalogIndex({ silent: true }).then();
      }
      const preferredProvider = resolvePreferredProviderForModel(currentRow);
      if (
        normalizeChannelModelProviderValue(currentRow.provider) === '' &&
        preferredProvider !== ''
      ) {
        setInputs((prev) =>
          buildNextInputsWithModelConfigs(
            prev,
            visibleModelConfigs.map((row) =>
              row.upstream_model === targetModel
                ? { ...row, provider: preferredProvider }
                : row,
            ),
          ),
        );
      }
      setDetailEditingModelKey(targetModel);
      setDetailEditingModelSnapshot({ ...currentRow });
    },
    [
      loadProviderCatalogIndex,
      providerCatalogLoaded,
      providerCatalogLoading,
      providerOptions.length,
      resolvePreferredProviderForModel,
      visibleModelConfigs,
    ],
  );

  const openAppendProviderModal = useCallback(
    async (row) => {
      const catalog = await loadProviderCatalogIndex({
        silent: false,
        force: true,
      });
      if (!catalog) {
        return;
      }
      if (catalog.providerOptions.length === 0) {
        showInfo(t('channel.edit.model_selector.provider_no_options'));
        return;
      }
      setAppendProviderForm({
        provider: inferAssignableProviderForRowWithOptions(
          row,
          catalog.providerOptions,
        ),
        model: (row?.upstream_model || row?.model || '').toString().trim(),
        type: normalizeChannelModelType(row?.type),
      });
      setAppendProviderModalOpen(true);
    },
    [loadProviderCatalogIndex, t],
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
      await loadProviderCatalogIndex({ silent: true, force: true });
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
    loadProviderCatalogIndex,
    t,
  ]);

  const handleRunModelTests = useCallback(
    async ({ targetModels = [], scope = 'batch' } = {}) => {
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
      const persistedChannelId = await persistWorkingChannel({
        status: isCreateMode ? 4 : undefined,
      });
      if (persistedChannelId === '') {
        return;
      }
      const targetChannelId = (
        persistedChannelId ||
        previewChannelID ||
        creatingChannelIdRef.current ||
        creatingChannelId ||
        ''
      ).trim();
      const targetConfigs = visibleModelConfigs
        .filter((row) => normalizedTargets.includes(row.model))
        .map((row) => ({
          model: row.model,
          endpoint: normalizeChannelModelEndpoint(row.type, row.endpoint),
        }));
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
      modelTestTargetModels,
      creatingChannelId,
      inputs.base_url,
      inputs,
      inputs.models,
      inputs.protocol,
      inputs.test_model,
      detailTestingReadonly,
      isCreateMode,
      persistWorkingChannel,
      previewChannelID,
      t,
      visibleModelConfigs,
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

  const toggleAllModelTestTargets = useCallback(
    (checked) => {
      if (detailTestingReadonly) {
        return;
      }
      if (!checked) {
        setModelTestTargetModels([]);
        return;
      }
      setModelTestTargetModels(modelTestRows.map((row) => row.model));
    },
    [detailTestingReadonly, modelTestRows],
  );

  const updateModelTestEndpoint = useCallback(
    async (modelName, endpoint) => {
      if (detailTestingReadonly) {
        return;
      }
      const nextConfigs = visibleModelConfigs.map((row) => {
        if (row.model !== modelName) {
          return row;
        }
        return {
          ...row,
          endpoint: normalizeChannelModelEndpoint(row.type, endpoint),
        };
      });
      if (isDetailMode) {
        await persistDetailModelConfigs(nextConfigs);
        return;
      }
      setInputs((prev) => buildNextInputsWithModelConfigs(prev, nextConfigs));
    },
    [
      detailTestingReadonly,
      isDetailMode,
      persistDetailModelConfigs,
      visibleModelConfigs,
    ],
  );

  const handleDownloadModelTestArtifact = useCallback(
    async (resultItem) => {
      const normalizedChannelId = (
        resultItem?.channel_id ||
        channelId ||
        previewChannelID ||
        ''
      )
        .toString()
        .trim();
      const normalizedModel = (resultItem?.model || '').toString().trim();
      const normalizedEndpoint = (resultItem?.endpoint || '')
        .toString()
        .trim();
      if (
        normalizedChannelId === '' ||
        normalizedModel === '' ||
        normalizedEndpoint === ''
      ) {
        showError(t('channel.edit.model_tester.download_unavailable'));
        return;
      }
      try {
        const response = await API.get(
          `/api/v1/admin/channel/${normalizedChannelId}/tests/artifact`,
          {
            params: {
              model: normalizedModel,
              endpoint: normalizedEndpoint,
            },
            responseType: 'blob',
          },
        );
        const responseContentType = (
          response.headers?.['content-type'] || ''
        ).toString();
        if (responseContentType.includes('application/json')) {
          const text = await response.data.text();
          let parsed = null;
          try {
            parsed = JSON.parse(text);
          } catch {
            parsed = null;
          }
          showError(
            parsed?.message ||
              parsed?.error?.message ||
              t('channel.edit.model_tester.download_failed'),
          );
          return;
        }
        const blob = new Blob([response.data], {
          type:
            responseContentType ||
            resultItem?.artifact_content_type ||
            'application/octet-stream',
        });
        const downloadUrl = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = parseDownloadFilename(
          response.headers?.['content-disposition'],
          resultItem?.artifact_name || `${normalizedModel}.bin`,
        );
        document.body.appendChild(link);
        link.click();
        link.remove();
        window.URL.revokeObjectURL(downloadUrl);
      } catch (error) {
        showError(
          error?.response?.data?.message ||
            error?.message ||
            t('channel.edit.model_tester.download_failed'),
        );
      }
    },
    [channelId, previewChannelID, t],
  );

  const toggleModelSelection = useCallback(
    async (upstreamModel, checked) => {
      const nextConfigs = visibleModelConfigs.map((row) =>
        row.upstream_model === upstreamModel && canSelectChannelModel(row)
          ? { ...row, selected: !!checked }
          : row,
      );
      if (isDetailMode) {
        if (
          detailModelsEditing &&
          detailEditingModelKey === (upstreamModel || '').toString().trim()
        ) {
          setInputs((prev) => buildNextInputsWithModelConfigs(prev, nextConfigs));
          return;
        }
        await persistDetailModelConfigs(nextConfigs);
        return;
      }
      setInputs((prev) => buildNextInputsWithModelConfigs(prev, nextConfigs));
    },
    [
      canSelectChannelModel,
      detailEditingModelKey,
      detailModelsEditing,
      isDetailMode,
      persistDetailModelConfigs,
      visibleModelConfigs,
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
        buildNextInputsWithModelConfigs(
          prev,
          visibleModelConfigs.map((row) => {
            if (row.upstream_model !== targetModel) {
              return row;
            }
            if (field === 'model') {
              const alias = (value || '').toString().trim();
              const targetAlias = alias || row.upstream_model;
              const duplicated = visibleModelConfigs.some(
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
            if (field === 'provider') {
              return {
                ...row,
                provider: normalizeChannelModelProviderValue(value),
              };
            }
            return {
              ...row,
              [field]: value,
            };
          }),
        ),
      );
    },
    [detailEditingModelKey, detailModelsEditing, isDetailMode, visibleModelConfigs],
  );

  const selectAllModels = useCallback(() => {
    const nextConfigs = visibleModelConfigs.map((row) => ({
      ...row,
      selected: canSelectChannelModel(row),
    }));
    if (isDetailMode) {
      return;
    }
    setInputs((prev) => buildNextInputsWithModelConfigs(prev, nextConfigs));
  }, [
    canSelectChannelModel,
    isDetailMode,
    visibleModelConfigs,
  ]);

  const clearSelectedModels = useCallback(() => {
    const nextConfigs = visibleModelConfigs.map((row) => ({
      ...row,
      selected: false,
    }));
    if (isDetailMode) {
      return;
    }
    setInputs((prev) => buildNextInputsWithModelConfigs(prev, nextConfigs));
  }, [isDetailMode, visibleModelConfigs]);

  useEffect(() => {
    const selectedModels = visibleModelConfigs
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
  }, [inputs.test_model, visibleModelConfigs]);

  useEffect(() => {
    if (hasChannelID) {
      creatingStepProvidedRef.current = false;
      return;
    }
    const query = new URLSearchParams(location.search);
    creatingStepProvidedRef.current = query.get('step') !== null;
  }, [hasChannelID, location.search]);

  useEffect(() => {
    if (!isDetailMode) {
      setDetailBasicEditing(false);
      setDetailEditingModelKey('');
      setDetailEditingModelSnapshot(null);
      setDetailAdvancedEditing(false);
    }
  }, [isDetailMode]);

  useEffect(() => {
    if (hasChannelID) {
      return;
    }
    if (creatingChannelIdFromQuery === creatingChannelId) {
      return;
    }
    setCreatingChannelId(creatingChannelIdFromQuery);
    creatingChannelIdRef.current = creatingChannelIdFromQuery;
  }, [creatingChannelIdFromQuery, creatingChannelId, hasChannelID]);

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
    if (creatingChannelIdFromQuery !== '') {
      if (skipNextCreatingReloadRef.current === creatingChannelIdFromQuery) {
        skipNextCreatingReloadRef.current = '';
        setLoading(false);
        return;
      }
      setLoading(true);
      loadChannelById(creatingChannelIdFromQuery, false, true).then();
      return;
    }
    setChannelKeySet(false);
    restoreCreateChannelCache();
    setLoading(false);
  }, [
    channelId,
    copyFromId,
    creatingChannelIdFromQuery,
    hasChannelID,
    loadChannelById,
    restoreCreateChannelCache,
  ]);

  useEffect(() => {
    const targetChannelId = (
      (hasChannelID ? channelId : creatingChannelId) || ''
    )
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
    creatingChannelId,
    hasChannelID,
    loadChannelTasksFromServer,
    refreshChannelRuntimeState,
  ]);

  useEffect(() => {
    if (hasChannelID) {
      return;
    }
    const query = new URLSearchParams(location.search);
    const stepParam = query.get('step');
    if (stepParam === null) {
      return;
    }
    const queryStep = parseCreateStep(stepParam);
    setCreateStep((prev) => (prev === queryStep ? prev : queryStep));
  }, [hasChannelID, location.search]);

  useEffect(() => {
    if (hasChannelID) {
      return;
    }
    const query = new URLSearchParams(location.search);
    const stepParam = query.get('step');
    if (createStep <= CREATE_CHANNEL_STEP_MIN) {
      if (stepParam === null) {
        return;
      }
      query.delete('step');
    } else {
      const nextStep = String(createStep);
      if (stepParam === nextStep) {
        return;
      }
      query.set('step', nextStep);
    }
    const nextSearch = query.toString();
    navigate(
      {
        pathname: location.pathname,
        search: nextSearch ? `?${nextSearch}` : '',
      },
      { replace: true },
    );
  }, [createStep, hasChannelID, location.pathname, location.search, navigate]);

  useEffect(() => {
    if (hasChannelID) {
      return;
    }
    const query = new URLSearchParams(location.search);
    const currentChannelID = (query.get('channel_id') || '').trim();
    const nextChannelID = (creatingChannelId || '').trim();
    if (currentChannelID === nextChannelID) {
      return;
    }
    if (nextChannelID === '') {
      query.delete('channel_id');
    } else {
      query.set('channel_id', nextChannelID);
    }
    const nextSearch = query.toString();
    navigate(
      {
        pathname: location.pathname,
        search: nextSearch ? `?${nextSearch}` : '',
      },
      { replace: true },
    );
  }, [
    creatingChannelId,
    hasChannelID,
    location.pathname,
    location.search,
    navigate,
  ]);

  useEffect(() => {
    if (hasChannelID || loading || typeof window === 'undefined') {
      return;
    }
    const payload = {
      step: createStep,
      inputs: sanitizeCreateInputsForLocalStorage(inputs),
      config: sanitizeCreateConfigForLocalStorage(config),
      modelsSyncError,
      modelsLastSyncedAt,
      verifiedModelSignature,
      modelTestResults,
      modelTestTargetModels,
      modelTestError,
      modelTestedAt,
      modelTestedSignature,
      channel_id: creatingChannelId,
      channel_key_set: channelKeySet,
      savedAt: Date.now(),
    };
    localStorage.setItem(CHANNEL_CREATE_CACHE_KEY, JSON.stringify(payload));
  }, [
    channelKeySet,
    config,
    createStep,
    creatingChannelId,
    inputs,
    hasChannelID,
    loading,
    modelTestResults,
    modelTestTargetModels,
    modelTestError,
    modelTestedAt,
    modelTestedSignature,
    modelsLastSyncedAt,
    modelsSyncError,
    verifiedModelSignature,
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
    if (!showStepTwo) {
      return;
    }
    loadProviderCatalogIndex({ silent: true }).then();
  }, [loadProviderCatalogIndex, showStepTwo]);

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
    const modelConfigError = validateModelConfigs(visibleModelConfigs, t);
    const identifierError = validateChannelIdentifier(inputs.name, t);
    if (!isDetailMode && identifierError !== '') {
      showInfo(identifierError);
      return;
    }
    if (
      isCreateMode &&
      effectiveKey.trim() === '' &&
      !canReuseStoredKeyForCreate
    ) {
      showInfo(t('channel.edit.messages.key_required'));
      return;
    }
    if (requireVerificationBeforeProceed) {
      if (!hasModelPreviewCredentials) {
        showInfo(t('channel.edit.model_selector.verify_prerequisite'));
        return;
      }
      if (!isCurrentSignatureVerified) {
        showInfo(t('channel.edit.model_selector.verify_required'));
        return;
      }
    }
    if (inputs.protocol !== 'proxy' && inputs.models.length === 0) {
      showInfo(t('channel.edit.messages.models_required'));
      return;
    }
    if (modelConfigError) {
      showInfo(modelConfigError);
      return;
    }
    let localInputs = buildChannelPayload();
    let res;
    if (creatingChannelId) {
      res = await API.put(`/api/v1/admin/channel/`, {
        ...localInputs,
        id: creatingChannelId,
        status: 1,
      });
    } else {
      res = await API.post(`/api/v1/admin/channel/`, localInputs);
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('channel.edit.messages.create_success'));
      clearCreateChannelCache();
      navigate('/admin/channel', { replace: true });
      return;
    } else {
      showError(message);
    }
  };

  return (
    <div className='dashboard-container'>
      <Modal
        size='small'
        open={detailModelsEditing}
        onClose={cancelDetailModelsEdit}
        closeOnDimmerClick={!detailModelMutating}
        closeOnEscape={!detailModelMutating}
        className='router-channel-model-editor-modal'
      >
        <Modal.Header>
          {`${t('common.edit')} · ${
            detailEditingModelRow?.upstream_model || '-'
          }`}
        </Modal.Header>
        <Modal.Content>
          {detailEditingModelRow ? (
            <Form className='router-channel-model-editor-form'>
              <div className='router-channel-model-editor-card'>
                <div className='router-channel-model-editor-section-title'>
                  {t('channel.edit.model_selector.editor.info_title')}
                </div>
                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-modal-input'
                    label={t('channel.edit.model_selector.table.name')}
                    value={detailEditingModelRow.upstream_model || '-'}
                    readOnly
                  />
                  <Form.Input
                    className='router-modal-input'
                    label={t('channel.edit.model_selector.table.type')}
                    value={t(
                      `channel.model_types.${normalizeChannelModelType(
                        detailEditingModelRow.type,
                      )}`,
                    )}
                    readOnly
                  />
                </Form.Group>
                <Form.Group widths='equal'>
                  <Form.Input
                    className='router-modal-input'
                    label={t('channel.edit.model_selector.table.alias')}
                    value={detailEditingModelRow.model || ''}
                    onChange={(e, { value }) =>
                      updateModelConfigField(
                        detailEditingModelRow.upstream_model,
                        'model',
                        value || detailEditingModelRow.upstream_model,
                      )
                    }
                  />
                  <Form.Input
                    className='router-modal-input'
                    label={t('channel.edit.model_selector.table.price_unit')}
                    value={detailEditingModelRow.price_unit || '-'}
                    readOnly
                  />
                </Form.Group>
                <Form.Field>
                  <label>{t('channel.edit.model_selector.table.providers')}</label>
                  <div className='router-channel-model-editor-provider-row'>
                    <Dropdown
                      selection
                      fluid
                      className='router-modal-dropdown'
                      placeholder={t(
                        'channel.edit.model_selector.editor.provider_placeholder',
                      )}
                      options={getProviderSelectOptionsForModel(
                        detailEditingModelRow,
                      )}
                      value={resolvePreferredProviderForModel(
                        detailEditingModelRow,
                      )}
                      disabled={
                        providerCatalogLoading ||
                        getProviderSelectOptionsForModel(detailEditingModelRow)
                          .length === 0
                      }
                      onChange={(e, { value }) =>
                        updateModelConfigField(
                          detailEditingModelRow.upstream_model,
                          'provider',
                          value || '',
                        )
                      }
                    />
                    {getProviderSelectOptionsForModel(detailEditingModelRow)
                      .length === 0 ? (
                      <>
                        <span className='router-text-meta'>
                          {t(
                            'channel.edit.model_selector.editor.provider_empty',
                          )}
                        </span>
                        <Button
                          type='button'
                          className='router-inline-button'
                          basic
                          onClick={() =>
                            openAppendProviderModal(detailEditingModelRow)
                          }
                        >
                          {t('channel.edit.model_selector.provider_add')}
                        </Button>
                      </>
                    ) : null}
                  </div>
                </Form.Field>
              </div>

              <div className='router-channel-model-editor-card'>
                <div className='router-channel-model-editor-section-title'>
                  {t('channel.edit.model_selector.editor.status_title')}
                </div>
                <div className='router-channel-model-editor-toggle-row'>
                  <div className='router-channel-model-editor-toggle-copy'>
                    <div className='router-channel-model-editor-toggle-label'>
                      {t('channel.edit.model_selector.table.selected')}
                    </div>
                    <div className='router-channel-model-editor-toggle-hint'>
                      {t('channel.edit.model_selector.editor.status_hint')}
                    </div>
                  </div>
                  <Checkbox
                    toggle
                    checked={!!detailEditingModelRow.selected}
                    disabled={
                      detailModelMutating ||
                      providerCatalogLoading ||
                      (!canSelectChannelModel(detailEditingModelRow) &&
                        !detailEditingModelRow.selected)
                    }
                    onChange={(e, { checked }) =>
                      toggleModelSelection(
                        detailEditingModelRow.upstream_model,
                        checked,
                      )
                    }
                  />
                </div>
              </div>

              <div className='router-channel-model-editor-card'>
                <div className='router-channel-model-editor-section-title'>
                  {t('channel.edit.model_selector.editor.pricing_title')}
                </div>
                <Form.Group widths='equal'>
                  <Form.Field>
                    <label>{t('channel.edit.model_selector.table.input_price')}</label>
                    {getComplexPricingDetailsForModel(detailEditingModelRow).some(
                      (detail) =>
                        (detail.price_components || []).some(
                          (component) => Number(component.input_price || 0) > 0,
                        ),
                    ) ? (
                      <Button
                        type='button'
                        basic
                        className='router-inline-button'
                        onClick={() => openComplexPricingModal(detailEditingModelRow)}
                      >
                        {t('channel.edit.model_selector.pricing_detail_button')}
                      </Button>
                    ) : (
                      <Form.Input
                        className='router-modal-input'
                        type='number'
                        min='0'
                        step='0.01'
                        placeholder='-'
                        value={detailEditingModelRow.input_price ?? ''}
                        onChange={(e, { value }) =>
                          updateModelConfigField(
                            detailEditingModelRow.upstream_model,
                            'input_price',
                            value,
                          )
                        }
                      />
                    )}
                  </Form.Field>
                  <Form.Field>
                    <label>{t('channel.edit.model_selector.table.output_price')}</label>
                    {getComplexPricingDetailsForModel(detailEditingModelRow).some(
                      (detail) =>
                        (detail.price_components || []).some(
                          (component) => Number(component.output_price || 0) > 0,
                        ),
                    ) ? (
                      <Button
                        type='button'
                        basic
                        className='router-inline-button'
                        onClick={() => openComplexPricingModal(detailEditingModelRow)}
                      >
                        {t('channel.edit.model_selector.pricing_detail_button')}
                      </Button>
                    ) : (
                      <Form.Input
                        className='router-modal-input'
                        type='number'
                        min='0'
                        step='0.01'
                        placeholder='-'
                        value={detailEditingModelRow.output_price ?? ''}
                        onChange={(e, { value }) =>
                          updateModelConfigField(
                            detailEditingModelRow.upstream_model,
                            'output_price',
                            value,
                          )
                        }
                      />
                    )}
                  </Form.Field>
                </Form.Group>
              </div>
            </Form>
          ) : null}
        </Modal.Content>
        <Modal.Actions>
          <Button
            type='button'
            className='router-modal-button'
            onClick={cancelDetailModelsEdit}
            disabled={detailModelMutating}
          >
            {t('channel.edit.buttons.cancel')}
          </Button>
          <Button
            type='button'
            className='router-modal-button'
            color='blue'
            loading={detailModelMutating}
            disabled={detailModelMutating}
            onClick={saveDetailModelsConfig}
          >
            {t('channel.edit.buttons.save')}
          </Button>
        </Modal.Actions>
      </Modal>
      <Modal
        size='large'
        open={complexPricingModalOpen}
        onClose={closeComplexPricingModal}
      >
        <Modal.Header>
          {t('channel.edit.model_selector.pricing_detail_title')}
        </Modal.Header>
        <Modal.Content scrolling>
          <div className='router-block-gap-sm'>
            <div className='router-text-meta'>
              {t('channel.edit.model_selector.pricing_detail_model', {
                model:
                  complexPricingModalData?.model ||
                  complexPricingModalData?.alias ||
                  '-',
              })}
            </div>
            {complexPricingModalData?.alias &&
            complexPricingModalData.alias !== complexPricingModalData.model ? (
              <div className='router-text-meta'>
                {t('channel.edit.model_selector.pricing_detail_alias', {
                  alias: complexPricingModalData.alias,
                })}
              </div>
            ) : null}
          </div>
          {(complexPricingModalData?.details || []).length === 0 ? (
            <div className='router-empty-cell'>
              {t('channel.edit.model_selector.pricing_detail_empty')}
            </div>
          ) : (
            (complexPricingModalData?.details || []).map((detail, index) => (
              <div
                key={`${detail.provider || 'provider'}-${detail.model || 'model'}-${index}`}
                className='router-block-gap-sm'
                style={{ marginBottom: '1rem' }}
              >
                <div className='router-toolbar router-block-gap-sm'>
                  <div className='router-toolbar-start'>
                    <Label basic className='router-tag'>
                      {detail.provider || '-'}
                    </Label>
                    <Label basic className='router-tag'>
                      {detail.model || '-'}
                    </Label>
                    <Label basic className='router-tag'>
                      {t(
                        `channel.model_types.${normalizeChannelModelType(
                          detail.type,
                        )}`,
                      )}
                    </Label>
                  </div>
                </div>
                <Table celled compact className='router-detail-subtable'>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.input_price',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.output_price',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.price_unit',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.currency',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.source',
                        )}
                      </Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    <Table.Row>
                      <Table.Cell>{detail.input_price || 0}</Table.Cell>
                      <Table.Cell>{detail.output_price || 0}</Table.Cell>
                      <Table.Cell>{detail.price_unit || '-'}</Table.Cell>
                      <Table.Cell>{detail.currency || 'USD'}</Table.Cell>
                      <Table.Cell>{detail.source || 'manual'}</Table.Cell>
                    </Table.Row>
                  </Table.Body>
                </Table>
                <Table celled compact className='router-detail-subtable'>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.component',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.condition',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.input_price',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.output_price',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.price_unit',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.currency',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.source',
                        )}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t(
                          'channel.edit.model_selector.pricing_detail_table.source_url',
                        )}
                      </Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {detail.price_components.map(
                      (component, componentIndex) => (
                        <Table.Row
                          key={`${detail.provider || 'provider'}-${detail.model || 'model'}-${component.component || 'component'}-${component.condition || 'condition'}-${componentIndex}`}
                        >
                          <Table.Cell>{component.component || '-'}</Table.Cell>
                          <Table.Cell>{component.condition || '-'}</Table.Cell>
                          <Table.Cell>{component.input_price || 0}</Table.Cell>
                          <Table.Cell>{component.output_price || 0}</Table.Cell>
                          <Table.Cell>{component.price_unit || '-'}</Table.Cell>
                          <Table.Cell>{component.currency || 'USD'}</Table.Cell>
                          <Table.Cell>
                            {component.source || 'manual'}
                          </Table.Cell>
                          <Table.Cell>{component.source_url || '-'}</Table.Cell>
                        </Table.Row>
                      ),
                    )}
                  </Table.Body>
                </Table>
              </div>
            ))
          )}
        </Modal.Content>
        <Modal.Actions>
          <Button
            type='button'
            className='router-modal-button'
            onClick={closeComplexPricingModal}
          >
            {t('channel.edit.buttons.cancel')}
          </Button>
        </Modal.Actions>
      </Modal>
      <Modal
        size='tiny'
        open={appendProviderModalOpen}
        onClose={closeAppendProviderModal}
        closeOnDimmerClick={!appendingProviderModel}
      >
        <Modal.Header>
          {t('channel.edit.model_selector.append_dialog.title')}
        </Modal.Header>
        <Modal.Content>
          <Form>
            <Form.Field>
              <label>
                {t('channel.edit.model_selector.append_dialog.provider')}
              </label>
              <Dropdown
                selection
                search={filterProviderOptionsByQuery}
                className='router-modal-dropdown'
                placeholder={t(
                  'channel.edit.model_selector.append_dialog.provider_placeholder',
                )}
                options={providerOptions}
                value={appendProviderForm.provider}
                noResultsMessage={t('common.no_data')}
                onChange={(e, { value }) =>
                  setAppendProviderForm((prev) => ({
                    ...prev,
                    provider: (value || '').toString(),
                  }))
                }
              />
            </Form.Field>
            <Form.Input
              className='router-modal-input'
              label={t('channel.edit.model_selector.append_dialog.model')}
              value={appendProviderForm.model}
              onChange={(e, { value }) =>
                setAppendProviderForm((prev) => ({
                  ...prev,
                  model: value || '',
                }))
              }
            />
            <Form.Select
              className='router-modal-dropdown'
              label={t('channel.edit.model_selector.append_dialog.type')}
              options={CHANNEL_MODEL_TYPE_OPTIONS}
              value={appendProviderForm.type}
              onChange={(e, { value }) =>
                setAppendProviderForm((prev) => ({
                  ...prev,
                  type: normalizeChannelModelType(value),
                }))
              }
            />
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button
            type='button'
            className='router-modal-button'
            onClick={closeAppendProviderModal}
          >
            {t('channel.edit.model_selector.append_dialog.cancel')}
          </Button>
          <Button
            type='button'
            className='router-modal-button'
            color='blue'
            loading={appendingProviderModel}
            disabled={appendingProviderModel}
            onClick={handleAppendModelToProvider}
          >
            {t('channel.edit.model_selector.append_dialog.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
      <Card fluid className='chart-card'>
        <Card.Content>
          {isDetailMode && (
            <div className='router-entity-detail-breadcrumb router-block-gap-sm'>
              <Breadcrumb size='small'>
                <Breadcrumb.Section link onClick={handleCancel}>
                  {t('header.channel')}
                </Breadcrumb.Section>
                <Breadcrumb.Divider icon='right chevron' />
                <Breadcrumb.Section active>
                  {inputs.name || channelId || '-'}
                </Breadcrumb.Section>
              </Breadcrumb>
            </div>
          )}
          {isCreateMode && (
            <div className='router-toolbar-start router-block-gap-sm'>
              <Button
                type='button'
                className='router-page-button'
                onClick={handleCancel}
              >
                {t('channel.edit.buttons.cancel')}
              </Button>
              {createStep > CREATE_CHANNEL_STEP_MIN && (
                <Button
                  type='button'
                  className='router-page-button'
                  onClick={moveToPreviousCreateStep}
                >
                  {t('channel.edit.buttons.previous_step')}
                </Button>
              )}
              {createStep < CREATE_CHANNEL_STEP_MAX ? (
                <Button
                  type='button'
                  className='router-page-button'
                  positive
                  onClick={
                    createStep === 1
                      ? moveToStepTwo
                      : createStep === 2
                        ? moveToStepThree
                        : moveToStepFour
                  }
                >
                  {t('channel.edit.buttons.next_step')}
                </Button>
              ) : (
                <Button
                  type='button'
                  className='router-page-button'
                  positive
                  onClick={submit}
                  disabled={
                    requireVerificationBeforeProceed &&
                    !isCurrentSignatureVerified
                  }
                >
                  {t('channel.edit.buttons.submit')}
                </Button>
              )}
            </div>
          )}
          <Form loading={loading} autoComplete='new-password'>
            {isCreateMode && (
              <div className='router-block-gap-sm'>
                <div className='router-toolbar-start'>
                  <Button
                    type='button'
                    className='router-page-button'
                    basic={createStep !== 1}
                    color={createStep === 1 ? 'blue' : undefined}
                    onClick={() => goToCreateStep(1)}
                  >
                    {t('channel.edit.wizard.step_basic')}
                  </Button>
                  <Button
                    type='button'
                    className='router-page-button'
                    basic={createStep !== 2}
                    color={createStep === 2 ? 'blue' : undefined}
                    onClick={moveToStepTwo}
                  >
                    {t('channel.edit.wizard.step_models')}
                  </Button>
                  <Button
                    type='button'
                    className='router-page-button'
                    basic={createStep !== 3}
                    color={createStep === 3 ? 'blue' : undefined}
                    onClick={moveToStepThree}
                  >
                    {t('channel.edit.wizard.step_model_tests')}
                  </Button>
                  <Button
                    type='button'
                    className='router-page-button'
                    basic={createStep !== 4}
                    color={createStep === 4 ? 'blue' : undefined}
                    onClick={moveToStepFour}
                  >
                    {t('channel.edit.wizard.step_advanced')}
                  </Button>
                </div>
              </div>
            )}
            {showStepOne &&
              (isDetailMode ? (
                <section className='router-entity-detail-section'>
                  <div className='router-entity-detail-section-header'>
                    <div className='router-toolbar-start'>
                      <span className='router-entity-detail-section-title'>
                        {t('channel.edit.detail_basic_title')}
                      </span>
                    </div>
                    <div className='router-toolbar-end'>
                      {detailBasicEditing ? (
                        <>
                          <Button
                            type='button'
                            className='router-page-button'
                            onClick={cancelDetailBasicEdit}
                            disabled={detailBasicSaving}
                          >
                            {t('channel.edit.buttons.cancel')}
                          </Button>
                          <Button
                            type='button'
                            className='router-page-button'
                            color='blue'
                            loading={detailBasicSaving}
                            disabled={detailBasicSaving}
                            onClick={saveDetailBasicInfo}
                          >
                            {t('channel.edit.buttons.save')}
                          </Button>
                        </>
                      ) : (
                        <Button
                          type='button'
                          className='router-page-button'
                          color='blue'
                          disabled={detailBasicEditLocked}
                          onClick={() => setDetailBasicEditing(true)}
                        >
                          {t('common.edit')}
                        </Button>
                      )}
                    </div>
                  </div>
                  <Form.Group widths='equal'>
                    <Form.Input
                      className='router-section-input'
                      label={t('channel.edit.identifier')}
                      name='name'
                      placeholder={t('channel.edit.identifier_placeholder')}
                      onChange={handleInputChange}
                      value={inputs.name}
                      required
                      maxLength={CHANNEL_IDENTIFIER_MAX_LENGTH}
                      readOnly={detailBasicReadonly}
                    />
                    <Form.Field>
                      {detailBasicReadonly ? (
                        <Form.Input
                          className='router-section-input'
                          label={t('channel.edit.type')}
                          value={
                            currentProtocolOption?.text ||
                            inputs.protocol ||
                            '-'
                          }
                          readOnly
                        />
                      ) : (
                        <Form.Select
                          className='router-section-dropdown'
                          label={t('channel.edit.type')}
                          name='protocol'
                          required
                          search
                          options={channelProtocolOptions}
                          value={inputs.protocol}
                          onChange={handleInputChange}
                        />
                      )}
                    </Form.Field>
                  </Form.Group>
                  {baseURLField && keyField ? (
                    <Form.Group widths='equal'>
                      {baseURLField}
                      {keyField}
                    </Form.Group>
                  ) : (
                    <>
                      {baseURLField}
                      {keyField}
                    </>
                  )}
                  {inputs.protocol === 'azure' && (
                    <>
                      <Message className='router-section-message'>
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
                      </Message>
                      <Form.Field>
                        <Form.Input
                          className='router-section-input'
                          label='默认 API 版本'
                          name='other'
                          placeholder='请输入默认 API 版本，例如：2024-03-01-preview，该配置可以被实际的请求查询参数所覆盖'
                          onChange={handleInputChange}
                          value={inputs.other}
                          autoComplete='new-password'
                          {...inputReadonlyProps}
                        />
                      </Form.Field>
                    </>
                  )}
                  {inputs.protocol === 'xunfei' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.spark_version')}
                        name='other'
                        placeholder={t('channel.edit.spark_version_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.other}
                        autoComplete='new-password'
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'aiproxy-library' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.knowledge_id')}
                        name='other'
                        placeholder={t('channel.edit.knowledge_id_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.other}
                        autoComplete='new-password'
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'ali' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.plugin_param')}
                        name='other'
                        placeholder={t('channel.edit.plugin_param_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.other}
                        autoComplete='new-password'
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'coze' && (
                    <Message className='router-section-message'>
                      {t('channel.edit.coze_notice')}
                    </Message>
                  )}
                  {inputs.protocol === 'doubao' && (
                    <Message className='router-section-message'>
                      {t('channel.edit.douban_notice')}
                      <a
                        target='_blank'
                        rel='noreferrer'
                        href='https://console.volcengine.com/ark/region:ark+cn-beijing/endpoint'
                      >
                        {t('channel.edit.douban_notice_link')}
                      </a>
                      {t('channel.edit.douban_notice_2')}
                    </Message>
                  )}
                  {inputs.protocol === 'awsclaude' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label='Region'
                        name='region'
                        required
                        placeholder={t('channel.edit.aws_region_placeholder')}
                        onChange={handleConfigChange}
                        value={config.region}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label='AK'
                        name='ak'
                        required
                        placeholder={t('channel.edit.aws_ak_placeholder')}
                        onChange={handleConfigChange}
                        value={config.ak}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label='SK'
                        name='sk'
                        required
                        placeholder={t('channel.edit.aws_sk_placeholder')}
                        onChange={handleConfigChange}
                        value={config.sk}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'vertexai' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label='Region'
                        name='region'
                        required
                        placeholder={t('channel.edit.vertex_region_placeholder')}
                        onChange={handleConfigChange}
                        value={config.region}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.vertex_project_id')}
                        name='vertex_ai_project_id'
                        required
                        placeholder={t(
                          'channel.edit.vertex_project_id_placeholder',
                        )}
                        onChange={handleConfigChange}
                        value={config.vertex_ai_project_id}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.vertex_credentials')}
                        name='vertex_ai_adc'
                        required
                        placeholder={t(
                          'channel.edit.vertex_credentials_placeholder',
                        )}
                        onChange={handleConfigChange}
                        value={config.vertex_ai_adc}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'coze' && (
                    <Form.Input
                      className='router-section-input'
                      label={t('channel.edit.user_id')}
                      name='user_id'
                      required
                      placeholder={t('channel.edit.user_id_placeholder')}
                      onChange={handleConfigChange}
                      value={config.user_id}
                      autoComplete=''
                      {...inputReadonlyProps}
                    />
                  )}
                  {inputs.protocol === 'cloudflare' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label='Account ID'
                        name='user_id'
                        required
                        placeholder='请输入 Account ID，例如：d8d7c61dbc334c32d3ced580e4bf42b4'
                        onChange={handleConfigChange}
                        value={config.user_id}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  <Form.Group widths='equal'>
                    <Form.Input
                      className='router-section-input'
                      label={t('channel.edit.created_time')}
                      value={
                        inputs.created_time
                          ? timestamp2string(inputs.created_time)
                          : '-'
                      }
                      readOnly
                    />
                  </Form.Group>
                </section>
              ) : (
                <>
                  <Form.Group widths='equal'>
                    <Form.Input
                      className='router-section-input'
                      label={t('channel.edit.identifier')}
                      name='name'
                      placeholder={t('channel.edit.identifier_placeholder')}
                      onChange={handleInputChange}
                      value={inputs.name}
                      required
                      maxLength={CHANNEL_IDENTIFIER_MAX_LENGTH}
                      readOnly={detailBasicReadonly}
                    />
                    <Form.Field>
                      {detailBasicReadonly ? (
                        <Form.Input
                          className='router-section-input'
                          label={t('channel.edit.type')}
                          value={
                            currentProtocolOption?.text ||
                            inputs.protocol ||
                            '-'
                          }
                          readOnly
                        />
                      ) : (
                        <Form.Select
                          className='router-section-dropdown'
                          label={t('channel.edit.type')}
                          name='protocol'
                          required
                          search
                          options={channelProtocolOptions}
                          value={inputs.protocol}
                          onChange={handleInputChange}
                        />
                      )}
                    </Form.Field>
                  </Form.Group>
                  {baseURLField && keyField ? (
                    <Form.Group widths='equal'>
                      {baseURLField}
                      {keyField}
                    </Form.Group>
                  ) : (
                    <>
                      {baseURLField}
                      {keyField}
                    </>
                  )}
                  {inputs.protocol === 'azure' && (
                    <>
                      <Message className='router-section-message'>
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
                      </Message>
                      <Form.Field>
                        <Form.Input
                          className='router-section-input'
                          label='默认 API 版本'
                          name='other'
                          placeholder='请输入默认 API 版本，例如：2024-03-01-preview，该配置可以被实际的请求查询参数所覆盖'
                          onChange={handleInputChange}
                          value={inputs.other}
                          autoComplete='new-password'
                          {...inputReadonlyProps}
                        />
                      </Form.Field>
                    </>
                  )}
                  {inputs.protocol === 'xunfei' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.spark_version')}
                        name='other'
                        placeholder={t('channel.edit.spark_version_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.other}
                        autoComplete='new-password'
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'aiproxy-library' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.knowledge_id')}
                        name='other'
                        placeholder={t('channel.edit.knowledge_id_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.other}
                        autoComplete='new-password'
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'ali' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.plugin_param')}
                        name='other'
                        placeholder={t('channel.edit.plugin_param_placeholder')}
                        onChange={handleInputChange}
                        value={inputs.other}
                        autoComplete='new-password'
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'coze' && (
                    <Message className='router-section-message'>
                      {t('channel.edit.coze_notice')}
                    </Message>
                  )}
                  {inputs.protocol === 'doubao' && (
                    <Message className='router-section-message'>
                      {t('channel.edit.douban_notice')}
                      <a
                        target='_blank'
                        rel='noreferrer'
                        href='https://console.volcengine.com/ark/region:ark+cn-beijing/endpoint'
                      >
                        {t('channel.edit.douban_notice_link')}
                      </a>
                      {t('channel.edit.douban_notice_2')}
                    </Message>
                  )}
                  {inputs.protocol === 'awsclaude' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label='Region'
                        name='region'
                        required
                        placeholder={t('channel.edit.aws_region_placeholder')}
                        onChange={handleConfigChange}
                        value={config.region}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label='AK'
                        name='ak'
                        required
                        placeholder={t('channel.edit.aws_ak_placeholder')}
                        onChange={handleConfigChange}
                        value={config.ak}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label='SK'
                        name='sk'
                        required
                        placeholder={t('channel.edit.aws_sk_placeholder')}
                        onChange={handleConfigChange}
                        value={config.sk}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'vertexai' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label='Region'
                        name='region'
                        required
                        placeholder={t('channel.edit.vertex_region_placeholder')}
                        onChange={handleConfigChange}
                        value={config.region}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.vertex_project_id')}
                        name='vertex_ai_project_id'
                        required
                        placeholder={t(
                          'channel.edit.vertex_project_id_placeholder',
                        )}
                        onChange={handleConfigChange}
                        value={config.vertex_ai_project_id}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.vertex_credentials')}
                        name='vertex_ai_adc'
                        required
                        placeholder={t(
                          'channel.edit.vertex_credentials_placeholder',
                        )}
                        onChange={handleConfigChange}
                        value={config.vertex_ai_adc}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                  {inputs.protocol === 'coze' && (
                    <Form.Input
                      className='router-section-input'
                      label={t('channel.edit.user_id')}
                      name='user_id'
                      required
                      placeholder={t('channel.edit.user_id_placeholder')}
                      onChange={handleConfigChange}
                      value={config.user_id}
                      autoComplete=''
                      {...inputReadonlyProps}
                    />
                  )}
                  {inputs.protocol === 'cloudflare' && (
                    <Form.Field>
                      <Form.Input
                        className='router-section-input'
                        label='Account ID'
                        name='user_id'
                        required
                        placeholder='请输入 Account ID，例如：d8d7c61dbc334c32d3ced580e4bf42b4'
                        onChange={handleConfigChange}
                        value={config.user_id}
                        autoComplete=''
                        {...inputReadonlyProps}
                      />
                    </Form.Field>
                  )}
                </>
              ))}
            {showStepTwo && inputs.protocol !== 'proxy' && (
              <>
                {isDetailMode && (
                  <section className='router-entity-detail-section'>
                    <div className='router-entity-detail-section-header'>
                      <div className='router-toolbar-start router-block-gap-sm'>
                        <span className='router-entity-detail-section-title'>
                          {t('channel.edit.detail_models_title')}
                        </span>
                        <span className='router-toolbar-meta'>
                          ({modelSectionMetaText})
                        </span>
                      </div>
                      <div className='router-toolbar-end router-block-gap-sm'>
                        <Dropdown
                          selection
                          className='router-section-dropdown router-dropdown-min-170 router-detail-filter-dropdown'
                          compact
                          disabled={detailModelsEditing}
                          options={[
                            {
                              key: 'all',
                              value: 'all',
                              text: t('channel.edit.model_selector.filters.all'),
                            },
                            {
                              key: 'enabled',
                              value: 'enabled',
                              text: t(
                                'channel.edit.model_selector.filters.enabled',
                              ),
                            },
                            {
                              key: 'disabled',
                              value: 'disabled',
                              text: t(
                                'channel.edit.model_selector.filters.disabled',
                              ),
                            },
                          ]}
                          value={detailModelFilter}
                          onChange={(e, { value }) =>
                            setDetailModelFilter((value || 'all').toString())
                          }
                        />
                        <Form.Input
                          className='router-section-input router-search-form-sm'
                          icon='search'
                          iconPosition='left'
                          disabled={detailModelsEditing}
                          placeholder={t(
                            'channel.edit.model_selector.search_placeholder',
                          )}
                          value={modelSearchKeyword}
                          onChange={(e, { value }) =>
                            setModelSearchKeyword(value || '')
                          }
                        />
                        <Button
                          type='button'
                          className='router-page-button'
                          color='green'
                          loading={fetchModelsLoading || !!activeRefreshModelsTask}
                          disabled={
                            detailModelsEditing ||
                            fetchModelsLoading ||
                            !!activeRefreshModelsTask ||
                            detailModelMutating
                          }
                          onClick={() => handleFetchModels({ silent: false })}
                        >
                          {t('channel.edit.buttons.sync_models')}
                        </Button>
                      </div>
                    </div>
                    <Form.Field>
                      <Table
                        celled
                        stackable
                        className='router-detail-table'
                        compact='very'
                        style={{ tableLayout: 'fixed' }}
                      >
                        <Table.Header>
                          <Table.Row>
                            <Table.HeaderCell width={1} textAlign='center'>
                              {t('channel.edit.model_selector.table.selected')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={3}>
                              {t('channel.edit.model_selector.table.name')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={1}>
                              {t('channel.edit.model_selector.table.type')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={3}>
                              {t('channel.edit.model_selector.table.providers')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={3}>
                              {t('channel.edit.model_selector.table.alias')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={2}>
                              {t('channel.edit.model_selector.table.price_unit')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={2}>
                              {t('channel.edit.model_selector.table.input_price')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={2}>
                              {t('channel.edit.model_selector.table.output_price')}
                            </Table.HeaderCell>
                            <Table.HeaderCell width={2}>
                              {t('channel.table.actions')}
                            </Table.HeaderCell>
                          </Table.Row>
                        </Table.Header>
                        <Table.Body>
                          {searchedModelConfigs.length === 0 ? (
                            <Table.Row>
                              <Table.Cell
                                className='router-empty-cell'
                                colSpan={9}
                              >
                                {modelSearchKeyword.trim() !== ''
                                  ? t('channel.edit.model_selector.empty_search')
                                  : visibleModelConfigs.length > 0
                                    ? t('channel.edit.model_selector.empty_filtered')
                                    : t('channel.edit.model_selector.empty')}
                              </Table.Cell>
                            </Table.Row>
                          ) : (
                          renderedModelConfigs.map((row) => {
                              const providerOwners = getProviderOwnersForModel(row);
                              const selectedProviderItems =
                                getSelectedProviderDisplayItems(row);
                              const complexPricingDetails =
                                getComplexPricingDetailsForModel(row);
                              const hasComplexInputPricing =
                                complexPricingDetails.some((detail) =>
                                  (detail.price_components || []).some(
                                    (component) =>
                                      Number(component.input_price || 0) > 0,
                                  ),
                                );
                              const hasComplexOutputPricing =
                                complexPricingDetails.some((detail) =>
                                  (detail.price_components || []).some(
                                    (component) =>
                                      Number(component.output_price || 0) > 0,
                                  ),
                                );
                              const isUnassigned = providerOwners.length === 0;
                              const rowEditDisabled =
                                detailModelsEditLocked ||
                                detailModelMutating ||
                                detailModelsEditing;
                              return (
                                <Table.Row
                                  key={`${row.upstream_model}-${row.model}`}
                                >
                                  <Table.Cell
                                    textAlign='center'
                                    className='router-cell-checkbox'
                                  >
                                    <Checkbox
                                      checked={!!row.selected}
                                      disabled={
                                        detailModelMutating ||
                                        detailModelsEditing ||
                                        providerCatalogLoading ||
                                        (!canSelectChannelModel(row) &&
                                          !row.selected)
                                      }
                                      onChange={(e, { checked }) =>
                                        toggleModelSelection(
                                          row.upstream_model,
                                          checked,
                                        )
                                      }
                                    />
                                  </Table.Cell>
                                  <Table.Cell
                                    title={row.upstream_model}
                                    className='router-cell-truncate'
                                  >
                                    <span className='router-nowrap'>
                                      {row.upstream_model}
                                    </span>
                                    {row.inactive && (
                                      <Label
                                        basic
                                        color='grey'
                                        className='router-tag'
                                      >
                                        {t('channel.edit.model_selector.inactive')}
                                      </Label>
                                    )}
                                  </Table.Cell>
                                  <Table.Cell>
                                    {t(
                                      `channel.model_types.${normalizeChannelModelType(
                                        row.type,
                                      )}`,
                                    )}
                                  </Table.Cell>
                                  <Table.Cell>
                                    {selectedProviderItems.length > 0 ? (
                                      selectedProviderItems.map((provider) => (
                                        <Label
                                          key={`${row.upstream_model}-${provider.key}`}
                                          basic
                                          className='router-tag'
                                          title={provider.text}
                                        >
                                          {provider.text}
                                        </Label>
                                      ))
                                    ) : providerOwners.length > 0 ? (
                                      providerOwners.map((providerId) => (
                                        <Label
                                          key={`${row.upstream_model}-${providerId}`}
                                          basic
                                          className='router-tag'
                                        >
                                          {providerId}
                                        </Label>
                                      ))
                                    ) : providerCatalogLoading ? (
                                      <Label basic className='router-tag'>
                                        {t(
                                          'channel.edit.model_selector.provider_loading',
                                        )}
                                      </Label>
                                    ) : isUnassigned ? (
                                      '-'
                                    ) : (
                                      '-'
                                    )}
                                  </Table.Cell>
                                  <Table.Cell
                                    title={row.model}
                                    className='router-cell-truncate'
                                  >
                                    {row.model}
                                  </Table.Cell>
                                  <Table.Cell>
                                    <span className='router-nowrap'>
                                      {row.price_unit}
                                    </span>
                                  </Table.Cell>
                                  <Table.Cell>
                                    {hasComplexInputPricing ? (
                                      <Button
                                        type='button'
                                        basic
                                        className='router-inline-button'
                                        onClick={() => openComplexPricingModal(row)}
                                      >
                                        {t(
                                          'channel.edit.model_selector.pricing_detail_button',
                                        )}
                                      </Button>
                                    ) : (
                                      <span className='router-nowrap'>
                                        {row.input_price ?? '-'}
                                      </span>
                                    )}
                                  </Table.Cell>
                                  <Table.Cell>
                                    {hasComplexOutputPricing ? (
                                      <Button
                                        type='button'
                                        basic
                                        className='router-inline-button'
                                        onClick={() => openComplexPricingModal(row)}
                                      >
                                        {t(
                                          'channel.edit.model_selector.pricing_detail_button',
                                        )}
                                      </Button>
                                    ) : (
                                      <span className='router-nowrap'>
                                        {row.output_price ?? '-'}
                                      </span>
                                    )}
                                  </Table.Cell>
                                  <Table.Cell collapsing className='router-nowrap'>
                                    <div className='router-inline-actions'>
                                      <Button
                                        type='button'
                                        className='router-inline-button'
                                        disabled={rowEditDisabled}
                                        onClick={() =>
                                          startDetailModelEdit(
                                            row.upstream_model,
                                          )
                                        }
                                      >
                                        {t('common.edit')}
                                      </Button>
                                      {isUnassigned && !providerCatalogLoading ? (
                                        <Button
                                          type='button'
                                          className='router-inline-button'
                                          basic
                                          disabled={rowEditDisabled}
                                          onClick={() => openAppendProviderModal(row)}
                                        >
                                          {t(
                                            'channel.edit.model_selector.provider_add',
                                          )}
                                        </Button>
                                      ) : null}
                                    </div>
                                  </Table.Cell>
                                </Table.Row>
                              );
                            })
                          )}
                        </Table.Body>
                      </Table>
                      {detailModelTotalPages > 1 && (
                        <div className='router-pagination-wrap'>
                          <Pagination
                            className='router-section-pagination'
                            activePage={detailModelPage}
                            totalPages={detailModelTotalPages}
                            onPageChange={(e, { activePage }) =>
                              setDetailModelPage(Number(activePage) || 1)
                            }
                          />
                        </div>
                      )}
                      {modelsSyncError && (
                        <div className='router-error-text router-error-text-top'>
                          {modelsSyncError}
                        </div>
                      )}
                    </Form.Field>
                  </section>
                )}
                {!isDetailMode && (
                  <Form.Field>
                    <div className='router-toolbar router-block-gap-xs'>
                      <div className='router-toolbar-start router-block-gap-sm'>
                        <span className='router-entity-detail-section-title'>
                          {t('channel.edit.models')}
                        </span>
                        <span className='router-toolbar-meta'>
                          ({modelSectionMetaText})
                        </span>
                      </div>
                      <div className='router-toolbar-end router-block-gap-sm'>
                        <Button
                          type='button'
                          className='router-page-button'
                          color='green'
                          loading={
                            fetchModelsLoading || !!activeRefreshModelsTask
                          }
                          disabled={
                            fetchModelsLoading ||
                            !!activeRefreshModelsTask ||
                            (requiresConnectionVerification &&
                              !hasModelPreviewCredentials)
                          }
                          onClick={() => handleFetchModels({ silent: false })}
                        >
                          {fetchModelsButtonText}
                        </Button>
                        <Button
                          type='button'
                          className='router-page-button'
                          onClick={selectAllModels}
                          disabled={visibleModelConfigs.length === 0}
                        >
                          {t('channel.edit.buttons.select_all')}
                        </Button>
                        <Button
                          type='button'
                          className='router-page-button'
                          onClick={clearSelectedModels}
                          disabled={inputs.models.length === 0}
                        >
                          {t('channel.edit.buttons.clear')}
                        </Button>
                        <Form.Input
                          className='router-section-input router-search-form-sm'
                          icon='search'
                          iconPosition='left'
                          placeholder={t(
                            'channel.edit.model_selector.search_placeholder',
                          )}
                          value={modelSearchKeyword}
                          onChange={(e, { value }) =>
                            setModelSearchKeyword(value || '')
                          }
                        />
                      </div>
                    </div>
                    <Table celled stackable className='router-detail-table'>
                      <Table.Header>
                        <Table.Row>
                          <Table.HeaderCell width={1} textAlign='center'>
                            {t('channel.edit.model_selector.table.selected')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={5}>
                            {t('channel.edit.model_selector.table.name')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={2}>
                            {t('channel.edit.model_selector.table.type')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={4}>
                            {t('channel.edit.model_selector.table.providers')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={5}>
                            {t('channel.edit.model_selector.table.alias')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={2}>
                            {t('channel.edit.model_selector.table.price_unit')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={2}>
                            {t('channel.edit.model_selector.table.input_price')}
                          </Table.HeaderCell>
                          <Table.HeaderCell width={2}>
                            {t('channel.edit.model_selector.table.output_price')}
                          </Table.HeaderCell>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {searchedModelConfigs.length === 0 ? (
                          <Table.Row>
                            <Table.Cell
                              className='router-empty-cell'
                              colSpan={8}
                            >
                              {modelSearchKeyword.trim() !== ''
                                ? t('channel.edit.model_selector.empty_search')
                                : t('channel.edit.model_selector.empty')}
                            </Table.Cell>
                          </Table.Row>
                        ) : (
                          renderedModelConfigs.map((row) => {
                            const providerOwners =
                              getProviderOwnersForModel(row);
                            const selectedProviderItems =
                              getSelectedProviderDisplayItems(row);
                            const complexPricingDetails =
                              getComplexPricingDetailsForModel(row);
                            const hasComplexInputPricing =
                              complexPricingDetails.some((detail) =>
                                (detail.price_components || []).some(
                                  (component) =>
                                    Number(component.input_price || 0) > 0,
                                ),
                              );
                            const hasComplexOutputPricing =
                              complexPricingDetails.some((detail) =>
                                (detail.price_components || []).some(
                                  (component) =>
                                    Number(component.output_price || 0) > 0,
                                ),
                              );
                            const canSelectRow = providerOwners.length > 0;
                            return (
                              <Table.Row
                                key={`${row.upstream_model}-${row.model}`}
                              >
                                <Table.Cell textAlign='center'>
                                  <Checkbox
                                    checked={!!row.selected}
                                    disabled={!canSelectRow && !row.selected}
                                    onChange={(e, { checked }) =>
                                      toggleModelSelection(
                                        row.upstream_model,
                                        checked,
                                      )
                                    }
                                  />
                                </Table.Cell>
                                <Table.Cell title={row.upstream_model}>
                                  <span className='router-nowrap'>
                                    {row.upstream_model}
                                  </span>
                                  {row.inactive && (
                                    <Label
                                      basic
                                      color='grey'
                                      className='router-tag'
                                    >
                                      {t('channel.edit.model_selector.inactive')}
                                    </Label>
                                  )}
                                </Table.Cell>
                                <Table.Cell>
                                  {t(
                                    `channel.model_types.${normalizeChannelModelType(
                                      row.type,
                                    )}`,
                                  )}
                                </Table.Cell>
                                <Table.Cell>
                                  {selectedProviderItems.length > 0 ? (
                                    selectedProviderItems.map((provider) => (
                                      <Label
                                        key={`${row.upstream_model}-${provider.key}`}
                                        basic
                                        className='router-tag'
                                        title={provider.text}
                                      >
                                        {provider.text}
                                      </Label>
                                    ))
                                  ) : providerOwners.length > 0 ? (
                                    providerOwners.map((providerId) => (
                                      <Label
                                        key={`${row.upstream_model}-${providerId}`}
                                        basic
                                        className='router-tag'
                                      >
                                        {providerId}
                                      </Label>
                                    ))
                                  ) : providerCatalogLoading ? (
                                    <Label basic className='router-tag'>
                                      {t(
                                        'channel.edit.model_selector.provider_loading',
                                      )}
                                    </Label>
                                  ) : (
                                    <Button
                                      type='button'
                                      className='router-inline-button'
                                      basic
                                      disabled={providerCatalogLoading}
                                      onClick={() => openAppendProviderModal(row)}
                                    >
                                      {t(
                                        'channel.edit.model_selector.provider_add',
                                      )}
                                    </Button>
                                  )}
                                </Table.Cell>
                                <Table.Cell>
                                  <Form.Input
                                    className='router-inline-input router-inline-input-wide'
                                    transparent
                                    value={row.model}
                                    onChange={(e, { value }) =>
                                      updateModelConfigField(
                                        row.upstream_model,
                                        'model',
                                        value || row.upstream_model,
                                      )
                                    }
                                  />
                                </Table.Cell>
                                <Table.Cell>
                                  <Form.Input
                                    className='router-inline-input'
                                    transparent
                                    readOnly
                                    value={row.price_unit}
                                  />
                                </Table.Cell>
                                <Table.Cell>
                                  {hasComplexInputPricing ? (
                                    <Button
                                      type='button'
                                      basic
                                      className='router-inline-button'
                                      onClick={() => openComplexPricingModal(row)}
                                    >
                                      {t(
                                        'channel.edit.model_selector.pricing_detail_button',
                                      )}
                                    </Button>
                                  ) : (
                                    <Form.Input
                                      className='router-inline-input'
                                      type='number'
                                      min='0'
                                      step='0.01'
                                      transparent
                                      placeholder='-'
                                      value={row.input_price ?? ''}
                                      onChange={(e, { value }) =>
                                        updateModelConfigField(
                                          row.upstream_model,
                                          'input_price',
                                          value,
                                        )
                                      }
                                    />
                                  )}
                                </Table.Cell>
                                <Table.Cell>
                                  {hasComplexOutputPricing ? (
                                    <Button
                                      type='button'
                                      basic
                                      className='router-inline-button'
                                      onClick={() => openComplexPricingModal(row)}
                                    >
                                      {t(
                                        'channel.edit.model_selector.pricing_detail_button',
                                      )}
                                    </Button>
                                  ) : (
                                    <Form.Input
                                      className='router-inline-input'
                                      type='number'
                                      min='0'
                                      step='0.01'
                                      transparent
                                      placeholder='-'
                                      value={row.output_price ?? ''}
                                      onChange={(e, { value }) =>
                                        updateModelConfigField(
                                          row.upstream_model,
                                          'output_price',
                                          value,
                                        )
                                      }
                                    />
                                  )}
                                </Table.Cell>
                              </Table.Row>
                            );
                          })
                        )}
                      </Table.Body>
                    </Table>
                    {detailModelTotalPages > 1 && (
                      <div className='router-pagination-wrap'>
                        <Pagination
                          className='router-section-pagination'
                          activePage={detailModelPage}
                          totalPages={detailModelTotalPages}
                          onPageChange={(e, { activePage }) =>
                            setDetailModelPage(Number(activePage) || 1)
                          }
                        />
                      </div>
                    )}
                    {modelsSyncError && (
                      <div className='router-error-text router-error-text-top'>
                        {modelsSyncError}
                      </div>
                    )}
                  </Form.Field>
                )}
              </>
            )}
            {showStepThree && inputs.protocol !== 'proxy' && (
              <>
                {isDetailMode && (
                  <section className='router-entity-detail-section'>
                    <div className='router-entity-detail-section-header'>
                      <div className='router-toolbar-start'>
                        <span className='router-entity-detail-section-title'>
                          {t('channel.edit.model_tester.title')}
                        </span>
                      </div>
                    </div>
                    <Form.Field>
                      <Message info className='router-section-message'>
                        {t('channel.edit.model_tester.hint')}
                      </Message>
                      <Message className='router-section-message'>
                        {t('channel.edit.model_tester.selection_notice')}
                      </Message>
                      <div className='router-toolbar-end router-block-gap-sm'>
                        <>
                          <Button
                            type='button'
                            className='router-section-button'
                            color='blue'
                            loading={
                              modelTesting && modelTestingScope === 'batch'
                            }
                            disabled={
                              detailTestingReadonly ||
                              detailModelMutating ||
                              modelTesting ||
                              modelTestTargetModels.length === 0 ||
                              selectedModelTestHasActiveTasks
                            }
                            onClick={() =>
                              handleRunModelTests({
                                targetModels: modelTestTargetModels,
                                scope: 'batch',
                              })
                            }
                          >
                            {t('channel.edit.model_tester.button')}
                          </Button>
                          <Label basic className='router-tag'>
                            {t('channel.edit.model_tester.selection', {
                              selected: modelTestTargetModels.length,
                              total: modelTestRows.length,
                            })}
                          </Label>
                        </>
                        <Button
                          type='button'
                          className='router-page-button'
                          basic
                          onClick={() => openChannelTaskView()}
                        >
                          {t('task.title')}
                        </Button>
                        <Button
                          type='button'
                          className='router-page-button'
                          basic
                          onClick={() =>
                            openChannelTaskView({
                              type: 'channel_model_test',
                            })
                          }
                        >
                          {t('task.buttons.model_tests')}
                        </Button>
                        <Button
                          type='button'
                          className='router-page-button'
                          basic
                          onClick={() =>
                            openChannelTaskView({
                              status: 'pending,running',
                            })
                          }
                        >
                          {t('task.buttons.running')}
                        </Button>
                        {modelTestedAt > 0 && (
                          <span className='router-toolbar-meta'>
                            {t('channel.edit.model_tester.last_tested', {
                              time: new Date(modelTestedAt).toLocaleString(),
                            })}
                          </span>
                        )}
                      </div>
                      {modelTestError && (
                        <div className='router-error-text router-block-gap-sm'>
                          {modelTestError}
                        </div>
                      )}
                      <Table celled stackable className='router-detail-table'>
                        <Table.Header>
                          <Table.Row>
                            <Table.HeaderCell collapsing textAlign='center'>
                              <Checkbox
                                checked={allModelTestTargetsSelected}
                                disabled={
                                  detailTestingReadonly || detailModelMutating
                                }
                                onChange={(e, { checked }) =>
                                  toggleAllModelTestTargets(!!checked)
                                }
                              />
                            </Table.HeaderCell>
                            <Table.HeaderCell>
                              {t('channel.edit.model_tester.table.model')}
                            </Table.HeaderCell>
                            <Table.HeaderCell>
                              {t('channel.edit.model_tester.table.type')}
                            </Table.HeaderCell>
                            <Table.HeaderCell>
                              {t('channel.edit.model_tester.table.endpoint')}
                            </Table.HeaderCell>
                            <Table.HeaderCell collapsing>
                              {t('channel.edit.model_tester.table.status')}
                            </Table.HeaderCell>
                            <Table.HeaderCell collapsing>
                              {t('channel.edit.model_tester.table.latency')}
                            </Table.HeaderCell>
                            <Table.HeaderCell>
                              {t('channel.edit.model_tester.table.message')}
                            </Table.HeaderCell>
                            <Table.HeaderCell collapsing>
                              {t('channel.edit.model_tester.table.actions')}
                            </Table.HeaderCell>
                          </Table.Row>
                        </Table.Header>
                        <Table.Body>
                          {modelTestRows.length === 0 ? (
                            <Table.Row>
                              <Table.Cell className='router-empty-cell' colSpan='8'>
                                {t('channel.edit.model_tester.empty')}
                              </Table.Cell>
                            </Table.Row>
                          ) : (
                            modelTestRows.map((row) => {
                              const normalizedEndpoint =
                                normalizeChannelModelEndpoint(
                                  row.type,
                                  row.endpoint,
                                );
                              const item = modelTestResultsByKey.get(
                                buildModelTestResultKey(
                                  row.model,
                                  normalizedEndpoint,
                                ),
                              );
                              const activeTask =
                                activeChannelTasksByModel.get(row.model) || null;
                              const hasTestedOtherEndpoint =
                                (testedEndpointsByModel.get(row.model) ||
                                  new Set()).size > 0;
                              const isStale =
                                !activeTask && !item && hasTestedOtherEndpoint;
                              const canDownloadArtifact =
                                !!item?.artifact_path || !!item?.artifact_name;
                              const effectiveStatus =
                                activeTask?.status ||
                                item?.status ||
                                (isStale ? 'stale' : 'untested');
                              const labelColor =
                                effectiveStatus === 'running'
                                  ? 'blue'
                                  : effectiveStatus === 'pending'
                                    ? 'orange'
                                    : effectiveStatus === 'stale'
                                      ? 'yellow'
                                      : effectiveStatus === 'untested'
                                        ? undefined
                                        : effectiveStatus === 'supported'
                                          ? 'green'
                                          : effectiveStatus === 'skipped'
                                            ? 'grey'
                                            : 'red';
                              return (
                                <Table.Row key={row.model}>
                                  <Table.Cell textAlign='center'>
                                    <Checkbox
                                      checked={modelTestTargetModels.includes(
                                        row.model,
                                      )}
                                      disabled={
                                        detailTestingReadonly ||
                                        detailModelMutating
                                      }
                                      onChange={(e, { checked }) =>
                                        toggleModelTestTarget(
                                          row.model,
                                          !!checked,
                                        )
                                      }
                                    />
                                  </Table.Cell>
                                  <Table.Cell>{row.model || '-'}</Table.Cell>
                                  <Table.Cell>{row.type || '-'}</Table.Cell>
                                  <Table.Cell>
                                    {row.type === 'text' ? (
                                      <Dropdown
                                        selection
                                        className='router-mini-dropdown'
                                        options={TEXT_MODEL_ENDPOINT_OPTIONS}
                                        disabled={
                                          detailTestingReadonly ||
                                          detailModelMutating
                                        }
                                        value={
                                          row.endpoint ||
                                          defaultChannelModelEndpoint(row.type)
                                        }
                                        onChange={(e, { value }) =>
                                          updateModelTestEndpoint(row.model, value)
                                        }
                                      />
                                    ) : row.type === 'image' ? (
                                      <Dropdown
                                        selection
                                        className='router-mini-dropdown'
                                        options={IMAGE_MODEL_ENDPOINT_OPTIONS}
                                        disabled={
                                          detailTestingReadonly ||
                                          detailModelMutating
                                        }
                                        value={
                                          row.endpoint ||
                                          defaultChannelModelEndpoint(row.type)
                                        }
                                        onChange={(e, { value }) =>
                                          updateModelTestEndpoint(row.model, value)
                                        }
                                      />
                                    ) : (
                                      row.endpoint || '-'
                                    )}
                                  </Table.Cell>
                                  <Table.Cell>
                                    <Label
                                      basic
                                      color={labelColor}
                                      className='router-tag'
                                    >
                                      {t(
                                        `channel.edit.model_tester.status.${
                                          effectiveStatus
                                        }`,
                                      )}
                                    </Label>
                                  </Table.Cell>
                                  <Table.Cell>
                                    {item?.latency_ms > 0
                                      ? `${item.latency_ms} ms`
                                      : '-'}
                                  </Table.Cell>
                                  <Table.Cell>
                                    {item?.message ||
                                      (isStale
                                        ? t('channel.edit.model_tester.stale')
                                        : effectiveStatus === 'untested'
                                          ? t('channel.edit.model_tester.untested')
                                          : '-')}
                                  </Table.Cell>
                                  <Table.Cell collapsing>
                                    <div className='router-inline-actions'>
                                      <Button
                                        type='button'
                                        className='router-inline-button'
                                        basic
                                        loading={
                                          (modelTesting &&
                                            modelTestingScope === 'single' &&
                                            modelTestingTargetSet.has(row.model)) ||
                                          !!activeTask
                                        }
                                        disabled={
                                          detailTestingReadonly ||
                                          detailModelMutating ||
                                          modelTesting ||
                                          activeChannelTasksByModel.has(row.model)
                                        }
                                        onClick={() =>
                                          handleRunModelTests({
                                            targetModels: [row.model],
                                            scope: 'single',
                                          })
                                        }
                                      >
                                        {t('channel.edit.model_tester.single')}
                                      </Button>
                                      <Button
                                        type='button'
                                        className='router-inline-button'
                                        basic
                                        disabled={!canDownloadArtifact}
                                        onClick={() =>
                                          handleDownloadModelTestArtifact(item)
                                        }
                                      >
                                        {t('common.download')}
                                      </Button>
                                    </div>
                                  </Table.Cell>
                                </Table.Row>
                              );
                            })
                          )}
                        </Table.Body>
                      </Table>
                    </Form.Field>
                  </section>
                )}
                {!isDetailMode && (
                  <Form.Field>
                    <div className='router-toolbar router-block-gap-xs'>
                      <div className='router-toolbar-start'>
                        <span className='router-entity-detail-section-title'>
                          {t('channel.edit.model_tester.title')}
                        </span>
                      </div>
                    </div>
                    <Message info className='router-section-message'>
                      {t('channel.edit.model_tester.hint')}
                    </Message>
                    <Message className='router-section-message'>
                      {t('channel.edit.model_tester.selection_notice')}
                    </Message>
                    <div className='router-toolbar router-block-gap-sm'>
                      <>
                        <Button
                          type='button'
                          className='router-section-button'
                          color='blue'
                          loading={
                            modelTesting && modelTestingScope === 'batch'
                          }
                          disabled={
                            detailModelMutating ||
                            modelTesting ||
                            modelTestTargetModels.length === 0 ||
                            selectedModelTestHasActiveTasks
                          }
                          onClick={() =>
                            handleRunModelTests({
                              targetModels: modelTestTargetModels,
                              scope: 'batch',
                            })
                          }
                        >
                          {t('channel.edit.model_tester.button')}
                        </Button>
                        <Label basic className='router-tag'>
                          {t('channel.edit.model_tester.selection', {
                            selected: modelTestTargetModels.length,
                            total: modelTestRows.length,
                          })}
                        </Label>
                      </>
                      <Button
                        type='button'
                        className='router-page-button'
                        basic
                        onClick={() => openChannelTaskView()}
                      >
                        {t('task.title')}
                      </Button>
                      <Button
                        type='button'
                        className='router-page-button'
                        basic
                        onClick={() =>
                          openChannelTaskView({
                            type: 'channel_model_test',
                          })
                        }
                      >
                        {t('task.buttons.model_tests')}
                      </Button>
                      <Button
                        type='button'
                        className='router-page-button'
                        basic
                        onClick={() =>
                          openChannelTaskView({
                            status: 'pending,running',
                          })
                        }
                      >
                        {t('task.buttons.running')}
                      </Button>
                      {modelTestedAt > 0 && (
                        <span className='router-toolbar-meta'>
                          {t('channel.edit.model_tester.last_tested', {
                            time: new Date(modelTestedAt).toLocaleString(),
                          })}
                        </span>
                      )}
                    </div>
                    {modelTestError && (
                      <div className='router-error-text router-block-gap-sm'>
                        {modelTestError}
                      </div>
                    )}
                    <Table celled stackable className='router-detail-table'>
                      <Table.Header>
                        <Table.Row>
                          <Table.HeaderCell collapsing textAlign='center'>
                            <Checkbox
                              checked={allModelTestTargetsSelected}
                              disabled={detailModelMutating}
                              onChange={(e, { checked }) =>
                                toggleAllModelTestTargets(!!checked)
                              }
                            />
                          </Table.HeaderCell>
                          <Table.HeaderCell>
                            {t('channel.edit.model_tester.table.model')}
                          </Table.HeaderCell>
                          <Table.HeaderCell>
                            {t('channel.edit.model_tester.table.type')}
                          </Table.HeaderCell>
                          <Table.HeaderCell>
                            {t('channel.edit.model_tester.table.endpoint')}
                          </Table.HeaderCell>
                          <Table.HeaderCell collapsing>
                            {t('channel.edit.model_tester.table.status')}
                          </Table.HeaderCell>
                          <Table.HeaderCell collapsing>
                            {t('channel.edit.model_tester.table.latency')}
                          </Table.HeaderCell>
                          <Table.HeaderCell>
                            {t('channel.edit.model_tester.table.message')}
                          </Table.HeaderCell>
                          <Table.HeaderCell collapsing>
                            {t('channel.edit.model_tester.table.actions')}
                          </Table.HeaderCell>
                        </Table.Row>
                      </Table.Header>
                      <Table.Body>
                        {modelTestRows.length === 0 ? (
                          <Table.Row>
                            <Table.Cell className='router-empty-cell' colSpan='8'>
                              {t('channel.edit.model_tester.empty')}
                            </Table.Cell>
                          </Table.Row>
                        ) : (
                          modelTestRows.map((row) => {
                            const normalizedEndpoint =
                              normalizeChannelModelEndpoint(
                                row.type,
                                row.endpoint,
                              );
                            const item = modelTestResultsByKey.get(
                              buildModelTestResultKey(
                                row.model,
                                normalizedEndpoint,
                              ),
                            );
                            const activeTask =
                              activeChannelTasksByModel.get(row.model) || null;
                            const hasTestedOtherEndpoint =
                              (testedEndpointsByModel.get(row.model) ||
                                new Set()).size > 0;
                            const isStale =
                              !activeTask && !item && hasTestedOtherEndpoint;
                            const canDownloadArtifact =
                              !!item?.artifact_path || !!item?.artifact_name;
                            const effectiveStatus =
                              activeTask?.status ||
                              item?.status ||
                              (isStale ? 'stale' : 'untested');
                            const labelColor =
                              effectiveStatus === 'running'
                                ? 'blue'
                                : effectiveStatus === 'pending'
                                  ? 'orange'
                                  : effectiveStatus === 'stale'
                                    ? 'yellow'
                                    : effectiveStatus === 'untested'
                                      ? undefined
                                      : effectiveStatus === 'supported'
                                        ? 'green'
                                        : effectiveStatus === 'skipped'
                                          ? 'grey'
                                          : 'red';
                            return (
                              <Table.Row key={row.model}>
                                <Table.Cell textAlign='center'>
                                  <Checkbox
                                    checked={modelTestTargetModels.includes(
                                      row.model,
                                    )}
                                    disabled={detailModelMutating}
                                    onChange={(e, { checked }) =>
                                      toggleModelTestTarget(row.model, !!checked)
                                    }
                                  />
                                </Table.Cell>
                                <Table.Cell>{row.model || '-'}</Table.Cell>
                                <Table.Cell>{row.type || '-'}</Table.Cell>
                                <Table.Cell>
                                  {row.type === 'text' ? (
                                    <Dropdown
                                      selection
                                      className='router-mini-dropdown'
                                      options={TEXT_MODEL_ENDPOINT_OPTIONS}
                                      disabled={detailModelMutating}
                                      value={
                                        row.endpoint ||
                                        defaultChannelModelEndpoint(row.type)
                                      }
                                      onChange={(e, { value }) =>
                                        updateModelTestEndpoint(row.model, value)
                                      }
                                    />
                                  ) : row.type === 'image' ? (
                                    <Dropdown
                                      selection
                                      className='router-mini-dropdown'
                                      options={IMAGE_MODEL_ENDPOINT_OPTIONS}
                                      disabled={detailModelMutating}
                                      value={
                                        row.endpoint ||
                                        defaultChannelModelEndpoint(row.type)
                                      }
                                      onChange={(e, { value }) =>
                                        updateModelTestEndpoint(row.model, value)
                                      }
                                    />
                                  ) : (
                                    row.endpoint || '-'
                                  )}
                                </Table.Cell>
                                <Table.Cell>
                                  <Label
                                    basic
                                    color={labelColor}
                                    className='router-tag'
                                  >
                                    {t(
                                      `channel.edit.model_tester.status.${
                                        effectiveStatus
                                      }`,
                                    )}
                                  </Label>
                                </Table.Cell>
                                <Table.Cell>
                                  {item?.latency_ms > 0
                                    ? `${item.latency_ms} ms`
                                    : '-'}
                                </Table.Cell>
                                <Table.Cell>
                                  {item?.message ||
                                    (isStale
                                      ? t('channel.edit.model_tester.stale')
                                      : effectiveStatus === 'untested'
                                        ? t('channel.edit.model_tester.untested')
                                        : '-')}
                                </Table.Cell>
                                <Table.Cell collapsing>
                                  <div className='router-inline-actions'>
                                    <Button
                                      type='button'
                                      className='router-inline-button'
                                      basic
                                      loading={
                                        (modelTesting &&
                                          modelTestingScope === 'single' &&
                                          modelTestingTargetSet.has(row.model)) ||
                                        !!activeTask
                                      }
                                      disabled={
                                        detailModelMutating ||
                                        modelTesting ||
                                        activeChannelTasksByModel.has(row.model)
                                      }
                                      onClick={() =>
                                        handleRunModelTests({
                                          targetModels: [row.model],
                                          scope: 'single',
                                        })
                                      }
                                    >
                                      {t('channel.edit.model_tester.single')}
                                    </Button>
                                    <Button
                                      type='button'
                                      className='router-inline-button'
                                      basic
                                      disabled={!canDownloadArtifact}
                                      onClick={() =>
                                        handleDownloadModelTestArtifact(item)
                                      }
                                    >
                                      {t('common.download')}
                                    </Button>
                                  </div>
                                </Table.Cell>
                              </Table.Row>
                            );
                          })
                        )}
                      </Table.Body>
                    </Table>
                  </Form.Field>
                )}
              </>
            )}
            {showStepFour && inputs.protocol !== 'proxy' && (
              <>
                {isDetailMode && (
                  <section className='router-entity-detail-section'>
                    <div className='router-entity-detail-section-header'>
                      <div className='router-toolbar-start'>
                        <span className='router-entity-detail-section-title'>
                          {t('channel.edit.wizard.step_advanced')}
                        </span>
                      </div>
                      <div className='router-toolbar-end'>
                        {detailAdvancedEditing ? (
                          <>
                            <Button
                              type='button'
                              className='router-page-button'
                              onClick={cancelDetailAdvancedEdit}
                              disabled={detailAdvancedSaving}
                            >
                              {t('channel.edit.buttons.cancel')}
                            </Button>
                            <Button
                              type='button'
                              className='router-page-button'
                              color='blue'
                              loading={detailAdvancedSaving}
                              disabled={detailAdvancedSaving}
                              onClick={saveDetailAdvancedConfig}
                            >
                              {t('channel.edit.buttons.save')}
                            </Button>
                          </>
                        ) : (
                          <Button
                            type='button'
                            className='router-page-button'
                            color='blue'
                            disabled={detailAdvancedEditLocked}
                            onClick={() => setDetailAdvancedEditing(true)}
                          >
                            {t('common.edit')}
                          </Button>
                        )}
                      </div>
                    </div>
                    <Form.Field>
                      <Form.TextArea
                        className='router-section-textarea router-code-textarea router-code-textarea-md'
                        label={t('channel.edit.system_prompt')}
                        placeholder={t('channel.edit.system_prompt_placeholder')}
                        name='system_prompt'
                        onChange={handleInputChange}
                        value={inputs.system_prompt}
                        autoComplete='new-password'
                        readOnly={detailAdvancedReadonly}
                      />
                    </Form.Field>
                  </section>
                )}
                {!isDetailMode && (
                  <Form.Field>
                    <Form.TextArea
                      className='router-section-textarea router-code-textarea router-code-textarea-md'
                      label={t('channel.edit.system_prompt')}
                      placeholder={t('channel.edit.system_prompt_placeholder')}
                      name='system_prompt'
                      onChange={handleInputChange}
                      value={inputs.system_prompt}
                      autoComplete='new-password'
                    />
                  </Form.Field>
                )}
              </>
            )}
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditChannel;
