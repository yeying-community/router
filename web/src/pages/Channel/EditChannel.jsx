import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Button, Card, Checkbox, Form, Message} from 'semantic-ui-react';
import {useLocation, useNavigate, useParams} from 'react-router-dom';
import {API, showError, showInfo, showSuccess, verifyJSON,} from '../../helpers';
import {getChannelOptions, loadChannelOptions} from '../../helpers/helper';

const MODEL_MAPPING_EXAMPLE = {
  'gpt-3.5-turbo-0301': 'gpt-3.5-turbo',
  'gpt-4-0314': 'gpt-4',
  'gpt-4-32k-0314': 'gpt-4-32k',
};

const normalizeModelId = (model) => {
  if (typeof model === 'string') return model;
  if (model && typeof model === 'object') {
    if (typeof model.id === 'string') return model.id;
    if (typeof model.name === 'string') return model.name;
    if (typeof model.model === 'string') return model.model;
  }
  return null;
};

const flattenModels = (payload, meta) => {
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(meta)) {
    const set = new Set();
    meta.forEach((entry) => {
      const models = entry?.models;
      if (Array.isArray(models)) {
        models.forEach((model) => set.add(model));
      }
    });
    return Array.from(set);
  }
  if (payload && typeof payload === 'object') {
    const set = new Set();
    Object.values(payload).forEach((models) => {
      if (Array.isArray(models)) {
        models.forEach((model) => set.add(model));
      }
    });
    return Array.from(set);
  }
  return [];
};

const buildModelOptions = (models) => {
  const seen = new Set();
  const options = [];
  const ids = [];
  models.forEach((model) => {
    const id = normalizeModelId(model);
    if (!id || seen.has(id)) return;
    seen.add(id);
    options.push({
      key: id,
      text: id,
      value: id,
    });
    ids.push(id);
  });
  return { options, ids };
};

const OPENAI_COMPATIBLE_TYPES = new Set([50, 51]);

const isOpenAICompatibleType = (type) => OPENAI_COMPATIBLE_TYPES.has(type);

const normalizeBaseURL = (baseURL) =>
  (baseURL || '').trim().replace(/\/+$/, '');

const buildChannelConnectionSignature = ({ type, key, baseURL }) =>
  `${type}|${normalizeBaseURL(baseURL)}|${(key || '').trim()}`;

function type2secretPrompt(type, t) {
  switch (type) {
    case 15:
      return t('channel.edit.key_prompts.zhipu');
    case 18:
      return t('channel.edit.key_prompts.spark');
    case 22:
      return t('channel.edit.key_prompts.fastgpt');
    case 23:
      return t('channel.edit.key_prompts.tencent');
    default:
      return t('channel.edit.key_prompts.default');
  }
}

const EditChannel = () => {
  const { t } = useTranslation();
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const channelId = params.id;
  const isEdit = channelId !== undefined;
  const copyFromId = useMemo(() => {
    if (isEdit) return 0;
    const query = new URLSearchParams(location.search);
    const id = Number(query.get('copy_from') || 0);
    return Number.isInteger(id) && id > 0 ? id : 0;
  }, [isEdit, location.search]);
  const [loading, setLoading] = useState(isEdit || copyFromId > 0);
  const handleCancel = () => {
    navigate('/channel');
  };

  const originInputs = {
    name: '',
    type: 1,
    key: '',
    base_url: '',
    other: '',
    model_mapping: '',
    model_ratio: '',
    completion_ratio: '',
    system_prompt: '',
    models: [],
    groups: [],
  };
  const [inputs, setInputs] = useState(originInputs);
  const [originModelOptions, setOriginModelOptions] = useState([]);
  const [modelOptions, setModelOptions] = useState([]);
  const [groupOptions, setGroupOptions] = useState([]);
  const [channelTypeOptions, setChannelTypeOptions] = useState(() =>
    getChannelOptions()
  );
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [modelsSyncError, setModelsSyncError] = useState('');
  const [modelsLastSyncedAt, setModelsLastSyncedAt] = useState(0);
  const [verifiedModelSignature, setVerifiedModelSignature] = useState('');
  const [config, setConfig] = useState({
    region: '',
    sk: '',
    ak: '',
    user_id: '',
    vertex_ai_project_id: '',
    vertex_ai_adc: '',
    user_agent: '',
  });
  const fetchingModelsRef = useRef(false);
  const isOpenAICompatibleCreate = !isEdit && isOpenAICompatibleType(inputs.type);
  const hasModelPreviewCredentials =
    (inputs.key || '').trim() !== '' &&
    (normalizeBaseURL(inputs.base_url) !== '' || inputs.type === 51);
  const currentModelSignature = useMemo(
    () =>
      buildChannelConnectionSignature({
        type: inputs.type,
        key: inputs.key,
        baseURL: inputs.base_url,
      }),
    [inputs.base_url, inputs.key, inputs.type]
  );
  const requiresConnectionVerification = isOpenAICompatibleCreate && inputs.type !== 43;
  const showCreateModelFlowGuide = !isEdit && inputs.type !== 43;
  const isCurrentSignatureVerified =
    requiresConnectionVerification &&
    verifiedModelSignature !== '' &&
    currentModelSignature === verifiedModelSignature;
  const visibleModelOptions =
    requiresConnectionVerification && !isCurrentSignatureVerified
      ? []
      : modelOptions;
  const modelSyncStatusText = useMemo(() => {
    if (!requiresConnectionVerification) {
      return null;
    }
    if (fetchModelsLoading) {
      return t('channel.edit.model_selector.verify_in_progress');
    }
    if (!hasModelPreviewCredentials) {
      return t('channel.edit.model_selector.verify_prerequisite');
    }
    if (isCurrentSignatureVerified) {
      return t('channel.edit.model_selector.verify_ready');
    }
    if (
      verifiedModelSignature !== '' &&
      verifiedModelSignature !== currentModelSignature
    ) {
      return t('channel.edit.model_selector.verify_stale');
    }
    return t('channel.edit.model_selector.verify_required');
  }, [
    currentModelSignature,
    fetchModelsLoading,
    hasModelPreviewCredentials,
    isCurrentSignatureVerified,
    requiresConnectionVerification,
    t,
    verifiedModelSignature,
  ]);
  const modelSyncStatusColor =
    isCurrentSignatureVerified ? '#1f8f4b' : 'rgba(0, 0, 0, 0.6)';
  const fetchModelsButtonText = requiresConnectionVerification
    ? t('channel.edit.buttons.verify_and_fetch_models')
    : t('channel.edit.buttons.fetch_models');
  const flowStepsText = requiresConnectionVerification
    ? t('channel.edit.model_selector.flow_steps')
    : t('channel.edit.model_selector.flow_steps_no_verify');
  const protocolVerifySupportText = requiresConnectionVerification
    ? t('channel.edit.model_selector.protocol_support_yes')
    : t('channel.edit.model_selector.protocol_support_no');
  const unifiedModelStatusText = useMemo(() => {
    if (requiresConnectionVerification) {
      return modelSyncStatusText;
    }
    if (showCreateModelFlowGuide) {
      return t('channel.edit.model_selector.manual_mode_hint');
    }
    return null;
  }, [modelSyncStatusText, requiresConnectionVerification, showCreateModelFlowGuide, t]);
  const unifiedModelStatusColor = requiresConnectionVerification
    ? modelSyncStatusColor
    : 'rgba(0, 0, 0, 0.6)';

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const handleConfigChange = (e, { name, value }) => {
    setConfig((inputs) => ({ ...inputs, [name]: value }));
  };

  const loadChannelById = useCallback(async (targetId, forCopy = false) => {
    let res = await API.get(`/api/v1/admin/channel/${targetId}`);
    const { success, message, data } = res.data;
    if (success) {
      if (data.models === '') {
        data.models = [];
      } else {
        data.models = data.models.split(',');
      }
      if (data.group === '') {
        data.groups = [];
      } else {
        data.groups = data.group.split(',');
      }
      if (data.model_mapping !== '') {
        data.model_mapping = JSON.stringify(
          JSON.parse(data.model_mapping),
          null,
          2
        );
      }
      if (data.model_ratio) {
        data.model_ratio = JSON.stringify(JSON.parse(data.model_ratio), null, 2);
      } else {
        data.model_ratio = '';
      }
      if (data.completion_ratio) {
        data.completion_ratio = JSON.stringify(
          JSON.parse(data.completion_ratio),
          null,
          2
        );
      } else {
        data.completion_ratio = '';
      }
      if (forCopy) {
        setInputs({
          name: data.name || '',
          type: data.type || 1,
          key: data.key || '',
          base_url: data.base_url || '',
          other: data.other || '',
          model_mapping: data.model_mapping || '',
          model_ratio: data.model_ratio || '',
          completion_ratio: data.completion_ratio || '',
          system_prompt: data.system_prompt || '',
          models: data.models || [],
          groups: data.groups && data.groups.length > 0 ? data.groups : [],
        });
      } else {
        setInputs(data);
      }
      if (data.config !== '') {
        const parsedConfig = JSON.parse(data.config);
        delete parsedConfig.use_responses;
        setConfig((prev) => ({ ...prev, ...parsedConfig }));
      }
    } else {
      showError(message);
    }
    setLoading(false);
  }, []);

  const applyModelCandidates = useCallback((models, selectAll = false) => {
    const { options, ids } = buildModelOptions(models);
    setOriginModelOptions(options);
    setInputs((prev) => {
      const selected = selectAll
        ? ids
        : prev.models.filter((model) => ids.includes(model));
      return { ...prev, models: selected };
    });
    return ids;
  }, []);

  const fetchModels = useCallback(
    async (silent = false) => {
      try {
        const res = await API.get(`/api/v1/public/channel/models`);
        const payload = res?.data?.data;
        const meta = res?.data?.meta;
        const flattenedModels = flattenModels(payload, meta);
        applyModelCandidates(flattenedModels, false);
      } catch (error) {
        if (!silent) {
          showError(error?.message || error);
        }
      }
    },
    [applyModelCandidates]
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
        const key = (inputs.key || '').trim();
        const useChannelPreviewAPI =
          isOpenAICompatibleType(inputs.type) &&
          key !== '' &&
          (normalizedBaseURL !== '' || inputs.type === 51);
        const requestSignature = buildChannelConnectionSignature({
          type: inputs.type,
          key,
          baseURL: normalizedBaseURL,
        });
        if (
          useChannelPreviewAPI
        ) {
          const res = await API.post(`/api/v1/admin/channel/preview/models`, {
            type: inputs.type,
            key,
            base_url: normalizedBaseURL,
            config,
          });
          const { success, message, data } = res.data || {};
          if (!success) {
            const errorMessage = message || t('channel.edit.messages.fetch_models_failed');
            setModelsSyncError(errorMessage);
            setVerifiedModelSignature('');
            if (!silent) {
              showError(errorMessage);
            }
            return false;
          }
          models = Array.isArray(data) ? data.filter((model) => model) : [];
        } else {
          const res = await API.get(`/api/v1/public/channel/models`);
          const payload = res?.data?.data;
          const meta = res?.data?.meta;
          models = flattenModels(payload, meta);
        }

        const ids = applyModelCandidates(models, selectAll);
        if (ids.length === 0) {
          const message = t('channel.edit.messages.models_empty');
          setModelsSyncError(message);
          if (useChannelPreviewAPI) {
            setVerifiedModelSignature('');
          }
          if (!silent) {
            showInfo(message);
          }
          return false;
        }

        setModelsSyncError('');
        setModelsLastSyncedAt(Date.now());
        if (useChannelPreviewAPI) {
          setVerifiedModelSignature(requestSignature);
        }
        if (!silent) {
          showSuccess(t('channel.messages.operation_success'));
        }
        return true;
      } catch (error) {
        const errorMessage = error?.message || t('channel.edit.messages.fetch_models_failed');
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
      config,
      inputs.base_url,
      inputs.key,
      inputs.type,
      t,
    ]
  );

  const fetchGroups = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/admin/group/`);
      setGroupOptions(
        res.data.data.map((group) => ({
          key: group,
          text: group,
          value: group,
        }))
      );
    } catch (error) {
      showError(error.message);
    }
  }, []);

  const fetchChannelTypes = useCallback(async () => {
    const options = await loadChannelOptions();
    if (Array.isArray(options) && options.length > 0) {
      setChannelTypeOptions(options);
    }
  }, []);

  useEffect(() => {
    let localModelOptions = [...originModelOptions];
    inputs.models.forEach((model) => {
      if (!localModelOptions.find((option) => option.key === model)) {
        localModelOptions.push({
          key: model,
          text: model,
          value: model,
        });
      }
    });
    setModelOptions(localModelOptions);
  }, [originModelOptions, inputs.models]);

  const toggleModelSelection = useCallback((modelId, checked) => {
    setInputs((prev) => {
      const set = new Set(prev.models || []);
      if (checked) {
        set.add(modelId);
      } else {
        set.delete(modelId);
      }
      return { ...prev, models: Array.from(set) };
    });
  }, []);

  const selectAllModels = useCallback(() => {
    const allModelIds = modelOptions.map((option) => option.value);
    setInputs((prev) => ({ ...prev, models: allModelIds }));
  }, [modelOptions]);

  const clearSelectedModels = useCallback(() => {
    setInputs((prev) => ({ ...prev, models: [] }));
  }, []);

  useEffect(() => {
    if (isEdit) {
      setLoading(true);
      loadChannelById(channelId).then();
      return;
    }
    if (copyFromId > 0) {
      setLoading(true);
      loadChannelById(copyFromId, true).then();
      return;
    }
    setLoading(false);
  }, [channelId, copyFromId, isEdit, loadChannelById]);

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
    setOriginModelOptions([]);
    setInputs((prev) => ({ ...prev, models: [] }));
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
    if (loading) {
      return;
    }
    if (isEdit) {
      return;
    }
    if (inputs.type === 43) {
      return;
    }
    if (isOpenAICompatibleCreate && hasModelPreviewCredentials) {
      const timer = setTimeout(() => {
        handleFetchModels({ silent: true, selectAll: true }).then();
      }, 700);
      return () => clearTimeout(timer);
    }
    if (isOpenAICompatibleCreate) {
      return;
    }
    const timer = setTimeout(() => {
      fetchModels(true).then();
    }, 700);
    return () => clearTimeout(timer);
  }, [
    fetchModels,
    hasModelPreviewCredentials,
    handleFetchModels,
    inputs.base_url,
    inputs.key,
    inputs.type,
    isEdit,
    isOpenAICompatibleCreate,
    loading,
  ]);

  useEffect(() => {
    if (isEdit || copyFromId > 0) {
      fetchModels(true).then();
    }
    fetchGroups().then();
    fetchChannelTypes().then();
  }, [copyFromId, fetchModels, fetchGroups, fetchChannelTypes, isEdit]);

  const submit = async () => {
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
    if (!isEdit && (inputs.name.trim() === '' || effectiveKey.trim() === '')) {
      showInfo(t('channel.edit.messages.name_required'));
      return;
    }
    if (inputs.groups.length === 0) {
      showInfo(t('channel.edit.messages.groups_required'));
      return;
    }
    if (requiresConnectionVerification) {
      if (!hasModelPreviewCredentials) {
        showInfo(t('channel.edit.model_selector.verify_prerequisite'));
        return;
      }
      if (!isCurrentSignatureVerified) {
        showInfo(t('channel.edit.model_selector.verify_required'));
        return;
      }
    }
    if (inputs.type !== 43 && inputs.models.length === 0) {
      showInfo(t('channel.edit.messages.models_required'));
      return;
    }
    if (inputs.model_mapping !== '' && !verifyJSON(inputs.model_mapping)) {
      showInfo(t('channel.edit.messages.model_mapping_invalid'));
      return;
    }
    if (inputs.model_ratio !== '' && !verifyJSON(inputs.model_ratio)) {
      showInfo('模型倍率必须是合法的 JSON 格式！');
      return;
    }
    if (inputs.completion_ratio !== '' && !verifyJSON(inputs.completion_ratio)) {
      showInfo('补全倍率必须是合法的 JSON 格式！');
      return;
    }
    let localInputs = { ...inputs, key: effectiveKey };
    if (localInputs.key === 'undefined|undefined|undefined') {
      localInputs.key = ''; // prevent potential bug
    }
    if (localInputs.base_url && localInputs.base_url.endsWith('/')) {
      localInputs.base_url = localInputs.base_url.slice(
        0,
        localInputs.base_url.length - 1
      );
    }
    if (localInputs.type === 3 && localInputs.other === '') {
      localInputs.other = '2024-03-01-preview';
    }
    let res;
    localInputs.models = localInputs.models.join(',');
    localInputs.group = localInputs.groups.join(',');
    const submitConfig = { ...config };
    delete submitConfig.use_responses;
    localInputs.config = JSON.stringify(submitConfig);
    if (isEdit) {
      res = await API.put(`/api/v1/admin/channel/`, {
        ...localInputs,
        id: parseInt(channelId),
      });
    } else {
      res = await API.post(`/api/v1/admin/channel/`, localInputs);
    }
    const { success, message } = res.data;
    if (success) {
      if (isEdit) {
        showSuccess(t('channel.edit.messages.update_success'));
      } else {
        showSuccess(t('channel.edit.messages.create_success'));
        setInputs(originInputs);
      }
      navigate('/channel');
    } else {
      showError(message);
    }
  };

  const isRealtimeModelFetchAvailable =
    isOpenAICompatibleType(inputs.type) && hasModelPreviewCredentials;

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header'>
            {isEdit
              ? t('channel.edit.title_edit')
              : t('channel.edit.title_create')}
          </Card.Header>
          <Form loading={loading} autoComplete='new-password'>
            <Form.Field>
              <Form.Input
                label={t('channel.edit.name')}
                name='name'
                placeholder={t('channel.edit.name_placeholder')}
                onChange={handleInputChange}
                value={inputs.name}
                required
              />
            </Form.Field>
            <Form.Field>
              <Form.Select
                label={t('channel.edit.type')}
                name='type'
                required
                search
                options={channelTypeOptions}
                value={inputs.type}
                onChange={handleInputChange}
              />
              {!isEdit && (
                <div style={{ color: 'rgba(0, 0, 0, 0.6)', marginTop: '4px' }}>
                  {protocolVerifySupportText}
                </div>
              )}
            </Form.Field>
            {inputs.type === 3 && (
              <Form.Field>
                <Form.Input
                  label='AZURE_OPENAI_ENDPOINT'
                  name='base_url'
                  placeholder='请输入 AZURE_OPENAI_ENDPOINT，例如：https://docs-test-001.openai.azure.com'
                  onChange={handleInputChange}
                  value={inputs.base_url}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 8 && (
              <Form.Field>
                <Form.Input
                  required
                  label={t('channel.edit.proxy_url')}
                  name='base_url'
                  placeholder={t('channel.edit.proxy_url_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.base_url}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 50 && (
              <Form.Field>
                <Form.Input
                  required
                  label={t('channel.edit.base_url')}
                  name='base_url'
                  placeholder={t('channel.edit.base_url_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.base_url}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 22 && (
              <Form.Field>
                <Form.Input
                  label='私有部署地址'
                  name='base_url'
                  placeholder={
                    '请输入私有部署地址，格式为：https://fastgpt.run' +
                    '/api' +
                    '/openapi'
                  }
                  onChange={handleInputChange}
                  value={inputs.base_url}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type !== 3 &&
              inputs.type !== 33 &&
              inputs.type !== 8 &&
              inputs.type !== 50 &&
              inputs.type !== 22 && (
                <Form.Field>
                  <Form.Input
                    label={t('channel.edit.proxy_url')}
                    name='base_url'
                    placeholder={t('channel.edit.proxy_url_placeholder')}
                    onChange={handleInputChange}
                    value={inputs.base_url}
                    autoComplete='new-password'
                  />
                </Form.Field>
              )}

            {inputs.type !== 33 &&
              inputs.type !== 42 && (
                <Form.Field>
                  <Form.Input
                    label={t('channel.edit.key')}
                    name='key'
                    required
                    placeholder={type2secretPrompt(inputs.type, t)}
                    onChange={handleInputChange}
                    value={inputs.key}
                    autoComplete='new-password'
                  />
                </Form.Field>
              )}
            {isOpenAICompatibleType(inputs.type) && (
              <>
                <Form.Field>
                  <Form.Input
                    label={t('channel.edit.user_agent.label')}
                    name='user_agent'
                    placeholder={t('channel.edit.user_agent.placeholder')}
                    onChange={handleConfigChange}
                    value={config.user_agent || ''}
                    autoComplete='new-password'
                  />
                  <div
                    style={{ color: 'rgba(0, 0, 0, 0.6)', marginTop: '4px' }}
                  >
                    {t('channel.edit.user_agent.help')}
                  </div>
                </Form.Field>
              </>
            )}
            <Form.Field>
              <Form.Dropdown
                label={t('channel.edit.group')}
                placeholder={t('channel.edit.group_placeholder')}
                name='groups'
                required
                fluid
                multiple
                selection
                allowAdditions
                additionLabel={t('channel.edit.group_addition')}
                onChange={handleInputChange}
                value={inputs.groups}
                autoComplete='new-password'
                options={groupOptions}
              />
            </Form.Field>

            {/* Azure OpenAI specific fields */}
            {inputs.type === 3 && (
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
                  />
                </Form.Field>
              </>
            )}

            {inputs.type === 18 && (
              <Form.Field>
                <Form.Input
                  label={t('channel.edit.spark_version')}
                  name='other'
                  placeholder={t('channel.edit.spark_version_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 21 && (
              <Form.Field>
                <Form.Input
                  label={t('channel.edit.knowledge_id')}
                  name='other'
                  placeholder={t('channel.edit.knowledge_id_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 17 && (
              <Form.Field>
                <Form.Input
                  label={t('channel.edit.plugin_param')}
                  name='other'
                  placeholder={t('channel.edit.plugin_param_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 34 && (
              <Message>{t('channel.edit.coze_notice')}</Message>
            )}
            {inputs.type === 40 && (
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
            {inputs.type !== 43 && (
              <Form.Field>
                <label>{t('channel.edit.models')}</label>
                {showCreateModelFlowGuide && (
                  <Message info style={{ marginBottom: '10px' }}>
                    <Message.Header>
                      {t('channel.edit.model_selector.flow_title')}
                    </Message.Header>
                    <p style={{ marginTop: '6px', marginBottom: 0 }}>
                      {flowStepsText}
                    </p>
                  </Message>
                )}
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
                      total: visibleModelOptions.length,
                    })}
                  </span>
                  <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
                    <Button
                      type='button'
                      size='tiny'
                      onClick={selectAllModels}
                      disabled={visibleModelOptions.length === 0}
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
                  </div>
                </div>
                {isOpenAICompatibleType(inputs.type) && (
                  <div style={{ color: 'rgba(0, 0, 0, 0.6)', marginBottom: '8px' }}>
                    {isRealtimeModelFetchAvailable
                      ? t('channel.edit.model_selector.realtime_enabled')
                      : t('channel.edit.model_selector.realtime_hint')}
                  </div>
                )}
                {unifiedModelStatusText && (
                  <div style={{ color: unifiedModelStatusColor, marginBottom: '8px' }}>
                    {unifiedModelStatusText}
                  </div>
                )}
                <div
                  style={{
                    border: '1px solid rgba(34, 36, 38, 0.15)',
                    borderRadius: '6px',
                    padding: '10px 12px',
                    maxHeight: '320px',
                    overflowY: 'auto',
                    display: 'grid',
                    gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
                    gap: '8px 16px',
                  }}
                >
                  {visibleModelOptions.length === 0 ? (
                    <div style={{ color: 'rgba(0, 0, 0, 0.55)' }}>
                      {t('channel.edit.model_selector.empty')}
                    </div>
                  ) : (
                    visibleModelOptions.map((option) => (
                      <Checkbox
                        key={option.key}
                        label={option.text}
                        checked={inputs.models.includes(option.value)}
                        onChange={(e, { checked }) =>
                          toggleModelSelection(option.value, checked)
                        }
                      />
                    ))
                  )}
                </div>
                {modelsSyncError && (
                  <div style={{ color: '#d9534f', marginTop: '8px' }}>
                    {modelsSyncError}
                  </div>
                )}
                {modelsLastSyncedAt > 0 && (
                  <div style={{ color: 'rgba(0, 0, 0, 0.55)', marginTop: '8px' }}>
                    {t('channel.edit.model_selector.last_synced', {
                      time: new Date(modelsLastSyncedAt).toLocaleString(),
                    })}
                  </div>
                )}
              </Form.Field>
            )}
            {inputs.type !== 43 && (
              <>
                <Form.Field>
                  <Form.TextArea
                    label={t('channel.edit.model_mapping')}
                    placeholder={`${t(
                      'channel.edit.model_mapping_placeholder'
                    )}\n${JSON.stringify(MODEL_MAPPING_EXAMPLE, null, 2)}`}
                    name='model_mapping'
                    onChange={handleInputChange}
                    value={inputs.model_mapping}
                    style={{
                      minHeight: 150,
                      fontFamily: 'JetBrains Mono, Consolas',
                    }}
                    autoComplete='new-password'
                  />
                </Form.Field>
                <Form.Field>
                  <Form.TextArea
                    label={`${t('operation.ratio.model.title', '模型倍率')}（JSON）`}
                    placeholder={t(
                      'operation.ratio.model.placeholder',
                      '为一个 JSON 文本，键为模型名称，值为倍率'
                    )}
                    name='model_ratio'
                    onChange={handleInputChange}
                    value={inputs.model_ratio}
                    style={{
                      minHeight: 150,
                      fontFamily: 'JetBrains Mono, Consolas',
                    }}
                    autoComplete='new-password'
                  />
                </Form.Field>
                <Form.Field>
                  <Form.TextArea
                    label={`${t('operation.ratio.completion.title', '补全倍率')}（JSON）`}
                    placeholder={t(
                      'operation.ratio.completion.placeholder',
                      '为一个 JSON 文本，键为模型名称，值为倍率'
                    )}
                    name='completion_ratio'
                    onChange={handleInputChange}
                    value={inputs.completion_ratio}
                    style={{
                      minHeight: 150,
                      fontFamily: 'JetBrains Mono, Consolas',
                    }}
                    autoComplete='new-password'
                  />
                </Form.Field>
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
                  />
                </Form.Field>
              </>
            )}
            {inputs.type === 33 && (
              <Form.Field>
                <Form.Input
                  label='Region'
                  name='region'
                  required
                  placeholder={t('channel.edit.aws_region_placeholder')}
                  onChange={handleConfigChange}
                  value={config.region}
                  autoComplete=''
                />
                <Form.Input
                  label='AK'
                  name='ak'
                  required
                  placeholder={t('channel.edit.aws_ak_placeholder')}
                  onChange={handleConfigChange}
                  value={config.ak}
                  autoComplete=''
                />
                <Form.Input
                  label='SK'
                  name='sk'
                  required
                  placeholder={t('channel.edit.aws_sk_placeholder')}
                  onChange={handleConfigChange}
                  value={config.sk}
                  autoComplete=''
                />
              </Form.Field>
            )}
            {inputs.type === 42 && (
              <Form.Field>
                <Form.Input
                  label='Region'
                  name='region'
                  required
                  placeholder={t('channel.edit.vertex_region_placeholder')}
                  onChange={handleConfigChange}
                  value={config.region}
                  autoComplete=''
                />
                <Form.Input
                  label={t('channel.edit.vertex_project_id')}
                  name='vertex_ai_project_id'
                  required
                  placeholder={t('channel.edit.vertex_project_id_placeholder')}
                  onChange={handleConfigChange}
                  value={config.vertex_ai_project_id}
                  autoComplete=''
                />
                <Form.Input
                  label={t('channel.edit.vertex_credentials')}
                  name='vertex_ai_adc'
                  required
                  placeholder={t('channel.edit.vertex_credentials_placeholder')}
                  onChange={handleConfigChange}
                  value={config.vertex_ai_adc}
                  autoComplete=''
                />
              </Form.Field>
            )}
            {inputs.type === 34 && (
              <Form.Input
                label={t('channel.edit.user_id')}
                name='user_id'
                required
                placeholder={t('channel.edit.user_id_placeholder')}
                onChange={handleConfigChange}
                value={config.user_id}
                autoComplete=''
              />
            )}
            {inputs.type === 37 && (
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
                />
              </Form.Field>
            )}
            <Button onClick={handleCancel}>
              {t('channel.edit.buttons.cancel')}
            </Button>
            <Button
              type={isEdit ? 'button' : 'submit'}
              positive
              onClick={submit}
              disabled={requiresConnectionVerification && !isCurrentSignatureVerified}
            >
              {t('channel.edit.buttons.submit')}
            </Button>
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditChannel;
