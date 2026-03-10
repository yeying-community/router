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
import { API, showError, showInfo, showSuccess } from '../../helpers';
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

const buildProviderCatalogIndex = (items) => {
  const providerOptions = [];
  const modelOwners = {};
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
    });
  });
  providerOptions.sort((a, b) => a.value.localeCompare(b.value));
  Object.keys(modelOwners).forEach((modelName) => {
    modelOwners[modelName].sort((a, b) => a.localeCompare(b));
  });
  return { providerOptions, modelOwners };
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
  let page = 0;
  while (page < 50) {
    const res = await API.get(`/api/v1/admin/channel/${normalizedChannelId}/models`, {
      params: {
        p: page,
        page_size: 100,
      },
    });
    const { success, message, data } = res.data || {};
    if (!success) {
      throw new Error(message || 'fetch channel models failed');
    }
    const pageItems = normalizeChannelModelConfigs(
      extractChannelModelListItems(data)
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
  const res = await API.get(`/api/v1/admin/channel/${normalizedChannelId}/tests`);
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch channel tests failed');
  }
  return {
    items: normalizeModelTestResults(data?.items),
    lastTestedAt: Number(data?.last_tested_at || 0),
  };
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
    modelConfigs
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
        item && typeof item === 'object' && typeof item.model === 'string'
    )
    .map((item) => ({
      model: item.model || '',
      upstream_model: item.upstream_model || '',
      type: normalizeChannelModelType(item.type),
      endpoint: item.endpoint || '',
      status: item.status || 'unsupported',
      supported: !!item.supported,
      message: item.message || '',
      latency_ms: Number(item.latency_ms || 0),
      tested_at: Number(item.tested_at || 0),
    }));
};

const mergeModelTestResults = (previousResults, nextResults) => {
  const merged = new Map();
  normalizeModelTestResults(previousResults).forEach((item) => {
    if (!item.model) {
      return;
    }
    merged.set(item.model, item);
  });
  normalizeModelTestResults(nextResults).forEach((item) => {
    if (!item.model) {
      return;
    }
    merged.set(item.model, item);
  });
  return Array.from(merged.values()).sort((a, b) =>
    (a.model || '').localeCompare(b.model || '')
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
  const isEditMode = hasChannelID && !isDetailMode;
  const isCreateMode = !hasChannelID;
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
    hasChannelID || copyFromId !== '' || creatingChannelIdFromQuery !== ''
  );
  const [createStep, setCreateStep] = useState(() => {
    const query = new URLSearchParams(location.search);
    return parseCreateStep(query.get('step'));
  });
  const [creatingChannelId, setCreatingChannelId] = useState(
    creatingChannelIdFromQuery
  );
  const [channelKeySet, setChannelKeySet] = useState(false);
  const handleCancel = () => {
    navigate('/admin/channel');
  };
  const openEditPage = useCallback(() => {
    if (!channelId) {
      return;
    }
    navigate(`/admin/channel/edit/${channelId}`);
  }, [channelId, navigate]);

  const [inputs, setInputs] = useState(CHANNEL_ORIGIN_INPUTS);
  const [channelProtocolOptions, setChannelProtocolOptions] = useState(() =>
    getChannelProtocolOptions()
  );
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [modelsSyncError, setModelsSyncError] = useState('');
  const [modelsLastSyncedAt, setModelsLastSyncedAt] = useState(0);
  const [verifiedModelSignature, setVerifiedModelSignature] = useState('');
  const [modelTestResults, setModelTestResults] = useState([]);
  const [modelTesting, setModelTesting] = useState(false);
  const [modelTestingScope, setModelTestingScope] = useState('');
  const [modelTestingTargets, setModelTestingTargets] = useState([]);
  const [modelTestError, setModelTestError] = useState('');
  const [modelTestedAt, setModelTestedAt] = useState(0);
  const [modelTestedSignature, setModelTestedSignature] = useState('');
  const [modelTestTargetModels, setModelTestTargetModels] = useState([]);
  const [config, setConfig] = useState(CHANNEL_DEFAULT_CONFIG);
  const [providerOptions, setProviderOptions] = useState([]);
  const [providerModelOwners, setProviderModelOwners] = useState({});
  const [providerCatalogLoading, setProviderCatalogLoading] = useState(false);
  const [providerCatalogLoaded, setProviderCatalogLoaded] = useState(false);
  const [appendProviderModalOpen, setAppendProviderModalOpen] = useState(false);
  const [appendingProviderModel, setAppendingProviderModel] = useState(false);
  const [autoAssigningProviders, setAutoAssigningProviders] = useState(false);
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
          normalizedProtocol
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
    [buildEffectiveKey]
  );
  const previewChannelID = useMemo(
    () => ((hasChannelID ? channelId : creatingChannelId) || '').trim(),
    [channelId, creatingChannelId, hasChannelID]
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
    [effectivePreviewKey, inputs.base_url, inputs.protocol, previewChannelID]
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
    ]
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
  const inputReadonlyProps = isDetailMode ? { readOnly: true } : {};
  const textAreaReadonlyProps = isDetailMode ? { readOnly: true } : {};
  const visibleModelConfigs = useMemo(
    () => normalizeChannelModelConfigs(inputs.model_configs),
    [inputs.model_configs]
  );
  const modelTestResultsByModel = useMemo(() => {
    const index = new Map();
    normalizeModelTestResults(modelTestResults).forEach((item) => {
      if (!item.model) {
        return;
      }
      index.set(item.model, item);
    });
    return index;
  }, [modelTestResults]);
  const modelTestRows = useMemo(() => {
    return visibleModelConfigs.filter((row) => {
      if (row.inactive) {
        return false;
      }
      if (row.selected) {
        return true;
      }
      return modelTestResultsByModel.has(row.model);
    });
  }, [modelTestResultsByModel, visibleModelConfigs]);
  const allModelTestTargetsSelected = useMemo(() => {
    if (modelTestRows.length === 0) {
      return false;
    }
    return modelTestRows.every((row) =>
      modelTestTargetModels.includes(row.model)
    );
  }, [modelTestRows, modelTestTargetModels]);
  const isModelTestSignatureFresh =
    modelTestedSignature !== '' &&
    modelTestedSignature === currentModelTestSignature;
  const modelTestingTargetSet = useMemo(
    () => new Set(modelTestingTargets),
    [modelTestingTargets]
  );
  const getProviderOwnersForModel = useCallback(
    (row) => {
      const owners = new Set();
      buildProviderLookupKeys(row).forEach((key) => {
        (providerModelOwners[key] || []).forEach((providerId) => {
          owners.add(providerId);
        });
      });
      return Array.from(owners).sort((a, b) => a.localeCompare(b));
    },
    [providerModelOwners]
  );
  const inferAssignableProviderForRow = useCallback(
    (row) => inferAssignableProviderForRowWithOptions(row, providerOptions),
    [providerOptions]
  );
  const canSelectChannelModel = useCallback(
    (row) =>
      row?.inactive !== true && getProviderOwnersForModel(row).length > 0,
    [getProviderOwnersForModel]
  );
  const activeModelConfigs = useMemo(
    () => visibleModelConfigs.filter((row) => row.inactive !== true),
    [visibleModelConfigs]
  );
  const detailModelStats = useMemo(() => {
    return visibleModelConfigs.reduce(
      (acc, row) => {
        const owners = getProviderOwnersForModel(row);
        if (owners.length > 0) {
          acc.assigned += 1;
          return acc;
        }
        acc.unassigned += 1;
        const inferredProvider = inferAssignableProviderForRow(row);
        if (inferredProvider !== '') {
          acc.autoAssignable += 1;
        } else {
          acc.manualRequired += 1;
        }
        return acc;
      },
      {
        assigned: 0,
        unassigned: 0,
        autoAssignable: 0,
        manualRequired: 0,
      }
    );
  }, [
    getProviderOwnersForModel,
    inferAssignableProviderForRow,
    visibleModelConfigs,
  ]);
  const detailFilteredModelConfigs = useMemo(() => {
    if (!isDetailMode) {
      return visibleModelConfigs;
    }
    return visibleModelConfigs.filter((row) => {
      const hasOwners = getProviderOwnersForModel(row).length > 0;
      if (detailModelFilter === 'unassigned') {
        return !hasOwners;
      }
      if (detailModelFilter === 'manual') {
        return !hasOwners && inferAssignableProviderForRow(row) === '';
      }
      return true;
    });
  }, [
    detailModelFilter,
    getProviderOwnersForModel,
    inferAssignableProviderForRow,
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
      const candidates = [
        row?.upstream_model,
        row?.model,
        row?.type,
        providerOwners,
      ].map(normalizeSearchKeyword);
      return candidates.some((candidate) => candidate.includes(keyword));
    });
  }, [
    deferredModelSearchKeyword,
    detailFilteredModelConfigs,
    getProviderOwnersForModel,
  ]);
  const detailModelTotalPages = useMemo(() => {
    return Math.max(
      1,
      Math.ceil(searchedModelConfigs.length / CHANNEL_MODEL_PAGE_SIZE)
    );
  }, [searchedModelConfigs.length]);
  const renderedModelConfigs = useMemo(() => {
    const offset = (detailModelPage - 1) * CHANNEL_MODEL_PAGE_SIZE;
    return searchedModelConfigs.slice(offset, offset + CHANNEL_MODEL_PAGE_SIZE);
  }, [searchedModelConfigs, detailModelPage]);
  const autoAssignableRows = useMemo(() => {
    return visibleModelConfigs.filter((row) => {
      const owners = getProviderOwnersForModel(row);
      return owners.length === 0 && inferAssignableProviderForRow(row) !== '';
    });
  }, [
    getProviderOwnersForModel,
    inferAssignableProviderForRow,
    visibleModelConfigs,
  ]);
  const modelSelectionSummaryText = useMemo(
    () =>
      t('channel.edit.model_selector.summary', {
        selected: inputs.models.length,
        total: activeModelConfigs.length,
      }),
    [activeModelConfigs.length, inputs.models.length, t]
  );
  const modelAssignmentSummaryText = useMemo(() => {
    if (!isDetailMode) {
      return '';
    }
    return t('channel.edit.model_selector.assignment_summary', {
      assigned: detailModelStats.assigned,
      unassigned: detailModelStats.unassigned,
      auto: detailModelStats.autoAssignable,
      manual: detailModelStats.manualRequired,
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
        cachedState.modelTestResults
      )
        ? cachedState.modelTestResults
        : Array.isArray(cachedState.capabilityResults)
        ? cachedState.capabilityResults
        : [];
      const restoredModelTestTargetModels = Array.isArray(
        cachedState.modelTestTargetModels
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
          normalizeModelTestResults(restoredModelTestResults)
        );
      }
      if (restoredModelTestTargetModels.length > 0) {
        setModelTestTargetModels(
          normalizeModelIDs(restoredModelTestTargetModels)
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
    [isCreateMode]
  );

  const moveToPreviousCreateStep = useCallback(() => {
    goToCreateStep(createStep - 1);
  }, [createStep, goToCreateStep]);

  const buildChannelPayload = useCallback(() => {
    const effectiveKey = buildEffectiveKey();
    const derivedModelState = buildChannelModelState(inputs.model_configs);
    let localInputs = { ...inputs, key: effectiveKey };
    localInputs.id = (localInputs.id || '').toString().trim();
    localInputs.name = normalizeChannelIdentifier(localInputs.name);
    if (localInputs.key === 'undefined|undefined|undefined') {
      localInputs.key = '';
    }
    if (localInputs.base_url && localInputs.base_url.endsWith('/')) {
      localInputs.base_url = localInputs.base_url.slice(
        0,
        localInputs.base_url.length - 1
      );
    }
    if (localInputs.protocol === 'azure' && localInputs.other === '') {
      localInputs.other = '2024-03-01-preview';
    }
    localInputs.model_configs = derivedModelState.modelConfigs;
    localInputs.models = derivedModelState.selectedModels.join(',');
    const submitConfig = { ...config };
    localInputs.config = JSON.stringify(submitConfig);
    return localInputs;
  }, [buildEffectiveKey, config, inputs]);

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
    ]
  );

  const saveCreatingChannel = useCallback(async () => {
    const targetChannelID = await persistWorkingChannel({ status: 4 });
    return targetChannelID !== '';
  }, [persistWorkingChannel]);

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
        const remoteConfigs = await fetchAllChannelModelConfigs(targetChannelID);
        const remoteModels = normalizeModelIDs(
          remoteConfigs
            .filter((row) => row && row.selected === true)
            .map((row) => row.model)
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
    [creatingChannelId]
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
    if (
      inputs.protocol !== 'proxy' &&
      inputs.models.length > 0 &&
      !isModelTestSignatureFresh
    ) {
      showInfo(t('channel.edit.model_tester.verify_required'));
      return;
    }
    goToCreateStep(4);
  }, [
    createStep,
    ensureModelsStepCompleted,
    goToCreateStep,
    inputs.models.length,
    inputs.protocol,
    isModelTestSignatureFresh,
    t,
  ]);

  const loadChannelModelConfigsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchAllChannelModelConfigs(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.messages.fetch_models_failed')
        );
      }
    },
    [t]
  );

  const loadChannelTestsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelTests(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.model_tester.test_failed')
        );
      }
    },
    [t]
  );

  const loadChannelById = useCallback(
    async (targetId, forCopy = false, fromCreating = false) => {
      try {
        let res = await API.get(`/api/v1/admin/channel/${targetId}`);
        const { success, message, data } = res.data;
        if (success) {
          const [remoteModelConfigs, channelTestsData] = await Promise.all([
            loadChannelModelConfigsFromServer(data.id || targetId),
            forCopy
              ? Promise.resolve({ items: [], lastTestedAt: 0 })
              : loadChannelTestsFromServer(data.id || targetId),
          ]);
          const storedModelTestResults = normalizeModelTestResults(
            channelTestsData.items
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
            });
            setModelTestResults([]);
            setModelTestError('');
            setModelTestedAt(0);
            setModelTestedSignature('');
            setModelTestTargetModels([]);
          } else {
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
            });
            setModelTestResults(storedModelTestResults);
            setModelTestError('');
            setModelTestedAt(storedModelTestedAt);
            setModelTestedSignature(
              storedModelTestResults.length > 0 && storedModelTestedAt > 0
                ? loadedModelTestSignature
                : ''
            );
            setModelTestTargetModels([]);
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
              })
            );
          }
        } else {
          showError(message);
        }
      } finally {
        setLoading(false);
      }
    },
    [hasChannelID, loadChannelModelConfigsFromServer, loadChannelTestsFromServer]
  );

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
          `/api/v1/admin/channel/${targetChannelId}/refresh`
        );
        const { success, message } = res.data || {};
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
        const nextConfigs = await loadChannelModelConfigsFromServer(
          targetChannelId
        );
        if (nextConfigs.length === 0) {
          const message = t('channel.edit.messages.models_empty');
          setModelsSyncError(message);
          setVerifiedModelSignature('');
          if (!silent) {
            showInfo(message);
          }
          return false;
        }
        const nextInputs = buildNextInputsWithModelConfigs(inputs, nextConfigs);

        setInputs(nextInputs);
        setModelsSyncError('');
        setModelsLastSyncedAt(Date.now());
        setVerifiedModelSignature(requestSignature);
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
      persistWorkingChannel,
      previewChannelID,
      t,
    ]
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
                message || t('channel.edit.model_selector.provider_load_failed')
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
        setProviderCatalogLoaded(true);
        return nextCatalog;
      } catch (error) {
        if (!silent) {
          showError(
            error?.message ||
              t('channel.edit.model_selector.provider_load_failed')
          );
        }
        return null;
      } finally {
        setProviderCatalogLoading(false);
      }
    },
    [
      providerCatalogLoaded,
      providerCatalogLoading,
      providerModelOwners,
      providerOptions,
      t,
    ]
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
          catalog.providerOptions
        ),
        model: (row?.upstream_model || row?.model || '').toString().trim(),
        type: normalizeChannelModelType(row?.type),
      });
      setAppendProviderModalOpen(true);
    },
    [loadProviderCatalogIndex, t]
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
      const res = await API.post(`/api/v1/admin/providers/${providerId}/model`, {
        model: modelName,
        type: normalizeChannelModelType(appendProviderForm.type),
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(
          message || t('channel.edit.model_selector.provider_append_failed')
        );
        return;
      }
      await loadProviderCatalogIndex({ silent: true, force: true });
      showSuccess(t('channel.edit.model_selector.provider_append_success'));
      closeAppendProviderModal();
    } catch (error) {
      showError(
        error?.message ||
          t('channel.edit.model_selector.provider_append_failed')
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

  const handleAutoAssignModels = useCallback(async () => {
    const catalog = await loadProviderCatalogIndex({
      silent: false,
      force: true,
    });
    if (!catalog) {
      return;
    }
    const assignableRows = visibleModelConfigs.filter((row) => {
      const owners = getProviderOwnersForModel(row);
      return (
        owners.length === 0 &&
        inferAssignableProviderForRowWithOptions(
          row,
          catalog.providerOptions
        ) !== ''
      );
    });
    if (assignableRows.length === 0) {
      showInfo(t('channel.edit.model_selector.auto_assign_empty'));
      return;
    }
    setAutoAssigningProviders(true);
    let successCount = 0;
    let failedCount = 0;
    try {
      for (const row of assignableRows) {
        const providerId = inferAssignableProviderForRowWithOptions(
          row,
          catalog.providerOptions
        );
        const modelName = (row?.upstream_model || row?.model || '')
          .toString()
          .trim();
        if (providerId === '' || modelName === '') {
          failedCount += 1;
          continue;
        }
        try {
          const res = await API.post(
            `/api/v1/admin/providers/${providerId}/model`,
            {
              model: modelName,
              type: normalizeChannelModelType(row?.type),
            }
          );
          if (res?.data?.success) {
            successCount += 1;
          } else {
            failedCount += 1;
          }
        } catch {
          failedCount += 1;
        }
      }
      await loadProviderCatalogIndex({ silent: true, force: true });
      if (successCount > 0) {
        showSuccess(
          t('channel.edit.model_selector.auto_assign_success', {
            success: successCount,
            failed: failedCount,
          })
        );
      } else {
        showInfo(t('channel.edit.model_selector.auto_assign_empty'));
      }
    } finally {
      setAutoAssigningProviders(false);
    }
  }, [
    getProviderOwnersForModel,
    loadProviderCatalogIndex,
    t,
    visibleModelConfigs,
  ]);

  const handleRunModelTests = useCallback(
    async ({ targetModels = [], scope = 'batch' } = {}) => {
      if (inputs.protocol === 'proxy') {
        return;
      }
      const normalizedTargets = normalizeModelIDs(
        Array.isArray(targetModels) && targetModels.length > 0
          ? targetModels
          : modelTestTargetModels
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
      setModelTesting(true);
      setModelTestingScope(scope === 'single' ? 'single' : 'batch');
      setModelTestingTargets(normalizedTargets);
      try {
        const res = await API.post(
          `/api/v1/admin/channel/${targetChannelId}/tests`,
          {
            test_model: inputs.test_model || '',
            target_models: normalizedTargets,
          }
        );
        const { success, message, data } = res.data || {};
        if (!success) {
          const errorMessage =
            message || t('channel.edit.model_tester.test_failed');
          setModelTestResults([]);
          setModelTestError(errorMessage);
          setModelTestedAt(0);
          setModelTestedSignature('');
          showError(errorMessage);
          return;
        }
        const nextResults = normalizeModelTestResults(data?.results);
        let nextModelConfigs = visibleModelConfigs;
        try {
          const refreshedModelConfigs = await loadChannelModelConfigsFromServer(
            targetChannelId
          );
          if (refreshedModelConfigs.length > 0) {
            nextModelConfigs = refreshedModelConfigs;
          }
        } catch {
          nextModelConfigs = visibleModelConfigs;
        }
        const nextInputs = buildNextInputsWithModelConfigs(
          inputs,
          nextModelConfigs
        );
        const nextSignature = buildChannelModelTestSignature({
          protocol: inputs.protocol,
          key: effectivePreviewKey,
          baseURL: inputs.base_url,
          channelID: targetChannelId,
          models: nextInputs.models,
          modelConfigs: nextInputs.model_configs,
        });
        setInputs(nextInputs);
        setModelTestResults((previousResults) =>
          mergeModelTestResults(previousResults, nextResults)
        );
        setModelTestError('');
        setModelTestedAt(Date.now());
        setModelTestedSignature(nextSignature);
        showSuccess(t('channel.edit.model_tester.test_success'));
      } catch (error) {
        const errorMessage =
          error?.message || t('channel.edit.model_tester.test_failed');
        setModelTestResults([]);
        setModelTestError(errorMessage);
        setModelTestedAt(0);
        setModelTestedSignature('');
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
      isCreateMode,
      persistWorkingChannel,
      previewChannelID,
      t,
      loadChannelModelConfigsFromServer,
      visibleModelConfigs,
    ]
  );

  const toggleModelTestTarget = useCallback((modelName, checked) => {
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
  }, []);

  const toggleAllModelTestTargets = useCallback(
    (checked) => {
      if (!checked) {
        setModelTestTargetModels([]);
        return;
      }
      setModelTestTargetModels(modelTestRows.map((row) => row.model));
    },
    [modelTestRows]
  );

  const updateModelTestEndpoint = useCallback(
    (modelName, endpoint) => {
      setInputs((prev) =>
        buildNextInputsWithModelConfigs(
          prev,
          visibleModelConfigs.map((row) => {
            if (row.model !== modelName) {
              return row;
            }
            return {
              ...row,
              endpoint: normalizeChannelModelEndpoint(row.type, endpoint),
            };
          })
        )
      );
    },
    [visibleModelConfigs]
  );

  const toggleModelSelection = useCallback(
    (upstreamModel, checked) => {
      setInputs((prev) =>
        buildNextInputsWithModelConfigs(
          prev,
          visibleModelConfigs.map((row) =>
            row.upstream_model === upstreamModel && canSelectChannelModel(row)
              ? { ...row, selected: !!checked }
              : row
          )
        )
      );
    },
    [canSelectChannelModel, visibleModelConfigs]
  );

  const updateModelConfigField = useCallback(
    (upstreamModel, field, value) => {
      setInputs((prev) =>
        buildNextInputsWithModelConfigs(
          prev,
          visibleModelConfigs.map((row) => {
            if (row.upstream_model !== upstreamModel) {
              return row;
            }
            if (field === 'model') {
              const alias = (value || '').toString().trim();
              const targetAlias = alias || row.upstream_model;
              const duplicated = visibleModelConfigs.some(
                (item) =>
                  item.upstream_model !== upstreamModel &&
                  item.model === targetAlias
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
            return {
              ...row,
              [field]: value,
            };
          })
        )
      );
    },
    [visibleModelConfigs]
  );

  const selectAllModels = useCallback(() => {
    setInputs((prev) =>
      buildNextInputsWithModelConfigs(
        prev,
        visibleModelConfigs.map((row) => ({
          ...row,
          selected: canSelectChannelModel(row),
        }))
      )
    );
  }, [canSelectChannelModel, visibleModelConfigs]);

  const clearSelectedModels = useCallback(() => {
    setInputs((prev) =>
      buildNextInputsWithModelConfigs(
        prev,
        visibleModelConfigs.map((row) => ({ ...row, selected: false }))
      )
    );
  }, [visibleModelConfigs]);

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
      { replace: true }
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
      { replace: true }
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
    if (modelTestedSignature === '') {
      return;
    }
    if (modelTestedSignature === currentModelTestSignature) {
      return;
    }
    setModelTestResults([]);
    setModelTestError(t('channel.edit.model_tester.stale'));
    setModelTestedAt(0);
    setModelTestedSignature('');
  }, [modelTestedSignature, currentModelTestSignature, t]);

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
    if (
      inputs.protocol !== 'proxy' &&
      inputs.models.length > 0 &&
      !isModelTestSignatureFresh
    ) {
      showInfo(t('channel.edit.model_tester.verify_required'));
      return;
    }
    if (modelConfigError) {
      showInfo(modelConfigError);
      return;
    }
    let localInputs = buildChannelPayload();
    let res;
    if (isEditMode) {
      res = await API.put(`/api/v1/admin/channel/`, {
        ...localInputs,
        id: channelId,
      });
    } else if (creatingChannelId) {
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
      if (isEditMode) {
        showSuccess(t('channel.edit.messages.update_success'));
      } else {
        showSuccess(t('channel.edit.messages.create_success'));
        clearCreateChannelCache();
      }
      navigate('/admin/channel', { replace: true });
      return;
    } else {
      showError(message);
    }
  };

  return (
    <div className='dashboard-container'>
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
                  'channel.edit.model_selector.append_dialog.provider_placeholder'
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
            <div className='router-toolbar-start router-block-gap-sm'>
              <Button
                type='button'
                className='router-page-button'
                onClick={handleCancel}
              >
                <Icon name='undo' />
                {t('channel.edit.buttons.back')}
              </Button>
              <Button
                type='button'
                className='router-page-button'
                color='blue'
                onClick={openEditPage}
              >
                <Icon name='edit' />
                {t('channel.buttons.edit')}
              </Button>
            </div>
          )}
          {isEditMode && (
            <div className='router-toolbar-start router-block-gap-sm'>
              <Button
                type='button'
                className='router-page-button'
                onClick={handleCancel}
              >
                {t('channel.edit.buttons.cancel')}
              </Button>
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
          {isEditMode && (
            <Card.Header className='header router-page-title'>
              {t('channel.edit.title_edit')}
            </Card.Header>
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
            {showStepOne && (
              <>
                <Form.Input
                  className='router-section-input'
                  label={t('channel.edit.identifier')}
                  name='name'
                  placeholder={t('channel.edit.identifier_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.name}
                  required
                  maxLength={CHANNEL_IDENTIFIER_MAX_LENGTH}
                  readOnly={isDetailMode}
                />
                <Form.Group widths='equal'>
                  <Form.Field>
                    {isDetailMode ? (
                      <Form.Input
                        className='router-section-input'
                        label={t('channel.edit.type')}
                        value={
                          currentProtocolOption?.text || inputs.protocol || '-'
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
                {/* Azure OpenAI specific fields */}
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
              </>
            )}
            {showStepTwo && inputs.protocol !== 'proxy' && (
              <Form.Field>
                <div className='router-toolbar router-block-gap-xs'>
                  <div className='router-toolbar-start'>
                    <span className='router-section-title router-section-title-inline'>
                      {t('channel.edit.models')}
                    </span>
                    <span className='router-toolbar-meta'>
                      ({modelSectionMetaText})
                    </span>
                  </div>
                  {isDetailMode ? (
                    <div className='router-toolbar-end'>
                      <Dropdown
                        selection
                        className='router-section-dropdown router-dropdown-min-170'
                        compact
                        options={[
                          {
                            key: 'all',
                            value: 'all',
                            text: t('channel.edit.model_selector.filters.all'),
                          },
                          {
                            key: 'unassigned',
                            value: 'unassigned',
                            text: t(
                              'channel.edit.model_selector.filters.unassigned'
                            ),
                          },
                          {
                            key: 'manual',
                            value: 'manual',
                            text: t(
                              'channel.edit.model_selector.filters.manual'
                            ),
                          },
                        ]}
                        value={detailModelFilter}
                        onChange={(e, { value }) =>
                          setDetailModelFilter((value || 'all').toString())
                        }
                      />
                      <Button
                        type='button'
                        className='router-section-button'
                        color='blue'
                        loading={autoAssigningProviders}
                        disabled={
                          autoAssigningProviders ||
                          providerCatalogLoading ||
                          autoAssignableRows.length === 0
                        }
                        onClick={handleAutoAssignModels}
                      >
                        {t('channel.edit.model_selector.auto_assign')}
                      </Button>
                      <Form.Input
                        className='router-inline-input router-search-form-sm'
                        icon='search'
                        iconPosition='left'
                        placeholder={t(
                          'channel.edit.model_selector.search_placeholder'
                        )}
                        value={modelSearchKeyword}
                        onChange={(e, { value }) =>
                          setModelSearchKeyword(value || '')
                        }
                      />
                    </div>
                  ) : (
                    <div className='router-toolbar-end'>
                      <Button
                        type='button'
                        className='router-page-button'
                        color='green'
                        loading={fetchModelsLoading}
                        disabled={
                          fetchModelsLoading ||
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
                      <Button
                        type='button'
                        className='router-page-button'
                        color='blue'
                        loading={autoAssigningProviders}
                        disabled={
                          autoAssigningProviders ||
                          providerCatalogLoading ||
                          autoAssignableRows.length === 0
                        }
                        onClick={handleAutoAssignModels}
                      >
                        {t('channel.edit.model_selector.auto_assign')}
                      </Button>
                      <Form.Input
                        className='router-inline-input router-search-form-sm'
                        icon='search'
                        iconPosition='left'
                        placeholder={t(
                          'channel.edit.model_selector.search_placeholder'
                        )}
                        value={modelSearchKeyword}
                        onChange={(e, { value }) =>
                          setModelSearchKeyword(value || '')
                        }
                      />
                    </div>
                  )}
                </div>
                <Table
                  celled
                  stackable
                  className='router-detail-table'
                  compact={isDetailMode ? 'very' : undefined}
                  style={isDetailMode ? { tableLayout: 'fixed' } : undefined}
                >
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell width={1} textAlign='center'>
                        {t('channel.edit.model_selector.table.selected')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={isDetailMode ? 3 : 5}>
                        {t('channel.edit.model_selector.table.name')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={isDetailMode ? 1 : 2}>
                        {t('channel.edit.model_selector.table.type')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={isDetailMode ? 2 : 4}>
                        {t('channel.edit.model_selector.table.providers')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={isDetailMode ? 3 : 5}>
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
                      {isDetailMode && (
                        <Table.HeaderCell width={1}>
                          {t('channel.edit.model_selector.table.actions')}
                        </Table.HeaderCell>
                      )}
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {searchedModelConfigs.length === 0 ? (
                      <Table.Row>
                        <Table.Cell
                          className='router-empty-cell'
                          colSpan={isDetailMode ? 9 : 8}
                        >
                          {modelSearchKeyword.trim() !== ''
                            ? t('channel.edit.model_selector.empty_search')
                            : isDetailMode && visibleModelConfigs.length > 0
                            ? t('channel.edit.model_selector.empty_filtered')
                            : t('channel.edit.model_selector.empty')}
                        </Table.Cell>
                      </Table.Row>
                    ) : (
                      renderedModelConfigs.map((row) => {
                        const providerOwners = getProviderOwnersForModel(row);
                        const isUnassigned = providerOwners.length === 0;
                        const canSelectRow = providerOwners.length > 0;
                        return (
                          <Table.Row key={`${row.upstream_model}-${row.model}`}>
                            <Table.Cell
                              textAlign='center'
                              className={
                                isDetailMode
                                  ? 'router-cell-checkbox'
                                  : undefined
                              }
                            >
                              <Checkbox
                                checked={!!row.selected}
                                disabled={
                                  isDetailMode ||
                                  providerCatalogLoading ||
                                  (!canSelectRow && !row.selected)
                                }
                                onChange={(e, { checked }) =>
                                  toggleModelSelection(
                                    row.upstream_model,
                                    checked
                                  )
                                }
                              />
                            </Table.Cell>
                            <Table.Cell
                              title={row.upstream_model}
                              className={
                                isDetailMode
                                  ? 'router-cell-truncate'
                                  : undefined
                              }
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
                                  row.type
                                )}`
                              )}
                            </Table.Cell>
                            <Table.Cell>
                              {providerOwners.length > 0 ? (
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
                                    'channel.edit.model_selector.provider_loading'
                                  )}
                                </Label>
                              ) : !isDetailMode ? (
                                <Button
                                  type='button'
                                  className='router-inline-button'
                                  basic
                                  onClick={() => openAppendProviderModal(row)}
                                >
                                  {t(
                                    'channel.edit.model_selector.provider_add'
                                  )}
                                </Button>
                              ) : (
                                <Label
                                  basic
                                  color='orange'
                                  className='router-tag'
                                >
                                  {t(
                                    'channel.edit.model_selector.provider_unassigned'
                                  )}
                                </Label>
                              )}
                            </Table.Cell>
                            <Table.Cell
                              title={isDetailMode ? row.model : undefined}
                              className={
                                isDetailMode
                                  ? 'router-cell-truncate'
                                  : undefined
                              }
                            >
                              {isDetailMode ? (
                                row.model
                              ) : (
                                <Form.Input
                                  className='router-inline-input router-inline-input-wide'
                                  transparent
                                  value={row.model}
                                  onChange={(e, { value }) =>
                                    updateModelConfigField(
                                      row.upstream_model,
                                      'model',
                                      value || row.upstream_model
                                    )
                                  }
                                />
                              )}
                            </Table.Cell>
                            <Table.Cell>
                              {isDetailMode ? (
                                <span className='router-nowrap'>
                                  {row.price_unit}
                                </span>
                              ) : (
                                <Form.Input
                                  className='router-inline-input'
                                  transparent
                                  readOnly
                                  value={row.price_unit}
                                />
                              )}
                            </Table.Cell>
                            <Table.Cell>
                              {isDetailMode ? (
                                <span className='router-nowrap'>
                                  {row.input_price ?? '-'}
                                </span>
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
                                      value
                                    )
                                  }
                                />
                              )}
                            </Table.Cell>
                            <Table.Cell>
                              {isDetailMode ? (
                                <span className='router-nowrap'>
                                  {row.output_price ?? '-'}
                                </span>
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
                                      value
                                    )
                                  }
                                />
                              )}
                            </Table.Cell>
                            {isDetailMode && (
                              <Table.Cell
                                collapsing
                                className='router-nowrap router-cell-shrink'
                              >
                                {isUnassigned ? (
                                  <Button
                                    type='button'
                                    className='router-inline-button'
                                    basic
                                    onClick={() => openAppendProviderModal(row)}
                                  >
                                    {t(
                                      'channel.edit.model_selector.provider_add'
                                    )}
                                  </Button>
                                ) : (
                                  '-'
                                )}
                              </Table.Cell>
                            )}
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
            {showStepThree && inputs.protocol !== 'proxy' && (
              <Form.Field>
                <label>{t('channel.edit.model_tester.title')}</label>
                <Message info className='router-section-message'>
                  {t('channel.edit.model_tester.hint')}
                </Message>
                <div
                  className={`${
                    isDetailMode ? 'router-toolbar-end' : 'router-toolbar'
                  } router-block-gap-sm`}
                >
                  {!isDetailMode && (
                    <>
                      <Button
                        type='button'
                        className='router-section-button'
                        color='blue'
                        loading={modelTesting && modelTestingScope === 'batch'}
                        disabled={
                          modelTesting || modelTestTargetModels.length === 0
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
                  )}
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
                      {!isDetailMode && (
                        <Table.HeaderCell collapsing textAlign='center'>
                          <Checkbox
                            checked={allModelTestTargetsSelected}
                            onChange={(e, { checked }) =>
                              toggleAllModelTestTargets(!!checked)
                            }
                          />
                        </Table.HeaderCell>
                      )}
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
                      {!isDetailMode && (
                        <Table.HeaderCell collapsing>
                          {t('channel.edit.model_tester.table.actions')}
                        </Table.HeaderCell>
                      )}
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {modelTestRows.length === 0 ? (
                      <Table.Row>
                        <Table.Cell
                          className='router-empty-cell'
                          colSpan={isDetailMode ? '6' : '8'}
                        >
                          {t('channel.edit.model_tester.empty')}
                        </Table.Cell>
                      </Table.Row>
                    ) : (
                      modelTestRows.map((row) => {
                        const item = modelTestResultsByModel.get(row.model);
                        const labelColor =
                          item?.status === 'supported'
                            ? 'green'
                            : item?.status === 'skipped'
                            ? 'grey'
                            : 'red';
                        return (
                          <Table.Row key={row.model}>
                            {!isDetailMode && (
                              <Table.Cell textAlign='center'>
                                <Checkbox
                                  checked={modelTestTargetModels.includes(
                                    row.model
                                  )}
                                  onChange={(e, { checked }) =>
                                    toggleModelTestTarget(row.model, !!checked)
                                  }
                                />
                              </Table.Cell>
                            )}
                            <Table.Cell>{row.model || '-'}</Table.Cell>
                            <Table.Cell>{row.type || '-'}</Table.Cell>
                            <Table.Cell>
                              {isDetailMode || row.type !== 'text' ? (
                                row.endpoint || '-'
                              ) : (
                                <Dropdown
                                  selection
                                  className='router-mini-dropdown'
                                  options={TEXT_MODEL_ENDPOINT_OPTIONS}
                                  value={
                                    row.endpoint ||
                                    defaultChannelModelEndpoint(row.type)
                                  }
                                  onChange={(e, { value }) =>
                                    updateModelTestEndpoint(row.model, value)
                                  }
                                />
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
                                    item?.status || 'unsupported'
                                  }`
                                )}
                              </Label>
                            </Table.Cell>
                            <Table.Cell>
                              {item?.latency_ms > 0
                                ? `${item.latency_ms} ms`
                                : '-'}
                            </Table.Cell>
                            <Table.Cell>{item?.message || '-'}</Table.Cell>
                            {!isDetailMode && (
                              <Table.Cell collapsing>
                                <Button
                                  type='button'
                                  className='router-inline-button'
                                  basic
                                  loading={
                                    modelTesting &&
                                    modelTestingScope === 'single' &&
                                    modelTestingTargetSet.has(row.model)
                                  }
                                  disabled={modelTesting}
                                  onClick={() =>
                                    handleRunModelTests({
                                      targetModels: [row.model],
                                      scope: 'single',
                                    })
                                  }
                                >
                                  {t('channel.edit.model_tester.single')}
                                </Button>
                              </Table.Cell>
                            )}
                          </Table.Row>
                        );
                      })
                    )}
                  </Table.Body>
                </Table>
              </Form.Field>
            )}
            {showStepFour && inputs.protocol !== 'proxy' && (
              <Form.Field>
                <Form.TextArea
                  className='router-section-textarea router-code-textarea router-code-textarea-md'
                  label={t('channel.edit.system_prompt')}
                  placeholder={t('channel.edit.system_prompt_placeholder')}
                  name='system_prompt'
                  onChange={handleInputChange}
                  value={inputs.system_prompt}
                  autoComplete='new-password'
                  {...textAreaReadonlyProps}
                />
              </Form.Field>
            )}
            {showStepOne && inputs.protocol === 'awsclaude' && (
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
            {showStepOne && inputs.protocol === 'vertexai' && (
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
                  placeholder={t('channel.edit.vertex_project_id_placeholder')}
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
                  placeholder={t('channel.edit.vertex_credentials_placeholder')}
                  onChange={handleConfigChange}
                  value={config.vertex_ai_adc}
                  autoComplete=''
                  {...inputReadonlyProps}
                />
              </Form.Field>
            )}
            {showStepOne && inputs.protocol === 'coze' && (
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
            {showStepOne && inputs.protocol === 'cloudflare' && (
              <Form.Field>
                <Form.Input
                  className='router-section-input'
                  label='Account ID'
                  name='user_id'
                  required
                  placeholder={
                    '请输入 Account ID，例如：d8d7c61dbc334c32d3ced580e4bf42b4'
                  }
                  onChange={handleConfigChange}
                  value={config.user_id}
                  autoComplete=''
                  {...inputReadonlyProps}
                />
              </Form.Field>
            )}
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditChannel;
