package main

import (
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
	noTimestamp := flag.Bool("no-timestamp", false, "omit the ### HH:MM:SS header")
	flag.Parse()

	token := os.Getenv("DROPBOX_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "DROPBOX_TOKEN environment variable is required")
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
