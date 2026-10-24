/* i18n-skip */
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppEmpty,
  AppField,
  AppForm,
  AppFormActions,
  AppIcon,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSection,
  AppSelect,
  AppTable,
  AppTabs,
  AppTag,
  AppTextarea,
} from '../../router-ui';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import './index.css';

const PROTOCOL_OPTIONS = [
  { value: 'openai', label: 'OpenAI 兼容' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'ali', label: 'Qwen' },
  { value: 'deepseek', label: 'DeepSeek' },
];
const ENDPOINT_OPTIONS = [
  { value: '/v1/chat/completions', label: '对话' },
  { value: '/v1/responses', label: 'Responses' },
  { value: '/v1/embeddings', label: 'Embeddings' },
];
const STATUS_COLOR = {
  draft: 'grey',
  reviewing: 'orange',
  active: 'green',
  ready: 'green',
  published: 'green',
  accrued: 'blue',
  held: 'orange',
  settled: 'green',
  reversed: 'red',
  restricted: 'orange',
  suspended: 'red',
  rejected: 'red',
  offline: 'red',
};
const STATUS_LABEL = {
  draft: '草稿',
  reviewing: '审核中',
  active: '已通过',
  ready: '已就绪',
  published: '已发布',
  accrued: '待结算',
  held: '已冻结',
  settled: '已结算',
  reversed: '已冲销',
  restricted: '受限',
  suspended: '已暂停',
  rejected: '已驳回',
  offline: '已下线',
};
const emptyService = {
  name: '',
  protocol: 'openai',
  base_url: '',
  api_key: '',
  region: '',
  data_policy: '{\n  "retention": "none",\n  "training_usage": "prohibited"\n}',
  capacity_policy: '{\n  "max_concurrency": 8\n}',
  models_text: '',
  endpoint: '/v1/chat/completions',
};
const emptyOffer = {
  service_id: '',
  model: '',
  scope: 'public',
  currency: 'USD',
  input_price_micros: 0,
  output_price_micros: 0,
};

const statusTag = (value) => (
  <AppTag color={STATUS_COLOR[value] || 'grey'}>
    {STATUS_LABEL[value] || value || '-'}
  </AppTag>
);
const parseModels = (value, endpoint) =>
  Array.from(
    new Set(
      String(value || '')
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  ).map((model) => ({
    model,
    upstream_model: model,
    endpoint,
    metering_unit: 'token',
  }));

function PublisherWorkspace({
  embedded = false,
  sourceConnection = null,
  onSourceConsumed = null,
}) {
  const [profile, setProfile] = useState(null);
  const [services, setServices] = useState([]);
  const [offers, setOffers] = useState([]);
  const [settlements, setSettlements] = useState([]);
  const [groupAssignments, setGroupAssignments] = useState([]);
  const [activeTab, setActiveTab] = useState(
    sourceConnection ? 'services' : 'profile'
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [serviceModal, setServiceModal] = useState(false);
  const [offerModal, setOfferModal] = useState(false);
  const [editingService, setEditingService] = useState(null);
  const [sourceConnectionForService, setSourceConnectionForService] =
    useState(null);
  const [editingOffer, setEditingOffer] = useState(null);
  const [verifyingID, setVerifyingID] = useState('');
  const [profileForm] = AppForm.useForm();
  const [serviceForm] = AppForm.useForm();
  const [offerForm] = AppForm.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [
        profileResponse,
        serviceResponse,
        offerResponse,
        settlementResponse,
        assignmentResponse,
      ] = await Promise.all([
        API.get('/api/v1/public/publisher/profile'),
        API.get('/api/v1/public/publisher/services'),
        API.get('/api/v1/public/publisher/offers'),
        API.get('/api/v1/public/publisher/settlements'),
        API.get('/api/v1/public/publisher/group-assignments'),
      ]);
      for (const response of [
        profileResponse,
        serviceResponse,
        offerResponse,
        settlementResponse,
        assignmentResponse,
      ]) {
        if (!response.data?.success)
          throw new Error(response.data?.message || '加载发布者工作区失败');
      }
      setProfile(profileResponse.data?.data || null);
      setServices(
        Array.isArray(serviceResponse.data?.data)
          ? serviceResponse.data.data
          : []
      );
      setOffers(
        Array.isArray(offerResponse.data?.data) ? offerResponse.data.data : []
      );
      setSettlements(
        Array.isArray(settlementResponse.data?.data)
          ? settlementResponse.data.data
          : []
      );
      setGroupAssignments(
        Array.isArray(assignmentResponse.data?.data)
          ? assignmentResponse.data.data
          : []
      );
      profileForm.setFieldsValue(
        profileResponse.data?.data || { display_name: '', contact_email: '' }
      );
    } catch (error) {
      showError(error?.message || '加载发布者工作区失败');
    } finally {
      setLoading(false);
    }
  }, [profileForm]);

  useEffect(() => {
    load().then();
  }, [load]);

  const saveProfile = async () => {
    try {
      const values = await profileForm.validateFields();
      setSaving(true);
      const response = await API.put(
        '/api/v1/public/publisher/profile',
        values
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('发布者资料已保存');
      load().then();
    } catch (error) {
      if (!error?.errorFields)
        showError(error?.message || '保存发布者资料失败');
    } finally {
      setSaving(false);
    }
  };

  const submitApplication = async () => {
    try {
      const response = await API.post('/api/v1/public/publisher/applications');
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('发布者申请已提交');
      load().then();
    } catch (error) {
      showError(error?.message || '提交申请失败');
    }
  };

  const openService = useCallback(
    (row = null, personalConnection = null) => {
      setEditingService(row);
      setSourceConnectionForService(personalConnection);
      serviceForm.setFieldsValue(
        row
          ? {
              ...row,
              api_key: '',
              models_text: (row.models || [])
                .map((model) => model.model)
                .join('\n'),
              endpoint: row.models?.[0]?.endpoint || '/v1/chat/completions',
              data_policy: row.data_policy || '{}',
              capacity_policy: row.capacity_policy || '{}',
            }
          : personalConnection
          ? {
              ...emptyService,
              name: personalConnection.name || '',
              protocol: personalConnection.protocol || 'openai',
              base_url: personalConnection.base_url || '',
              models_text: Array.isArray(personalConnection.models)
                ? personalConnection.models.join('\n')
                : '',
            }
          : emptyService
      );
      setServiceModal(true);
    },
    [serviceForm]
  );

  useEffect(() => {
    if (!sourceConnection?.id) return;
    setActiveTab('services');
    openService(null, sourceConnection);
  }, [openService, sourceConnection]);

  const saveService = async () => {
    try {
      const values = await serviceForm.validateFields();
      const payload = {
        ...values,
        data_policy: JSON.parse(values.data_policy || '{}'),
        capacity_policy: JSON.parse(values.capacity_policy || '{}'),
        models: parseModels(values.models_text, values.endpoint),
      };
      delete payload.models_text;
      delete payload.endpoint;
      if (!editingService && !payload.api_key.trim()) {
        serviceForm.setFields([
          { name: 'api_key', errors: ['请输入 API Key'] },
        ]);
        return;
      }
      setSaving(true);
      let response;
      if (editingService) {
        response = await API.put(
          `/api/v1/public/publisher/services/${editingService.id}`,
          payload
        );
      } else if (sourceConnectionForService?.id) {
        response = await API.post(
          `/api/v1/public/publisher/services/from-personal/${sourceConnectionForService.id}`,
          payload
        );
      } else {
        response = await API.post('/api/v1/public/publisher/services', payload);
      }
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess(
        editingService
          ? '供给渠道已更新，需要重新验证'
          : '供给渠道已创建'
      );
      setServiceModal(false);
      setSourceConnectionForService(null);
      onSourceConsumed?.();
      load().then();
    } catch (error) {
      if (error?.errorFields) return;
      showError(
        error instanceof SyntaxError
          ? '数据规则必须是有效 JSON'
          : error?.message || '保存供给渠道失败'
      );
    } finally {
      setSaving(false);
    }
  };

  const verifyService = async (row) => {
    setVerifyingID(row.id);
    try {
      const response = await API.post(
        `/api/v1/public/publisher/services/${row.id}/verify`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('供给渠道验证通过');
    } catch (error) {
      showError(error?.message || '供给渠道验证失败');
    } finally {
      setVerifyingID('');
      load().then();
    }
  };

  const deleteService = async (row) => {
    try {
      const response = await API.delete(
        `/api/v1/public/publisher/services/${row.id}`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('供给渠道已删除');
      load().then();
    } catch (error) {
      showError(error?.message || '删除供给渠道失败');
    }
  };

  const serviceModelOptions = useMemo(
    () =>
      services.flatMap((service) =>
        (service.models || []).map((item) => ({
          value: `${service.id}::${item.model}`,
          label: `${service.name} / ${item.model}`,
        }))
      ),
    [services]
  );
  const openOffer = (row = null) => {
    setEditingOffer(row);
    offerForm.setFieldsValue(row || emptyOffer);
    setOfferModal(true);
  };
  const saveOffer = async () => {
    try {
      const values = await offerForm.validateFields();
      const [serviceID, model] = String(
        values.service_model || `${values.service_id}::${values.model}`
      ).split('::');
      const payload = { ...values, service_id: serviceID, model };
      delete payload.service_model;
      setSaving(true);
      const response = editingOffer
        ? await API.put(
            `/api/v1/public/publisher/offers/${editingOffer.id}`,
            payload
          )
        : await API.post('/api/v1/public/publisher/offers', payload);
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess(editingOffer ? '报价已更新' : '报价草稿已创建');
      setOfferModal(false);
      load().then();
    } catch (error) {
      if (!error?.errorFields) showError(error?.message || '保存报价失败');
    } finally {
      setSaving(false);
    }
  };
  const submitOffer = async (row) => {
    try {
      const response = await API.post(
        `/api/v1/public/publisher/offers/${row.id}/submit`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('报价已提交审核');
      load().then();
    } catch (error) {
      showError(error?.message || '提交报价失败');
    }
  };
  const deleteOffer = async (row) => {
    try {
      const response = await API.delete(
        `/api/v1/public/publisher/offers/${row.id}`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('报价已删除');
      load().then();
    } catch (error) {
      showError(error?.message || '删除报价失败');
    }
  };

  const serviceColumns = [
    {
      title: '供给渠道',
      dataIndex: 'name',
      render: (value, row) => (
        <div className='publisher-service-name'>
          <strong>{value}</strong>
          <span>{row.base_url}</span>
        </div>
      ),
    },
    {
      title: '模型',
      dataIndex: 'models',
      render: (models) =>
        (models || []).map((item) => item.model).join(', ') || '-',
    },
    {
      title: '区域',
      dataIndex: 'region',
      width: 100,
      render: (value) => value || '-',
    },
    {
      title: '验证',
      width: 160,
      render: (_, row) =>
        row.last_checked_at ? (
          <div className='publisher-verify-state'>
            {statusTag(row.last_check_ok ? 'ready' : 'offline')}
            <span title={row.last_check_error || ''}>
              {row.last_check_ok
                ? timestamp2string(row.last_checked_at)
                : row.last_check_error || timestamp2string(row.last_checked_at)}
            </span>
          </div>
        ) : (
          statusTag('draft')
        ),
    },
    { title: '状态', dataIndex: 'status', width: 94, render: statusTag },
    {
      title: '操作',
      width: 210,
      render: (_, row) => (
        <div className='publisher-actions'>
          <AppButton
            type='text'
            loading={verifyingID === row.id}
            onClick={() => verifyService(row)}
          >
            测试连接
          </AppButton>
          <AppButton type='text' onClick={() => openService(row)}>
            编辑
          </AppButton>
          <AppButton type='text' danger onClick={() => deleteService(row)}>
            删除
          </AppButton>
        </div>
      ),
    },
  ];
  const offerColumns = [
    { title: '模型', dataIndex: 'model' },
    {
      title: '供给渠道',
      dataIndex: 'service_id',
      render: (value) =>
        services.find((item) => item.id === value)?.name || value,
    },
    {
      title: '输入 / 输出价格',
      render: (_, row) =>
        `${row.input_price_micros} / ${row.output_price_micros} 微美元 / 百万 Token`,
    },
    { title: '状态', dataIndex: 'status', width: 96, render: statusTag },
    {
      title: '操作',
      width: 206,
      render: (_, row) => (
        <div className='publisher-actions'>
          {(row.status === 'draft' || row.status === 'rejected') && (
            <AppButton type='text' onClick={() => submitOffer(row)}>
              提交审核
            </AppButton>
          )}
          {(row.status === 'draft' || row.status === 'rejected') && (
            <AppButton type='text' onClick={() => openOffer(row)}>
              编辑
            </AppButton>
          )}
          {(row.status === 'draft' || row.status === 'rejected') && (
            <AppButton type='text' danger onClick={() => deleteOffer(row)}>
              删除
            </AppButton>
          )}
        </div>
      ),
    },
  ];
  const settlementColumns = [
    {
      title: '请求',
      dataIndex: 'request_log_id',
      render: (value) => <span className='publisher-monospace'>{value}</span>,
    },
    { title: '模型', dataIndex: 'model' },
    {
      title: '消费者成交额',
      dataIndex: 'consumer_amount_micros',
      render: (value) => `${value} 微美元`,
    },
    {
      title: '平台服务费',
      dataIndex: 'platform_fee_amount_micros',
      render: (value) => `${value} 微美元`,
    },
    {
      title: '发布者应收',
      dataIndex: 'publisher_payable_micros',
      render: (value) => `${value} 微美元`,
    },
    { title: '状态', dataIndex: 'status', width: 96, render: statusTag },
    {
      title: '产生时间',
      dataIndex: 'created_at',
      width: 168,
      render: (value) => (value ? timestamp2string(value) : '-'),
    },
  ];
  const assignmentColumns = [
    {
      title: '运营分组',
      dataIndex: 'group_name',
      render: (value, row) => value || row.group_id || '-',
    },
    { title: '模型', dataIndex: 'model' },
    {
      title: '报价',
      dataIndex: 'offer_id',
      render: (value) => <span className='publisher-monospace'>{value}</span>,
    },
    { title: '优先级', dataIndex: 'priority', width: 96, align: 'right' },
    {
      title: '状态',
      dataIndex: 'enabled',
      width: 96,
      render: (value) => (
        <AppTag color={value ? 'green' : 'grey'}>
          {value ? '已启用' : '已停用'}
        </AppTag>
      ),
    },
  ];

  return (
    <div
      className={`${
        embedded ? '' : 'dashboard-container '
      }publisher-workspace-page${
        embedded ? ' publisher-workspace-embedded' : ''
      }`}
    >
      {!embedded ? (
        <AppAlert
          type='info'
          message='可信发布试点'
          description='自用渠道始终仅供本人使用。参与供给会创建独立的供给渠道，只复制连接名称、协议、地址和模型范围；API Key 必须重新录入。报价审核和分组绑定不会改变渠道类型，成功调用进入结算。'
        />
      ) : null}
      <AppTabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'profile',
            label: '发布者资料',
            children: (
              <AppSection
                title='发布者资料'
                extra={profile ? statusTag(profile.status) : null}
              >
                <AppForm
                  form={profileForm}
                  layout='vertical'
                  className='publisher-profile-form'
                >
                  <AppField label='发布者名称' required>
                    <AppForm.Item
                      name='display_name'
                      rules={[{ required: true, message: '请输入发布者名称' }]}
                      noStyle
                    >
                      <AppInput />
                    </AppForm.Item>
                  </AppField>
                  <AppField label='联系邮箱' required>
                    <AppForm.Item
                      name='contact_email'
                      rules={[
                        {
                          required: true,
                          type: 'email',
                          message: '请输入有效联系邮箱',
                        },
                      ]}
                      noStyle
                    >
                      <AppInput />
                    </AppForm.Item>
                  </AppField>
                  <AppField label='钱包身份'>
                    <AppInput
                      value={
                        profile?.wallet_identity_did ||
                        '完成钱包身份验证后可创建发布者资料'
                      }
                      disabled
                    />
                  </AppField>
                  <AppFormActions>
                    <AppButton
                      color='blue'
                      loading={saving}
                      onClick={saveProfile}
                    >
                      保存资料
                    </AppButton>
                    {profile?.status === 'draft' ||
                    profile?.status === 'restricted' ? (
                      <AppButton onClick={submitApplication}>
                        提交审核
                      </AppButton>
                    ) : null}
                  </AppFormActions>
                </AppForm>
                {profile?.review_note ? (
                  <p className='publisher-review-note'>
                    审核说明：{profile.review_note}
                  </p>
                ) : null}
              </AppSection>
            ),
          },
          {
            key: 'services',
            label: '供给渠道',
            children: (
              <AppSection
                title='供给渠道'
                extra={
                  <AppButton
                    color='blue'
                    icon={<AppIcon name='plus' />}
                    onClick={() => openService()}
                  >
                    创建供给渠道
                  </AppButton>
                }
              >
                <p className='publisher-hint'>
                  供给渠道与自用渠道都由你独立运营，但属于两个凭据独立的渠道实例。测试连接仅验证地址和凭据，不会生成异步任务，也不会发起模型推理。
                </p>
                <AppTable
                  rowKey='id'
                  columns={serviceColumns}
                  dataSource={services}
                  loading={loading}
                  scroll={{ x: 1100 }}
                  pagination={false}
                  locale={{
                    emptyText: <AppEmpty>还没有供给渠道</AppEmpty>,
                  }}
                />
              </AppSection>
            ),
          },
          {
            key: 'offers',
            label: '报价',
            children: (
              <AppSection
                title='报价'
                extra={
                  <AppButton
                    color='blue'
                    icon={<AppIcon name='plus' />}
                    disabled={!services.length}
                    onClick={() => openOffer()}
                  >
                    创建报价
                  </AppButton>
                }
              >
                <p className='publisher-hint'>
                  报价是供给渠道下的商业声明，不是独立渠道。提交前，发布者必须已通过审核，关联供给渠道必须完成验证。价格使用微美元
                  / 百万 Token，避免浮点金额误差。
                </p>
                <AppTable
                  rowKey='id'
                  columns={offerColumns}
                  dataSource={offers}
                  loading={loading}
                  scroll={{ x: 1080 }}
                  pagination={false}
                  locale={{ emptyText: <AppEmpty>还没有报价草稿</AppEmpty> }}
                />
              </AppSection>
            ),
          },
          {
            key: 'groups',
            label: '分组结果',
            children: (
              <AppSection title='运营分组结果'>
                <p className='publisher-hint'>
                  分组由运营统一管理。这里仅展示已发布报价被加入哪些路由分组，发布者不能自行修改。
                </p>
                <AppTable
                  rowKey={(row) => `${row.group_id}:${row.offer_id}`}
                  columns={assignmentColumns}
                  dataSource={groupAssignments}
                  loading={loading}
                  scroll={{ x: 820 }}
                  pagination={false}
                  locale={{ emptyText: <AppEmpty>暂未加入运营分组</AppEmpty> }}
                />
              </AppSection>
            ),
          },
          {
            key: 'settlements',
            label: '结算',
            children: (
              <AppSection title='发布者结算'>
                <p className='publisher-hint'>
                  每条记录对应一次供给渠道的成功调用，保留成交价、平台服务费和发布者应收的不可变快照。
                </p>
                <AppTable
                  rowKey='request_log_id'
                  columns={settlementColumns}
                  dataSource={settlements}
                  loading={loading}
                  scroll={{ x: 1120 }}
                  pagination={false}
                  locale={{
                    emptyText: <AppEmpty>还没有发布者结算记录</AppEmpty>,
                  }}
                />
              </AppSection>
            ),
          },
        ]}
      />
      <AppModal
        open={serviceModal}
        onClose={() => {
          setServiceModal(false);
          setSourceConnectionForService(null);
          onSourceConsumed?.();
        }}
        title={
          editingService
            ? '编辑供给渠道'
            : sourceConnectionForService
            ? '申请参与供给'
            : '创建供给渠道'
        }
        size='small'
        footer={
          <AppFormActions>
            <AppButton
              onClick={() => {
                setServiceModal(false);
                setSourceConnectionForService(null);
                onSourceConsumed?.();
              }}
            >
              取消
            </AppButton>
            <AppButton color='blue' loading={saving} onClick={saveService}>
              保存
            </AppButton>
          </AppFormActions>
        }
      >
        {sourceConnectionForService ? (
          <AppAlert
            type='info'
            message='将创建独立供给服务'
            description='只复制非敏感连接信息。自用渠道的 API Key 不会共享，请为供给渠道重新输入凭据。'
          />
        ) : null}
        <AppForm
          form={serviceForm}
          layout='vertical'
          initialValues={emptyService}
        >
          <AppField label='服务名称' required>
            <AppForm.Item
              name='name'
              rules={[{ required: true, message: '请输入服务名称' }]}
              noStyle
            >
              <AppInput />
            </AppForm.Item>
          </AppField>
          <AppField label='上游协议'>
            <AppForm.Item name='protocol' noStyle>
              <AppSelect options={PROTOCOL_OPTIONS} />
            </AppForm.Item>
          </AppField>
          <AppField label='Base URL' required hint='仅支持 HTTPS 公网地址'>
            <AppForm.Item
              name='base_url'
              rules={[{ required: true, message: '请输入 Base URL' }]}
              noStyle
            >
              <AppInput placeholder='https://api.example.com/v1' />
            </AppForm.Item>
          </AppField>
          <AppField
            label={editingService ? '轮换 API Key' : 'API Key'}
            required={!editingService}
            hint={
              editingService
                ? '留空则保留当前凭据'
                : '凭据会加密保存，之后不会再显示；从自用渠道申请供给时也必须重新输入'
            }
          >
            <AppForm.Item
              name='api_key'
              rules={
                editingService
                  ? []
                  : [{ required: true, message: '请输入 API Key' }]
              }
              noStyle
            >
              <AppInput type='password' autoComplete='new-password' />
            </AppForm.Item>
          </AppField>
          <AppField label='实际处理区域'>
            <AppForm.Item name='region' noStyle>
              <AppInput placeholder='例如 CN-SH' />
            </AppForm.Item>
          </AppField>
          <AppField
            label='模型范围'
            required
            hint='可信发布试点仅支持当前标准模型目录；每行一个模型'
          >
            <AppForm.Item
              name='models_text'
              rules={[{ required: true, message: '至少填写一个模型' }]}
              noStyle
            >
              <AppTextarea rows={4} placeholder='gpt-5.4' />
            </AppForm.Item>
          </AppField>
          <AppField label='调用端点'>
            <AppForm.Item name='endpoint' noStyle>
              <AppSelect options={ENDPOINT_OPTIONS} />
            </AppForm.Item>
          </AppField>
          <AppField label='数据规则' required>
            <AppForm.Item
              name='data_policy'
              rules={[{ required: true, message: '请输入数据规则 JSON' }]}
              noStyle
            >
              <AppTextarea rows={4} />
            </AppForm.Item>
          </AppField>
          <AppField label='容量规则' required>
            <AppForm.Item
              name='capacity_policy'
              rules={[{ required: true, message: '请输入容量规则 JSON' }]}
              noStyle
            >
              <AppTextarea rows={3} />
            </AppForm.Item>
          </AppField>
        </AppForm>
      </AppModal>
      <AppModal
        open={offerModal}
        onClose={() => setOfferModal(false)}
        title={editingOffer ? '编辑报价草稿' : '创建报价草稿'}
        size='small'
        footer={
          <AppFormActions>
            <AppButton onClick={() => setOfferModal(false)}>取消</AppButton>
            <AppButton color='blue' loading={saving} onClick={saveOffer}>
              保存
            </AppButton>
          </AppFormActions>
        }
      >
        <AppForm form={offerForm} layout='vertical' initialValues={emptyOffer}>
          <AppField label='供给渠道和模型' required>
            <AppForm.Item
              name='service_model'
              initialValue={
                editingOffer
                  ? `${editingOffer.service_id}::${editingOffer.model}`
                  : undefined
              }
              rules={[
                { required: !editingOffer, message: '请选择供给渠道和模型' },
              ]}
              noStyle
            >
              <AppSelect
                options={serviceModelOptions}
                search
                placeholder='选择已声明的模型'
              />
            </AppForm.Item>
          </AppField>
          {editingOffer ? (
            <>
              <AppForm.Item name='service_id' hidden>
                <AppInput />
              </AppForm.Item>
              <AppForm.Item name='model' hidden>
                <AppInput />
              </AppForm.Item>
            </>
          ) : null}
          <AppField label='报价范围'>
            <AppForm.Item name='scope' noStyle>
              <AppSelect
                options={[
                  { value: 'public', label: '公开目录' },
                  { value: 'invite', label: '邀请范围' },
                ]}
              />
            </AppForm.Item>
          </AppField>
          <AppField label='输入价格' required hint='微美元 / 百万 Token'>
            <AppForm.Item
              name='input_price_micros'
              rules={[{ required: true, message: '请输入输入价格' }]}
              noStyle
            >
              <AppInputNumber min={0} />
            </AppForm.Item>
          </AppField>
          <AppField label='输出价格' required hint='微美元 / 百万 Token'>
            <AppForm.Item
              name='output_price_micros'
              rules={[{ required: true, message: '请输入输出价格' }]}
              noStyle
            >
              <AppInputNumber min={0} />
            </AppForm.Item>
          </AppField>
        </AppForm>
      </AppModal>
    </div>
  );
}

export default PublisherWorkspace;
