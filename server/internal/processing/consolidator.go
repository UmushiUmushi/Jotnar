// Stage 2: metadata batch → inference → journal entry.
package processing

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

type Consolidator struct {
	client   *inference.Client
	config   *config.Manager
	metadata *store.MetadataStore
	journal  *store.JournalStore
}

func NewConsolidator(client *inference.Client, cfg *config.Manager, metadata *store.MetadataStore, journal *store.JournalStore) *Consolidator {
	return &Consolidator{client: client, config: cfg, metadata: metadata, journal: journal}
}

// Run checks for unconsolidated metadata and creates journal entries.
func (c *Consolidator) Run() error {
	rows, err := c.metadata.GetUnconsolidated()
	if err != nil {
		return fmt.Errorf("get unconsolidated: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	log.Printf("Consolidation: found %d unconsolidated metadata rows", len(rows))

	cfg := c.config.Get()
	window := time.Duration(cfg.ConsolidationWindowMin) * time.Minute

	// Group metadata into time windows
	batches := groupByWindow(rows, window)

	for i, batch := range batches {
		log.Printf("Consolidation: processing batch %d/%d (%d rows, %s to %s)",
			i+1, len(batches), len(batch),
			batch[0].CapturedAt.Format("15:04"),
			batch[len(batch)-1].CapturedAt.Format("15:04"))
		if err := c.consolidateBatch(batch, cfg.JournalTone); err != nil {
			return err
		}
	}

	log.Printf("Consolidation: created %d journal entries", len(batches))
	return nil
}

func (c *Consolidator) consolidateBatch(batch []store.Metadata, tone string) error {
	prompt := formatMetadataForPrompt(batch)

	req := inference.ChatRequest{
		Messages: []inference.Message{
			{Role: "system", Content: inference.ConsolidationSystemPrompt(tone)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.5,
		MaxTokens:   2048,
		Think:       inference.BoolPtr(false),
	}

	narrative, err := c.client.Complete(req)
	if err != nil {
		return fmt.Errorf("consolidation inference: %w", err)
	}

	entry := store.JournalEntry{
		ID:        uuid.New().String(),
		Narrative: strings.TrimSpace(narrative),
		TimeStart: batch[0].CapturedAt,
		TimeEnd:   batch[len(batch)-1].CapturedAt,
		CreatedAt: time.Now().UTC(),
	}

	if err := c.journal.Create(entry); err != nil {
		return fmt.Errorf("create journal entry: %w", err)
	}

	ids := make([]string, len(batch))
	for i, m := range batch {
		ids[i] = m.ID
	}
	if err := c.metadata.LinkToEntry(ids, entry.ID); err != nil {
		return fmt.Errorf("link metadata: %w", err)
	}

	return nil
}

// SoftConsolidate creates a narrative from the given metadata rows without saving.
func (c *Consolidator) SoftConsolidate(rows []store.Metadata, tone string) (string, error) {
	prompt := formatMetadataForPrompt(rows)

	req := inference.ChatRequest{
		Messages: []inference.Message{
			{Role: "system", Content: inference.ConsolidationSystemPrompt(tone)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.5,
		MaxTokens:   2048,
		Think:       inference.BoolPtr(false),
	}

	narrative, err := c.client.Complete(req)
	if err != nil {
		return "", fmt.Errorf("consolidation inference: %w", err)
	}
	return strings.TrimSpace(narrative), nil
}

func groupByWindow(rows []store.Metadata, window time.Duration) [][]store.Metadata {
	if len(rows) == 0 {
		return nil
	}

	var batches [][]store.Metadata
	var current []store.Metadata
	windowStart := rows[0].CapturedAt

	for _, r := range rows {
		if r.CapturedAt.Sub(windowStart) > window && len(current) > 0 {
			batches = append(batches, current)
			current = nil
			windowStart = r.CapturedAt
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}

	return batches
}

func formatMetadataForPrompt(rows []store.Metadata) string {
	var sb strings.Builder
	for _, m := range rows {
		sb.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n",
			m.CapturedAt.Format("15:04:05"),
			m.AppName,
			m.Category,
			m.Interpretation,
		))
	}
	return sb.String()
}
