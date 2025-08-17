package main

import (
	"errors"
	"fmt"
)

// Error types for better error handling and agent processing
var (
	// Path errors
	ErrPathRequired    = errors.New("path is required")
	ErrPathOutsideRoot = errors.New("path escapes base folder")
	ErrPathNotFound    = errors.New("path not found")
	ErrPathIsSymlink   = errors.New("path is a symlink")
	ErrPathIsDirectory = errors.New("path is a directory")
	ErrPathNotRegular  = errors.New("path is not a regular file")

	// Operation errors
	ErrFileExists        = errors.New("file already exists")
	ErrInsufficientSpace = errors.New("insufficient disk space")
	ErrFileTooLarge      = errors.New("file exceeds size limit")
	ErrLockTimeout       = errors.New("lock acquisition timeout")
	ErrInvalidStrategy   = errors.New("invalid write strategy")

	// Pattern errors
	ErrPatternRequired = errors.New("pattern is required")
	ErrInvalidRegex    = errors.New("invalid regular expression")
	ErrInvalidGlob     = errors.New("invalid glob pattern")
)

// OperationError provides detailed context for failed operations
type OperationError struct {
	Op      string // Operation name
	Path    string // File path
	Err     error  // Underlying error
	Details string // Additional details
}

func (e *OperationError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s %s: %v (%s)", e.Op, e.Path, e.Err, e.Details)
	}
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

// ValidationError for input validation failures
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s (value: %v)", e.Field, e.Message, e.Value)
}

// newOpError creates a new operation error
func newOpError(op, path string, err error, details ...string) error {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	return &OperationError{
		Op:      op,
		Path:    path,
		Err:     err,
		Details: detail,
	}
}

// ErrorResponse for agent-friendly error reporting
type ErrorResponse struct {
	Error     string            `json:"error"`
	Code      string            `json:"code"`
	Operation string            `json:"operation,omitempty"`
	Path      string            `json:"path,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

// toErrorResponse converts errors to agent-friendly format
func toErrorResponse(err error) ErrorResponse {
	resp := ErrorResponse{
		Error: err.Error(),
	}

	// Extract operation error details
	var opErr *OperationError
	if errors.As(err, &opErr) {
		resp.Operation = opErr.Op
		resp.Path = opErr.Path
		resp.Error = opErr.Err.Error()
	}

	// Set error codes for common errors
	switch {
	case errors.Is(err, ErrPathOutsideRoot):
		resp.Code = "PATH_ESCAPE"
	case errors.Is(err, ErrPathNotFound):
		resp.Code = "NOT_FOUND"
	case errors.Is(err, ErrFileExists):
		resp.Code = "ALREADY_EXISTS"
	case errors.Is(err, ErrInsufficientSpace):
		resp.Code = "NO_SPACE"
	case errors.Is(err, ErrFileTooLarge):
		resp.Code = "FILE_TOO_LARGE"
	case errors.Is(err, ErrLockTimeout):
		resp.Code = "LOCK_TIMEOUT"
	default:
		resp.Code = "UNKNOWN_ERROR"
	}

	return resp
}
