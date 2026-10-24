package duty

import (
	"strings"
	"testing"

	channelcontroller "github.com/yeying-community/router/internal/admin/controller/channel"
	dashboardcontroller "github.com/yeying-community/router/internal/admin/controller/dashboard"
	"github.com/yeying-community/router/internal/admin/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDutyHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DutyIncidentState{}, &model.ChannelAlertState{}); err != nil {
		t.Fatalf("migrate duty state tables: %v", err)
	}
	return db
}

func TestParseDutyWindow(t *testing.T) {
	tests := []struct {
		input       string
		wantWindow  string
		wantSeconds int64
	}{
		{input: "1h", wantWindow: "1h", wantSeconds: 60 * 60},
		{input: "24H", wantWindow: "24h", wantSeconds: 24 * 60 * 60},
		{input: "unexpected", wantWindow: "6h", wantSeconds: 6 * 60 * 60},
	}
	for _, test := range tests {
		window, seconds := parseDutyWindow(test.input)
		if window != test.wantWindow || seconds != test.wantSeconds {
			t.Fatalf("parseDutyWindow(%q) = (%q, %d), want (%q, %d)", test.input, window, seconds, test.wantWindow, test.wantSeconds)
		}
	}
}

func TestConfigureIncidentLinksDoesNotExposeLegacyAlertPage(t *testing.T) {
	incident := newDutyIncident("channel:channel-1", "channel", "Channel issue", "", "warning", 100)
	incident.ChannelID = "channel-1"

	configureIncidentLinks(incident, 100, 200)

	if incident.Links.Alerts != "" {
		t.Fatalf("configureIncidentLinks() alerts = %q, want empty", incident.Links.Alerts)
	}
	for _, link := range []string{
		incident.Links.Channel,
		incident.Links.Tasks,
		incident.Links.Logs,
		incident.Links.Models,
		incident.Links.Finance,
	} {
		if strings.Contains(link, "tab=alerts") {
			t.Fatalf("configureIncidentLinks() exposed legacy alert link %q", link)
		}
	}
}

func TestTaskNeedsDutyAttention(t *testing.T) {
	const now = int64(10_000)
	tests := []struct {
		name string
		task model.AsyncTask
		want bool
	}{
		{name: "failed immediately", task: model.AsyncTask{Status: model.AsyncTaskStatusFailed, CreatedAt: now - 10}, want: true},
		{name: "recent pending", task: model.AsyncTask{Status: model.AsyncTaskStatusPending, CreatedAt: now - 60}, want: false},
		{name: "stale pending", task: model.AsyncTask{Status: model.AsyncTaskStatusPending, CreatedAt: now - dutyTaskStaleAfterSecond}, want: true},
		{name: "recent running uses start", task: model.AsyncTask{Status: model.AsyncTaskStatusRunning, CreatedAt: now - 5000, StartedAt: now - 60}, want: false},
		{name: "stale running", task: model.AsyncTask{Status: model.AsyncTaskStatusRunning, StartedAt: now - dutyTaskStaleAfterSecond - 1}, want: true},
		{name: "succeeded", task: model.AsyncTask{Status: model.AsyncTaskStatusSucceeded, CreatedAt: now - 5000}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := taskNeedsDutyAttention(test.task, now); got != test.want {
				t.Fatalf("taskNeedsDutyAttention() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDutyOperationName(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		want     string
	}{
		{name: "model test", taskType: model.AsyncTaskTypeChannelModelTest, want: "模型测试"},
		{name: "model refresh", taskType: model.AsyncTaskTypeChannelRefreshModels, want: "模型同步"},
		{name: "billing refresh", taskType: model.AsyncTaskTypeChannelRefreshBilling, want: "账务刷新"},
		{name: "provider usage", taskType: model.AsyncTaskTypeChannelSyncProviderUsage, want: "用量同步"},
		{name: "unknown", taskType: "unknown", want: "后台执行"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dutyOperationName(test.taskType); got != test.want {
				t.Fatalf("dutyOperationName(%q) = %q, want %q", test.taskType, got, test.want)
			}
		})
	}
}

func TestDutyOperationLink(t *testing.T) {
	tests := []struct {
		name string
		task model.AsyncTask
		want string
	}{
		{
			name: "model test preserves scoped filter safely",
			task: model.AsyncTask{
				Type:      model.AsyncTaskTypeChannelModelTest,
				ChannelId: "channel/one",
				Model:     "qwen image/plus",
				Endpoint:  "/v1/images/edits?mode=mask",
			},
			want: "/admin/channel/detail/channel%2Fone?endpoint=%2Fv1%2Fimages%2Fedits%3Fmode%3Dmask&model=qwen+image%2Fplus&tab=tests",
		},
		{
			name: "model refresh",
			task: model.AsyncTask{Type: model.AsyncTaskTypeChannelRefreshModels, ChannelId: "channel-1"},
			want: "/admin/channel/detail/channel-1?tab=models",
		},
		{
			name: "billing refresh",
			task: model.AsyncTask{Type: model.AsyncTaskTypeChannelRefreshBilling, ChannelId: "channel-1"},
			want: "/admin/channel/detail/channel-1?tab=procurement",
		},
		{
			name: "provider usage sync",
			task: model.AsyncTask{Type: model.AsyncTaskTypeChannelSyncProviderUsage, ChannelId: "channel-1"},
			want: "/admin/channel/detail/channel-1?tab=procurement",
		},
		{
			name: "missing channel",
			task: model.AsyncTask{Type: model.AsyncTaskTypeChannelModelTest},
			want: "/admin/channel",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dutyOperationLink(test.task); got != test.want {
				t.Fatalf("dutyOperationLink() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFilterDutyChannelAlertsKeepsOpenAlertsAcrossWindows(t *testing.T) {
	alerts := []channelcontroller.DutyChannelAlert{
		{ID: "old-open", Level: "warning", Status: model.ChannelAlertStatusActive, CreatedAt: 10},
		{ID: "future-open", Level: "warning", Status: model.ChannelAlertStatusActive, CreatedAt: 201},
		{ID: "recent-resolved", Level: "warning", Status: model.ChannelAlertStatusResolved, CreatedAt: 150},
		{ID: "old-resolved", Level: "warning", Status: model.ChannelAlertStatusResolved, CreatedAt: 10},
		{ID: "info", Level: "info", Status: model.ChannelAlertStatusActive, CreatedAt: 150},
	}

	filtered := filterDutyChannelAlerts(alerts, 100, 200)
	if len(filtered) != 2 {
		t.Fatalf("filtered alerts = %d, want the old open and recent resolved alerts", len(filtered))
	}
	if filtered[0].ID != "old-open" || filtered[1].ID != "recent-resolved" {
		t.Fatalf("filtered alert IDs = [%s, %s], want [old-open, recent-resolved]", filtered[0].ID, filtered[1].ID)
	}
}

func TestPersistDutyIncidentStateSynchronizesLinkedAlertsAtomically(t *testing.T) {
	db := openDutyHandlerTestDB(t)
	signals := []dutySignal{{
		ReferenceType: model.ChannelAlertTypeCircuitBreaker,
		ReferenceKey:  "alert-1",
		ChannelID:     "channel-1",
	}}

	state, err := persistDutyIncidentStateWithDB(
		db,
		"channel:channel-1",
		model.DutyIncidentStatusInvestigating,
		"user-1",
		"operator",
		"checking impact",
		signals,
	)
	if err != nil {
		t.Fatalf("persist investigating state: %v", err)
	}
	if state.Status != model.DutyIncidentStatusInvestigating || state.LastNote != "checking impact" {
		t.Fatalf("unexpected incident state: %+v", state)
	}
	alert := model.ChannelAlertState{}
	if err := db.Where("alert_type = ? AND alert_key = ?", model.ChannelAlertTypeCircuitBreaker, "alert-1").Take(&alert).Error; err != nil {
		t.Fatalf("load linked alert state: %v", err)
	}
	if alert.Status != model.ChannelAlertStatusAcknowledged || alert.AcknowledgedBy != "user-1" {
		t.Fatalf("unexpected linked alert acknowledgement: %+v", alert)
	}

	if _, err := persistDutyIncidentStateWithDB(
		db,
		"channel:channel-1",
		model.DutyIncidentStatusResolved,
		"user-1",
		"operator",
		"recovered",
		signals,
	); err != nil {
		t.Fatalf("persist resolved state: %v", err)
	}
	if err := db.Where("alert_type = ? AND alert_key = ?", model.ChannelAlertTypeCircuitBreaker, "alert-1").Take(&alert).Error; err != nil {
		t.Fatalf("reload linked alert state: %v", err)
	}
	if alert.Status != model.ChannelAlertStatusResolved || alert.ResolvedBy != "user-1" {
		t.Fatalf("unexpected linked alert resolution: %+v", alert)
	}

	if _, err := persistDutyIncidentStateWithDB(
		db,
		"channel:channel-1",
		model.DutyIncidentStatusActive,
		"",
		"",
		"regressed",
		signals,
	); err != nil {
		t.Fatalf("reopen incident: %v", err)
	}
	if err := db.Where("alert_type = ? AND alert_key = ?", model.ChannelAlertTypeCircuitBreaker, "alert-1").Take(&alert).Error; err != nil {
		t.Fatalf("reload reopened alert state: %v", err)
	}
	if alert.Status != model.ChannelAlertStatusActive || alert.ResolvedAt != 0 || alert.ResolvedBy != "" {
		t.Fatalf("unexpected linked alert after reopening: %+v", alert)
	}
}

func TestPersistDutyIncidentStateRollsBackWhenLinkedAlertFails(t *testing.T) {
	db := openDutyHandlerTestDB(t)
	signals := []dutySignal{
		{
			ReferenceType: model.ChannelAlertTypeCircuitBreaker,
			ReferenceKey:  "valid-alert",
			ChannelID:     "channel-1",
		},
		{
			ReferenceType: "unsupported",
			ReferenceKey:  "invalid-alert",
			ChannelID:     "channel-1",
		},
	}
	if _, err := persistDutyIncidentStateWithDB(
		db,
		"channel:channel-1",
		model.DutyIncidentStatusAcknowledged,
		"user-1",
		"operator",
		"acknowledged",
		signals,
	); err == nil {
		t.Fatal("expected an invalid linked alert to fail the transaction")
	}
	var incidentCount int64
	if err := db.Model(&model.DutyIncidentState{}).Where("incident_key = ?", "channel:channel-1").Count(&incidentCount).Error; err != nil {
		t.Fatalf("count incident state: %v", err)
	}
	var alertCount int64
	if err := db.Model(&model.ChannelAlertState{}).Count(&alertCount).Error; err != nil {
		t.Fatalf("count alert states: %v", err)
	}
	if incidentCount != 0 || alertCount != 0 {
		t.Fatalf("failed update left persisted rows: incidents=%d alerts=%d", incidentCount, alertCount)
	}
}

func TestCalculateIncidentPriority(t *testing.T) {
	critical := newDutyIncident("channel:1", "channel", "critical", "", "critical", 1)
	critical.Impact.FailureCount = 1000
	critical.Impact.FailedTaskCount = 2
	calculateIncidentPriority(critical, 8*60*60)
	if critical.Priority != "P0" {
		t.Fatalf("critical priority = %q, want P0 (score %d)", critical.Priority, critical.PriorityScore)
	}

	warning := newDutyIncident("channel:2", "channel", "warning", "", "warning", 1)
	calculateIncidentPriority(warning, 60)
	if warning.Priority != "P2" {
		t.Fatalf("warning priority = %q, want P2 (score %d)", warning.Priority, warning.PriorityScore)
	}
}

func TestApplyDutyIncidentStateReopensOnNewSignal(t *testing.T) {
	incident := newDutyIncident("channel:1", "channel", "test", "", "critical", 100)
	incident.LastSeenAt = 300
	applyDutyIncidentState(incident, model.DutyIncidentState{
		Status:           model.DutyIncidentStatusResolved,
		AssigneeUserId:   "user-1",
		AssigneeUsername: "operator",
		LastNote:         "recovered",
		AcknowledgedAt:   150,
		MitigatedAt:      180,
		ResolvedAt:       200,
	})
	if incident.Status != model.DutyIncidentStatusActive {
		t.Fatalf("incident status = %q, want active", incident.Status)
	}
	if incident.ResolvedAt != 0 {
		t.Fatalf("incident resolved_at = %d, want 0", incident.ResolvedAt)
	}
	if incident.AssigneeUsername != "operator" || incident.LastNote != "recovered" {
		t.Fatalf("incident handling metadata was not retained: %+v", incident)
	}
}

func TestFinalizeInspectionDomainDoesNotTreatUnknownAsHealthy(t *testing.T) {
	domain := dutyInspectionDomain{
		Key:               "channels",
		MetricKind:        "objects",
		TotalCount:        1,
		UnknownCount:      1,
		ObservationStatus: dutyObservationInsufficient,
	}
	finalizeInspectionDomain(&domain, nil)
	if domain.HealthStatus != dutyHealthUnknown {
		t.Fatalf("health status = %q, want unknown", domain.HealthStatus)
	}
	if domain.ObservationStatus != dutyObservationInsufficient {
		t.Fatalf("observation status = %q, want insufficient", domain.ObservationStatus)
	}
}

func TestBuildDutyInspectionNoIncidentsWithInsufficientObservationIsUnknown(t *testing.T) {
	inspection := buildDutyInspection(
		10_000,
		[]dashboardcontroller.DutyChannelHealth{{
			ID: "channel-1", Name: "channel-1", HealthLevel: dutyHealthHealthy,
			HasTestData: false, CoverageRate: 0,
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		0,
		nil,
		nil,
		nil,
	)
	if inspection.HealthStatus != dutyHealthUnknown {
		t.Fatalf("inspection health = %q, want unknown", inspection.HealthStatus)
	}
	if inspection.ObservationStatus != dutyObservationInsufficient {
		t.Fatalf("inspection observation = %q, want insufficient", inspection.ObservationStatus)
	}
	if inspection.FindingCount == 0 {
		t.Fatal("expected an observation finding")
	}
}

func TestBuildDutyInspectionLinksExistingIncident(t *testing.T) {
	incident := newDutyIncident(channelIncidentKey("channel-1"), "channel", "test", "", dutyHealthCritical, 9_000)
	inspection := buildDutyInspection(
		10_000,
		[]dashboardcontroller.DutyChannelHealth{{
			ID: "channel-1", Name: "channel-1", HealthLevel: dutyHealthCritical,
			HasTestData: true, CoverageRate: 1, LastTestedAt: 9_900,
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		0,
		nil,
		nil,
		[]dutyIncident{*incident},
	)
	if len(inspection.Findings) == 0 {
		t.Fatal("expected a channel finding")
	}
	if inspection.Findings[0].IncidentID != incident.ID {
		t.Fatalf("finding incident = %q, want %q", inspection.Findings[0].IncidentID, incident.ID)
	}
}
