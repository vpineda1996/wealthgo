package client

import (
	"encoding/json"
	"errors"
	"fmt"
)

// GetTokenInfo retrieves token information
func (api *WealthsimpleAPIBase) GetTokenInfo() (*TokenInformation, error) {
	if api.Session.TokenInfo == nil {
		headers := map[string]any{
			"x-wealthsimple-client": "@wealthsimple/wealthsimple",
		}
		response, err := api.SendGet(fmt.Sprintf("%s/token/info", api.OAuthBaseURL), headers, false)
		if err != nil {
			return nil, err
		}

		b, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}

		var tokenInfo TokenInformation
		if err := json.Unmarshal(b, &tokenInfo); err != nil {
			return nil, err
		}

		api.Session.TokenInfo = &tokenInfo
	}
	return api.Session.TokenInfo, nil
}

// Login logs in to the Wealthsimple API
func Login(username, password, otpAnswer string, persistSessionFct func(string) error, scope string) (*WealthsimpleAPI, error) {
	api := newWealthsimpleAPI(nil)
	if scope == "" {
		scope = api.ScopeReadOnly
	}
	_, err := api.LoginInternal(username, password, otpAnswer, persistSessionFct, scope)
	if err != nil {
		return nil, err
	}
	return api, nil
}

// LoginInternal logs in to the Wealthsimple API
func (api *WealthsimpleAPIBase) LoginInternal(username, password, otpAnswer string, persistSessionFct func(string) error, scope string) (*WSAPISession, error) {
	data := map[string]interface{}{
		"grant_type":     "password",
		"username":       username,
		"password":       password,
		"skip_provision": "true",
		"scope":          scope,
		"client_id":      api.Session.ClientID,
		"otp_claim":      nil,
	}

	headers := map[string]interface{}{
		"x-wealthsimple-client": "@wealthsimple/wealthsimple",
		"x-ws-profile":          "undefined",
	}

	if otpAnswer != "" {
		headers["x-wealthsimple-otp"] = fmt.Sprintf("%s;remember=true", otpAnswer)
	}

	// Send the POST request for token
	response, err := api.SendPost(
		fmt.Sprintf("%s/token", api.OAuthBaseURL),
		data,
		headers,
		false,
	)
	if err != nil {
		return nil, err
	}

	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: unexpected response type", ErrUnexpected)
	}

	// Check if there was an error
	if errMsg, ok := responseMap["error"].(string); ok {
		if errMsg == "invalid_grant" && otpAnswer == "" {
			return nil, ErrOTPRequired
		}
		return nil, &WSAPIError{Err: ErrLoginFailed, Response: responseMap}
	}

	// Update the session with the tokens
	accessToken, ok := responseMap["access_token"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: access_token not found in response", ErrUnexpected)
	}

	refreshToken, ok := responseMap["refresh_token"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: refresh_token not found in response", ErrUnexpected)
	}

	api.Session.AccessToken = accessToken
	api.Session.RefreshToken = refreshToken

	// Persist the session if a persist function is provided
	if persistSessionFct != nil {
		sessionJSON, err := api.Session.ToJSON()
		if err != nil {
			return nil, err
		}
		if err := persistSessionFct(sessionJSON); err != nil {
			return nil, err
		}
	}

	return api.Session, nil
}

// FromToken creates a new WealthsimpleAPI instance from a session token
func FromToken(sess *WSAPISession, persistSessionFct func(string) error) (*WealthsimpleAPI, error) {
	api := newWealthsimpleAPI(sess)
	if err := api.CheckOAuthToken(persistSessionFct); err != nil {
		return nil, err
	}
	return api, nil
}

// CheckOAuthToken checks if the OAuth token is valid and refreshes it if needed
func (api *WealthsimpleAPIBase) CheckOAuthToken(persistSessionFct func(string) error) error {
	if api.Session.AccessToken != "" {
		// Try to use the token
		_, err := api.SearchSecurity("XEQT")
		if err == nil {
			return nil
		}

		// Check if the error is due to authorization
		var wsErr *WSAPIError
		if errors.As(err, &wsErr) {
			if msg, ok := wsErr.Response["message"].(string); !ok || msg != "Not Authorized." {
				return err
			}
			// Access token expired; try to refresh it below
		} else {
			return err
		}
	}

	if api.Session.RefreshToken != "" {
		data := map[string]interface{}{
			"grant_type":    "refresh_token",
			"refresh_token": api.Session.RefreshToken,
			"client_id":     api.Session.ClientID,
		}
		headers := map[string]interface{}{
			"x-wealthsimple-client": "@wealthsimple/wealthsimple",
			"x-ws-profile":          "invest",
		}
		response, err := api.SendPost(fmt.Sprintf("%s/token", api.OAuthBaseURL), data, headers, false)
		if err != nil {
			return err
		}

		responseMap, ok := response.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%w: unexpected response type", ErrUnexpected)
		}

		accessToken, ok := responseMap["access_token"].(string)
		if !ok {
			return fmt.Errorf("%w: access_token not found in response", ErrUnexpected)
		}

		refreshToken, ok := responseMap["refresh_token"].(string)
		if !ok {
			return fmt.Errorf("%w: refresh_token not found in response", ErrUnexpected)
		}

		api.Session.AccessToken = accessToken
		api.Session.RefreshToken = refreshToken

		// Persist the session if a persist function is provided
		if persistSessionFct != nil {
			sessionJSON, err := api.Session.ToJSON()
			if err != nil {
				return err
			}
			if err := persistSessionFct(sessionJSON); err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("%w: OAuth token invalid and cannot be refreshed", ErrManualLogin)
}
