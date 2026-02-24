# OAuth Authentication — Design

## Summary

Add OAuth2 authentication with refresh tokens to dropbox-appender. One-time `dropbox-appender auth` command for setup, then silent token refresh on every run.

## Auth Subcommand

`dropbox-appender auth`:
1. Reads app_key + app_secret from config file or env vars
2. Prints Dropbox authorize URL
3. Prompts user to paste authorization code
4. Exchanges code for refresh token via `POST https://api.dropboxapi.com/oauth2/token`
5. Saves refresh_token to config file
6. Prints success

## Config File

Path: `~/.config/dropbox-appender/config.json`

```json
{
  "app_key": "your_app_key",
  "app_secret": "your_app_secret",
  "refresh_token": "stored_after_auth"
}
```

Env var overrides: `DROPBOX_APP_KEY`, `DROPBOX_APP_SECRET`, `DROPBOX_REFRESH_TOKEN`.

## Token Resolution (every normal run)

Priority order:
1. `DROPBOX_TOKEN` env var → use directly (backward compat)
2. Refresh token (config/env) + app key/secret → call `/oauth2/token` with `grant_type=refresh_token` for fresh short-lived access token
3. Nothing → error: `"run: dropbox-appender auth"`

No caching of short-lived tokens — tool runs briefly.

## Error Handling

| Condition | Behavior |
|---|---|
| No app_key/secret for `auth` | Exit 1: guidance to set in config or env |
| Invalid auth code | Exit 1: print Dropbox error |
| Refresh token expired/revoked | Exit 1: `"refresh token invalid, run: dropbox-appender auth"` |
| `DROPBOX_TOKEN` set | Use directly, skip refresh flow |

## New/Modified Files

```
├── main.go           # Add auth subcommand routing
├── auth.go           # Auth flow: authorize URL, code exchange, token refresh
├── auth_test.go      # Tests with mock OAuth server
├── config.go         # Config file read/write
├── config_test.go    # Config tests
├── dropbox.go        # Unchanged
├── dropbox_test.go   # Unchanged
```

## Dropbox OAuth Endpoints

- Authorize: `https://www.dropbox.com/oauth2/authorize?client_id=APP_KEY&response_type=code&token_access_type=offline`
- Token exchange: `POST https://api.dropboxapi.com/oauth2/token` with `grant_type=authorization_code`
- Token refresh: `POST https://api.dropboxapi.com/oauth2/token` with `grant_type=refresh_token`
