package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestUpload_Success(t *testing.T) {
	var receivedBody string
	var receivedArg string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong auth header")
		}
		if r.Header.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("wrong content type: %s", r.Header.Get("Content-Type"))
		}
		receivedArg = r.Header.Get("Dropbox-API-Arg")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	err := client.Upload("/Journal/2025/01/Note20250115.md", "file content here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody != "file content here" {
		t.Errorf("expected 'file content here', got %q", receivedBody)
	}

	// Verify the Dropbox-API-Arg contains overwrite mode and mute
	var argMap map[string]interface{}
	json.Unmarshal([]byte(receivedArg), &argMap)
	mode, ok := argMap["mode"].(string)
	if !ok || mode != "overwrite" {
		t.Errorf("expected mode 'overwrite', got %v", argMap["mode"])
	}
	mute, ok := argMap["mute"].(bool)
	if !ok || !mute {
		t.Errorf("expected mute true, got %v", argMap["mute"])
	}
}

func TestUpload_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error_summary": "internal"}`))
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	err := client.Upload("/some/path.md", "content")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestIntegration_AppendToExistingFile(t *testing.T) {
	existingContent := "### 10:00:00\nmorning note\n"
	var uploadedContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "download"):
			w.WriteHeader(200)
			w.Write([]byte(existingContent))
		case strings.Contains(r.URL.Path, "upload"):
			body, _ := io.ReadAll(r.Body)
			uploadedContent = string(body)
			w.WriteHeader(200)
			w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	now := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	path := resolvePath(now)

	existing, err := client.Download(path)
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	entry := formatEntry(now, "afternoon note", false)
	newContent := appendContent(existing, entry)

	err = client.Upload(path, newContent)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	expected := "### 10:00:00\nmorning note\n\n### 14:30:45\nafternoon note\n"
	if uploadedContent != expected {
		t.Errorf("expected %q, got %q", expected, uploadedContent)
	}
}

func TestIntegration_NewFile(t *testing.T) {
	var uploadedContent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "download"):
			w.WriteHeader(409)
			w.Write([]byte(`{"error_summary": "path/not_found/"}`))
		case strings.Contains(r.URL.Path, "upload"):
			body, _ := io.ReadAll(r.Body)
			uploadedContent = string(body)
			w.WriteHeader(200)
			w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	client := &DropboxClient{Token: "test-token", BaseURL: server.URL}
	now := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	path := resolvePath(now)

	existing, err := client.Download(path)
	if err != nil {
		t.Fatalf("download: %v", err)
	}

	entry := formatEntry(now, "first note of the day", false)
	newContent := appendContent(existing, entry)

	err = client.Upload(path, newContent)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	expected := "### 09:00:00\nfirst note of the day\n"
	if uploadedContent != expected {
		t.Errorf("expected %q, got %q", expected, uploadedContent)
	}
}
