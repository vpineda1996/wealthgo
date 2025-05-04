package client

import (
	"fmt"

	"github.com/samber/lo"
	"github.com/vpineda1996/wealthgo/client/graphql/generated"
)

// GetAccounts retrieves accounts
func (api *WealthsimpleAPI) GetAccounts(openOnly bool, useCache bool) ([]generated.Account, error) {
	cacheKey := "all"
	if openOnly {
		cacheKey = "open"
	}

	if !useCache || api.AccountCache[cacheKey] == nil {
		tokenInfo, err := api.GetTokenInfo()
		if err != nil {
			return nil, err
		}

		identityID := tokenInfo.IdentityCanonicalId
		accounts, err := DoGraphQLQuery[[]generated.Account](
			&api.WealthsimpleAPIBase,
			GraphQlQueryOpts{
				QueryName: "FetchAllAccountFinancials",
				Variables: map[string]any{
					"pageSize":   25,
					"identityId": identityID,
				},
				DataResponsePath: "identity.accounts.edges",
				ExpectType:       arrayType,
			},
		)
		if err != nil {
			return nil, err
		}

		accounts = lo.Filter(accounts, func(acc generated.Account, _ int) bool {
			if openOnly {
				return acc.Status == "open"
			} else {
				return true
			}
		})

		api.AccountCache[cacheKey] = accounts
	}

	return api.AccountCache[cacheKey], nil
}

// GetAccountBalances retrieves account balances
func (api *WealthsimpleAPI) GetAccountBalances(accountID string) (map[SecuritySymbol]string, error) {

	accounts, err := DoGraphQLQuery[[]generated.Account](
		&api.WealthsimpleAPIBase,
		GraphQlQueryOpts{
			QueryName: "FetchAccountsWithBalance",
			Variables: map[string]interface{}{
				"type": "TRADING",
				"ids":  []string{accountID},
			},
			DataResponsePath: "accounts",
			ExpectType:       arrayType,
		})
	if err != nil {
		return nil, err
	}

	if len(accounts) != 1 {
		return nil, fmt.Errorf("%w: no account found, got %d", ErrUnexpected, len(accounts))
	}

	balances := make(map[SecuritySymbol]string)
	custodianAccounts := accounts[0].CustodianAccounts
	for _, ca := range custodianAccounts {
		financials := ca.Financials
		balance := financials.Balance
		for _, b := range balance {
			securityId := b.SecurityId
			quantity := b.Quantity

			if securityId != "sec-c-cad" && securityId != "sec-c-usd" {
				symbol, err := api.SecurityIDToSymbol(securityId)
				if err != nil {
					continue
				}
				securityId = string(symbol)
			}
			balances[SecuritySymbol(securityId)] = quantity
		}
	}

	return balances, nil
}
