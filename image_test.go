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

func TestImageExtForMIME(t *testing.T) {
	cases := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/jpg":  ".jpg",
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"image/bmp":  ".bmp",
		"image/xyz":  ".png",
		"":           ".png",
	}
	for mime, want := range cases {
		if got := imageExtForMIME(mime); got != want {
			t.Errorf("imageExtForMIME(%q) = %q, want %q", mime, got, want)
		}
	}
}

func TestImageAttachmentPath(t *testing.T) {
	got := imageAttachmentPath("", "foo", ".png")
	want := "/Notes/attachments/foo.png"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestImageAttachmentPath_CustomFolder(t *testing.T) {
	got := imageAttachmentPath("/Notes/other", "bar", ".jpg")
	want := "/Notes/other/bar.jpg"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestImageFileName(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	got := imageFileName(now)
	want := "image-20250115-143045"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestImageMarkdownLink(t *testing.T) {
	got := imageMarkdownLink("image-20250115-143045", ".png")
	want := "![image-20250115-143045](../../../attachments/image-20250115-143045.png)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// fakeClipboardReader returns a fixed byte slice, or an error, for use in tests.
type fakeClipboardReader struct {
	data []byte
	err  error
}

func (f fakeClipboardReader) ReadImage(mime string) ([]byte, error) {
	return f.data, f.err
}

func TestRunImageWithReader_EmptyClipboard(t *testing.T) {
	var stderr bytes.Buffer
	code := runImageWithReader(nil, strings.NewReader(""), io.Discard, &stderr,
		fakeClipboardReader{data: nil}, time.Now(), "", "", defaultImageMIME)
	if code != 1 {
		t.Errorf("expected exit code 1 for empty clipboard, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no image data") {
		t.Errorf("expected guidance in stderr, got %q", stderr.String())
	}
}

// TestRunImageWithClient_NewJournal verifies that, given a fake Dropbox server
// with no existing journal, the raw image bytes are uploaded to the
// attachments folder (without string corruption) and a timestamped markdown
// image link is appended to a fresh journal entry.
func TestRunImageWithClient_NewJournal(t *testing.T) {
	var uploadedImage []byte
	var uploadedImageArg string
	var uploadedJournal string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		arg := r.Header.Get("Dropbox-API-Arg")
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/download"):
			w.WriteHeader(409)
			w.Write([]byte(`{"error_summary": "path/not_found/"}`))
		case strings.HasSuffix(r.URL.Path, "/2/files/upload"):
			if strings.Contains(arg, "/Notes/attachments/") {
				uploadedImageArg = arg
				uploadedImage = body
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

	// Bytes that would be corrupted by string round-tripping (invalid UTF-8).
	imagePayload := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0xFF, 0xFE, 0x00, 0x01}
	var stderr bytes.Buffer
	code := runImageWithClient(&stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		imagePayload, "my-image", "", defaultImageMIME)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}

	if !bytes.Equal(uploadedImage, imagePayload) {
		t.Errorf("image upload body:\n got %v\nwant %v", uploadedImage, imagePayload)
	}
	if !strings.Contains(uploadedImageArg, "/Notes/attachments/my-image.png") {
		t.Errorf("image upload path arg: %q", uploadedImageArg)
	}

	expectedJournal := "### 14:30:45\n![my-image](../../../attachments/my-image.png)\n"
	if uploadedJournal != expectedJournal {
		t.Errorf("journal content:\n got %q\nwant %q", uploadedJournal, expectedJournal)
	}
}

// TestRunImageWithClient_ExistingJournal verifies the image link is appended
// after existing journal content.
func TestRunImageWithClient_ExistingJournal(t *testing.T) {
	var uploadedJournal string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		arg := r.Header.Get("Dropbox-API-Arg")
		switch {
		case strings.HasSuffix(r.URL.Path, "/2/files/download"):
			w.WriteHeader(200)
			w.Write([]byte("### 09:00:00\nmorning note\n"))
		case strings.HasSuffix(r.URL.Path, "/2/files/upload"):
			if !strings.Contains(arg, "/Notes/attachments/") {
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
	code := runImageWithClient(&stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		[]byte("fakepng"), "img2", "", defaultImageMIME)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}

	expected := "### 09:00:00\nmorning note\n\n### 14:30:45\n![img2](../../../attachments/img2.png)\n"
	if uploadedJournal != expected {
		t.Errorf("journal content:\n got %q\nwant %q", uploadedJournal, expected)
	}
}

// TestRunImageWithClient_DefaultName verifies the name is derived from time
// when none is provided.
func TestRunImageWithClient_DefaultName(t *testing.T) {
	var uploadedImageArg string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arg := r.Header.Get("Dropbox-API-Arg")
		if strings.HasSuffix(r.URL.Path, "/2/files/upload") && strings.Contains(arg, "/Notes/attachments/") {
			uploadedImageArg = arg
		}
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	var stderr bytes.Buffer
	code := runImageWithClient(&stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		[]byte("fakepng"), "", "", defaultImageMIME)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(uploadedImageArg, "image-20250115-143045.png") {
		t.Errorf("expected default-named upload, got arg: %q", uploadedImageArg)
	}
}

// TestRunImageWithClient_JpegType verifies the extension follows the -type flag.
func TestRunImageWithClient_JpegType(t *testing.T) {
	var uploadedImageArg string
	var uploadedJournal string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		arg := r.Header.Get("Dropbox-API-Arg")
		if strings.HasSuffix(r.URL.Path, "/2/files/upload") {
			if strings.Contains(arg, "/Notes/attachments/") {
				uploadedImageArg = arg
			} else {
				uploadedJournal = string(body)
			}
		}
		w.WriteHeader(200)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	var stderr bytes.Buffer
	code := runImageWithClient(&stderr,
		&DropboxClient{Token: "test-token", BaseURL: server.URL},
		time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		[]byte("fakejpeg"), "photo", "", "image/jpeg")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(uploadedImageArg, "/Notes/attachments/photo.jpg") {
		t.Errorf("expected .jpg upload, got arg: %q", uploadedImageArg)
	}
	if !strings.Contains(uploadedJournal, "![photo](../../../attachments/photo.jpg)") {
		t.Errorf("expected .jpg journal link, got: %q", uploadedJournal)
	}
}
