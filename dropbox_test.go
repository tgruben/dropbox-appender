package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownload_ExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong auth header")
		}
		arg := r.Header.Get("Dropbox-API-Arg")
		if arg == "" {
			t.Errorf("missing Dropbox-API-Arg header")
		}
		w.WriteHeader(200)
		w.Write([]byte("existing content"))
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	content, err := client.Download("/Journal/2025/01/Note20250115.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "existing content" {
		t.Errorf("expected 'existing content', got %q", content)
	}
}

func TestDownload_FileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		w.Write([]byte(`{"error_summary": "path/not_found/"}`))
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	content, err := client.Download("/Journal/2025/01/Note20250115.md")
	if err != nil {
		t.Fatalf("unexpected error for not-found: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestDownload_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error_summary": "internal_error"}`))
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	_, err := client.Download("/some/path.md")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
