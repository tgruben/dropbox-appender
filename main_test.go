package main

import (
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
