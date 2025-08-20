package main

import "strings"

// expandBraces performs a simple brace expansion similar to shell behavior.
// It supports a single level or nested brace expressions separated by commas.
// If no braces are present or they are unbalanced, the original path is returned.
func expandBraces(p string) []string {
	open := strings.Index(p, "{")
	if open == -1 {
		return []string{p}
	}
	close := strings.Index(p[open+1:], "}")
	if close == -1 {
		return []string{p}
	}
	close += open + 1
	prefix := p[:open]
	suffix := p[close+1:]
	inner := p[open+1 : close]
	parts := strings.Split(inner, ",")
	var results []string
	for _, part := range parts {
		expanded := prefix + part + suffix
		results = append(results, expandBraces(expanded)...)
	}
	return results
}
