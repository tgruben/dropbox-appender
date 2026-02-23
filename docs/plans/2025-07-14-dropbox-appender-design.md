# Dropbox Daily Note Appender — Design

## Summary

A single-binary Go CLI that appends timestamped entries to a daily journal file in Dropbox at `/Journal/YYYY/MM/NoteYYYYMMDD.md`. Uses the Dropbox REST API directly (no SDK).

## Usage

```bash
# CLI argument
dropbox-appender "Had a great meeting"

# Stdin
echo "Some note" | dropbox-appender

# Both provided — CLI arg wins, stdin ignored
dropbox-appender "This takes priority"
```

## Authentication

- `DROPBOX_TOKEN` environment variable (full Dropbox access scope)
- Fails fast with clear message if unset

## Data Flow

1. **Resolve path** — compute `/Journal/YYYY/MM/NoteYYYYMMDD.md` from current local time
2. **Read input** — check CLI args first, then stdin; error if neither provided
3. **Format entry** — prepend `### HH:MM:SS\n` to the input text, add trailing newline
4. **Download existing file** — `POST /2/files/download`; if 409 (not found), start empty
5. **Append** — concatenate existing content + `\n` (if non-empty) + new entry
6. **Upload** — `POST /2/files/upload` with `mode: overwrite`, `mute: true`

Dropbox creates intermediate directories automatically on upload.

## API Endpoints

- `POST https://content.dropboxapi.com/2/files/download` — get existing file
- `POST https://content.dropboxapi.com/2/files/upload` — write file (overwrite mode)

## Error Handling

| Condition | Behavior |
|---|---|
| No `DROPBOX_TOKEN` | Exit 1: `"DROPBOX_TOKEN environment variable is required"` |
| No input (no arg, no stdin) | Exit 1: `"no input provided: pass text as argument or via stdin"` |
| Download 409 (path/not_found) | Not an error — new file, start with empty content |
| Other API errors | Exit 1, print status code + error body |
| Network failures | Exit 1, print underlying error |

## Project Structure

```
dropbox-appender/
├── main.go           # CLI entry: arg/stdin parsing, env var, orchestration
├── dropbox.go        # Dropbox API client: download, upload
├── dropbox_test.go   # Tests with HTTP test server mocking Dropbox
├── go.mod
└── README.md         # Usage, setup instructions
```

## Approach

Direct Dropbox HTTP API via `net/http`. Zero external dependencies. Only two API calls needed — an SDK is overkill for this scope.
