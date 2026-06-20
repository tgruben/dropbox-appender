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

# Save a sketch from an .excalidraw JSON file on stdin
cat drawing.excalidraw | dropbox-appender sketch

# Paste the current clipboard image into /Notes/attachments and link it
dropbox-appender image
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

### `image` subcommand

`dropbox-appender image` reads an image from the Wayland clipboard via
`wl-paste`, uploads it to `/Notes/attachments/image-YYYYMMDD-HHMMSS.png`, and
appends a markdown image link to today's journal. Useful for pasting
screenshots directly into your notes.

```markdown
### 14:30:45
![image-20250115-143045](../../../attachments/image-20250115-143045.png)
```

Flags:

- `-name` — filename without extension (default: `image-YYYYMMDD-HHMMSS`)
- `-folder` — Dropbox folder for attachments (default: `/Notes/attachments`)
- `-type` — clipboard MIME type (default: `image/png`; also supports
  `image/jpeg`, `image/gif`, `image/webp`, `image/bmp`)

## License

[MIT](LICENSE)
