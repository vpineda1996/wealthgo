# WealthGo

A Go client library for the Wealthsimple API.

## Overview

WealthGo provides a Go interface to interact with the Wealthsimple API, allowing you to access your Wealthsimple accounts, view balances, search for securities, get market data, and more.

Any merge requests to improve this library are welcomed!

## Features

- Authentication with Wealthsimple (including OTP support)
- Session management
- Account information and balances
- Security search and market data
- Historical quotes for securities
- Account activity tracking

## Installation

```bash
go get github.com/vpineda1996/wealthgo
```

## Usage

### Basic Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/vpineda1996/wealthgo/client"
)

func main() {
	// Login to Wealthsimple
	api, err := client.Login("your-username", "your-password", "", nil, "")
	if err != nil {
		if err == client.ErrOTPRequired {
			// Handle OTP requirement
			fmt.Println("OTP is required. Please provide OTP and try again.")
			return
		}
		log.Fatalf("Login failed: %v", err)
	}

	// Get accounts
	accounts, err := api.GetAccounts(true, false)
	if err != nil {
		log.Fatalf("Failed to get accounts: %v", err)
	}

	// Display account information
	for _, account := range accounts {
		fmt.Printf("Account ID: %s\n", account.Id)
		fmt.Printf("Account Type: %s\n", *account.Type)
		fmt.Printf("Account Status: %s\n", account.Status)
		fmt.Printf("Account Balance: %s %s\n", 
			account.Financials.CurrentCombined.NetLiquidationValueV2.Amount,
			account.Financials.CurrentCombined.NetLiquidationValueV2.Currency)
	}
}
```

### Session Management

You can persist and reuse sessions to avoid logging in each time:

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/vpineda1996/wealthgo/client"
)

// Save session to a file
func persistSession(sessionJSON string) error {
	return os.WriteFile("session.json", []byte(sessionJSON), 0644)
}

// Load session from a file
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

func main() {
	// Try to load an existing session
	session, err := loadSession()
	var api *client.WealthsimpleAPI

	if err != nil {
		// No session exists, log in with credentials
		api, err = client.Login("your-username", "your-password", "", persistSession, "")
		if err != nil {
			log.Fatalf("Login failed: %v", err)
		}
	} else {
		// Create API instance from existing session
		api, err = client.FromToken(session, persistSession)
		if err != nil {
			log.Fatalf("Failed to create API from token: %v", err)
		}
	}

	// Now use the API...
}
```

### Security Search and Market Data

```go
// Search for a security
securities, err := api.SearchSecurity("AAPL")
if err != nil {
	log.Printf("Failed to search security: %v", err)
} else {
	// Get market data for the first security
	if len(securities) > 0 {
		securityID := securities[0].Id
		marketData, err := api.GetSecurityMarketData(securityID, false)
		if err != nil {
			log.Printf("Failed to get market data: %v", err)
		}
		
		// Get historical quotes
		quotes, err := api.GetSecurityHistoricalQuotes(securityID, "1m")
		if err != nil {
			log.Printf("Failed to get historical quotes: %v", err)
		}
	}
}
```

### Account Activities

```go
// Get account activities
activities, err := api.GetActivities(accountID, 10, "", true)
if err != nil {
	log.Printf("Failed to get activities: %v", err)
} else {
	for _, activity := range activities {
		description := api.ActivityDescription(&activity)
		fmt.Println(description)
	}
}
```

## Complete Example

See the [example/main.go](example/main.go) file for a complete example of how to use the library.

## Development

### Continuous Integration

This project uses GitHub Actions for continuous integration and deployment:

- Every push to main/master branches triggers a build and test workflow
- Commits with "new release" in the message will automatically create a new GitHub release
- To create a new version release, include the version in your commit message like: "new release v1.2.3"

The CI pipeline:
1. Builds the library to ensure it compiles correctly
2. Runs tests to verify functionality
3. Creates a GitHub release when triggered by the appropriate commit message

## Disclaimer

This is an unofficial client library and is not affiliated with, maintained, authorized, endorsed, or sponsored by Wealthsimple.