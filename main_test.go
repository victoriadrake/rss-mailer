package main

import (
	"strings"
	"testing"
)

func TestBuildEmail(t *testing.T) {
	env := map[string]string{
		"TITLE":            "My Newsletter",
		"SENDER_NAME":      "Victoria",
		"SENDER_EMAIL":     "hi@example.com",
		"WEBSITE":          "https://example.com",
		"UNSUBSCRIBE_LINK": "https://example.com/unsub",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	event := Invocation{
		Title:       "New Post",
		Description: "A short description.",
		Content:     "<p>Rich HTML content</p>",
		Plain:       "Plain text content",
		Link:        "https://example.com/posts/new-post",
	}

	input := buildEmail(event, "subscriber@example.com", "sub-123")

	if got := *input.Message.Subject.Data; got != "My Newsletter: New Post" {
		t.Errorf("subject = %q, want %q", got, "My Newsletter: New Post")
	}
	if got := input.Destination.ToAddresses[0]; got != "subscriber@example.com" {
		t.Errorf("recipient = %q", got)
	}
	if got := *input.Source; !strings.Contains(got, "Victoria") || !strings.Contains(got, "hi@example.com") {
		t.Errorf("source = %q, missing sender name/email", got)
	}
	html := *input.Message.Body.Html.Data
	if !strings.Contains(html, event.Content) {
		t.Errorf("html body missing content")
	}
	if !strings.Contains(html, "subscriber@example.com") || !strings.Contains(html, "sub-123") {
		t.Errorf("html body missing unsubscribe params")
	}
	plain := *input.Message.Body.Text.Data
	if !strings.Contains(plain, event.Plain) || !strings.Contains(plain, event.Link) {
		t.Errorf("plain body missing content or link")
	}
}
