package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSketchAttachmentPath(t *testing.T) {
	got := sketchAttachmentPath("", "foo")
	want := "/Notes/attachments/Excalidraw/foo.excalidraw"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSketchAttachmentPath_CustomFolder(t *testing.T) {
	got := sketchAttachmentPath("/Notes/other", "bar")
	want := "/Notes/other/bar.excalidraw"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSketchFileName(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	got := sketchFileName(now)
	want := "sketch-20250115-143045"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSketchMarkdownLink(t *testing.T) {
	got := sketchMarkdownLink("sketch-20250115-143045")
	want := "[sketch-20250115-143045](../../../attachments/Excalidraw/sketch-20250115-143045.excalidraw)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRunSketch_EmptyStdin(t *testing.T) {
	var stderr bytes.Buffer
	code := runSketch([]string{}, strings.NewReader(""), io.Discard, &stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 for empty stdin, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no sketch data") {
		t.Errorf("expected guidance in stderr, got %q", stderr.String())
	}
}

// TestRunSketchWithClient_NewJournal verifies that, given a fake Dropbox server
// with no existing journal, the sketch payload is uploaded to the attachments
// folder and a timestamped markdown link is appended to a fresh journal entry.
func TestRunSketchWithClient_NewJournal(t *testing.T) {
	var uploadedSketch string
	var uploadedSketchArg string
	var uploadedJournal string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		arg := r.Header.Get("Dropbox-API-Arg")
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/download"):
			w.WriteHeader(409)
			w.Write([]byte(`{"error_summary": "path/not_found/"}`))
		case strings.HasSuffix(r.URL.Path, "/2/files/upload"):
			if strings.Contains(arg, "Excalidraw") {
				uploadedSketchArg = arg
				uploadedSketch = string(body)
			} else {
				uploadedJournal = string(body)
			}
			w.WriteHeader(200)
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	sketchPayload := `{"type":"excalidraw","elements":[]}`
	var stderr bytes.Buffer
	code := runSketchWithClient(
		nil, strings.NewReader(""), io.Discard, &stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		sketchPayload, "my-sketch", "",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}

	if uploadedSketch != sketchPayload {
		t.Errorf("sketch upload body:\n got %q\nwant %q", uploadedSketch, sketchPayload)
	}
	if !strings.Contains(uploadedSketchArg, "/Notes/attachments/Excalidraw/my-sketch.excalidraw") {
		t.Errorf("sketch upload path arg: %q", uploadedSketchArg)
	}

	expectedJournal := "### 14:30:45\n[my-sketch](../../../attachments/Excalidraw/my-sketch.excalidraw)\n"
	if uploadedJournal != expectedJournal {
		t.Errorf("journal content:\n got %q\nwant %q", uploadedJournal, expectedJournal)
	}
}

// TestRunSketchWithClient_ExistingJournal verifies the link is appended after
// existing journal content.
func TestRunSketchWithClient_ExistingJournal(t *testing.T) {
	var uploadedJournal string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		arg := r.Header.Get("Dropbox-API-Arg")
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/download"):
			w.WriteHeader(200)
			w.Write([]byte("### 09:00:00\nmorning note\n"))
		case strings.HasSuffix(r.URL.Path, "/2/files/upload"):
			if !strings.Contains(arg, "Excalidraw") {
				uploadedJournal = string(body)
			}
			w.WriteHeader(200)
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var stderr bytes.Buffer
	code := runSketchWithClient(
		nil, strings.NewReader(""), io.Discard, &stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		`{"type":"excalidraw"}`, "s2", "",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}

	expected := "### 09:00:00\nmorning note\n\n### 14:30:45\n[s2](../../../attachments/Excalidraw/s2.excalidraw)\n"
	if uploadedJournal != expected {
		t.Errorf("journal content:\n got %q\nwant %q", uploadedJournal, expected)
	}
}

// TestRunSketchWithClient_DefaultName verifies the name is derived from time
// when none is provided.
func TestRunSketchWithClient_DefaultName(t *testing.T) {
	var uploadedSketchArg string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arg := r.Header.Get("Dropbox-API-Arg")
		if strings.HasSuffix(r.URL.Path, "/2/files/upload") && strings.Contains(arg, "Excalidraw") {
			uploadedSketchArg = arg
		}
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	var stderr bytes.Buffer
	code := runSketchWithClient(
		nil, strings.NewReader(""), io.Discard, &stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		`{"type":"excalidraw"}`, "", "",
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(uploadedSketchArg, "sketch-20250115-143045.excalidraw") {
		t.Errorf("expected default-named upload, got arg: %q", uploadedSketchArg)
	}
}
