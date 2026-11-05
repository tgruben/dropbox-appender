package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
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

// appendToJournal downloads an existing journal file (if any), appends the
// entry, and re-uploads it. Shared by the default text mode and the sketch
// subcommand.
func appendToJournal(client *DropboxClient, path, entry string) error {
	existing, err := client.Download(path)
	if err != nil {
		return fmt.Errorf("downloading journal: %w", err)
	}
	if err := client.Upload(path, appendContent(existing, entry)); err != nil {
		return fmt.Errorf("uploading journal: %w", err)
	}
	return nil
}

// buildVersion reports the module version stamped into the binary by the Go
// toolchain: the release tag for `go install ...@vX.Y.Z` or a `go build` from a
// tagged checkout, or a pseudo-version for untagged/dirty builds.
func buildVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}

// rootOptions holds the flags accepted by the default journal-append mode.
type rootOptions struct {
	noTimestamp bool
	version     bool
}

// newRootFlagSet defines the top-level flags, binding them to opts. Its Usage
// prints the full help, including every subcommand and its flags.
func newRootFlagSet(opts *rootOptions) *flag.FlagSet {
	fs := flag.NewFlagSet("dropbox-appender", flag.ExitOnError)
	fs.BoolVar(&opts.noTimestamp, "no-timestamp", false, "omit the ### HH:MM:SS header")
	fs.BoolVar(&opts.version, "version", false, "print the version and exit")
	fs.Usage = func() { printUsage(fs.Output(), fs) }
	return fs
}

// newAuthFlagSet defines the auth subcommand, which takes no flags; it exists
// so `auth -h` prints help instead of starting the authorization flow.
func newAuthFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: dropbox-appender auth\n\n"+
			"Authorize with Dropbox interactively and save the refresh token to\n"+
			"  "+defaultConfigPath()+"\n"+
			"Requires app_key and app_secret in that file or the DROPBOX_APP_KEY\n"+
			"and DROPBOX_APP_SECRET env vars.\n")
	}
	return fs
}

// printUsage writes the top-level help: default-mode usage and flags followed
// by each subcommand's usage and flags.
func printUsage(w io.Writer, root *flag.FlagSet) {
	fmt.Fprint(w, "Usage: dropbox-appender [flags] [text...]\n"+
		"       dropbox-appender <command> [flags]\n\n"+
		"Append a timestamped entry to today's Dropbox journal\n"+
		"(/Notes/Journal/YYYY/MM/NoteYYYYMMDD.md). Text comes from the arguments,\n"+
		"or from stdin when no arguments are given.\n\n"+
		"Flags:\n")
	root.PrintDefaults()
	fmt.Fprint(w, "\nCommands:\n"+
		"  auth     authorize with Dropbox and save a refresh token\n"+
		"  sketch   upload an .excalidraw sketch from stdin and link it in today's journal\n"+
		"  image    upload the clipboard image and link it in today's journal\n"+
		"  file     append stdin to a Dropbox file and link it in today's journal\n")
	for _, fs := range []*flag.FlagSet{newAuthFlagSet(), newSketchFlagSet(&sketchOptions{}), newImageFlagSet(&imageOptions{}), newFileFlagSet(&fileOptions{})} {
		fmt.Fprintln(w)
		fs.SetOutput(w)
		fs.Usage()
	}
	fmt.Fprint(w, "\nEnvironment:\n"+
		"  DROPBOX_TOKEN          access token used directly, bypassing refresh\n"+
		"  DROPBOX_REFRESH_TOKEN  refresh token (overrides the config file)\n"+
		"  DROPBOX_APP_KEY        app key (overrides the config file)\n"+
		"  DROPBOX_APP_SECRET     app secret (overrides the config file)\n")
}

func main() {
	// Check for subcommands before flag parsing.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			fs := newAuthFlagSet()
			if err := fs.Parse(os.Args[2:]); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					os.Exit(0)
				}
				os.Exit(2)
			}
			runAuth(defaultConfigPath())
			return
		case "sketch":
			os.Exit(runSketch(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
		case "image":
			os.Exit(runImage(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
		case "file":
			os.Exit(runFile(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
		}
	}

	var opts rootOptions
	fs := newRootFlagSet(&opts)
	fs.Parse(os.Args[1:]) // ExitOnError: exits on -h or bad flags.
	if opts.version {
		fmt.Println(buildVersion())
		return
	}

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

	input, err := readInput(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	now := time.Now()
	client := &DropboxClient{Token: token}
	path := resolvePath(now)
	entry := formatEntry(now, input, opts.noTimestamp)
	if err := appendToJournal(client, path, entry); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Appended to %s\n", path)
}
