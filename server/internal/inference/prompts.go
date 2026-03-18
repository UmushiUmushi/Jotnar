// System prompts for interpretation and consolidation stages.
package inference

import "fmt"

func InterpretationSystemPrompt(detail string) string {
	detailInstruction := ""
	switch detail {
	case "minimal":
		detailInstruction = "Provide only the app name and a very brief activity description (e.g., 'Using Discord')."
	case "detailed":
		detailInstruction = "Provide the app name, key topics, names mentioned, and full context of what the user is doing."
	default: // standard
		detailInstruction = "Provide the app name and a summarized description of the activity."
	}

	return fmt.Sprintf(`You are an AI that interprets screenshots from a user's device for their personal journal.
Analyze the screenshot and return a JSON object with these fields:
- "interpretation": a natural language description of what the user is doing
- "category": one of: gaming, social, browsing, coding, productivity, media, communication, other
- "app_name": the name of the application visible in the screenshot

%s

Return ONLY valid JSON, no markdown fences or extra text. /no_think`, detailInstruction)
}

func ConsolidationSystemPrompt(tone string) string {
	toneInstruction := ""
	switch tone {
	case "concise":
		toneInstruction = "Write in a brief, factual style. Example: 'Discord 20min, Reddit 15min, YouTube 10min'."
	case "narrative":
		toneInstruction = "Write in a diary-like storytelling style, as if the user is reflecting on their day."
	default: // casual
		toneInstruction = "Write in a relaxed first-person style. Example: 'Hung out on Discord for a bit, then switched to Reddit'."
	}

	return fmt.Sprintf(`You are an AI that creates journal entries from a collection of screenshot interpretations.
Given a list of metadata entries (each with a timestamp, app name, category, and interpretation),
write a single cohesive journal entry that summarizes what the user was doing during this time period.

%s

Write the journal entry directly — no JSON, no markdown, just the narrative text. /no_think`, toneInstruction)
}
