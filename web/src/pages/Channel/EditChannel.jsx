import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Button, Card, Form, Input, Message} from 'semantic-ui-react';
import {useLocation, useNavigate, useParams} from 'react-router-dom';
import {API, copy, getChannelModels, showError, showInfo, showSuccess, verifyJSON,} from '../../helpers';
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

const normalizeModelProviderSelection = (provider) => {
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
      if (trimmed === '火山' || trimmed === '豆包' || trimmed === '字节')
        return 'volcengine';
      return lower;
  }
};

const buildFallbackModelProviderOptions = (t) => [
  {
    key: 'openai',
    text: t('channel.edit.model_provider_options.gpt'),
    value: 'openai',
  },
  {
    key: 'google',
    text: t('channel.edit.model_provider_options.gemini'),
    value: 'google',
  },
  {
    key: 'anthropic',
    text: t('channel.edit.model_provider_options.claude'),
    value: 'anthropic',
  },
  {
    key: 'deepseek',
    text: t('channel.edit.model_provider_options.deepseek'),
    value: 'deepseek',
  },
  {
    key: 'qwen',
    text: t('channel.edit.model_provider_options.qwen'),
    value: 'qwen',
  },
  {
    key: 'xai',
    text: t('channel.edit.model_provider_options.xai'),
    value: 'xai',
  },
  {
    key: 'mistral',
    text: t('channel.edit.model_provider_options.mistral'),
    value: 'mistral',
  },
  {
    key: 'cohere',
    text: t('channel.edit.model_provider_options.cohere'),
    value: 'cohere',
  },
  {
    key: 'zhipu',
    text: t('channel.edit.model_provider_options.zhipu'),
    value: 'zhipu',
  },
  {
    key: 'hunyuan',
    text: t('channel.edit.model_provider_options.hunyuan'),
    value: 'hunyuan',
  },
  {
    key: 'volcengine',
    text: t('channel.edit.model_provider_options.volcengine'),
    value: 'volcengine',
  },
  {
    key: 'minimax',
    text: t('channel.edit.model_provider_options.minimax'),
    value: 'minimax',
  },
];

const buildModelProviderOptionsFromCatalog = (items) => {
  if (!Array.isArray(items)) return [];
  const indexByProvider = new Map();
  const options = [];
  items.forEach((item) => {
    const provider = normalizeModelProviderSelection(
      item?.provider || item?.name || ''
    );
    if (!provider) return;
    const name = typeof item?.name === 'string' ? item.name.trim() : '';
    const text =
      name && name.toLowerCase() !== provider
        ? `${name} (${provider})`
        : name || provider;
    const option = {
      key: provider,
      text,
      value: provider,
    };
    if (indexByProvider.has(provider)) {
      const index = indexByProvider.get(provider);
      options[index] = option;
      return;
    }
    indexByProvider.set(provider, options.length);
    options.push(option);
  });
  return options.sort((a, b) => a.value.localeCompare(b.value));
};

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
    model_provider: '',
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
  const [basicModels, setBasicModels] = useState([]);
  const [fullModels, setFullModels] = useState([]);
  const [catalogModelProviderOptions, setCatalogModelProviderOptions] = useState([]);
  const [customModel, setCustomModel] = useState('');
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [config, setConfig] = useState({
    region: '',
    sk: '',
    ak: '',
    user_id: '',
    vertex_ai_project_id: '',
    vertex_ai_adc: '',
    user_agent: '',
  });
  const fallbackModelProviderOptions = useMemo(
    () => buildFallbackModelProviderOptions(t),
    [t]
  );
  const modelProviderOptions = useMemo(() => {
    const baseOptions =
      catalogModelProviderOptions.length > 0
        ? catalogModelProviderOptions
        : fallbackModelProviderOptions;
    const selectedProvider = normalizeModelProviderSelection(
      inputs.model_provider
    );
    if (!selectedProvider) {
      return baseOptions;
    }
    if (baseOptions.some((option) => option.value === selectedProvider)) {
      return baseOptions;
    }
    return [
      ...baseOptions,
      { key: selectedProvider, text: selectedProvider, value: selectedProvider },
    ];
  }, [catalogModelProviderOptions, fallbackModelProviderOptions, inputs.model_provider]);
  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
    if (name === 'type') {
      setBasicModels(getChannelModels(value));
    }
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
      data.model_provider = normalizeModelProviderSelection(data.model_provider);
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
          model_provider: data.model_provider || '',
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
      setBasicModels(getChannelModels(data.type));
    } else {
      showError(message);
    }
    setLoading(false);
  }, []);

  const fetchModels = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/public/channel/models`);
      const payload = res?.data?.data;
      const meta = res?.data?.meta;
      const flattenedModels = flattenModels(payload, meta);
      const { options, ids } = buildModelOptions(flattenedModels);
      setOriginModelOptions(options);
      setFullModels(ids);
    } catch (error) {
      showError(error?.message || error);
    }
  }, []);

  const handleFetchModels = async () => {
    const selectedProvider = normalizeModelProviderSelection(
      inputs.model_provider
    );
    setFetchModelsLoading(true);
    try {
      let models = [];
      if (
        isOpenAICompatibleType(inputs.type) &&
        inputs.key &&
        inputs.key.trim() !== ''
      ) {
        const res = await API.post(`/api/v1/admin/channel/preview/models`, {
          type: inputs.type,
          key: inputs.key,
          base_url: inputs.base_url,
          config,
          model_provider: selectedProvider,
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || '获取模型失败');
          return;
        }
        models = Array.isArray(data) ? data.filter((model) => model) : [];
      } else {
        const params = selectedProvider
          ? { model_provider: selectedProvider }
          : undefined;
        const res = await API.get(`/api/v1/public/channel/models`, { params });
        const payload = res?.data?.data;
        const meta = res?.data?.meta;
        models = flattenModels(payload, meta);
      }

      const { ids } = buildModelOptions(models);
      if (ids.length === 0) {
        showInfo(
          selectedProvider
            ? '未找到符合所选供应商的模型'
            : '未返回可用模型'
        );
        return;
      }

      setInputs((prev) => ({ ...prev, models: ids }));
      showSuccess(t('channel.messages.operation_success'));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setFetchModelsLoading(false);
    }
  };

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

  const fetchModelProviders = useCallback(async () => {
    try {
      const res = await API.get(`/api/v1/admin/model-provider`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('channel.providers.messages.load_failed'));
        return;
      }
      const options = buildModelProviderOptionsFromCatalog(data);
      if (options.length > 0) {
        setCatalogModelProviderOptions(options);
      }
    } catch (error) {
      showError(error?.message || error);
    }
  }, [t]);

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
    if (!isEdit && copyFromId <= 0) {
      let localModels = getChannelModels(inputs.type);
      setBasicModels(localModels);
    }
  }, [copyFromId, inputs.type, isEdit]);

  useEffect(() => {
    fetchModels().then();
    fetchGroups().then();
    fetchModelProviders().then();
    fetchChannelTypes().then();
  }, [fetchModels, fetchGroups, fetchModelProviders, fetchChannelTypes]);

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
    if (normalizeModelProviderSelection(inputs.model_provider) === '') {
      showInfo(t('channel.edit.messages.model_provider_required'));
      return;
    }
    if (inputs.groups.length === 0) {
      showInfo(t('channel.edit.messages.groups_required'));
      return;
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
    localInputs.model_provider = normalizeModelProviderSelection(
      localInputs.model_provider
    );
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

  const addCustomModel = () => {
    if (customModel.trim() === '') return;
    if (inputs.models.includes(customModel)) return;
    let localModels = [...inputs.models];
    localModels.push(customModel);
    let localModelOptions = [];
    localModelOptions.push({
      key: customModel,
      text: customModel,
      value: customModel,
    });
    setModelOptions((modelOptions) => {
      return [...modelOptions, ...localModelOptions];
    });
    setCustomModel('');
    handleInputChange(null, { name: 'models', value: localModels });
  };

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
                label={t('channel.edit.model_provider')}
                name='model_provider'
                required
                options={modelProviderOptions}
                value={inputs.model_provider}
                onChange={(e, { name, value }) =>
                  handleInputChange(e, { name, value: value || '' })
                }
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
                <Form.Dropdown
                  label={t('channel.edit.models')}
                  placeholder={t('channel.edit.models_placeholder')}
                  name='models'
                  required
                  fluid
                  multiple
                  search
                  onLabelClick={(e, { value }) => {
                    copy(value).then();
                  }}
                  selection
                  onChange={handleInputChange}
                  value={inputs.models}
                  autoComplete='new-password'
                  options={modelOptions}
                />
              </Form.Field>
            )}
            {inputs.type !== 43 && (
              <Form.Field>
                <Button
                  type='button'
                  color='green'
                  loading={fetchModelsLoading}
                  disabled={fetchModelsLoading}
                  onClick={handleFetchModels}
                >
                  获取模型
                </Button>
              </Form.Field>
            )}
            {inputs.type !== 43 && (
              <div style={{ lineHeight: '40px', marginBottom: '12px' }}>
                <Button
                  type={'button'}
                  onClick={() => {
                    handleInputChange(null, {
                      name: 'models',
                      value: basicModels,
                    });
                  }}
                >
                  {t('channel.edit.buttons.fill_models')}
                </Button>
                <Button
                  type={'button'}
                  onClick={() => {
                    handleInputChange(null, {
                      name: 'models',
                      value: fullModels,
                    });
                  }}
                >
                  {t('channel.edit.buttons.fill_all')}
                </Button>
                <Button
                  type={'button'}
                  onClick={() => {
                    handleInputChange(null, { name: 'models', value: [] });
                  }}
                >
                  {t('channel.edit.buttons.clear')}
                </Button>
                <Input
                  action={
                    <Button type={'button'} onClick={addCustomModel}>
                      {t('channel.edit.buttons.add_custom')}
                    </Button>
                  }
                  placeholder={t('channel.edit.buttons.custom_placeholder')}
                  value={customModel}
                  onChange={(e, { value }) => {
                    setCustomModel(value);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      addCustomModel();
                      e.preventDefault();
                    }
                  }}
                />
              </div>
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
