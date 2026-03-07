import React, {
  useCallback,
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
  Form,
  Icon,
  Label,
  Message,
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
    selected: row.selected !== false,
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
    .filter((row) => row.selected)
    .map((row) => row.model);
  return {
    modelConfigs: normalizedConfigs,
    selectedModels,
  };
};

const buildFetchedModelConfigs = (
  previousConfigs,
  models,
  selectAll = true,
) => {
  const fetchedConfigs = normalizeChannelModelConfigs(models);
  if (fetchedConfigs.length > 0) {
    return fetchedConfigs.map((row) => ({
      ...row,
      selected: selectAll ? true : row.selected !== false,
    }));
  }

  const previousRows = normalizeChannelModelConfigs(previousConfigs);
  const previousByUpstream = new Map();
  previousRows.forEach((row) => {
    const upstreamModel = row.upstream_model || row.model;
    if (!upstreamModel || previousByUpstream.has(upstreamModel)) {
      return;
    }
    previousByUpstream.set(upstreamModel, row);
  });
  return buildModelIDs(models).map((modelId) => {
    const existing = previousByUpstream.get(modelId);
    if (existing) {
      return {
        ...existing,
        upstream_model: modelId,
        selected: selectAll ? true : existing.selected,
      };
    }
    return {
      model: modelId,
      upstream_model: modelId,
      selected: selectAll,
      input_price: null,
      output_price: null,
      price_unit: 'per_1k_tokens',
      currency: 'USD',
    };
  });
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
    if (row?.input_price !== null && normalizePriceOverrideValue(row?.input_price) === null) {
      return t('channel.edit.messages.model_config_invalid');
    }
    if (row?.output_price !== null && normalizePriceOverrideValue(row?.output_price) === null) {
      return t('channel.edit.messages.model_config_invalid');
    }
  }
  return '';
};

const buildChannelConnectionSignature = ({
  protocol,
  key,
  baseURL,
  draftID,
}) => {
  const normalizedKey = (key || '').trim();
  const normalizedDraftID = (draftID || '').trim();
  const keyPart =
    normalizedKey !== '' ? normalizedKey : `@draft:${normalizedDraftID}`;
  return `${protocol}|${normalizeBaseURL(baseURL)}|${keyPart}`;
};

const buildChannelCapabilitySignature = ({
  protocol,
  key,
  baseURL,
  draftID,
  models,
}) =>
  `${buildChannelConnectionSignature({
    protocol,
    key,
    baseURL,
    draftID,
  })}|${normalizeModelIDs(models).join(',')}`;

const normalizeCapabilityResults = (results) => {
  if (!Array.isArray(results)) {
    return [];
  }
  return results
    .filter(
      (item) =>
        item && typeof item === 'object' && typeof item.capability === 'string',
    )
    .map((item) => ({
      capability: item.capability,
      label: item.label || item.capability,
      endpoint: item.endpoint || '',
      model: item.model || '',
      status: item.status || 'unsupported',
      supported: !!item.supported,
      message: item.message || '',
      latency_ms: Number(item.latency_ms || 0),
    }));
};

const sanitizeDraftInputsForLocalStorage = (inputs) => {
  if (!inputs || typeof inputs !== 'object') {
    return CHANNEL_ORIGIN_INPUTS;
  }
  return {
    ...inputs,
    key: '',
  };
};

const sanitizeDraftConfigForLocalStorage = (config) => {
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

const CHANNEL_CREATE_DRAFT_KEY = 'router.channel.create.draft.v1';
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
    if (hasChannelID) return 0;
    const query = new URLSearchParams(location.search);
    const id = Number(query.get('copy_from') || 0);
    return Number.isInteger(id) && id > 0 ? id : 0;
  }, [hasChannelID, location.search]);
  const draftIdFromQuery = useMemo(() => {
    if (hasChannelID) return '';
    const query = new URLSearchParams(location.search);
    return (query.get('draft_id') || '').trim();
  }, [hasChannelID, location.search]);
  const [loading, setLoading] = useState(hasChannelID || copyFromId > 0);
  const [createStep, setCreateStep] = useState(() => {
    const query = new URLSearchParams(location.search);
    return parseCreateStep(query.get('step'));
  });
  const [draftChannelId, setDraftChannelId] = useState(draftIdFromQuery);
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
    getChannelProtocolOptions(),
  );
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [modelsSyncError, setModelsSyncError] = useState('');
  const [modelsLastSyncedAt, setModelsLastSyncedAt] = useState(0);
  const [verifiedModelSignature, setVerifiedModelSignature] = useState('');
  const [capabilityResults, setCapabilityResults] = useState([]);
  const [capabilityTesting, setCapabilityTesting] = useState(false);
  const [capabilityTestError, setCapabilityTestError] = useState('');
  const [capabilityTestedAt, setCapabilityTestedAt] = useState(0);
  const [capabilityTestedSignature, setCapabilityTestedSignature] =
    useState('');
  const [config, setConfig] = useState(CHANNEL_DEFAULT_CONFIG);
  const fetchingModelsRef = useRef(false);
  const draftChannelIdRef = useRef(draftIdFromQuery);
  const currentProtocolOption = useMemo(() => {
    const normalizedProtocol = (inputs.protocol || '').toString().trim().toLowerCase();
    if (normalizedProtocol === '') {
      return null;
    }
    return (
      channelProtocolOptions.find(
        (option) =>
          (option?.value || '').toString().trim().toLowerCase() === normalizedProtocol,
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
    () => ((hasChannelID ? channelId : draftChannelId) || '').trim(),
    [channelId, draftChannelId, hasChannelID],
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
        draftID: previewChannelID,
      }),
    [effectivePreviewKey, inputs.base_url, inputs.protocol, previewChannelID],
  );
  const currentCapabilitySignature = useMemo(
    () =>
      buildChannelCapabilitySignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: inputs.base_url,
        draftID: previewChannelID,
        models: inputs.models,
      }),
    [
      effectivePreviewKey,
      inputs.base_url,
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
  const inputReadonlyProps = isDetailMode ? { readOnly: true } : {};
  const textAreaReadonlyProps = isDetailMode ? { readOnly: true } : {};
  const visibleModelConfigs = useMemo(
    () => normalizeChannelModelConfigs(inputs.model_configs),
    [inputs.model_configs],
  );

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const handleConfigChange = (e, { name, value }) => {
    setConfig((inputs) => ({ ...inputs, [name]: value }));
  };

  const baseURLField = useMemo(() => {
    if (inputs.protocol === 'azure') {
      return (
        <Form.Field>
          <Form.Input
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

  const clearCreateDraft = useCallback(() => {
    if (typeof window === 'undefined') {
      return;
    }
    localStorage.removeItem(CHANNEL_CREATE_DRAFT_KEY);
  }, []);

  const restoreCreateDraft = useCallback(() => {
    if (typeof window === 'undefined') {
      return false;
    }
    const raw = localStorage.getItem(CHANNEL_CREATE_DRAFT_KEY);
    if (!raw) {
      return false;
    }
    try {
      const draft = JSON.parse(raw);
      if (!draft || typeof draft !== 'object') {
        return false;
      }
      if (!draft.inputs || typeof draft.inputs !== 'object') {
        return false;
      }

      setInputs({
        ...CHANNEL_ORIGIN_INPUTS,
        ...sanitizeDraftInputsForLocalStorage(draft.inputs),
      });
      if (draft.config && typeof draft.config === 'object') {
        setConfig({
          ...CHANNEL_DEFAULT_CONFIG,
          ...sanitizeDraftConfigForLocalStorage(draft.config),
        });
      }
      if (typeof draft.modelsSyncError === 'string') {
        setModelsSyncError(draft.modelsSyncError);
      }
      if (Number.isFinite(draft.modelsLastSyncedAt)) {
        setModelsLastSyncedAt(draft.modelsLastSyncedAt);
      }
      if (typeof draft.verifiedModelSignature === 'string') {
        setVerifiedModelSignature(draft.verifiedModelSignature);
      }
      if (Array.isArray(draft.capabilityResults)) {
        setCapabilityResults(
          normalizeCapabilityResults(draft.capabilityResults),
        );
      }
      if (typeof draft.capabilityTestError === 'string') {
        setCapabilityTestError(draft.capabilityTestError);
      }
      if (Number.isFinite(draft.capabilityTestedAt)) {
        setCapabilityTestedAt(draft.capabilityTestedAt);
      }
      if (typeof draft.capabilityTestedSignature === 'string') {
        setCapabilityTestedSignature(draft.capabilityTestedSignature);
      }
      if (typeof draft.draft_channel_id === 'string') {
        const restoredDraftID = draft.draft_channel_id.trim();
        setDraftChannelId(restoredDraftID);
        draftChannelIdRef.current = restoredDraftID;
      }
      if (typeof draft.channel_key_set === 'boolean') {
        setChannelKeySet(draft.channel_key_set);
      } else {
        setChannelKeySet(false);
      }
      setCreateStep(parseCreateStep(draft.step));
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

  const buildChannelPayload = useCallback(() => {
    const effectiveKey = buildEffectiveKey();
    const derivedModelState = buildChannelModelState(inputs.model_configs);
    let localInputs = { ...inputs, key: effectiveKey };
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
    const submitConfig = { ...config };
    localInputs.config = JSON.stringify(submitConfig);
    return localInputs;
  }, [buildEffectiveKey, config, inputs]);

  const createDraftChannel = useCallback(async () => {
    const payload = buildChannelPayload();
    const res = await API.post('/api/v1/admin/channel/draft', {
      name: payload.name,
      protocol: payload.protocol,
      key: payload.key,
      base_url: payload.base_url,
      config: payload.config,
    });
    const { success, message, data } = res.data || {};
    if (!success) {
      showError(message || t('channel.edit.messages.create_draft_failed'));
      return '';
    }
    const id = (data?.id || '').toString();
    if (id === '') {
      showError(t('channel.edit.messages.create_draft_failed'));
      return '';
    }
    setDraftChannelId(id);
    draftChannelIdRef.current = id;
    if ((payload.key || '').trim() !== '') {
      setChannelKeySet(true);
    }
    return id;
  }, [buildChannelPayload, t]);

  const saveDraftChannel = useCallback(async () => {
    let targetDraftID = (
      draftChannelIdRef.current ||
      draftChannelId ||
      ''
    ).trim();
    if (targetDraftID === '') {
      if (!isCreateMode) {
        return true;
      }
      const createdID = await createDraftChannel();
      if (createdID === '') {
        return false;
      }
      targetDraftID = createdID;
    }
    const payload = buildChannelPayload();
    const res = await API.put('/api/v1/admin/channel/', {
      ...payload,
      id: targetDraftID,
      status: 4,
    });
    const { success, message } = res.data || {};
    if (!success) {
      showError(message || t('channel.edit.messages.update_draft_failed'));
      return false;
    }
    if ((payload.key || '').trim() !== '') {
      setChannelKeySet(true);
    }
    return true;
  }, [
    buildChannelPayload,
    createDraftChannel,
    draftChannelId,
    isCreateMode,
    t,
  ]);

  const verifyDraftModelsPersisted = useCallback(
    async (expectedModels) => {
      const targetDraftID = (
        draftChannelIdRef.current ||
        draftChannelId ||
        ''
      ).trim();
      if (targetDraftID === '') {
        return false;
      }
      try {
        const checkRes = await API.get(
          `/api/v1/admin/channel/${targetDraftID}?select_all=1`,
        );
        const { success, data } = checkRes.data || {};
        if (!success || !data) {
          return false;
        }
        const remoteModels = normalizeModelIDs(
          Array.isArray(data.model_configs) && data.model_configs.length > 0
            ? data.model_configs
                .filter((row) => row && row.selected !== false)
                .map((row) => row.model)
            : (data.models || '')
                .split(',')
                .map((item) => item.trim())
                .filter((item) => item !== ''),
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
    [draftChannelId],
  );

  const ensureDraftChannel = useCallback(async () => {
    if (!isCreateMode) {
      return true;
    }
    if (draftChannelId) {
      return saveDraftChannel();
    }
    const createdID = await createDraftChannel();
    return createdID !== '';
  }, [createDraftChannel, draftChannelId, isCreateMode, saveDraftChannel]);

  const moveToStepTwo = useCallback(async () => {
    const effectiveKey = buildEffectiveKey();
    if (
      inputs.name.trim() === '' ||
      (effectiveKey.trim() === '' && !canReuseStoredKeyForCreate)
    ) {
      showInfo(t('channel.edit.messages.name_required'));
      return;
    }
    if (isCreateMode) {
      const ok = await ensureDraftChannel();
      if (!ok) {
        return;
      }
    }
    goToCreateStep(2);
  }, [
    buildEffectiveKey,
    canReuseStoredKeyForCreate,
    ensureDraftChannel,
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
      const ok = await saveDraftChannel();
      if (!ok) {
        return false;
      }
      const expectedModels = [...inputs.models];
      const persisted = await verifyDraftModelsPersisted(expectedModels);
      if (!persisted) {
        showError(t('channel.edit.messages.update_draft_failed'));
        return false;
      }
    }
    return true;
  }, [
    draftChannelId,
    hasModelPreviewCredentials,
    inputs.models.length,
    inputs.models,
    inputs.protocol,
    isCreateMode,
    isCurrentSignatureVerified,
    requireVerificationBeforeProceed,
    requiresConnectionVerification,
    saveDraftChannel,
    t,
    verifyDraftModelsPersisted,
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

  const loadChannelById = useCallback(
    async (targetId, forCopy = false, selectAll = true, fromDraft = false) => {
      const query = selectAll ? '?select_all=1' : '';
      let res = await API.get(`/api/v1/admin/channel/${targetId}${query}`);
      const { success, message, data } = res.data;
      if (success) {
        const keySet = !!data.key_set;
        const selectedModels =
          data.models === ''
            ? []
            : (data.models || '')
                .split(',')
                .map((item) => item.trim())
                .filter((item) => item !== '');
        const availableModels = Array.isArray(data.available_models)
          ? data.available_models
          : [];
        const storedCapabilityResults = normalizeCapabilityResults(
          data.capability_results,
        );
        const storedCapabilityTestedAt =
          Number(data.capability_last_tested_at || 0) > 0
            ? Number(data.capability_last_tested_at) * 1000
            : 0;
        let parsedConfig = {};
        if (data.config !== '') {
          parsedConfig = JSON.parse(data.config);
        }
        const normalizedProtocol = resolveProtocolFromChannelPayload(data);
        const modelState = buildChannelModelState(
          buildModelConfigsFromLegacyFields({
            modelConfigs: data.model_configs,
            availableModels:
              availableModels.length > 0 ? availableModels : selectedModels,
            selectedModels,
            modelMapping: data.model_mapping || '',
            inputPrice: data.input_price || '',
            outputPrice: data.output_price || '',
            priceUnit: data.price_unit || '',
            currency: data.currency || '',
          }),
        );
        const loadedCapabilitySignature = buildChannelCapabilitySignature({
          protocol: normalizedProtocol,
          key: '',
          baseURL: data.base_url || '',
          draftID: data.id || targetId,
          models: modelState.selectedModels,
        });

        if (forCopy) {
          setInputs({
            name: data.name || '',
            protocol: normalizedProtocol,
            key: '',
            base_url: data.base_url || '',
            other: data.other || '',
            model_configs: modelState.modelConfigs,
            system_prompt: data.system_prompt || '',
            models: modelState.selectedModels,
            test_model: data.test_model || modelState.selectedModels[0] || '',
          });
          setCapabilityResults([]);
          setCapabilityTestError('');
          setCapabilityTestedAt(0);
          setCapabilityTestedSignature('');
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
          setCapabilityResults(storedCapabilityResults);
          setCapabilityTestError('');
          setCapabilityTestedAt(storedCapabilityTestedAt);
          setCapabilityTestedSignature(
            storedCapabilityResults.length > 0 && storedCapabilityTestedAt > 0
              ? loadedCapabilitySignature
              : '',
          );
        }
        setConfig((prev) => ({
          ...prev,
          ...parsedConfig,
        }));
        if (fromDraft || hasChannelID) {
          setChannelKeySet(keySet);
        } else {
          setChannelKeySet(false);
        }
      } else {
        showError(message);
      }
      setLoading(false);
    },
    [hasChannelID],
  );

  const applyModelCandidates = useCallback(
    (models, selectAll = false) => {
      const nextConfigs = buildFetchedModelConfigs(
        inputs.model_configs,
        models,
        selectAll,
      );
      const nextInputs = buildNextInputsWithModelConfigs(inputs, nextConfigs);
      setInputs(nextInputs);
      return nextInputs.models;
    },
    [inputs],
  );

  const handleFetchModels = useCallback(
    async ({ silent = false, selectAll = true } = {}) => {
      if (fetchingModelsRef.current) {
        return false;
      }
      fetchingModelsRef.current = true;
      setFetchModelsLoading(true);
      try {
        let models = [];
        const normalizedBaseURL = normalizeBaseURL(inputs.base_url);
        const key = buildEffectiveKey().trim();
        const requestSignature = buildChannelConnectionSignature({
          protocol: inputs.protocol,
          key,
          baseURL: normalizedBaseURL,
          draftID: previewChannelID,
        });
        const res = await API.post(`/api/v1/admin/channel/preview/models`, {
          protocol: inputs.protocol,
          key,
          base_url: normalizedBaseURL,
          draft_id: previewChannelID,
          config,
        });
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
        models = Array.isArray(data) ? data.filter((model) => model) : [];

        const ids = applyModelCandidates(models, selectAll);
        if (ids.length === 0) {
          const message = t('channel.edit.messages.models_empty');
          setModelsSyncError(message);
          setVerifiedModelSignature('');
          if (!silent) {
            showInfo(message);
          }
          return false;
        }

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
      applyModelCandidates,
      buildEffectiveKey,
      config,
      inputs.base_url,
      inputs.protocol,
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

  const handleTestCapabilities = useCallback(async () => {
    if (inputs.protocol === 'proxy') {
      return;
    }
    if (inputs.models.length === 0) {
      showInfo(t('channel.edit.messages.models_required'));
      return;
    }
    const ok = isCreateMode ? await saveDraftChannel() : true;
    if (!ok) {
      return;
    }
    setCapabilityTesting(true);
    try {
      const res = await API.post('/api/v1/admin/channel/preview/capabilities', {
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        base_url: normalizeBaseURL(inputs.base_url),
        draft_id: previewChannelID,
        config,
        models: inputs.models,
        model_configs: visibleModelConfigs,
        test_model: inputs.test_model || '',
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        const errorMessage =
          message || t('channel.edit.capability_tester.test_failed');
        setCapabilityResults([]);
        setCapabilityTestError(errorMessage);
        setCapabilityTestedAt(0);
        setCapabilityTestedSignature('');
        showError(errorMessage);
        return;
      }
      setCapabilityResults(normalizeCapabilityResults(data?.results));
      setCapabilityTestError('');
      setCapabilityTestedAt(Date.now());
      setCapabilityTestedSignature(currentCapabilitySignature);
      showSuccess(t('channel.edit.capability_tester.test_success'));
    } catch (error) {
      const errorMessage =
        error?.message || t('channel.edit.capability_tester.test_failed');
      setCapabilityResults([]);
      setCapabilityTestError(errorMessage);
      setCapabilityTestedAt(0);
      setCapabilityTestedSignature('');
      showError(errorMessage);
    } finally {
      setCapabilityTesting(false);
    }
  }, [
    config,
    currentCapabilitySignature,
    effectivePreviewKey,
    inputs.base_url,
    inputs.models,
    inputs.protocol,
    inputs.test_model,
    isCreateMode,
    previewChannelID,
    saveDraftChannel,
    t,
  ]);

  const toggleModelSelection = useCallback(
    (upstreamModel, checked) => {
      setInputs((prev) =>
        buildNextInputsWithModelConfigs(
          prev,
          visibleModelConfigs.map((row) =>
            row.upstream_model === upstreamModel
              ? { ...row, selected: !!checked }
              : row,
          ),
        ),
      );
    },
    [visibleModelConfigs],
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
            return {
              ...row,
              [field]: value,
            };
          }),
        ),
      );
    },
    [visibleModelConfigs],
  );

  const selectAllModels = useCallback(() => {
    setInputs((prev) =>
      buildNextInputsWithModelConfigs(
        prev,
        visibleModelConfigs.map((row) => ({ ...row, selected: true })),
      ),
    );
  }, [visibleModelConfigs]);

  const clearSelectedModels = useCallback(() => {
    setInputs((prev) =>
      buildNextInputsWithModelConfigs(
        prev,
        visibleModelConfigs.map((row) => ({ ...row, selected: false })),
      ),
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
      return;
    }
    if (draftIdFromQuery === draftChannelId) {
      return;
    }
    setDraftChannelId(draftIdFromQuery);
    draftChannelIdRef.current = draftIdFromQuery;
  }, [draftIdFromQuery, draftChannelId, hasChannelID]);

  useEffect(() => {
    if (hasChannelID) {
      setLoading(true);
      loadChannelById(channelId, false, true, false).then();
      return;
    }
    if (copyFromId > 0) {
      setLoading(true);
      loadChannelById(copyFromId, true, true, false).then();
      return;
    }
    if (draftIdFromQuery !== '') {
      setLoading(true);
      loadChannelById(draftIdFromQuery, true, true, true).then();
      return;
    }
    setChannelKeySet(false);
    restoreCreateDraft();
    setLoading(false);
  }, [
    channelId,
    copyFromId,
    draftIdFromQuery,
    hasChannelID,
    loadChannelById,
    restoreCreateDraft,
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
    const currentDraftID = (query.get('draft_id') || '').trim();
    const nextDraftID = (draftChannelId || '').trim();
    if (currentDraftID === nextDraftID) {
      return;
    }
    if (nextDraftID === '') {
      query.delete('draft_id');
    } else {
      query.set('draft_id', nextDraftID);
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
    draftChannelId,
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
      inputs: sanitizeDraftInputsForLocalStorage(inputs),
      config: sanitizeDraftConfigForLocalStorage(config),
      modelsSyncError,
      modelsLastSyncedAt,
      verifiedModelSignature,
      capabilityResults,
      capabilityTestError,
      capabilityTestedAt,
      capabilityTestedSignature,
      draft_channel_id: draftChannelId,
      channel_key_set: channelKeySet,
      savedAt: Date.now(),
    };
    localStorage.setItem(CHANNEL_CREATE_DRAFT_KEY, JSON.stringify(payload));
  }, [
    channelKeySet,
    config,
    createStep,
    draftChannelId,
    inputs,
    hasChannelID,
    loading,
    capabilityResults,
    capabilityTestError,
    capabilityTestedAt,
    capabilityTestedSignature,
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
    if (capabilityTestedSignature === '') {
      return;
    }
    if (capabilityTestedSignature === currentCapabilitySignature) {
      return;
    }
    setCapabilityResults([]);
    setCapabilityTestError(t('channel.edit.capability_tester.stale'));
    setCapabilityTestedAt(0);
    setCapabilityTestedSignature('');
  }, [capabilityTestedSignature, currentCapabilitySignature, t]);

  useEffect(() => {
    fetchChannelTypes().then();
  }, [fetchChannelTypes]);

  const submit = async () => {
    const effectiveKey = buildEffectiveKey();
    const modelConfigError = validateModelConfigs(visibleModelConfigs, t);
    if (
      isCreateMode &&
      (inputs.name.trim() === '' ||
        (effectiveKey.trim() === '' && !canReuseStoredKeyForCreate))
    ) {
      showInfo(t('channel.edit.messages.name_required'));
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
    if (isEditMode) {
      res = await API.put(`/api/v1/admin/channel/`, {
        ...localInputs,
        id: channelId,
      });
    } else if (draftChannelId) {
      res = await API.put(`/api/v1/admin/channel/`, {
        ...localInputs,
        id: draftChannelId,
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
        clearCreateDraft();
      }
      navigate('/admin/channel', { replace: true });
      return;
    } else {
      showError(message);
    }
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          {isDetailMode && (
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
              <Button type='button' onClick={handleCancel}>
                <Icon name='undo' />
                {t('channel.edit.buttons.back')}
              </Button>
              <Button type='button' color='blue' onClick={openEditPage}>
                <Icon name='edit' />
                {t('channel.buttons.edit')}
              </Button>
            </div>
          )}
          <Card.Header className='header'>
            {isDetailMode
              ? t('channel.edit.title_detail')
              : isEditMode
                ? t('channel.edit.title_edit')
                : t('channel.edit.title_create')}
          </Card.Header>
          <Form loading={loading} autoComplete='new-password'>
            {isCreateMode && (
              <div style={{ marginBottom: '12px' }}>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    flexWrap: 'wrap',
                  }}
                >
                  <Button
                    type='button'
                    size='tiny'
                    basic={createStep !== 1}
                    color={createStep === 1 ? 'blue' : undefined}
                    onClick={() => goToCreateStep(1)}
                  >
                    {t('channel.edit.wizard.step_basic')}
                  </Button>
                  <Button
                    type='button'
                    size='tiny'
                    basic={createStep !== 2}
                    color={createStep === 2 ? 'blue' : undefined}
                    onClick={moveToStepTwo}
                  >
                    {t('channel.edit.wizard.step_models')}
                  </Button>
                  <Button
                    type='button'
                    size='tiny'
                    basic={createStep !== 3}
                    color={createStep === 3 ? 'blue' : undefined}
                    onClick={moveToStepThree}
                  >
                    {t('channel.edit.wizard.step_capabilities')}
                  </Button>
                  <Button
                    type='button'
                    size='tiny'
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
                <Form.Group widths='equal'>
                  <Form.Field>
                    <Form.Input
                      label={t('channel.edit.name')}
                      name='name'
                      placeholder={t('channel.edit.name_placeholder')}
                      onChange={handleInputChange}
                      value={inputs.name}
                      readOnly={isDetailMode}
                      required
                    />
                  </Form.Field>
                  <Form.Field>
                    {isDetailMode ? (
                      <Form.Input
                        label={t('channel.edit.type')}
                        value={currentProtocolOption?.text || inputs.protocol || '-'}
                        readOnly
                      />
                    ) : (
                      <Form.Select
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
                    <Message>
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
                  <Message>{t('channel.edit.coze_notice')}</Message>
                )}
                {inputs.protocol === 'doubao' && (
                  <Message>
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
                <label>{t('channel.edit.models')}</label>
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    flexWrap: 'wrap',
                    gap: '8px',
                    marginBottom: '10px',
                  }}
                >
                  <span style={{ color: 'rgba(0, 0, 0, 0.6)' }}>
                    {t('channel.edit.model_selector.summary', {
                      selected: inputs.models.length,
                      total: visibleModelConfigs.length,
                    })}
                  </span>
                  {!isDetailMode && (
                    <div
                      style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}
                    >
                      <Button
                        type='button'
                        size='tiny'
                        color='green'
                        loading={fetchModelsLoading}
                        disabled={
                          fetchModelsLoading ||
                          (requiresConnectionVerification &&
                            !hasModelPreviewCredentials)
                        }
                        onClick={() =>
                          handleFetchModels({ silent: false, selectAll: true })
                        }
                      >
                        {fetchModelsButtonText}
                      </Button>
                      <Button
                        type='button'
                        size='tiny'
                        onClick={selectAllModels}
                        disabled={visibleModelConfigs.length === 0}
                      >
                        {t('channel.edit.buttons.select_all')}
                      </Button>
                      <Button
                        type='button'
                        size='tiny'
                        onClick={clearSelectedModels}
                        disabled={inputs.models.length === 0}
                      >
                        {t('channel.edit.buttons.clear')}
                      </Button>
                    </div>
                  )}
                </div>
                <Table celled stackable size='small'>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell collapsing textAlign='center'>
                        {t('channel.edit.model_selector.table.selected')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={5}>
                        {t('channel.edit.model_selector.table.name')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={5}>
                        {t('channel.edit.model_selector.table.alias')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={3}>
                        {t('channel.edit.model_selector.table.price_unit')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={3}>
                        {t('channel.edit.model_selector.table.input_price')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={3}>
                        {t('channel.edit.model_selector.table.output_price')}
                      </Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {visibleModelConfigs.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan='6'>
                          {t('channel.edit.model_selector.empty')}
                        </Table.Cell>
                      </Table.Row>
                    ) : (
                      visibleModelConfigs.map((row) => (
                        <Table.Row key={`${row.upstream_model}-${row.model}`}>
                          <Table.Cell textAlign='center' collapsing>
                            <Checkbox
                              checked={!!row.selected}
                              disabled={isDetailMode}
                              onChange={(e, { checked }) =>
                                toggleModelSelection(
                                  row.upstream_model,
                                  checked,
                                )
                              }
                            />
                          </Table.Cell>
                          <Table.Cell>{row.upstream_model}</Table.Cell>
                          <Table.Cell>
                            {isDetailMode ? (
                              row.model
                            ) : (
                              <Form.Input
                                transparent
                                style={{ minWidth: 220 }}
                                value={row.model}
                                onChange={(e, { value }) =>
                                  updateModelConfigField(
                                    row.upstream_model,
                                    'model',
                                    value || row.upstream_model,
                                  )
                                }
                              />
                            )}
                          </Table.Cell>
                          <Table.Cell>
                            {isDetailMode ? (
                              row.price_unit
                            ) : (
                              <Form.Input
                                transparent
                                readOnly
                                value={row.price_unit}
                              />
                            )}
                          </Table.Cell>
                          <Table.Cell>
                            {isDetailMode ? (
                              row.input_price ?? '-'
                            ) : (
                              <Form.Input
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
                            {isDetailMode ? (
                              row.output_price ?? '-'
                            ) : (
                              <Form.Input
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
                      ))
                    )}
                  </Table.Body>
                </Table>
                {modelsSyncError && (
                  <div style={{ color: '#d9534f', marginTop: '8px' }}>
                    {modelsSyncError}
                  </div>
                )}
              </Form.Field>
            )}
            {showStepThree && inputs.protocol !== 'proxy' && (
              <Form.Field>
                <label>{t('channel.edit.capability_tester.title')}</label>
                <Message info>
                  {t('channel.edit.capability_tester.hint')}
                </Message>
                <div
                  style={{
                    display: 'flex',
                    justifyContent: isDetailMode ? 'flex-end' : 'space-between',
                    alignItems: 'center',
                    flexWrap: 'wrap',
                    gap: '8px',
                    marginBottom: '12px',
                  }}
                >
                  {!isDetailMode && (
                    <Button
                      type='button'
                      color='blue'
                      loading={capabilityTesting}
                      disabled={capabilityTesting || inputs.models.length === 0}
                      onClick={handleTestCapabilities}
                    >
                      {t('channel.edit.capability_tester.button')}
                    </Button>
                  )}
                  {capabilityTestedAt > 0 && (
                    <span style={{ color: 'rgba(0, 0, 0, 0.6)' }}>
                      {t('channel.edit.capability_tester.last_tested', {
                        time: new Date(capabilityTestedAt).toLocaleString(),
                      })}
                    </span>
                  )}
                </div>
                {capabilityTestError && (
                  <div style={{ color: '#d9534f', marginBottom: '12px' }}>
                    {capabilityTestError}
                  </div>
                )}
                <Table celled stackable>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell>
                        {t('channel.edit.capability_tester.table.capability')}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t('channel.edit.capability_tester.table.endpoint')}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t('channel.edit.capability_tester.table.model')}
                      </Table.HeaderCell>
                      <Table.HeaderCell collapsing>
                        {t('channel.edit.capability_tester.table.status')}
                      </Table.HeaderCell>
                      <Table.HeaderCell collapsing>
                        {t('channel.edit.capability_tester.table.latency')}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t('channel.edit.capability_tester.table.message')}
                      </Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {capabilityResults.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan='6'>
                          {t('channel.edit.capability_tester.empty')}
                        </Table.Cell>
                      </Table.Row>
                    ) : (
                      capabilityResults.map((item) => {
                        const labelColor =
                          item.status === 'supported'
                            ? 'green'
                            : item.status === 'skipped'
                              ? 'grey'
                              : 'red';
                        return (
                          <Table.Row
                            key={`${item.capability}-${item.endpoint}-${item.model}`}
                          >
                            <Table.Cell>{item.label}</Table.Cell>
                            <Table.Cell>{item.endpoint || '-'}</Table.Cell>
                            <Table.Cell>{item.model || '-'}</Table.Cell>
                            <Table.Cell>
                              <Label basic color={labelColor}>
                                {t(
                                  `channel.edit.capability_tester.status.${item.status}`,
                                )}
                              </Label>
                            </Table.Cell>
                            <Table.Cell>
                              {item.latency_ms > 0
                                ? `${item.latency_ms} ms`
                                : '-'}
                            </Table.Cell>
                            <Table.Cell>{item.message || '-'}</Table.Cell>
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
                  label={t('channel.edit.system_prompt')}
                  placeholder={t('channel.edit.system_prompt_placeholder')}
                  name='system_prompt'
                  onChange={handleInputChange}
                  value={inputs.system_prompt}
                  style={{
                    minHeight: 150,
                    fontFamily: 'JetBrains Mono, Consolas',
                  }}
                  autoComplete='new-password'
                  {...textAreaReadonlyProps}
                />
              </Form.Field>
            )}
            {showStepOne && inputs.protocol === 'awsclaude' && (
              <Form.Field>
                <Form.Input
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
            {isDetailMode ? null : isEditMode ? (
              <>
                <Button onClick={handleCancel}>
                  {t('channel.edit.buttons.cancel')}
                </Button>
                <Button
                  type='button'
                  positive
                  onClick={submit}
                  disabled={
                    requireVerificationBeforeProceed &&
                    !isCurrentSignatureVerified
                  }
                >
                  {t('channel.edit.buttons.submit')}
                </Button>
              </>
            ) : (
              <>
                <Button type='button' onClick={handleCancel}>
                  {t('channel.edit.buttons.cancel')}
                </Button>
                {createStep > CREATE_CHANNEL_STEP_MIN && (
                  <Button type='button' onClick={moveToPreviousCreateStep}>
                    {t('channel.edit.buttons.previous_step')}
                  </Button>
                )}
                {createStep < CREATE_CHANNEL_STEP_MAX ? (
                  <Button
                    type='button'
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
              </>
            )}
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditChannel;
