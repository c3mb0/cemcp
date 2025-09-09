package main

import (
	"strings"
	"testing"
)

func TestFormatAsFencedMarkdown(t *testing.T) {
	testCases := []struct {
		content    string
		mimeType   string
		filePath   string
		expected   string
	}{
		{
			content:  "Hello World",
			mimeType: "text/plain; charset=utf-8",
			filePath: "test.txt",
			expected: "```txt\nHello World\n```",
		},
		{
			content:  "{ \"name\": \"test\" }",
			mimeType: "application/json",
			filePath: "test.json",
			expected: "```json\n{ \"name\": \"test\" }\n```",
		},
		{
			content:  "<html><body>Hello</body></html>",
			mimeType: "text/html",
			filePath: "test.html",
			expected: "```html\n<html><body>Hello</body></html>\n```",
		},
		{
			content:  "function hello() { return 'world'; }",
			mimeType: "application/octet-stream", // Unknown MIME type
			filePath: "test.js",
			expected: "```js\nfunction hello() { return 'world'; }\n```",
		},
		{
			content:  "fn main() { println!(\"Hello\"); }",
			mimeType: "application/octet-stream", // Unknown MIME type
			filePath: "main.rs",
			expected: "```rust\nfn main() { println!(\"Hello\"); }\n```",
		},
	}

	for _, tc := range testCases {
		result := formatAsFencedMarkdown(tc.content, tc.mimeType, tc.filePath)
		if result != tc.expected {
			t.Errorf("formatAsFencedMarkdown(%q, %q, %q) = %q; want %q", tc.content, tc.mimeType, tc.filePath, result, tc.expected)
		}
	}
}

func TestFormatAsFencedMarkdownContentLength(t *testing.T) {
	// Test content length validation
	largeContent := strings.Repeat("A", maxMarkdownContentLength+1)
	result := formatAsFencedMarkdown(largeContent, "text/plain", "test.txt")
	expected := "```\n[Content too large for markdown formatting]\n```"
	if result != expected {
		t.Errorf("formatAsFencedMarkdown with large content = %q; want %q", result, expected)
	}

	// Test content at the threshold
	thresholdContent := strings.Repeat("B", maxMarkdownContentLength)
	result2 := formatAsFencedMarkdown(thresholdContent, "text/plain", "test.txt")
	expected2 := "```txt\n" + thresholdContent + "\n```"
	if result2 != expected2 {
		t.Errorf("formatAsFencedMarkdown with threshold content failed")
	}
}