package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// isMultimodalFile checks if the file should be processed with multimodal capabilities
func isMultimodalFile(mimeType, filePath string) (string, bool) {
	// Image files
	if strings.HasPrefix(mimeType, "image/") {
		return "image", true
	}
	
	// PDF files
	if mimeType == "application/pdf" {
		return "pdf", true
	}
	
	// Jupyter notebooks
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".ipynb" {
		return "notebook", true
	}
	
	return "", false
}

// detectMultimodalMIME performs MIME detection with special handling for multimodal files
func detectMultimodalMIME(filePath string, data []byte) string {
	// First try standard MIME detection
	mimeType := detectMIME(filePath, data)
	
	// Special handling for notebooks
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".ipynb" {
		return "application/x-ipynb+json"
	}
	
	// Handle image formats that might not be detected properly
	if len(data) > 0 {
		if len(data) >= 4 {
			// PNG signature
			if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
				return "image/png"
			}
			// JPEG signature
			if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
				return "image/jpeg"
			}
			// GIF signature
			if len(data) >= 6 && (string(data[0:6]) == "GIF87a" || string(data[0:6]) == "GIF89a") {
				return "image/gif"
			}
			// WebP signature
			if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
				return "image/webp"
			}
		}
		// SVG detection (XML-based)
		if strings.Contains(strings.ToLower(string(data[:minInt(1024, len(data))])), "<svg") {
			return "image/svg+xml"
		}
	}
	
	return mimeType
}

// handleMultimodalRead processes multimodal files (images, PDFs, notebooks)
func handleMultimodalRead(sessions map[string]*SessionState, mu *sync.RWMutex) mcp.StructuredToolHandlerFunc[MultimodalReadArgs, MultimodalReadResult] {
	return func(ctx context.Context, req mcp.CallToolRequest, args MultimodalReadArgs) (MultimodalReadResult, error) {
		state, err := getSessionState(ctx, sessions, mu)
		if err != nil {
			return MultimodalReadResult{}, err
		}
		root := state.Root
		start := time.Now()
		
		// Create a timeout context for processing
		processingCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		
		dprintf("%s -> multimodal_read path=%q", sessionContext(ctx), args.Path)
		
		var res MultimodalReadResult
		
		// Validate required arguments
		if args.Path == "" {
			errResp := NewValidationError("path", "path is required", []string{`"path": "/path/to/file.jpg"`})
			errResp.Operation = "multimodal_read"
			return res, errResp
		}
		
		// Resolve file path
		full, err := safeJoinResolveFinal(root, args.Path)
		if err != nil {
			dprintf("multimodal_read error: %v", err)
			return res, WrapError(err, "multimodal_read", args.Path)
		}
		
		// Get file info
		fi, err := os.Stat(full)
		if err != nil {
			dprintf("multimodal_read stat error: %v", err)
			return res, WrapError(err, "multimodal_read", args.Path)
		}
		
		// Check file size limit (512MB max)
		const maxFileSize = 512 * 1024 * 1024 // 512MB
		if fi.Size() > maxFileSize {
			return res, fmt.Errorf("file %s is too large (%d bytes). Maximum supported file size is %d bytes (512MB)", 
				args.Path, fi.Size(), maxFileSize)
		}
		
		// Read file content for hash and analysis
		content, sha, err := readFileWithHash(full, fi.Size(), fi.Size())
		if err != nil {
			dprintf("multimodal_read read error: %v", err)
			return res, WrapError(err, "multimodal_read", args.Path)
		}
		
		// Detect MIME type
		mimeType := detectMultimodalMIME(full, content)
		
		// Check if this is a multimodal file
		fileType, isMultimodal := isMultimodalFile(mimeType, full)
		if !isMultimodal {
			return res, fmt.Errorf("file %s is not a supported multimodal type (detected: %s)", args.Path, mimeType)
		}
		
		// Base result
		res = MultimodalReadResult{
			Path:     args.Path,
			MIMEType: mimeType,
			FileType: fileType,
			SHA256:   sha,
			MetaFields: MetaFields{
				Mode:       fmt.Sprintf("%#o", fi.Mode()&os.ModePerm),
				ModifiedAt: fi.ModTime().UTC().Format(time.RFC3339),
			},
		}
		
		// Process based on file type with timeout
		switch fileType {
		case "image":
			resultChan := make(chan *ImageAnalysisResult, 1)
			errChan := make(chan error, 1)
			
			go func() {
				result, err := processImageWithContext(processingCtx, full, content, args.Options)
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}()
			
			select {
			case imageResult := <-resultChan:
				res.Image = imageResult
			case err := <-errChan:
				return res, fmt.Errorf("image processing failed: %v", err)
			case <-processingCtx.Done():
				return res, fmt.Errorf("image processing timed out")
			}
			
		case "pdf":
			resultChan := make(chan *PDFContentResult, 1)
			errChan := make(chan error, 1)
			
			go func() {
				result, err := processPDFWithContext(processingCtx, full, content, args.Options)
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}()
			
			select {
			case pdfResult := <-resultChan:
				res.PDF = pdfResult
			case err := <-errChan:
				return res, fmt.Errorf("PDF processing failed: %v", err)
			case <-processingCtx.Done():
				return res, fmt.Errorf("PDF processing timed out")
			}
			
		case "notebook":
			resultChan := make(chan *NotebookContentResult, 1)
			errChan := make(chan error, 1)
			
			go func() {
				result, err := processNotebookWithContext(processingCtx, full, content, args.Options)
				if err != nil {
					errChan <- err
				} else {
					resultChan <- result
				}
			}()
			
			select {
			case notebookResult := <-resultChan:
				res.Notebook = notebookResult
			case err := <-errChan:
				return res, fmt.Errorf("notebook processing failed: %v", err)
			case <-processingCtx.Done():
				return res, fmt.Errorf("notebook processing timed out")
			}
			
		default:
			return res, fmt.Errorf("unsupported file type: %s", fileType)
		}
		
		// Set processing time
		res.ProcessingTime = time.Since(start).Seconds()
		
		dprintf("<- multimodal_read ok type=%s dur=%s", fileType, time.Since(start))
		return res, nil
	}
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatMultimodalReadResult formats the multimodal read result for text output
func formatMultimodalReadResult(r MultimodalReadResult) string {
	result := fmt.Sprintf("path=%s type=%s mime=%s sha=%s processing_time=%.3fs", 
		r.Path, r.FileType, r.MIMEType, r.SHA256, r.ProcessingTime)
	
	switch r.FileType {
	case "image":
		if r.Image != nil {
			result += fmt.Sprintf(" image_size=%dx%d format=%s", r.Image.Width, r.Image.Height, r.Image.Format)
			if r.Image.OCRText != "" {
				result += fmt.Sprintf(" ocr_text_length=%d", len(r.Image.OCRText))
			}
		}
	case "pdf":
		if r.PDF != nil {
			result += fmt.Sprintf(" pages=%d text_length=%d", r.PDF.PageCount, len(r.PDF.TextContent))
		}
	case "notebook":
		if r.Notebook != nil {
			result += fmt.Sprintf(" cells=%d code_cells=%d markdown_cells=%d", 
				r.Notebook.CellCount, r.Notebook.CodeCells, r.Notebook.MarkdownCells)
		}
	}
	
	return result
}

// Context-aware wrapper functions for timeout handling

// processImageWithContext wraps processImage with context support
func processImageWithContext(ctx context.Context, filePath string, content []byte, optionsStr string) (*ImageAnalysisResult, error) {
	done := make(chan struct {
		result *ImageAnalysisResult
		err    error
	}, 1)
	
	go func() {
		result, err := processImage(filePath, content, optionsStr)
		done <- struct {
			result *ImageAnalysisResult
			err    error
		}{result, err}
	}()
	
	select {
	case res := <-done:
		return res.result, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processPDFWithContext wraps processPDF with context support
func processPDFWithContext(ctx context.Context, filePath string, content []byte, optionsStr string) (*PDFContentResult, error) {
	done := make(chan struct {
		result *PDFContentResult
		err    error
	}, 1)
	
	go func() {
		result, err := processPDF(filePath, content, optionsStr)
		done <- struct {
			result *PDFContentResult
			err    error
		}{result, err}
	}()
	
	select {
	case res := <-done:
		return res.result, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processNotebookWithContext wraps processNotebook with context support
func processNotebookWithContext(ctx context.Context, filePath string, content []byte, optionsStr string) (*NotebookContentResult, error) {
	done := make(chan struct {
		result *NotebookContentResult
		err    error
	}, 1)
	
	go func() {
		result, err := processNotebook(filePath, content, optionsStr)
		done <- struct {
			result *NotebookContentResult
			err    error
		}{result, err}
	}()
	
	select {
	case res := <-done:
		return res.result, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}