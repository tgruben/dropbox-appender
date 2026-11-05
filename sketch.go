package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// defaultSketchFolder is the Dropbox folder where .excalidraw files are stored.
const defaultSketchFolder = "/Notes/attachments/Excalidraw"

// sketchAttachmentPath returns the Dropbox path for a sketch file.
func sketchAttachmentPath(folder, name string) string {
	if folder == "" {
		folder = defaultSketchFolder
	}
	return fmt.Sprintf("%s/%s.excalidraw", folder, name)
}

// sketchFileName builds the default filename (no extension) for a sketch from
// the current time: sketch-YYYYMMDD-HHMMSS.
func sketchFileName(now time.Time) string {
	return fmt.Sprintf("sketch-%s", now.Format("20060102-150405"))
}

// sketchMarkdownLink returns a plain markdown link from today's journal entry
// (at /Notes/Journal/YYYY/MM/NoteYYYYMMDD.md) to the sketch stored at
// /Notes/attachments/Excalidraw/<name>.excalidraw. The journal is three
// directories below /Notes, so the link uses ../../../ to reach /Notes.
func sketchMarkdownLink(name string) string {
	return fmt.Sprintf("[%s](../../../attachments/Excalidraw/%s.excalidraw)", name, name)
}

// sketchOptions holds the flags accepted by the sketch subcommand.
type sketchOptions struct {
	name   string
	folder string
}

// newSketchFlagSet defines the sketch subcommand's flags, binding them to opts.
// It is also used to render the sketch section of the top-level help.
func newSketchFlagSet(opts *sketchOptions) *flag.FlagSet {
	fs := flag.NewFlagSet("sketch", flag.ContinueOnError)
	fs.StringVar(&opts.name, "name", "", "filename (without extension) for the sketch; defaults to sketch-YYYYMMDD-HHMMSS")
	fs.StringVar(&opts.folder, "folder", defaultSketchFolder, "Dropbox folder for sketch attachments")
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), "Usage: dropbox-appender sketch [flags] < drawing.excalidraw\n\n"+
			"Upload an .excalidraw JSON document read from stdin and link it from\n"+
			"today's journal entry.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	return fs
}

// runSketch implements the `dropbox-appender sketch` subcommand: it reads an
// .excalidraw JSON document from stdin, uploads it to the attachments folder,
// and appends a markdown link to today's journal entry. It returns the process
// exit code.
func runSketch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var opts sketchOptions
	fs := newSketchFlagSet(&opts)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "error reading stdin: %v\n", err)
		return 1
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		fmt.Fprintln(stderr, "no sketch data provided on stdin")
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

	return runSketchWithClient(args, stdin, stdout, stderr,
		&DropboxClient{Token: token}, time.Now(), string(data), opts.name, opts.folder)
}

// runSketchWithClient is the testable core of the sketch subcommand. It uploads
// the provided sketch payload and appends a markdown link to the journal for the
// given time, using the provided client. name and folder may be empty to use
// defaults; if name is empty it is derived from now.
func runSketchWithClient(args []string, stdin io.Reader, stdout, stderr io.Writer,
	client *DropboxClient, now time.Time, payload, name, folder string) int {

	_ = args
	_ = stdin
	_ = stdout

	if name == "" {
		name = sketchFileName(now)
	}

	attPath := sketchAttachmentPath(folder, name)
	if err := client.Upload(attPath, payload); err != nil {
		fmt.Fprintf(stderr, "error uploading sketch: %v\n", err)
		return 1
	}

	journalPath := resolvePath(now)
	entry := formatEntry(now, sketchMarkdownLink(name), false)
	if err := appendToJournal(client, journalPath, entry); err != nil {
		fmt.Fprintf(stderr, "error updating journal: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "Saved %s and linked in %s\n", attPath, journalPath)
	return 0
}
