# dropbox-appender

A minimal Go CLI that appends timestamped entries to a daily journal file in Dropbox.

Entries go to `/Notes/Journal/YYYY/MM/NoteYYYYMMDD.md` with a `### HH:MM:SS` header.

## Setup

1. Go to [Dropbox App Console](https://www.dropbox.com/developers/apps)
2. Create an app with **Full Dropbox** access
3. Generate an access token
4. Export it:

```bash
export DROPBOX_TOKEN="your-token-here"
```

## Install

```bash
go install github.com/tgruben/dropbox-appender@latest
```

Or build from source:

```bash
git clone https://github.com/tgruben/dropbox-appender.git
cd dropbox-appender
go build -o dropbox-appender .
```

## Usage

```bash
# Pass text as argument
dropbox-appender "Had a great meeting"

# Pipe from stdin
echo "Some note" | dropbox-appender

# Without timestamp header
dropbox-appender -no-timestamp "Just the text"
```

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
