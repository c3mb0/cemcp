package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrorCategory defines the general category of errors
type ErrorCategory string

const (
	CategoryValidation ErrorCategory = "VALIDATION"
	CategoryPermission ErrorCategory = "PERMISSION"  
	CategoryResource   ErrorCategory = "RESOURCE"
	CategorySystem     ErrorCategory = "SYSTEM"
	CategoryNetwork    ErrorCategory = "NETWORK"
	CategoryOperation  ErrorCategory = "OPERATION"
)

// ErrorSeverity defines the severity level of errors
type ErrorSeverity string

const (
	SeverityInfo     ErrorSeverity = "INFO"
	SeverityWarning  ErrorSeverity = "WARNING"
	SeverityError    ErrorSeverity = "ERROR"
	SeverityCritical ErrorSeverity = "CRITICAL"
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

	// New comprehensive error types
	ErrInvalidArgument   = errors.New("invalid argument provided")
	ErrArgumentRequired  = errors.New("required argument missing")
	ErrArgumentRange     = errors.New("argument value out of range")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrResourceExhausted = errors.New("resource exhausted")
	ErrSystemFailure     = errors.New("system failure")
	ErrNetworkFailure    = errors.New("network failure")
	ErrTimeout           = errors.New("operation timeout")
	ErrCanceled          = errors.New("operation canceled")
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

// ErrorResponse provides standardized error reporting across all MCP tools
type ErrorResponse struct {
	Message     string                 `json:"error"`              // Human-readable error message
	Code        string                 `json:"code"`               // Machine-readable error code
	Category    ErrorCategory          `json:"category"`           // Error category for grouping
	Severity    ErrorSeverity          `json:"severity"`           // Error severity level
	Operation   string                 `json:"operation,omitempty"`// Operation that failed
	Path        string                 `json:"path,omitempty"`     // File path if relevant
	Timestamp   string                 `json:"timestamp"`          // ISO 8601 timestamp
	RequestID   string                 `json:"requestId,omitempty"`// Request ID for tracking
	Context     map[string]interface{} `json:"context,omitempty"`  // Additional context
	Recovery    *RecoveryHint          `json:"recovery,omitempty"` // Recovery suggestions
	Cause       *ErrorResponse         `json:"cause,omitempty"`    // Root cause error
}

// RecoveryHint provides suggestions for error recovery
type RecoveryHint struct {
	Suggestion string   `json:"suggestion"`          // Human-readable suggestion
	Actions    []string `json:"actions,omitempty"`   // Specific actions to take
	Examples   []string `json:"examples,omitempty"`  // Example corrections
}

// toErrorResponse converts errors to standardized format
func toErrorResponse(err error) ErrorResponse {
	resp := ErrorResponse{
		Message:   err.Error(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Severity:  SeverityError,
	}

	// Extract operation error details
	var opErr *OperationError
	if errors.As(err, &opErr) {
		resp.Operation = opErr.Op
		resp.Path = opErr.Path
		resp.Message = opErr.Err.Error()
		if opErr.Details != "" {
			resp.Context = map[string]interface{}{
				"details": opErr.Details,
			}
		}
	}

	// Comprehensive error code mapping with categories and recovery hints
	switch {
	// Validation errors
	case errors.Is(err, ErrPathRequired):
		resp.Code = "VALIDATION_PATH_REQUIRED"
		resp.Category = CategoryValidation
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Provide a valid file path",
			Actions:    []string{"Include 'path' parameter in request"},
			Examples:   []string{`"path": "/path/to/file.txt"`},
		}
	case errors.Is(err, ErrPatternRequired):
		resp.Code = "VALIDATION_PATTERN_REQUIRED"
		resp.Category = CategoryValidation
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Provide a search pattern",
			Actions:    []string{"Include 'pattern' parameter in request"},
			Examples:   []string{`"pattern": "*.js"`, `"pattern": "function.*\\("` },
		}
	case errors.Is(err, ErrInvalidRegex):
		resp.Code = "VALIDATION_INVALID_REGEX"
		resp.Category = CategoryValidation
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Fix the regular expression syntax",
			Actions:    []string{"Check regex syntax", "Escape special characters"},
			Examples:   []string{`"pattern": "func\\("`, `"pattern": "[a-zA-Z]+"`},
		}
	case errors.Is(err, ErrInvalidGlob):
		resp.Code = "VALIDATION_INVALID_GLOB"
		resp.Category = CategoryValidation
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Fix the glob pattern syntax",
			Actions:    []string{"Check glob syntax", "Use valid wildcards"},
			Examples:   []string{`"pattern": "*.js"`, `"pattern": "src/**/*.go"`},
		}

	// Permission errors
	case errors.Is(err, ErrPathOutsideRoot):
		resp.Code = "PERMISSION_PATH_ESCAPE"
		resp.Category = CategoryPermission
		resp.Severity = SeverityCritical
		resp.Recovery = &RecoveryHint{
			Suggestion: "Use paths within the allowed base folder",
			Actions:    []string{"Remove '..' path components", "Use absolute paths within base"},
		}

	// Resource errors
	case errors.Is(err, ErrPathNotFound):
		resp.Code = "RESOURCE_NOT_FOUND"
		resp.Category = CategoryResource
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Verify the file or directory exists",
			Actions:    []string{"Check path spelling", "Create missing directories", "Use correct file extension"},
		}
	case errors.Is(err, ErrFileExists):
		resp.Code = "RESOURCE_ALREADY_EXISTS"
		resp.Category = CategoryResource
		resp.Severity = SeverityWarning
		resp.Recovery = &RecoveryHint{
			Suggestion: "Use different filename or overwrite strategy",
			Actions:    []string{"Choose unique filename", "Use 'overwrite' write strategy"},
			Examples:   []string{`"writeStrategy": "overwrite"`},
		}
	case errors.Is(err, ErrInsufficientSpace):
		resp.Code = "RESOURCE_NO_SPACE"
		resp.Category = CategoryResource
		resp.Severity = SeverityCritical
		resp.Recovery = &RecoveryHint{
			Suggestion: "Free up disk space or use smaller files",
			Actions:    []string{"Delete unnecessary files", "Move files to different location", "Use streaming write"},
		}
	case errors.Is(err, ErrFileTooLarge):
		resp.Code = "RESOURCE_FILE_TOO_LARGE"
		resp.Category = CategoryResource
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Reduce file size or process in smaller chunks",
			Actions:    []string{"Split large file", "Use streaming operations", "Increase size limit if allowed"},
		}

	// System errors  
	case errors.Is(err, ErrPathIsSymlink):
		resp.Code = "SYSTEM_SYMLINK_DETECTED"
		resp.Category = CategorySystem
		resp.Severity = SeverityWarning
		resp.Recovery = &RecoveryHint{
			Suggestion: "Use the target file path instead of symlink",
			Actions:    []string{"Resolve symlink target", "Use direct file path"},
		}
	case errors.Is(err, ErrPathIsDirectory):
		resp.Code = "SYSTEM_IS_DIRECTORY"
		resp.Category = CategorySystem
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Specify a file path, not a directory",
			Actions:    []string{"Add filename to path", "Use list operation for directories"},
		}
	case errors.Is(err, ErrPathNotRegular):
		resp.Code = "SYSTEM_NOT_REGULAR_FILE"
		resp.Category = CategorySystem
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Use path to a regular file (not device, socket, etc.)",
			Actions:    []string{"Check file type", "Use different file path"},
		}

	// Operation errors
	case errors.Is(err, ErrLockTimeout):
		resp.Code = "OPERATION_LOCK_TIMEOUT"
		resp.Category = CategoryOperation
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Retry operation or check for stuck locks",
			Actions:    []string{"Wait and retry", "Check for long-running operations", "Restart service if needed"},
		}
	case errors.Is(err, ErrInvalidStrategy):
		resp.Code = "OPERATION_INVALID_STRATEGY"
		resp.Category = CategoryOperation
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Use a valid write strategy",
			Actions:    []string{"Check valid strategy names"},
			Examples:   []string{`"writeStrategy": "create"`, `"writeStrategy": "overwrite"`, `"writeStrategy": "append"`},
		}

	default:
		resp.Code = "UNKNOWN_ERROR"
		resp.Category = CategorySystem
		resp.Severity = SeverityError
		resp.Recovery = &RecoveryHint{
			Suggestion: "Check error message for details and retry",
			Actions:    []string{"Review error details", "Check system logs", "Contact support if persistent"},
		}
	}

	return resp
}

// NewStandardError creates a standardized error response
func NewStandardError(code string, category ErrorCategory, severity ErrorSeverity, message string, operation string) ErrorResponse {
	return ErrorResponse{
		Message:   message,
		Code:      code,
		Category:  category,
		Severity:  severity,
		Operation: operation,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewValidationError creates a validation error response
func NewValidationError(field, message string, examples []string) ErrorResponse {
	return ErrorResponse{
		Message:   fmt.Sprintf("validation failed for %s: %s", field, message),
		Code:      fmt.Sprintf("VALIDATION_%s", strings.ToUpper(field)),
		Category:  CategoryValidation,
		Severity:  SeverityError,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Context: map[string]interface{}{
			"field": field,
		},
		Recovery: &RecoveryHint{
			Suggestion: fmt.Sprintf("Fix the %s parameter", field),
			Examples:   examples,
		},
	}
}

// FormatErrorResponse formats an ErrorResponse as a JSON string for Go error interface
func (e ErrorResponse) Error() string {
	// Return JSON representation for structured error handling
	data, _ := json.MarshalIndent(e, "", "  ")
	return string(data)
}

// WrapError creates a standardized error from a Go error
func WrapError(err error, operation, path string) error {
	if err == nil {
		return nil
	}
	
	// If it's already an ErrorResponse, return as-is
	var errResp ErrorResponse
	if strings.HasPrefix(err.Error(), "{") {
		// Try to parse as JSON ErrorResponse
		if json.Unmarshal([]byte(err.Error()), &errResp) == nil {
			return err
		}
	}
	
	// Create new standardized error
	errResp = toErrorResponse(err)
	if operation != "" {
		errResp.Operation = operation
	}
	if path != "" {
		errResp.Path = path
	}
	
	return errResp
}
