package client

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/vpineda1996/wealthgo/client/graphql/generated"
)

type MarketData = map[string]any

type SecurityMarketDataCacheGetter func(string) (*generated.Security, bool)
type SecurityMarketDataCacheSetter func(string, *generated.Security)

// WealthsimpleAPIBase is the base struct for the Wealthsimple API
type WealthsimpleAPIBase struct {
	Session                       *WSAPISession
	SecurityMarketDataCacheGetter SecurityMarketDataCacheGetter
	SecurityMarketDataCacheSetter SecurityMarketDataCacheSetter
	UserAgent                     string

	// Constants
	OAuthBaseURL   string
	GraphQLURL     string
	GraphQLVersion string
	GraphQLQueries map[string]string
	ScopeReadOnly  string
	ScopeReadWrite string
}

// WealthsimpleAPI extends WealthsimpleAPIBase with additional functionality
type WealthsimpleAPI struct {
	WealthsimpleAPIBase
	AccountCache map[string][]generated.Account
}

//go:embed graphql/queries/*.graphql
var graphQlQueries embed.FS

// newWealthsimpleAPI creates a new WealthsimpleAPI instance
func newWealthsimpleAPI(sess *WSAPISession) *WealthsimpleAPI {
	api := &WealthsimpleAPI{
		WealthsimpleAPIBase: WealthsimpleAPIBase{
			OAuthBaseURL:   "https://api.production.wealthsimple.com/v1/oauth/v2",
			GraphQLURL:     "https://my.wealthsimple.com/graphql",
			GraphQLVersion: "12",
			GraphQLQueries: make(map[string]string),
			ScopeReadOnly:  "invest.read trade.read tax.read",
			ScopeReadWrite: "invest.read trade.read tax.read invest.write trade.write tax.write",
			Session:        &WSAPISession{},
		},
		AccountCache: make(map[string][]generated.Account),
	}

	// Read GraphQL query files into the api.GraphQLQueries map
	files, err := graphQlQueries.ReadDir("graphql/queries")
	if err != nil {
		panic(fmt.Errorf("failed to read embedded GraphQL files: %v", err))
	}

	for _, file := range files {
		if !file.IsDir() {
			content, err := graphQlQueries.ReadFile("graphql/queries/" + file.Name())
			if err != nil {
				panic(fmt.Errorf("failed to read file %s: %v", file.Name(), err))
			}
			queryName := strings.TrimSuffix(file.Name(), ".graphql")
			api.GraphQLQueries[queryName] = string(content)
		}
	}

	api.StartSession(sess)
	return api
}

// UUIDv4 generates a new UUID v4
func UUIDv4() string {
	return uuid.New().String()
}

// SetUserAgent sets the user agent for API requests
func (api *WealthsimpleAPI) SetUserAgent(userAgent string) {
	api.UserAgent = userAgent
}

// SendHTTPRequest sends an HTTP request to the specified URL
func (api *WealthsimpleAPIBase) SendHTTPRequest(url string, method string, data map[string]interface{}, headers map[string]interface{}, returnHeaders bool) (interface{}, error) {
	if headers == nil {
		headers = make(map[string]interface{})
	}

	if method == "POST" {
		headers["Content-Type"] = "application/json"
	}

	if api.Session.SessionID != "" {
		headers["x-ws-session-id"] = api.Session.SessionID
	}

	if api.Session.AccessToken != "" && (data == nil || data["grant_type"] != "refresh_token") {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", api.Session.AccessToken)
	}

	if api.Session.WSSDI != "" {
		headers["x-ws-device-id"] = api.Session.WSSDI
	}

	if api.UserAgent != "" {
		headers["User-Agent"] = api.UserAgent
	}

	var reqBody io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCurl, err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCurl, err)
	}

	// Add headers to request
	for k, v := range headers {
		req.Header.Set(k, fmt.Sprintf("%v", v))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCurl, err)
	}
	defer resp.Body.Close()

	if returnHeaders {
		// Combine headers and body as a single string
		var headerStr strings.Builder
		for k, v := range resp.Header {
			headerStr.WriteString(fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, ", ")))
		}
		headerStr.WriteString("\r\n")

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCurl, err)
		}

		return headerStr.String() + string(bodyBytes), nil
	}

	fmt.Println("Response Status:", resp.Status)

	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCurl, err)
	}
	return result, nil
}

// SendGet sends a GET request
func (api *WealthsimpleAPIBase) SendGet(url string, headers map[string]interface{}, returnHeaders bool) (interface{}, error) {
	return api.SendHTTPRequest(url, http.MethodGet, nil, headers, returnHeaders)
}

// SendPost sends a POST request
func (api *WealthsimpleAPIBase) SendPost(url string, data map[string]interface{}, headers map[string]interface{}, returnHeaders bool) (interface{}, error) {
	return api.SendHTTPRequest(url, http.MethodPost, data, headers, returnHeaders)
}

// StartSession initializes a session
func (api *WealthsimpleAPIBase) StartSession(sess *WSAPISession) error {
	if sess != nil {
		api.Session.AccessToken = sess.AccessToken
		api.Session.WSSDI = sess.WSSDI
		api.Session.SessionID = sess.SessionID
		api.Session.ClientID = sess.ClientID
		api.Session.RefreshToken = sess.RefreshToken
		return nil
	}

	var appJSURL string

	if api.Session.WSSDI == "" || api.Session.ClientID == "" {
		// Fetch login page
		response, err := api.SendGet("https://my.wealthsimple.com/app/login", nil, true)
		if err != nil {
			return err
		}

		responseStr, ok := response.(string)
		if !ok {
			return fmt.Errorf("%w: unexpected response type", ErrUnexpected)
		}

		// Look for wssdi in set-cookie headers
		if api.Session.WSSDI == "" {
			re := regexp.MustCompile(`(?i)wssdi=([a-f0-9]+);`)
			matches := re.FindStringSubmatch(responseStr)
			if len(matches) > 1 {
				api.Session.WSSDI = matches[1]
			}
		}

		// Look for app JS URL
		if appJSURL == "" {
			re := regexp.MustCompile(`(?i)<script.*src="(.+/app-[a-f0-9]+\.js)`)
			matches := re.FindStringSubmatch(responseStr)
			if len(matches) > 1 {
				appJSURL = matches[1]
			}
		}

		if api.Session.WSSDI == "" {
			return fmt.Errorf("%w: couldn't find wssdi in login page response headers", ErrUnexpected)
		}
	}

	if api.Session.ClientID == "" {
		if appJSURL == "" {
			return fmt.Errorf("%w: couldn't find app JS URL in login page response body", ErrUnexpected)
		}

		// Fetch the app JS file
		response, err := api.SendGet(appJSURL, nil, true)
		if err != nil {
			return err
		}

		responseStr, ok := response.(string)
		if !ok {
			return fmt.Errorf("%w: unexpected response type", ErrUnexpected)
		}

		// Look for clientId in the app JS file
		re := regexp.MustCompile(`(?i)production:.*clientId:"([a-f0-9]+)"`)
		matches := re.FindStringSubmatch(responseStr)
		if len(matches) > 1 {
			api.Session.ClientID = matches[1]
		}

		if api.Session.ClientID == "" {
			return fmt.Errorf("%w: couldn't find clientId in app JS", ErrUnexpected)
		}
	}

	if api.Session.SessionID == "" {
		api.Session.SessionID = UUIDv4()
	}

	return nil
}

var (
	objectType = reflect.TypeOf(map[string]interface{}{})
	arrayType  = reflect.TypeOf([]map[string]interface{}{})

	validate = validator.New(validator.WithRequiredStructEnabled())
)

type GraphQlQueryOpts struct {
	QueryName        string         `validate:"required,min=1"`
	Variables        map[string]any `validate:"required"`
	DataResponsePath string         `validate:"required,min=1"`
	ExpectType       reflect.Type   `validate:"required"`
}

func DoGraphQLQuery[ResponseType any](api *WealthsimpleAPIBase, opts GraphQlQueryOpts) (ResponseType, error) {
	// Validate the GraphQlQueryOpts struct
	if err := validate.Struct(opts); err != nil {
		return lo.Empty[ResponseType](), fmt.Errorf("validation error: %w", err)
	}

	queryName := opts.QueryName
	variables := opts.Variables
	dataResponsePath := opts.DataResponsePath
	expectType := opts.ExpectType

	query := map[string]any{
		"operationName": queryName,
		"query":         api.GraphQLQueries[queryName],
		"variables":     variables,
	}

	headers := map[string]any{
		"x-ws-profile":     "trade",
		"x-ws-api-version": api.GraphQLVersion,
		"x-ws-locale":      "en-CA",
		"x-platform-os":    "web",
	}

	response, err := api.SendPost(
		api.GraphQLURL,
		query,
		headers,
		false,
	)

	empty := lo.Empty[ResponseType]()

	if err != nil {
		return empty, err
	}

	responseMap, ok := response.(map[string]interface{})
	if !ok {
		return empty, fmt.Errorf("%w: unexpected response type", ErrUnexpected)
	}

	data, ok := responseMap["data"]
	if !ok {
		return empty, fmt.Errorf("no data present in request %s: %w", queryName,
			&WSAPIError{Err: ErrWSApi, Response: responseMap})
	}

	if data == nil {
		return empty, fmt.Errorf("data is nil on request %s: %w", queryName,
			&WSAPIError{Err: ErrWSApi, Response: responseMap})
	}

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return empty, fmt.Errorf("%w: unexpected data type", ErrUnexpected)
	}

	// Navigate through the response path
	pathParts := strings.Split(dataResponsePath, ".")
	var result any = dataMap
	for _, part := range pathParts {
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			return empty, fmt.Errorf("%w: unexpected data structure", ErrUnexpected)
		}
		result = resultMap[part]
		if result == nil {
			return empty, fmt.Errorf("%w: path %s not found in response", ErrUnexpected, part)
		}

		if part == "edges" {
			edges, ok := result.([]any)
			if !ok {
				return empty, fmt.Errorf("%w: unexpected data structure", ErrUnexpected)
			}
			nds := make([]any, len(edges))
			for i, edge := range edges {
				edgeMap, ok := edge.(map[string]any)
				if !ok {
					return empty, fmt.Errorf("%w: unexpected data structure", ErrUnexpected)
				}
				nd, ok := edgeMap["node"]
				if !ok {
					return empty, fmt.Errorf("%w: unexpected data structure", ErrUnexpected)
				}
				nds[i] = nd
			}
			result = nds
		}
	}

	if expectType != nil {
		resultValue := reflect.ValueOf(result)
		if resultValue.Kind() != expectType.Kind() {
			return empty, fmt.Errorf("%w: expected type %s, got %s", ErrUnexpected, expectType, resultValue.Kind())
		}
	}

	// HACK!
	b, err := json.Marshal(result)
	if err != nil {
		return empty, fmt.Errorf("%v, %w", ErrUnexpected, err)
	}

	var marshalledRes ResponseType
	err = json.Unmarshal(b, &marshalledRes)
	if err != nil {
		return empty, fmt.Errorf("%w: unexpected result format, %w", ErrUnexpected, err)
	}

	return marshalledRes, nil
}
