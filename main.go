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
