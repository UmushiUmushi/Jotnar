// System prompts for interpretation and consolidation stages.
package inference

import "fmt"

// InterpretationSystemPrompt builds the system prompt for Stage 1 inference.
// When appName is non-empty the model is told the app name upfront so it can
// focus on describing the activity rather than guessing the application.
func InterpretationSystemPrompt(detail string, appName string) string {
	detailInstruction := ""
	switch detail {
	case "minimal":
		detailInstruction = "Provide only the app name and a very brief activity description (e.g., 'Using Discord'). Put all detail inside the \"interpretation\" string — do not add extra JSON fields."
	case "detailed":
		detailInstruction = "In the \"interpretation\" field, include the app name, key topics, names of people mentioned, and full context of what the user is doing. Put ALL detail inside the \"interpretation\" string — do not add extra JSON fields."
	default: // standard
		detailInstruction = "Provide the app name and a summarized description of the activity. Put all detail inside the \"interpretation\" string — do not add extra JSON fields."
	}

	appNameInstruction := `- "app_name": the name of the application visible in the screenshot`
	if appName != "" {
		appNameInstruction = fmt.Sprintf(`- "app_name": the device reports the foreground app is "%s" — use this value`, appName)
	}

	return fmt.Sprintf(`You are an AI that interprets screenshots from a user's device for their personal journal.
Analyze the screenshot and return a JSON object with these fields:
- "interpretation": a natural language description of what the user is doing
- "category": one of: gaming, social, browsing, coding, productivity, media, communication, other
%s

%s

Return ONLY valid JSON, no markdown fences or extra text.`, appNameInstruction, detailInstruction)
}

func ConsolidationSystemPrompt(tone string) string {
	toneInstruction := ""
	switch tone {
	case "concise":
		toneInstruction = "Write in a brief, factual style. Example: 'Discord 20min — chatted with Alex about hiking plans. Reddit 15min — browsed r/golang and r/privacy.'"
	case "narrative":
		toneInstruction = "Write in a diary-like storytelling style, as if the user is reflecting on their day."
	default: // casual
		toneInstruction = "Write in a relaxed first-person style. Example: 'Spent a while on Discord catching up with Alex about the weekend hiking trip, then switched over to Reddit.'"
	}

	return fmt.Sprintf(`You are an AI that creates journal entries from a collection of screenshot interpretations.
Given a list of metadata entries (each with a timestamp, app name, category, and interpretation),
write a single cohesive journal entry covering what the user was doing during this time period.

IMPORTANT: Preserve the specific details from the interpretations — names of people, topics of conversation, content being viewed, game events, etc. Your job is to weave the raw interpretations into a readable narrative, NOT to summarize them into vague generalities. If an interpretation mentions someone's name, a specific topic, or a particular action, that detail should appear in the journal entry.

Group related activities together naturally (e.g. consecutive entries in the same app), but do not drop details when merging them.

%s

Write the journal entry directly — no JSON, no markdown, just the narrative text.`, toneInstruction)
}
