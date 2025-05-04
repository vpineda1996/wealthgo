package client

import (
	"errors"
	"fmt"
)

// Error types
var (
	ErrCurl          = errors.New("curl error")
	ErrLoginFailed   = errors.New("login failed")
	ErrManualLogin   = errors.New("manual login required")
	ErrOTPRequired   = errors.New("OTP required")
	ErrUnexpected    = errors.New("unexpected error")
	ErrWSApi         = errors.New("WS API error")
	ErrNotAuthorized = errors.New("not authorized")
)

// WSAPIError represents an error with additional response data
type WSAPIError struct {
	Err      error
	Response map[string]interface{}
}

func (e *WSAPIError) Error() string {
	if e.Response != nil {
		if msg, ok := e.Response["message"].(string); ok {
			return fmt.Sprintf("%v: %s", e.Err, msg)
		} else if msg, ok := e.Response["error_description"].(string); ok {
			return fmt.Sprintf("%v: %s", e.Err, msg)
		} else {
			return fmt.Sprintf("%v: %v", e.Err, e.Response)
		}
	}
	return e.Err.Error()
}
