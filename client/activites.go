package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/vpineda1996/wealthgo/client/graphql/generated"
)

// GetActivities retrieves account activities
func (api *WealthsimpleAPI) GetActivities(accountID string, howMany int, orderBy string, ignoreRejected bool) ([]generated.ActivityFeedItem, error) {
	if orderBy == "" {
		orderBy = "OCCURRED_AT_DESC"
	}
	if howMany <= 0 {
		howMany = 50
	}

	// Calculate end date
	endDate := time.Now().Add(time.Hour * 24).Format(time.RFC3339)
	activities, err := DoGraphQLQuery[[]generated.ActivityFeedItem](
		&api.WealthsimpleAPIBase,
		GraphQlQueryOpts{
			QueryName: "FetchActivityFeedItems",
			Variables: map[string]any{
				"orderBy": orderBy,
				"first":   howMany,
				"condition": map[string]any{
					"endDate":    endDate,
					"accountIds": []string{accountID},
				},
			},
			DataResponsePath: "activityFeedItems.edges",
			ExpectType:       arrayType,
		})
	if err != nil {
		return nil, err
	}
	filterFn := func(activity generated.ActivityFeedItem, _ int) bool {
		if !ignoreRejected {
			return true
		}
		status := activity.Status
		return activity.Type != "LEGACY_TRANSFER" || (status != "rejected" && status != "cancelled")
	}
	activities = lo.Filter(activities, filterFn)
	return activities, nil
}

// activityAddDescription adds a description to an activity
func (api *WealthsimpleAPI) ActivityDescription(activity *generated.ActivityFeedItem) string {

	// Default description
	description := fmt.Sprintf("%s: %s", activity.Type, activity.SubType)
	actType := activity.Type
	subType := activity.SubType

	// Set description based on activity type
	switch activity.Type {
	case "INTERNAL_TRANSFER":
		opposingAccountID := activity.OpposingAccountId
		if opposingAccountID != nil {
			if activity.SubType == "SOURCE" {
				description = fmt.Sprintf("Transfer out: Transfer to Wealthsimple %s", *opposingAccountID)
			} else {
				description = fmt.Sprintf("Transfer in: Transfer from Wealthsimple %s", *opposingAccountID)
			}
		}

	case "DIY_BUY", "DIY_SELL":
		action := "buy"
		if actType == "DIY_SELL" {
			action = "sell"
		}
		verb := strings.ReplaceAll(subType, "_", " ")
		if verb != "" {
			verb = strings.ToUpper(verb[:1]) + verb[1:]
		}

		securityID := activity.SecurityId
		assetQuantity := activity.AssetQuantity
		amount := activity.Amount

		if securityID != nil {
			symbol, err := api.SecurityIDToSymbol(*securityID)
			if err == nil {
				qty, _ := strconv.ParseFloat(assetQuantity, 64)
				amt, _ := strconv.ParseFloat(amount, 64)
				price := 0.0
				if qty > 0 {
					price = amt / qty
				}
				description = fmt.Sprintf("%s: %s %g x %s @ %g", verb, action, qty, symbol, price)
			}
		}

	case "DEPOSIT", "WITHDRAWAL":
		if subType == "E_TRANSFER" || subType == "E_TRANSFER_FUNDING" {
			direction := "from"
			if actType == "WITHDRAWAL" {
				direction = "to"
			}
			email, name := activity.ETransferEmail, activity.ETransferName
			description = fmt.Sprintf("%s: Interac e-transfer %s %s %s", actType, direction, *name, *email)
		} else if subType == "EFT" {
			direction := "from"
			if actType == "WITHDRAWAL" {
				direction = "to"
			}

			externalID := activity.ExternalCanonicalId
			description = fmt.Sprintf("%s: EFT %s %s", actType, direction, *externalID)
		}
	}

	return description
}
