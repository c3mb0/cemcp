package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestImageProcessing tests image analysis functionality
func TestImageProcessing(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	
	// Test PNG processing (mock)
	t.Run("PNG Processing", func(t *testing.T) {
		// Create a simple PNG test file content (mock)
		pngContent := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		}
		pngFile := filepath.Join(tempDir, "test.png")
		if err := os.WriteFile(pngFile, pngContent, 0644); err != nil {
			t.Fatal(err)
		}
		
		// Test PNG detection
		mimeType := detectMultimodalMIME(pngFile, pngContent)
		if mimeType != "image/png" {
			t.Errorf("Expected image/png, got %s", mimeType)
		}
		
		// Test file type detection
		fileType, isMultimodal := isMultimodalFile(mimeType, pngFile)
		if !isMultimodal {
			t.Error("PNG should be detected as multimodal")
		}
		if fileType != "image" {
			t.Errorf("Expected image, got %s", fileType)
		}
	})
	
	// Test JPEG processing
	t.Run("JPEG Processing", func(t *testing.T) {
		jpegContent := []byte{0xFF, 0xD8, 0xFF, 0xE0}
		jpegFile := filepath.Join(tempDir, "test.jpg")
		if err := os.WriteFile(jpegFile, jpegContent, 0644); err != nil {
			t.Fatal(err)
		}
		
		mimeType := detectMultimodalMIME(jpegFile, jpegContent)
		if mimeType != "image/jpeg" {
			t.Errorf("Expected image/jpeg, got %s", mimeType)
		}
	})
	
	// Test SVG processing
	t.Run("SVG Processing", func(t *testing.T) {
		svgContent := []byte(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
			<circle cx="50" cy="50" r="40" stroke="black" fill="red" />
			<text x="50" y="55" text-anchor="middle">Test</text>
		</svg>`)
		svgFile := filepath.Join(tempDir, "test.svg")
		if err := os.WriteFile(svgFile, svgContent, 0644); err != nil {
			t.Fatal(err)
		}
		
		mimeType := detectMultimodalMIME(svgFile, svgContent)
		if mimeType != "image/svg+xml" {
			t.Errorf("Expected image/svg+xml, got %s", mimeType)
		}
		
		// Test SVG processing
		options := ImageProcessingOptions{
			EnableDescription: true,
		}
		result, err := processSVG(svgFile, svgContent, options)
		if err != nil {
			t.Fatalf("SVG processing failed: %v", err)
		}
		
		if result.Width != 100 || result.Height != 100 {
			t.Errorf("Expected 100x100, got %dx%d", result.Width, result.Height)
		}
		
		if result.OCRText != "Test" {
			t.Errorf("Expected 'Test', got '%s'", result.OCRText)
		}
		
		if !strings.Contains(result.Description, "SVG vector image") {
			t.Errorf("Description should mention SVG: %s", result.Description)
		}
	})
}

// TestPDFProcessing tests PDF analysis functionality
func TestPDFProcessing(t *testing.T) {
	t.Run("PDF Options Parsing", func(t *testing.T) {
		optionsJSON := `{"max_pages": 5, "extract_text": true, "extract_metadata": true}`
		
		// Test that we can parse the options without error
		// (This would normally be done inside processPDF)
		var options PDFProcessingOptions
		err := json.Unmarshal([]byte(optionsJSON), &options)
		if err != nil {
			t.Fatalf("Failed to parse PDF options: %v", err)
		}
		
		if options.MaxPages != 5 {
			t.Errorf("Expected MaxPages=5, got %d", options.MaxPages)
		}
		if !options.ExtractText {
			t.Error("Expected ExtractText=true")
		}
		if !options.ExtractMetadata {
			t.Error("Expected ExtractMetadata=true")
		}
	})
	
	t.Run("PDF Visual Element Detection", func(t *testing.T) {
		// Test table detection
		tableText := `Name    Age    City
		John    25     NYC
		Jane    30     LA
		Bob     35     Chicago`
		
		if !detectTablePatterns(tableText) {
			t.Error("Should detect table patterns")
		}
		
		// Test list detection
		listText := `1. First item
		2. Second item
		3. Third item`
		
		if !detectListPatterns(listText) {
			t.Error("Should detect list patterns")
		}
		
		// Test bullet list detection
		bulletText := `• Item one
		• Item two
		• Item three`
		
		if !detectListPatterns(bulletText) {
			t.Error("Should detect bullet list patterns")
		}
	})
}

// TestNotebookProcessing tests Jupyter notebook functionality
func TestNotebookProcessing(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a test notebook
	notebook := map[string]interface{}{
		"cells": []map[string]interface{}{
			{
				"cell_type": "markdown",
				"metadata": map[string]interface{}{},
				"source":   []string{"# Test Notebook\n", "This is a test markdown cell."},
			},
			{
				"cell_type":       "code",
				"execution_count": 1,
				"metadata":        map[string]interface{}{},
				"outputs":         []interface{}{},
				"source":          []string{"print('Hello, world!')"},
			},
		},
		"metadata": map[string]interface{}{
			"kernelspec": map[string]interface{}{
				"display_name": "Python 3",
				"language":     "python",
				"name":         "python3",
			},
		},
		"nbformat":       4,
		"nbformat_minor": 4,
	}
	
	notebookContent, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	
	notebookFile := filepath.Join(tempDir, "test.ipynb")
	if err := os.WriteFile(notebookFile, notebookContent, 0644); err != nil {
		t.Fatal(err)
	}
	
	t.Run("Notebook Detection", func(t *testing.T) {
		fileType, isMultimodal := isMultimodalFile("application/json", notebookFile)
		if !isMultimodal {
			t.Error("Notebook should be detected as multimodal")
		}
		if fileType != "notebook" {
			t.Errorf("Expected notebook, got %s", fileType)
		}
	})
	
	t.Run("Notebook Processing", func(t *testing.T) {
		options := NotebookProcessingOptions{
			IncludeOutputs: true,
			ValidateFormat: true,
		}
		
		optionsJSON, _ := json.Marshal(options)
		result, err := processNotebook(notebookFile, notebookContent, string(optionsJSON))
		if err != nil {
			t.Fatalf("Notebook processing failed: %v", err)
		}
		
		if result.CellCount != 2 {
			t.Errorf("Expected 2 cells, got %d", result.CellCount)
		}
		
		if result.CodeCells != 1 {
			t.Errorf("Expected 1 code cell, got %d", result.CodeCells)
		}
		
		if result.MarkdownCells != 1 {
			t.Errorf("Expected 1 markdown cell, got %d", result.MarkdownCells)
		}
	})
	
	t.Run("Notebook Cell Editing", func(t *testing.T) {
		// Test cell editing
		args := NotebookEditArgs{
			Path:       notebookFile,
			CellIndex:  0,
			NewSource:  "# Modified Title\nThis is modified content.",
			CellType:   "markdown",
			InsertMode: false,
		}
		
		result, err := handleNotebookEdit(notebookFile, args)
		if err != nil {
			t.Fatalf("Cell editing failed: %v", err)
		}
		
		if !result.Success {
			t.Errorf("Cell editing should succeed: %s", result.Message)
		}
		
		if result.Action != "edit" {
			t.Errorf("Expected edit action, got %s", result.Action)
		}
		
		// Test cell insertion
		insertArgs := NotebookEditArgs{
			Path:       notebookFile,
			CellIndex:  1,
			NewSource:  "# Inserted Cell\nThis is an inserted cell.",
			CellType:   "markdown",
			InsertMode: true,
		}
		
		insertResult, err := handleNotebookEdit(notebookFile, insertArgs)
		if err != nil {
			t.Fatalf("Cell insertion failed: %v", err)
		}
		
		if !insertResult.Success {
			t.Errorf("Cell insertion should succeed: %s", insertResult.Message)
		}
		
		if insertResult.Action != "insert" {
			t.Errorf("Expected insert action, got %s", insertResult.Action)
		}
	})
}

// TestFileSizeValidation tests file size limits
func TestFileSizeValidation(t *testing.T) {
	t.Run("Large File Detection", func(t *testing.T) {
		// Test the maxFileSize constant (would be used in actual file processing)
		const maxFileSize = 512 * 1024 * 1024 // 512MB
		
		// Simulate a large file size
		largeFileSize := int64(600 * 1024 * 1024) // 600MB
		
		if largeFileSize <= maxFileSize {
			t.Error("Large file should exceed size limit")
		}
		
		// Simulate a normal file size
		normalFileSize := int64(10 * 1024 * 1024) // 10MB
		
		if normalFileSize > maxFileSize {
			t.Error("Normal file should not exceed size limit")
		}
	})
}

// TestProcessingTimeout tests timeout handling
func TestProcessingTimeout(t *testing.T) {
	t.Run("Timeout Context", func(t *testing.T) {
		// Test that we can create a timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		// Simulate a quick operation that should complete
		done := make(chan bool, 1)
		go func() {
			time.Sleep(100 * time.Millisecond)
			done <- true
		}()
		
		select {
		case <-done:
			// Operation completed successfully
		case <-ctx.Done():
			t.Error("Operation should not have timed out")
		}
	})
}

// TestMIMEDetection tests MIME type detection for multimodal files
func TestMIMEDetection(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		filename string
		expected string
	}{
		{
			name:     "PNG",
			content:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			filename: "test.png",
			expected: "image/png",
		},
		{
			name:     "JPEG",
			content:  []byte{0xFF, 0xD8, 0xFF, 0xE0},
			filename: "test.jpg",
			expected: "image/jpeg",
		},
		{
			name:     "GIF",
			content:  []byte("GIF89a"),
			filename: "test.gif",
			expected: "image/gif",
		},
		{
			name:     "WebP",
			content:  []byte("RIFF____WEBP"),
			filename: "test.webp",
			expected: "image/webp",
		},
		{
			name:     "SVG",
			content:  []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"),
			filename: "test.svg",
			expected: "image/svg+xml",
		},
		{
			name:     "Notebook",
			content:  []byte(`{"cells":[],"metadata":{},"nbformat":4}`),
			filename: "test.ipynb",
			expected: "application/x-ipynb+json",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMultimodalMIME(tt.filename, tt.content)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestStringConversions tests utility functions
func TestStringConversions(t *testing.T) {
	t.Run("String Slice to Interface", func(t *testing.T) {
		input := []string{"line1", "line2", "line3"}
		result := convertStringSliceToInterface(input)
		
		if len(result) != len(input) {
			t.Errorf("Expected %d items, got %d", len(input), len(result))
		}
		
		// Check that lines end with \n except the last one
		for i, item := range result {
			str, ok := item.(string)
			if !ok {
				t.Errorf("Item %d should be a string", i)
				continue
			}
			
			if i < len(result)-1 {
				if !strings.HasSuffix(str, "\n") {
					t.Errorf("Item %d should end with \\n: %q", i, str)
				}
			} else {
				if strings.HasSuffix(str, "\n") {
					t.Errorf("Last item should not end with \\n: %q", str)
				}
			}
		}
	})
	
	t.Run("Interface to String Slice", func(t *testing.T) {
		// Test string input
		stringInput := "line1\nline2\nline3"
		result := convertToStringSlice(stringInput)
		
		expected := []string{"line1", "line2", "line3"}
		if len(result) != len(expected) {
			t.Errorf("Expected %d lines, got %d", len(expected), len(result))
		}
		
		for i, line := range result {
			if i < len(expected) && line != expected[i] {
				t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
			}
		}
		
		// Test interface slice input
		interfaceInput := []interface{}{"line1", "line2", "line3"}
		result2 := convertToStringSlice(interfaceInput)
		
		if len(result2) != len(expected) {
			t.Errorf("Expected %d lines, got %d", len(expected), len(result2))
		}
		
		for i, line := range result2 {
			if i < len(expected) && line != expected[i] {
				t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
			}
		}
	})
}