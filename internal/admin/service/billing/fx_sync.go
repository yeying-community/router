package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yeying-community/router/internal/admin/model"
)

const (
	fxProviderFrankfurter       = "frankfurter"
	fxFrankfurterLatestEndpoint = "https://api.frankfurter.app/latest"
	fxSyncTimeout               = 15 * time.Second
)

type FXSyncUpdatedRate struct {
	Pair  string  `json:"pair"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

type FXSyncSkippedRate struct {
	Pair   string `json:"pair"`
	Base   string `json:"base"`
	Quote  string `json:"quote"`
	Reason string `json:"reason"`
}

type FXSyncResult struct {
	Provider      string              `json:"provider"`
	Base          string              `json:"base"`
	Date          string              `json:"date"`
	CurrencyCount int                 `json:"currency_count"`
	UpdatedCount  int                 `json:"updated_count"`
	SkippedCount  int                 `json:"skipped_count"`
	Updated       []FXSyncUpdatedRate `json:"updated"`
	Skipped       []FXSyncSkippedRate `json:"skipped"`
}

type frankfurterLatestResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`
}

func SyncFXMarketRates(ctx context.Context) (FXSyncResult, error) {
	result := FXSyncResult{
		Provider: fxProviderFrankfurter,
		Base:     model.BillingCurrencyCodeUSD,
		Updated:  make([]FXSyncUpdatedRate, 0),
		Skipped:  make([]FXSyncSkippedRate, 0),
	}
	if model.DB == nil {
		return result, fmt.Errorf("database handle is nil")
	}

	codes, err := listConfiguredFXCurrencies()
	if err != nil {
		return result, err
	}
	result.CurrencyCount = len(codes)
	if len(codes) < 2 {
		return result, nil
	}

	targetCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		if code == model.BillingCurrencyCodeUSD {
			continue
		}
		targetCodes = append(targetCodes, code)
	}

	payload := frankfurterLatestResponse{
		Base:  model.BillingCurrencyCodeUSD,
		Rates: make(map[string]float64),
	}
	if len(targetCodes) > 0 {
		payload, err = fetchFrankfurterLatestRates(ctx, model.BillingCurrencyCodeUSD, targetCodes)
		if err != nil {
			return result, err
		}
	}
	result.Date = payload.Date

	usdRates := map[string]float64{
		model.BillingCurrencyCodeUSD: 1,
	}
	for code, rate := range payload.Rates {
		if rate > 0 {
			usdRates[code] = rate
		}
	}

	items := make([]model.FXMarketRate, 0, len(codes)*(len(codes)-1))
	appendSkipped := func(base string, quote string, reason string) {
		result.Skipped = append(result.Skipped, FXSyncSkippedRate{
			Pair:   fmt.Sprintf("%s/%s", base, quote),
			Base:   base,
			Quote:  quote,
			Reason: reason,
		})
	}

	for _, base := range codes {
		baseUSDRate, ok := usdRates[base]
		if !ok || baseUSDRate <= 0 {
			for _, quote := range codes {
				if base == quote {
					continue
				}
				appendSkipped(base, quote, "base_rate_not_found")
			}
			continue
		}
		for _, quote := range codes {
			if base == quote {
				continue
			}
			quoteUSDRate, ok := usdRates[quote]
			if !ok || quoteUSDRate <= 0 {
				appendSkipped(base, quote, "quote_rate_not_found")
				continue
			}
			rate := quoteUSDRate / baseUSDRate
			if rate <= 0 {
				appendSkipped(base, quote, "invalid_rate_value")
				continue
			}
			items = append(items, model.FXMarketRate{
				Base:  base,
				Quote: quote,
				Rate:  rate,
			})
			result.Updated = append(result.Updated, FXSyncUpdatedRate{
				Pair:  fmt.Sprintf("%s/%s", base, quote),
				Base:  base,
				Quote: quote,
				Rate:  rate,
			})
		}
	}

	if err := model.UpsertFXMarketRatesWithDB(model.DB, result.Provider, result.Date, items); err != nil {
		return result, err
	}

	result.UpdatedCount = len(result.Updated)
	result.SkippedCount = len(result.Skipped)
	return result, nil
}

func fetchFrankfurterLatestRates(ctx context.Context, base string, targetCodes []string) (frankfurterLatestResponse, error) {
	normalizedBase := strings.ToUpper(strings.TrimSpace(base))
	if normalizedBase == "" {
		normalizedBase = model.BillingCurrencyCodeUSD
	}

	normalizedTargets := make([]string, 0, len(targetCodes))
	seen := make(map[string]struct{}, len(targetCodes))
	for _, rawCode := range targetCodes {
		code := strings.ToUpper(strings.TrimSpace(rawCode))
		if code == "" || code == normalizedBase {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		normalizedTargets = append(normalizedTargets, code)
	}
	sort.Strings(normalizedTargets)
	if len(normalizedTargets) == 0 {
		return frankfurterLatestResponse{}, fmt.Errorf("no target currency for FX sync")
	}

	query := url.Values{}
	query.Set("from", normalizedBase)
	query.Set("to", strings.Join(normalizedTargets, ","))
	requestURL := fxFrankfurterLatestEndpoint + "?" + query.Encode()

	if ctx == nil {
		ctx = context.Background()
	}
	requestCtx, cancel := context.WithTimeout(ctx, fxSyncTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return frankfurterLatestResponse{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return frankfurterLatestResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return frankfurterLatestResponse{}, fmt.Errorf("fx upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	payload := frankfurterLatestResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return frankfurterLatestResponse{}, err
	}
	if len(payload.Rates) == 0 {
		return frankfurterLatestResponse{}, fmt.Errorf("fx upstream returned empty rates")
	}
	normalizedRates := make(map[string]float64, len(payload.Rates))
	for code, rate := range payload.Rates {
		normalizedRates[strings.ToUpper(strings.TrimSpace(code))] = rate
	}
	payload.Rates = normalizedRates
	payload.Base = strings.ToUpper(strings.TrimSpace(payload.Base))
	return payload, nil
}
