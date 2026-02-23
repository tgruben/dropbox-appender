package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://content.dropboxapi.com"

// DropboxClient talks to the Dropbox content API.
type DropboxClient struct {
	Token   string
	BaseURL string // override for testing
}

func (c *DropboxClient) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}

// Download fetches a file from Dropbox. Returns empty string if file doesn't exist.
func (c *DropboxClient) Download(path string) (string, error) {
	arg, _ := json.Marshal(map[string]string{"path": path})

	req, err := http.NewRequest("POST", c.baseURL()+"/2/files/download", nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Dropbox-API-Arg", string(arg))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == 409 {
		var apiErr struct {
			ErrorSummary string `json:"error_summary"`
		}
		json.Unmarshal(body, &apiErr)
		if strings.Contains(apiErr.ErrorSummary, "not_found") {
			return "", nil
		}
		return "", fmt.Errorf("dropbox API error: %s", apiErr.ErrorSummary)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

// Upload writes content to a file in Dropbox, overwriting if it exists.
func (c *DropboxClient) Upload(path string, content string) error {
	arg, _ := json.Marshal(map[string]interface{}{
		"path": path,
		"mode": "overwrite",
		"mute": true,
	})

	req, err := http.NewRequest("POST", c.baseURL()+"/2/files/upload", strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Dropbox-API-Arg", string(arg))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
