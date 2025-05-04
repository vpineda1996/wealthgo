package client

import (
	"fmt"

	"github.com/vpineda1996/wealthgo/client/graphql/generated"
)

// SecuritySymbol represents a security symbol with an optional exchange prefix
// Example: "NYSE:APPL" or "APPL"
type SecuritySymbol string

// SecurityIDToSymbol converts a security ID to a symbol
func (api *WealthsimpleAPI) SecurityIDToSymbol(securityID string) (SecuritySymbol, error) {
	symbol := fmt.Sprintf("[%s]", securityID)

	if api.SecurityMarketDataCacheGetter != nil {
		marketData, err := api.GetSecurityMarketData(securityID, true)
		if err != nil {
			return "", err
		}
		if marketData != nil && marketData.Stock != nil {
			symbol = marketData.Stock.Symbol

			if marketData.Stock.PrimaryExchange != nil {
				symbol = fmt.Sprintf("%s:%s", *marketData.Stock.PrimaryExchange, symbol)
			}
		}
	}

	return SecuritySymbol(symbol), nil
}

// SetSecurityMarketDataCache sets the cache functions for security market data
func (api *WealthsimpleAPI) SetSecurityMarketDataCache(getter SecurityMarketDataCacheGetter, setter SecurityMarketDataCacheSetter) {
	api.SecurityMarketDataCacheGetter = getter
	api.SecurityMarketDataCacheSetter = setter
}

// GetSecurityMarketData retrieves security market data
func (api *WealthsimpleAPI) GetSecurityMarketData(securityID string, useCache bool) (*generated.Security, error) {
	if useCache && api.SecurityMarketDataCacheGetter != nil {
		cachedValue, ok := api.SecurityMarketDataCacheGetter(securityID)
		if ok && cachedValue != nil {
			return cachedValue, nil
		}
	}

	marketData, err := DoGraphQLQuery[generated.Security](
		&api.WealthsimpleAPIBase,
		GraphQlQueryOpts{
			QueryName:        "FetchSecurityMarketData",
			Variables:        map[string]any{"id": securityID},
			DataResponsePath: "security",
			ExpectType:       objectType,
		},
	)

	if err != nil {
		return nil, err
	}

	if useCache && api.SecurityMarketDataCacheSetter != nil {
		api.SecurityMarketDataCacheSetter(securityID, &marketData)
	}

	return &marketData, nil
}

// GetSecurityHistoricalQuotes retrieves historical quotes for a security
func (api *WealthsimpleAPI) GetSecurityHistoricalQuotes(securityID string, timeRange string) ([]generated.HistoricalQuote, error) {
	if timeRange == "" {
		timeRange = "1m"
	}

	result, err := DoGraphQLQuery[[]generated.HistoricalQuote](
		&api.WealthsimpleAPIBase,
		GraphQlQueryOpts{
			QueryName:        "FetchSecurityHistoricalQuotes",
			Variables:        map[string]any{"id": securityID, "timerange": timeRange},
			DataResponsePath: "security.historicalQuotes",
			ExpectType:       arrayType,
		})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SearchSecurity searches for a security by query
func (api *WealthsimpleAPIBase) SearchSecurity(query string) ([]generated.Security, error) {
	return DoGraphQLQuery[[]generated.Security](
		api, GraphQlQueryOpts{
			QueryName: "FetchSecuritySearchResult",
			Variables: map[string]any{
				"query": query,
			},
			DataResponsePath: "securitySearch.results",
			ExpectType:       arrayType,
		},
	)
}
