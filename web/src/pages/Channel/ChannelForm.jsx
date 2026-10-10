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
import ChannelDetailPublishTab from './components/ChannelDetailPublishTab';
import ChannelDetailTestsTab from './components/ChannelDetailTestsTab';
import ChannelProcurementView from './components/ChannelProcurementView';
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
  AppSelect,
  AppSpin,
  AppTabs,
} from '../../router-ui';
import { CHANNEL_DETAIL_MODEL_COLUMN_WIDTHS } from '../../constants/tableWidthPresets';
import {
  buildBlockedSelectedModelsMessage,
  buildChannelConnectionSignature,
  buildChannelEndpointKey,
  buildChannelModelState,
  buildChannelModelTestSignature,
  buildEmptyEndpointPolicyDraft,
  buildEndpointAccessPolicyJSON,
  buildModelTestResultKey,
  buildNextInputsWithChannelModels,
  buildProviderIndex,
  buildProviderLookupKeys,
  CHANNEL_DEFAULT_CONFIG,
  CHANNEL_ENDPOINT_COLUMN_WIDTHS,
  CHANNEL_IDENTIFIER_MAX_LENGTH,
  CHANNEL_MODEL_PAGE_SIZE,
  CHANNEL_MODEL_TEST_GROUP_COLUMN_WIDTHS,
  CHANNEL_ORIGIN_INPUTS,
  deleteChannelEndpointPolicy,
  ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL,
  ENDPOINT_POLICY_TEMPLATES,
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
  getChannelModelsFromInputs,
  inferAssignableProviderForRowWithOptions,
  isActiveAsyncTaskStatus,
  isEffectiveVolcengineRealtimeProtocol,
  isRecordNotFoundMessage,
  maskChannelKeyPreview,
  mergePriceComponentOverrides,
  missingRequiredBillingCredentialField,
  normalizeAsyncTasks,
  normalizeAsyncTaskStatus,
  normalizeBaseURL,
  normalizeChannelBillingProfile,
  normalizeChannelCostTrackingModeValue,
  normalizeChannelEndpointPolicyRows,
  normalizeChannelEndpointRows,
  normalizeChannelIdentifier,
  normalizeChannelModelEndpoint,
  normalizeChannelModelEndpoints,
  normalizeChannelModelProviderValue,
  normalizeChannelModels,
  normalizeChannelModelType,
  normalizeComplexPriceComponents,
  normalizeDetailTab,
  normalizeExplicitChannelModelEndpoint,
  normalizeExplicitChannelModelEndpoints,
  normalizeModelIDs,
  normalizeModelTestResults,
  normalizePriceOverrideValue,
  normalizePriceUnitValue,
  normalizeProviderIdentifier,
  normalizeSearchKeyword,
  parseEndpointAccessPolicyBaseURL,
  prettyJSONString,
  protocol2secretPrompt,
  protocolSelectionHint,
  PROVIDER_MODEL_TAG_OPTIONS,
  resolveBillingAdapterCredentialFields,
  resolveChannelBillingSourceValue,
  resolveEffectiveAPIBaseURL,
  resolveEffectiveProtocolFromInputs,
  resolveProtocolFromChannelPayload,
  supportsModelTestStream,
  validateChannelIdentifier,
  validateProtocolSpecificChannelConfig,
  VOLCENGINE_STANDARD_PROTOCOL,
} from './ChannelForm.helpers';

const ChannelForm = ({ mode = 'auto' } = {}) => {
  const { t } = useTranslation();
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const channelId = params.id;
  const normalizedRouteChannelId = (channelId || '').toString().trim();
  const normalizedMode = (mode || 'auto').toString().trim().toLowerCase();
  const forceCreateMode =
    normalizedMode === 'add' || normalizedMode === 'create';
  const forceDetailMode =
    normalizedMode === 'edit' || normalizedMode === 'detail';
  const hasChannelID = !forceCreateMode && normalizedRouteChannelId !== '';
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
  const [loading, setLoading] = useState(hasChannelID);
  const activeDetailTab = useMemo(() => {
    if (!isDetailMode) {
      return 'overview';
    }
    const query = new URLSearchParams(location.search);
    return normalizeDetailTab(query.get('tab'));
  }, [isDetailMode, location.search]);
  const [channelKeySet, setChannelKeySet] = useState(false);
  const handleBackToChannelList = useCallback(() => {
    navigate(returnPath || '/admin/channel');
  }, [navigate, returnPath]);
  const handleCancel = () => {
    if (isDetailMode && returnPath !== '') {
      navigate(returnPath);
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
        { replace: false, state: location.state }
      );
    },
    [isDetailMode, location.pathname, location.search, location.state, navigate]
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
    getChannelProtocolOptions()
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
  const [endpointBatchMutating, setEndpointBatchMutating] = useState(false);
  const [channelEndpointPolicies, setChannelEndpointPolicies] = useState([]);
  const [channelEndpointPoliciesLoading, setChannelEndpointPoliciesLoading] =
    useState(false);
  const [channelEndpointPoliciesError, setChannelEndpointPoliciesError] =
    useState('');
  const [endpointPolicyDeletingKey, setEndpointPolicyDeletingKey] =
    useState('');
  const [policyEditorOpen, setPolicyEditorOpen] = useState(false);
  const [policyEditorSaving, setPolicyEditorSaving] = useState(false);
  const [selectedPolicyTemplate, setSelectedPolicyTemplate] = useState('');
  const [policyDraft, setPolicyDraft] = useState(
    buildEmptyEndpointPolicyDraft('', '', '')
  );
  const [modelTesting, setModelTesting] = useState(false);
  const [channelBillingSummary, setChannelBillingSummary] = useState(null);
  const [channelBillingProfile, setChannelBillingProfile] = useState(null);
  const [channelBillingAdapters, setChannelBillingAdapters] = useState([]);
  const [channelBillingSnapshots, setChannelBillingSnapshots] = useState([]);
  const [channelBillingActions, setChannelBillingActions] = useState([]);
  const [channelProcurementBatches, setChannelProcurementBatches] = useState(
    []
  );
  const [channelCostQuotes, setChannelCostQuotes] = useState({
    service_available: false,
    reason: '',
    rows: [],
  });
  const [channelBillingLoading, setChannelBillingLoading] = useState(false);
  const [channelBillingError, setChannelBillingError] = useState('');
  const [channelBillingSubmitting, setChannelBillingSubmitting] =
    useState(false);
  const [detailBillingEditing, setDetailBillingEditing] = useState(false);
  const [detailBillingDraft, setDetailBillingDraft] = useState(null);
  const [modelTestingScope, setModelTestingScope] = useState('');
  const [modelTestingTargets, setModelTestingTargets] = useState([]);
  const [channelTasks, setChannelTasks] = useState([]);
  const [modelTestError, setModelTestError] = useState('');
  const [audioTestLanguage, setAudioTestLanguage] = useState('zh-CN');
  const [responsesTestMode, setResponsesTestMode] = useState('text');
  const [imageEditTestURL, setImageEditTestURL] = useState('');
  const [imageEditTestData, setImageEditTestData] = useState('');
  const [imageEditTestFileName, setImageEditTestFileName] = useState('');
  const handleImageEditTestFileChange = useCallback(
    (event) => {
      const file = event?.target?.files?.[0];
      if (!file) {
        setImageEditTestData('');
        setImageEditTestFileName('');
        return;
      }
      const reader = new FileReader();
      reader.onload = () => {
        setImageEditTestData((reader.result || '').toString());
        setImageEditTestFileName(file.name || '');
      };
      reader.onerror = () => {
        setImageEditTestData('');
        setImageEditTestFileName('');
        showError(t('channel.edit.model_tester.image_edit_file_failed'));
      };
      reader.readAsDataURL(file);
    },
    [t]
  );
  const openChannelTaskView = useCallback(
    (extraParams = {}) => {
      const targetChannelId = (channelId || '').toString().trim();
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
      query.set('tab', 'tasks');
      const search = query.toString();
      navigate(`/admin/channel${search ? `?${search}` : ''}`, {
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
    ]
  );
  const [modelTestedAt, setModelTestedAt] = useState(0);
  const [modelTestedSignature, setModelTestedSignature] = useState('');
  const [modelTestTargetModels, setModelTestTargetModels] = useState([]);
  const [detailModelMutating, setDetailModelMutating] = useState(false);
  const [publishMutatingModel, setPublishMutatingModel] = useState('');
  const [detailBasicEditing, setDetailBasicEditing] = useState(false);
  const [detailEditingModelKey, setDetailEditingModelKey] = useState('');
  const [detailEditingModelSnapshot, setDetailEditingModelSnapshot] =
    useState(null);
  const [detailBasicSaving, setDetailBasicSaving] = useState(false);
  const [config, setConfig] = useState(CHANNEL_DEFAULT_CONFIG);
  const [providerOptions, setProviderOptions] = useState([]);
  const [providerModelOwners, setProviderModelOwners] = useState({});
  const [providerModelDetailsIndex, setProviderModelDetailsIndex] = useState(
    {}
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
    tags: ['text'],
  });
  const [modelSearchKeyword, setModelSearchKeyword] = useState('');
  const [detailModelFilter, setDetailModelFilter] = useState('all');
  const [detailUpstreamStatusFilter, setDetailUpstreamStatusFilter] =
    useState('all');
  const [detailProviderFilter, setDetailProviderFilter] = useState('all');
  const [detailModelPage, setDetailModelPage] = useState(1);
  const [detailModelPageSize, setDetailModelPageSize] = useState(
    CHANNEL_MODEL_PAGE_SIZE,
  );
  const fetchingModelsRef = useRef(false);
  const pendingRefreshTaskIdRef = useRef('');
  const pendingRefreshSignatureRef = useRef('');
  const pendingRefreshBeforeCountRef = useRef(0);
  const pendingRefreshBeforeModelsRef = useRef([]);
  const pendingBillingRefreshTaskIdRef = useRef('');
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
  const effectiveAPIBaseURL = useMemo(
    () => resolveEffectiveAPIBaseURL(inputs, config),
    [config, inputs]
  );
  const previewChannelID = useMemo(
    () => ((hasChannelID ? channelId : '') || '').trim(),
    [channelId, hasChannelID]
  );
  const currentModelSignature = useMemo(
    () =>
      buildChannelConnectionSignature({
        protocol: inputs.protocol,
        key: effectivePreviewKey,
        baseURL: effectiveAPIBaseURL,
        channelID: previewChannelID,
      }),
    [
      effectiveAPIBaseURL,
      effectivePreviewKey,
      inputs.protocol,
      previewChannelID,
    ]
  );
  const requiresConnectionVerification = false;
  const showStepOne = isDetailMode ? activeDetailTab === 'overview' : true;
  const showStepTwo =
    isDetailMode &&
    (activeDetailTab === 'models' || activeDetailTab === 'endpoints');
  const showDetailOverviewTab = isDetailMode && activeDetailTab === 'overview';
  const showDetailModelsTab = isDetailMode && activeDetailTab === 'models';
  const showDetailEndpointsTab =
    isDetailMode && activeDetailTab === 'endpoints';
  const showDetailTestsTab = isDetailMode && activeDetailTab === 'tests';
  const showDetailPublishTab = isDetailMode && activeDetailTab === 'publish';
  const showDetailProcurementTab =
    isDetailMode && activeDetailTab === 'procurement';
  const detailBasicReadonly = isDetailMode && !detailBasicEditing;
  const detailModelsEditing =
    isDetailMode && detailEditingModelKey.toString().trim() !== '';
  const isAnyDetailSectionEditing =
    detailBasicEditing || detailModelsEditing || detailBillingEditing;
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
      key: 'endpoints',
      label: t('channel.edit.detail_tabs.endpoints'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'endpoints',
    },
    {
      key: 'tests',
      label: t('channel.edit.detail_tabs.tests'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'tests',
    },
    {
      key: 'publish',
      label: t('channel.edit.detail_tabs.publish'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'publish',
    },
    {
      key: 'procurement',
      label: t('channel.edit.detail_tabs.procurement'),
      disabled: isAnyDetailSectionEditing && activeDetailTab !== 'procurement',
    },
  ];
  const detailBasicEditLocked =
    isDetailMode &&
    !detailBasicEditing &&
    (detailModelsEditing || detailBillingEditing);
  const detailBillingEditLocked =
    isDetailMode &&
    !detailBillingEditing &&
    (detailBasicEditing || detailModelsEditing);
  const detailModelsEditLocked =
    isDetailMode && (detailBasicEditing || detailBillingEditing);
  const detailTestingReadonly = isDetailMode && isAnyDetailSectionEditing;
  const detailPublishReadonly = isDetailMode && isAnyDetailSectionEditing;
  const inputReadonlyProps = detailBasicReadonly ? { readOnly: true } : {};
  const visibleChannelModels = useMemo(
    () => normalizeChannelModels(inputs.channel_models, inputs.protocol),
    [inputs.channel_models, inputs.protocol]
  );
  const detailEditingModelRow = useMemo(() => {
    if (!detailModelsEditing) {
      return null;
    }
    return (
      visibleChannelModels.find(
        (row) => row.upstream_model === detailEditingModelKey
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
    const modelByName = new Map();
    visibleChannelModels
      .filter((row) => row.selected === true)
      .forEach((row) => {
        const modelName = (row.model || '').toString().trim();
        if (modelName !== '') {
          modelByName.set(modelName, row);
        }
      });
    return channelEndpoints
      .filter((endpointRow) => endpointRow.enabled === true)
      .map((endpointRow) => {
        const modelName = (endpointRow.model || '').toString().trim();
        const endpoint = (endpointRow.endpoint || '').toString().trim();
        const modelRow = modelByName.get(modelName);
        if (!modelRow || endpoint === '') {
          return null;
        }
        return {
          ...modelRow,
          endpoint,
          test_key: buildChannelEndpointKey(modelName, endpoint),
          endpoint_base_url: endpointRow.base_url || '',
          endpoint_updated_at: endpointRow.updated_at || 0,
          endpoint_last_test_status: endpointRow.last_test_status || '',
          endpoint_last_tested_at: endpointRow.last_tested_at || 0,
          endpoint_last_test_error: endpointRow.last_test_error || '',
        };
      })
      .filter(Boolean);
  }, [channelEndpoints, visibleChannelModels]);
  const modelTestingTargetSet = useMemo(
    () => new Set(modelTestingTargets),
    [modelTestingTargets]
  );
  const activeChannelTasksByModel = useMemo(() => {
    const index = new Map();
    normalizeAsyncTasks(channelTasks)
      .filter(
        (item) =>
          item.type === 'channel_model_test' &&
          isActiveAsyncTaskStatus(item.status)
      )
      .forEach((item) => {
        if (!item.model) {
          return;
        }
        const key = buildChannelEndpointKey(item.model, item.endpoint);
        const existing = index.get(key);
        if (!existing || existing.status === 'pending') {
          index.set(key, item);
        }
      });
    return index;
  }, [channelTasks]);
  const activeRefreshModelsTask = useMemo(
    () =>
      normalizeAsyncTasks(channelTasks).find(
        (item) =>
          item.type === 'channel_refresh_models' &&
          isActiveAsyncTaskStatus(item.status)
      ) || null,
    [channelTasks]
  );
  const selectedModelTestHasActiveTasks = useMemo(
    () =>
      modelTestTargetModels.some((targetKey) =>
        activeChannelTasksByModel.has(targetKey)
      ),
    [activeChannelTasksByModel, modelTestTargetModels]
  );
  useEffect(() => {
    const visibleModelSet = new Set(modelTestRows.map((row) => row.test_key));
    setModelTestTargetModels((previous) => {
      const next = previous.filter((targetKey) => visibleModelSet.has(targetKey));
      if (next.length === previous.length) {
        return previous;
      }
      return next;
    });
  }, [modelTestRows]);
  const getProviderOwnersForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider
      );
      const owners = new Set();
      buildProviderLookupKeys(row).forEach((key) => {
        (providerModelOwners[key] || []).forEach((providerId) => {
          owners.add(providerId);
        });
      });
      const sortedOwners = Array.from(owners).sort((a, b) =>
        a.localeCompare(b)
      );
      if (selectedProvider === '' || !sortedOwners.includes(selectedProvider)) {
        return sortedOwners;
      }
      return [
        selectedProvider,
        ...sortedOwners.filter((item) => item !== selectedProvider),
      ];
    },
    [providerModelOwners]
  );
  const getProviderSelectOptionsForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider
      );
      const providerOptionById = new Map(
        (Array.isArray(providerOptions) ? providerOptions : []).map(
          (option) => [normalizeProviderIdentifier(option?.value || ''), option]
        )
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
            normalizedSelectedProvider
        ),
      ];
    },
    [providerOptions]
  );
  const resolvePreferredProviderForModel = useCallback(
    (row) => {
      const selectedProvider = normalizeChannelModelProviderValue(
        row?.provider
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
    [getProviderOwnersForModel]
  );
  const getSelectedProviderDisplayItems = useCallback(
    (row) => {
      const selectedProvider = resolvePreferredProviderForModel(row);
      if (selectedProvider === '') {
        return [];
      }
      const providerOptionById = new Map(
        (Array.isArray(providerOptions) ? providerOptions : []).map(
          (option) => [normalizeProviderIdentifier(option?.value || ''), option]
        )
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
    [providerOptions, resolvePreferredProviderForModel]
  );
  const getComplexPricingDetailsForModel = useCallback(
    (row) => {
      const owners = getProviderOwnersForModel(row);
      const keys = buildProviderLookupKeys(row);
      const details = [];
      const seen = new Set();
      owners.forEach((providerId) => {
        const providerDetails = providerModelDetailsIndex[providerId] || {};
        const detail = keys.map((key) => providerDetails[key]).find(Boolean);
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
          row?.price_components
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
      return details.sort((a, b) => {
        const byProvider = (a.provider || '').localeCompare(b.provider || '');
        if (byProvider !== 0) {
          return byProvider;
        }
        return (a.model || '').localeCompare(b.model || '');
      });
    },
    [getProviderOwnersForModel, providerModelDetailsIndex]
  );
  const getEffectivePriceForModel = useCallback(
    (row, field) => {
      const override = normalizePriceOverrideValue(row?.[field]);
      if (override !== null) {
        return override;
      }
      const owners = getProviderOwnersForModel(row);
      const keys = buildProviderLookupKeys(row);
      let resolved = null;
      owners.some((providerId) => {
        const providerDetails = providerModelDetailsIndex[providerId] || {};
        const detail = keys.map((key) => providerDetails[key]).find(Boolean);
        if (!detail) {
          return false;
        }
        const base = Number(detail?.[field] || 0);
        if (base > 0) {
          resolved = base;
          return true;
        }
        return false;
      });
      return resolved;
    },
    [getProviderOwnersForModel, providerModelDetailsIndex]
  );
  const getEffectivePriceMetaForModel = useCallback(
    (row) => {
      const owners = getProviderOwnersForModel(row);
      const keys = buildProviderLookupKeys(row);
      let matched = null;
      owners.some((providerId) => {
        const providerDetails = providerModelDetailsIndex[providerId] || {};
        const detail = keys.map((key) => providerDetails[key]).find(Boolean);
        if (detail) {
          matched = detail;
          return true;
        }
        return false;
      });
      const overrideUnit = (row?.price_unit || '').toString().trim();
      const priceUnit =
        overrideUnit || (matched?.price_unit || '').toString().trim();
      const currency = (matched?.currency || '').toString().trim();
      return { price_unit: priceUnit, currency };
    },
    [getProviderOwnersForModel, providerModelDetailsIndex]
  );
  const getProviderCandidateEndpointsForModel = useCallback(
    (row) => {
      const providerId = resolvePreferredProviderForModel(row);
      const providerDetails = providerModelDetailsIndex[providerId] || {};
      const providerCandidates = [];
      let matchedProviderDetail = false;
      const detail = buildProviderLookupKeys(row)
        .map((key) => providerDetails[key])
        .find(Boolean);
      if (detail) {
        matchedProviderDetail = true;
        if (Array.isArray(detail.supported_endpoints)) {
          detail.supported_endpoints.forEach((endpoint) => {
            providerCandidates.push(endpoint);
          });
        }
      }
      const candidates = matchedProviderDetail
        ? providerCandidates
        : normalizeExplicitChannelModelEndpoints(
            row?.type,
            row?.endpoints || row?.endpoint_list || [],
            row?.endpoint,
            inputs.protocol
          );
      const seen = new Set();
      const result = [];
      candidates.forEach((endpoint) => {
        const normalized = normalizeChannelModelEndpoint(
          row?.type,
          endpoint,
          inputs.protocol
        );
        if (normalized === '' || seen.has(normalized)) {
          return;
        }
        seen.add(normalized);
        result.push(normalized);
      });
      if (matchedProviderDetail && result.length === 0) {
        return [];
      }
      return result;
    },
    [
      inputs.protocol,
      providerModelDetailsIndex,
      resolvePreferredProviderForModel,
    ]
  );
  const getEffectiveModelEndpoint = useCallback(
    (row) => {
      const configuredEndpoint = (row?.endpoint || '').toString().trim();
      if (configuredEndpoint !== '') {
        return normalizeExplicitChannelModelEndpoint(
          row?.type,
          configuredEndpoint,
          inputs.protocol
        );
      }
      const normalizedCurrent = normalizeExplicitChannelModelEndpoint(
        row?.type,
        row?.endpoint,
        inputs.protocol
      );
      const providerEndpoints = getProviderCandidateEndpointsForModel(row);
      if (
        normalizedCurrent !== '' &&
        providerEndpoints.includes(normalizedCurrent)
      ) {
        return normalizedCurrent;
      }
      return providerEndpoints[0] || '';
    },
    [getProviderCandidateEndpointsForModel, inputs.protocol]
  );
  const openComplexPricingModal = useCallback(
    (row) => {
      const details = getComplexPricingDetailsForModel(row);
      const meta = getEffectivePriceMetaForModel(row);
      setComplexPricingModalData({
        model: row?.upstream_model || row?.model || '',
        base: {
          input_price: getEffectivePriceForModel(row, 'input_price'),
          output_price: getEffectivePriceForModel(row, 'output_price'),
          price_unit: meta.price_unit,
          currency: meta.currency,
        },
        details,
      });
      setComplexPricingModalOpen(true);
    },
    [
      getComplexPricingDetailsForModel,
      getEffectivePriceForModel,
      getEffectivePriceMetaForModel,
    ]
  );
  const closeComplexPricingModal = useCallback(() => {
    setComplexPricingModalOpen(false);
    setComplexPricingModalData(null);
  }, []);
  const hasProviderConfiguredForModel = useCallback(
    (row) => getProviderOwnersForModel(row).length > 0,
    [getProviderOwnersForModel]
  );
  const canSelectChannelModel = useCallback(
    (row) =>
      hasProviderConfiguredForModel(row) &&
      !(row?.enable_block_reason || '').toString().trim(),
    [hasProviderConfiguredForModel]
  );
  const activeChannelModels = useMemo(
    () => visibleChannelModels,
    [visibleChannelModels]
  );
  const detailProviderFilterOptions = useMemo(() => {
    const providerOptionById = new Map(
      (Array.isArray(providerOptions) ? providerOptions : []).map((option) => [
        normalizeProviderIdentifier(option?.value || ''),
        option,
      ])
    );
    const providerIds = new Set();
    visibleChannelModels.forEach((row) => {
      getProviderOwnersForModel(row).forEach((providerId) => {
        const normalizedProvider = normalizeProviderIdentifier(providerId);
        if (normalizedProvider !== '') {
          providerIds.add(normalizedProvider);
        }
      });
    });
    const options = Array.from(providerIds)
      .sort((left, right) => left.localeCompare(right))
      .map((providerId) => {
        const option = providerOptionById.get(providerId);
        return {
          key: providerId,
          value: providerId,
          text: option?.text || providerId,
        };
      });
    return [
      {
        key: 'all',
        value: 'all',
        text: t('channel.edit.model_selector.filters.provider_all'),
      },
      ...options,
    ];
  }, [getProviderOwnersForModel, providerOptions, t, visibleChannelModels]);
  const detailFilteredChannelModels = useMemo(() => {
    if (!isDetailMode) {
      return visibleChannelModels;
    }
    const normalizedProviderFilter =
      normalizeProviderIdentifier(detailProviderFilter);
    return visibleChannelModels.filter((row) => {
      if (detailModelFilter === 'enabled') {
        if (row.selected !== true) {
          return false;
        }
      }
      if (detailModelFilter === 'disabled') {
        if (row.selected === true) {
          return false;
        }
      }
      if (
        detailUpstreamStatusFilter !== '' &&
        detailUpstreamStatusFilter !== 'all'
      ) {
        const syncStatus = (row?.sync_status || 'unknown').toString().trim();
        if (syncStatus !== detailUpstreamStatusFilter) {
          return false;
        }
      }
      if (
        normalizedProviderFilter !== '' &&
        normalizedProviderFilter !== 'all'
      ) {
        return getProviderOwnersForModel(row).some(
          (providerId) =>
            normalizeProviderIdentifier(providerId) === normalizedProviderFilter
        );
      }
      return true;
    });
  }, [
    detailModelFilter,
    detailUpstreamStatusFilter,
    detailProviderFilter,
    getProviderOwnersForModel,
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
      Math.ceil(searchedChannelModels.length / detailModelPageSize)
    );
  }, [searchedChannelModels.length, detailModelPageSize]);
  const renderedChannelModels = useMemo(() => {
    const offset = (detailModelPage - 1) * detailModelPageSize;
    return searchedChannelModels.slice(
      offset,
      offset + detailModelPageSize
    );
  }, [searchedChannelModels, detailModelPage, detailModelPageSize]);
  const modelSelectionSummaryText = useMemo(
    () =>
      t('channel.edit.model_selector.summary', {
        selected: inputs.models.length,
        total: activeChannelModels.length,
      }),
    [activeChannelModels.length, inputs.models.length, t]
  );
  const modelSectionMetaText = useMemo(
    () => modelSelectionSummaryText,
    [modelSelectionSummaryText]
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
      }
    );
  }, [channelEndpoints]);
  const endpointSummaryText = useMemo(
    () =>
      t('channel.edit.endpoint_capabilities.summary', {
        total: endpointCapabilityStats.total,
        capability_enabled: endpointCapabilityStats.enabled,
      }),
    [endpointCapabilityStats.enabled, endpointCapabilityStats.total, t]
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
    const keyDisplayValue =
      detailBasicReadonly
        ? inputs.key_preview || (channelKeySet ? '********' : '-')
        : inputs.key;
    return (
      <AppFormRow>
        <AppField label={t('channel.edit.key')} required={isCreateMode}>
          <AppInput
            className='router-section-input'
            name='key'
            type={detailBasicReadonly ? 'text' : 'password'}
            required={isCreateMode}
            placeholder={
              channelKeySet && (inputs.key || '').trim() === ''
                ? '********'
                : protocol2secretPrompt(inputs.protocol, t)
            }
            onChange={handleInputChange}
            value={keyDisplayValue}
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
    detailBasicEditing,
    detailBasicReadonly,
    inputs.key,
    inputs.key_preview,
    inputs.protocol,
    isCreateMode,
    t,
  ]);

  const buildChannelPayloadFromState = useCallback(
    (baseInputs, baseConfig, options = {}) => {
      const { includeModelState = true } = options;
      const effectiveKey = buildEffectiveKey();
      let localInputs = { ...baseInputs, key: effectiveKey };
      const effectiveProtocol = resolveEffectiveProtocolFromInputs(baseInputs);
      localInputs.id = (localInputs.id || '').toString().trim();
      localInputs.name = normalizeChannelIdentifier(localInputs.name);
      localInputs.protocol = effectiveProtocol;
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
      if (includeModelState) {
        const derivedModelState = buildChannelModelState(
          baseInputs.channel_models,
          effectiveProtocol
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
      };
      localInputs.config = JSON.stringify(submitConfig);
      return localInputs;
    },
    [buildEffectiveKey]
  );

  const buildChannelPayload = useCallback(
    (options = {}) => buildChannelPayloadFromState(inputs, config, options),
    [buildChannelPayloadFromState, config, inputs]
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
        inputs.channel_models,
        inputs.protocol,
        t
      );
      if (blockedMessage !== '') {
        showError(blockedMessage);
        return false;
      }
      const nextInputs = buildNextInputsWithChannelModels(
        inputs,
        nextChannelModels,
        inputs.protocol
      );
      setDetailModelMutating(true);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelID}/models`,
          {
            channel_models: getChannelModelsFromInputs(nextInputs),
          }
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
        try {
          setChannelEndpointsLoading(true);
          const nextEndpoints = await fetchChannelEndpoints(targetChannelID);
          setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
          setChannelEndpointsError('');
        } catch (error) {
          setChannelEndpoints([]);
          setChannelEndpointsError(
            error?.message ||
              t('channel.edit.endpoint_capabilities.load_failed')
          );
        } finally {
          setChannelEndpointsLoading(false);
        }
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.messages.save_channel_failed')
        );
        return false;
      } finally {
        setDetailModelMutating(false);
      }
    },
    [channelId, inputs, isDetailMode, t]
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
      const protocolConfigError = validateProtocolSpecificChannelConfig(
        inputs,
        config,
        t
      );
      if (protocolConfigError !== '') {
        showError(protocolConfigError);
        return false;
      }
      if (includeModelState) {
        const blockedMessage = buildBlockedSelectedModelsMessage(
          inputs.channel_models,
          inputs.channel_models,
          inputs.protocol,
          t
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
          setInputs((prev) => ({
            ...prev,
            key: '',
            key_preview: maskChannelKeyPreview(payload.key),
          }));
        }
        if (successMessage) {
          showSuccess(successMessage);
        }
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.messages.save_channel_failed')
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
    ]
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
  }, [
    detailModelsEditing,
    persistDetailChannelModels,
    t,
    visibleChannelModels,
  ]);

  const loadChannelModelsFromServer = useCallback(
    async (targetChannelId, protocol) => {
      try {
        return await fetchAllChannelModels(targetChannelId, protocol);
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

  const loadChannelEndpointsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelEndpoints(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.endpoint_capabilities.load_failed')
        );
      }
    },
    [t]
  );

  const loadChannelEndpointPoliciesFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelEndpointPolicies(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.endpoint_policies.load_failed')
        );
      }
    },
    [t]
  );

  const loadChannelTasksFromServer = useCallback(async (targetChannelId) => {
    try {
      return await fetchActiveChannelTasks(targetChannelId);
    } catch (error) {
      throw new Error(error?.message || 'fetch channel tasks failed');
    }
  }, []);

  const loadChannelBillingAdaptersFromServer = useCallback(async () => {
    try {
      return await fetchChannelBillingAdapters();
    } catch (error) {
      throw new Error(
        error?.message || t('channel.edit.billing.adapters_load_failed')
      );
    }
  }, [t]);

  const loadChannelBillingSummaryFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelBillingSummary(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.billing.load_failed')
        );
      }
    },
    [t]
  );

  const loadChannelBillingProfileFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelBillingProfile(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.billing.load_failed')
        );
      }
    },
    [t]
  );

  const loadChannelBillingSnapshotsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelBillingSnapshots(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.billing.load_failed')
        );
      }
    },
    [t]
  );

  const loadChannelBillingActionsFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelBillingActions(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.billing.load_failed')
        );
      }
    },
    [t]
  );

  const loadChannelProcurementBatchesFromServer = useCallback(
    async (targetChannelId) => {
      try {
        return await fetchChannelProcurementBatches(targetChannelId);
      } catch (error) {
        throw new Error(
          error?.message || t('channel.edit.billing.load_failed')
        );
      }
    },
    [t]
  );

  const refreshChannelBillingState = useCallback(
    async (targetChannelId) => {
      const normalizedChannelId = (targetChannelId || '').toString().trim();
      if (normalizedChannelId === '') {
        return;
      }
      setChannelBillingLoading(true);
      try {
        const [summary, profile, adapters] =
          await Promise.all([
            loadChannelBillingSummaryFromServer(normalizedChannelId),
            loadChannelBillingProfileFromServer(normalizedChannelId),
            loadChannelBillingAdaptersFromServer(),
          ]);
        setChannelBillingAdapters(adapters);
        setChannelBillingSummary(summary);
        setChannelBillingProfile(profile);
        setDetailBillingDraft(profile);
        setChannelBillingError('');
      } catch (error) {
        setChannelBillingError(
          error?.message || t('channel.edit.billing.load_failed')
        );
      } finally {
        setChannelBillingLoading(false);
      }
    },
    [
      loadChannelBillingAdaptersFromServer,
      loadChannelBillingProfileFromServer,
      loadChannelBillingSummaryFromServer,
      t,
    ]
  );

  const refreshChannelProcurementState = useCallback(
    async (targetChannelId) => {
      const normalizedChannelId = (targetChannelId || '').toString().trim();
      if (normalizedChannelId === '') {
        return;
      }
      setChannelBillingLoading(true);
      try {
        const [snapshots, batches] = await Promise.all([
          loadChannelBillingSnapshotsFromServer(normalizedChannelId),
          loadChannelProcurementBatchesFromServer(normalizedChannelId),
        ]);
        setChannelBillingSnapshots(Array.isArray(snapshots) ? snapshots : []);
        setChannelProcurementBatches(Array.isArray(batches) ? batches : []);
        setChannelBillingError('');
        // Cost-quote reconciliation is read-only and optional: a missing/errored
        // billing service must never block the procurement view.
        try {
          const quotes = await fetchChannelCostQuotes(normalizedChannelId);
          setChannelCostQuotes(
            quotes || { service_available: false, reason: '', rows: [] }
          );
        } catch (quoteError) {
          setChannelCostQuotes({
            service_available: false,
            reason: quoteError?.message || '',
            rows: [],
          });
        }
      } catch (error) {
        setChannelBillingError(
          error?.message || t('channel.edit.billing.load_failed')
        );
      } finally {
        setChannelBillingLoading(false);
      }
    },
    [
      loadChannelBillingSnapshotsFromServer,
      loadChannelProcurementBatchesFromServer,
      t,
    ]
  );
  useEffect(() => {
    if (!isDetailMode || activeDetailTab !== 'procurement') {
      return;
    }
    const targetChannelId = (channelId || '').toString().trim();
    if (targetChannelId === '') {
      return;
    }
    refreshChannelProcurementState(targetChannelId);
  }, [isDetailMode, activeDetailTab, channelId, refreshChannelProcurementState]);

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
        nextBillingSummary,
        nextBillingProfile,
      ] = await Promise.all([
        loadChannelModelsFromServer(normalizedChannelId, inputs.protocol),
        loadChannelTestsFromServer(normalizedChannelId),
        loadChannelTasksFromServer(normalizedChannelId),
        loadChannelEndpointsFromServer(normalizedChannelId),
        loadChannelEndpointPoliciesFromServer(normalizedChannelId),
        loadChannelBillingSummaryFromServer(normalizedChannelId),
        loadChannelBillingProfileFromServer(normalizedChannelId),
      ]);
      const nextInputs = buildNextInputsWithChannelModels(
        inputs,
        nextChannelModels,
        inputs.protocol
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
          : 0
      );
      setModelTestedSignature(
        Number(nextTests.lastTestedAt || 0) > 0 ? nextSignature : ''
      );
      setChannelTasks(normalizeAsyncTasks(nextTasks));
      setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
      setChannelEndpointsError('');
      setChannelEndpointPolicies(
        normalizeChannelEndpointPolicyRows(nextPolicies)
      );
      setChannelEndpointPoliciesError('');
      setChannelBillingSummary(nextBillingSummary);
      setChannelBillingProfile(nextBillingProfile);
      setDetailBillingDraft(nextBillingProfile);
      setChannelBillingError('');
      return { channelModels: nextChannelModels };
    },
    [
      effectiveAPIBaseURL,
      effectivePreviewKey,
      inputs,
      inputs.protocol,
      loadChannelBillingProfileFromServer,
      loadChannelBillingSummaryFromServer,
      loadChannelEndpointPoliciesFromServer,
      loadChannelEndpointsFromServer,
      loadChannelModelsFromServer,
      loadChannelTasksFromServer,
      loadChannelTestsFromServer,
    ]
  );

  const updateChannelModelPublish = useCallback(
    async (row, publishEnabled) => {
      if (!isDetailMode || detailPublishReadonly) {
        return false;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const modelName = (row?.model || row?.upstream_model || '')
        .toString()
        .trim();
      const publishedModel = (row?.published_model || modelName).toString().trim();
      if (targetChannelId === '' || modelName === '') {
        return false;
      }
      if (publishEnabled && publishedModel === '') {
        showError(t('channel.edit.publish.published_model_required'));
        return false;
      }
      setPublishMutatingModel(modelName);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/models/publish`,
          {
            model: modelName,
            publish_enabled: !!publishEnabled,
            published_model: publishedModel,
          }
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(message || t('channel.edit.publish.update_failed'));
          return false;
        }
        await refreshChannelRuntimeState(targetChannelId);
        showSuccess(
          t(
            publishEnabled
              ? 'channel.edit.publish.publish_success'
              : 'channel.edit.publish.unpublish_success'
          )
        );
        return true;
      } catch (error) {
        showError(error?.message || t('channel.edit.publish.update_failed'));
        return false;
      } finally {
        setPublishMutatingModel('');
      }
    },
    [
      channelId,
      detailPublishReadonly,
      isDetailMode,
      refreshChannelRuntimeState,
      t,
    ]
  );

  const updateChannelModelPublishedName = useCallback(
    (row, value) => {
      const targetModel = (row?.model || row?.upstream_model || '')
        .toString()
        .trim();
      if (targetModel === '' || detailPublishReadonly) {
        return;
      }
      const normalizedValue = (value || '').toString().trim();
      setInputs((prev) =>
        buildNextInputsWithChannelModels(
          prev,
          visibleChannelModels.map((item) => {
            const itemModel = (item?.model || item?.upstream_model || '')
              .toString()
              .trim();
            if (itemModel !== targetModel) {
              return item;
            }
            const fallbackName = (item.model || item.upstream_model || '')
              .toString()
              .trim();
            return {
              ...item,
              published_model: normalizedValue,
            };
          }),
          prev.protocol
        )
      );
    },
    [detailPublishReadonly, visibleChannelModels]
  );

  const submitChannelBillingRefresh = useCallback(
    async (targetChannelId, options = {}) => {
      const normalizedChannelId = (targetChannelId || '').toString().trim();
      if (normalizedChannelId === '') {
        throw new Error(t('channel.messages.billing_update_submit_failed'));
      }
      const { silent = false } = options;
      const res = await API.post(
        `/api/v1/admin/channel/${normalizedChannelId}/refresh`,
        {
          action: 'billing',
        }
      );
      const { success, message, data, meta } = res.data || {};
      if (!success) {
        throw new Error(
          message || t('channel.messages.billing_update_submit_failed')
        );
      }
      const refreshTask = normalizeAsyncTasks([data?.task])[0];
      if (!refreshTask?.id) {
        throw new Error(t('channel.messages.billing_update_submit_failed'));
      }
      pendingBillingRefreshTaskIdRef.current = refreshTask.id;
      setChannelTasks((prev) =>
        normalizeAsyncTasks([...normalizeAsyncTasks(prev), refreshTask])
      );
      if (!silent) {
        showSuccess(
          meta?.reused
            ? t('channel.messages.billing_update_reused', {
                name: (inputs.name || normalizedChannelId).toString().trim(),
              })
            : t('channel.messages.billing_update_submitted', {
                name: (inputs.name || normalizedChannelId).toString().trim(),
              })
        );
      }
      return refreshTask;
    },
    [inputs.name, t]
  );

  const refreshChannelBillingNow = useCallback(async () => {
    const targetChannelId = (channelId || '').toString().trim();
    if (targetChannelId === '') {
      return;
    }
    setChannelBillingSubmitting(true);
    try {
      await submitChannelBillingRefresh(targetChannelId);
    } catch (error) {
      showError(
        error?.message || t('channel.messages.billing_update_submit_failed')
      );
    } finally {
      if (pendingBillingRefreshTaskIdRef.current === '') {
        setChannelBillingSubmitting(false);
      }
    }
  }, [channelId, submitChannelBillingRefresh, t]);

  const updateChannelManualBillingSnapshot = useCallback(
    async ({
      id,
      purchase_at,
      purchase_currency,
      purchase_amount,
      purchase_fx_rate,
      purchase_cost_amount,
      entitlement_name,
      event_type,
      parent_snapshot_id,
      old_batch_disposition,
      valid_from,
      valid_until,
      items,
      message,
    }) => {
      const targetChannelId = (channelId || '').toString().trim();
      if (targetChannelId === '') {
        return false;
      }
      const normalizedItems = (Array.isArray(items) ? items : [])
        .map((item) => ({
          id: (item?.id || '').toString().trim(),
          resource_type: (item?.resource_type || '').toString().trim(),
          quota_type: (item?.quota_type || '').toString().trim(),
          quota_label: (item?.quota_label || '').toString().trim(),
          amount: Number(item?.amount),
          limit_amount: Number(item?.limit_amount || 0),
          used_amount: Number(item?.used_amount || 0),
          remaining_amount: Number(item?.remaining_amount || 0),
          currency: (item?.currency || '').toString().trim(),
          reset_at: Number(item?.reset_at || 0),
          expires_at: Number(item?.expires_at || 0),
          source_ref: (item?.source_ref || '').toString().trim(),
        }))
        .filter(
          (item) =>
            item.resource_type !== '' &&
            (item.resource_type === 'plan' ||
              (Number.isFinite(item.amount) &&
                item.amount >= 0 &&
                (item.amount > 0 ||
                  item.limit_amount > 0 ||
                  item.remaining_amount > 0)))
        );
      if (normalizedItems.length === 0) {
        showInfo(t('channel.edit.billing.manual_snapshot_invalid'));
        return false;
      }
      setChannelBillingSubmitting(true);
      try {
        const method = (id || '').toString().trim() === '' ? 'post' : 'put';
        const path =
          (id || '').toString().trim() === ''
            ? `/api/v1/admin/channel/${targetChannelId}/billing/snapshots`
            : `/api/v1/admin/channel/${targetChannelId}/billing/snapshots/${(id || '')
                .toString()
                .trim()}`;
        const res = await API[method](
          path,
          {
            ...(method === 'put' ? { id: (id || '').toString().trim() } : {}),
            purchase_at: Number(purchase_at || 0),
            purchase_currency: (purchase_currency || '').toString().trim(),
            purchase_amount: Number(purchase_amount || 0),
            purchase_fx_rate: Number(purchase_fx_rate || 0),
            purchase_cost_amount: Number(purchase_cost_amount || 0),
            entitlement_name: (entitlement_name || '').toString().trim(),
            event_type: (event_type || 'purchase').toString().trim(),
            parent_snapshot_id: (parent_snapshot_id || '').toString().trim(),
            old_batch_disposition: (old_batch_disposition || 'keep').toString().trim(),
            valid_from: Number(valid_from || 0),
            valid_until: Number(valid_until || 0),
            items: normalizedItems,
            message: (message || '').toString().trim(),
          }
        );
        const { success, message: responseMessage } = res.data || {};
        if (!success) {
          showError(
            responseMessage || t('channel.edit.billing.manual_snapshot_failed')
          );
          return false;
        }
        await refreshChannelBillingState(targetChannelId);
        await refreshChannelProcurementState(targetChannelId);
        showSuccess(t('channel.edit.billing.manual_snapshot_success'));
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.billing.manual_snapshot_failed')
        );
        return false;
      } finally {
        setChannelBillingSubmitting(false);
      }
    },
    [channelId, refreshChannelBillingState, refreshChannelProcurementState, t]
  );

  const deleteChannelManualBillingSnapshot = useCallback(
    async (snapshotId) => {
      const targetChannelId = (channelId || '').toString().trim();
      const normalizedSnapshotId = (snapshotId || '').toString().trim();
      if (targetChannelId === '' || normalizedSnapshotId === '') {
        return false;
      }
      setChannelBillingSubmitting(true);
      try {
        const res = await API.delete(
          `/api/v1/admin/channel/${targetChannelId}/billing/snapshots/${normalizedSnapshotId}`
        );
        const { success, message: responseMessage } = res.data || {};
        if (!success) {
          showError(
            responseMessage || t('channel.edit.billing.delete_purchase_record_failed')
          );
          return false;
        }
        await refreshChannelBillingState(targetChannelId);
        await refreshChannelProcurementState(targetChannelId);
        showSuccess(t('channel.edit.billing.delete_purchase_record_success'));
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.billing.delete_purchase_record_failed')
        );
        return false;
      } finally {
        setChannelBillingSubmitting(false);
      }
    },
    [
      channelId,
      refreshChannelBillingState,
      refreshChannelProcurementState,
      showError,
      showSuccess,
      t,
    ]
  );

  const updateChannelProcurementBatchCost = useCallback(
    async (batchId, payload) => {
      const targetChannelId = (channelId || '').toString().trim();
      const normalizedBatchId = (batchId || '').toString().trim();
      if (targetChannelId === '' || normalizedBatchId === '') {
        return false;
      }
      setChannelBillingSubmitting(true);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/billing/procurement-batches/${normalizedBatchId}/cost`,
          {
            purchase_currency: (payload?.purchase_currency || '')
              .toString()
              .trim(),
            purchase_amount: Number(payload?.purchase_amount || 0),
            purchase_fx_rate: Number(payload?.purchase_fx_rate || 0),
            purchase_cost_amount: Number(payload?.purchase_cost_amount || 0),
            capacity_effective: Number(payload?.capacity_effective || 0),
            cost_source: (payload?.cost_source || 'actual').toString().trim(),
            cost_status: (payload?.cost_status || 'active').toString().trim(),
            scope_type: (payload?.scope_type || 'global').toString().trim(),
            scope_value: (payload?.scope_value || '').toString().trim(),
          }
        );
        const { success, message: responseMessage } = res.data || {};
        if (!success) {
          showError(
            responseMessage ||
              t('channel.edit.billing.procurement_update_failed')
          );
          return false;
        }
        await refreshChannelBillingState(targetChannelId);
        await refreshChannelProcurementState(targetChannelId);
        showSuccess(t('channel.edit.billing.procurement_update_success'));
        return true;
      } catch (error) {
        showError(
          error?.message || t('channel.edit.billing.procurement_update_failed')
        );
        return false;
      } finally {
        setChannelBillingSubmitting(false);
      }
    },
    [channelId, refreshChannelBillingState, refreshChannelProcurementState, t]
  );

  const updateChannelProcurementBatchStatus = useCallback(
    async (batchId, costStatus) => {
      const targetChannelId = (channelId || '').toString().trim();
      const normalizedBatchId = (batchId || '').toString().trim();
      if (targetChannelId === '' || normalizedBatchId === '') {
        return false;
      }
      setChannelBillingSubmitting(true);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/billing/procurement-batches/${normalizedBatchId}/status`,
          {
            cost_status: (costStatus || '').toString().trim(),
          }
        );
        const { success, message: responseMessage } = res.data || {};
        if (!success) {
          showError(
            responseMessage ||
              t('channel.edit.billing.procurement_status_update_failed')
          );
          return false;
        }
        await refreshChannelBillingState(targetChannelId);
        await refreshChannelProcurementState(targetChannelId);
        showSuccess(
          t('channel.edit.billing.procurement_status_update_success')
        );
        return true;
      } catch (error) {
        showError(
          error?.message ||
            t('channel.edit.billing.procurement_status_update_failed')
        );
        return false;
      } finally {
        setChannelBillingSubmitting(false);
      }
    },
    [channelId, refreshChannelBillingState, refreshChannelProcurementState, t]
  );

  const loadChannelProcurementBatchConsumptions = useCallback(
    async (batchId) => {
      const targetChannelId = (channelId || '').toString().trim();
      const normalizedBatchId = (batchId || '').toString().trim();
      if (targetChannelId === '' || normalizedBatchId === '') {
        return [];
      }
      try {
        return await fetchChannelProcurementBatchConsumptions(
          targetChannelId,
          normalizedBatchId
        );
      } catch (error) {
        showError(
          error?.message ||
            t('channel.edit.billing.procurement_consumptions_load_failed')
        );
        return [];
      }
    },
    [channelId, t]
  );

  const updateBillingProfileDraft = useCallback(
    (patch) => {
      setDetailBillingDraft((prev) => ({
        ...(prev || {
          channel_id: (channelId || '').toString().trim(),
          billing_source: 'manual',
          cost_tracking_mode: 'untracked',
          billing_credentials: {},
          action_capabilities: [],
        }),
        ...(patch || {}),
      }));
    },
    [channelId]
  );

  const cancelDetailBillingEdit = useCallback(() => {
    setDetailBillingDraft(channelBillingProfile);
    setDetailBillingEditing(false);
  }, [channelBillingProfile]);

  const saveDetailBillingProfile = useCallback(async () => {
    const targetChannelId = (channelId || '').toString().trim();
    if (targetChannelId === '' || !detailBillingDraft) {
      return;
    }
    const billingSource = resolveChannelBillingSourceValue(
      detailBillingDraft.billing_source,
      channelBillingAdapters
    );
    const credentialFields = resolveBillingAdapterCredentialFields(
      billingSource,
      channelBillingAdapters
    );
    const billingCredentials =
      billingSource === 'manual'
        ? {}
        : filterBillingCredentialsByFields(
            detailBillingDraft.billing_credentials,
            credentialFields
          );
    const missingCredentialField = missingRequiredBillingCredentialField(
      credentialFields,
      billingCredentials
    );
    if (missingCredentialField) {
      showError(
        t('channel.edit.billing.credential_required', {
          field: missingCredentialField,
        })
      );
      return;
    }
    setChannelBillingSubmitting(true);
    try {
      const res = await API.put(
        `/api/v1/admin/channel/${targetChannelId}/billing/profile`,
        {
          billing_source: billingSource,
          cost_tracking_mode: normalizeChannelCostTrackingModeValue(
            detailBillingDraft.cost_tracking_mode
          ),
          billing_credentials: billingCredentials,
        }
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('channel.edit.billing.profile_update_failed'));
        return;
      }
      const normalizedProfile = normalizeChannelBillingProfile(data);
      setChannelBillingProfile(normalizedProfile);
      setDetailBillingDraft(normalizedProfile);
      setDetailBillingEditing(false);
      await refreshChannelBillingState(targetChannelId);
      if (
        Array.isArray(normalizedProfile?.action_capabilities) &&
        normalizedProfile.action_capabilities.includes('refresh_billing')
      ) {
        await submitChannelBillingRefresh(targetChannelId, { silent: true });
      }
      showSuccess(t('channel.edit.billing.profile_update_success'));
    } catch (error) {
      showError(
        error?.message || t('channel.edit.billing.profile_update_failed')
      );
    } finally {
      setChannelBillingSubmitting(false);
    }
  }, [
    channelId,
    channelBillingAdapters,
    detailBillingDraft,
    refreshChannelBillingState,
    submitChannelBillingRefresh,
    t,
  ]);

  // Immediate-save for the cost-tracking mode switch that lives on the cost tab.
  // Unlike the overview edit flow it has no edit/cancel state: it reuses the
  // already-loaded billing source + credentials and only changes the mode, then
  // reloads the profile (for the accurate cost-missing count) and procurement
  // facts (free/actual change the auto-managed batches).
  const saveChannelCostTrackingMode = useCallback(
    async (mode) => {
      const targetChannelId = (channelId || '').toString().trim();
      if (targetChannelId === '') {
        return;
      }
      const billingSource = resolveChannelBillingSourceValue(
        channelBillingProfile?.billing_source,
        channelBillingAdapters
      );
      const credentialFields = resolveBillingAdapterCredentialFields(
        billingSource,
        channelBillingAdapters
      );
      const billingCredentials =
        billingSource === 'manual'
          ? {}
          : filterBillingCredentialsByFields(
              channelBillingProfile?.billing_credentials,
              credentialFields
            );
      setChannelBillingSubmitting(true);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/billing/profile`,
          {
            billing_source: billingSource,
            cost_tracking_mode: normalizeChannelCostTrackingModeValue(mode),
            billing_credentials: billingCredentials,
          }
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(message || t('channel.edit.billing.profile_update_failed'));
          return;
        }
        await refreshChannelBillingState(targetChannelId);
        await refreshChannelProcurementState(targetChannelId);
        showSuccess(t('channel.edit.billing.profile_update_success'));
      } catch (error) {
        showError(
          error?.message || t('channel.edit.billing.profile_update_failed')
        );
      } finally {
        setChannelBillingSubmitting(false);
      }
    },
    [
      channelId,
      channelBillingProfile,
      channelBillingAdapters,
      refreshChannelBillingState,
      refreshChannelProcurementState,
      t,
    ]
  );

  const loadChannelById = useCallback(
    async (targetId, fromCreating = false) => {
      try {
        let res = await API.get(`/api/v1/admin/channel/${targetId}`);
        const { success, message, data } = res.data;
        if (success) {
          const [
            remoteChannelModels,
            channelTestsData,
            activeTasks,
            billingSummaryData,
            billingProfileData,
            billingAdaptersData,
          ] = await Promise.all([
            loadChannelModelsFromServer(
              data.id || targetId,
              resolveProtocolFromChannelPayload(data)
            ),
            loadChannelTestsFromServer(data.id || targetId),
            loadChannelTasksFromServer(data.id || targetId),
            loadChannelBillingSummaryFromServer(data.id || targetId),
            loadChannelBillingProfileFromServer(data.id || targetId),
            loadChannelBillingAdaptersFromServer(),
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
          const modelState = buildChannelModelState(
            remoteChannelModels,
            normalizedProtocol
          );
          const loadedModelTestSignature = buildChannelModelTestSignature({
            protocol: normalizedProtocol,
            key: '',
            baseURL: resolveEffectiveAPIBaseURL(data, parsedConfig),
            channelID: data.id || targetId,
            models: modelState.selectedModels,
            channelModels: modelState.channelModels,
          });

          pendingRefreshTaskIdRef.current = '';
          pendingRefreshSignatureRef.current = '';
          setInputs({
            id: data.id,
            name: data.name || '',
            protocol: normalizedProtocol,
            key: '',
            key_preview: data.key_preview || '',
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
              : ''
          );
          setModelTestTargetModels([]);
          setChannelTasks(normalizeAsyncTasks(activeTasks));
          setChannelBillingSummary(billingSummaryData);
          setChannelBillingProfile(billingProfileData);
          setDetailBillingDraft(billingProfileData);
          setChannelBillingAdapters(billingAdaptersData);
          setChannelBillingError('');
          setConfig({
            ...CHANNEL_DEFAULT_CONFIG,
            ...parsedConfig,
            api_base_url: normalizeBaseURL(parsedConfig.api_base_url),
          });
          if (hasChannelID) {
            setChannelKeySet(!!data.key_set);
          } else {
            setChannelKeySet(false);
          }
        } else {
          if (isRecordNotFoundMessage(message)) {
            showInfo(t('channel.edit.messages.channel_not_found'));
            navigate('/admin/channel', { replace: true });
            return;
          }
          showError(message);
        }
      } catch (error) {
        if (isRecordNotFoundMessage(error?.message)) {
          showInfo(t('channel.edit.messages.channel_not_found'));
          navigate('/admin/channel', { replace: true });
          return;
        }
        showError(error?.message || t('channel.edit.messages.load_failed'));
      } finally {
        setLoading(false);
      }
    },
    [
      navigate,
      hasChannelID,
      loadChannelBillingProfileFromServer,
      loadChannelBillingSummaryFromServer,
      loadChannelModelsFromServer,
      loadChannelTasksFromServer,
      loadChannelTestsFromServer,
      t,
    ]
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
              : row
          ),
          prev.protocol
        )
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
          `/api/v1/admin/channel/${targetChannelId}/refresh`
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
          normalizeAsyncTasks([...normalizeAsyncTasks(prev), refreshTask])
        );
        pendingRefreshTaskIdRef.current = refreshTask.id;
        pendingRefreshSignatureRef.current = requestSignature;
        pendingRefreshBeforeCountRef.current = Array.isArray(
          inputs.channel_models,
        )
          ? inputs.channel_models.length
          : 0;
        pendingRefreshBeforeModelsRef.current = Array.isArray(
          inputs.channel_models,
        )
          ? inputs.channel_models
              .map((row) => (row?.model || '').toString().trim())
              .filter((name) => name !== '')
          : [];
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
    ]
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
              t('channel.edit.model_selector.provider_load_failed')
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
    ]
  );

  const startDetailModelEdit = useCallback(
    (upstreamModel) => {
      const targetModel = (upstreamModel || '').toString().trim();
      if (targetModel === '') {
        return;
      }
      const currentRow =
        visibleChannelModels.find(
          (row) => row.upstream_model === targetModel
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
    ]
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
          providerIndex.providerOptions
        ),
        model: (row?.upstream_model || row?.model || '').toString().trim(),
        type: normalizeChannelModelType(row?.type),
      });
      setAppendProviderModalOpen(true);
    },
    [loadProviderIndex, t]
  );

  const closeAppendProviderModal = useCallback(() => {
    if (appendingProviderModel) {
      return;
    }
    setAppendProviderModalOpen(false);
    setAppendProviderForm({
      provider: '',
      model: '',
      tags: ['text'],
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
          tags: Array.isArray(appendProviderForm.tags)
            ? appendProviderForm.tags
            : [],
        }
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(
          message || t('channel.edit.model_selector.provider_append_failed')
        );
        return;
      }
      await loadProviderIndex({ silent: true, force: true });
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
  }, [appendProviderForm, closeAppendProviderModal, loadProviderIndex, t]);

  const handleRunModelTests = useCallback(
    async ({ targetModels = [], targetConfigs = [], scope = 'batch' } = {}) => {
      if (!isDetailMode) {
        return;
      }
      if (detailTestingReadonly) {
        return;
      }
      if (inputs.protocol === 'proxy') {
        return;
      }
      const explicitConfigs = Array.isArray(targetConfigs) ? targetConfigs : [];
      const selectedTargetKeys =
        explicitConfigs.length > 0
          ? explicitConfigs
              .map((item) => buildChannelEndpointKey(item?.model, item?.endpoint))
              .filter((item) => item !== '::')
          : normalizeModelIDs(
              Array.isArray(targetModels) && targetModels.length > 0
                ? targetModels
                : modelTestTargetModels
            );
      const targetKeySet = new Set(selectedTargetKeys);
      const selectedRows =
        explicitConfigs.length > 0
          ? explicitConfigs
              .map((item) => {
                const key = buildChannelEndpointKey(item?.model, item?.endpoint);
                return modelTestRows.find((row) => row.test_key === key) || null;
              })
              .filter(Boolean)
          : modelTestRows.filter((row) => targetKeySet.has(row.test_key));
      if (selectedRows.length === 0) {
        showInfo(t('channel.edit.messages.models_required'));
        return;
      }
      const targetChannelId = (channelId || '').toString().trim();
      if (targetChannelId === '') {
        return;
      }
      const nextTargetConfigs = selectedRows.map((row) => {
          const endpoint = getEffectiveModelEndpoint(row);
          const targetConfig = {
            model: row.model,
            endpoint,
          };
          if (supportsModelTestStream(row)) {
            targetConfig.is_stream = !!row.is_stream;
          }
          if (endpoint === '/v1/responses') {
            targetConfig.responses_test_mode = responsesTestMode;
          }
          return targetConfig;
        });
      const normalizedTargets = normalizeModelIDs(
        selectedRows.map((row) => row.model)
      );
      setModelTesting(true);
      setModelTestingScope(scope === 'single' ? 'single' : 'batch');
      setModelTestingTargets(selectedRows.map((row) => row.test_key));
      try {
        const res = await API.post(
          `/api/v1/admin/channel/${targetChannelId}/tests`,
          {
            test_model: inputs.test_model || '',
            target_models: normalizedTargets,
            target_configs: nextTargetConfigs,
            audio_language: audioTestLanguage,
            image_edit_url: imageEditTestURL,
            image_edit_data: imageEditTestData,
          }
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
          normalizeAsyncTasks([...normalizeAsyncTasks(prev), ...nextTasks])
        );
        setModelTestError('');
        showSuccess(
          t('channel.edit.model_tester.task_created', {
            count: Number(meta?.created || nextTasks.length || 0),
            reused: Number(meta?.reused || 0),
          })
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
      modelTestRows,
      audioTestLanguage,
      responsesTestMode,
      imageEditTestURL,
      imageEditTestData,
    ]
  );

  const toggleModelTestTarget = useCallback(
    (targetKey, checked) => {
      if (detailTestingReadonly) {
        return;
      }
      setModelTestTargetModels((prev) => {
        const normalized = (targetKey || '').toString().trim();
        if (normalized === '') {
          return prev;
        }
        if (checked) {
          return normalizeModelIDs([...prev, normalized]);
        }
        return prev.filter((item) => item !== normalized);
      });
    },
    [detailTestingReadonly]
  );

  const updateAllModelTestStreams = useCallback(
    async (isStream, modelNames = []) => {
      if (detailTestingReadonly) {
        return;
      }
      const targetSet = new Set(
        (Array.isArray(modelNames) ? modelNames : [])
          .map((item) => (item || '').toString().trim())
          .filter(Boolean)
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
        buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol)
      );
    },
    [
      detailTestingReadonly,
      isDetailMode,
      persistDetailChannelModels,
      visibleChannelModels,
    ]
  );

  const updateChannelEndpointCapability = useCallback(
    async (row, nextValues = {}) => {
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
      setEndpointMutatingKey(endpointKey);
      try {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/endpoints`,
          {
            model: modelName,
            endpoint,
            base_url: baseURL,
            enabled: !!enabled,
          }
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(
            message || t('channel.edit.endpoint_capabilities.update_failed')
          );
          return;
        }
        const nextEndpoints = await loadChannelEndpointsFromServer(
          targetChannelId
        );
        setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
        setChannelEndpointsError('');
        showSuccess(
          t(
            enabled
              ? 'channel.edit.endpoint_capabilities.enable_success'
              : 'channel.edit.endpoint_capabilities.disable_success'
          )
        );
      } catch (error) {
        showError(
          error?.message ||
            t('channel.edit.endpoint_capabilities.update_failed')
        );
      } finally {
        setEndpointMutatingKey('');
      }
    },
    [
      channelId,
      endpointCapabilityReadonly,
      isDetailMode,
      loadChannelEndpointsFromServer,
      t,
    ]
  );

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
      const endpointPolicies = channelEndpointPolicies.filter(
        (item) => item.model === modelName && item.endpoint === endpoint
      );
      const legacyBaseURL = normalizeBaseURL(row?.base_url || '');
      const baseDraft = buildEmptyEndpointPolicyDraft(
        targetChannelId,
        modelName,
        endpoint
      );
      setPolicyDraft({
        ...baseDraft,
        template_key: '',
        original_template_key: (baseDraft.template_key || '').toString().trim(),
        endpoint_enabled: row?.enabled === true,
        endpoint_legacy_base_url: legacyBaseURL,
        endpoint_policy_rows: endpointPolicies,
        access_base_url: legacyBaseURL,
        endpoint_enable_block_reason: (row?.enable_block_reason || '')
          .toString()
          .trim(),
      });
      setSelectedPolicyTemplate('');
      setPolicyEditorOpen(true);
    },
    [channelEndpointPolicies, channelId]
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
      (item) => item.value === templateValue
    );
    if (!template) {
      return;
    }
    setPolicyDraft((prev) => {
      const existingPolicy =
        (Array.isArray(prev.endpoint_policy_rows)
          ? prev.endpoint_policy_rows
          : []
        ).find(
          (item) =>
            (item.template_key || '').toString().trim() === templateValue
        ) || null;
      const patch = existingPolicy
        ? {
            ...existingPolicy,
            capabilities: prettyJSONString(existingPolicy.capabilities),
            request_policy: prettyJSONString(existingPolicy.request_policy),
            response_policy: prettyJSONString(existingPolicy.response_policy),
            access_base_url:
              templateValue === ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL
                ? parseEndpointAccessPolicyBaseURL(existingPolicy.request_policy)
                : '',
          }
        : template.buildDraft(prev);
      return {
        ...prev,
        id: existingPolicy ? existingPolicy.id : '',
        ...patch,
        template_key: templateValue,
        original_template_key: existingPolicy
          ? (existingPolicy.template_key || '').toString().trim()
          : '',
      };
    });
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
    const templateKey = (policyDraft.template_key || '').toString().trim();
    let requestPolicy = (policyDraft.request_policy || '').toString().trim();
    let capabilities = (policyDraft.capabilities || '').toString().trim();
    let responsePolicy = (policyDraft.response_policy || '').toString().trim();
    if (templateKey === ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL) {
      const baseURL = normalizeBaseURL(policyDraft.access_base_url || '');
      if (baseURL === '') {
        showError(t('channel.edit.endpoint_policies.editor.base_url_required'));
        return;
      }
      requestPolicy = buildEndpointAccessPolicyJSON(baseURL);
      capabilities = '';
      responsePolicy = '';
    }
    setPolicyEditorSaving(true);
    try {
      const shouldSavePolicy =
        (policyDraft.id || '').toString().trim() !== '' ||
        [templateKey, capabilities, requestPolicy, responsePolicy, policyDraft.reason].some(
          (value) => (value || '').toString().trim() !== ''
        ) ||
        policyDraft.enabled === false;
      if (shouldSavePolicy && templateKey === '') {
        showError(t('channel.edit.endpoint_policies.editor.template_required'));
        return;
      }
      const endpointRes = await API.put(
        `/api/v1/admin/channel/${targetChannelId}/endpoints`,
        {
          model: modelName,
          endpoint,
          base_url:
            templateKey === ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL
              ? ''
              : normalizeBaseURL(policyDraft.endpoint_legacy_base_url || ''),
          enabled: policyDraft.endpoint_enabled === true,
        }
      );
      const endpointResult = endpointRes.data || {};
      if (!endpointResult.success) {
        showError(
          endpointResult.message ||
            t('channel.edit.endpoint_capabilities.update_failed')
        );
        return;
      }
      if (shouldSavePolicy) {
        const res = await API.put(
          `/api/v1/admin/channel/${targetChannelId}/policies`,
          {
            id: (policyDraft.id || '').toString().trim(),
            model: modelName,
            endpoint,
            enabled: !!policyDraft.enabled,
            template_key: templateKey,
            capabilities,
            request_policy: requestPolicy,
            response_policy: responsePolicy,
            reason: (policyDraft.reason || '').toString(),
            source: 'manual',
            last_verified_at: Number(policyDraft.last_verified_at || 0),
          }
        );
        const { success, message } = res.data || {};
        if (!success) {
          showError(
            message || t('channel.edit.endpoint_policies.update_failed')
          );
          return;
        }
      }
      const nextPolicies = await loadChannelEndpointPoliciesFromServer(
        targetChannelId
      );
      const nextEndpoints = await loadChannelEndpointsFromServer(
        targetChannelId
      );
      setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
      setChannelEndpointsError('');
      setChannelEndpointPolicies(
        normalizeChannelEndpointPolicyRows(nextPolicies)
      );
      setChannelEndpointPoliciesError('');
      showSuccess(t('channel.edit.endpoint_policies.update_success'));
      closeEndpointPolicyEditor();
    } catch (error) {
      showError(
        error?.message || t('channel.edit.endpoint_policies.update_failed')
      );
    } finally {
      setPolicyEditorSaving(false);
    }
  }, [
    closeEndpointPolicyEditor,
    loadChannelEndpointsFromServer,
    loadChannelEndpointPoliciesFromServer,
    policyDraft,
    policyEditorSaving,
    t,
  ]);

  const removeEndpointPolicy = useCallback(
    async (policyRow) => {
      if (endpointPolicyReadonly) {
        return;
      }
      const targetChannelId = (policyRow?.channel_id || channelId || '')
        .toString()
        .trim();
      const policyID = (policyRow?.id || '').toString().trim();
      if (targetChannelId === '' || policyID === '') {
        showError(t('channel.edit.endpoint_policies.invalid'));
        return;
      }
      setEndpointPolicyDeletingKey(policyID);
      try {
        await deleteChannelEndpointPolicy(targetChannelId, policyID);
        const nextPolicies = await loadChannelEndpointPoliciesFromServer(
          targetChannelId
        );
        const nextEndpoints = await loadChannelEndpointsFromServer(
          targetChannelId
        );
        setChannelEndpointPolicies(
          normalizeChannelEndpointPolicyRows(nextPolicies)
        );
        setChannelEndpointPoliciesError('');
        setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
        setChannelEndpointsError('');
        showSuccess(t('channel.edit.endpoint_policies.remove_success'));
      } catch (error) {
        showError(
          error?.message || t('channel.edit.endpoint_policies.remove_failed')
        );
      } finally {
        setEndpointPolicyDeletingKey('');
      }
    },
    [
      channelId,
      endpointPolicyReadonly,
      loadChannelEndpointsFromServer,
      loadChannelEndpointPoliciesFromServer,
      t,
    ]
  );

  const handleBatchUpdateEndpointCapabilities = useCallback(
    async (rows, enabled) => {
      if (!isDetailMode || endpointCapabilityReadonly || endpointBatchMutating) {
        return false;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const targetRows = Array.isArray(rows) ? rows : [];
      if (targetChannelId === '' || targetRows.length === 0) {
        showInfo(t('channel.edit.endpoint_capabilities.batch.select_required'));
        return false;
      }
      setEndpointBatchMutating(true);
      let successCount = 0;
      let failedCount = 0;
      let skippedCount = 0;
      try {
        for (const row of targetRows) {
          const modelName = (row?.model || '').toString().trim();
          const endpoint = (row?.endpoint || '').toString().trim();
          if (modelName === '' || endpoint === '') {
            continue;
          }
          // 存在启用阻断的端点无法开启，直接跳过。
          if (
            enabled &&
            (row?.enable_block_reason || '').toString().trim() !== '' &&
            row?.enabled !== true
          ) {
            skippedCount += 1;
            continue;
          }
          // 已是目标状态则省去一次请求，直接计入成功。
          if ((row?.enabled === true) === enabled) {
            successCount += 1;
            continue;
          }
          try {
            const res = await API.put(
              `/api/v1/admin/channel/${targetChannelId}/endpoints`,
              {
                model: modelName,
                endpoint,
                base_url: normalizeBaseURL(row?.base_url),
                enabled: !!enabled,
              }
            );
            const { success } = res.data || {};
            if (success) {
              successCount += 1;
            } else {
              failedCount += 1;
            }
          } catch (error) {
            failedCount += 1;
          }
        }
        const nextEndpoints = await loadChannelEndpointsFromServer(
          targetChannelId
        );
        setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
        setChannelEndpointsError('');
        const message = t(
          enabled
            ? 'channel.edit.endpoint_capabilities.batch.enable_done'
            : 'channel.edit.endpoint_capabilities.batch.disable_done',
          { success: successCount, failed: failedCount, skipped: skippedCount }
        );
        if (failedCount > 0) {
          showError(message);
        } else {
          showSuccess(message);
        }
        return failedCount === 0;
      } finally {
        setEndpointBatchMutating(false);
      }
    },
    [
      channelId,
      endpointBatchMutating,
      endpointCapabilityReadonly,
      isDetailMode,
      loadChannelEndpointsFromServer,
      t,
    ]
  );

  const handleBatchClearEndpointPolicies = useCallback(
    async (rows) => {
      if (endpointPolicyReadonly || endpointBatchMutating) {
        return false;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const targetRows = Array.isArray(rows) ? rows : [];
      if (targetChannelId === '') {
        return false;
      }
      const keySet = new Set(
        targetRows.map((row) =>
          buildChannelEndpointKey(row?.model, row?.endpoint)
        )
      );
      const policyIDs = channelEndpointPolicies
        .filter((policy) =>
          keySet.has(buildChannelEndpointKey(policy.model, policy.endpoint))
        )
        .map((policy) => (policy?.id || '').toString().trim())
        .filter((policyID) => policyID !== '');
      if (policyIDs.length === 0) {
        showInfo(
          t('channel.edit.endpoint_capabilities.batch.clear_policies_none')
        );
        return false;
      }
      setEndpointBatchMutating(true);
      let successCount = 0;
      let failedCount = 0;
      try {
        for (const policyID of policyIDs) {
          try {
            await deleteChannelEndpointPolicy(targetChannelId, policyID);
            successCount += 1;
          } catch (error) {
            failedCount += 1;
          }
        }
        const nextPolicies = await loadChannelEndpointPoliciesFromServer(
          targetChannelId
        );
        const nextEndpoints = await loadChannelEndpointsFromServer(
          targetChannelId
        );
        setChannelEndpointPolicies(
          normalizeChannelEndpointPolicyRows(nextPolicies)
        );
        setChannelEndpointPoliciesError('');
        setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
        setChannelEndpointsError('');
        const message = t(
          'channel.edit.endpoint_capabilities.batch.clear_policies_done',
          { success: successCount, failed: failedCount }
        );
        if (failedCount > 0) {
          showError(message);
        } else {
          showSuccess(message);
        }
        return failedCount === 0;
      } finally {
        setEndpointBatchMutating(false);
      }
    },
    [
      buildChannelEndpointKey,
      channelId,
      channelEndpointPolicies,
      endpointBatchMutating,
      endpointPolicyReadonly,
      loadChannelEndpointsFromServer,
      loadChannelEndpointPoliciesFromServer,
      t,
    ]
  );

  const handleBatchApplyEndpointPolicy = useCallback(
    async (rows, draft) => {
      if (endpointPolicyReadonly || endpointBatchMutating) {
        return false;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const targetRows = Array.isArray(rows) ? rows : [];
      if (targetChannelId === '' || targetRows.length === 0) {
        showInfo(t('channel.edit.endpoint_capabilities.batch.select_required'));
        return false;
      }
      const templateKey = (draft?.template_key || '').toString().trim();
      if (templateKey === '') {
        showError(t('channel.edit.endpoint_policies.editor.template_required'));
        return false;
      }
      let requestPolicy = (draft?.request_policy || '').toString().trim();
      let capabilities = (draft?.capabilities || '').toString().trim();
      let responsePolicy = (draft?.response_policy || '').toString().trim();
      let overrideBaseURL = '';
      if (templateKey === ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL) {
        overrideBaseURL = normalizeBaseURL(draft?.access_base_url || '');
        if (overrideBaseURL === '') {
          showError(
            t('channel.edit.endpoint_policies.editor.base_url_required')
          );
          return false;
        }
        requestPolicy = buildEndpointAccessPolicyJSON(overrideBaseURL);
        capabilities = '';
        responsePolicy = '';
      }
      const enabled = draft?.enabled === true;
      const reason = (draft?.reason || '').toString();
      setEndpointBatchMutating(true);
      let successCount = 0;
      let failedCount = 0;
      try {
        for (const row of targetRows) {
          const modelName = (row?.model || '').toString().trim();
          const endpoint = (row?.endpoint || '').toString().trim();
          if (modelName === '' || endpoint === '') {
            continue;
          }
          const endpointKey = buildChannelEndpointKey(modelName, endpoint);
          const existingPolicy =
            channelEndpointPolicies.find(
              (policy) =>
                buildChannelEndpointKey(policy.model, policy.endpoint) ===
                  endpointKey &&
                (policy.template_key || '').toString().trim() === templateKey
            ) || null;
          try {
            const endpointRes = await API.put(
              `/api/v1/admin/channel/${targetChannelId}/endpoints`,
              {
                model: modelName,
                endpoint,
                base_url:
                  templateKey === ENDPOINT_POLICY_TEMPLATE_OVERRIDE_BASE_URL
                    ? ''
                    : normalizeBaseURL(row?.base_url),
                enabled: row?.enabled === true,
              }
            );
            if (!(endpointRes.data || {}).success) {
              failedCount += 1;
              continue;
            }
            const res = await API.put(
              `/api/v1/admin/channel/${targetChannelId}/policies`,
              {
                id: existingPolicy
                  ? (existingPolicy.id || '').toString().trim()
                  : '',
                model: modelName,
                endpoint,
                enabled,
                template_key: templateKey,
                capabilities,
                request_policy: requestPolicy,
                response_policy: responsePolicy,
                reason,
                source: 'manual',
                last_verified_at: 0,
              }
            );
            if ((res.data || {}).success) {
              successCount += 1;
            } else {
              failedCount += 1;
            }
          } catch (error) {
            failedCount += 1;
          }
        }
        const nextPolicies = await loadChannelEndpointPoliciesFromServer(
          targetChannelId
        );
        const nextEndpoints = await loadChannelEndpointsFromServer(
          targetChannelId
        );
        setChannelEndpointPolicies(
          normalizeChannelEndpointPolicyRows(nextPolicies)
        );
        setChannelEndpointPoliciesError('');
        setChannelEndpoints(normalizeChannelEndpointRows(nextEndpoints));
        setChannelEndpointsError('');
        const message = t(
          'channel.edit.endpoint_capabilities.batch.apply_policy_done',
          { success: successCount, failed: failedCount }
        );
        if (failedCount > 0) {
          showError(message);
        } else {
          showSuccess(message);
        }
        return failedCount === 0;
      } finally {
        setEndpointBatchMutating(false);
      }
    },
    [
      buildChannelEndpointKey,
      channelId,
      channelEndpointPolicies,
      endpointBatchMutating,
      endpointPolicyReadonly,
      loadChannelEndpointsFromServer,
      loadChannelEndpointPoliciesFromServer,
      t,
    ]
  );

  const toggleModelSelection = useCallback(
    async (upstreamModel, checked) => {
      const nextConfigs = visibleChannelModels.map((row) =>
        row.upstream_model === upstreamModel &&
        (checked ? canSelectChannelModel(row) : row.selected === true)
          ? {
              ...row,
              selected: !!checked,
              disabled_reason: checked ? '' : row.disabled_reason,
              disabled_at: checked ? 0 : row.disabled_at,
              disabled_by: checked ? '' : row.disabled_by,
            }
          : row
      );
      if (isDetailMode) {
        if (
          detailModelsEditing &&
          detailEditingModelKey === (upstreamModel || '').toString().trim()
        ) {
          setInputs((prev) =>
            buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol)
          );
          return;
        }
        await persistDetailChannelModels(nextConfigs);
        return;
      }
      setInputs((prev) =>
        buildNextInputsWithChannelModels(prev, nextConfigs, prev.protocol)
      );
    },
    [
      canSelectChannelModel,
      detailEditingModelKey,
      detailModelsEditing,
      isDetailMode,
      persistDetailChannelModels,
      visibleChannelModels,
    ]
  );
  const handleDeleteDetailModel = useCallback(
    async (row) => {
      if (!isDetailMode || detailModelMutating || detailModelsEditing) {
        return;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const modelName = (row?.model || '').toString().trim();
      const upstreamModel = (row?.upstream_model || '').toString().trim();
      if (
        targetChannelId === '' ||
        (modelName === '' && upstreamModel === '')
      ) {
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
          }
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
          error?.message || t('channel.edit.model_selector.delete_failed')
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
    ]
  );

  const handleBatchSelectDetailModels = useCallback(
    async (rows) => {
      if (!isDetailMode || detailModelMutating || detailModelsEditing) {
        return false;
      }
      const targetRows = Array.isArray(rows)
        ? rows.filter((row) => canSelectChannelModel(row))
        : [];
      if (targetRows.length === 0) {
        showInfo(t('channel.edit.model_selector.batch_select_select_required'));
        return false;
      }
      const targetUpstreamModels = new Set(
        targetRows.map((row) => (row?.upstream_model || '').toString().trim())
      );
      const nextConfigs = visibleChannelModels.map((row) =>
        targetUpstreamModels.has((row?.upstream_model || '').toString().trim())
          ? {
              ...row,
              selected: true,
              disabled_reason: '',
              disabled_at: 0,
              disabled_by: '',
            }
          : row
      );
      const ok = await persistDetailChannelModels(nextConfigs);
      if (ok) {
        showSuccess(
          t('channel.edit.model_selector.batch_select_done', {
            count: targetRows.length,
          })
        );
      }
      return ok;
    },
    [
      canSelectChannelModel,
      detailModelMutating,
      detailModelsEditing,
      isDetailMode,
      persistDetailChannelModels,
      t,
      visibleChannelModels,
    ]
  );

  const handleBatchDeleteDetailModels = useCallback(
    async (rows) => {
      if (!isDetailMode || detailModelMutating || detailModelsEditing) {
        return false;
      }
      const targetChannelId = (channelId || '').toString().trim();
      const targetRows = Array.isArray(rows)
        ? rows.filter((row) => {
            const modelName = (row?.model || '').toString().trim();
            const upstreamModel = (row?.upstream_model || '').toString().trim();
            return modelName !== '' || upstreamModel !== '';
          })
        : [];
      if (targetChannelId === '' || targetRows.length === 0) {
        showInfo(t('channel.edit.model_selector.batch_delete_select_required'));
        return false;
      }
      setDetailModelMutating(true);
      let successCount = 0;
      let failedCount = 0;
      try {
        for (const row of targetRows) {
          const modelName = (row?.model || '').toString().trim();
          const upstreamModel = (row?.upstream_model || '').toString().trim();
          try {
            const res = await API.delete(
              `/api/v1/admin/channel/${targetChannelId}/models`,
              {
                params: {
                  model: modelName,
                  upstream_model: upstreamModel,
                },
              }
            );
            const { success } = res.data || {};
            if (success) {
              successCount += 1;
            } else {
              failedCount += 1;
            }
          } catch (error) {
            failedCount += 1;
          }
        }
        if (successCount > 0) {
          await refreshChannelRuntimeState(targetChannelId);
          setDetailEditingModelKey('');
          setDetailEditingModelSnapshot(null);
        }
        const message = t('channel.edit.model_selector.batch_delete_done', {
          success: successCount,
          failed: failedCount,
        });
        if (failedCount > 0) {
          showError(message);
        } else {
          showSuccess(message);
        }
        return successCount > 0;
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
    ]
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
                prev.protocol
              );
              const nextEndpoints = normalizeChannelModelEndpoints(
                row.type,
                row.endpoints,
                nextEndpoint,
                prev.protocol
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
                prev.protocol
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
          prev.protocol
        )
      );
    },
    [
      detailEditingModelKey,
      detailModelsEditing,
      isDetailMode,
      visibleChannelModels,
    ]
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
      loadChannelById(channelId, false).then();
      return;
    }
    setChannelKeySet(false);
    setConfig(CHANNEL_DEFAULT_CONFIG);
    setLoading(false);
  }, [channelId, hasChannelID, loadChannelById]);

  useEffect(() => {
    if (!isDetailMode || !channelId) {
      setChannelEndpoints([]);
      setChannelEndpointsError('');
      setChannelEndpointsLoading(false);
      return undefined;
    }
    if (!showDetailEndpointsTab && !showDetailTestsTab) {
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
          error?.message || t('channel.edit.endpoint_capabilities.load_failed')
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
  }, [
    channelId,
    isDetailMode,
    loadChannelEndpointsFromServer,
    showDetailEndpointsTab,
    showDetailTestsTab,
    t,
  ]);

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
          error?.message || t('channel.edit.endpoint_policies.load_failed')
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
      isActiveAsyncTaskStatus(item?.status)
    );
    if (!hasActiveTasks) {
      return undefined;
    }
    const timer = window.setInterval(async () => {
      try {
        const nextTasks = await loadChannelTasksFromServer(targetChannelId);
        const stillActive = nextTasks.some((item) =>
          isActiveAsyncTaskStatus(item?.status)
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
          const runtimeState = await refreshChannelRuntimeState(targetChannelId);
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
              const afterModels = Array.isArray(runtimeState?.channelModels)
                ? runtimeState.channelModels
                : [];
              const totalCount = Array.isArray(runtimeState?.channelModels)
                ? afterModels.length
                : pendingRefreshBeforeCountRef.current;
              const beforeModelSet = new Set(
                pendingRefreshBeforeModelsRef.current,
              );
              let addedCount = 0;
              let staleCount = 0;
              afterModels.forEach((row) => {
                const modelName = (row?.model || '').toString().trim();
                if (modelName !== '' && !beforeModelSet.has(modelName)) {
                  addedCount += 1;
                }
                if ((row?.sync_status || '').toString().trim() === 'not_returned') {
                  staleCount += 1;
                }
              });
              showSuccess(
                t('channel.edit.messages.sync_models_result', {
                  added: addedCount,
                  stale: staleCount,
                  total: totalCount,
                }),
              );
            } else {
              setVerifiedModelSignature('');
              setModelsSyncError(
                completedRefreshTask?.error_message ||
                  t('channel.edit.messages.fetch_models_failed')
              );
            }
            pendingRefreshSignatureRef.current = '';
            pendingRefreshBeforeCountRef.current = 0;
            pendingRefreshBeforeModelsRef.current = [];
          }
          const billingRefreshTaskId = pendingBillingRefreshTaskIdRef.current;
          if (billingRefreshTaskId !== '') {
            let completedBillingRefreshTask = null;
            try {
              completedBillingRefreshTask = await fetchTaskById(
                billingRefreshTaskId
              );
            } catch {
              completedBillingRefreshTask = null;
            }
            pendingBillingRefreshTaskIdRef.current = '';
            setChannelBillingSubmitting(false);
            if (
              completedBillingRefreshTask &&
              normalizeAsyncTaskStatus(completedBillingRefreshTask.status) ===
                'succeeded'
            ) {
              showSuccess(
                t('channel.messages.billing_update_success', {
                  name: (inputs.name || targetChannelId).toString().trim(),
                })
              );
            } else {
              showError(
                completedBillingRefreshTask?.error_message ||
                  t('channel.messages.billing_update_failed', {
                    name: (inputs.name || targetChannelId).toString().trim(),
                  })
              );
            }
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
    inputs.name,
    loadChannelTasksFromServer,
    loadChannelTestsFromServer,
    refreshChannelRuntimeState,
    t,
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
    if (!showStepTwo && !showDetailTestsTab && !showDetailPublishTab) {
      return;
    }
    loadProviderIndex({ silent: true }).then();
  }, [loadProviderIndex, showDetailPublishTab, showDetailTestsTab, showStepTwo]);

  useEffect(() => {
    if (detailModelPage <= detailModelTotalPages) {
      return;
    }
    setDetailModelPage(detailModelTotalPages);
  }, [detailModelPage, detailModelTotalPages]);

  useEffect(() => {
    setDetailModelPage(1);
  }, [
    detailModelFilter,
    detailProviderFilter,
    detailUpstreamStatusFilter,
    modelSearchKeyword,
  ]);

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
    const protocolConfigError = validateProtocolSpecificChannelConfig(
      inputs,
      config,
      t
    );
    if (protocolConfigError !== '') {
      showError(protocolConfigError);
      return;
    }
    let localInputs = buildChannelPayload();
    const res = await API.post(`/api/v1/admin/channel/`, localInputs);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(t('channel.edit.messages.create_success'));
      const targetChannelID = (data?.id || localInputs.id || '')
        .toString()
        .trim();
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
        </AppFormRow>
        <div className='router-form-hint router-form-hint-tight'>
          {t('channel.edit.address_routing_hint')}
        </div>
      </>
    );
  };

  const renderProtocolSpecificFields = () => {
    const isVolcengineRealtime = isEffectiveVolcengineRealtimeProtocol(inputs);
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
                  {t('channel.edit.azure_notice_prefix')}
                  <strong>{t('channel.edit.azure_notice_strong')}</strong>
                  {t('channel.edit.azure_notice_body')}
                  <a
                    target='_blank'
                    rel='noreferrer'
                    href='https://github.com/yeying-community/router/issues/133?notification_referrer_id=NT_kwDOAmJSYrM2NjIwMzI3NDgyOjM5OTk4MDUw#issuecomment-1571602271'
                  >
                    {t('channel.edit.azure_notice_link')}
                  </a>
                  {t('channel.edit.azure_notice_suffix')}
                </span>
              }
            />
            <AppFormRow>
              <AppField label={t('channel.edit.azure_api_version')}>
                <AppInput
                  className='router-section-input'
                  name='other'
                  placeholder={t('channel.edit.azure_api_version_placeholder')}
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
        {inputs.protocol === 'coze' && (
          <AppAlert
            type='info'
            showIcon
            className='router-section-message'
            title={t('channel.edit.coze_notice')}
          />
        )}
        {inputs.protocol === VOLCENGINE_STANDARD_PROTOCOL &&
          !isVolcengineRealtime && (
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
        {isVolcengineRealtime && (
          <>
            <AppAlert
              type='info'
              showIcon
              className='router-section-message'
              title={
                <span>
                  {t('channel.edit.volc_realtime_notice')}
                  <code>volc.speech.dialog</code>
                  {t('channel.edit.volc_realtime_notice_2')}
                </span>
              }
            />
            <AppFormRow>
              <AppField label='App ID' required>
                <AppInput
                  className='router-section-input'
                  name='app_id'
                  required
                  placeholder={t('channel.edit.volc_app_id_placeholder')}
                  onChange={handleConfigChange}
                  value={config.app_id}
                  autoComplete='off'
                  {...inputReadonlyProps}
                />
              </AppField>
              <AppField label='Resource ID'>
                <AppInput
                  className='router-section-input'
                  name='resource_id'
                  placeholder={t('channel.edit.volc_resource_id_placeholder')}
                  onChange={handleConfigChange}
                  value={config.resource_id}
                  autoComplete='off'
                  {...inputReadonlyProps}
                />
              </AppField>
            </AppFormRow>
          </>
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
                placeholder={t('channel.edit.cloudflare_account_id_placeholder')}
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
      <ChannelAppendProviderModal
        t={t}
        open={appendProviderModalOpen}
        onClose={closeAppendProviderModal}
        appendingProviderModel={appendingProviderModel}
        filterProviderOptionsByQuery={filterProviderOptionsByQuery}
        providerOptions={providerOptions}
        appendProviderForm={appendProviderForm}
        setAppendProviderForm={setAppendProviderForm}
        providerModelTagOptions={PROVIDER_MODEL_TAG_OPTIONS}
        handleAppendModelToProvider={handleAppendModelToProvider}
      />
      {isDetailMode ? (
        <AppFilterHeader
          breadcrumbs={[
            { key: 'admin', label: t('header.admin_workspace') },
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
            <AppButton className='router-page-button' onClick={handleCancel}>
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
                billingProfile={channelBillingProfile}
                billingAdapters={channelBillingAdapters}
                billingSummary={channelBillingSummary}
                billingLoading={channelBillingLoading}
                billingError={channelBillingError}
                detailBillingEditing={detailBillingEditing}
                detailBillingDraft={detailBillingDraft}
                billingSubmitting={channelBillingSubmitting}
                detailBillingEditLocked={detailBillingEditLocked}
                setDetailBillingEditing={setDetailBillingEditing}
                onUpdateBillingProfileDraft={updateBillingProfileDraft}
                onCancelBillingProfileEdit={cancelDetailBillingEdit}
                onSaveBillingProfile={saveDetailBillingProfile}
                onRefreshBilling={refreshChannelBillingNow}
                channelID={channelId}
                onNavigateTab={goToDetailTab}
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
                    detailUpstreamStatusFilter={detailUpstreamStatusFilter}
                    setDetailUpstreamStatusFilter={
                      setDetailUpstreamStatusFilter
                    }
                    detailProviderFilter={detailProviderFilter}
                    setDetailProviderFilter={setDetailProviderFilter}
                    detailProviderFilterOptions={detailProviderFilterOptions}
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
                    detailModelsEditLocked={detailModelsEditLocked}
                    providerDataLoading={providerDataLoading}
                    toggleModelSelection={toggleModelSelection}
                    canSelectChannelModel={canSelectChannelModel}
                    normalizeChannelModelType={normalizeChannelModelType}
                    startDetailModelEdit={startDetailModelEdit}
                    handleDeleteDetailModel={handleDeleteDetailModel}
                    handleBatchSelectDetailModels={handleBatchSelectDetailModels}
                    handleBatchDeleteDetailModels={handleBatchDeleteDetailModels}
                    detailModelPage={detailModelPage}
                    setDetailModelPage={setDetailModelPage}
                    detailModelTotal={searchedChannelModels.length}
                    detailModelPageSize={detailModelPageSize}
                    setDetailModelPageSize={setDetailModelPageSize}
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
                    channelEndpointPoliciesError={channelEndpointPoliciesError}
                    endpointPolicyReadonly={endpointPolicyReadonly}
                    endpointPolicyDeletingKey={endpointPolicyDeletingKey}
                    removeEndpointPolicy={removeEndpointPolicy}
                    openEndpointPolicyEditor={openEndpointPolicyEditor}
                    endpointBatchMutating={endpointBatchMutating}
                    handleBatchUpdateEndpointCapabilities={
                      handleBatchUpdateEndpointCapabilities
                    }
                    handleBatchClearEndpointPolicies={
                      handleBatchClearEndpointPolicies
                    }
                    handleBatchApplyEndpointPolicy={
                      handleBatchApplyEndpointPolicy
                    }
                    endpointPolicyTemplates={ENDPOINT_POLICY_TEMPLATES}
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
                updateAllModelTestStreams={updateAllModelTestStreams}
                resolvePreferredProviderForModel={
                  resolvePreferredProviderForModel
                }
                normalizeChannelModelType={normalizeChannelModelType}
                audioTestLanguage={audioTestLanguage}
                setAudioTestLanguage={setAudioTestLanguage}
                responsesTestMode={responsesTestMode}
                setResponsesTestMode={setResponsesTestMode}
                imageEditTestURL={imageEditTestURL}
                setImageEditTestURL={setImageEditTestURL}
                imageEditTestFileName={imageEditTestFileName}
                imageEditTestData={imageEditTestData}
                setImageEditTestData={setImageEditTestData}
                setImageEditTestFileName={setImageEditTestFileName}
                handleImageEditTestFileChange={handleImageEditTestFileChange}
              />
            )}
            {showDetailPublishTab && inputs.protocol !== 'proxy' && (
              <ChannelDetailPublishTab
                t={t}
                channelModels={visibleChannelModels}
                getComplexPricingDetailsForModel={
                  getComplexPricingDetailsForModel
                }
                getEffectivePriceForModel={getEffectivePriceForModel}
                getEffectivePriceMetaForModel={getEffectivePriceMetaForModel}
                openComplexPricingModal={openComplexPricingModal}
                normalizeChannelModelType={normalizeChannelModelType}
                onUpdatePublishedModelName={updateChannelModelPublishedName}
                onUpdatePublish={updateChannelModelPublish}
                onNavigateTab={goToDetailTab}
                publishMutatingModel={publishMutatingModel}
                publishReadonly={detailPublishReadonly}
              />
            )}
            {showDetailProcurementTab && (
              <ChannelProcurementView
                t={t}
                billingLoading={channelBillingLoading}
                billingSnapshots={channelBillingSnapshots}
                procurementBatches={channelProcurementBatches}
                billingReadonly={false}
                billingSubmitting={channelBillingSubmitting}
                billingError={channelBillingError}
                onRefreshBilling={() =>
                  refreshChannelProcurementState(channelId)
                }
                onManualSnapshotUpdate={updateChannelManualBillingSnapshot}
                onManualSnapshotDelete={deleteChannelManualBillingSnapshot}
                onProcurementBatchCostUpdate={updateChannelProcurementBatchCost}
                onProcurementBatchStatusUpdate={
                  updateChannelProcurementBatchStatus
                }
                onProcurementBatchConsumptionsLoad={
                  loadChannelProcurementBatchConsumptions
                }
                timestamp2string={timestamp2string}
                channelID={channelId}
                showProcurementBatches
                costTrackingMode={
                  channelBillingProfile?.cost_tracking_mode || 'untracked'
                }
                onCostTrackingModeChange={saveChannelCostTrackingMode}
                costTrackingSubmitting={channelBillingSubmitting}
                costMissingModelCount={
                  channelBillingProfile?.cost_missing_model_count || 0
                }
                costQuotes={channelCostQuotes}
                onSyncCostQuotes={async () => {
                  try {
                    const result = await syncChannelCostQuotes(channelId);
                    showSuccess(
                      t('channel.edit.billing.cost_reconcile_sync_done', {
                        cached: result.cached,
                        skipped: result.skipped,
                      })
                    );
                    await refreshChannelProcurementState(channelId);
                  } catch (error) {
                    showError(
                      error?.message ||
                        t('channel.edit.billing.cost_reconcile_sync_failed')
                    );
                  }
                }}
              />
            )}
          </div>
        </AppSpin>
      </div>
    </div>
  );
};

export default ChannelForm;
