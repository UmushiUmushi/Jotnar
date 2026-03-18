// Normalize model responses into standardized structs.
package inference

import (
	"encoding/json"
	"fmt"
	"strings"
)

// InterpretationResult holds parsed output from Stage 1 inference.
type InterpretationResult struct {
	Interpretation string `json:"interpretation"`
	Category       string `json:"category"`
	AppName        string `json:"app_name"`
}

// ParseInterpretation extracts structured data from the model's JSON response.
// Handles markdown fences, and falls back to extracting the first JSON object
// from the text (e.g. when the model mixes reasoning with its answer).
func ParseInterpretation(raw string) (InterpretationResult, error) {
	// Strip markdown fences if the model wraps its output
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var result InterpretationResult
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		return result, nil
	}

	// Fallback: find the first JSON object in the text
	if extracted := extractJSON(cleaned); extracted != "" {
		if err := json.Unmarshal([]byte(extracted), &result); err == nil {
			return result, nil
		}
	}

	return InterpretationResult{}, fmt.Errorf("no valid JSON found in response (length: %d)", len(raw))
}

// extractJSON finds the first { ... } block in the text using brace matching.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
