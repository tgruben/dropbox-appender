# Dropbox Daily Note Appender — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI that appends timestamped entries to a daily journal file in Dropbox.

**Architecture:** Single-binary CLI using `net/http` to call two Dropbox REST endpoints (download + upload). Input from CLI args or stdin. Auth via `DROPBOX_TOKEN` env var.

**Tech Stack:** Go stdlib only (`net/http`, `encoding/json`, `os`, `time`, `io`, `strings`, `fmt`)

---

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`

**Step 1: Initialize the module**

Run:
```bash
cd /data/dropbox_appender
go mod init github.com/dropbox-appender
```

**Step 2: Commit**

```bash
git add go.mod
git commit -m "init: go module"
```

---

### Task 2: Dropbox Client — Download

**Files:**
- Create: `dropbox.go`
- Create: `dropbox_test.go`

**Step 1: Write the failing test for Download**

Create `dropbox_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestDownload`
Expected: FAIL — `DropboxClient` not defined

**Step 3: Write the implementation**

Create `dropbox.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://content.dropboxapi.com"

// DropboxClient talks to the Dropbox content API.
type DropboxClient struct {
	Token   string
	BaseURL string // override for testing
}

func (c *DropboxClient) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}

// Download fetches a file from Dropbox. Returns empty string if file doesn't exist.
func (c *DropboxClient) Download(path string) (string, error) {
	arg, _ := json.Marshal(map[string]string{"path": path})

	req, err := http.NewRequest("POST", c.baseURL()+"/2/files/download", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Dropbox-API-Arg", string(arg))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == 409 {
		var apiErr struct {
			ErrorSummary string `json:"error_summary"`
		}
		json.Unmarshal(body, &apiErr)
		if strings.Contains(apiErr.ErrorSummary, "not_found") {
			return "", nil
		}
		return "", fmt.Errorf("dropbox API error: %s", apiErr.ErrorSummary)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestDownload`
Expected: PASS (all 3 tests)

**Step 5: Commit**

```bash
git add dropbox.go dropbox_test.go
git commit -m "feat: dropbox client download with tests"
```

---

### Task 3: Dropbox Client — Upload

**Files:**
- Modify: `dropbox.go`
- Modify: `dropbox_test.go`

**Step 1: Write the failing test for Upload**

Append to `dropbox_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestUpload`
Expected: FAIL — `Upload` method not defined

**Step 3: Write the implementation**

Append to `dropbox.go`:

```go
// Upload writes content to a file in Dropbox, overwriting if it exists.
func (c *DropboxClient) Upload(path string, content string) error {
	arg, _ := json.Marshal(map[string]interface{}{
		"path": path,
		"mode": "overwrite",
		"mute": true,
	})

	req, err := http.NewRequest("POST", c.baseURL()+"/2/files/upload", strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Dropbox-API-Arg", string(arg))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestUpload`
Expected: PASS (both tests)

**Step 5: Commit**

```bash
git add dropbox.go dropbox_test.go
git commit -m "feat: dropbox client upload with tests"
```

---

### Task 4: Main — Input Parsing & Entry Formatting

**Files:**
- Create: `main.go`

**Step 1: Write main.go with input parsing and entry formatting**

Create `main.go`:

```go
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// resolvePath returns the Dropbox path for today's journal file.
func resolvePath(now time.Time) string {
	return fmt.Sprintf("/Journal/%s/%s/Note%s.md",
		now.Format("2006"),
		now.Format("01"),
		now.Format("20060102"),
	)
}

// formatEntry prepends a timestamp header to the input text.
func formatEntry(now time.Time, text string) string {
	return fmt.Sprintf("### %s\n%s\n", now.Format("15:04:05"), text)
}

// readInput reads from CLI args first, then stdin.
func readInput(args []string) (string, error) {
	if len(args) > 1 {
		return strings.Join(args[1:], " "), nil
	}

	// Check if stdin has data (is not a terminal)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, nil
		}
	}

	return "", fmt.Errorf("no input provided: pass text as argument or via stdin")
}

// appendContent combines existing file content with the new entry.
func appendContent(existing string, entry string) string {
	if existing == "" {
		return entry
	}
	return existing + "\n" + entry
}

func main() {
	token := os.Getenv("DROPBOX_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "DROPBOX_TOKEN environment variable is required")
		os.Exit(1)
	}

	input, err := readInput(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	now := time.Now()
	path := resolvePath(now)
	entry := formatEntry(now, input)

	client := &DropboxClient{Token: token}

	existing, err := client.Download(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error downloading: %v\n", err)
		os.Exit(1)
	}

	newContent := appendContent(existing, entry)

	if err := client.Upload(path, newContent); err != nil {
		fmt.Fprintf(os.Stderr, "error uploading: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Appended to %s\n", path)
}
```

**Step 2: Verify it compiles**

Run: `go build -o /dev/null .`
Expected: no errors

**Step 3: Commit**

```bash
git add main.go
git commit -m "feat: main with input parsing, path resolution, orchestration"
```

---

### Task 5: Unit Tests for Main Helpers

**Files:**
- Create: `main_test.go`

**Step 1: Write tests for resolvePath, formatEntry, and appendContent**

Create `main_test.go`:

```go
package main

import (
	"testing"
	"time"
)

func TestResolvePath(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)
	path := resolvePath(now)
	expected := "/Journal/2025/01/Note20250115.md"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestResolvePath_December(t *testing.T) {
	now := time.Date(2025, 12, 3, 9, 0, 0, 0, time.UTC)
	path := resolvePath(now)
	expected := "/Journal/2025/12/Note20251203.md"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestFormatEntry(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	entry := formatEntry(now, "Had a great meeting")
	expected := "### 14:30:45\nHad a great meeting\n"
	if entry != expected {
		t.Errorf("expected %q, got %q", expected, entry)
	}
}

func TestAppendContent_Empty(t *testing.T) {
	result := appendContent("", "### 14:30:45\nnew entry\n")
	expected := "### 14:30:45\nnew entry\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAppendContent_Existing(t *testing.T) {
	existing := "### 10:00:00\nmorning note\n"
	entry := "### 14:30:45\nafternoon note\n"
	result := appendContent(existing, entry)
	expected := "### 10:00:00\nmorning note\n\n### 14:30:45\nafternoon note\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
```

**Step 2: Run tests**

Run: `go test -v ./...`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add main_test.go
git commit -m "test: unit tests for path resolution, entry formatting, content append"
```

---

### Task 6: Integration Test — Full Round Trip

**Files:**
- Modify: `dropbox_test.go`

**Step 1: Write integration test that mocks a full download→append→upload cycle**

Append to `dropbox_test.go`:

```go
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

	entry := formatEntry(now, "afternoon note")
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

	entry := formatEntry(now, "first note of the day")
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
```

Note: add `"strings"`, `"io"`, and `"time"` to the import block in `dropbox_test.go` if not already present.

**Step 2: Run all tests**

Run: `go test -v ./...`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add dropbox_test.go
git commit -m "test: integration tests for full download-append-upload cycle"
```

---

### Task 7: README

**Files:**
- Create: `README.md`

**Step 1: Write README**

Create `README.md`:

```markdown
# dropbox-appender

Append timestamped entries to a daily journal file in Dropbox.

Entries go to `/Journal/YYYY/MM/NoteYYYYMMDD.md` with a `### HH:MM:SS` header.

## Setup

1. Go to [Dropbox App Console](https://www.dropbox.com/developers/apps)
2. Create an app with **Full Dropbox** access
3. Generate an access token
4. Export it:

```bash
export DROPBOX_TOKEN="your-token-here"
```

## Usage

```bash
# Pass text as argument
dropbox-appender "Had a great meeting"

# Pipe from stdin
echo "Some note" | dropbox-appender
```

## Build

```bash
go build -o dropbox-appender .
```

## Example Output

After two entries, `/Journal/2025/01/Note20250115.md` contains:

```markdown
### 10:00:00
morning standup went well

### 14:30:45
Had a great meeting
```
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README with setup and usage"
```

---

### Task 8: Final Verification

**Step 1: Run all tests**

Run: `go test -v ./...`
Expected: ALL PASS

**Step 2: Build the binary**

Run: `go build -o dropbox-appender .`
Expected: binary created, no errors

**Step 3: Smoke test (no token)**

Run: `unset DROPBOX_TOKEN && ./dropbox-appender "test"`
Expected: stderr output `DROPBOX_TOKEN environment variable is required`, exit code 1

**Step 4: Final commit if any cleanup needed**

```bash
git log --oneline
```

Expected commit history:
```
docs: README with setup and usage
test: integration tests for full download-append-upload cycle
test: unit tests for path resolution, entry formatting, content append
feat: main with input parsing, path resolution, orchestration
feat: dropbox client upload with tests
feat: dropbox client download with tests
init: go module
Add design doc for dropbox daily note appender
```
