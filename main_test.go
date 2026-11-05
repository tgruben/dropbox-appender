package main

import (
	"strings"
	"testing"
	"time"
)

func TestResolvePath(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)
	path := resolvePath(now)
	expected := "/Notes/Journal/2025/01/Note20250115.md"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestResolvePath_December(t *testing.T) {
	now := time.Date(2025, 12, 3, 9, 0, 0, 0, time.UTC)
	path := resolvePath(now)
	expected := "/Notes/Journal/2025/12/Note20251203.md"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestFormatEntry(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	entry := formatEntry(now, "Had a great meeting", false)
	expected := "### 14:30:45\nHad a great meeting\n"
	if entry != expected {
		t.Errorf("expected %q, got %q", expected, entry)
	}
}

func TestFormatEntry_NoTimestamp(t *testing.T) {
	now := time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC)
	entry := formatEntry(now, "Had a great meeting", true)
	expected := "Had a great meeting\n"
	if entry != expected {
		t.Errorf("expected %q, got %q", expected, entry)
	}
}

func TestAppendContent_Empty(t *testing.T) {
	result := appendContent("", "### 14:30:45\nnew entry\n")
	expected := "### 14:30:45\nnew entry\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestAppendContent_Existing(t *testing.T) {
	existing := "### 10:00:00\nmorning note\n"
	entry := "### 14:30:45\nafternoon note\n"
	result := appendContent(existing, entry)
	expected := "### 10:00:00\nmorning note\n\n### 14:30:45\nafternoon note\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/Notes/Projects/alpha", "/Notes/Projects/alpha.md"},
		{"Notes/Projects/alpha", "/Notes/Projects/alpha.md"},
		{"/Notes/Projects/alpha.md", "/Notes/Projects/alpha.md"},
		{"alpha", "/alpha.md"},
		{"/Notes/my note.md", "/Notes/my note.md"},
	}
	for _, c := range cases {
		if got := normalizePath(c.in); got != c.want {
			t.Errorf("normalizePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFileLink(t *testing.T) {
	got := fileLink("/Notes/Projects/alpha.md")
	want := "[[Projects/alpha]]"
	if got != want {
		t.Errorf("fileLink = %q, want %q", got, want)
	}
}

func TestResolveToken_EnvVar(t *testing.T) {
	t.Setenv("DROPBOX_TOKEN", "direct_token")
	cfg := &Config{}
	token, err := resolveToken(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "direct_token" {
		t.Errorf("expected direct_token, got %s", token)
	}
}

func TestResolveToken_EnvVarTakesPriority(t *testing.T) {
	t.Setenv("DROPBOX_TOKEN", "direct_token")
	cfg := &Config{AppKey: "k", AppSecret: "s", RefreshToken: "r"}
	token, err := resolveToken(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "direct_token" {
		t.Errorf("expected direct_token, got %s", token)
	}
}

func TestResolveToken_NoAuth(t *testing.T) {
	t.Setenv("DROPBOX_TOKEN", "")
	cfg := &Config{}
	_, err := resolveToken(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "dropbox-appender auth") {
		t.Errorf("expected guidance to run auth, got: %v", err)
	}
}
