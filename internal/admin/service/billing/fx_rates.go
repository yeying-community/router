package billing

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yeying-community/router/internal/admin/model"
)

type FXMarketRateItem struct {
	Pair  string  `json:"pair"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

type FXMarketRatesResult struct {
	Provider   string             `json:"provider"`
	Date       string             `json:"date"`
	Base       string             `json:"base"`
	Currencies []string           `json:"currencies"`
	Items      []FXMarketRateItem `json:"items"`
}

func GetFXMarketRates(ctx context.Context, currencies []string) (FXMarketRatesResult, error) {
	result := FXMarketRatesResult{
		Provider:   fxProviderFrankfurter,
		Base:       model.BillingCurrencyCodeUSD,
		Currencies: make([]string, 0),
		Items:      make([]FXMarketRateItem, 0),
	}

	supportedCurrencies, err := listSupportedBillingCurrencies()
	if err != nil {
		return result, err
	}
	selectedCurrencies := append([]string{}, supportedCurrencies...)
	filterCurrencies := normalizeFXCurrencies(currencies)
	if len(filterCurrencies) > 0 {
		selectedCurrencies = intersectCurrencies(supportedCurrencies, filterCurrencies)
	}
	if len(selectedCurrencies) == 0 {
		result.Currencies = selectedCurrencies
		return result, nil
	}
	result.Currencies = append(result.Currencies, selectedCurrencies...)

	targetCodes := make([]string, 0, len(selectedCurrencies))
	for _, code := range selectedCurrencies {
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

	availableCurrencies := make([]string, 0, len(selectedCurrencies))
	for _, code := range selectedCurrencies {
		if _, ok := usdRates[code]; ok {
			availableCurrencies = append(availableCurrencies, code)
		}
	}
	result.Currencies = availableCurrencies
	if len(availableCurrencies) < 2 {
		return result, nil
	}

	items := make([]FXMarketRateItem, 0, len(availableCurrencies)*(len(availableCurrencies)-1))
	for _, base := range availableCurrencies {
		baseUSDRate := usdRates[base]
		if baseUSDRate <= 0 {
			continue
		}
		for _, quote := range availableCurrencies {
			if base == quote {
				continue
			}
			quoteUSDRate := usdRates[quote]
			if quoteUSDRate <= 0 {
				continue
			}
			rate := quoteUSDRate / baseUSDRate
			if rate <= 0 {
				continue
			}
			items = append(items, FXMarketRateItem{
				Pair:  fmt.Sprintf("%s/%s", base, quote),
				Base:  base,
				Quote: quote,
				Rate:  rate,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Base != items[j].Base {
			return items[i].Base < items[j].Base
		}
		return items[i].Quote < items[j].Quote
	})
	result.Items = items
	return result, nil
}

func normalizeFXCurrencies(currencies []string) []string {
	seen := make(map[string]struct{}, len(currencies))
	normalized := make([]string, 0, len(currencies))
	for _, raw := range currencies {
		for _, part := range strings.Split(raw, ",") {
			code := strings.ToUpper(strings.TrimSpace(part))
			if code == "" {
				continue
			}
			if len(code) < 3 || len(code) > 8 {
				continue
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			normalized = append(normalized, code)
		}
	}
	sort.Strings(normalized)
	return normalized
}

func containsCurrency(currencies []string, target string) bool {
	for _, code := range currencies {
		if code == target {
			return true
		}
	}
	return false
}

func listSupportedBillingCurrencies() ([]string, error) {
	rows, err := model.ListBillingCurrencies()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(rows))
	codes := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Status != model.BillingCurrencyStatusEnabled {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(row.Code))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return []string{model.BillingCurrencyCodeUSD}, nil
	}
	if !containsCurrency(codes, model.BillingCurrencyCodeUSD) {
		codes = append(codes, model.BillingCurrencyCodeUSD)
	}
	sort.Strings(codes)
	return codes, nil
}

func intersectCurrencies(base []string, filter []string) []string {
	filterSet := make(map[string]struct{}, len(filter))
	for _, code := range filter {
		filterSet[code] = struct{}{}
	}
	result := make([]string, 0, len(base))
	for _, code := range base {
		if _, ok := filterSet[code]; ok {
			result = append(result, code)
		}
	}
	return result
}
