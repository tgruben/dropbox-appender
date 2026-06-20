package main

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// defaultImageFolder is the Dropbox folder where pasted images are stored.
const defaultImageFolder = "/Notes/attachments"

// defaultImageMIME is the clipboard MIME type requested from wl-paste when no
// -type flag is given. Wayland screenshots are conventionally PNG.
const defaultImageMIME = "image/png"

// imageExtForMIME maps a clipboard image MIME type to the file extension used
// for the uploaded attachment. Unknown types fall back to .png.
func imageExtForMIME(mime string) string {
	switch mime {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".png"
	}
}

// imageAttachmentPath returns the Dropbox path for a pasted image file.
func imageAttachmentPath(folder, name, ext string) string {
	if folder == "" {
		folder = defaultImageFolder
	}
	return fmt.Sprintf("%s/%s%s", folder, name, ext)
}

// imageFileName builds the default filename (no extension) for a pasted image
// from the current time: image-YYYYMMDD-HHMMSS.
func imageFileName(now time.Time) string {
	return fmt.Sprintf("image-%s", now.Format("20060102-150405"))
}

// imageMarkdownLink returns a markdown image link from today's journal entry
// (at /Notes/Journal/YYYY/MM/NoteYYYYMMDD.md) to the image stored at
// /Notes/attachments/<name><ext>. The journal is three directories below
// /Notes, so the link uses ../../../ to reach /Notes.
func imageMarkdownLink(name, ext string) string {
	return fmt.Sprintf("![%s](../../../attachments/%s%s)", name, name, ext)
}

// wlPasteReader reads image bytes from the Wayland clipboard via wl-paste.
type wlPasteReader struct{}

// ReadImage runs `wl-paste -t <mime>` and returns the raw image bytes. An empty
// clipboard yields an empty slice with no error.
func (wlPasteReader) ReadImage(mime string) ([]byte, error) {
	cmd := exec.Command("wl-paste", "-t", mime)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running wl-paste -t %s: %w", mime, err)
	}
	return out, nil
}

// runImage implements the `dropbox-appender image` subcommand: it reads an
// image from the Wayland clipboard, uploads it to the attachments folder, and
// appends a markdown image link to today's journal entry. It returns the
// process exit code.
func runImage(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "filename (without extension) for the image; defaults to image-YYYYMMDD-HHMMSS")
	folder := fs.String("folder", defaultImageFolder, "Dropbox folder for image attachments")
	mime := fs.String("type", defaultImageMIME, "clipboard image MIME type to paste")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	return runImageWithReader(args, stdin, stdout, stderr, wlPasteReader{}, time.Now(), *name, *folder, *mime)
}

// runImageWithReader is the entry point that takes a clipboard reader, used by
// runImage so the wl-paste dependency can be injected. It loads config and
// resolves a token before delegating to runImageWithClient.
func runImageWithReader(args []string, stdin io.Reader, stdout, stderr io.Writer,
	reader clipboardImageReader, now time.Time, name, folder, mime string) int {

	_ = args
	_ = stdin
	_ = stdout

	data, err := reader.ReadImage(mime)
	if err != nil {
		fmt.Fprintf(stderr, "error reading clipboard: %v\n", err)
		return 1
	}
	if len(data) == 0 {
		fmt.Fprintln(stderr, "no image data in clipboard")
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

	return runImageWithClient(stderr,
		&DropboxClient{Token: token}, now, data, name, folder, mime)
}

// clipboardImageReader abstracts reading image bytes from the clipboard so the
// core logic is testable without wl-paste.
type clipboardImageReader interface {
	ReadImage(mime string) ([]byte, error)
}

// runImageWithClient is the testable core of the image subcommand. It uploads
// the provided image bytes and appends a markdown image link to the journal for
// the given time, using the provided client. name and folder may be empty to
// use defaults; if name is empty it is derived from now.
func runImageWithClient(stderr io.Writer, client *DropboxClient, now time.Time,
	data []byte, name, folder, mime string) int {

	if name == "" {
		name = imageFileName(now)
	}
	ext := imageExtForMIME(mime)

	attPath := imageAttachmentPath(folder, name, ext)
	if err := client.UploadBytes(attPath, data); err != nil {
		fmt.Fprintf(stderr, "error uploading image: %v\n", err)
		return 1
	}

	journalPath := resolvePath(now)
	entry := formatEntry(now, imageMarkdownLink(name, ext), false)
	if err := appendToJournal(client, journalPath, entry); err != nil {
		fmt.Fprintf(stderr, "error updating journal: %v\n", err)
		return 1
	}

	fmt.Fprintf(stderr, "Saved %s and linked in %s\n", attPath, journalPath)
	return 0
}
