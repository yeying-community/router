package config

import (
	"os"
	"strings"
)

const (
	TopUpModeRedirect = "redirect"
	TopUpModeAPI      = "api"
)

func normalizeTopUpExternalPayURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	// The current yeying-room API examples use public/index.php, while some admin-generated
	// docs may expose backend.php paths that are not routable for external-pay endpoints.
	if strings.Contains(trimmed, "/public/backend.php/addons/ymqzixishi/api.external_pay/") {
		return strings.Replace(trimmed, "/public/backend.php/", "/public/index.php/", 1)
	}
	return trimmed
}

func EffectiveTopUpMode() string {
	switch strings.TrimSpace(strings.ToLower(TopUpMode)) {
	case TopUpModeAPI:
		return TopUpModeAPI
	case TopUpModeRedirect:
		return TopUpModeRedirect
	case "":
		if strings.TrimSpace(TopUpAPICreateURL) != "" {
			return TopUpModeAPI
		}
		return TopUpModeRedirect
	default:
		if strings.TrimSpace(TopUpAPICreateURL) != "" {
			return TopUpModeAPI
		}
		return TopUpModeRedirect
	}
}

func ResolvedTopUpAPICreateURL() string {
	if value := normalizeTopUpExternalPayURL(TopUpAPICreateURL); value != "" {
		return value
	}
	if EffectiveTopUpMode() != TopUpModeAPI {
		return ""
	}
	link := normalizeTopUpExternalPayURL(TopUpLink)
	if link == "" {
		return ""
	}
	if strings.Contains(link, "/api.external_pay/cashier") {
		return strings.Replace(link, "/api.external_pay/cashier", "/api.external_pay/create", 1)
	}
	if strings.HasSuffix(link, "/cashier") {
		return strings.TrimSuffix(link, "/cashier") + "/create"
	}
	return ""
}

func ResolvedTopUpAPIQueryURL() string {
	if value := normalizeTopUpExternalPayURL(TopUpAPIQueryURL); value != "" {
		return value
	}
	if EffectiveTopUpMode() != TopUpModeAPI {
		return ""
	}
	createURL := strings.TrimSpace(ResolvedTopUpAPICreateURL())
	if createURL == "" {
		return ""
	}
	if strings.Contains(createURL, "/api.external_pay/create") {
		return strings.Replace(createURL, "/api.external_pay/create", "/api.external_pay/query", 1)
	}
	if strings.HasSuffix(createURL, "/create") {
		return strings.TrimSuffix(createURL, "/create") + "/query"
	}
	return ""
}

func TopUpAvailabilityEndpoint() string {
	if EffectiveTopUpMode() == TopUpModeAPI {
		return ResolvedTopUpAPICreateURL()
	}
	return strings.TrimSpace(TopUpLink)
}

func TopUpMerchantAppValue() string {
	if value := strings.TrimSpace(TopUpMerchantApp); value != "" {
		return value
	}
	return "router"
}

func TopUpAPIUniacidValue() int {
	if TopUpAPIUniacid > 0 {
		return TopUpAPIUniacid
	}
	return 1
}

func TopUpAPITimeoutSecondsValue() int {
	if TopUpAPITimeoutSeconds > 0 {
		return TopUpAPITimeoutSeconds
	}
	return 15
}

func ConfiguredTopUpCallbackToken() string {
	if token := strings.TrimSpace(TopUpCallbackToken); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("TOPUP_CALLBACK_TOKEN"))
}

func TopUpModeIssues() []string {
	issues := make([]string, 0, 1)
	switch EffectiveTopUpMode() {
	case TopUpModeAPI:
		if strings.TrimSpace(ResolvedTopUpAPICreateURL()) == "" {
			issues = append(issues, "operation.top_up_api_create_url is empty")
		}
	default:
		if strings.TrimSpace(TopUpLink) == "" {
			issues = append(issues, "operation.top_up_link is empty")
		}
	}
	return issues
}

func TopUpCreateIssues() []string {
	issues := append(make([]string, 0, 3), TopUpModeIssues()...)
	if strings.TrimSpace(ServerAddress) == "" {
		issues = append(issues, "server.address is empty")
	}
	if strings.TrimSpace(TopUpSignSecret) == "" {
		issues = append(issues, "operation.top_up_sign_secret is empty")
	}
	return issues
}

func TopUpCallbackIssues() []string {
	issues := make([]string, 0, 1)
	if strings.TrimSpace(ConfiguredTopUpCallbackToken()) == "" {
		issues = append(issues, "operation.top_up_callback_token is empty")
	}
	return issues
}

func TopUpConfigIssues() []string {
	issues := append(make([]string, 0, 4), TopUpCreateIssues()...)
	issues = append(issues, TopUpCallbackIssues()...)
	return issues
}

func TopUpFeatureEnabled() bool {
	return len(TopUpCreateIssues()) == 0
}
