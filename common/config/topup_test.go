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
