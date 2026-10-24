package duty

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/yeying-community/router/common/ctxkey"
	"github.com/yeying-community/router/common/helper"
	adminbilling "github.com/yeying-community/router/internal/admin/controller/billing"
	channelcontroller "github.com/yeying-community/router/internal/admin/controller/channel"
	dashboardcontroller "github.com/yeying-community/router/internal/admin/controller/dashboard"
	logcontroller "github.com/yeying-community/router/internal/admin/controller/log"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/gorm"
)

const (
	dutySourceReady                 = "ready"
	dutySourceError                 = "error"
	dutyTaskStaleAfterSecond        = int64(30 * 60)
	dutyObservationStaleAfterSecond = int64(7 * 24 * 60 * 60)

	dutyHealthHealthy  = "healthy"
	dutyHealthWarning  = "warning"
	dutyHealthCritical = "critical"
	dutyHealthUnknown  = "unknown"

	dutyObservationReady        = "ready"
	dutyObservationInsufficient = "insufficient"
	dutyObservationStale        = "stale"
	dutyObservationError        = "error"
)

type dutySourceState struct {
	Key       string `json:"key"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
}

type dutySignal struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Level         string `json:"level"`
	Title         string `json:"title"`
	Summary       string `json:"summary,omitempty"`
	CreatedAt     int64  `json:"created_at"`
	Link          string `json:"link,omitempty"`
	ReferenceType string `json:"-"`
	ReferenceKey  string `json:"-"`
	ChannelID     string `json:"-"`
}

type dutyTask struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Model        string `json:"model,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	FinishedAt   int64  `json:"finished_at,omitempty"`
	Link         string `json:"link"`
}

type dutyFinanceIssue struct {
	Key     string `json:"key"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Count   int64  `json:"count,omitempty"`
	Link    string `json:"link"`
}

type dutyImpact struct {
	RequestCount      int64   `json:"request_count"`
	FailureCount      int64   `json:"failure_count"`
	FallbackCount     int64   `json:"fallback_count"`
	FailureRate       float64 `json:"failure_rate"`
	FailedTaskCount   int     `json:"failed_task_count"`
	FinanceIssueCount int     `json:"finance_issue_count"`
	FinancialRisk     int64   `json:"financial_risk_count"`
	AffectedModels    int     `json:"affected_model_count"`
}

type dutyLinks struct {
	Channel string `json:"channel,omitempty"`
	Alerts  string `json:"alerts,omitempty"`
	Tasks   string `json:"tasks,omitempty"`
	Logs    string `json:"logs,omitempty"`
	Models  string `json:"models,omitempty"`
	Finance string `json:"finance,omitempty"`
}

type dutyIncident struct {
	ID               string              `json:"id"`
	ScopeType        string              `json:"scope_type"`
	Title            string              `json:"title"`
	Summary          string              `json:"summary"`
	Severity         string              `json:"severity"`
	Priority         string              `json:"priority"`
	PriorityScore    int                 `json:"priority_score"`
	Status           string              `json:"status"`
	AssigneeUserID   string              `json:"assignee_user_id,omitempty"`
	AssigneeUsername string              `json:"assignee_username,omitempty"`
	LastNote         string              `json:"last_note,omitempty"`
	StartedAt        int64               `json:"started_at"`
	LastSeenAt       int64               `json:"last_seen_at"`
	AcknowledgedAt   int64               `json:"acknowledged_at,omitempty"`
	MitigatedAt      int64               `json:"mitigated_at,omitempty"`
	ResolvedAt       int64               `json:"resolved_at,omitempty"`
	ChannelID        string              `json:"channel_id,omitempty"`
	ChannelName      string              `json:"channel_name,omitempty"`
	Models           []string            `json:"models"`
	Endpoints        []string            `json:"endpoints"`
	Impact           dutyImpact          `json:"impact"`
	Signals          []dutySignal        `json:"signals"`
	Tasks            []dutyTask          `json:"tasks"`
	FinanceIssues    []dutyFinanceIssue  `json:"finance_issues"`
	Links            dutyLinks           `json:"links"`
	modelSet         map[string]struct{} `json:"-"`
	endpointSet      map[string]struct{} `json:"-"`
}

type dutySummary struct {
	ActiveIncidents   int   `json:"active_incidents"`
	Critical          int   `json:"critical_incidents"`
	Warning           int   `json:"warning_incidents"`
	Unassigned        int   `json:"unassigned_incidents"`
	AffectedModels    int   `json:"affected_models"`
	FailureCount      int64 `json:"failure_count"`
	FallbackCount     int64 `json:"fallback_count"`
	FailedTaskCount   int   `json:"failed_task_count"`
	FinanceIssueCount int   `json:"finance_issue_count"`
}

type dutyInspectionDomain struct {
	Key               string  `json:"key"`
	HealthStatus      string  `json:"health_status"`
	ObservationStatus string  `json:"observation_status"`
	MetricKind        string  `json:"metric_kind"`
	TotalCount        int     `json:"total_count"`
	HealthyCount      int     `json:"healthy_count"`
	WarningCount      int     `json:"warning_count"`
	CriticalCount     int     `json:"critical_count"`
	UnknownCount      int     `json:"unknown_count"`
	FindingCount      int     `json:"finding_count"`
	CoverageRate      float64 `json:"coverage_rate"`
	LastChangedAt     int64   `json:"last_changed_at"`
	Link              string  `json:"link"`
}

type dutyFinding struct {
	ID                string `json:"id"`
	Domain            string `json:"domain"`
	ObjectType        string `json:"object_type"`
	ObjectID          string `json:"object_id"`
	ObjectName        string `json:"object_name"`
	Severity          string `json:"severity"`
	HealthStatus      string `json:"health_status"`
	ObservationStatus string `json:"observation_status"`
	Title             string `json:"title"`
	Summary           string `json:"summary"`
	FirstSeenAt       int64  `json:"first_seen_at"`
	LastSeenAt        int64  `json:"last_seen_at"`
	IncidentID        string `json:"incident_id,omitempty"`
	Link              string `json:"link"`
}

type dutyInspection struct {
	HealthStatus      string                 `json:"health_status"`
	ObservationStatus string                 `json:"observation_status"`
	CoverageRate      float64                `json:"coverage_rate"`
	AbnormalCount     int                    `json:"abnormal_object_count"`
	UnknownCount      int                    `json:"unknown_object_count"`
	FindingCount      int                    `json:"finding_count"`
	Domains           []dutyInspectionDomain `json:"domains"`
	Findings          []dutyFinding          `json:"findings"`
}

type dutyIncidentResponse struct {
	OverallStatus string            `json:"overall_status"`
	Window        string            `json:"window"`
	WindowStartAt int64             `json:"window_start_at"`
	WindowEndAt   int64             `json:"window_end_at"`
	GeneratedAt   int64             `json:"generated_at"`
	Summary       dutySummary       `json:"summary"`
	Sources       []dutySourceState `json:"sources"`
	Inspection    dutyInspection    `json:"inspection"`
	Incidents     []dutyIncident    `json:"incidents"`
}

type updateDutyIncidentRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note" binding:"max=1000"`
}

func parseDutyWindow(raw string) (string, int64) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "1h":
		return "1h", 60 * 60
	case "24h":
		return "24h", 24 * 60 * 60
	default:
		return "6h", 6 * 60 * 60
	}
}

func severityRank(level string) int {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case "critical", "error":
		return 3
	case "warning", "warn":
		return 2
	default:
		return 1
	}
}

func normalizedSeverity(level string) string {
	if severityRank(level) >= 3 {
		return "critical"
	}
	return "warning"
}

func maxTimestamp(values ...int64) int64 {
	result := int64(0)
	for _, value := range values {
		if value > result {
			result = value
		}
	}
	return result
}

func minPositiveTimestamp(current int64, candidate int64) int64 {
	if candidate <= 0 {
		return current
	}
	if current <= 0 || candidate < current {
		return candidate
	}
	return current
}

func safeScopePart(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, ":", "_"))
}

func channelIncidentKey(channelID string) string {
	return "channel:" + safeScopePart(channelID)
}

func modelIncidentKey(modelName string) string {
	return "model:" + safeScopePart(modelName)
}

func financeIncidentKey(issueKey string) string {
	return "finance:" + safeScopePart(issueKey)
}

func newDutyIncident(key string, scopeType string, title string, summary string, severity string, observedAt int64) *dutyIncident {
	return &dutyIncident{
		ID:            key,
		ScopeType:     scopeType,
		Title:         strings.TrimSpace(title),
		Summary:       strings.TrimSpace(summary),
		Severity:      normalizedSeverity(severity),
		Status:        model.DutyIncidentStatusActive,
		StartedAt:     observedAt,
		LastSeenAt:    observedAt,
		Models:        []string{},
		Endpoints:     []string{},
		Signals:       []dutySignal{},
		Tasks:         []dutyTask{},
		FinanceIssues: []dutyFinanceIssue{},
		modelSet:      map[string]struct{}{},
		endpointSet:   map[string]struct{}{},
	}
}

func raiseIncidentSeverity(incident *dutyIncident, severity string, title string, summary string) {
	if incident == nil {
		return
	}
	if severityRank(severity) > severityRank(incident.Severity) {
		incident.Severity = normalizedSeverity(severity)
		if strings.TrimSpace(title) != "" {
			incident.Title = strings.TrimSpace(title)
		}
		if strings.TrimSpace(summary) != "" {
			incident.Summary = strings.TrimSpace(summary)
		}
	}
}

func addIncidentModel(incident *dutyIncident, modelName string) {
	if incident == nil {
		return
	}
	normalized := strings.TrimSpace(modelName)
	if normalized == "" || normalized == "-" {
		return
	}
	if _, ok := incident.modelSet[normalized]; ok {
		return
	}
	incident.modelSet[normalized] = struct{}{}
	incident.Models = append(incident.Models, normalized)
}

func addIncidentEndpoint(incident *dutyIncident, endpoint string) {
	if incident == nil {
		return
	}
	normalized := strings.TrimSpace(endpoint)
	if normalized == "" || normalized == "-" {
		return
	}
	if _, ok := incident.endpointSet[normalized]; ok {
		return
	}
	incident.endpointSet[normalized] = struct{}{}
	incident.Endpoints = append(incident.Endpoints, normalized)
}

func sourceState(key string, count int, updatedAt int64, err error) dutySourceState {
	state := dutySourceState{Key: key, Status: dutySourceReady, UpdatedAt: updatedAt}
	if err != nil {
		state.Status = dutySourceError
		state.Message = err.Error()
	}
	return state
}

func incidentKeyForChannelOrModel(channelID string, modelName string, fallback string) (string, string) {
	if normalizedChannelID := strings.TrimSpace(channelID); normalizedChannelID != "" && normalizedChannelID != "-" {
		return channelIncidentKey(normalizedChannelID), "channel"
	}
	if normalizedModel := strings.TrimSpace(modelName); normalizedModel != "" && normalizedModel != "-" {
		return modelIncidentKey(normalizedModel), "model"
	}
	return fallback, "event"
}

func taskNeedsDutyAttention(task model.AsyncTask, now int64) bool {
	status := model.NormalizeAsyncTaskStatus(task.Status)
	if status == model.AsyncTaskStatusFailed {
		return true
	}
	if status != model.AsyncTaskStatusPending && status != model.AsyncTaskStatusRunning {
		return false
	}
	observedAt := task.CreatedAt
	if status == model.AsyncTaskStatusRunning && task.StartedAt > 0 {
		observedAt = task.StartedAt
	}
	return observedAt > 0 && now-observedAt >= dutyTaskStaleAfterSecond
}

func dutyOperationName(taskType string) string {
	switch model.NormalizeAsyncTaskType(taskType) {
	case model.AsyncTaskTypeChannelModelTest:
		return "模型测试"
	case model.AsyncTaskTypeChannelRefreshModels:
		return "模型同步"
	case model.AsyncTaskTypeChannelRefreshBilling:
		return "账务刷新"
	case model.AsyncTaskTypeChannelSyncProviderUsage:
		return "用量同步"
	default:
		return "后台执行"
	}
}

func dutyOperationLink(task model.AsyncTask) string {
	channelID := strings.TrimSpace(task.ChannelId)
	if channelID == "" {
		return "/admin/channel"
	}
	base := "/admin/channel/detail/" + url.PathEscape(channelID)
	switch model.NormalizeAsyncTaskType(task.Type) {
	case model.AsyncTaskTypeChannelModelTest:
		query := url.Values{"tab": []string{"tests"}}
		if modelName := strings.TrimSpace(task.Model); modelName != "" {
			query.Set("model", modelName)
		}
		if endpoint := strings.TrimSpace(task.Endpoint); endpoint != "" {
			query.Set("endpoint", endpoint)
		}
		return base + "?" + query.Encode()
	case model.AsyncTaskTypeChannelRefreshModels:
		return base + "?tab=models"
	case model.AsyncTaskTypeChannelRefreshBilling, model.AsyncTaskTypeChannelSyncProviderUsage:
		return base + "?tab=procurement"
	default:
		return base
	}
}

func filterDutyChannelAlerts(alerts []channelcontroller.DutyChannelAlert, startAt int64, endAt int64) []channelcontroller.DutyChannelAlert {
	filtered := make([]channelcontroller.DutyChannelAlert, 0, len(alerts))
	for _, alert := range alerts {
		status := strings.TrimSpace(alert.Status)
		if status == model.ChannelAlertStatusResolved {
			if alert.CreatedAt < startAt || alert.CreatedAt > endAt {
				continue
			}
		} else if alert.CreatedAt > endAt {
			continue
		}
		if severityRank(alert.Level) <= 1 {
			continue
		}
		filtered = append(filtered, alert)
	}
	return filtered
}

func applyDutyIncidentState(incident *dutyIncident, state model.DutyIncidentState) {
	if incident == nil {
		return
	}
	incident.Status = state.Status
	incident.AssigneeUserID = state.AssigneeUserId
	incident.AssigneeUsername = state.AssigneeUsername
	incident.LastNote = state.LastNote
	incident.AcknowledgedAt = state.AcknowledgedAt
	incident.MitigatedAt = state.MitigatedAt
	incident.ResolvedAt = state.ResolvedAt
	if state.Status == model.DutyIncidentStatusResolved && incident.LastSeenAt > state.ResolvedAt {
		incident.Status = model.DutyIncidentStatusActive
		incident.ResolvedAt = 0
	}
}

func normalizedHealthStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case dutyHealthHealthy:
		return dutyHealthHealthy
	case dutyHealthWarning:
		return dutyHealthWarning
	case dutyHealthCritical, "error":
		return dutyHealthCritical
	default:
		return dutyHealthUnknown
	}
}

func healthStatusRank(value string) int {
	switch normalizedHealthStatus(value) {
	case dutyHealthCritical:
		return 3
	case dutyHealthWarning:
		return 2
	case dutyHealthUnknown:
		return 1
	default:
		return 0
	}
}

func observationStatusRank(value string) int {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case dutyObservationError:
		return 3
	case dutyObservationInsufficient:
		return 2
	case dutyObservationStale:
		return 1
	default:
		return 0
	}
}

func mergeObservationStatus(current string, candidate string) string {
	if observationStatusRank(candidate) > observationStatusRank(current) {
		return candidate
	}
	return current
}

func objectObservationStatus(hasData bool, coverageRate float64, lastObservedAt int64, now int64) string {
	if !hasData || coverageRate < 1 {
		return dutyObservationInsufficient
	}
	if lastObservedAt <= 0 {
		return dutyObservationInsufficient
	}
	if now > lastObservedAt && now-lastObservedAt > dutyObservationStaleAfterSecond {
		return dutyObservationStale
	}
	return dutyObservationReady
}

func addDomainHealth(domain *dutyInspectionDomain, healthStatus string) {
	if domain == nil {
		return
	}
	switch normalizedHealthStatus(healthStatus) {
	case dutyHealthHealthy:
		domain.HealthyCount++
	case dutyHealthWarning:
		domain.WarningCount++
	case dutyHealthCritical:
		domain.CriticalCount++
	default:
		domain.UnknownCount++
	}
}

func finalizeInspectionDomain(domain *dutyInspectionDomain, loadErr error) {
	if domain == nil {
		return
	}
	if loadErr != nil {
		domain.HealthStatus = dutyHealthUnknown
		domain.ObservationStatus = dutyObservationError
		return
	}
	switch {
	case domain.CriticalCount > 0:
		domain.HealthStatus = dutyHealthCritical
	case domain.WarningCount > 0:
		domain.HealthStatus = dutyHealthWarning
	case domain.UnknownCount > 0 || domain.TotalCount == 0 && domain.MetricKind == "objects":
		domain.HealthStatus = dutyHealthUnknown
	default:
		domain.HealthStatus = dutyHealthHealthy
	}
	if strings.TrimSpace(domain.ObservationStatus) == "" {
		if domain.TotalCount == 0 && domain.MetricKind == "objects" {
			domain.ObservationStatus = dutyObservationInsufficient
		} else {
			domain.ObservationStatus = dutyObservationReady
		}
	}
}

func dutyFindingIncidentID(key string, incidents map[string]struct{}) string {
	if _, ok := incidents[key]; ok {
		return key
	}
	return ""
}

func buildDutyInspection(
	endAt int64,
	channels []dashboardcontroller.DutyChannelHealth,
	channelsErr error,
	models []dashboardcontroller.DutyModelHealth,
	modelsErr error,
	anomalies []logcontroller.DutyRouteAnomaly,
	anomaliesErr error,
	tasks []model.AsyncTask,
	taskTotal int64,
	tasksErr error,
	financeIssues []adminbilling.DutyFinanceIssue,
	incidents []dutyIncident,
) dutyInspection {
	inspection := dutyInspection{
		HealthStatus:      dutyHealthHealthy,
		ObservationStatus: dutyObservationReady,
		Domains:           make([]dutyInspectionDomain, 0, 5),
		Findings:          make([]dutyFinding, 0),
	}
	incidentIDs := make(map[string]struct{}, len(incidents))
	for _, incident := range incidents {
		incidentIDs[incident.ID] = struct{}{}
	}

	channelDomain := dutyInspectionDomain{
		Key: "channels", MetricKind: "objects", TotalCount: len(channels),
		ObservationStatus: dutyObservationReady, Link: "/admin/channel?tab=health",
	}
	channelCoverageTotal := 0.0
	for _, item := range channels {
		healthStatus := normalizedHealthStatus(item.HealthLevel)
		if item.CircuitBreakerActive || item.BillingLevel == model.ChannelBillingItemStatusDepleted {
			healthStatus = dutyHealthCritical
		} else if item.BillingLevel == model.ChannelBillingItemStatusLow && healthStatusRank(healthStatus) < healthStatusRank(dutyHealthWarning) {
			healthStatus = dutyHealthWarning
		}
		observationStatus := objectObservationStatus(item.HasTestData, item.CoverageRate, item.LastTestedAt, endAt)
		if observationStatus == dutyObservationInsufficient && !item.HasTestData {
			healthStatus = dutyHealthUnknown
		}
		addDomainHealth(&channelDomain, healthStatus)
		channelDomain.ObservationStatus = mergeObservationStatus(channelDomain.ObservationStatus, observationStatus)
		channelCoverageTotal += item.CoverageRate
		channelDomain.LastChangedAt = maxTimestamp(channelDomain.LastChangedAt, item.LastTestedAt)
		if healthStatus == dutyHealthHealthy && observationStatus == dutyObservationReady {
			continue
		}
		severity := dutyHealthWarning
		if healthStatus == dutyHealthCritical {
			severity = dutyHealthCritical
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.ID)
		}
		inspection.Findings = append(inspection.Findings, dutyFinding{
			ID: "channel-health:" + safeScopePart(item.ID), Domain: "channels", ObjectType: "channel",
			ObjectID: item.ID, ObjectName: name, Severity: severity, HealthStatus: healthStatus,
			ObservationStatus: observationStatus, Title: name + " 健康状态需要关注",
			Summary:    fmt.Sprintf("通过率 %.1f%%，测试覆盖率 %.1f%%", item.PassRate*100, item.CoverageRate*100),
			LastSeenAt: item.LastTestedAt, IncidentID: dutyFindingIncidentID(channelIncidentKey(item.ID), incidentIDs),
			Link: "/admin/channel/detail/" + item.ID + "?tab=tests",
		})
		channelDomain.FindingCount++
	}
	if channelDomain.TotalCount > 0 {
		channelDomain.CoverageRate = channelCoverageTotal / float64(channelDomain.TotalCount)
	}
	finalizeInspectionDomain(&channelDomain, channelsErr)
	inspection.Domains = append(inspection.Domains, channelDomain)

	modelDomain := dutyInspectionDomain{
		Key: "models", MetricKind: "objects", TotalCount: len(models),
		ObservationStatus: dutyObservationReady, Link: "/workspace/service/models?tab=operations",
	}
	modelCoverageTotal := 0.0
	for _, item := range models {
		coverageRate := 0.0
		if item.ChannelCount > 0 {
			coverageRate = math.Min(1, float64(item.TestedChannelCount)/float64(item.ChannelCount))
		}
		healthStatus := normalizedHealthStatus(item.HealthLevel)
		observationStatus := objectObservationStatus(item.TestedChannelCount > 0, coverageRate, item.LastTestedAt, endAt)
		if observationStatus == dutyObservationInsufficient && item.TestedChannelCount == 0 {
			healthStatus = dutyHealthUnknown
		}
		addDomainHealth(&modelDomain, healthStatus)
		modelDomain.ObservationStatus = mergeObservationStatus(modelDomain.ObservationStatus, observationStatus)
		modelCoverageTotal += coverageRate
		modelDomain.LastChangedAt = maxTimestamp(modelDomain.LastChangedAt, item.LastTestedAt)
		if healthStatus == dutyHealthHealthy && observationStatus == dutyObservationReady {
			continue
		}
		severity := dutyHealthWarning
		if healthStatus == dutyHealthCritical {
			severity = dutyHealthCritical
		}
		inspection.Findings = append(inspection.Findings, dutyFinding{
			ID: "model-health:" + safeScopePart(item.Name), Domain: "models", ObjectType: "model",
			ObjectID: item.Name, ObjectName: item.Name, Severity: severity, HealthStatus: healthStatus,
			ObservationStatus: observationStatus, Title: item.Name + " 运行状态需要关注",
			Summary:    fmt.Sprintf("通过率 %.1f%%，已测试渠道 %d/%d", item.PassRate*100, item.TestedChannelCount, item.ChannelCount),
			LastSeenAt: item.LastTestedAt, IncidentID: dutyFindingIncidentID(modelIncidentKey(item.Name), incidentIDs),
			Link: "/workspace/service/models?tab=operations&model=" + item.Name,
		})
		modelDomain.FindingCount++
	}
	if modelDomain.TotalCount > 0 {
		modelDomain.CoverageRate = modelCoverageTotal / float64(modelDomain.TotalCount)
	}
	finalizeInspectionDomain(&modelDomain, modelsErr)
	inspection.Domains = append(inspection.Domains, modelDomain)

	routeDomain := dutyInspectionDomain{
		Key: "routes", MetricKind: "findings", ObservationStatus: dutyObservationReady,
		FindingCount: len(anomalies), Link: "/admin/log",
	}
	for _, anomaly := range anomalies {
		healthStatus := dutyHealthWarning
		if anomaly.FailureRate >= 0.2 || anomaly.FailureCount >= 100 {
			healthStatus = dutyHealthCritical
		}
		addDomainHealth(&routeDomain, healthStatus)
		routeDomain.LastChangedAt = maxTimestamp(routeDomain.LastChangedAt, anomaly.LastObservedAt)
		objectName := strings.TrimSpace(anomaly.ChannelName)
		if objectName == "" {
			objectName = strings.TrimSpace(anomaly.Model)
		}
		incidentKey, _ := incidentKeyForChannelOrModel(anomaly.ChannelID, anomaly.Model, "route:unknown")
		inspection.Findings = append(inspection.Findings, dutyFinding{
			ID:     fmt.Sprintf("route:%s:%s", safeScopePart(anomaly.ChannelID), safeScopePart(anomaly.Model)),
			Domain: "routes", ObjectType: "route", ObjectID: strings.TrimSpace(anomaly.ChannelID), ObjectName: objectName,
			Severity: healthStatus, HealthStatus: healthStatus, ObservationStatus: dutyObservationReady,
			Title: objectName + " 路由异常", Summary: anomaly.LastError, LastSeenAt: anomaly.LastObservedAt,
			IncidentID: dutyFindingIncidentID(incidentKey, incidentIDs), Link: "/admin/log",
		})
	}
	finalizeInspectionDomain(&routeDomain, anomaliesErr)
	inspection.Domains = append(inspection.Domains, routeDomain)

	taskDomain := dutyInspectionDomain{
		Key: "tasks", MetricKind: "objects", TotalCount: int(taskTotal), ObservationStatus: dutyObservationReady,
		Link: "/admin/channel",
	}
	for _, task := range tasks {
		if !taskNeedsDutyAttention(task, endAt) {
			continue
		}
		addDomainHealth(&taskDomain, dutyHealthWarning)
		taskDomain.FindingCount++
		observedAt := maxTimestamp(task.CreatedAt, task.StartedAt, task.FinishedAt)
		taskDomain.LastChangedAt = maxTimestamp(taskDomain.LastChangedAt, observedAt)
		objectName := strings.TrimSpace(task.ChannelName)
		if objectName == "" {
			objectName = strings.TrimSpace(task.Model)
		}
		if objectName == "" {
			objectName = strings.TrimSpace(task.Type)
		}
		incidentKey, _ := incidentKeyForChannelOrModel(task.ChannelId, task.Model, "task:"+safeScopePart(task.Id))
		operationName := dutyOperationName(task.Type)
		inspection.Findings = append(inspection.Findings, dutyFinding{
			ID: "task:" + safeScopePart(task.Id), Domain: "tasks", ObjectType: "operation", ObjectID: task.Id, ObjectName: objectName,
			Severity: dutyHealthWarning, HealthStatus: dutyHealthWarning, ObservationStatus: dutyObservationReady,
			Title: objectName + " " + operationName + "需要处理", Summary: task.ErrorMessage, LastSeenAt: observedAt,
			IncidentID: dutyFindingIncidentID(incidentKey, incidentIDs), Link: dutyOperationLink(task),
		})
	}
	if taskDomain.TotalCount > taskDomain.WarningCount {
		taskDomain.HealthyCount = taskDomain.TotalCount - taskDomain.WarningCount
	}
	if taskDomain.TotalCount > 0 {
		taskDomain.CoverageRate = 1
	}
	finalizeInspectionDomain(&taskDomain, tasksErr)
	inspection.Domains = append(inspection.Domains, taskDomain)

	financeDomain := dutyInspectionDomain{
		Key: "finance", MetricKind: "findings", ObservationStatus: dutyObservationReady,
		FindingCount: len(financeIssues), CoverageRate: 1, Link: "/admin/finance?tab=overview",
	}
	for _, issue := range financeIssues {
		healthStatus := normalizedSeverity(issue.Level)
		addDomainHealth(&financeDomain, healthStatus)
		financeDomain.LastChangedAt = maxTimestamp(financeDomain.LastChangedAt, issue.CheckedAt)
		link := strings.TrimSpace(issue.Link)
		if link == "" || strings.HasPrefix(link, "/admin/finance/health") || strings.HasPrefix(link, "/admin/finance/procurement") {
			link = "/admin/finance?tab=overview"
		}
		incidentKey := financeIncidentKey(issue.Key)
		inspection.Findings = append(inspection.Findings, dutyFinding{
			ID: "finance:" + safeScopePart(issue.Key), Domain: "finance", ObjectType: "finance_check",
			ObjectID: issue.Key, ObjectName: issue.Title, Severity: healthStatus, HealthStatus: healthStatus,
			ObservationStatus: dutyObservationReady, Title: issue.Title, Summary: issue.Message,
			LastSeenAt: issue.CheckedAt, IncidentID: dutyFindingIncidentID(incidentKey, incidentIDs), Link: link,
		})
	}
	finalizeInspectionDomain(&financeDomain, nil)
	inspection.Domains = append(inspection.Domains, financeDomain)

	coverageTotal := 0.0
	coverageObjectCount := 0
	for index := range inspection.Domains {
		domain := &inspection.Domains[index]
		inspection.AbnormalCount += domain.WarningCount + domain.CriticalCount
		inspection.UnknownCount += domain.UnknownCount
		inspection.FindingCount += domain.FindingCount
		inspection.ObservationStatus = mergeObservationStatus(inspection.ObservationStatus, domain.ObservationStatus)
		if healthStatusRank(domain.HealthStatus) > healthStatusRank(inspection.HealthStatus) {
			inspection.HealthStatus = domain.HealthStatus
		}
		if domain.MetricKind == "objects" && (domain.Key == "channels" || domain.Key == "models") {
			coverageTotal += domain.CoverageRate * float64(domain.TotalCount)
			coverageObjectCount += domain.TotalCount
		}
	}
	if coverageObjectCount > 0 {
		inspection.CoverageRate = coverageTotal / float64(coverageObjectCount)
	}
	if inspection.HealthStatus == dutyHealthHealthy && inspection.ObservationStatus != dutyObservationReady {
		inspection.HealthStatus = dutyHealthUnknown
	}
	sort.SliceStable(inspection.Findings, func(i, j int) bool {
		if healthStatusRank(inspection.Findings[i].Severity) != healthStatusRank(inspection.Findings[j].Severity) {
			return healthStatusRank(inspection.Findings[i].Severity) > healthStatusRank(inspection.Findings[j].Severity)
		}
		return inspection.Findings[i].LastSeenAt > inspection.Findings[j].LastSeenAt
	})
	return inspection
}

func buildDutyIncidentResponse(window string, startAt int64, endAt int64) dutyIncidentResponse {
	var alerts []channelcontroller.DutyChannelAlert
	var anomalies []logcontroller.DutyRouteAnomaly
	var tasks []model.AsyncTask
	var financeIssues []adminbilling.DutyFinanceIssue
	var channelHealth []dashboardcontroller.DutyChannelHealth
	var modelHealth []dashboardcontroller.DutyModelHealth
	var alertsErr error
	var anomaliesErr error
	var tasksErr error
	var taskTotal int64
	var channelHealthErr error
	var modelHealthErr error
	var financeCheckedAt int64

	var wait sync.WaitGroup
	wait.Add(6)
	go func() {
		defer wait.Done()
		alerts, alertsErr = channelcontroller.ListDutyChannelAlerts(500)
	}()
	go func() {
		defer wait.Done()
		anomalies, anomaliesErr = logcontroller.ListDutyRouteAnomalies(startAt, endAt, 100)
	}()
	go func() {
		defer wait.Done()
		tasks, taskTotal, tasksErr = model.ListDutyAttentionTasksWithDB(
			model.DB,
			startAt,
			endAt,
			endAt-dutyTaskStaleAfterSecond,
		)
	}()
	go func() {
		defer wait.Done()
		financeIssues, financeCheckedAt = adminbilling.ListDutyFinanceIssues()
	}()
	go func() {
		defer wait.Done()
		channelHealth, channelHealthErr = dashboardcontroller.ListDutyChannelHealth()
	}()
	go func() {
		defer wait.Done()
		modelHealth, modelHealthErr = dashboardcontroller.ListDutyModelHealth(startAt, endAt)
	}()
	wait.Wait()

	filteredAlerts := filterDutyChannelAlerts(alerts, startAt, endAt)
	filteredTasks := make([]model.AsyncTask, 0, len(tasks))
	for _, task := range tasks {
		if model.NormalizeAsyncTaskType(task.Type) == model.AsyncTaskTypeChannelModelTest {
			continue
		}
		if !taskNeedsDutyAttention(task, endAt) {
			continue
		}
		filteredTasks = append(filteredTasks, task)
	}
	taskTotal = int64(len(filteredTasks))

	incidents := make(map[string]*dutyIncident)
	ensure := func(key string, scopeType string, title string, summary string, severity string, observedAt int64) *dutyIncident {
		if existing, ok := incidents[key]; ok {
			existing.StartedAt = minPositiveTimestamp(existing.StartedAt, observedAt)
			existing.LastSeenAt = maxTimestamp(existing.LastSeenAt, observedAt)
			raiseIncidentSeverity(existing, severity, title, summary)
			return existing
		}
		incident := newDutyIncident(key, scopeType, title, summary, severity, observedAt)
		incidents[key] = incident
		return incident
	}

	for _, alert := range filteredAlerts {
		key, scopeType := incidentKeyForChannelOrModel(alert.ChannelID, alert.Model, "alert:"+safeScopePart(alert.Type)+":"+safeScopePart(alert.ID))
		title := alert.Title
		if scopeType == "channel" && strings.TrimSpace(alert.ChannelName) != "" {
			title = alert.ChannelName + " 运行异常"
		}
		incident := ensure(key, scopeType, title, alert.Detail, alert.Level, alert.CreatedAt)
		incident.ChannelID = strings.TrimSpace(alert.ChannelID)
		incident.ChannelName = strings.TrimSpace(alert.ChannelName)
		addIncidentModel(incident, alert.Model)
		addIncidentEndpoint(incident, alert.Endpoint)
		incident.Signals = append(incident.Signals, dutySignal{
			ID:            "alert:" + alert.Type + ":" + alert.ID,
			Type:          "channel_alert",
			Level:         normalizedSeverity(alert.Level),
			Title:         alert.Title,
			Summary:       strings.TrimSpace(strings.Join([]string{alert.Summary, alert.Detail}, " ")),
			CreatedAt:     alert.CreatedAt,
			Link:          "/admin/dashboard?view=incidents&domain=channels&incident=" + url.QueryEscape(incident.ID),
			ReferenceType: alert.Type,
			ReferenceKey:  alert.ID,
			ChannelID:     alert.ChannelID,
		})
	}

	for _, anomaly := range anomalies {
		severity := "warning"
		if anomaly.FailureRate >= 0.2 || anomaly.FailureCount >= 100 {
			severity = "critical"
		}
		key, scopeType := incidentKeyForChannelOrModel(anomaly.ChannelID, anomaly.Model, "route:unknown")
		title := strings.TrimSpace(anomaly.Model) + " 路由异常"
		if scopeType == "channel" {
			label := strings.TrimSpace(anomaly.ChannelName)
			if label == "" {
				label = strings.TrimSpace(anomaly.ChannelID)
			}
			title = label + " 路由异常"
		}
		incident := ensure(key, scopeType, title, anomaly.LastError, severity, anomaly.LastObservedAt)
		if incident.ChannelID == "" {
			incident.ChannelID = strings.TrimSpace(anomaly.ChannelID)
			incident.ChannelName = strings.TrimSpace(anomaly.ChannelName)
		}
		addIncidentModel(incident, anomaly.Model)
		incident.Impact.RequestCount += anomaly.RequestCount
		incident.Impact.FailureCount += anomaly.FailureCount
		incident.Impact.FallbackCount += anomaly.FallbackCount
		incident.Signals = append(incident.Signals, dutySignal{
			ID:        fmt.Sprintf("route:%s:%s", safeScopePart(anomaly.ChannelID), safeScopePart(anomaly.Model)),
			Type:      "route_anomaly",
			Level:     normalizedSeverity(severity),
			Title:     title,
			Summary:   anomaly.LastError,
			CreatedAt: anomaly.LastObservedAt,
			Link:      "/admin/log",
		})
	}

	for _, task := range filteredTasks {
		key, scopeType := incidentKeyForChannelOrModel(task.ChannelId, task.Model, "task:"+safeScopePart(task.Id))
		severity := "warning"
		operationName := dutyOperationName(task.Type)
		title := operationName + "执行异常"
		if strings.TrimSpace(task.ChannelName) != "" {
			title = task.ChannelName + " " + operationName + "异常"
		} else if strings.TrimSpace(task.Model) != "" {
			title = task.Model + " " + operationName + "异常"
		}
		incident := ensure(key, scopeType, title, task.ErrorMessage, severity, maxTimestamp(task.CreatedAt, task.FinishedAt))
		if incident.ChannelID == "" {
			incident.ChannelID = strings.TrimSpace(task.ChannelId)
			incident.ChannelName = strings.TrimSpace(task.ChannelName)
		}
		addIncidentModel(incident, task.Model)
		if task.Status == model.AsyncTaskStatusFailed {
			incident.Impact.FailedTaskCount++
		}
		incident.Tasks = append(incident.Tasks, dutyTask{
			ID:           task.Id,
			Type:         task.Type,
			Status:       task.Status,
			Model:        task.Model,
			ErrorMessage: task.ErrorMessage,
			CreatedAt:    task.CreatedAt,
			FinishedAt:   task.FinishedAt,
			Link:         dutyOperationLink(task),
		})
	}

	for _, issue := range financeIssues {
		key := financeIncidentKey(issue.Key)
		link := strings.TrimSpace(issue.Link)
		if link == "" || strings.HasPrefix(link, "/admin/finance/health") || strings.HasPrefix(link, "/admin/finance/procurement") {
			link = "/admin/finance?tab=overview"
		}
		incident := ensure(key, "finance", issue.Title, issue.Message, issue.Level, issue.CheckedAt)
		incident.Impact.FinanceIssueCount++
		incident.Impact.FinancialRisk += int64(math.Max(1, float64(issue.Count)))
		incident.FinanceIssues = append(incident.FinanceIssues, dutyFinanceIssue{
			Key: issue.Key, Level: normalizedSeverity(issue.Level), Title: issue.Title,
			Message: issue.Message, Count: issue.Count, Link: link,
		})
		incident.Signals = append(incident.Signals, dutySignal{
			ID: "finance:" + issue.Key, Type: "finance_issue", Level: normalizedSeverity(issue.Level),
			Title: issue.Title, Summary: issue.Message, CreatedAt: issue.CheckedAt, Link: link,
		})
	}

	keys := make([]string, 0, len(incidents))
	for key := range incidents {
		keys = append(keys, key)
	}
	states, statesErr := model.ListDutyIncidentStatesByKeysWithDB(model.DB, keys)

	result := make([]dutyIncident, 0, len(incidents))
	for _, incident := range incidents {
		sort.Strings(incident.Models)
		sort.Strings(incident.Endpoints)
		sort.SliceStable(incident.Signals, func(i, j int) bool { return incident.Signals[i].CreatedAt > incident.Signals[j].CreatedAt })
		sort.SliceStable(incident.Tasks, func(i, j int) bool { return incident.Tasks[i].CreatedAt > incident.Tasks[j].CreatedAt })
		incident.Impact.AffectedModels = len(incident.Models)
		if incident.Impact.RequestCount > 0 {
			incident.Impact.FailureRate = float64(incident.Impact.FailureCount) / float64(incident.Impact.RequestCount)
		}
		if state, ok := states[incident.ID]; ok {
			applyDutyIncidentState(incident, state)
		}
		configureIncidentLinks(incident, startAt, endAt)
		calculateIncidentPriority(incident, endAt)
		incident.modelSet = nil
		incident.endpointSet = nil
		result = append(result, *incident)
	}
	sort.SliceStable(result, func(i, j int) bool {
		leftResolved := result[i].Status == model.DutyIncidentStatusResolved
		rightResolved := result[j].Status == model.DutyIncidentStatusResolved
		if leftResolved != rightResolved {
			return !leftResolved
		}
		if result[i].PriorityScore != result[j].PriorityScore {
			return result[i].PriorityScore > result[j].PriorityScore
		}
		return result[i].LastSeenAt > result[j].LastSeenAt
	})

	sources := []dutySourceState{
		sourceState("channels", len(channelHealth), endAt, channelHealthErr),
		sourceState("models", len(modelHealth), endAt, modelHealthErr),
		sourceState("alerts", len(filteredAlerts), endAt, alertsErr),
		sourceState("routes", len(anomalies), endAt, anomaliesErr),
		sourceState("tasks", len(filteredTasks), endAt, tasksErr),
		sourceState("finance", len(financeIssues), financeCheckedAt, nil),
	}
	if statesErr != nil {
		sources = append(sources, sourceState("incident_states", 0, endAt, statesErr))
	} else {
		sources = append(sources, sourceState("incident_states", len(states), endAt, nil))
	}
	response := dutyIncidentResponse{
		OverallStatus: "healthy",
		Window:        window,
		WindowStartAt: startAt,
		WindowEndAt:   endAt,
		GeneratedAt:   helper.GetTimestamp(),
		Sources:       sources,
		Incidents:     result,
	}
	response.Inspection = buildDutyInspection(
		endAt,
		channelHealth,
		channelHealthErr,
		modelHealth,
		modelHealthErr,
		anomalies,
		anomaliesErr,
		tasks,
		taskTotal,
		tasksErr,
		financeIssues,
		result,
	)
	modelNames := map[string]struct{}{}
	for _, incident := range result {
		if incident.Status == model.DutyIncidentStatusResolved {
			continue
		}
		response.Summary.ActiveIncidents++
		if incident.Severity == "critical" {
			response.Summary.Critical++
		} else {
			response.Summary.Warning++
		}
		if strings.TrimSpace(incident.AssigneeUserID) == "" {
			response.Summary.Unassigned++
		}
		response.Summary.FailureCount += incident.Impact.FailureCount
		response.Summary.FallbackCount += incident.Impact.FallbackCount
		response.Summary.FailedTaskCount += incident.Impact.FailedTaskCount
		response.Summary.FinanceIssueCount += incident.Impact.FinanceIssueCount
		for _, modelName := range incident.Models {
			modelNames[modelName] = struct{}{}
		}
	}
	response.Summary.AffectedModels = len(modelNames)
	if response.Summary.Critical > 0 || response.Inspection.HealthStatus == dutyHealthCritical {
		response.OverallStatus = "critical"
	} else if response.Summary.Warning > 0 || response.Inspection.HealthStatus == dutyHealthWarning {
		response.OverallStatus = "warning"
	} else if response.Inspection.HealthStatus == dutyHealthUnknown || response.Inspection.ObservationStatus != dutyObservationReady {
		response.OverallStatus = "unknown"
	} else {
		for _, source := range sources {
			if source.Status == dutySourceError {
				response.OverallStatus = "unknown"
				break
			}
		}
	}
	return response
}

func configureIncidentLinks(incident *dutyIncident, startAt int64, endAt int64) {
	if incident == nil {
		return
	}
	if incident.ChannelID != "" {
		incident.Links.Channel = "/admin/channel/detail/" + incident.ChannelID + "?tab=tests"
		incident.Links.Logs = fmt.Sprintf("/admin/log?channel=%s&start_timestamp=%d&end_timestamp=%d", incident.ChannelID, startAt, endAt)
		incident.Links.Finance = "/admin/finance?tab=overview&channel_id=" + incident.ChannelID
	}
	if len(incident.Models) > 0 {
		modelName := incident.Models[0]
		incident.Links.Models = "/workspace/service/models?tab=operations&model=" + modelName
		if incident.Links.Logs == "" {
			incident.Links.Logs = fmt.Sprintf("/admin/log?model_name=%s&start_timestamp=%d&end_timestamp=%d", modelName, startAt, endAt)
		}
		if incident.Links.Finance == "" {
			incident.Links.Finance = "/admin/finance?tab=overview&model=" + modelName
		}
	}
	if incident.ScopeType == "finance" && len(incident.FinanceIssues) > 0 {
		incident.Links.Finance = incident.FinanceIssues[0].Link
	}
}

func calculateIncidentPriority(incident *dutyIncident, now int64) {
	if incident == nil {
		return
	}
	score := 60
	if incident.Severity == "critical" {
		score = 100
	}
	score += int(math.Min(40, float64(incident.Impact.FailureCount)/25))
	score += int(math.Min(20, float64(incident.Impact.FallbackCount)/25))
	score += int(math.Min(36, float64(incident.Impact.FailedTaskCount*12)))
	score += int(math.Min(30, float64(incident.Impact.FinanceIssueCount*15)))
	score += int(math.Min(20, float64(len(incident.Models)*4)))
	if incident.StartedAt > 0 && now > incident.StartedAt {
		score += int(math.Min(10, float64((now-incident.StartedAt)/(60*60))))
	}
	incident.PriorityScore = score
	switch {
	case incident.Severity == "critical" && score >= 140:
		incident.Priority = "P0"
	case incident.Severity == "critical" || score >= 95:
		incident.Priority = "P1"
	default:
		incident.Priority = "P2"
	}
}

func GetDutyIncidents(c *gin.Context) {
	window, seconds := parseDutyWindow(c.Query("window"))
	endAt := helper.GetTimestamp()
	response := buildDutyIncidentResponse(window, endAt-seconds, endAt)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": response})
}

func persistDutyIncidentStateWithDB(
	db *gorm.DB,
	incidentKey string,
	status string,
	assigneeUserID string,
	assigneeUsername string,
	note string,
	signals []dutySignal,
) (model.DutyIncidentState, error) {
	if db == nil {
		return model.DutyIncidentState{}, fmt.Errorf("database handle is nil")
	}
	normalizedStatus := model.NormalizeDutyIncidentStatus(status)
	if normalizedStatus == "" {
		return model.DutyIncidentState{}, fmt.Errorf("invalid duty incident status")
	}
	row := model.DutyIncidentState{}
	err := db.Transaction(func(tx *gorm.DB) error {
		updated, err := model.UpsertDutyIncidentStateWithDB(
			tx,
			incidentKey,
			normalizedStatus,
			assigneeUserID,
			assigneeUsername,
			note,
		)
		if err != nil {
			return err
		}
		row = updated
		seen := make(map[string]struct{}, len(signals))
		for _, signal := range signals {
			if signal.ReferenceType == "" || signal.ReferenceKey == "" || signal.ChannelID == "" {
				continue
			}
			key := signal.ReferenceType + "::" + signal.ReferenceKey + "::" + signal.ChannelID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			ref := model.ChannelAlertStateRef{AlertType: signal.ReferenceType, AlertKey: signal.ReferenceKey}
			switch normalizedStatus {
			case model.DutyIncidentStatusActive:
				_, err = model.ReopenChannelAlertStateWithDB(tx, ref, signal.ChannelID, note)
			case model.DutyIncidentStatusResolved:
				_, err = model.ResolveChannelAlertStateWithDB(tx, ref, signal.ChannelID, assigneeUserID, note)
			default:
				_, err = model.AcknowledgeChannelAlertStateWithDB(tx, ref, signal.ChannelID, assigneeUserID, note)
			}
			if err != nil {
				return fmt.Errorf("update linked channel alert %s: %w", key, err)
			}
		}
		return nil
	})
	if err != nil {
		return model.DutyIncidentState{}, err
	}
	return row, nil
}

func UpdateDutyIncident(c *gin.Context) {
	incidentKey := strings.TrimSpace(c.Param("id"))
	request := updateDutyIncidentRequest{}
	if incidentKey == "" || len(incidentKey) > 255 || c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "值班事件参数无效"})
		return
	}
	status := model.NormalizeDutyIncidentStatus(request.Status)
	if status == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "值班事件状态无效"})
		return
	}
	window, seconds := parseDutyWindow(c.Query("window"))
	endAt := helper.GetTimestamp()
	response := buildDutyIncidentResponse(window, endAt-seconds, endAt)
	var signals []dutySignal
	found := false
	for _, incident := range response.Incidents {
		if incident.ID != incidentKey {
			continue
		}
		found = true
		signals = incident.Signals
		break
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "值班事件不存在或已超出当前队列"})
		return
	}
	row, err := persistDutyIncidentStateWithDB(
		model.DB,
		incidentKey,
		status,
		strings.TrimSpace(c.GetString(ctxkey.Id)),
		strings.TrimSpace(c.GetString(ctxkey.Username)),
		request.Note,
		signals,
	)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}
