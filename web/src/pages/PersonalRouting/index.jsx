/* i18n-skip */
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  AppButton,
  AppEmpty,
  AppField,
  AppForm,
  AppFormActions,
  AppIcon,
  AppInput,
  AppModal,
  AppSection,
  AppSegmented,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTabs,
  AppTag,
  AppTextarea,
} from '../../router-ui';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import PublisherWorkspace from '../PublisherWorkspace';
import './index.css';

const POLICY_OPTIONS = [
  { value: 'personal_first', label: '自用渠道优先，供给渠道回退' },
  { value: 'personal_only', label: '仅自用渠道' },
  { value: 'community_only', label: '仅供给渠道' },
  { value: 'community_first', label: '供给渠道优先' },
];

const PROTOCOL_OPTIONS = [
  { value: 'openai', label: 'OpenAI 兼容' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'ali', label: 'Qwen' },
  { value: 'deepseek', label: 'DeepSeek' },
];

const emptyConnection = {
  name: '',
  protocol: 'openai',
  base_url: '',
  api_key: '',
  models_text: '',
  priority: 0,
  status: 1,
};

const PERIOD_OPTIONS = [
  { value: 'last_hour', label: '1 小时' },
  { value: 'last_24_hours', label: '24 小时' },
  { value: 'last_7_days', label: '7 天' },
  { value: 'last_30_days', label: '30 天' },
];

const HEALTH_POINT_COLOR = {
  success: 'var(--router-status-success, #16a34a)',
  warning: 'var(--router-status-warning, #d97706)',
  failure: 'var(--router-status-danger, #dc2626)',
  unknown: 'var(--router-border-strong, #cbd5e1)',
};

function parseModels(value) {
  return Array.from(
    new Set(
      String(value || '')
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  ).sort();
}

function PersonalRouting() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab');
  const activeTab = ['channels', 'routes', 'supply'].includes(requestedTab)
    ? requestedTab
    : 'channels';
  const [period, setPeriod] = useState(
    searchParams.get('period') || 'last_24_hours'
  );
  const [connections, setConnections] = useState([]);
  const [routes, setRoutes] = useState([]);
  const [communityOfferRoutes, setCommunityOfferRoutes] = useState([]);
  const [routeModelOptions, setRouteModelOptions] = useState([]);
  const [quota, setQuota] = useState(null);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [saving, setSaving] = useState(false);
  const [verifyingConnectionID, setVerifyingConnectionID] = useState('');
  const [supplySource, setSupplySource] = useState(null);
  const [form] = AppForm.useForm();
  const [routeForm] = AppForm.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [
        connectionResponse,
        routeResponse,
        communityOfferRouteResponse,
        quotaResponse,
        modelsResponse,
      ] = await Promise.all([
        API.get('/api/v1/public/personal-provider/connections', {
          params: { period },
        }),
        API.get('/api/v1/public/personal-provider/model-routes'),
        API.get('/api/v1/public/community-offer-routing/model-routes'),
        API.get('/api/v1/public/personal-provider/routing-quota'),
        API.get('/api/v1/public/user/models/available'),
      ]);
      if (!connectionResponse.data?.success)
        throw new Error(connectionResponse.data?.message);
      if (!routeResponse.data?.success)
        throw new Error(routeResponse.data?.message);
      if (!communityOfferRouteResponse.data?.success)
        throw new Error(communityOfferRouteResponse.data?.message);
      if (!quotaResponse.data?.success)
        throw new Error(quotaResponse.data?.message);
      if (!modelsResponse.data?.success)
        throw new Error(modelsResponse.data?.message);
      setConnections(
        Array.isArray(connectionResponse.data?.data)
          ? connectionResponse.data.data
          : []
      );
      setRoutes(
        Array.isArray(routeResponse.data?.data) ? routeResponse.data.data : []
      );
      setCommunityOfferRoutes(
        Array.isArray(communityOfferRouteResponse.data?.data)
          ? communityOfferRouteResponse.data.data
          : []
      );
      setQuota(quotaResponse.data?.data || null);
      setRouteModelOptions(
        (Array.isArray(modelsResponse.data?.data)
          ? modelsResponse.data.data
          : []
        )
          .map((model) => String(model || '').trim())
          .filter(Boolean)
          .map((model) => ({ value: model, label: model }))
      );
    } catch (error) {
      showError(error?.message || '加载个人路由失败');
    } finally {
      setLoading(false);
    }
  }, [period]);

  useEffect(() => {
    load().then();
  }, [load]);

  const selectTab = (tab) => {
    const next = new URLSearchParams(searchParams);
    next.set('tab', tab);
    if (period) next.set('period', period);
    setSearchParams(next);
  };

  const selectPeriod = (_, detail) => {
    const nextPeriod = detail.value;
    setPeriod(nextPeriod);
    const next = new URLSearchParams(searchParams);
    next.set('period', nextPeriod);
    next.set('tab', activeTab);
    setSearchParams(next, { replace: true });
  };

  const openCreate = () => {
    setEditing(null);
    form.setFieldsValue(emptyConnection);
    setModalOpen(true);
  };
  const openEdit = (row) => {
    setEditing(row);
    form.setFieldsValue({
      ...row,
      api_key: '',
      models_text: Array.isArray(row.models) ? row.models.join('\n') : '',
    });
    setModalOpen(true);
  };

  const saveConnection = async () => {
    try {
      const values = await form.validateFields();
      const payload = {
        ...values,
        models: parseModels(values.models_text),
        status: values.status ? 1 : 2,
      };
      delete payload.models_text;
      if (!editing && !payload.api_key.trim()) {
        form.setFields([{ name: 'api_key', errors: ['请输入 API Key'] }]);
        return;
      }
      setSaving(true);
      const response = editing
        ? await API.put(
            `/api/v1/public/personal-provider/connections/${editing.id}`,
            payload
          )
        : await API.post(
            '/api/v1/public/personal-provider/connections',
            payload
          );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess(editing ? '自用渠道已更新' : '自用渠道已添加');
      setModalOpen(false);
      load().then();
    } catch (error) {
      if (error?.errorFields) return;
      showError(error?.message || '保存自用渠道失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleConnection = async (row, checked) => {
    try {
      const response = await API.put(
        `/api/v1/public/personal-provider/connections/${row.id}`,
        {
          name: row.name,
          protocol: row.protocol,
          base_url: row.base_url,
          models: row.models,
          priority: row.priority,
          status: checked ? 1 : 2,
        }
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      load().then();
    } catch (error) {
      showError(error?.message || '更新连接状态失败');
    }
  };

  const deleteConnection = async (row) => {
    try {
      const response = await API.delete(
        `/api/v1/public/personal-provider/connections/${row.id}`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('自用渠道已移除');
      load().then();
    } catch (error) {
      showError(error?.message || '移除自用渠道失败');
    }
  };

  const verifyConnection = async (row) => {
    setVerifyingConnectionID(row.id);
    try {
      const response = await API.post(
        `/api/v1/public/personal-provider/connections/${row.id}/verify`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('连接和 API Key 验证通过');
      load().then();
    } catch (error) {
      showError(error?.message || '验证自用渠道失败');
      load().then();
    } finally {
      setVerifyingConnectionID('');
    }
  };

  const saveRoute = async () => {
    try {
      const values = await routeForm.validateFields();
      const response = await API.put(
        '/api/v1/public/personal-provider/model-routes',
        values
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      routeForm.resetFields();
      showSuccess('模型路由规则已保存');
      load().then();
    } catch (error) {
      if (!error?.errorFields) showError(error?.message || '保存模型路由失败');
    }
  };

  const deleteRoute = async (row) => {
    try {
      const response = await API.delete(
        `/api/v1/public/personal-provider/model-routes/${encodeURIComponent(
          row.model
        )}`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('模型路由规则已移除');
      load().then();
    } catch (error) {
      showError(error?.message || '移除模型路由失败');
    }
  };

  const deleteCommunityOfferRoute = async (row) => {
    try {
      const response = await API.delete(
        `/api/v1/public/community-offer-routing/model-routes/${encodeURIComponent(
          row.model
        )}`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('社区报价路由已移除');
      load().then();
    } catch (error) {
      showError(error?.message || '移除社区报价路由失败');
    }
  };

  const routeColumns = useMemo(
    () => [
      { title: '模型', dataIndex: 'model' },
      {
        title: '路由策略',
        dataIndex: 'route_policy',
        render: (value) =>
          POLICY_OPTIONS.find((item) => item.value === value)?.label || value,
      },
      {
        title: '操作',
        width: 96,
        render: (_, row) => (
          <AppButton type='text' danger onClick={() => deleteRoute(row)}>
            移除
          </AppButton>
        ),
      },
    ],
    []
  );
  const communityOfferRouteColumns = useMemo(
    () => [
      { title: '模型', dataIndex: 'model' },
      {
        title: '固定社区报价',
        dataIndex: 'offer_id',
        render: (value) => (
          <span className='personal-routing-models'>{value}</span>
        ),
      },
      {
        title: '操作',
        width: 96,
        render: (_, row) => (
          <AppButton
            type='text'
            danger
            onClick={() => deleteCommunityOfferRoute(row)}
          >
            移除
          </AppButton>
        ),
      },
    ],
    []
  );

  const openSupplyFromConnection = (row) => {
    setSupplySource(row);
    selectTab('supply');
  };

  const renderVerification = (row) => {
    if (!row.last_checked_at) return <AppTag color='grey'>未验证</AppTag>;
    if (row.last_check_ok) return <AppTag color='green'>验证通过</AppTag>;
    return (
      <AppTag color='red' title={row.last_check_error || '验证失败'}>
        验证失败
      </AppTag>
    );
  };

  const renderHealthStrip = (row) => {
    const points = Array.isArray(row.runtime_metrics?.points)
      ? row.runtime_metrics.points
      : [];
    const label = `${row.name}：成功 ${
      row.runtime_metrics?.success_count || 0
    }，失败 ${row.runtime_metrics?.failure_count || 0}`;
    return (
      <div
        className='personal-channel-health-strip'
        role='img'
        aria-label={label}
      >
        {points.map((point, index) => (
          <span
            key={`${point.bucket_start || index}`}
            title={`${point.success_count || 0} 成功 / ${
              point.failure_count || 0
            } 失败`}
            style={{
              background:
                HEALTH_POINT_COLOR[point.state] || HEALTH_POINT_COLOR.unknown,
            }}
          />
        ))}
      </div>
    );
  };

  const providers = (
    <AppSection
      className='personal-routing-section personal-channel-section'
      title='自用渠道诊断'
      extra={
        <div className='personal-channel-section-actions'>
          <AppSegmented
            options={PERIOD_OPTIONS}
            value={period}
            onChange={selectPeriod}
          />
          <AppButton
            color='blue'
            icon={<AppIcon name='plus' />}
            onClick={openCreate}
          >
            添加自用渠道
          </AppButton>
        </div>
      }
    >
      <p className='personal-routing-section-hint'>
        运行健康来自真实调用日志；连接验证只检查地址和凭据，两者互不替代。模型测试和连接验证不会生成“系统任务”。
      </p>
      {loading ? (
        <div className='personal-channel-loading'>正在加载渠道运行数据…</div>
      ) : connections.length === 0 ? (
        <AppEmpty>还没有自用渠道</AppEmpty>
      ) : (
        <div className='personal-channel-diagnostic-list'>
          {connections.map((row) => {
            const metrics = row.runtime_metrics || {};
            const successRate =
              metrics.request_count > 0
                ? `${(Number(metrics.success_rate || 0) * 100).toFixed(1)}%`
                : '-';
            return (
              <article className='personal-channel-diagnostic-row' key={row.id}>
                <div className='personal-channel-identity'>
                  <div className='personal-channel-title'>
                    <strong>{row.name}</strong>
                    <AppTag>
                      {PROTOCOL_OPTIONS.find(
                        (item) => item.value === row.protocol
                      )?.label || row.protocol}
                    </AppTag>
                  </div>
                  <span>{row.base_url || '使用协议官方地址'}</span>
                  <span>
                    {Array.isArray(row.models) ? row.models.join(', ') : '-'}
                  </span>
                </div>
                <div className='personal-channel-observation'>
                  {renderHealthStrip(row)}
                  <div className='personal-channel-metrics'>
                    <div>
                      <span>成功 / 失败</span>
                      <strong>
                        {metrics.success_count || 0} /{' '}
                        {metrics.failure_count || 0}
                      </strong>
                    </div>
                    <div>
                      <span>成功率</span>
                      <strong>{successRate}</strong>
                    </div>
                    <div>
                      <span>平均延迟</span>
                      <strong>
                        {metrics.avg_latency_ms > 0
                          ? `${metrics.avg_latency_ms} ms`
                          : '-'}
                      </strong>
                    </div>
                    <div>
                      <span>调用量</span>
                      <strong>{metrics.request_count || 0}</strong>
                    </div>
                  </div>
                </div>
                <details className='personal-channel-mobile-details'>
                  <summary>查看验证与操作</summary>
                  <div className='personal-channel-secondary'>
                    <div>
                      <span>连接验证</span>
                      {renderVerification(row)}
                      <small>
                        {row.last_checked_at
                          ? timestamp2string(row.last_checked_at)
                          : '尚未验证'}
                      </small>
                    </div>
                    <div>
                      <span>可用性</span>
                      <strong>额度 / 成本不可用</strong>
                      <small>个人凭据不提供账务快照</small>
                    </div>
                  </div>
                </details>
                <div className='personal-channel-secondary personal-channel-secondary-desktop'>
                  <div>
                    <span>连接验证</span>
                    {renderVerification(row)}
                    <small>
                      {row.last_checked_at
                        ? timestamp2string(row.last_checked_at)
                        : '尚未验证'}
                    </small>
                  </div>
                  <div>
                    <span>可用性</span>
                    <strong>额度 / 成本不可用</strong>
                    <small>个人凭据不提供账务快照</small>
                  </div>
                </div>
                <div className='personal-channel-row-actions'>
                  <AppSwitch
                    checked={row.status === 1}
                    onChange={(_, detail) =>
                      toggleConnection(row, detail.checked)
                    }
                  />
                  <AppButton
                    type='text'
                    loading={verifyingConnectionID === row.id}
                    onClick={() => verifyConnection(row)}
                  >
                    验证连接
                  </AppButton>
                  <AppButton type='text' onClick={() => openEdit(row)}>
                    编辑
                  </AppButton>
                  <AppButton
                    type='text'
                    onClick={() => openSupplyFromConnection(row)}
                  >
                    {row.supply ? '查看供给' : '申请参与供给'}
                  </AppButton>
                  <AppButton
                    type='text'
                    danger
                    onClick={() => deleteConnection(row)}
                  >
                    移除
                  </AppButton>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </AppSection>
  );

  const modelRoutes = (
    <div className='personal-routing-stack'>
      <AppSection title='个人来源用量'>
        <div className='personal-routing-quota'>
          <div>
            <strong>{quota?.used_requests ?? 0}</strong>
            <span>{quota?.period || '-'} 已使用请求</span>
          </div>
          <p>当前仅展示个人来源调用次数，不是套餐限额或收费项目。</p>
        </div>
      </AppSection>
      <AppSection title='模型路由规则'>
        <p className='personal-routing-section-hint'>
          模型规则可指定来源优先级或收紧来源范围；API Token
          设置为“仅自用渠道”或“仅供给渠道”时，模型规则不能放宽该限制。
        </p>
        <AppForm
          form={routeForm}
          layout='vertical'
          className='personal-routing-rule-form'
        >
          <AppField label='模型' required>
            <AppForm.Item
              name='model'
              rules={[{ required: true, message: '请选择模型' }]}
              noStyle
            >
              <AppSelect
                options={routeModelOptions}
                search
                placeholder='选择当前账号可用模型'
              />
            </AppForm.Item>
          </AppField>
          <AppField label='策略' required>
            <AppForm.Item
              name='route_policy'
              initialValue='personal_first'
              noStyle
            >
              <AppSelect options={POLICY_OPTIONS} />
            </AppForm.Item>
          </AppField>
          <AppFormActions>
            <AppButton color='blue' onClick={saveRoute}>
              保存规则
            </AppButton>
          </AppFormActions>
        </AppForm>
        <AppTable
          rowKey='model'
          columns={routeColumns}
          dataSource={routes}
          loading={loading}
          pagination={false}
          locale={{
            emptyText: (
              <AppEmpty>未设置模型级规则，将使用 API Token 的默认策略</AppEmpty>
            ),
          }}
        />
      </AppSection>
      <AppSection title='固定模型报价'>
        <p className='personal-routing-section-hint'>
          在“模型报价”中选择报价后，会在这里固定到对应模型。报价属于供给渠道，不是新的渠道类型；固定报价不会自动切换到其他发布者，移除后恢复
          API Token 和模型规则定义的来源策略。
        </p>
        <AppTable
          rowKey='model'
          columns={communityOfferRouteColumns}
          dataSource={communityOfferRoutes}
          loading={loading}
          pagination={false}
          locale={{
            emptyText: <AppEmpty>尚未为任何模型选择报价</AppEmpty>,
          }}
        />
      </AppSection>
    </div>
  );

  const supply = (
    <PublisherWorkspace
      embedded
      sourceConnection={supplySource}
      onSourceConsumed={() => setSupplySource(null)}
    />
  );

  return (
    <div className='dashboard-container personal-routing-page'>
      <header className='personal-routing-header'>
        <div>
          <h1>我的渠道</h1>
          <p>
            这里管理你的自用渠道：连接仅供本人调用；参与供给时会创建凭据独立的供给渠道。平台自营渠道由平台统一运营。
          </p>
        </div>
      </header>
      <AppTabs
        activeKey={activeTab}
        onChange={selectTab}
        items={[
          { key: 'channels', label: '渠道', children: providers },
          { key: 'routes', label: '路由策略', children: modelRoutes },
          { key: 'supply', label: '供给与收益', children: supply },
        ]}
      />
      <AppModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={editing ? '编辑自用渠道' : '添加自用渠道'}
        size='small'
        footer={
          <AppFormActions>
            <AppButton onClick={() => setModalOpen(false)}>取消</AppButton>
            <AppButton color='blue' loading={saving} onClick={saveConnection}>
              保存
            </AppButton>
          </AppFormActions>
        }
      >
        <AppForm form={form} layout='vertical' initialValues={emptyConnection}>
          <AppField label='连接名称' required>
            <AppForm.Item
              name='name'
              rules={[{ required: true, message: '请输入连接名称' }]}
              noStyle
            >
              <AppInput placeholder='例如 我的 OpenAI' />
            </AppForm.Item>
          </AppField>
          <AppField label='上游协议' required>
            <AppForm.Item name='protocol' noStyle>
              <AppSelect options={PROTOCOL_OPTIONS} />
            </AppForm.Item>
          </AppField>
          <AppField
            label='Base URL'
            hint='留空时使用该协议的官方地址；自定义地址仅支持 HTTPS 公网地址'
          >
            <AppForm.Item name='base_url' noStyle>
              <AppInput placeholder='https://api.example.com/v1' />
            </AppForm.Item>
          </AppField>
          <AppField
            label={editing ? '轮换 API Key' : 'API Key'}
            required={!editing}
            hint={
              editing ? '留空则保留当前凭据' : '凭据将加密保存，之后不会再显示'
            }
          >
            <AppForm.Item
              name='api_key'
              rules={
                editing ? [] : [{ required: true, message: '请输入 API Key' }]
              }
              noStyle
            >
              <AppInput type='password' autoComplete='new-password' />
            </AppForm.Item>
          </AppField>
          <AppField
            label='模型范围'
            required
            hint='每行一个模型，也支持逗号分隔'
          >
            <AppForm.Item
              name='models_text'
              rules={[{ required: true, message: '至少填写一个模型' }]}
              noStyle
            >
              <AppTextarea rows={4} placeholder={'gpt-5.1\ngpt-5.1-mini'} />
            </AppForm.Item>
          </AppField>
          <AppField label='优先级'>
            <AppForm.Item name='priority' noStyle>
              <AppInput type='number' />
            </AppForm.Item>
          </AppField>
          <AppField label='启用连接'>
            <AppForm.Item
              name='status'
              valuePropName='checked'
              getValueProps={(value) => ({
                checked: value === 1 || value === true,
              })}
              getValueFromEvent={(_, detail) => detail.checked}
              noStyle
            >
              <AppSwitch />
            </AppForm.Item>
          </AppField>
        </AppForm>
      </AppModal>
    </div>
  );
}

export default PersonalRouting;
