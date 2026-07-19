package channel

import (
	"strings"
	"testing"
)

func TestSanitizeBillingAlertReasonMasksURLAndHost(t *testing.T) {
	input := `Get "https://billing.example.com/api/public/usage/stats?credential=X9R5SVBSME3S": dial tcp: lookup billing.example.com: no such host`
	got := sanitizeBillingAlertReason(assertErrString(input))
	if strings.Contains(got, "billing.example.com") {
		t.Fatalf("sanitized reason leaked host: %q", got)
	}
	if strings.Contains(got, "X9R5SVBSME3S") {
		t.Fatalf("sanitized reason leaked credential: %q", got)
	}
	if got != "网络错误：账务服务域名解析失败" {
		t.Fatalf("unexpected sanitized reason: %q", got)
	}
}

func TestSanitizeBillingAlertReasonMasksGenericResponseError(t *testing.T) {
	input := `Get "https://api.example.com/v1/billing?token=abc123": unexpected end of JSON input from api.example.com`
	got := sanitizeBillingAlertReason(assertErrString(input))
	if strings.Contains(got, "api.example.com") {
		t.Fatalf("sanitized reason leaked host: %q", got)
	}
	if strings.Contains(got, "abc123") {
		t.Fatalf("sanitized reason leaked token: %q", got)
	}
	if !strings.Contains(got, "[已脱敏]") && !strings.Contains(got, "[已脱敏地址]") {
		t.Fatalf("expected sanitized placeholders, got %q", got)
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}

func assertErrString(value string) error {
	return errString(value)
}
