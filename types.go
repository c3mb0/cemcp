package main

// Write strategies define how content is written to files
type writeStrategy string

const (
	strategyOverwrite    writeStrategy = "overwrite"     // Replace entire file content
	strategyNoClobber    writeStrategy = "no_clobber"    // Fail if file exists
	strategyAppend       writeStrategy = "append"        // Add to end of file
	strategyPrepend      writeStrategy = "prepend"       // Add to beginning of file
	strategyReplaceRange writeStrategy = "replace_range" // Replace specific byte range
)

// Encoding types for file content
type encodingKind string

const (
	encText   encodingKind = "text"   // UTF-8 text content
	encBase64 encodingKind = "base64" // Base64 encoded binary
)

// MetaFields contains common file metadata
type MetaFields struct {
	Mode       string `json:"mode,omitempty"`        // File permissions in octal
	ModifiedAt string `json:"modified_at,omitempty"` // Last modification time (RFC3339)
}

// ReadArgs defines parameters for reading files
type ReadArgs struct {
	Path     string `json:"path" description:"File path or file:// URI within root"`
	Encoding string `json:"encoding,omitempty" description:"Force text or base64; auto-detected if empty"`
	MaxBytes int    `json:"max_bytes,omitempty" description:"Maximum bytes to return (default 64KB)"`
}

// ReadResult contains file read operation results
type ReadResult struct {
	Path      string `json:"path" description:"Original requested path"`
	Size      int64  `json:"size" description:"Total file size in bytes"`
	MIMEType  string `json:"mime_type" description:"Detected MIME type"`
	SHA256    string `json:"sha256" description:"SHA256 hash of content (if under 32MB)"`
	Encoding  string `json:"encoding" description:"Content encoding used (text/base64)"`
	Content   string `json:"content" description:"File content (possibly truncated)"`
	Truncated bool   `json:"truncated" description:"Whether content was truncated"`
	MetaFields
}

// PeekArgs defines parameters for peeking into files
type PeekArgs struct {
	Path     string `json:"path" description:"File path"`
	Offset   int    `json:"offset,omitempty" description:"Byte offset to start at (default 0)"`
	MaxBytes int    `json:"max_bytes,omitempty" description:"Window size in bytes (default 4KB)"`
}

// PeekResult contains file peek operation results
type PeekResult struct {
	Path     string `json:"path" description:"Original requested path"`
	Offset   int    `json:"offset" description:"Starting byte offset"`
	Size     int64  `json:"size" description:"Total file size"`
	EOF      bool   `json:"eof" description:"Whether window reached end of file"`
	Encoding string `json:"encoding" description:"Content encoding (text/base64)"`
	Content  string `json:"content" description:"Window content"`
	MetaFields
}

// WriteArgs defines parameters for writing files
type WriteArgs struct {
	Path       string        `json:"path" description:"Target file path"`
	Encoding   string        `json:"encoding" description:"Content encoding: text or base64"`
	Content    string        `json:"content" description:"Data to write"`
	Strategy   writeStrategy `json:"strategy,omitempty" description:"Write behavior (default overwrite)"`
	CreateDirs *bool         `json:"create_dirs,omitempty" description:"Create parent directories if needed"`
	Mode       string        `json:"mode,omitempty" description:"File mode in octal (e.g., 0644)"`
	Start      *int          `json:"start,omitempty" description:"Start byte for replace_range strategy"`
	End        *int          `json:"end,omitempty" description:"End byte (exclusive) for replace_range"`
}

// WriteResult contains file write operation results
type WriteResult struct {
	Path     string `json:"path" description:"File path written"`
	Action   string `json:"action" description:"Write strategy used"`
	Bytes    int    `json:"bytes" description:"Total bytes in final file"`
	Created  bool   `json:"created" description:"Whether file was newly created"`
	MIMEType string `json:"mime_type" description:"Detected MIME type"`
	SHA256   string `json:"sha256" description:"SHA256 of final content"`
	MetaFields
}

// EditArgs defines parameters for editing files
type EditArgs struct {
	Path    string `json:"path" description:"Target text file"`
	Pattern string `json:"pattern" description:"Substring or regex to match"`
	Replace string `json:"replace" description:"Replacement text"`
	Regex   bool   `json:"regex,omitempty" description:"Treat pattern as regex"`
	Count   int    `json:"count,omitempty" description:"Max replacements (0=all)"`
}

// EditResult contains file edit operation results
type EditResult struct {
	Path         string `json:"path" description:"File path edited"`
	Replacements int    `json:"replacements" description:"Number of replacements made"`
	Bytes        int    `json:"bytes" description:"Final file size"`
	SHA256       string `json:"sha256" description:"SHA256 of final content"`
	MetaFields
}

// ListArgs defines parameters for listing directories
type ListArgs struct {
	Path       string `json:"path" description:"Directory to list"`
	Recursive  bool   `json:"recursive,omitempty" description:"Recurse into subdirectories"`
	MaxEntries int    `json:"max_entries,omitempty" description:"Maximum entries to return"`
}

// ListEntry represents a single file/directory entry
type ListEntry struct {
	Path       string `json:"path" description:"Relative path from root"`
	Name       string `json:"name" description:"Base filename"`
	Kind       string `json:"kind" description:"Type: file/dir/symlink/other"`
	Size       int64  `json:"size" description:"Size in bytes"`
	Mode       string `json:"mode" description:"Permissions in octal"`
	ModifiedAt string `json:"modified_at" description:"Last modified time (RFC3339)"`
}

// ListResult contains directory listing results
type ListResult struct {
	Entries []ListEntry `json:"entries" description:"Directory entries"`
}

// GlobArgs defines parameters for glob pattern matching
type GlobArgs struct {
	Pattern    string `json:"pattern" description:"Glob pattern (supports ** for recursion)"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum matches to return"`
}

// GlobResult contains glob matching results
type GlobResult struct {
	Matches []string `json:"matches" description:"Matched file paths"`
}

// SearchArgs defines parameters for text search
type SearchArgs struct {
	Pattern    string `json:"pattern" description:"Text or regex pattern to find"`
	Path       string `json:"path,omitempty" description:"Start directory (default root)"`
	Regex      bool   `json:"regex,omitempty" description:"Interpret pattern as regex"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum matches to return"`
}

// SearchMatch represents a single search result
type SearchMatch struct {
	Path string `json:"path" description:"File path relative to root"`
	Line int    `json:"line" description:"Line number of match"`
	Text string `json:"text" description:"Matching line content"`
}

// SearchResult contains text search results
type SearchResult struct {
	Matches    []SearchMatch          `json:"matches" description:"Found matches"`
	Statistics map[string]interface{} `json:"statistics,omitempty" description:"Search statistics"`
}
