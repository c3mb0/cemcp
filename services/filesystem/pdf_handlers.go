package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFProcessingOptions contains options for PDF processing
type PDFProcessingOptions struct {
	MaxPages       int  `json:"max_pages"`        // Maximum pages to process (0 = no limit)
	ExtractText    bool `json:"extract_text"`     // Whether to extract text content
	ExtractMetadata bool `json:"extract_metadata"` // Whether to extract PDF metadata
	PageRange      struct {
		Start int `json:"start"` // Starting page (1-indexed, 0 = from beginning)
		End   int `json:"end"`   // Ending page (1-indexed, 0 = to end)
	} `json:"page_range"`
}

// processPDF handles PDF parsing, text extraction, and metadata analysis
func processPDF(filePath string, content []byte, optionsStr string) (*PDFContentResult, error) {
	// Parse options
	var options PDFProcessingOptions
	if optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			// Use default options if parsing fails
			options = PDFProcessingOptions{
				MaxPages:        0, // No limit
				ExtractText:     true,
				ExtractMetadata: true,
			}
		}
	} else {
		// Default options
		options = PDFProcessingOptions{
			MaxPages:        0,
			ExtractText:     true,
			ExtractMetadata: true,
		}
	}

	result := &PDFContentResult{
		FileSize: int64(len(content)),
		Metadata: make(map[string]string),
		Pages:    make([]PDFPageContent, 0),
	}

	// Open PDF from bytes
	reader := bytes.NewReader(content)
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %v", err)
	}

	result.PageCount = pdfReader.NumPage()

	// Extract metadata if requested
	if options.ExtractMetadata {
		extractPDFMetadata(pdfReader, result)
	}

	// Determine page range to process
	startPage := 1
	endPage := result.PageCount
	
	if options.PageRange.Start > 0 {
		startPage = options.PageRange.Start
	}
	if options.PageRange.End > 0 && options.PageRange.End < endPage {
		endPage = options.PageRange.End
	}
	if options.MaxPages > 0 && (endPage-startPage+1) > options.MaxPages {
		endPage = startPage + options.MaxPages - 1
	}
	
	// Ensure valid range
	if startPage > result.PageCount {
		startPage = result.PageCount
	}
	if endPage > result.PageCount {
		endPage = result.PageCount
	}
	if startPage < 1 {
		startPage = 1
	}

	var fullTextContent strings.Builder
	
	// Extract text from each page
	if options.ExtractText {
		for pageNum := startPage; pageNum <= endPage; pageNum++ {
			page := pdfReader.Page(pageNum)
			if page.V.IsNull() {
				continue
			}
			
			text, err := extractPageText(page)
			if err != nil {
				dprintf("Failed to extract text from page %d: %v", pageNum, err)
				// Continue with other pages even if one fails
				text = fmt.Sprintf("[Error extracting text from page %d: %v]", pageNum, err)
			}
			
			// Enhanced page analysis
			hasImages := checkPageHasImages(page)
			visualElements := analyzePageVisualElements(page, text)
			wordCount := len(strings.Fields(text))
			
			pageContent := PDFPageContent{
				PageNumber:     pageNum,
				Text:           text,
				HasImages:      hasImages,
				VisualElements: visualElements,
				WordCount:      wordCount,
			}
			
			result.Pages = append(result.Pages, pageContent)
			fullTextContent.WriteString(text)
			fullTextContent.WriteString("\n\n--- Page Break ---\n\n")
			
			// Check if page has images
			if pageContent.HasImages {
				result.HasImages = true
			}
		}
	}
	
	result.TextContent = fullTextContent.String()
	
	// Add processing metadata
	result.Metadata["pages_processed"] = fmt.Sprintf("%d-%d", startPage, endPage)
	result.Metadata["text_length"] = fmt.Sprintf("%d", len(result.TextContent))
	result.Metadata["has_images"] = fmt.Sprintf("%v", result.HasImages)

	return result, nil
}

// extractPDFMetadata extracts metadata from the PDF document
func extractPDFMetadata(reader *pdf.Reader, result *PDFContentResult) {
	// The pdf library has limited metadata extraction capabilities
	// We extract what we can and provide basic document information
	
	result.Metadata["total_pages"] = fmt.Sprintf("%d", result.PageCount)
	result.Metadata["pdf_version"] = "1.4" // Default, actual version detection is complex
	
	// Try to extract document info if available
	// Note: This is basic implementation - full metadata extraction would require
	// more sophisticated PDF parsing
	result.Metadata["producer"] = "Unknown"
	result.Metadata["creator"] = "Unknown"
	result.Metadata["title"] = "Untitled"
	result.Metadata["subject"] = ""
	result.Metadata["author"] = "Unknown"
	
	// Estimate document complexity
	if result.PageCount == 1 {
		result.Metadata["document_type"] = "single-page"
	} else if result.PageCount <= 10 {
		result.Metadata["document_type"] = "short-document"
	} else if result.PageCount <= 50 {
		result.Metadata["document_type"] = "medium-document"
	} else {
		result.Metadata["document_type"] = "long-document"
	}
}

// extractPageText extracts text content from a PDF page
func extractPageText(page pdf.Page) (string, error) {
	// Get text content from the page
	textContent := page.Content()
	
	var result strings.Builder
	
	// The pdf library provides raw content - we need to extract text
	// This is a simplified text extraction that looks for text objects
	for _, text := range textContent.Text {
		if len(text.S) > 0 {
			result.WriteString(text.S)
			result.WriteString(" ")
		}
	}
	
	// Clean up the extracted text
	extractedText := strings.TrimSpace(result.String())
	
	// Basic cleanup of extracted text
	extractedText = strings.ReplaceAll(extractedText, "  ", " ")
	extractedText = strings.ReplaceAll(extractedText, "\t", " ")
	
	// Split into lines and rejoin to normalize spacing
	lines := strings.Split(extractedText, "\n")
	var cleanLines []string
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if cleaned != "" {
			cleanLines = append(cleanLines, cleaned)
		}
	}
	
	return strings.Join(cleanLines, "\n"), nil
}

// checkPageHasImages checks if a PDF page contains images
func checkPageHasImages(page pdf.Page) bool {
	content := page.Content()
	
	// Check for image-related keywords in text content
	for _, obj := range content.Text {
		text := strings.ToLower(obj.S)
		if strings.Contains(text, "image") || 
		   strings.Contains(text, "figure") || 
		   strings.Contains(text, "photo") ||
		   strings.Contains(text, "img") ||
		   strings.Contains(text, "jpeg") ||
		   strings.Contains(text, "png") {
			return true
		}
	}
	
	// Look for drawing operations that might indicate graphics
	// This is a heuristic approach - proper image detection requires deeper PDF parsing
	// TODO: Need to determine correct API for pdf.Content
	hasDrawingOps := false
	
	return hasDrawingOps
}

// analyzePageVisualElements analyzes visual elements present on a PDF page
func analyzePageVisualElements(page pdf.Page, textContent string) []string {
	var elements []string
	_ = page.Content() // TODO: Need to determine correct API for pdf.Content
	contentStr := ""
	
	// Detect text presence
	if strings.TrimSpace(textContent) != "" {
		elements = append(elements, "text")
	}
	
	// Detect potential images
	if checkPageHasImages(page) {
		elements = append(elements, "images")
	}
	
	// Detect tables (heuristic based on text patterns)
	if detectTablePatterns(textContent) {
		elements = append(elements, "tables")
	}
	
	// Detect lists (heuristic based on text patterns)  
	if detectListPatterns(textContent) {
		elements = append(elements, "lists")
	}
	
	// Detect drawing operations (lines, shapes)
	if strings.Contains(contentStr, "line") || 
	   strings.Contains(contentStr, "rect") ||
	   strings.Contains(contentStr, "path") ||
	   strings.Contains(contentStr, "fill") ||
	   strings.Contains(contentStr, "stroke") {
		elements = append(elements, "graphics")
	}
	
	// Detect forms
	if strings.Contains(contentStr, "widget") ||
	   strings.Contains(contentStr, "field") ||
	   strings.Contains(contentStr, "form") {
		elements = append(elements, "forms")
	}
	
	return elements
}

// detectTablePatterns detects potential table structures in text
func detectTablePatterns(text string) bool {
	lines := strings.Split(text, "\n")
	
	// Look for consistent column alignment patterns
	tabCount := 0
	spacePatternCount := 0
	
	for _, line := range lines {
		// Count tabs
		if strings.Count(line, "\t") >= 2 {
			tabCount++
		}
		
		// Look for consistent spacing patterns (multiple spaces)
		if regexp.MustCompile(`\s{3,}`).MatchString(line) {
			spacePatternCount++
		}
	}
	
	// If significant number of lines have tabular patterns
	totalLines := len(lines)
	return (tabCount > totalLines/4) || (spacePatternCount > totalLines/3)
}

// detectListPatterns detects potential list structures in text
func detectListPatterns(text string) bool {
	lines := strings.Split(text, "\n")
	bulletCount := 0
	numberCount := 0
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		
		// Detect bullet points
		if strings.HasPrefix(trimmed, "•") || 
		   strings.HasPrefix(trimmed, "-") ||
		   strings.HasPrefix(trimmed, "*") ||
		   strings.HasPrefix(trimmed, "◦") {
			bulletCount++
		}
		
		// Detect numbered lists
		if regexp.MustCompile(`^\d+\.\s`).MatchString(trimmed) {
			numberCount++
		}
	}
	
	// If significant number of lines are list items
	return bulletCount >= 3 || numberCount >= 3
}