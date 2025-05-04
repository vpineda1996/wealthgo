package client

import "encoding/json"

// WSAPISession represents a session with the Wealthsimple API
type WSAPISession struct {
	AccessToken  string
	RefreshToken string
	WSSDI        string
	SessionID    string
	ClientID     string
	TokenInfo    *TokenInformation
}

type TokenInformation struct {
	IdentityCanonicalId string `json:"identity_canonical_id"`
	ApplicationUid      string `json:"application_uid"`
}

// ToJSON converts the session to a JSON string
func (s *WSAPISession) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
