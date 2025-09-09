package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// NotebookProcessingOptions contains options for notebook processing
type NotebookProcessingOptions struct {
	MaxCells       int    `json:"max_cells"`        // Maximum cells to process (0 = no limit)
	IncludeOutputs bool   `json:"include_outputs"`  // Whether to include cell outputs
	FilterCellType string `json:"filter_cell_type"` // Filter by cell type: "code", "markdown", or "" for all
	ValidateFormat bool   `json:"validate_format"`  // Whether to validate notebook format
}

// RawNotebook represents the raw JSON structure of a Jupyter notebook
type RawNotebook struct {
	Cells    []map[string]interface{} `json:"cells"`
	Metadata map[string]interface{}   `json:"metadata"`
	NbFormat int                      `json:"nbformat"`
	NbFormatMinor int                 `json:"nbformat_minor"`
}

// processNotebook handles Jupyter notebook parsing and validation
func processNotebook(filePath string, content []byte, optionsStr string) (*NotebookContentResult, error) {
	// Parse options
	var options NotebookProcessingOptions
	if optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			// Use default options if parsing fails
			options = NotebookProcessingOptions{
				MaxCells:       0,    // No limit
				IncludeOutputs: true,
				FilterCellType: "",   // All cell types
				ValidateFormat: true,
			}
		}
	} else {
		// Default options
		options = NotebookProcessingOptions{
			MaxCells:       0,
			IncludeOutputs: true,
			FilterCellType: "",
			ValidateFormat: true,
		}
	}

	result := &NotebookContentResult{
		FileSize: int64(len(content)),
		Cells:    make([]NotebookCell, 0),
		Metadata: make(map[string]interface{}),
	}

	// Parse the notebook JSON
	var rawNotebook RawNotebook
	if err := json.Unmarshal(content, &rawNotebook); err != nil {
		return nil, fmt.Errorf("failed to parse notebook JSON: %v", err)
	}

	// Validate notebook format if requested
	if options.ValidateFormat {
		if err := validateNotebookFormat(&rawNotebook); err != nil {
			return nil, fmt.Errorf("invalid notebook format: %v", err)
		}
	}

	// Store notebook metadata
	result.Metadata = rawNotebook.Metadata
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}

	// Add format information
	result.Metadata["nbformat"] = rawNotebook.NbFormat
	result.Metadata["nbformat_minor"] = rawNotebook.NbFormatMinor

	// Extract kernel information if available
	if kernelInfo, ok := result.Metadata["kernelspec"]; ok {
		result.KernelInfo = kernelInfo.(map[string]interface{})
	}

	// Count cells by type
	codeCells := 0
	markdownCells := 0
	totalCells := len(rawNotebook.Cells)
	
	// Determine how many cells to process
	maxCells := totalCells
	if options.MaxCells > 0 && options.MaxCells < totalCells {
		maxCells = options.MaxCells
	}

	// Process cells
	for i := 0; i < maxCells; i++ {
		rawCell := rawNotebook.Cells[i]
		
		// Extract cell type
		cellType, ok := rawCell["cell_type"].(string)
		if !ok {
			continue // Skip cells without type
		}
		
		// Apply cell type filter if specified
		if options.FilterCellType != "" && cellType != options.FilterCellType {
			continue
		}
		
		// Count cells by type
		switch cellType {
		case "code":
			codeCells++
		case "markdown":
			markdownCells++
		}
		
		// Convert raw cell to structured cell
		cell, err := parseNotebookCell(rawCell, options.IncludeOutputs)
		if err != nil {
			dprintf("Failed to parse cell %d: %v", i, err)
			continue
		}
		
		result.Cells = append(result.Cells, *cell)
	}

	// Set count information
	result.CellCount = len(result.Cells)
	result.CodeCells = codeCells
	result.MarkdownCells = markdownCells

	return result, nil
}

// validateNotebookFormat validates that the notebook follows Jupyter format standards
func validateNotebookFormat(notebook *RawNotebook) error {
	// Check required fields
	if notebook.NbFormat == 0 {
		return fmt.Errorf("missing or invalid nbformat field")
	}
	
	if notebook.NbFormat < 3 || notebook.NbFormat > 4 {
		return fmt.Errorf("unsupported notebook format version: %d", notebook.NbFormat)
	}
	
	if notebook.Cells == nil {
		return fmt.Errorf("missing cells array")
	}
	
	// Validate each cell has required fields
	for i, cell := range notebook.Cells {
		cellType, ok := cell["cell_type"].(string)
		if !ok {
			return fmt.Errorf("cell %d missing cell_type field", i)
		}
		
		if cellType != "code" && cellType != "markdown" && cellType != "raw" {
			return fmt.Errorf("cell %d has invalid cell_type: %s", i, cellType)
		}
		
		// Check for source field
		if _, ok := cell["source"]; !ok {
			return fmt.Errorf("cell %d missing source field", i)
		}
	}
	
	return nil
}

// parseNotebookCell converts a raw cell map to a structured NotebookCell
func parseNotebookCell(rawCell map[string]interface{}, includeOutputs bool) (*NotebookCell, error) {
	cell := &NotebookCell{}
	
	// Extract cell type
	if cellType, ok := rawCell["cell_type"].(string); ok {
		cell.CellType = cellType
	} else {
		return nil, fmt.Errorf("missing cell_type")
	}
	
	// Extract source
	if sourceData, ok := rawCell["source"]; ok {
		cell.Source = convertToStringSlice(sourceData)
	}
	
	// Extract metadata
	if metadata, ok := rawCell["metadata"]; ok {
		if metadataMap, ok := metadata.(map[string]interface{}); ok {
			cell.Metadata = metadataMap
		}
	}
	
	// For code cells, extract execution count and outputs
	if cell.CellType == "code" {
		if execCount, ok := rawCell["execution_count"]; ok {
			if execCountInt, ok := execCount.(float64); ok {
				count := int(execCountInt)
				cell.ExecutionCount = &count
			}
		}
		
		if includeOutputs {
			if outputs, ok := rawCell["outputs"]; ok {
				if outputsSlice, ok := outputs.([]interface{}); ok {
					cell.Outputs = outputsSlice
				}
			}
		}
	}
	
	return cell, nil
}

// convertToStringSlice converts various source formats to []string
func convertToStringSlice(source interface{}) []string {
	switch v := source.(type) {
	case string:
		// Single string - split by lines
		return strings.Split(v, "\n")
	case []interface{}:
		// Array of strings
		var result []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	case []string:
		// Already string slice
		return v
	default:
		// Convert to string
		return []string{fmt.Sprintf("%v", v)}
	}
}

// Additional types and functions for notebook editing support

// NotebookEditArgs defines parameters for editing notebook cells
type NotebookEditArgs struct {
	Path       string `json:"path" description:"Notebook file path"`
	CellIndex  int    `json:"cell_index" description:"Index of cell to edit (0-based)"`
	NewSource  string `json:"new_source" description:"New source content for the cell"`
	CellType   string `json:"cell_type,omitempty" description:"Cell type (code/markdown)"`
	InsertMode bool   `json:"insert_mode,omitempty" description:"Insert new cell instead of replacing"`
}

// NotebookEditResult contains results from notebook cell editing
type NotebookEditResult struct {
	Path      string `json:"path" description:"Notebook file path"`
	CellIndex int    `json:"cell_index" description:"Index of edited cell"`
	Action    string `json:"action" description:"Action performed (edit/insert/delete)"`
	Success   bool   `json:"success" description:"Whether operation succeeded"`
	Message   string `json:"message,omitempty" description:"Success or error message"`
}

// handleNotebookEdit processes notebook cell editing operations
func handleNotebookEdit(filePath string, args NotebookEditArgs) (*NotebookEditResult, error) {
	result := &NotebookEditResult{
		Path:      args.Path,
		CellIndex: args.CellIndex,
	}
	
	// Read the notebook file
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to read notebook file: %v", err)
		return result, err
	}
	
	// Parse the notebook JSON
	var notebook RawNotebook
	if err := json.Unmarshal(content, &notebook); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to parse notebook JSON: %v", err)
		return result, err
	}
	
	// Validate notebook format
	if err := validateNotebookFormat(&notebook); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Invalid notebook format: %v", err)
		return result, err
	}
	
	// Perform the editing operation
	if args.InsertMode {
		result.Action = "insert"
		err = insertNotebookCell(&notebook, args)
	} else {
		result.Action = "edit"
		err = editNotebookCell(&notebook, args)
	}
	
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result, err
	}
	
	// Write the modified notebook back to file
	modifiedContent, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to serialize notebook: %v", err)
		return result, err
	}
	
	if err := os.WriteFile(filePath, modifiedContent, 0644); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Failed to write notebook file: %v", err)
		return result, err
	}
	
	result.Success = true
	result.Message = fmt.Sprintf("Successfully %sed cell %d", result.Action, args.CellIndex)
	
	return result, nil
}

// editNotebookCell modifies an existing cell in the notebook
func editNotebookCell(notebook *RawNotebook, args NotebookEditArgs) error {
	if args.CellIndex < 0 || args.CellIndex >= len(notebook.Cells) {
		return fmt.Errorf("cell index %d out of range (0-%d)", args.CellIndex, len(notebook.Cells)-1)
	}
	
	cell := notebook.Cells[args.CellIndex]
	
	// Update cell type if specified
	if args.CellType != "" {
		if args.CellType != "code" && args.CellType != "markdown" && args.CellType != "raw" {
			return fmt.Errorf("invalid cell type: %s", args.CellType)
		}
		cell["cell_type"] = args.CellType
	}
	
	// Update source content
	sourceLines := strings.Split(args.NewSource, "\n")
	cell["source"] = convertStringSliceToInterface(sourceLines)
	
	// If converting to code cell, ensure required fields exist
	if cellType, ok := cell["cell_type"].(string); ok && cellType == "code" {
		if _, exists := cell["execution_count"]; !exists {
			cell["execution_count"] = nil
		}
		if _, exists := cell["outputs"]; !exists {
			cell["outputs"] = []interface{}{}
		}
	}
	
	return nil
}

// insertNotebookCell inserts a new cell at the specified index
func insertNotebookCell(notebook *RawNotebook, args NotebookEditArgs) error {
	if args.CellIndex < 0 || args.CellIndex > len(notebook.Cells) {
		return fmt.Errorf("insert index %d out of range (0-%d)", args.CellIndex, len(notebook.Cells))
	}
	
	// Default cell type
	cellType := args.CellType
	if cellType == "" {
		cellType = "code"
	}
	
	if cellType != "code" && cellType != "markdown" && cellType != "raw" {
		return fmt.Errorf("invalid cell type: %s", cellType)
	}
	
	// Create new cell
	sourceLines := strings.Split(args.NewSource, "\n")
	newCell := map[string]interface{}{
		"cell_type": cellType,
		"metadata": map[string]interface{}{},
		"source":   convertStringSliceToInterface(sourceLines),
	}
	
	// Add code cell specific fields
	if cellType == "code" {
		newCell["execution_count"] = nil
		newCell["outputs"] = []interface{}{}
	}
	
	// Insert the cell at the specified index
	newCells := make([]map[string]interface{}, len(notebook.Cells)+1)
	copy(newCells[:args.CellIndex], notebook.Cells[:args.CellIndex])
	newCells[args.CellIndex] = newCell
	copy(newCells[args.CellIndex+1:], notebook.Cells[args.CellIndex:])
	
	notebook.Cells = newCells
	
	return nil
}

// convertStringSliceToInterface converts []string to []interface{} for JSON serialization
func convertStringSliceToInterface(lines []string) []interface{} {
	result := make([]interface{}, len(lines))
	for i, line := range lines {
		result[i] = line + "\n" // Jupyter notebooks typically end each line with \n
	}
	// Remove trailing \n from last line to match Jupyter format
	if len(result) > 0 {
		if lastLine, ok := result[len(result)-1].(string); ok {
			result[len(result)-1] = strings.TrimSuffix(lastLine, "\n")
		}
	}
	return result
}