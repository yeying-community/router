import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppSegmented,
  AppSection,
  AppSpin,
  AppTable,
  AppTag,
} from '../../router-ui';
import './BillingProcurementReport.css';

const GROUP_BY_OPTIONS = [
  { label: '按渠道', value: 'channel' },
  { label: '按模型', value: 'model' },
];

const COST_SCOPE_OPTIONS = [
  { label: '全部请求', value: 'all' },
  { label: '仅未配置成本', value: 'unconfigured' },
];

const toDateTimeLocalValue = (date) => {
  const pad = (value) => String(value).padStart(2, '0');
  return [
    date.getFullYear(),
    '-',
    pad(date.getMonth() + 1),
    '-',
    pad(date.getDate()),
    'T',
    pad(date.getHours()),
    ':',
    pad(date.getMinutes()),
  ].join('');
};

const timestampFromDateTimeLocal = (value) => {
  const date = new Date(value || '');
  const timestamp = Math.floor(date.getTime() / 1000);
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp : 0;
};

const createLastSevenDaysRange = () => {
  const end = new Date();
  const start = new Date(end.getTime() - 7 * 24 * 60 * 60 * 1000);
  return {
    startAt: toDateTimeLocalValue(start),
    endAt: toDateTimeLocalValue(end),
  };
};

const formatCNY = (value) => `¥${formatDecimalNumber(value || 0, 4)}`;
const formatCount = (value) => formatDecimalNumber(value || 0, 0);
const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;

const normalizeReport = (payload) => {
  const items = Array.isArray(payload?.items) ? payload.items : [];
  return {
    group_by: payload?.group_by || 'channel',
    request_count: Number(payload?.request_count || 0),
    configured_cost_request_count: Number(payload?.configured_cost_request_count || 0),
    unconfigured_cost_request_count: Number(payload?.unconfigured_cost_request_count || 0),
    sell_amount_cny: Number(payload?.sell_amount_cny || 0),
    configured_sell_amount_cny: Number(payload?.configured_sell_amount_cny || 0),
    unconfigured_sell_amount_cny: Number(payload?.unconfigured_sell_amount_cny || 0),
    procurement_cost_cny: Number(payload?.procurement_cost_cny || 0),
    gross_profit_cny: Number(payload?.gross_profit_cny || 0),
    gross_margin: Number(payload?.gross_margin || 0),
    items: items.map((item) => ({
      ...item,
      request_count: Number(item?.request_count || 0),
      configured_cost_request_count: Number(item?.configured_cost_request_count || 0),
      unconfigured_cost_request_count: Number(item?.unconfigured_cost_request_count || 0),
      sell_amount_cny: Number(item?.sell_amount_cny || 0),
      configured_sell_amount_cny: Number(item?.configured_sell_amount_cny || 0),
      unconfigured_sell_amount_cny: Number(item?.unconfigured_sell_amount_cny || 0),
      procurement_cost_cny: Number(item?.procurement_cost_cny || 0),
      gross_profit_cny: Number(item?.gross_profit_cny || 0),
      gross_margin: Number(item?.gross_margin || 0),
    })),
  };
};

function BillingProcurementReport() {
  const { t } = useTranslation();
  const initialRange = useMemo(() => createLastSevenDaysRange(), []);
  const [groupBy, setGroupBy] = useState('channel');
  const [costScope, setCostScope] = useState('all');
  const [startAt, setStartAt] = useState(initialRange.startAt);
  const [endAt, setEndAt] = useState(initialRange.endAt);
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState(() => normalizeReport({}));

  const loadReport = async () => {
    const startTimestamp = timestampFromDateTimeLocal(startAt);
    const endTimestamp = timestampFromDateTimeLocal(endAt);
    if (!startTimestamp || !endTimestamp || endTimestamp < startTimestamp) {
      showError(t('billing.procurement_report.messages.invalid_time'));
      return;
    }
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/billing/procurement-report', {
        params: {
          start_at: startTimestamp,
          end_at: endTimestamp,
          group_by: groupBy,
          cost_scope: costScope,
        },
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('billing.procurement_report.messages.load_failed'));
        return;
      }
      setReport(normalizeReport(data));
    } catch (error) {
      showError(error?.message || t('billing.procurement_report.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadReport().then();
  }, [groupBy, costScope]);

  const summaryItems = [
    {
      key: 'request_count',
      label: t('billing.procurement_report.summary.request_count'),
      value: formatCount(report.request_count),
      hint: t('billing.procurement_report.summary.request_count_hint'),
    },
    {
      key: 'sell_amount',
      label: t('billing.procurement_report.summary.sell_amount'),
      value: formatCNY(report.sell_amount_cny),
      hint: t('billing.procurement_report.summary.sell_amount_hint'),
    },
    {
      key: 'procurement_cost',
      label: t('billing.procurement_report.summary.procurement_cost'),
      value: formatCNY(report.procurement_cost_cny),
      hint: t('billing.procurement_report.summary.procurement_cost_hint'),
    },
    {
      key: 'gross_profit',
      label: t('billing.procurement_report.summary.gross_profit'),
      value: formatCNY(report.gross_profit_cny),
      hint: t('billing.procurement_report.summary.gross_profit_hint'),
    },
    {
      key: 'gross_margin',
      label: t('billing.procurement_report.summary.gross_margin'),
      value: formatPercent(report.gross_margin),
      hint: t('billing.procurement_report.summary.gross_margin_hint'),
    },
    {
      key: 'unconfigured',
      label: t('billing.procurement_report.summary.unconfigured'),
      value: formatCount(report.unconfigured_cost_request_count),
      hint: formatCNY(report.unconfigured_sell_amount_cny),
      danger: report.unconfigured_cost_request_count > 0,
    },
  ];

  const columns = [
    {
      title:
        groupBy === 'model'
          ? t('billing.procurement_report.columns.model')
          : t('billing.procurement_report.columns.channel'),
      key: 'dimension',
      width: 240,
      render: (_, row) => {
        const label = row.dimension_name || row.dimension_key || '-';
        const key = (row.dimension_key || '').toString().trim();
        if (groupBy === 'channel' && key && key !== '-') {
          return (
            <Link
              className='billing-procurement-report-link'
              to={`/admin/channel/detail/${encodeURIComponent(key)}?tab=billing`}
            >
              {label}
            </Link>
          );
        }
        return label;
      },
    },
    {
      title: t('billing.procurement_report.columns.request_count'),
      dataIndex: 'request_count',
      width: 100,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.procurement_report.columns.configured_count'),
      dataIndex: 'configured_cost_request_count',
      width: 132,
      align: 'right',
      render: formatCount,
    },
    {
      title: t('billing.procurement_report.columns.unconfigured_count'),
      dataIndex: 'unconfigured_cost_request_count',
      width: 132,
      align: 'right',
      render: (value) =>
        Number(value || 0) > 0 ? (
          <AppTag color='orange'>{formatCount(value)}</AppTag>
        ) : (
          '-'
        ),
    },
    {
      title: t('billing.procurement_report.columns.sell_amount'),
      dataIndex: 'sell_amount_cny',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.procurement_cost'),
      dataIndex: 'procurement_cost_cny',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.gross_profit'),
      dataIndex: 'gross_profit_cny',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.gross_margin'),
      dataIndex: 'gross_margin',
      width: 100,
      align: 'right',
      render: formatPercent,
    },
  ];

  return (
    <div className='dashboard-container billing-procurement-report-page'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'operation', label: t('header.platform_operation') },
          {
            key: 'procurement-report',
            label: t('billing.procurement_report.title'),
            active: true,
          },
        ]}
        title={t('billing.procurement_report.title')}
        actions={
          <AppButton
            className='router-page-button'
            color='blue'
            loading={loading}
            onClick={() => loadReport()}
          >
            {t('common.refresh')}
          </AppButton>
        }
        query={
          <div className='billing-procurement-report-filters'>
            <AppSegmented
              className='billing-procurement-report-segmented'
              options={GROUP_BY_OPTIONS.map((item) => ({
                ...item,
                label: t(`billing.procurement_report.group_by.${item.value}`),
              }))}
              value={groupBy}
              onChange={(e, { value }) => setGroupBy(value)}
            />
            <AppSegmented
              className='billing-procurement-report-segmented'
              options={COST_SCOPE_OPTIONS.map((item) => ({
                ...item,
                label: t(`billing.procurement_report.cost_scope.${item.value}`),
              }))}
              value={costScope}
              onChange={(e, { value }) => setCostScope(value)}
            />
            <AppInput
              className='router-section-input billing-procurement-report-time-input'
              type='datetime-local'
              value={startAt}
              onChange={(e, { value }) => setStartAt(value)}
            />
            <AppInput
              className='router-section-input billing-procurement-report-time-input'
              type='datetime-local'
              value={endAt}
              onChange={(e, { value }) => setEndAt(value)}
            />
          </div>
        }
      />
      <AppSpin spinning={loading}>
        <AppSection className='billing-procurement-report-section'>
          <div className='billing-procurement-report-summary-grid'>
            {summaryItems.map((item) => (
              <div
                key={item.key}
                className={[
                  'billing-procurement-report-summary-card',
                  item.danger ? 'is-danger' : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
              >
                <div className='billing-procurement-report-summary-label'>
                  {item.label}
                </div>
                <div className='billing-procurement-report-summary-value'>
                  {item.value}
                </div>
                <div className='billing-procurement-report-summary-hint'>
                  {item.hint}
                </div>
              </div>
            ))}
          </div>
          <AppTable
            className='router-detail-table router-table-fit-page billing-procurement-report-table'
            rowKey={(row) => `${row.dimension_type}-${row.dimension_key}`}
            dataSource={report.items}
            columns={columns}
            pagination={false}
            scroll={{ x: 1100 }}
            locale={{
              emptyText: loading
                ? t('common.loading')
                : t('billing.procurement_report.empty'),
            }}
          />
        </AppSection>
      </AppSpin>
    </div>
  );
}

export default BillingProcurementReport;
