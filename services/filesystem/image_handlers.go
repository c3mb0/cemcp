package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	"github.com/otiai10/gosseract/v2"
	"golang.org/x/image/webp"
)

// ImageProcessingOptions contains options for image processing
type ImageProcessingOptions struct {
	EnableOCR         bool `json:"enable_ocr"`
	EnableDescription bool `json:"enable_description"`
	MaxOCRImageSize   int  `json:"max_ocr_image_size"` // Max pixels for OCR processing
}

// processImage handles image analysis, OCR, and metadata extraction
func processImage(filePath string, content []byte, optionsStr string) (*ImageAnalysisResult, error) {
	// Parse options
	var options ImageProcessingOptions
	if optionsStr != "" {
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
			// Use default options if parsing fails
			options = ImageProcessingOptions{
				EnableOCR:         true,
				EnableDescription: true,
				MaxOCRImageSize:   2048 * 2048, // 4MP max for OCR
			}
		}
	} else {
		// Default options
		options = ImageProcessingOptions{
			EnableOCR:         true,
			EnableDescription: true,
			MaxOCRImageSize:   2048 * 2048,
		}
	}

	result := &ImageAnalysisResult{
		FileSize: int64(len(content)),
		Metadata: make(map[string]string),
	}

	// Decode image to get dimensions and format
	var img image.Image
	var format string
	var err error

	// Handle different image formats
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".webp":
		img, err = webp.Decode(bytes.NewReader(content))
		format = "WebP"
	case ".svg":
		// SVG is handled differently as it's vector-based
		return processSVG(filePath, content, options)
	default:
		img, format, err = image.Decode(bytes.NewReader(content))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %v", err)
	}

	bounds := img.Bounds()
	result.Width = bounds.Dx()
	result.Height = bounds.Dy()
	result.Format = format

	// Add basic metadata
	result.Metadata["width"] = fmt.Sprintf("%d", result.Width)
	result.Metadata["height"] = fmt.Sprintf("%d", result.Height)
	result.Metadata["format"] = format
	result.Metadata["size_bytes"] = fmt.Sprintf("%d", result.FileSize)
	result.Metadata["aspect_ratio"] = fmt.Sprintf("%.3f", float64(result.Width)/float64(result.Height))
	
	// Extract EXIF metadata for JPEG images
	if strings.ToLower(format) == "jpeg" || ext == ".jpg" || ext == ".jpeg" {
		extractEXIFMetadata(content, result)
	}

	// Generate visual description
	if options.EnableDescription {
		result.Description = generateImageDescription(img, format, result.Width, result.Height)
	}

	// Perform OCR if enabled and image is suitable
	if options.EnableOCR {
		pixels := result.Width * result.Height
		if pixels <= options.MaxOCRImageSize {
			ocrText, err := performOCR(img, filePath)
			if err != nil {
				// OCR failure is not fatal, just log it
				dprintf("OCR failed for %s: %v", filePath, err)
				result.Metadata["ocr_error"] = err.Error()
			} else {
				result.OCRText = strings.TrimSpace(ocrText)
				result.Metadata["ocr_text_length"] = fmt.Sprintf("%d", len(result.OCRText))
			}
		} else {
			result.Metadata["ocr_skipped"] = "image too large for OCR processing"
		}
	}

	return result, nil
}

// generateImageDescription creates a basic description of the image
func generateImageDescription(img image.Image, format string, width, height int) string {
	description := fmt.Sprintf("A %s image with dimensions %dx%d pixels", format, width, height)

	// Add size category
	pixels := width * height
	if pixels < 100*100 {
		description += " (thumbnail size)"
	} else if pixels < 800*600 {
		description += " (small size)"
	} else if pixels < 1920*1080 {
		description += " (medium size)"
	} else if pixels < 3840*2160 {
		description += " (large size)"
	} else {
		description += " (very large/high resolution)"
	}

	// Add aspect ratio description
	aspectRatio := float64(width) / float64(height)
	if aspectRatio > 2.0 {
		description += ", wide panoramic aspect ratio"
	} else if aspectRatio > 1.5 {
		description += ", wide aspect ratio"
	} else if aspectRatio > 1.2 {
		description += ", landscape orientation"
	} else if aspectRatio > 0.8 {
		description += ", square-like aspect ratio"
	} else if aspectRatio > 0.6 {
		description += ", portrait orientation"
	} else {
		description += ", tall narrow aspect ratio"
	}

	// Analyze basic color properties
	colorDesc := analyzeImageColors(img)
	if colorDesc != "" {
		description += ". " + colorDesc
	}

	return description
}

// analyzeImageColors performs basic color analysis on a sample of the image
func analyzeImageColors(img image.Image) string {
	bounds := img.Bounds()
	
	// Sample pixels to analyze colors (take every 10th pixel to avoid too much computation)
	var rSum, gSum, bSum, aSum int64
	var pixelCount int64
	
	step := 10
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			// Convert from 16-bit to 8-bit
			rSum += int64(r >> 8)
			gSum += int64(g >> 8)
			bSum += int64(b >> 8)
			aSum += int64(a >> 8)
			pixelCount++
		}
	}
	
	if pixelCount == 0 {
		return ""
	}
	
	// Calculate average colors
	avgR := rSum / pixelCount
	avgG := gSum / pixelCount
	avgB := bSum / pixelCount
	avgA := aSum / pixelCount
	
	var colorDesc []string
	
	// Determine dominant color characteristics
	maxColor := maxInt64(avgR, maxInt64(avgG, avgB))
	minColor := minInt64(avgR, minInt64(avgG, avgB))
	
	// Check for transparency
	if avgA < 200 {
		colorDesc = append(colorDesc, "contains transparency")
	}
	
	// Determine if it's grayscale
	if maxColor-minColor < 30 {
		if maxColor < 80 {
			colorDesc = append(colorDesc, "predominantly dark/black tones")
		} else if maxColor > 180 {
			colorDesc = append(colorDesc, "predominantly light/white tones")
		} else {
			colorDesc = append(colorDesc, "predominantly grayscale")
		}
	} else {
		// Determine dominant color
		if avgR > avgG+30 && avgR > avgB+30 {
			colorDesc = append(colorDesc, "reddish tones")
		} else if avgG > avgR+30 && avgG > avgB+30 {
			colorDesc = append(colorDesc, "greenish tones")
		} else if avgB > avgR+30 && avgB > avgG+30 {
			colorDesc = append(colorDesc, "bluish tones")
		} else {
			colorDesc = append(colorDesc, "mixed colors")
		}
	}
	
	if len(colorDesc) > 0 {
		return "Color analysis: " + strings.Join(colorDesc, ", ")
	}
	
	return ""
}

// performOCR extracts text from the image using tesseract
func performOCR(img image.Image, filePath string) (string, error) {
	// Create a context with timeout for OCR processing
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Initialize tesseract client
	client := gosseract.NewClient()
	defer client.Close()
	
	// Set language to English (can be made configurable)
	_ = client.SetLanguage("eng")
	
	// Configure tesseract for better accuracy
	_ = client.SetPageSegMode(gosseract.PSM_AUTO)
	_ = client.SetConfigFile("digits")
	
	// Try to use the file directly if it exists and is a supported format
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".bmp" {
		if _, err := os.Stat(filePath); err == nil {
			// File exists, use it directly
			text, err := client.Text()
			if err != nil {
				// Fallback to image buffer processing
				return performOCRFromImage(client, img)
			}
			_ = client.SetImage(filePath)
			return strings.TrimSpace(text), nil
		}
	}
	
	// Process from image buffer
	return performOCRFromImage(client, img)
}

// performOCRFromImage processes OCR from an image.Image
func performOCRFromImage(client *gosseract.Client, img image.Image) (string, error) {
	// Convert image to bytes (PNG format for tesseract)
	var buf bytes.Buffer
	if err := encodePNG(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode image for OCR: %v", err)
	}
	
	// Set image from buffer
	if err := client.SetImageFromBytes(buf.Bytes()); err != nil {
		return "", fmt.Errorf("tesseract failed to process image: %v", err)
	}
	
	// Extract text
	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("OCR text extraction failed: %v", err)
	}
	
	return strings.TrimSpace(text), nil
}

// encodePNG encodes an image as PNG to a writer
func encodePNG(w *bytes.Buffer, img image.Image) error {
	// Use the standard library PNG encoder
	encoder := &png.Encoder{
		CompressionLevel: png.DefaultCompression,
	}
	return encoder.Encode(w, img)
}

// maxInt64 returns the maximum of two int64 values
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// minInt64 returns the minimum of two int64 values
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// extractEXIFMetadata extracts EXIF metadata from JPEG images
func extractEXIFMetadata(imageData []byte, result *ImageAnalysisResult) {
	// Parse EXIF data
	rawExif, err := exif.SearchAndExtractExif(imageData)
	if err != nil {
		// EXIF data not found or corrupted, which is common
		result.Metadata["exif_available"] = "false"
		return
	}
	
	result.Metadata["exif_available"] = "true"
	
	// Parse the EXIF entries
	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		result.Metadata["exif_parse_error"] = err.Error()
		return
	}
	
	ti := exif.NewTagIndex()
	_, index, err := exif.Collect(im, ti, rawExif)
	if err != nil {
		result.Metadata["exif_parse_error"] = err.Error()
		return
	}
	
	// Extract key EXIF fields
	for _, ifd := range index.RootIfd.Children() {
		for _, entry := range ifd.Entries() {
			tagName := entry.TagName()
			value, err := entry.FormatFirst()
			if err != nil {
				continue
			}
			
			// Extract important EXIF data
			switch tagName {
			case "Make":
				result.Metadata["camera_make"] = value
			case "Model":
				result.Metadata["camera_model"] = value
			case "DateTime", "DateTimeOriginal":
				result.Metadata["date_taken"] = value
			case "Software":
				result.Metadata["software"] = value
			case "Orientation":
				result.Metadata["orientation"] = value
			case "XResolution":
				result.Metadata["x_resolution"] = value
			case "YResolution":
				result.Metadata["y_resolution"] = value
			case "ResolutionUnit":
				result.Metadata["resolution_unit"] = value
			case "Flash":
				result.Metadata["flash"] = value
			case "FocalLength":
				result.Metadata["focal_length"] = value
			case "ExposureTime":
				result.Metadata["exposure_time"] = value
			case "FNumber":
				result.Metadata["f_number"] = value
			case "ISO", "ISOSpeedRatings":
				result.Metadata["iso"] = value
			case "WhiteBalance":
				result.Metadata["white_balance"] = value
			}
		}
	}
	
	// Extract GPS data if available
	if gpsIfd, err := index.RootIfd.ChildWithIfdPath(exifcommon.IfdGpsInfoStandardIfdIdentity); err == nil && gpsIfd != nil {
		extractGPSData(gpsIfd, result)
	}
}

// extractGPSData extracts GPS coordinates from EXIF data
func extractGPSData(gpsIfd *exif.Ifd, result *ImageAnalysisResult) {
	var lat, lng float64
	var latRef, lngRef string
	var hasGPS bool
	
	for _, entry := range gpsIfd.Entries() {
		tagName := entry.TagName()
		value, err := entry.FormatFirst()
		if err != nil {
			continue
		}
		
		switch tagName {
		case "GPSLatitude":
			// Parse DMS format
			if parsedLat, err := parseDMS(entry); err == nil {
				lat = parsedLat
				hasGPS = true
			}
			result.Metadata["gps_latitude_raw"] = value
		case "GPSLatitudeRef":
			latRef = value
		case "GPSLongitude":
			// Parse DMS format
			if parsedLng, err := parseDMS(entry); err == nil {
				lng = parsedLng
				hasGPS = true
			}
			result.Metadata["gps_longitude_raw"] = value
		case "GPSLongitudeRef":
			lngRef = value
		case "GPSAltitude":
			result.Metadata["gps_altitude"] = value
		case "GPSTimeStamp":
			result.Metadata["gps_timestamp"] = value
		case "GPSDateStamp":
			result.Metadata["gps_date"] = value
		}
	}
	
	// Convert to decimal degrees if we have coordinates
	if hasGPS {
		if latRef == "S" {
			lat = -lat
		}
		if lngRef == "W" {
			lng = -lng
		}
		result.Metadata["gps_latitude"] = fmt.Sprintf("%.6f", lat)
		result.Metadata["gps_longitude"] = fmt.Sprintf("%.6f", lng)
		result.Metadata["gps_available"] = "true"
	} else {
		result.Metadata["gps_available"] = "false"
	}
}

// parseDMS parses degrees, minutes, seconds from EXIF GPS data
func parseDMS(entry *exif.IfdTagEntry) (float64, error) {
	// This is a simplified parser - actual GPS parsing is complex
	// For production use, consider using the built-in GPS parsing from go-exif
	rationals, err := entry.Value()
	if err != nil {
		return 0, fmt.Errorf("cannot get entry value: %w", err)
	}
	
	// Convert to rationals if possible
	rationalSlice, ok := rationals.([]exifcommon.Rational)
	if !ok {
		return 0, fmt.Errorf("value is not rational slice")
	}
	if len(rationalSlice) < 3 {
		return 0, fmt.Errorf("invalid DMS format")
	}
	
	degrees := float64(rationalSlice[0].Numerator) / float64(rationalSlice[0].Denominator)
	minutes := float64(rationalSlice[1].Numerator) / float64(rationalSlice[1].Denominator)
	seconds := float64(rationalSlice[2].Numerator) / float64(rationalSlice[2].Denominator)
	
	return degrees + minutes/60.0 + seconds/3600.0, nil
}

// processSVG handles SVG vector image analysis and text extraction
func processSVG(filePath string, content []byte, options ImageProcessingOptions) (*ImageAnalysisResult, error) {
	result := &ImageAnalysisResult{
		FileSize: int64(len(content)),
		Format:   "SVG",
		Metadata: make(map[string]string),
	}
	
	svgContent := string(content)
	
	// Extract SVG dimensions
	width, height := extractSVGDimensions(svgContent)
	result.Width = width
	result.Height = height
	
	// Add basic metadata
	result.Metadata["width"] = fmt.Sprintf("%d", width)
	result.Metadata["height"] = fmt.Sprintf("%d", height)
	result.Metadata["format"] = "SVG"
	result.Metadata["size_bytes"] = fmt.Sprintf("%d", result.FileSize)
	if height > 0 {
		result.Metadata["aspect_ratio"] = fmt.Sprintf("%.3f", float64(width)/float64(height))
	}
	
	// Analyze SVG content
	elements := analyzeSVGContent(svgContent)
	result.Metadata["svg_elements"] = strings.Join(elements, ", ")
	result.Metadata["svg_element_count"] = fmt.Sprintf("%d", len(elements))
	
	// Extract text content from SVG
	textContent := extractSVGText(svgContent)
	if textContent != "" {
		result.OCRText = textContent
		result.Metadata["text_length"] = fmt.Sprintf("%d", len(textContent))
	}
	
	// Generate description
	if options.EnableDescription {
		result.Description = generateSVGDescription(width, height, elements, textContent)
	}
	
	return result, nil
}

// extractSVGDimensions extracts width and height from SVG content
func extractSVGDimensions(svgContent string) (int, int) {
	// Look for width and height attributes in the SVG root element
	svgRegex := regexp.MustCompile(`<svg[^>]*`)
	svgMatch := svgRegex.FindString(svgContent)
	
	if svgMatch == "" {
		return 0, 0
	}
	
	// Extract width
	widthRegex := regexp.MustCompile(`width\s*=\s*["']?(\d+(?:\.\d+)?)`)
	widthMatch := widthRegex.FindStringSubmatch(svgMatch)
	
	// Extract height
	heightRegex := regexp.MustCompile(`height\s*=\s*["']?(\d+(?:\.\d+)?)`)
	heightMatch := heightRegex.FindStringSubmatch(svgMatch)
	
	var width, height int
	
	if len(widthMatch) > 1 {
		if w, err := strconv.ParseFloat(widthMatch[1], 64); err == nil {
			width = int(w)
		}
	}
	
	if len(heightMatch) > 1 {
		if h, err := strconv.ParseFloat(heightMatch[1], 64); err == nil {
			height = int(h)
		}
	}
	
	// If width/height are not specified, try to extract from viewBox
	if width == 0 || height == 0 {
		viewBoxRegex := regexp.MustCompile(`viewBox\s*=\s*["']([^"']+)["']`)
		viewBoxMatch := viewBoxRegex.FindStringSubmatch(svgMatch)
		
		if len(viewBoxMatch) > 1 {
			parts := strings.Fields(viewBoxMatch[1])
			if len(parts) >= 4 {
				if w, err := strconv.ParseFloat(parts[2], 64); err == nil && width == 0 {
					width = int(w)
				}
				if h, err := strconv.ParseFloat(parts[3], 64); err == nil && height == 0 {
					height = int(h)
				}
			}
		}
	}
	
	// Default fallback dimensions
	if width == 0 {
		width = 300
	}
	if height == 0 {
		height = 150
	}
	
	return width, height
}

// analyzeSVGContent analyzes the SVG elements present in the content
func analyzeSVGContent(svgContent string) []string {
	var elements []string
	elementMap := make(map[string]bool)
	
	// Common SVG elements to detect
	svgElements := []string{
		"rect", "circle", "ellipse", "line", "polyline", "polygon",
		"path", "text", "tspan", "image", "g", "defs", "clipPath",
		"mask", "pattern", "linearGradient", "radialGradient", "use",
	}
	
	for _, element := range svgElements {
		pattern := fmt.Sprintf(`<%s[\s>]`, element)
		regex := regexp.MustCompile(pattern)
		if regex.MatchString(svgContent) && !elementMap[element] {
			elements = append(elements, element)
			elementMap[element] = true
		}
	}
	
	return elements
}

// extractSVGText extracts text content from SVG text elements
func extractSVGText(svgContent string) string {
	// Extract content from <text> elements
	textRegex := regexp.MustCompile(`<text[^>]*>([^<]+)</text>`)
	textMatches := textRegex.FindAllStringSubmatch(svgContent, -1)
	
	// Extract content from <tspan> elements
	tspanRegex := regexp.MustCompile(`<tspan[^>]*>([^<]+)</tspan>`)
	tspanMatches := tspanRegex.FindAllStringSubmatch(svgContent, -1)
	
	var textParts []string
	
	// Collect text from text elements
	for _, match := range textMatches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				textParts = append(textParts, text)
			}
		}
	}
	
	// Collect text from tspan elements
	for _, match := range tspanMatches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				textParts = append(textParts, text)
			}
		}
	}
	
	return strings.Join(textParts, " ")
}

// generateSVGDescription creates a description of the SVG content
func generateSVGDescription(width, height int, elements []string, textContent string) string {
	description := fmt.Sprintf("An SVG vector image with dimensions %dx%d pixels", width, height)
	
	if len(elements) > 0 {
		description += fmt.Sprintf(", containing %d types of elements: %s", len(elements), strings.Join(elements, ", "))
	}
	
	if textContent != "" {
		wordCount := len(strings.Fields(textContent))
		description += fmt.Sprintf(". Contains text with %d words", wordCount)
	}
	
	// Determine complexity based on element count
	if len(elements) <= 2 {
		description += ". Simple vector graphic"
	} else if len(elements) <= 5 {
		description += ". Moderately complex vector graphic"
	} else {
		description += ". Complex vector graphic with multiple element types"
	}
	
	return description
}