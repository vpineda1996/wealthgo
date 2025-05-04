package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/samber/lo"
	"github.com/vpineda1996/wealthgo/client"
)

// persistSession saves the session to a file
func persistSession(sessionJSON string) error {
	return os.WriteFile("session.json", []byte(sessionJSON), 0644)
}

// loadSession loads the session from a file
func loadSession() (*client.WSAPISession, error) {
	data, err := os.ReadFile("session.json")
	if err != nil {
		return nil, err
	}

	var session client.WSAPISession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// prettyPrint prints a JSON representation of the data
func prettyPrint(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return
	}
	fmt.Println(string(jsonData))
}

func main() {
	// Step 1: Try to load an existing session
	fmt.Println("Attempting to load existing session...")
	session, err := loadSession()
	var api *client.WealthsimpleAPI

	if err != nil {
		fmt.Println("No existing session found, logging in...")
		// Step 2: If no session exists, log in with credentials
		// Replace with actual credentials

		var username, password string
		var otpAnswer string

		fmt.Print("Enter username: ")
		fmt.Scanln(&username)

		fmt.Print("Enter password: ")
		fmt.Scanln(&password)

		fmt.Print("Enter OTP (if required): ")
		fmt.Scanln(&otpAnswer)

		api, err = client.Login(username, password, otpAnswer, persistSession, "")
		if err != nil {
			if err == client.ErrOTPRequired {
				fmt.Println("OTP is required. Please provide OTP and try again.")
				return
			}
			log.Fatalf("Login failed: %v", err)
		}
		fmt.Println("Login successful!")
	} else {
		fmt.Println("Session loaded successfully!")
		// Step 3: Create API instance from existing session
		api, err = client.FromToken(session, persistSession)
		if err != nil {
			log.Fatalf("Failed to create API from token: %v", err)
		}
	}

	// Step 4: Set a user agent (optional)
	api.SetUserAgent("WealthsimpleAPITest/1.0")

	// Step 5: Get token info
	fmt.Println("\n--- Token Info ---")
	tokenInfo, err := api.GetTokenInfo()
	if err != nil {
		log.Printf("Failed to get token info: %v", err)
	} else {
		prettyPrint(tokenInfo)
	}

	// Step 6: Search for a security
	fmt.Println("\n--- Security Search ---")
	securities, err := api.SearchSecurity("AAPL")
	if err != nil {
		log.Printf("Failed to search security: %v", err)
	} else {
		prettyPrint(securities)

		// If we found securities, get market data for the first one
		if len(securities) > 0 {
			securityID := securities[0].Id
			fmt.Println("\n--- Security Market Data ---")
			marketData := lo.Must(api.GetSecurityMarketData(securityID, false))
			prettyPrint(marketData)

			fmt.Println("\n--- Security Historical Quotes ---")
			quotes := lo.Must(api.GetSecurityHistoricalQuotes(securityID, "1m"))
			prettyPrint(quotes)

			fmt.Println("\n--- Security ID to Symbol ---")
			symbol := lo.Must(api.SecurityIDToSymbol(securityID))
			fmt.Printf("Symbol: %s\n", symbol)
		}
	}

	// Step 7: Get accounts
	fmt.Println("\n--- Accounts ---")
	accounts := lo.Must(api.GetAccounts(true, false))
	prettyPrint(accounts)

	for _, account := range accounts {
		accountID := account.Id

		fmt.Printf("Account ID: %s\n", accountID)
		fmt.Printf("Account Type: %s\n", *account.Type)
		fmt.Printf("Account Status: %s\n", account.Status)
		fmt.Printf("Account Currency: %s\n", *account.Currency)
		fmt.Printf("Account Balance: %s %s\n", account.Financials.CurrentCombined.NetLiquidationValueV2.Amount,
			account.Financials.CurrentCombined.NetLiquidationValueV2.Currency)

		fmt.Println("\n--- Account Activities ---")
		activities := lo.Must(api.GetActivities(accountID, 10, "", true))
		// If we found activities, get details for transfers
		for _, activity := range activities {
			fmt.Println("->\t", api.ActivityDescription(&activity))
		}
	}

	fmt.Println("\nAPI test completed!")
}
