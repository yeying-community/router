/* i18n-skip */
import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { AppButton, AppEmpty, AppInput, AppSection, AppTable, AppTag, AppToolbar } from '../../router-ui';
import { API, showError, showSuccess } from '../../helpers';
import './index.css';

function safePolicy(value) {
  try {
    const parsed = JSON.parse(value || '{}');
    return Object.entries(parsed).map(([key, item]) => `${key}: ${String(item)}`).join(' | ') || '-';
  } catch (_) { return '-'; }
}

function CommunityServices() {
  const [items, setItems] = useState([]);
  const [model, setModel] = useState('');
  const [routes, setRoutes] = useState([]);
  const [selectingOfferID, setSelectingOfferID] = useState('');
  const [loading, setLoading] = useState(true);
  const load = useCallback(async (nextModel = '') => {
    setLoading(true);
    try {
      const [response, routeResponse] = await Promise.all([
        API.get('/api/v1/public/model-services/offers', { params: { model: nextModel || undefined } }),
        API.get('/api/v1/public/community-offer-routing/model-routes'),
      ]);
      if (!response.data?.success) throw new Error(response.data?.message);
      if (!routeResponse.data?.success) throw new Error(routeResponse.data?.message);
      setItems(Array.isArray(response.data?.data) ? response.data.data : []);
      setRoutes(Array.isArray(routeResponse.data?.data) ? routeResponse.data.data : []);
    } catch (error) { showError(error?.message || '加载模型报价失败'); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { load().then(); }, [load]);
  const selectedOfferByModel = useMemo(() => new Map(routes.map((route) => [route.model, route.offer_id])), [routes]);
  const selectOffer = async (row) => {
    setSelectingOfferID(row.offer_id);
    try {
      const response = await API.put('/api/v1/public/community-offer-routing/model-routes', { model: row.model, offer_id: row.offer_id });
      if (!response.data?.success) throw new Error(response.data?.message);
      showSuccess('已将此报价设为该模型的固定来源');
      load(model.trim()).then();
    } catch (error) { showError(error?.message || '选择模型报价失败'); }
    finally { setSelectingOfferID(''); }
  };
  const columns = useMemo(() => [
    { title: '模型', dataIndex: 'model', width: 210 },
    { title: '发布者', dataIndex: 'publisher_display_name', width: 160, render: (value, row) => <div className='community-service-publisher'><strong>{value}</strong><span>{row.publisher_level === 'verified_community' ? '已验证社区发布者' : row.publisher_level}</span></div> },
    { title: '供给渠道', dataIndex: 'service_name', width: 180 },
    { title: '区域', dataIndex: 'region', width: 110, render: (value) => value || '-' },
    { title: '输入价格', dataIndex: 'input_price_micros', width: 150, render: (value) => `${value} 微美元 / 百万 Token` },
    { title: '输出价格', dataIndex: 'output_price_micros', width: 150, render: (value) => `${value} 微美元 / 百万 Token` },
    { title: '数据规则', dataIndex: 'data_policy', render: (value) => <span className='community-service-policy' title={safePolicy(value)}>{safePolicy(value)}</span> },
    { title: '状态', width: 100, render: (_, row) => selectedOfferByModel.get(row.model) === row.offer_id ? <AppTag color='blue'>当前选择</AppTag> : <AppTag color='green'>已审核</AppTag> },
    { title: '操作', width: 132, render: (_, row) => <AppButton type='text' color={selectedOfferByModel.get(row.model) === row.offer_id ? undefined : 'blue'} loading={selectingOfferID === row.offer_id} disabled={selectedOfferByModel.get(row.model) === row.offer_id} onClick={() => selectOffer(row)}>{selectedOfferByModel.get(row.model) === row.offer_id ? '正在使用' : '使用此报价'}</AppButton> },
  ], [selectedOfferByModel, selectingOfferID]);
  return <div className='dashboard-container community-services-page'><AppSection title='模型报价'><AppToolbar className='community-services-toolbar'><AppInput value={model} onChange={(event) => setModel(event.target.value)} placeholder='按模型 ID 筛选' /><AppButton color='blue' onClick={() => load(model.trim())}>查询</AppButton><AppButton onClick={() => { setModel(''); load(''); }}>清除</AppButton></AppToolbar><p className='community-services-hint'>这里展示已审核且健康的供给渠道报价。报价是渠道下的商业对象，不是新的渠道类型；选择后，该模型的调用会固定使用对应供给渠道，不会自动切换到其他发布者。调用按报价以 USD 结算，并从账户余额扣除对应额度。</p><AppTable rowKey='offer_id' columns={columns} dataSource={items} loading={loading} pagination={false} scroll={{ x: 1360 }} locale={{ emptyText: <AppEmpty>当前没有可发现的模型报价</AppEmpty> }} /></AppSection></div>;
}

export default CommunityServices;
