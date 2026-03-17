// Normalize model responses into standardized structs.
package inference

import (
	"encoding/json"
	"strings"
)

// InterpretationResult holds parsed output from Stage 1 inference.
type InterpretationResult struct {
	Interpretation string `json:"interpretation"`
	Category       string `json:"category"`
	AppName        string `json:"app_name"`
}

// ParseInterpretation extracts structured data from the model's JSON response.
func ParseInterpretation(raw string) (InterpretationResult, error) {
	// Strip markdown fences if the model wraps its output
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var result InterpretationResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return InterpretationResult{}, err
	}
	return result, nil
}
