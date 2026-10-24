/* i18n-skip */
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AppButton,
  AppEmpty,
  AppField,
  AppForm,
  AppFormActions,
  AppInputNumber,
  AppModal,
  AppSection,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTabs,
  AppTag,
  AppTextarea,
} from '../../router-ui';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import './index.css';

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
  reviewing: '待审核',
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
const statusTag = (value) => (
  <AppTag color={STATUS_COLOR[value] || 'grey'}>
    {STATUS_LABEL[value] || value || '-'}
  </AppTag>
);
const DELIVERY_STATUS = {
  prepared: ['orange', '待确认'],
  ready: ['blue', '待投递'],
  delivered: ['green', '已投递'],
  cancelled: ['grey', '已取消'],
  exception: ['red', '异常待处置'],
  resolved: ['green', '已人工处理'],
};
const deliveryStatusTag = (value) => {
  const [color, label] = DELIVERY_STATUS[value] || ['grey', value || '-'];
  return <AppTag color={color}>{label}</AppTag>;
};

function PublisherOperations({ embedded = false }) {
  const [publishers, setPublishers] = useState([]);
  const [services, setServices] = useState([]);
  const [offers, setOffers] = useState([]);
  const [settlements, setSettlements] = useState([]);
  const [deliveries, setDeliveries] = useState([]);
  const [groups, setGroups] = useState([]);
  const [selectedGroupID, setSelectedGroupID] = useState('');
  const [groupSupplies, setGroupSupplies] = useState([]);
  const [groupSupplyLoading, setGroupSupplyLoading] = useState(false);
  const [groupSupplySaving, setGroupSupplySaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [reviewing, setReviewing] = useState(null);
  const [saving, setSaving] = useState(false);
  const [retryingDeliveryID, setRetryingDeliveryID] = useState('');
  const [cancellingDelivery, setCancellingDelivery] = useState(null);
  const [cancelling, setCancelling] = useState(false);
  const [resolvingDelivery, setResolvingDelivery] = useState(null);
  const [resolving, setResolving] = useState(false);
  const [form] = AppForm.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [
        publisherResponse,
        serviceResponse,
        offerResponse,
        settlementResponse,
        deliveryResponse,
        groupResponse,
      ] = await Promise.all([
        API.get('/api/v1/admin/publishers/'),
        API.get('/api/v1/admin/publisher-services/'),
        API.get('/api/v1/admin/offers/review'),
        API.get('/api/v1/admin/publisher-settlements/'),
        API.get('/api/v1/admin/publisher-settlements/deliveries'),
        API.get('/api/v1/admin/groups', {
          params: { page: 1, page_size: 100 },
        }),
      ]);
      for (const response of [
        publisherResponse,
        serviceResponse,
        offerResponse,
        settlementResponse,
        deliveryResponse,
        groupResponse,
      ])
        if (!response.data?.success) throw new Error(response.data?.message);
      setPublishers(
        Array.isArray(publisherResponse.data?.data)
          ? publisherResponse.data.data
          : []
      );
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
      setDeliveries(
        Array.isArray(deliveryResponse.data?.data)
          ? deliveryResponse.data.data
          : []
      );
      const nextGroups = Array.isArray(groupResponse.data?.data?.items)
        ? groupResponse.data.data.items
        : [];
      setGroups(nextGroups);
      setSelectedGroupID((current) => current || nextGroups[0]?.id || '');
    } catch (error) {
      showError(error?.message || '加载发布审核失败');
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    load().then();
  }, [load]);
  const loadGroupSupplies = useCallback(async (groupID) => {
    if (!groupID) {
      setGroupSupplies([]);
      return;
    }
    setGroupSupplyLoading(true);
    try {
      const response = await API.get(`/api/v1/admin/group/${groupID}/supplies`);
      if (!response.data?.success) throw new Error(response.data?.message);
      setGroupSupplies(
        Array.isArray(response.data?.data) ? response.data.data : []
      );
    } catch (error) {
      showError(error?.message || '加载分组供给失败');
    } finally {
      setGroupSupplyLoading(false);
    }
  }, []);
  useEffect(() => {
    loadGroupSupplies(selectedGroupID).then();
  }, [loadGroupSupplies, selectedGroupID]);
  const updateGroupSupply = (offerID, field, value) =>
    setGroupSupplies((rows) =>
      rows.map((row) =>
        row.offer_id === offerID ? { ...row, [field]: value } : row
      )
    );
  const saveGroupSupplies = async () => {
    if (!selectedGroupID) return;
    setGroupSupplySaving(true);
    try {
      const response = await API.put(
        `/api/v1/admin/group/${selectedGroupID}/supplies`,
        { supplies: groupSupplies }
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      setGroupSupplies(
        Array.isArray(response.data?.data) ? response.data.data : []
      );
      showSuccess('分组供给已保存');
    } catch (error) {
      showError(error?.message || '保存分组供给失败');
    } finally {
      setGroupSupplySaving(false);
    }
  };
  const openReview = (kind, row, approve) => {
    setReviewing({ kind, row, approve });
    form.resetFields();
  };
  const submitReview = async () => {
    try {
      const values = await form.validateFields();
      const { kind, row, approve } = reviewing;
      let url = '';
      if (kind === 'publisher')
        url = `/api/v1/admin/publishers/${row.id}/${
          approve ? 'approve' : 'restrict'
        }`;
      if (kind === 'offer')
        url = `/api/v1/admin/offers/${row.id}/${
          approve ? 'approve' : 'reject'
        }`;
      if (kind === 'service')
        url = `/api/v1/admin/publisher-services/${row.id}/suspend`;
      setSaving(true);
      const response = await API.post(url, values);
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('审核操作已保存');
      setReviewing(null);
      load().then();
    } catch (error) {
      if (!error?.errorFields) showError(error?.message || '审核操作失败');
    } finally {
      setSaving(false);
    }
  };
  const retryDelivery = async (row) => {
    setRetryingDeliveryID(row.request_log_id);
    try {
      const response = await API.post(
        `/api/v1/admin/publisher-settlements/deliveries/${row.request_log_id}/retry`
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('结算投递已完成');
      load().then();
    } catch (error) {
      showError(error?.message || '结算投递尚未满足完成条件');
    } finally {
      setRetryingDeliveryID('');
    }
  };
  const openDeliveryCancellation = (row) => {
    setCancellingDelivery(row);
    form.resetFields();
  };
  const submitDeliveryCancellation = async () => {
    try {
      const values = await form.validateFields();
      setCancelling(true);
      const response = await API.post(
        `/api/v1/admin/publisher-settlements/deliveries/${cancellingDelivery.request_log_id}/cancel`,
        values
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('结算投递已取消并退款');
      setCancellingDelivery(null);
      load().then();
    } catch (error) {
      if (!error?.errorFields) showError(error?.message || '取消结算投递失败');
    } finally {
      setCancelling(false);
    }
  };
  const openDeliveryResolution = (row) => {
    setResolvingDelivery(row);
    form.resetFields();
  };
  const submitDeliveryResolution = async () => {
    try {
      const values = await form.validateFields();
      setResolving(true);
      const response = await API.post(
        `/api/v1/admin/publisher-settlements/deliveries/${resolvingDelivery.request_log_id}/resolve`,
        values
      );
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('异常处置已记录');
      setResolvingDelivery(null);
      load().then();
    } catch (error) {
      if (!error?.errorFields) showError(error?.message || '确认异常处置失败');
    } finally {
      setResolving(false);
    }
  };
  const publisherColumns = useMemo(
    () => [
      {
        title: '发布者',
        dataIndex: 'display_name',
        render: (value, row) => (
          <div className='publisher-operations-name'>
            <strong>{value}</strong>
            <span>{row.wallet_identity_did}</span>
          </div>
        ),
      },
      { title: '联系邮箱', dataIndex: 'contact_email' },
      { title: '状态', dataIndex: 'status', width: 96, render: statusTag },
      {
        title: '提交时间',
        dataIndex: 'submitted_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: '操作',
        width: 150,
        render: (_, row) => (
          <div className='publisher-operations-actions'>
            {row.status === 'reviewing' && (
              <AppButton
                type='text'
                onClick={() => openReview('publisher', row, true)}
              >
                通过
              </AppButton>
            )}
            {(row.status === 'reviewing' || row.status === 'active') && (
              <AppButton
                type='text'
                danger
                onClick={() => openReview('publisher', row, false)}
              >
                限制
              </AppButton>
            )}
          </div>
        ),
      },
    ],
    []
  );
  const serviceColumns = useMemo(
    () => [
      {
        title: '供给渠道',
        dataIndex: 'name',
        render: (value, row) => (
          <div className='publisher-operations-name'>
            <strong>{value}</strong>
            <span>{row.base_url}</span>
          </div>
        ),
      },
      { title: '发布者 ID', dataIndex: 'publisher_id' },
      {
        title: '验证状态',
        render: (_, row) => statusTag(row.last_check_ok ? 'ready' : row.status),
      },
      {
        title: '最近验证',
        dataIndex: 'last_checked_at',
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: '操作',
        width: 110,
        render: (_, row) =>
          row.status !== 'suspended' && (
            <AppButton
              type='text'
              danger
              onClick={() => openReview('service', row, false)}
            >
              暂停
            </AppButton>
          ),
      },
    ],
    []
  );
  const offerColumns = useMemo(
    () => [
      { title: '模型', dataIndex: 'model' },
      { title: '发布者 ID', dataIndex: 'publisher_id' },
      { title: '供给渠道 ID', dataIndex: 'service_id' },
      {
        title: '价格',
        render: (_, row) =>
          `${row.input_price_micros} / ${row.output_price_micros} 微美元 / 百万 Token`,
      },
      { title: '状态', dataIndex: 'status', width: 96, render: statusTag },
      {
        title: '操作',
        width: 150,
        render: (_, row) =>
          row.status === 'reviewing' && (
            <div className='publisher-operations-actions'>
              <AppButton
                type='text'
                onClick={() => openReview('offer', row, true)}
              >
                发布
              </AppButton>
              <AppButton
                type='text'
                danger
                onClick={() => openReview('offer', row, false)}
              >
                驳回
              </AppButton>
            </div>
          ),
      },
    ],
    []
  );
  const settlementColumns = useMemo(
    () => [
      {
        title: '请求',
        dataIndex: 'request_log_id',
        render: (value) => (
          <span className='publisher-operations-monospace'>{value}</span>
        ),
      },
      { title: '发布者', dataIndex: 'publisher_display_name' },
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
    ],
    []
  );
  const deliveryColumns = useMemo(
    () => [
      {
        title: '请求',
        dataIndex: 'request_log_id',
        render: (value) => (
          <span className='publisher-operations-monospace'>{value}</span>
        ),
      },
      {
        title: '报价',
        dataIndex: 'offer_id',
        render: (value) => (
          <span className='publisher-operations-monospace'>{value}</span>
        ),
      },
      {
        title: '消费者',
        dataIndex: 'consumer_user_id',
        render: (value) => (
          <span className='publisher-operations-monospace'>{value}</span>
        ),
      },
      {
        title: '用量',
        render: (_, row) => `${row.input_tokens} / ${row.output_tokens} Token`,
      },
      {
        title: '扣费 / 退款',
        render: (_, row) =>
          row.charge_recorded
            ? `${row.charged_quota} / ${row.refunded_quota} 额度`
            : `待确认 (${row.planned_quota || 0} 额度)`,
      },
      {
        title: '状态',
        dataIndex: 'status',
        width: 96,
        render: deliveryStatusTag,
      },
      { title: '尝试', dataIndex: 'attempts', width: 72, align: 'right' },
      {
        title: '最后错误',
        dataIndex: 'last_error',
        render: (value) => <span title={value || ''}>{value || '-'}</span>,
      },
      {
        title: '异常 / 结论',
        render: (_, row) => (
          <span title={row.resolution || row.exception_reason || ''}>
            {row.resolution || row.exception_reason || '-'}
          </span>
        ),
      },
      {
        title: '操作',
        width: 180,
        render: (_, row) => (
          <div className='publisher-operations-actions'>
            {row.status !== 'delivered' &&
              row.status !== 'cancelled' &&
              row.status !== 'exception' &&
              row.status !== 'resolved' && (
                <AppButton
                  type='text'
                  color='blue'
                  loading={retryingDeliveryID === row.request_log_id}
                  onClick={() => retryDelivery(row)}
                >
                  重试投递
                </AppButton>
              )}
            {row.status === 'prepared' && row.charge_recorded && (
              <AppButton
                type='text'
                danger
                onClick={() => openDeliveryCancellation(row)}
              >
                取消并退款
              </AppButton>
            )}
            {row.status === 'exception' && (
              <AppButton
                type='text'
                danger
                onClick={() => openDeliveryResolution(row)}
              >
                确认处置
              </AppButton>
            )}
          </div>
        ),
      },
    ],
    [retryingDeliveryID]
  );
  const groupSupplyColumns = useMemo(
    () => [
      {
        title: '加入分组',
        dataIndex: 'bound',
        width: 100,
        render: (value, row) => (
          <AppSwitch
            checked={Boolean(value)}
            onChange={(_, detail) =>
              updateGroupSupply(row.offer_id, 'bound', detail.checked)
            }
          />
        ),
      },
      { title: '模型', dataIndex: 'model', width: 190 },
      {
        title: '发布者 / 服务',
        render: (_, row) => (
          <div className='publisher-operations-name'>
            <strong>{row.publisher_display_name || '-'}</strong>
            <span>{row.service_name || row.offer_id}</span>
          </div>
        ),
      },
      {
        title: '区域',
        dataIndex: 'region',
        width: 110,
        render: (value) => value || '-',
      },
      {
        title: '优先级',
        dataIndex: 'priority',
        width: 120,
        render: (value, row) => (
          <AppInputNumber
            min={0}
            value={Number(value || 0)}
            disabled={!row.bound}
            onChange={(_, detail) =>
              updateGroupSupply(
                row.offer_id,
                'priority',
                Number(detail.value || 0)
              )
            }
          />
        ),
      },
      {
        title: '启用',
        dataIndex: 'enabled',
        width: 90,
        render: (value, row) => (
          <AppSwitch
            checked={Boolean(value)}
            disabled={!row.bound}
            onChange={(_, detail) =>
              updateGroupSupply(row.offer_id, 'enabled', detail.checked)
            }
          />
        ),
      },
    ],
    []
  );
  const needsNote =
    reviewing && (!reviewing.approve || reviewing.kind === 'service');
  const actionLabel =
    reviewing?.kind === 'service'
      ? '暂停服务'
      : reviewing?.approve
      ? reviewing.kind === 'offer'
        ? '发布报价'
        : '通过发布者'
      : reviewing?.kind === 'offer'
      ? '驳回报价'
      : '限制发布者';
  return (
    <div
      className={`${
        embedded ? '' : 'dashboard-container '
      }publisher-operations-page${
        embedded ? ' publisher-operations-embedded' : ''
      }`}
    >
      <AppTabs
        items={[
          {
            key: 'group-supplies',
            label: '分组供给',
            children: (
              <AppSection
                title='分组供给'
                extra={
                  <AppButton
                    color='blue'
                    loading={groupSupplySaving}
                    disabled={!selectedGroupID}
                    onClick={saveGroupSupplies}
                  >
                    保存分组供给
                  </AppButton>
                }
              >
                <div className='publisher-operations-group-toolbar'>
                  <AppField label='运营分组'>
                    <AppSelect
                      value={selectedGroupID}
                      options={groups.map((group) => ({
                        value: group.id,
                        label: group.name,
                      }))}
                      onChange={(_, detail) => setSelectedGroupID(detail.value)}
                      placeholder='选择分组'
                    />
                  </AppField>
                  <p>
                    模型报价通过独立绑定参与自动路由。绑定只决定路由范围，不会把供给渠道变成平台自营渠道；分组只能由运营管理。
                  </p>
                </div>
                <AppTable
                  rowKey='offer_id'
                  columns={groupSupplyColumns}
                  dataSource={groupSupplies}
                  loading={groupSupplyLoading}
                  pagination={false}
                  scroll={{ x: 980 }}
                  locale={{
                    emptyText: <AppEmpty>当前没有可绑定的已发布报价</AppEmpty>,
                  }}
                />
              </AppSection>
            ),
          },
          {
            key: 'publishers',
            label: `发布者${
              publishers.filter((item) => item.status === 'reviewing').length
                ? ` (${
                    publishers.filter((item) => item.status === 'reviewing')
                      .length
                  })`
                : ''
            }`,
            children: (
              <AppSection title='发布者审核'>
                <AppTable
                  rowKey='id'
                  columns={publisherColumns}
                  dataSource={publishers}
                  loading={loading}
                  pagination={false}
                  scroll={{ x: 960 }}
                  locale={{ emptyText: <AppEmpty>没有发布者申请</AppEmpty> }}
                />
              </AppSection>
            ),
          },
          {
            key: 'services',
            label: '供给渠道',
            children: (
              <AppSection title='独立运营供给渠道治理'>
                <AppTable
                  rowKey='id'
                  columns={serviceColumns}
                  dataSource={services}
                  loading={loading}
                  pagination={false}
                  scroll={{ x: 960 }}
                  locale={{
                    emptyText: <AppEmpty>没有独立运营供给渠道</AppEmpty>,
                  }}
                />
              </AppSection>
            ),
          },
          {
            key: 'offers',
            label: `报价审核${
              offers.filter((item) => item.status === 'reviewing').length
                ? ` (${
                    offers.filter((item) => item.status === 'reviewing').length
                  })`
                : ''
            }`,
            children: (
              <AppSection title='可售报价审核'>
                <AppTable
                  rowKey='id'
                  columns={offerColumns}
                  dataSource={offers}
                  loading={loading}
                  pagination={false}
                  scroll={{ x: 1080 }}
                  locale={{ emptyText: <AppEmpty>没有待审核报价</AppEmpty> }}
                />
              </AppSection>
            ),
          },
          {
            key: 'settlements',
            label: '结算账本',
            children: (
              <AppSection title='供给结算账本'>
                <AppTable
                  rowKey='request_log_id'
                  columns={settlementColumns}
                  dataSource={settlements}
                  loading={loading}
                  pagination={false}
                  scroll={{ x: 1200 }}
                  locale={{
                    emptyText: <AppEmpty>还没有供给结算记录</AppEmpty>,
                  }}
                />
              </AppSection>
            ),
          },
          {
            key: 'deliveries',
            label: `结算投递${
              deliveries.filter(
                (item) =>
                  !['delivered', 'cancelled', 'resolved'].includes(item.status)
              ).length
                ? ` (${
                    deliveries.filter(
                      (item) =>
                        !['delivered', 'cancelled', 'resolved'].includes(
                          item.status
                        )
                    ).length
                  })`
                : ''
            }`,
            children: (
              <AppSection title='跨库结算投递'>
                <AppTable
                  rowKey='request_log_id'
                  columns={deliveryColumns}
                  dataSource={deliveries}
                  loading={loading}
                  pagination={false}
                  scroll={{ x: 1740 }}
                  locale={{
                    emptyText: <AppEmpty>没有待处理的结算投递</AppEmpty>,
                  }}
                />
              </AppSection>
            ),
          },
        ]}
      />
      <AppModal
        open={Boolean(reviewing)}
        onClose={() => setReviewing(null)}
        title={actionLabel}
        size='small'
        footer={
          <AppFormActions>
            <AppButton onClick={() => setReviewing(null)}>取消</AppButton>
            <AppButton
              color={reviewing?.approve ? 'blue' : undefined}
              danger={!reviewing?.approve}
              loading={saving}
              onClick={submitReview}
            >
              确认
            </AppButton>
          </AppFormActions>
        }
      >
        <AppForm form={form} layout='vertical'>
          <AppField
            label='审核说明'
            required={needsNote}
            hint={needsNote ? '该操作必须记录原因' : '可选，说明本次审核决定'}
          >
            <AppForm.Item
              name='note'
              rules={
                needsNote ? [{ required: true, message: '请填写审核说明' }] : []
              }
              noStyle
            >
              <AppTextarea rows={4} />
            </AppForm.Item>
          </AppField>
        </AppForm>
      </AppModal>
      <AppModal
        open={Boolean(cancellingDelivery)}
        onClose={() => setCancellingDelivery(null)}
        title='取消并退款'
        size='small'
        footer={
          <AppFormActions>
            <AppButton onClick={() => setCancellingDelivery(null)}>
              返回
            </AppButton>
            <AppButton
              danger
              loading={cancelling}
              onClick={submitDeliveryCancellation}
            >
              确认退款
            </AppButton>
          </AppFormActions>
        }
      >
        <AppForm form={form} layout='vertical'>
          <AppField label='取消原因' required>
            <AppForm.Item
              name='reason'
              rules={[{ required: true, message: '请填写取消原因' }]}
              noStyle
            >
              <AppTextarea rows={4} />
            </AppForm.Item>
          </AppField>
        </AppForm>
      </AppModal>
      <AppModal
        open={Boolean(resolvingDelivery)}
        onClose={() => setResolvingDelivery(null)}
        title='确认异常处置'
        size='small'
        footer={
          <AppFormActions>
            <AppButton onClick={() => setResolvingDelivery(null)}>
              返回
            </AppButton>
            <AppButton
              danger
              loading={resolving}
              onClick={submitDeliveryResolution}
            >
              确认处置
            </AppButton>
          </AppFormActions>
        }
      >
        <AppForm form={form} layout='vertical'>
          <AppField label='处理结论' required>
            <AppForm.Item
              name='resolution'
              rules={[{ required: true, message: '请填写处理结论' }]}
              noStyle
            >
              <AppTextarea rows={4} />
            </AppForm.Item>
          </AppField>
        </AppForm>
      </AppModal>
    </div>
  );
}

export default PublisherOperations;
