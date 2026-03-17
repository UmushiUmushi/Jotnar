// Preview and commit reconsolidation from user edits.
package processing

import (
	"fmt"
	"time"

	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/store"
)

type Reconsolidator struct {
	consolidator *Consolidator
	config       *config.Manager
	metadata     *store.MetadataStore
	journal      *store.JournalStore
}

func NewReconsolidator(consolidator *Consolidator, cfg *config.Manager, metadata *store.MetadataStore, journal *store.JournalStore) *Reconsolidator {
	return &Reconsolidator{consolidator: consolidator, config: cfg, metadata: metadata, journal: journal}
}

// Preview generates a new narrative from the given metadata IDs without saving.
func (r *Reconsolidator) Preview(includeIDs []string) (string, error) {
	rows, err := r.metadata.GetByIDs(includeIDs)
	if err != nil {
		return "", fmt.Errorf("get metadata: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no metadata found for the given IDs")
	}

	cfg := r.config.Get()
	return r.consolidator.SoftConsolidate(rows, cfg.JournalTone)
}

// Commit performs the reconsolidation: deletes excluded metadata, rewrites the entry.
// If narrative is non-empty, it is used directly (e.g. from a prior preview or user edit).
// Otherwise, inference is run to generate a new narrative.
func (r *Reconsolidator) Commit(entryID string, includeIDs []string, narrative string) (*store.JournalEntry, error) {
	// Get all metadata currently linked to this entry
	allMeta, err := r.metadata.GetByEntryID(entryID)
	if err != nil {
		return nil, fmt.Errorf("get entry metadata: %w", err)
	}

	// Find excluded IDs
	includeSet := make(map[string]bool)
	for _, id := range includeIDs {
		includeSet[id] = true
	}
	var excludeIDs []string
	for _, m := range allMeta {
		if !includeSet[m.ID] {
			excludeIDs = append(excludeIDs, m.ID)
		}
	}

	// Get included metadata for time range calculation
	included, err := r.metadata.GetByIDs(includeIDs)
	if err != nil {
		return nil, fmt.Errorf("get included metadata: %w", err)
	}
	if len(included) == 0 {
		return nil, fmt.Errorf("no metadata to include")
	}

	// Use provided narrative or generate a new one via inference
	if narrative == "" {
		cfg := r.config.Get()
		narrative, err = r.consolidator.SoftConsolidate(included, cfg.JournalTone)
		if err != nil {
			return nil, fmt.Errorf("reconsolidate: %w", err)
		}
	}

	// Delete excluded metadata
	if err := r.metadata.DeleteByIDs(excludeIDs); err != nil {
		return nil, fmt.Errorf("delete excluded metadata: %w", err)
	}

	// Update the journal entry
	if err := r.journal.UpdateNarrative(entryID, narrative, true); err != nil {
		return nil, fmt.Errorf("update narrative: %w", err)
	}

	// Update time range
	timeStart := included[0].CapturedAt
	timeEnd := included[len(included)-1].CapturedAt
	if err := r.journal.UpdateTimeRange(entryID, timeStart, timeEnd); err != nil {
		return nil, fmt.Errorf("update time range: %w", err)
	}

	now := time.Now().UTC()
	return &store.JournalEntry{
		ID:        entryID,
		Narrative: narrative,
		TimeStart: timeStart,
		TimeEnd:   timeEnd,
		Edited:    true,
		UpdatedAt: &now,
	}, nil
}
