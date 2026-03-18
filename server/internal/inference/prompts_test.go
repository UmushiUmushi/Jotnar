package inference

import (
	"strings"
	"testing"
)

func TestInterpretationSystemPrompt_Minimal(t *testing.T) {
	prompt := InterpretationSystemPrompt("minimal")
	if !strings.Contains(prompt, "very brief") {
		t.Error("minimal prompt should contain 'very brief'")
	}
}

func TestInterpretationSystemPrompt_Standard(t *testing.T) {
	prompt := InterpretationSystemPrompt("standard")
	if !strings.Contains(prompt, "summarized") {
		t.Error("standard prompt should contain 'summarized'")
	}
}

func TestInterpretationSystemPrompt_Detailed(t *testing.T) {
	prompt := InterpretationSystemPrompt("detailed")
	if !strings.Contains(prompt, "key topics") {
		t.Error("detailed prompt should contain 'key topics'")
	}
}

func TestInterpretationSystemPrompt_UnknownDefaultsToStandard(t *testing.T) {
	prompt := InterpretationSystemPrompt("unknown_value")
	if !strings.Contains(prompt, "summarized") {
		t.Error("unknown detail level should default to standard (contain 'summarized')")
	}
}

func TestConsolidationSystemPrompt_Casual(t *testing.T) {
	prompt := ConsolidationSystemPrompt("casual")
	if !strings.Contains(prompt, "relaxed") {
		t.Error("casual prompt should contain 'relaxed'")
	}
}

func TestConsolidationSystemPrompt_Concise(t *testing.T) {
	prompt := ConsolidationSystemPrompt("concise")
	if !strings.Contains(prompt, "brief, factual") {
		t.Error("concise prompt should contain 'brief, factual'")
	}
}

func TestConsolidationSystemPrompt_Narrative(t *testing.T) {
	prompt := ConsolidationSystemPrompt("narrative")
	if !strings.Contains(prompt, "diary-like") {
		t.Error("narrative prompt should contain 'diary-like'")
	}
}
