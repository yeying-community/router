import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import {
  AppAlert,
  AppButton,
  AppEmpty,
  AppErrorState,
  AppFilterHeader,
  AppIcon,
  AppInput,
  AppSegmented,
  AppSelect,
  AppSpin,
  AppTag,
  AppTextarea,
  AppTooltip,
} from '../../router-ui';
import { API, showError, showSuccess } from '../../helpers';
import { formatUpdatedAt } from './dashboardShared';
import './Dashboard.css';
import './AdminDashboard.css';
import './DutyStation.css';

const DUTY_WINDOWS = ['1h', '6h', '24h'];
const WORK_VIEWS = [
  'attention',
  'objects',
  'incidents',
  'observation',
  'resolved',
];
const SORT_MODES = ['priority', 'recent'];
const OPEN_STATUSES = new Set([
  'active',
  'acknowledged',
  'investigating',
  'mitigated',
]);
const NEXT_STATUS = {
  active: 'acknowledged',
  acknowledged: 'investigating',
  investigating: 'mitigated',
  mitigated: 'resolved',
  resolved: 'active',
};

const EMPTY_DUTY_DASHBOARD = {
  overall_status: 'unknown',
  window: '6h',
  generated_at: 0,
  summary: {
    active_incidents: 0,
    critical_incidents: 0,
    warning_incidents: 0,
    unassigned_incidents: 0,
    affected_models: 0,
    failure_count: 0,
    fallback_count: 0,
    failed_task_count: 0,
    finance_issue_count: 0,
  },
  sources: [],
  inspection: {
    health_status: 'unknown',
    observation_status: 'insufficient',
    coverage_rate: 0,
    abnormal_object_count: 0,
    unknown_object_count: 0,
    finding_count: 0,
    domains: [],
    findings: [],
  },
  incidents: [],
};

function toPlainDutyText(value) {
  const text = String(value || '');
  if (text === '') return '';
  return text
    .replace(/<br\s*\/?\s*>/gi, ' ')
    .replace(/<\/(p|div|li|h[1-6])\s*>/gi, ' ')
    .replace(/<[^>]*>/g, ' ')
    .replace(/&nbsp;/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'")
    .replace(/\s+/g, ' ')
    .trim();
}

const normalizeDutyDashboard = (payload) => ({
  ...EMPTY_DUTY_DASHBOARD,
  ...(payload || {}),
  summary: { ...EMPTY_DUTY_DASHBOARD.summary, ...(payload?.summary || {}) },
  sources: Array.isArray(payload?.sources) ? payload.sources : [],
  inspection: {
    ...EMPTY_DUTY_DASHBOARD.inspection,
    ...(payload?.inspection || {}),
    domains: Array.isArray(payload?.inspection?.domains)
      ? payload.inspection.domains
      : [],
    findings: Array.isArray(payload?.inspection?.findings)
      ? payload.inspection.findings
      : [],
  },
  incidents: Array.isArray(payload?.incidents)
    ? payload.incidents.map((incident) => ({
        ...incident,
        title: toPlainDutyText(incident?.title),
        summary: toPlainDutyText(incident?.summary),
        signals: Array.isArray(incident?.signals)
          ? incident.signals.map((signal) => ({
              ...signal,
              title: toPlainDutyText(signal?.title),
              summary: toPlainDutyText(signal?.summary),
            }))
          : [],
        finance_issues: Array.isArray(incident?.finance_issues)
          ? incident.finance_issues.map((issue) => ({
              ...issue,
              title: toPlainDutyText(issue?.title),
              message: toPlainDutyText(issue?.message),
            }))
          : [],
      }))
    : [],
});

const queryChoice = (searchParams, key, allowed, fallback) => {
  const value = (searchParams.get(key) || '').trim().toLowerCase();
  return allowed.includes(value) ? value : fallback;
};

const formatCount = (value) => {
  const count = Number(value || 0);
  return Number.isFinite(count) ? count.toLocaleString() : '0';
};

const formatPercent = (value) => {
  const percent = Number(value || 0) * 100;
  return Number.isFinite(percent)
    ? `${percent.toFixed(percent >= 10 ? 1 : 2)}%`
    : '-';
};

const statusTone = (status) => {
  if (status === 'critical' || status === 'error') return 'danger';
  if (status === 'warning' || status === 'insufficient' || status === 'stale')
    return 'warning';
  if (status === 'healthy' || status === 'ready') return 'success';
  return 'neutral';
};

const priorityTone = (priority) => {
  if (priority === 'P0') return 'danger';
  if (priority === 'P1') return 'warning';
  return 'info';
};

const severityRank = (value) => {
  if (value === 'critical') return 3;
  if (value === 'warning') return 2;
  if (value === 'unknown') return 1;
  return 0;
};

const DutyTag = ({ children, tone = 'neutral', className = '' }) => (
  <AppTag className={`duty-tag duty-tag-${tone} ${className}`.trim()}>
    {children}
  </AppTag>
);

const resolveIncidentDomain = (incident) => {
  if (incident?.scope_type === 'channel') return 'channels';
  if (incident?.scope_type === 'model') return 'models';
  if (incident?.scope_type === 'finance') return 'finance';
  if (incident?.tasks?.length > 0) return 'tasks';
  return 'routes';
};

function AdminDashboard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const windowValue = queryChoice(searchParams, 'window', DUTY_WINDOWS, '6h');
  const workView = queryChoice(searchParams, 'view', WORK_VIEWS, 'attention');
  const domainScope = (searchParams.get('domain') || 'all').trim() || 'all';
  const sortMode = queryChoice(searchParams, 'sort', SORT_MODES, 'priority');
  const searchQuery = searchParams.get('q') || '';
  const requestedIncidentID = (searchParams.get('incident') || '').trim();
  const [dashboard, setDashboard] = useState(EMPTY_DUTY_DASHBOARD);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [selectedItemID, setSelectedItemID] = useState('');
  const [mobileDetail, setMobileDetail] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [actionNote, setActionNote] = useState('');

  const patchDutyQuery = useCallback(
    (patch, options = { replace: true }) => {
      const nextSearchParams = new URLSearchParams(searchParams);
      Object.entries(patch).forEach(([key, value]) => {
        if (value === undefined || value === null || value === '') {
          nextSearchParams.delete(key);
          return;
        }
        nextSearchParams.set(key, String(value));
      });
      setSearchParams(nextSearchParams, options);
    },
    [searchParams, setSearchParams]
  );

  const loadDashboard = useCallback(
    async ({ silent = false } = {}) => {
      if (silent) setRefreshing(true);
      else setLoading(true);
      try {
        const response = await API.get('/api/v1/admin/duty/incidents', {
          params: { window: windowValue },
        });
        if (response?.data?.success !== true) {
          throw new Error(
            response?.data?.message || t('dashboard.admin.duty.load_failed')
          );
        }
        setDashboard(normalizeDutyDashboard(response?.data?.data));
        setError('');
      } catch (loadError) {
        setError(loadError?.message || t('dashboard.admin.duty.load_failed'));
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [t, windowValue]
  );

  useEffect(() => {
    loadDashboard();
    const timer = window.setInterval(
      () => loadDashboard({ silent: true }),
      30000
    );
    return () => window.clearInterval(timer);
  }, [loadDashboard]);

  const sourceErrors = dashboard.sources.filter(
    (source) => source.status === 'error'
  );
  const openIncidents = dashboard.incidents.filter((incident) =>
    OPEN_STATUSES.has(incident.status)
  );
  const resolvedIncidents = dashboard.incidents.filter(
    (incident) => incident.status === 'resolved'
  );
  const unlinkedFindings = dashboard.inspection.findings.filter(
    (finding) => !finding.incident_id
  );
  const observationFindings = dashboard.inspection.findings.filter(
    (finding) =>
      finding.health_status === 'unknown' ||
      finding.observation_status !== 'ready'
  );

  const workViewCounts = {
    attention: openIncidents.length + unlinkedFindings.length,
    objects: dashboard.inspection.domains.length,
    incidents: openIncidents.length,
    observation: observationFindings.length,
    resolved: resolvedIncidents.length,
  };

  const queueItems = useMemo(() => {
    const incidentItems = (incidents) =>
      incidents.map((incident) => ({
        id: `incident:${incident.id}`,
        kind: 'incident',
        domain: resolveIncidentDomain(incident),
        title: incident.title || incident.id,
        summary: incident.summary,
        severity: incident.severity,
        healthStatus: incident.severity,
        observationStatus: 'ready',
        status: incident.status,
        priority: incident.priority,
        priorityScore:
          Number(incident.priority_score || 0) * 10 +
          severityRank(incident.severity),
        owner: incident.assignee_username,
        lastSeenAt: incident.last_seen_at,
        incident,
      }));
    const findingItems = (findings) =>
      findings.map((finding) => ({
        id: `finding:${finding.id}`,
        kind: 'finding',
        domain: finding.domain,
        title: finding.title || finding.object_name,
        summary: finding.summary,
        severity: finding.severity,
        healthStatus: finding.health_status,
        observationStatus: finding.observation_status,
        status: finding.incident_id ? 'linked' : 'new',
        priority: finding.severity === 'critical' ? 'P1' : 'P2',
        priorityScore:
          severityRank(finding.severity) * 10 +
          (finding.observation_status === 'error' ? 3 : 0),
        lastSeenAt: finding.last_seen_at,
        finding,
      }));
    const domainItems = dashboard.inspection.domains.map((domain) => ({
      id: `domain:${domain.key}`,
      kind: 'domain',
      domain: domain.key,
      title: t(`dashboard.admin.duty.inspection.domain.${domain.key}`),
      summary:
        domain.metric_kind === 'findings'
          ? t('dashboard.admin.duty.console.finding_domain_summary', {
              findings: domain.finding_count || 0,
            })
          : t('dashboard.admin.duty.console.domain_summary', {
              findings: domain.finding_count || 0,
              total: domain.total_count || 0,
            }),
      severity: domain.health_status,
      healthStatus: domain.health_status,
      observationStatus: domain.observation_status,
      status: domain.health_status,
      priority: '',
      priorityScore: severityRank(domain.health_status) * 10,
      lastSeenAt: domain.last_changed_at,
      metricKind: domain.metric_kind,
      domainItem: domain,
    }));

    if (workView === 'objects') return domainItems;
    if (workView === 'incidents') return incidentItems(openIncidents);
    if (workView === 'observation') return findingItems(observationFindings);
    if (workView === 'resolved') return incidentItems(resolvedIncidents);
    return [...incidentItems(openIncidents), ...findingItems(unlinkedFindings)];
  }, [
    dashboard.inspection.domains,
    observationFindings,
    openIncidents,
    resolvedIncidents,
    t,
    unlinkedFindings,
    workView,
  ]);

  const visibleItems = useMemo(() => {
    const normalizedSearch = searchQuery.trim().toLowerCase();
    const filtered = queueItems.filter((item) => {
      if (domainScope !== 'all' && item.domain !== domainScope) return false;
      if (!normalizedSearch) return true;
      return [item.title, item.summary, item.owner, item.domain]
        .filter(Boolean)
        .some((value) =>
          String(value).toLowerCase().includes(normalizedSearch)
        );
    });
    return filtered.sort((left, right) => {
      if (sortMode === 'recent')
        return Number(right.lastSeenAt || 0) - Number(left.lastSeenAt || 0);
      if (right.priorityScore !== left.priorityScore)
        return right.priorityScore - left.priorityScore;
      return Number(right.lastSeenAt || 0) - Number(left.lastSeenAt || 0);
    });
  }, [domainScope, queueItems, searchQuery, sortMode]);

  useEffect(() => {
    const requestedItemID = requestedIncidentID
      ? `incident:${requestedIncidentID}`
      : '';
    if (
      requestedItemID &&
      visibleItems.some((item) => item.id === requestedItemID)
    ) {
      if (selectedItemID !== requestedItemID) {
        setSelectedItemID(requestedItemID);
      }
      setMobileDetail(true);
      return;
    }
    if (visibleItems.some((item) => item.id === selectedItemID)) return;
    setSelectedItemID(visibleItems[0]?.id || '');
    setMobileDetail(false);
  }, [requestedIncidentID, selectedItemID, visibleItems]);

  const selectedItem =
    visibleItems.find((item) => item.id === selectedItemID) || null;
  const selectedIncident = selectedItem?.incident || null;

  useEffect(() => {
    setActionNote(selectedIncident?.last_note || '');
  }, [selectedIncident?.id, selectedIncident?.last_note]);

  const updateIncidentStatus = useCallback(async () => {
    if (!selectedIncident || actionLoading) return;
    const nextStatus = NEXT_STATUS[selectedIncident.status] || 'acknowledged';
    setActionLoading(true);
    try {
      const response = await API.post(
        `/api/v1/admin/duty/incidents/${encodeURIComponent(
          selectedIncident.id
        )}/status`,
        { status: nextStatus, note: actionNote.trim() },
        { params: { window: windowValue } }
      );
      if (response?.data?.success !== true) {
        throw new Error(
          response?.data?.message || t('dashboard.admin.duty.update_failed')
        );
      }
      showSuccess(t('dashboard.admin.duty.update_success'));
      await loadDashboard({ silent: true });
    } catch (updateError) {
      showError(
        updateError?.message || t('dashboard.admin.duty.update_failed')
      );
    } finally {
      setActionLoading(false);
    }
  }, [
    actionLoading,
    actionNote,
    loadDashboard,
    selectedIncident,
    t,
    windowValue,
  ]);

  const windowOptions = DUTY_WINDOWS.map((value) => ({
    value,
    label: t(`dashboard.admin.duty.window.${value}`),
  }));
  const domainOptions = [
    { value: 'all', label: t('dashboard.admin.duty.console.scope_all') },
    ...dashboard.inspection.domains.map((domain) => ({
      value: domain.key,
      label: t(`dashboard.admin.duty.inspection.domain.${domain.key}`),
    })),
  ];
  const sortOptions = SORT_MODES.map((value) => ({
    value,
    label: t(`dashboard.admin.duty.console.sort.${value}`),
  }));

  const renderLinkButton = (key, link) => {
    if (!link) return null;
    return (
      <AppButton
        key={key}
        basic
        size='small'
        icon={<AppIcon name='right chevron' />}
        onClick={() => navigate(link)}
      >
        {t(`dashboard.admin.duty.links.${key}`)}
      </AppButton>
    );
  };

  const openIncidentFromFinding = (finding) => {
    patchDutyQuery({
      view: 'incidents',
      domain: 'all',
      incident: finding.incident_id,
    });
    window.requestAnimationFrame(() => {
      setSelectedItemID(`incident:${finding.incident_id}`);
      setMobileDetail(true);
    });
  };

  return (
    <div className='dashboard-container admin-dashboard-container duty-station'>
      <AppFilterHeader
        className='admin-dashboard-toolbar duty-toolbar'
        breadcrumbs={[
          { key: 'overview', label: t('dashboard.admin.title') },
          { key: 'duty', label: t('dashboard.admin.duty.title'), active: true },
        ]}
        picker={
          <AppSegmented
            className='duty-window-control'
            options={windowOptions}
            value={windowValue}
            onChange={(_, data) =>
              patchDutyQuery({ window: data.value, incident: null })
            }
          />
        }
        actions={
          <div className='duty-refresh-control'>
            <span>
              {t('dashboard.admin.duty.refreshed_at', {
                time: formatUpdatedAt(dashboard.generated_at),
              })}
            </span>
            <AppTooltip title={t('dashboard.admin.buttons.refresh')}>
              <AppButton
                className='duty-icon-button'
                basic
                aria-label={t('dashboard.admin.buttons.refresh')}
                icon={<AppIcon name='exchange' />}
                loading={refreshing}
                onClick={() => loadDashboard({ silent: true })}
              />
            </AppTooltip>
          </div>
        }
      />

      {error && dashboard.generated_at > 0 ? (
        <AppAlert
          className='duty-stale-alert'
          type='warning'
          showIcon
          title={t('dashboard.admin.duty.refresh_failed')}
          description={error}
        />
      ) : null}

      {loading && dashboard.generated_at <= 0 ? (
        <AppSpin className='admin-dashboard-content-spin' />
      ) : error && dashboard.generated_at <= 0 ? (
        <AppErrorState
          className='duty-page-error'
          message={error}
          onRetry={() => loadDashboard()}
          retryText={t('common.retry')}
        />
      ) : (
        <>
          {sourceErrors.length > 0 ? (
            <AppAlert
              className='duty-source-alert'
              type='error'
              showIcon
              title={t('dashboard.admin.duty.source_error_title', {
                count: sourceErrors.length,
              })}
              description={sourceErrors
                .map((source) =>
                  t('dashboard.admin.duty.source_error_item', {
                    source: t(`dashboard.admin.duty.source.${source.key}`),
                    message:
                      source.message ||
                      t('dashboard.admin.duty.source_error_unknown'),
                  })
                )
                .join(' ')}
            />
          ) : null}

          <section
            className='duty-brief'
            aria-label={t('dashboard.admin.duty.overall')}
          >
            <span
              className={`duty-brief-mark duty-mark-${statusTone(
                dashboard.overall_status
              )}`}
              aria-hidden='true'
            />
            <div className='duty-brief-copy'>
              <strong>
                {t(`dashboard.admin.duty.status.${dashboard.overall_status}`)}
              </strong>
              <span>
                {t(
                  `dashboard.admin.duty.status_hint.${dashboard.overall_status}`
                )}
              </span>
            </div>
            <dl className='duty-brief-facts'>
              <div>
                <dt>{t('dashboard.admin.duty.console.needs_attention')}</dt>
                <dd>{formatCount(workViewCounts.attention)}</dd>
              </div>
              <div>
                <dt>{t('dashboard.admin.duty.summary.active')}</dt>
                <dd>{formatCount(dashboard.summary.active_incidents)}</dd>
              </div>
              <div>
                <dt>{t('dashboard.admin.duty.summary.coverage')}</dt>
                <dd>{formatPercent(dashboard.inspection.coverage_rate)}</dd>
              </div>
            </dl>
          </section>

          <section
            className={`duty-console ${mobileDetail ? 'is-mobile-detail' : ''}`}
          >
            <aside className='duty-console-nav'>
              <div className='duty-nav-group'>
                <h2>{t('dashboard.admin.duty.console.views')}</h2>
                <div className='duty-view-list'>
                  {WORK_VIEWS.map((view) => (
                    <button
                      key={view}
                      type='button'
                      className={workView === view ? 'is-active' : ''}
                      onClick={() => {
                        patchDutyQuery({ view, incident: null });
                        setMobileDetail(false);
                      }}
                    >
                      <span>
                        {t(`dashboard.admin.duty.console.view.${view}`)}
                      </span>
                      <strong>{formatCount(workViewCounts[view])}</strong>
                    </button>
                  ))}
                </div>
              </div>

              <div className='duty-nav-group duty-domain-nav'>
                <h2>{t('dashboard.admin.duty.console.scopes')}</h2>
                <button
                  type='button'
                  className={domainScope === 'all' ? 'is-active' : ''}
                  onClick={() =>
                    patchDutyQuery({ domain: 'all', incident: null })
                  }
                >
                  <span>{t('dashboard.admin.duty.console.scope_all')}</span>
                </button>
                {dashboard.inspection.domains.map((domain) => (
                  <button
                    key={domain.key}
                    type='button'
                    className={domainScope === domain.key ? 'is-active' : ''}
                    onClick={() =>
                      patchDutyQuery({ domain: domain.key, incident: null })
                    }
                  >
                    <span>
                      <i
                        className={`duty-domain-mark duty-mark-${statusTone(
                          domain.health_status
                        )}`}
                        aria-hidden='true'
                      />
                      {t(
                        `dashboard.admin.duty.inspection.domain.${domain.key}`
                      )}
                    </span>
                    <strong>{formatCount(domain.finding_count)}</strong>
                  </button>
                ))}
              </div>

              <div className='duty-observation-note'>
                <span>{t('dashboard.admin.duty.observation_summary')}</span>
                <strong>
                  {t(
                    `dashboard.admin.duty.observation_status.${dashboard.inspection.observation_status}`
                  )}{' '}
                  · {formatPercent(dashboard.inspection.coverage_rate)}
                </strong>
              </div>
            </aside>

            <div className='duty-console-main'>
              <header className='duty-list-header'>
                <div>
                  <h1>{t(`dashboard.admin.duty.console.view.${workView}`)}</h1>
                  <p>
                    {t(`dashboard.admin.duty.console.view_hint.${workView}`)}
                  </p>
                </div>
                <span className='duty-result-count'>
                  {t('dashboard.admin.duty.console.result_count', {
                    count: visibleItems.length,
                  })}
                </span>
              </header>

              <div className='duty-list-controls'>
                <AppInput
                  className='duty-search-input'
                  fluid
                  icon='search'
                  value={searchQuery}
                  placeholder={t('dashboard.admin.duty.console.search')}
                  aria-label={t('dashboard.admin.duty.console.search')}
                  onChange={(_, data) =>
                    patchDutyQuery({ q: data.value, incident: null })
                  }
                />
                <AppSelect
                  className='duty-mobile-scope'
                  compact
                  options={domainOptions}
                  value={domainScope}
                  aria-label={t('dashboard.admin.duty.console.scopes')}
                  onChange={(_, data) =>
                    patchDutyQuery({ domain: data.value, incident: null })
                  }
                />
                <AppSelect
                  className='duty-sort-select'
                  compact
                  options={sortOptions}
                  value={sortMode}
                  aria-label={t('dashboard.admin.duty.console.sort.label')}
                  onChange={(_, data) =>
                    patchDutyQuery({ sort: data.value, incident: null })
                  }
                />
              </div>

              <div className='duty-table-header' aria-hidden='true'>
                <span>{t('dashboard.admin.duty.console.columns.item')}</span>
                <span>{t('dashboard.admin.duty.console.columns.state')}</span>
                <span>{t('dashboard.admin.duty.console.columns.signal')}</span>
                <span>
                  {t('dashboard.admin.duty.console.columns.observation')}
                </span>
                <span>{t('dashboard.admin.duty.console.columns.changed')}</span>
                <span />
              </div>

              {visibleItems.length > 0 ? (
                <div className='duty-table-body'>
                  {visibleItems.map((item) => (
                    <button
                      key={item.id}
                      type='button'
                      className={`duty-table-row ${
                        item.id === selectedItemID ? 'is-selected' : ''
                      }`}
                      onClick={() => {
                        setSelectedItemID(item.id);
                        patchDutyQuery({
                          incident:
                            item.kind === 'incident' ? item.incident.id : null,
                        });
                        setMobileDetail(true);
                      }}
                    >
                      <span className='duty-item-cell'>
                        <span className='duty-item-title'>
                          <i
                            className={`duty-domain-mark duty-mark-${statusTone(
                              item.healthStatus
                            )}`}
                            aria-hidden='true'
                          />
                          <strong>{item.title}</strong>
                        </span>
                        <small>
                          {t(
                            `dashboard.admin.duty.inspection.domain.${item.domain}`
                          )}{' '}
                          ·{' '}
                          {item.summary ||
                            t('dashboard.admin.duty.incident.no_summary')}
                        </small>
                      </span>
                      <span className='duty-state-cell'>
                        {item.kind === 'incident' ? (
                          <>
                            <DutyTag tone={priorityTone(item.priority)}>
                              {item.priority}
                            </DutyTag>
                            <small>
                              {t(
                                `dashboard.admin.duty.incident.status.${item.status}`
                              )}
                            </small>
                          </>
                        ) : (
                          <DutyTag tone={statusTone(item.healthStatus)}>
                            {t(
                              `dashboard.admin.duty.health_status.${item.healthStatus}`
                            )}
                          </DutyTag>
                        )}
                      </span>
                      <span className='duty-signal-cell'>
                        {item.kind === 'incident'
                          ? t('dashboard.admin.duty.incident.impact_brief', {
                              failures: formatCount(
                                item.incident.impact?.failure_count
                              ),
                              tasks: formatCount(
                                item.incident.impact?.failed_task_count
                              ),
                              finance: formatCount(
                                item.incident.impact?.finance_issue_count
                              ),
                            })
                          : item.kind === 'domain'
                          ? item.metricKind === 'findings'
                            ? t(
                                'dashboard.admin.duty.console.finding_domain_signal',
                                {
                                  critical: item.domainItem.critical_count || 0,
                                  warning: item.domainItem.warning_count || 0,
                                }
                              )
                            : t('dashboard.admin.duty.console.domain_signal', {
                                abnormal:
                                  Number(item.domainItem.warning_count || 0) +
                                  Number(item.domainItem.critical_count || 0) +
                                  Number(item.domainItem.unknown_count || 0),
                                total: item.domainItem.total_count || 0,
                              })
                          : t(`dashboard.admin.duty.severity.${item.severity}`)}
                      </span>
                      <span className='duty-observation-cell'>
                        <DutyTag tone={statusTone(item.observationStatus)}>
                          {t(
                            `dashboard.admin.duty.observation_status.${item.observationStatus}`
                          )}
                        </DutyTag>
                      </span>
                      <time className='duty-time-cell'>
                        {item.lastSeenAt
                          ? formatUpdatedAt(item.lastSeenAt)
                          : '-'}
                      </time>
                      <AppIcon name='right chevron' />
                    </button>
                  ))}
                </div>
              ) : (
                <AppEmpty className='duty-console-empty'>
                  <strong>
                    {t('dashboard.admin.duty.console.empty_title')}
                  </strong>
                  <span>
                    {t('dashboard.admin.duty.console.empty_description')}
                  </span>
                </AppEmpty>
              )}
            </div>

            <aside className='duty-context-panel'>
              <AppButton
                className='duty-mobile-only duty-back-button'
                basic
                icon={<AppIcon name='left chevron' />}
                aria-label={t('common.back')}
                onClick={() => {
                  patchDutyQuery({ incident: null });
                  setMobileDetail(false);
                }}
              />
              {selectedItem?.kind === 'incident' ? (
                <IncidentDetail
                  incident={selectedItem.incident}
                  actionLoading={actionLoading}
                  actionNote={actionNote}
                  setActionNote={setActionNote}
                  updateIncidentStatus={updateIncidentStatus}
                  renderLinkButton={renderLinkButton}
                  navigate={navigate}
                  t={t}
                />
              ) : selectedItem?.kind === 'finding' ? (
                <FindingDetail
                  finding={selectedItem.finding}
                  openIncident={openIncidentFromFinding}
                  navigate={navigate}
                  t={t}
                />
              ) : selectedItem?.kind === 'domain' ? (
                <DomainDetail
                  domain={selectedItem.domainItem}
                  findings={dashboard.inspection.findings.filter(
                    (finding) => finding.domain === selectedItem.domain
                  )}
                  navigate={navigate}
                  t={t}
                />
              ) : (
                <AppEmpty className='duty-detail-empty'>
                  {t('dashboard.admin.duty.console.select_hint')}
                </AppEmpty>
              )}
            </aside>
          </section>
        </>
      )}
    </div>
  );
}

function DetailHeader({ badges, title, summary }) {
  return (
    <header className='duty-detail-header'>
      <div className='duty-detail-badges'>{badges}</div>
      <h2>{title}</h2>
      <p>{summary}</p>
    </header>
  );
}

function IncidentDetail({
  incident,
  actionLoading,
  actionNote,
  setActionNote,
  updateIncidentStatus,
  renderLinkButton,
  navigate,
  t,
}) {
  return (
    <div className='duty-detail'>
      <DetailHeader
        badges={
          <>
            <DutyTag tone={priorityTone(incident.priority)}>
              {incident.priority}
            </DutyTag>
            <DutyTag tone={statusTone(incident.severity)}>
              {t(`dashboard.admin.duty.severity.${incident.severity}`)}
            </DutyTag>
            <DutyTag tone='neutral'>
              {t(`dashboard.admin.duty.incident.status.${incident.status}`)}
            </DutyTag>
          </>
        }
        title={incident.title || incident.id}
        summary={
          incident.summary || t('dashboard.admin.duty.incident.no_summary')
        }
      />
      <dl className='duty-detail-meta'>
        <div>
          <dt>{t('dashboard.admin.duty.incident.assignee')}</dt>
          <dd>
            {incident.assignee_username ||
              t('dashboard.admin.duty.incident.unassigned')}
          </dd>
        </div>
        <div>
          <dt>{t('dashboard.admin.duty.incident.last_seen')}</dt>
          <dd>{formatUpdatedAt(incident.last_seen_at)}</dd>
        </div>
      </dl>

      <section className='duty-detail-block'>
        <h3>{t('dashboard.admin.duty.incident.impact_title')}</h3>
        <div className='duty-detail-metrics'>
          {[
            ['requests', incident.impact?.request_count],
            ['failures', incident.impact?.failure_count],
            ['failure_rate', formatPercent(incident.impact?.failure_rate)],
            ['fallbacks', incident.impact?.fallback_count],
            ['failed_tasks', incident.impact?.failed_task_count],
            ['finance', incident.impact?.finance_issue_count],
          ].map(([key, value]) => (
            <div key={key}>
              <span>{t(`dashboard.admin.duty.incident.impact.${key}`)}</span>
              <strong>
                {typeof value === 'string' ? value : formatCount(value)}
              </strong>
            </div>
          ))}
        </div>
        <div className='duty-scope-lines'>
          {incident.channel_name ? (
            <p>
              <span>{t('dashboard.admin.duty.incident.channel')}</span>
              {incident.channel_name}
            </p>
          ) : null}
          {incident.models?.length > 0 ? (
            <p>
              <span>{t('dashboard.admin.duty.incident.models')}</span>
              {incident.models.join(', ')}
            </p>
          ) : null}
          {incident.endpoints?.length > 0 ? (
            <p>
              <span>{t('dashboard.admin.duty.incident.endpoints')}</span>
              {incident.endpoints.join(', ')}
            </p>
          ) : null}
        </div>
      </section>

      <section className='duty-detail-block'>
        <h3>{t('dashboard.admin.duty.incident.signals')}</h3>
        {incident.signals?.length > 0 ? (
          <div className='duty-detail-list'>
            {incident.signals.map((signal) => (
              <div className='duty-detail-list-item' key={signal.id}>
                <span
                  className={`duty-signal-mark duty-signal-${signal.level}`}
                  aria-hidden='true'
                />
                <div>
                  <strong>{signal.title}</strong>
                  <p>
                    {signal.summary ||
                      t('dashboard.admin.duty.incident.no_summary')}
                  </p>
                  <time>{formatUpdatedAt(signal.created_at)}</time>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <AppEmpty>{t('dashboard.admin.duty.incident.no_signals')}</AppEmpty>
        )}
      </section>

      {incident.tasks?.length > 0 ? (
        <section className='duty-detail-block'>
          <h3>{t('dashboard.admin.duty.incident.tasks')}</h3>
          <div className='duty-detail-list'>
            {incident.tasks.map((task) => (
              <button
                key={task.id}
                type='button'
                className='duty-related-row'
                onClick={() => navigate(task.link)}
              >
                <span>
                  <strong>{task.type}</strong>
                  <small>{task.model || task.id}</small>
                </span>
                <DutyTag tone={task.status === 'failed' ? 'danger' : 'info'}>
                  {task.status}
                </DutyTag>
              </button>
            ))}
          </div>
        </section>
      ) : null}

      {incident.finance_issues?.length > 0 ? (
        <section className='duty-detail-block'>
          <h3>{t('dashboard.admin.duty.incident.finance')}</h3>
          <div className='duty-detail-list'>
            {incident.finance_issues.map((issue) => (
              <button
                key={issue.key}
                type='button'
                className='duty-related-row'
                onClick={() => navigate(issue.link)}
              >
                <span>
                  <strong>{issue.title}</strong>
                  <small>{issue.message}</small>
                </span>
                <DutyTag tone={statusTone(issue.level)}>
                  {t(`dashboard.admin.duty.severity.${issue.level}`)}
                </DutyTag>
              </button>
            ))}
          </div>
        </section>
      ) : null}

      <section className='duty-detail-block duty-detail-actions'>
        <h3>{t('dashboard.admin.duty.incident.handling')}</h3>
        <AppTextarea
          value={actionNote}
          rows={3}
          maxLength={1000}
          placeholder={t('dashboard.admin.duty.incident.note_placeholder')}
          onChange={(_, data) => setActionNote(data.value)}
        />
        <div className='duty-action-row'>
          <span>{t('dashboard.admin.duty.incident.action_hint')}</span>
          <AppButton
            color={incident.status === 'resolved' ? undefined : 'blue'}
            basic={incident.status === 'resolved'}
            loading={actionLoading}
            onClick={updateIncidentStatus}
          >
            {t(
              `dashboard.admin.duty.incident.actions.${
                NEXT_STATUS[incident.status] || 'acknowledged'
              }`
            )}
          </AppButton>
        </div>
      </section>

      <section className='duty-detail-block duty-drilldown'>
        <h3>{t('dashboard.admin.duty.incident.drilldown')}</h3>
        <div className='duty-link-row'>
          {Object.entries(incident.links || {}).map(([key, link]) =>
            renderLinkButton(key, link)
          )}
        </div>
      </section>
    </div>
  );
}

function FindingDetail({ finding, openIncident, navigate, t }) {
  return (
    <div className='duty-detail'>
      <DetailHeader
        badges={
          <>
            <DutyTag tone={statusTone(finding.severity)}>
              {t(`dashboard.admin.duty.severity.${finding.severity}`)}
            </DutyTag>
            <DutyTag tone={statusTone(finding.observation_status)}>
              {t(
                `dashboard.admin.duty.observation_status.${finding.observation_status}`
              )}
            </DutyTag>
          </>
        }
        title={finding.title || finding.object_name}
        summary={
          finding.summary || t('dashboard.admin.duty.incident.no_summary')
        }
      />
      <dl className='duty-detail-meta duty-finding-meta'>
        <div>
          <dt>{t('dashboard.admin.duty.console.object')}</dt>
          <dd>{finding.object_name || finding.object_id}</dd>
        </div>
        <div>
          <dt>{t('dashboard.admin.duty.console.health')}</dt>
          <dd>
            {t(`dashboard.admin.duty.health_status.${finding.health_status}`)}
          </dd>
        </div>
        <div>
          <dt>{t('dashboard.admin.duty.console.first_seen')}</dt>
          <dd>
            {finding.first_seen_at
              ? formatUpdatedAt(finding.first_seen_at)
              : '-'}
          </dd>
        </div>
        <div>
          <dt>{t('dashboard.admin.duty.incident.last_seen')}</dt>
          <dd>
            {finding.last_seen_at ? formatUpdatedAt(finding.last_seen_at) : '-'}
          </dd>
        </div>
      </dl>
      <section className='duty-detail-block duty-detail-actions'>
        <h3>{t('dashboard.admin.duty.console.next_action')}</h3>
        <p className='duty-detail-guidance'>
          {finding.incident_id
            ? t('dashboard.admin.duty.console.finding_linked_hint')
            : t('dashboard.admin.duty.console.finding_open_hint')}
        </p>
        <div className='duty-link-row'>
          {finding.incident_id ? (
            <AppButton color='blue' onClick={() => openIncident(finding)}>
              {t('dashboard.admin.duty.console.open_incident')}
            </AppButton>
          ) : null}
          {finding.link ? (
            <AppButton basic onClick={() => navigate(finding.link)}>
              {t('dashboard.admin.duty.console.open_object')}
            </AppButton>
          ) : null}
        </div>
      </section>
    </div>
  );
}

function DomainDetail({ domain, findings, navigate, t }) {
  const affectedCount =
    Number(domain.warning_count || 0) +
    Number(domain.critical_count || 0) +
    Number(domain.unknown_count || 0);
  return (
    <div className='duty-detail'>
      <DetailHeader
        badges={
          <>
            <DutyTag tone={statusTone(domain.health_status)}>
              {t(`dashboard.admin.duty.health_status.${domain.health_status}`)}
            </DutyTag>
            <DutyTag tone={statusTone(domain.observation_status)}>
              {t(
                `dashboard.admin.duty.observation_status.${domain.observation_status}`
              )}
            </DutyTag>
          </>
        }
        title={t(`dashboard.admin.duty.inspection.domain.${domain.key}`)}
        summary={t('dashboard.admin.duty.console.domain_detail_hint')}
      />
      <div className='duty-detail-metrics duty-domain-metrics'>
        {domain.metric_kind === 'findings' ? (
          <>
            <div>
              <span>{t('dashboard.admin.duty.summary.findings')}</span>
              <strong>{formatCount(domain.finding_count)}</strong>
            </div>
            <div>
              <span>{t('dashboard.admin.duty.severity.critical')}</span>
              <strong>{formatCount(domain.critical_count)}</strong>
            </div>
            <div>
              <span>{t('dashboard.admin.duty.severity.warning')}</span>
              <strong>{formatCount(domain.warning_count)}</strong>
            </div>
            <div>
              <span>{t('dashboard.admin.duty.summary.unknown_objects')}</span>
              <strong>{formatCount(domain.unknown_count)}</strong>
            </div>
          </>
        ) : (
          <>
            <div>
              <span>{t('dashboard.admin.duty.summary.abnormal_objects')}</span>
              <strong>{formatCount(affectedCount)}</strong>
            </div>
            <div>
              <span>{t('dashboard.admin.duty.console.total_objects')}</span>
              <strong>{formatCount(domain.total_count)}</strong>
            </div>
            <div>
              <span>{t('dashboard.admin.duty.summary.findings')}</span>
              <strong>{formatCount(domain.finding_count)}</strong>
            </div>
            <div>
              <span>{t('dashboard.admin.duty.summary.coverage')}</span>
              <strong>{formatPercent(domain.coverage_rate)}</strong>
            </div>
          </>
        )}
      </div>
      <section className='duty-detail-block'>
        <h3>{t('dashboard.admin.duty.inspection.findings')}</h3>
        {findings.length > 0 ? (
          <div className='duty-detail-list'>
            {findings.slice(0, 6).map((finding) => (
              <div className='duty-detail-list-item' key={finding.id}>
                <span
                  className={`duty-signal-mark duty-signal-${finding.severity}`}
                  aria-hidden='true'
                />
                <div>
                  <strong>{finding.title || finding.object_name}</strong>
                  <p>
                    {finding.summary ||
                      t('dashboard.admin.duty.incident.no_summary')}
                  </p>
                  <time>
                    {finding.last_seen_at
                      ? formatUpdatedAt(finding.last_seen_at)
                      : '-'}
                  </time>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <AppEmpty>
            {t('dashboard.admin.duty.inspection.empty_title')}
          </AppEmpty>
        )}
      </section>
      {domain.link ? (
        <section className='duty-detail-block duty-drilldown'>
          <AppButton basic onClick={() => navigate(domain.link)}>
            {t('dashboard.admin.duty.console.open_object')}
          </AppButton>
        </section>
      ) : null}
    </div>
  );
}

export default AdminDashboard;
