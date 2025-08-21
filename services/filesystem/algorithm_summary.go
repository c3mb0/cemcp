package main

import (
	"encoding/json"
	"strconv"
)

// summaryForAlgorithm converts algorithm-specific numeric values into float64.
// It accepts float64, json.Number, and string representations of numbers.
// For strings and json.Number values, strconv.ParseFloat is used to support
// wider client inputs.
func summaryForAlgorithm(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f
		}
		if f, err := strconv.ParseFloat(val.String(), 64); err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}
