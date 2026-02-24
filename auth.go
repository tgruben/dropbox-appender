package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultTokenURL = "https://api.dropboxapi.com/oauth2/token"

// tokenResponse is the response from Dropbox's /oauth2/token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// authorizeURL returns the Dropbox OAuth2 authorization URL.
func authorizeURL(appKey string) string {
	params := url.Values{
		"client_id":         {appKey},
		"response_type":     {"code"},
		"token_access_type": {"offline"},
	}
	return "https://www.dropbox.com/oauth2/authorize?" + params.Encode()
}

// exchangeCode exchanges an authorization code for access + refresh tokens.
func exchangeCode(tokenURL, appKey, appSecret, code string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {appKey},
		"client_secret": {appSecret},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &result, nil
}

// refreshAccessToken uses a refresh token to get a fresh short-lived access token.
func refreshAccessToken(tokenURL, appKey, appSecret, refreshToken string) (string, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {appKey},
		"client_secret": {appSecret},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return "", fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	return result.AccessToken, nil
}
