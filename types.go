package main

type writeStrategy string

const (
	strategyOverwrite    writeStrategy = "overwrite"
	strategyNoClobber    writeStrategy = "no_clobber"
	strategyAppend       writeStrategy = "append"
	strategyPrepend      writeStrategy = "prepend"
	strategyReplaceRange writeStrategy = "replace_range"
)

type encodingKind string

const (
	encText   encodingKind = "text"
	encBase64 encodingKind = "base64"
)

type MetaFields struct {
	Mode       string `json:"mode,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

type ReadArgs struct {
	Path     string `json:"path"`
	Encoding string `json:"encoding,omitempty"` // text|base64 (auto if empty)
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type ReadResult struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	MIMEType  string `json:"mime_type"`
	SHA256    string `json:"sha256"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	MetaFields
}

type PeekArgs struct {
	Path     string `json:"path"`
	Offset   int    `json:"offset,omitempty"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type PeekResult struct {
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	Size     int64  `json:"size"`
	EOF      bool   `json:"eof"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
	MetaFields
}

type WriteArgs struct {
	Path       string        `json:"path"`
	Encoding   string        `json:"encoding"` // text|base64
	Content    string        `json:"content"`
	Strategy   writeStrategy `json:"strategy,omitempty"`
	CreateDirs *bool         `json:"create_dirs,omitempty"`
	Mode       string        `json:"mode,omitempty"` // e.g. 0644
	Start      *int          `json:"start,omitempty"`
	End        *int          `json:"end,omitempty"`
}

type WriteResult struct {
	Path     string `json:"path"`
	Action   string `json:"action"`
	Bytes    int    `json:"bytes"`
	Created  bool   `json:"created"`
	MIMEType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	MetaFields
}

type EditArgs struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
	Regex   bool   `json:"regex,omitempty"`
	Count   int    `json:"count,omitempty"`
}

type EditResult struct {
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
	Bytes        int    `json:"bytes"`
	SHA256       string `json:"sha256"`
	MetaFields
}

type ListArgs struct {
	Path       string `json:"path"`
	Recursive  bool   `json:"recursive,omitempty"`
	MaxEntries int    `json:"max_entries,omitempty"`
}

type ListEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode"`
	ModifiedAt string `json:"modified_at"`
}

type ListResult struct {
	Entries []ListEntry `json:"entries"`
}

type GlobArgs struct {
	Pattern    string `json:"pattern"`
	MaxResults int    `json:"max_results,omitempty"`
}

type GlobResult struct {
	Matches []string `json:"matches"`
}

type SearchArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Regex      bool   `json:"regex,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

type SearchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type SearchResult struct {
	Matches []SearchMatch `json:"matches"`
}
