package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAuthorizeURL(t *testing.T) {
	u := authorizeURL("myappkey")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}
	if parsed.Host != "www.dropbox.com" {
		t.Errorf("expected www.dropbox.com, got %s", parsed.Host)
	}
	q := parsed.Query()
	if q.Get("client_id") != "myappkey" {
		t.Errorf("expected client_id=myappkey, got %s", q.Get("client_id"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("expected response_type=code, got %s", q.Get("response_type"))
	}
	if q.Get("token_access_type") != "offline" {
		t.Errorf("expected token_access_type=offline, got %s", q.Get("token_access_type"))
	}
}

func TestExchangeCode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("expected grant_type=authorization_code, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "test_code" {
			t.Errorf("expected code=test_code, got %s", r.FormValue("code"))
		}
		if r.FormValue("client_id") != "app_key" {
			t.Errorf("expected client_id=app_key, got %s", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "app_secret" {
			t.Errorf("expected client_secret=app_secret, got %s", r.FormValue("client_secret"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new_access","refresh_token":"new_refresh","token_type":"bearer"}`))
	}))
	defer server.Close()

	result, err := exchangeCode(server.URL, "app_key", "app_secret", "test_code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RefreshToken != "new_refresh" {
		t.Errorf("expected refresh_token=new_refresh, got %s", result.RefreshToken)
	}
	if result.AccessToken != "new_access" {
		t.Errorf("expected access_token=new_access, got %s", result.AccessToken)
	}
}

func TestExchangeCode_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error_description":"invalid code"}`))
	}))
	defer server.Close()

	_, err := exchangeCode(server.URL, "key", "secret", "bad_code")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("expected grant_type=refresh_token, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "my_refresh" {
			t.Errorf("expected refresh_token=my_refresh, got %s", r.FormValue("refresh_token"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"fresh_token","token_type":"bearer"}`))
	}))
	defer server.Close()

	token, err := refreshAccessToken(server.URL, "key", "secret", "my_refresh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "fresh_token" {
		t.Errorf("expected fresh_token, got %s", token)
	}
}

func TestRefreshAccessToken_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error_description":"invalid refresh token"}`))
	}))
	defer server.Close()

	_, err := refreshAccessToken(server.URL, "key", "secret", "bad_refresh")
	if err == nil {
		t.Fatal("expected error")
	}
}
