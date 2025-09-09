package main

// Content length threshold for markdown formatting (1MB default)
const maxMarkdownContentLength = 1024 * 1024

// formatAsFencedMarkdown formats content as a fenced markdown block with language information
func formatAsFencedMarkdown(content string, mimeType string, filePath string) string {
	if content == "" {
		return ""
	}

	// Skip formatting for very large content to avoid performance issues
	if len(content) > maxMarkdownContentLength {
		return "```\n[Content too large for markdown formatting]\n```"
	}

	// Map MIME types to language identifiers for fenced blocks
	var lang string
	
	switch mimeType {
	case "text/plain; charset=utf-8", "text/plain":
		lang = "txt"
	case "text/html":
		lang = "html"
	case "text/css":
		lang = "css"
	case "text/javascript", "application/javascript":
		lang = "js"
	case "text/x-go", "application/go":
		lang = "go"
	case "text/xml":
		lang = "xml"
	case "text/markdown", "text/x-markdown":
		lang = "md"
	case "application/json":
		lang = "json"
	case "application/ld+json":
		lang = "json"
	case "text/yaml", "application/x-yaml":
		lang = "yaml"
	case "text/x-python", "application/x-python":
		lang = "py"
	case "text/x-java", "application/x-java":
		lang = "java"
	case "text/x-c++", "application/x-c++":
		lang = "cpp"
	case "text/x-ruby", "application/x-ruby":
		lang = "rb"
	default:
		// Fall back to file extension for language detection
		lang = getLanguageFromExtension(filePath)
	}

	if lang != "" {
		return "```" + lang + "\n" + content + "\n```"
	} else {
		// For unknown types, return as plain text
		return "```\n" + content + "\n```"
	}
}