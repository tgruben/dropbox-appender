package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// fileOptions holds the flags accepted by the file subcommand.
type fileOptions struct {
	noTimestamp bool
}

// newFileFlagSet defines the file subcommand's flags, binding them to opts.
// It is also used to render the file section of the top-level help.
func newFileFlagSet(opts *fileOptions) *flag.FlagSet {
	fs := flag.NewFlagSet("file", flag.ContinueOnError)
	fs.BoolVar(&opts.noTimestamp, "no-timestamp", false, "omit the ### HH:MM:SS header from the appended content and the journal link")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: dropbox-appender file [flags] <path> < content.md\n\n"+
			"Append stdin to the Dropbox file at <path> (created if missing; leading /\n"+
			"and .md suffix added if omitted) and link to it from today's journal entry.\n\n"+
			"Flags:\n")
		fs.PrintDefaults()
	}
	return fs
}

// runFile implements the `dropbox-appender file` subcommand: it appends the
// content read from stdin to the Dropbox file named by the single positional
// argument, then links that file from today's journal entry. It returns the
// process exit code.
func runFile(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var opts fileOptions
	fs := newFileFlagSet(&opts)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 || strings.TrimSpace(fs.Arg(0)) == "" {
		fmt.Fprintln(stderr, "file: expected exactly one Dropbox path argument")
		fs.Usage()
		return 2
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "error reading stdin: %v\n", err)
		return 1
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		fmt.Fprintln(stderr, "no content provided on stdin")
		return 1
	}

	cfg, err := loadConfig(defaultConfigPath())
	if err != nil {
		fmt.Fprintf(stderr, "error loading config: %v\n", err)
		return 1
	}
	token, err := resolveToken(cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	now := time.Now()
	target := normalizePath(fs.Arg(0))
	client := &DropboxClient{Token: token}
	if err := appendWithLink(client, now, target, formatEntry(now, content, opts.noTimestamp), opts.noTimestamp); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Appended to %s\n", target)
	return 0
}

// normalizePath ensures the Dropbox path is absolute and ends in .md.
func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, ".md") {
		path += ".md"
	}
	return path
}

// obsidianVaultRoot is the Dropbox folder holding the Obsidian vault
// (/Notes/.obsidian). Wiki links resolve relative to it.
const obsidianVaultRoot = "/Notes/"

// fileLink returns an Obsidian wiki link to a Dropbox file path: relative to
// the vault root and without the .md suffix, e.g. /Notes/a/b.md -> [[a/b]].
func fileLink(p string) string {
	return "[[" + strings.TrimSuffix(strings.TrimPrefix(p, obsidianVaultRoot), ".md") + "]]"
}

// appendWithLink appends entry to file at path (creating it), then appends
// a timestamped link entry to today's journal pointing at that file.
func appendWithLink(client *DropboxClient, now time.Time, path string, entry string, noTimestamp bool) error {
	if err := appendToJournal(client, path, entry); err != nil {
		return err
	}

	linkEntry := formatEntry(now, fileLink(path), noTimestamp)
	return appendToJournal(client, resolvePath(now), linkEntry)
}
