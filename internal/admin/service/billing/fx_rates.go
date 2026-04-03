package billing

import (
	"context"
	"sort"
	"strings"

	"github.com/yeying-community/router/internal/admin/model"
)

type FXMarketRateItem struct {
	Pair      string  `json:"pair"`
	Base      string  `json:"base"`
	Quote     string  `json:"quote"`
	Rate      float64 `json:"rate"`
	Provider  string  `json:"provider"`
	RateDate  string  `json:"rate_date"`
	UpdatedAt int64   `json:"updated_at"`
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

	supportedCurrencies, err := listConfiguredFXCurrencies()
	if err != nil {
		return result, err
	}
	selectedCurrencies := append([]string{}, supportedCurrencies...)
	filterCurrencies := normalizeFXCurrencies(currencies)
	if len(filterCurrencies) > 0 {
		selectedCurrencies = intersectCurrencies(supportedCurrencies, filterCurrencies)
	}
	result.Currencies = append(result.Currencies, selectedCurrencies...)
	if len(selectedCurrencies) < 2 {
		return result, nil
	}

	rows, err := model.ListFXMarketRatesWithDB(model.DB, selectedCurrencies)
	if err != nil {
		return result, err
	}
	if len(rows) == 0 {
		if _, err := SyncFXMarketRates(ctx); err != nil {
			return result, err
		}
		rows, err = model.ListFXMarketRatesWithDB(model.DB, selectedCurrencies)
		if err != nil {
			return result, err
		}
	}

	items := make([]FXMarketRateItem, 0, len(rows))
	for _, row := range rows {
		base := strings.ToUpper(strings.TrimSpace(row.Base))
		quote := strings.ToUpper(strings.TrimSpace(row.Quote))
		if base == "" || quote == "" || base == quote || row.Rate <= 0 {
			continue
		}
		if result.Date == "" && strings.TrimSpace(row.RateDate) != "" {
			result.Date = strings.TrimSpace(row.RateDate)
		}
		if strings.TrimSpace(row.Provider) != "" {
			result.Provider = strings.TrimSpace(row.Provider)
		}
		items = append(items, FXMarketRateItem{
			Pair:      base + "/" + quote,
			Base:      base,
			Quote:     quote,
			Rate:      row.Rate,
			Provider:  strings.TrimSpace(row.Provider),
			RateDate:  strings.TrimSpace(row.RateDate),
			UpdatedAt: row.UpdatedAt,
		})
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

func listConfiguredFXCurrencies() ([]string, error) {
	rows, err := model.ListBillingCurrencies()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(rows))
	codes := make([]string, 0, len(rows))
	for _, row := range rows {
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
