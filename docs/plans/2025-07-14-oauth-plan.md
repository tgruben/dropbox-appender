# OAuth Authentication Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add OAuth2 with refresh tokens so users don't need to manage access tokens manually.

**Architecture:** New `auth` subcommand for one-time setup. Config file stores app_key, app_secret, refresh_token. On every run, refresh token is exchanged for a short-lived access token. `DROPBOX_TOKEN` env var still works as direct bypass.

**Tech Stack:** Go stdlib only (`net/http`, `net/url`, `encoding/json`, `os`, `path/filepath`)

---

### Task 1: Config File — Read/Write

**Files:**
- Create: `config.go`
- Create: `config_test.go`

**Step 1: Write the failing tests**

Create `config_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"app_key":"key1","app_secret":"secret1","refresh_token":"refresh1"}`), 0600)

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "key1" || cfg.AppSecret != "secret1" || cfg.RefreshToken != "refresh1" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, err := loadConfig("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "" || cfg.AppSecret != "" || cfg.RefreshToken != "" {
		t.Errorf("expected empty config, got: %+v", cfg)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"app_key":"file_key","app_secret":"file_secret","refresh_token":"file_refresh"}`), 0600)

	t.Setenv("DROPBOX_APP_KEY", "env_key")
	t.Setenv("DROPBOX_APP_SECRET", "env_secret")
	t.Setenv("DROPBOX_REFRESH_TOKEN", "env_refresh")

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "env_key" || cfg.AppSecret != "env_secret" || cfg.RefreshToken != "env_refresh" {
		t.Errorf("env vars should override file, got: %+v", cfg)
	}
}

func TestLoadConfig_PartialEnvOverride(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, []byte(`{"app_key":"file_key","app_secret":"file_secret","refresh_token":"file_refresh"}`), 0600)

	t.Setenv("DROPBOX_APP_KEY", "env_key")

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppKey != "env_key" {
		t.Errorf("expected env_key, got %s", cfg.AppKey)
	}
	if cfg.AppSecret != "file_secret" {
		t.Errorf("expected file_secret, got %s", cfg.AppSecret)
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "subdir", "config.json")

	cfg := &Config{AppKey: "k", AppSecret: "s", RefreshToken: "r"}
	err := saveConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if loaded.AppKey != "k" || loaded.AppSecret != "s" || loaded.RefreshToken != "r" {
		t.Errorf("round-trip failed: %+v", loaded)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run TestLoadConfig -run TestSaveConfig`
Expected: FAIL — types not defined

**Step 3: Write the implementation**

Create `config.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds OAuth credentials.
type Config struct {
	AppKey       string `json:"app_key"`
	AppSecret    string `json:"app_secret"`
	RefreshToken string `json:"refresh_token"`
}

// defaultConfigPath returns ~/.config/dropbox-appender/config.json.
func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dropbox-appender", "config.json")
}

// loadConfig reads config from file, then applies env var overrides.
func loadConfig(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, cfg)
	}

	// Env vars override file values
	if v := os.Getenv("DROPBOX_APP_KEY"); v != "" {
		cfg.AppKey = v
	}
	if v := os.Getenv("DROPBOX_APP_SECRET"); v != "" {
		cfg.AppSecret = v
	}
	if v := os.Getenv("DROPBOX_REFRESH_TOKEN"); v != "" {
		cfg.RefreshToken = v
	}

	return cfg, nil
}

// saveConfig writes config to file, creating directories as needed.
func saveConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run 'TestLoadConfig|TestSaveConfig'`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: config file read/write with env var overrides"
```

---

### Task 2: Auth — Token Exchange & Refresh

**Files:**
- Create: `auth.go`
- Create: `auth_test.go`

**Step 1: Write the failing tests**

Create `auth_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test -v -run 'TestAuthorize|TestExchange|TestRefresh'`
Expected: FAIL — functions not defined

**Step 3: Write the implementation**

Create `auth.go`:

```go
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
```

**Step 4: Run tests to verify they pass**

Run: `go test -v -run 'TestAuthorize|TestExchange|TestRefresh'`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add auth.go auth_test.go
git commit -m "feat: OAuth token exchange and refresh"
```

---

### Task 3: Wire Up Main — Auth Subcommand & Token Resolution

**Files:**
- Modify: `main.go`

**Step 1: Rewrite main.go to add auth subcommand and token resolution**

Replace `main.go` with:

```go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// resolvePath returns the Dropbox path for today's journal file.
func resolvePath(now time.Time) string {
	return fmt.Sprintf("/Notes/Journal/%s/%s/Note%s.md",
		now.Format("2006"),
		now.Format("01"),
		now.Format("20060102"),
	)
}

// formatEntry formats the input text, optionally with a timestamp header.
func formatEntry(now time.Time, text string, noTimestamp bool) string {
	if noTimestamp {
		return text + "\n"
	}
	return fmt.Sprintf("### %s\n%s\n", now.Format("15:04:05"), text)
}

// readInput reads from remaining CLI args first, then stdin.
func readInput(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}

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

// resolveToken gets an access token using the priority chain:
// 1. DROPBOX_TOKEN env var (direct)
// 2. Refresh token + app key/secret (auto-refresh)
// 3. Error
func resolveToken(cfg *Config) (string, error) {
	if token := os.Getenv("DROPBOX_TOKEN"); token != "" {
		return token, nil
	}

	if cfg.RefreshToken != "" && cfg.AppKey != "" && cfg.AppSecret != "" {
		token, err := refreshAccessToken(defaultTokenURL, cfg.AppKey, cfg.AppSecret, cfg.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("refresh token invalid, run: dropbox-appender auth\n  (%w)", err)
		}
		return token, nil
	}

	return "", fmt.Errorf("no authentication configured, run: dropbox-appender auth")
}

func runAuth(configPath string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.AppKey == "" || cfg.AppSecret == "" {
		fmt.Fprintln(os.Stderr, "app_key and app_secret required.")
		fmt.Fprintf(os.Stderr, "Set in %s or via DROPBOX_APP_KEY and DROPBOX_APP_SECRET env vars.\n", configPath)
		os.Exit(1)
	}

	fmt.Println("1. Open this URL in your browser:")
	fmt.Println()
	fmt.Println("  ", authorizeURL(cfg.AppKey))
	fmt.Println()
	fmt.Print("2. Enter the authorization code: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	code := strings.TrimSpace(scanner.Text())
	if code == "" {
		fmt.Fprintln(os.Stderr, "no code entered")
		os.Exit(1)
	}

	result, err := exchangeCode(defaultTokenURL, cfg.AppKey, cfg.AppSecret, code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cfg.RefreshToken = result.RefreshToken
	if err := saveConfig(configPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nAuthentication successful! Refresh token saved.")
}

func main() {
	// Check for auth subcommand before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "auth" {
		runAuth(defaultConfigPath())
		return
	}

	noTimestamp := flag.Bool("no-timestamp", false, "omit the ### HH:MM:SS header")
	flag.Parse()

	configPath := defaultConfigPath()
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	token, err := resolveToken(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	input, err := readInput(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	now := time.Now()
	path := resolvePath(now)
	entry := formatEntry(now, input, *noTimestamp)

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

**Step 3: Run all tests**

Run: `go test -v ./...`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat: auth subcommand and token resolution chain"
```

---

### Task 4: Tests for Token Resolution

**Files:**
- Modify: `main_test.go`

**Step 1: Add resolveToken tests**

Append to `main_test.go`:

```go
func TestResolveToken_EnvVar(t *testing.T) {
	t.Setenv("DROPBOX_TOKEN", "direct_token")
	cfg := &Config{}
	token, err := resolveToken(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "direct_token" {
		t.Errorf("expected direct_token, got %s", token)
	}
}

func TestResolveToken_EnvVarTakesPriority(t *testing.T) {
	t.Setenv("DROPBOX_TOKEN", "direct_token")
	cfg := &Config{AppKey: "k", AppSecret: "s", RefreshToken: "r"}
	token, err := resolveToken(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "direct_token" {
		t.Errorf("expected direct_token, got %s", token)
	}
}

func TestResolveToken_NoAuth(t *testing.T) {
	t.Setenv("DROPBOX_TOKEN", "")
	cfg := &Config{}
	_, err := resolveToken(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "dropbox-appender auth") {
		t.Errorf("expected guidance to run auth, got: %v", err)
	}
}
```

Note: add `"strings"` to the import block in `main_test.go`.

**Step 2: Run tests**

Run: `go test -v -run TestResolveToken`
Expected: ALL PASS

**Step 3: Commit**

```bash
git add main_test.go
git commit -m "test: token resolution priority chain"
```

---

### Task 5: Update README

**Files:**
- Modify: `README.md`

**Step 1: Update README with auth instructions**

Replace `README.md` with:

```markdown
# dropbox-appender

A minimal Go CLI that appends timestamped entries to a daily journal file in Dropbox.

Entries go to `/Notes/Journal/YYYY/MM/NoteYYYYMMDD.md` with a `### HH:MM:SS` header.

## Install

```bash
go install github.com/tgruben/dropbox-appender@latest
```

## Setup

### 1. Create a Dropbox App

1. Go to [Dropbox App Console](https://www.dropbox.com/developers/apps)
2. Create an app with **Full Dropbox** access
3. Note your **App key** and **App secret**

### 2. Configure

Create `~/.config/dropbox-appender/config.json`:

```json
{
  "app_key": "your_app_key",
  "app_secret": "your_app_secret"
}
```

Or use environment variables:

```bash
export DROPBOX_APP_KEY="your_app_key"
export DROPBOX_APP_SECRET="your_app_secret"
```

### 3. Authenticate

```bash
dropbox-appender auth
```

This opens a Dropbox authorization URL. Approve access, paste the code, and you're done. The refresh token is saved automatically.

## Usage

```bash
# Pass text as argument
dropbox-appender "Had a great meeting"

# Pipe from stdin
echo "Some note" | dropbox-appender

# Without timestamp header
dropbox-appender -no-timestamp "Just the text"
```

## Authentication Priority

1. `DROPBOX_TOKEN` env var — used directly (legacy/manual tokens)
2. Refresh token (from config or `DROPBOX_REFRESH_TOKEN` env var) — auto-refreshes a short-lived access token
3. No auth — prompts to run `dropbox-appender auth`

## Example Output

After two entries, `/Notes/Journal/2025/01/Note20250115.md` contains:

```markdown
### 10:00:00
morning standup went well

### 14:30:45
Had a great meeting
```

## License

[MIT](LICENSE)
```

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with OAuth auth instructions"
```

---

### Task 6: Final Verification

**Step 1: Run all tests**

Run: `go test -v ./...`
Expected: ALL PASS

**Step 2: Build**

Run: `go build -o dropbox-appender .`
Expected: no errors

**Step 3: Smoke test auth help**

Run: `./dropbox-appender auth 2>&1` (with no config)
Expected: error message about app_key and app_secret required

**Step 4: Smoke test backward compat**

Run: `DROPBOX_TOKEN=fake ./dropbox-appender "test" 2>&1`
Expected: attempts to use "fake" as token (will fail at Dropbox API, but proves token resolution works)

**Step 5: Clean up and push**

```bash
rm dropbox-appender
git tag v0.2.0
git push && git push --tags
```
