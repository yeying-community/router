package config

import "testing"

func TestResolvedTopUpAPIQueryURL(t *testing.T) {
	previousMode := TopUpMode
	previousLink := TopUpLink
	previousCreateURL := TopUpAPICreateURL
	previousQueryURL := TopUpAPIQueryURL
	t.Cleanup(func() {
		TopUpMode = previousMode
		TopUpLink = previousLink
		TopUpAPICreateURL = previousCreateURL
		TopUpAPIQueryURL = previousQueryURL
	})

	TopUpMode = TopUpModeAPI
	TopUpLink = ""
	TopUpAPIQueryURL = ""
	TopUpAPICreateURL = "https://pay.example.com/addons/ymq_zixishi/public/index.php/addons/ymqzixishi/api.external_pay/create?uniacid=1"

	if got := ResolvedTopUpAPIQueryURL(); got != "https://pay.example.com/addons/ymq_zixishi/public/index.php/addons/ymqzixishi/api.external_pay/query?uniacid=1" {
		t.Fatalf("unexpected query url: %q", got)
	}
}

func TestResolvedTopUpAPIQueryURLDerivesFromLegacyCreateURLInNodeMode(t *testing.T) {
	previousMode := TopUpMode
	previousLink := TopUpLink
	previousCreateURL := TopUpAPICreateURL
	previousQueryURL := TopUpAPIQueryURL
	t.Cleanup(func() {
		TopUpMode = previousMode
		TopUpLink = previousLink
		TopUpAPICreateURL = previousCreateURL
		TopUpAPIQueryURL = previousQueryURL
	})

	TopUpMode = TopUpModeNode
	TopUpLink = ""
	TopUpAPICreateURL = "https://pay.example.com/addons/ymqzixishi/api.external_pay/create?uniacid=1"
	TopUpAPIQueryURL = ""

	want := "https://pay.example.com/addons/ymqzixishi/api.external_pay/query?uniacid=1"
	if got := ResolvedTopUpAPIQueryURL(); got != want {
		t.Fatalf("legacy query url in Node mode = %q, want %q", got, want)
	}
}

func TestResolvedTopUpAPICreateURLNormalizesBackendPHP(t *testing.T) {
	previousMode := TopUpMode
	previousLink := TopUpLink
	previousCreateURL := TopUpAPICreateURL
	t.Cleanup(func() {
		TopUpMode = previousMode
		TopUpLink = previousLink
		TopUpAPICreateURL = previousCreateURL
	})

	TopUpMode = TopUpModeAPI
	TopUpLink = ""
	TopUpAPICreateURL = "https://www.tidukongjian.com/addons/ymq_zixishi/public/backend.php/addons/ymqzixishi/api.external_pay/create"

	if got := ResolvedTopUpAPICreateURL(); got != "https://www.tidukongjian.com/addons/ymq_zixishi/public/index.php/addons/ymqzixishi/api.external_pay/create" {
		t.Fatalf("unexpected normalized create url: %q", got)
	}
}

func TestResolvedTopUpAPIQueryURLNormalizesBackendPHP(t *testing.T) {
	previousMode := TopUpMode
	previousLink := TopUpLink
	previousCreateURL := TopUpAPICreateURL
	previousQueryURL := TopUpAPIQueryURL
	t.Cleanup(func() {
		TopUpMode = previousMode
		TopUpLink = previousLink
		TopUpAPICreateURL = previousCreateURL
		TopUpAPIQueryURL = previousQueryURL
	})

	TopUpMode = TopUpModeAPI
	TopUpLink = ""
	TopUpAPICreateURL = ""
	TopUpAPIQueryURL = "https://www.tidukongjian.com/addons/ymq_zixishi/public/backend.php/addons/ymqzixishi/api.external_pay/query?uniacid=1"

	if got := ResolvedTopUpAPIQueryURL(); got != "https://www.tidukongjian.com/addons/ymq_zixishi/public/index.php/addons/ymqzixishi/api.external_pay/query?uniacid=1" {
		t.Fatalf("unexpected normalized query url: %q", got)
	}
}

func TestTopUpNodeModeRequiresNodeCredentials(t *testing.T) {
	previousMode := TopUpMode
	previousURL := TopUpNodeURL
	previousMerchantID := TopUpNodeMerchantID
	previousMerchantSecret := TopUpNodeMerchantSecret
	previousServerAddress := ServerAddress
	previousSignSecret := TopUpSignSecret
	t.Cleanup(func() {
		TopUpMode = previousMode
		TopUpNodeURL = previousURL
		TopUpNodeMerchantID = previousMerchantID
		TopUpNodeMerchantSecret = previousMerchantSecret
		ServerAddress = previousServerAddress
		TopUpSignSecret = previousSignSecret
	})

	TopUpMode = TopUpModeNode
	TopUpNodeURL = "https://node.example.com"
	TopUpNodeMerchantID = "pm_test"
	TopUpNodeMerchantSecret = "merchant-secret"
	ServerAddress = "https://router.example.com"
	TopUpSignSecret = ""
	if got := TopUpConfigIssues(); len(got) != 0 {
		t.Fatalf("valid Node configuration has issues: %v", got)
	}
	TopUpNodeMerchantSecret = ""
	if got := TopUpCallbackIssues(); len(got) != 1 || got[0] != "PAYMENT_NODE_MERCHANT_SECRET is empty" {
		t.Fatalf("missing Node secret issues = %v", got)
	}
}
