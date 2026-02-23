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
