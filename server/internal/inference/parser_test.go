package inference

import (
	"testing"
)

func TestParseInterpretation_ValidJSON(t *testing.T) {
	raw := `{"interpretation":"Playing Genshin Impact","category":"gaming","app_name":"Genshin Impact"}`
	result, err := ParseInterpretation(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Interpretation != "Playing Genshin Impact" {
		t.Errorf("interpretation = %q, want %q", result.Interpretation, "Playing Genshin Impact")
	}
	if result.Category != "gaming" {
		t.Errorf("category = %q, want %q", result.Category, "gaming")
	}
	if result.AppName != "Genshin Impact" {
		t.Errorf("app_name = %q, want %q", result.AppName, "Genshin Impact")
	}
}

func TestParseInterpretation_MarkdownFences(t *testing.T) {
	raw := "```json\n{\"interpretation\":\"Browsing Reddit\",\"category\":\"browsing\",\"app_name\":\"Reddit\"}\n```"
	result, err := ParseInterpretation(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AppName != "Reddit" {
		t.Errorf("app_name = %q, want %q", result.AppName, "Reddit")
	}
}

func TestParseInterpretation_InvalidJSON(t *testing.T) {
	_, err := ParseInterpretation("not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseInterpretation_EmptyString(t *testing.T) {
	_, err := ParseInterpretation("")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}
