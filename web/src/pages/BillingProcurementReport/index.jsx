import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { API, showError } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppSelect,
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
    sell_base_amount: Number(payload?.sell_base_amount || 0),
    configured_sell_base_amount: Number(payload?.configured_sell_base_amount || 0),
    unconfigured_sell_base_amount: Number(payload?.unconfigured_sell_base_amount || 0),
    procurement_cost_base_amount: Number(payload?.procurement_cost_base_amount || 0),
    gross_profit_base_amount: Number(payload?.gross_profit_base_amount || 0),
    gross_margin: Number(payload?.gross_margin || 0),
    items: items.map((item) => ({
      ...item,
      unconfigured_channels: Array.isArray(item?.unconfigured_channels)
        ? item.unconfigured_channels.map((channel) => ({
            ...channel,
            request_count: Number(channel?.request_count || 0),
            last_request_at: Number(channel?.last_request_at || 0),
          }))
        : [],
      unconfigured_channel_count: Number(item?.unconfigured_channel_count || 0),
      request_count: Number(item?.request_count || 0),
      configured_cost_request_count: Number(item?.configured_cost_request_count || 0),
      unconfigured_cost_request_count: Number(item?.unconfigured_cost_request_count || 0),
      sell_base_amount: Number(item?.sell_base_amount || 0),
      configured_sell_base_amount: Number(item?.configured_sell_base_amount || 0),
      unconfigured_sell_base_amount: Number(item?.unconfigured_sell_base_amount || 0),
      procurement_cost_base_amount: Number(item?.procurement_cost_base_amount || 0),
      gross_profit_base_amount: Number(item?.gross_profit_base_amount || 0),
      gross_margin: Number(item?.gross_margin || 0),
    })),
  };
};

const normalizeHealth = (payload) => ({
  status: payload?.status || 'ok',
  checked_at: Number(payload?.checked_at || 0),
  critical_count: Number(payload?.critical_count || 0),
  warning_count: Number(payload?.warning_count || 0),
  issues: Array.isArray(payload?.issues)
    ? payload.issues.map((item) => ({
        ...item,
        count: Number(item?.count || 0),
      }))
    : [],
});

const channelBillingPath = (channelID) =>
  `/admin/channel/detail/${encodeURIComponent(channelID)}?tab=billing`;

function BillingProcurementReport() {
  const { t } = useTranslation();
  const initialRange = useMemo(() => createLastSevenDaysRange(), []);
  const [groupBy, setGroupBy] = useState('channel');
  const [costScope, setCostScope] = useState('all');
  const [groupID, setGroupID] = useState('');
  const [groupOptions, setGroupOptions] = useState([]);
  const [startAt, setStartAt] = useState(initialRange.startAt);
  const [endAt, setEndAt] = useState(initialRange.endAt);
  const [loading, setLoading] = useState(false);
  const [healthLoading, setHealthLoading] = useState(false);
  const [report, setReport] = useState(() => normalizeReport({}));
  const [health, setHealth] = useState(() => normalizeHealth({}));

  const loadGroups = async () => {
    try {
      const res = await API.get('/api/v1/admin/groups', {
        params: {
          page: 1,
          page_size: 200,
        },
      });
      const { success, data } = res.data || {};
      if (!success) {
        return;
      }
      const items = Array.isArray(data?.items) ? data.items : [];
      setGroupOptions(
        items.map((group) => ({
          key: group.id,
          value: group.id,
          text: group.name || group.id,
        })),
      );
    } catch {
      // Ignore non-critical filter bootstrap failure.
    }
  };

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
          group_id: groupID,
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

  const loadHealth = async () => {
    setHealthLoading(true);
    try {
      const res = await API.get('/api/v1/admin/billing/health');
      const { success, data } = res.data || {};
      if (success) {
        setHealth(normalizeHealth(data));
      }
    } catch {
      setHealth(
        normalizeHealth({
          status: 'warning',
          warning_count: 1,
          issues: [
            {
              key: 'health_load_failed',
              level: 'warning',
              title: t('billing.procurement_report.health.load_failed'),
              message: t('billing.procurement_report.health.load_failed_hint'),
            },
          ],
        }),
      );
    } finally {
      setHealthLoading(false);
    }
  };

  useEffect(() => {
    loadGroups().then();
    loadHealth().then();
  }, []);

  useEffect(() => {
    loadReport().then();
  }, [groupBy, costScope, groupID]);

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
      value: formatCNY(report.sell_base_amount),
      hint: t('billing.procurement_report.summary.sell_amount_hint'),
    },
    {
      key: 'procurement_cost',
      label: t('billing.procurement_report.summary.procurement_cost'),
      value: formatCNY(report.procurement_cost_base_amount),
      hint: t('billing.procurement_report.summary.procurement_cost_hint'),
    },
    {
      key: 'gross_profit',
      label: t('billing.procurement_report.summary.gross_profit'),
      value: formatCNY(report.gross_profit_base_amount),
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
      hint: formatCNY(report.unconfigured_sell_base_amount),
      danger: report.unconfigured_cost_request_count > 0,
    },
  ];

  const renderUnconfiguredChannels = (row) => {
    const channels = Array.isArray(row?.unconfigured_channels)
      ? row.unconfigured_channels
      : [];
    if (channels.length === 0) {
      return '-';
    }
    const total = Number(row?.unconfigured_channel_count || channels.length);
    return (
      <div className='billing-procurement-report-channel-links'>
        {channels.map((channel) => {
          const channelID = (channel?.id || '').toString().trim();
          if (!channelID) {
            return null;
          }
          const label = (channel?.name || channelID).toString().trim();
          return (
            <Link
              key={channelID}
              className='billing-procurement-report-link'
              to={channelBillingPath(channelID)}
              title={t('billing.procurement_report.actions.configure_cost')}
            >
              {label}
            </Link>
          );
        })}
        {total > channels.length ? (
          <AppTag className='router-tag'>
            {t('billing.procurement_report.columns.more_channels', {
              count: total - channels.length,
            })}
          </AppTag>
        ) : null}
      </div>
    );
  };

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
              to={channelBillingPath(key)}
            >
              {label}
            </Link>
          );
        }
        return label;
      },
    },
    ...(groupBy === 'model'
      ? [
          {
            title: t('billing.procurement_report.columns.related_channels'),
            key: 'unconfigured_channels',
            width: 220,
            render: (_, row) => renderUnconfiguredChannels(row),
          },
        ]
      : []),
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
      dataIndex: 'sell_base_amount',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.procurement_cost'),
      dataIndex: 'procurement_cost_base_amount',
      width: 132,
      align: 'right',
      render: formatCNY,
    },
    {
      title: t('billing.procurement_report.columns.gross_profit'),
      dataIndex: 'gross_profit_base_amount',
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

  const healthStatusClass = `is-${health.status || 'ok'}`;
  const healthIssues = health.issues.slice(0, 4);

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
            loading={loading || healthLoading}
            onClick={() => {
              loadHealth().then();
              loadReport().then();
            }}
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
            <AppSelect
              className='router-section-input billing-procurement-report-group-select'
              clearable
              search
              options={groupOptions}
              value={groupID}
              placeholder={t('billing.procurement_report.filters.group')}
              onChange={(e, { value }) => setGroupID((value || '').toString())}
            />
          </div>
        }
      />
      <AppSpin spinning={loading}>
        <AppSection className='billing-procurement-report-section'>
          <div className={`billing-procurement-report-health ${healthStatusClass}`}>
            <div className='billing-procurement-report-health-main'>
              <div className='billing-procurement-report-health-title'>
                {t(`billing.procurement_report.health.status.${health.status || 'ok'}`)}
              </div>
              <div className='billing-procurement-report-health-meta'>
                {t('billing.procurement_report.health.summary', {
                  critical: health.critical_count,
                  warning: health.warning_count,
                })}
              </div>
            </div>
            <div className='billing-procurement-report-health-issues'>
              {healthIssues.length === 0 ? (
                <span className='billing-procurement-report-health-ok'>
                  {t('billing.procurement_report.health.no_issue')}
                </span>
              ) : (
                healthIssues.map((issue) => {
                  const content = (
                    <>
                      <span className={`billing-procurement-report-health-level is-${issue.level || 'warning'}`}>
                        {t(`billing.procurement_report.health.level.${issue.level || 'warning'}`)}
                      </span>
                      <span className='billing-procurement-report-health-text'>
                        {issue.title}
                        {issue.count > 0 ? ` (${formatCount(issue.count)})` : ''}
                      </span>
                    </>
                  );
                  return issue.link ? (
                    <Link
                      key={issue.key}
                      className='billing-procurement-report-health-issue'
                      to={issue.link}
                      title={issue.message}
                    >
                      {content}
                    </Link>
                  ) : (
                    <span
                      key={issue.key}
                      className='billing-procurement-report-health-issue'
                      title={issue.message}
                    >
                      {content}
                    </span>
                  );
                })
              )}
            </div>
          </div>
        </AppSection>
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
            scroll={{ x: groupBy === 'model' ? 1320 : 1100 }}
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
