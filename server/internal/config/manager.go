// Read/write /data/config.yml, apply defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	_ "time/tzdata"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	ConsolidationWindowMin int    `yaml:"consolidation_window_min" json:"consolidation_window_min"`
	InterpretationDetail   string `yaml:"interpretation_detail" json:"interpretation_detail"`
	JournalTone            string `yaml:"journal_tone" json:"journal_tone"`
	MetadataRetentionDays  *int   `yaml:"metadata_retention_days" json:"metadata_retention_days"`
	Timezone               string `yaml:"timezone" json:"timezone"`
}

var validInterpretationDetails = map[string]bool{
	"minimal":  true,
	"standard": true,
	"detailed": true,
}

var validJournalTones = map[string]bool{
	"casual":    true,
	"concise":   true,
	"narrative": true,
}

// Validate checks that all enum fields contain valid values.
func (c *ServerConfig) Validate() error {
	if c.ConsolidationWindowMin <= 0 {
		return fmt.Errorf("consolidation_window_min must be positive, got %d", c.ConsolidationWindowMin)
	}
	if !validInterpretationDetails[c.InterpretationDetail] {
		return fmt.Errorf("invalid interpretation_detail %q, must be minimal/standard/detailed", c.InterpretationDetail)
	}
	if !validJournalTones[c.JournalTone] {
		return fmt.Errorf("invalid journal_tone %q, must be casual/concise/narrative", c.JournalTone)
	}
	if c.MetadataRetentionDays != nil && *c.MetadataRetentionDays < 1 {
		return fmt.Errorf("metadata_retention_days must be positive or null, got %d", *c.MetadataRetentionDays)
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", c.Timezone, err)
	}
	return nil
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ConsolidationWindowMin: 30,
		InterpretationDetail:   "standard",
		JournalTone:            "casual",
		MetadataRetentionDays:  nil,
		Timezone:               "UTC",
	}
}

type Manager struct {
	mu       sync.RWMutex
	config   ServerConfig
	filePath string
}

func NewManager(filePath string) (*Manager, error) {
	m := &Manager{filePath: filePath}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Get() ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Location returns the *time.Location for the configured timezone.
// Falls back to UTC if the timezone string is invalid.
func (c *ServerConfig) Location() *time.Location {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (m *Manager) Update(cfg ServerConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	return m.save()
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if os.IsNotExist(err) {
		m.config = DefaultServerConfig()
		return m.save()
	}
	if err != nil {
		return err
	}

	m.config = DefaultServerConfig()
	return yaml.Unmarshal(data, &m.config)
}

// save writes the config atomically via a temp file + rename.
func (m *Manager) save() error {
	data, err := yaml.Marshal(&m.config)
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.filePath)
	tmp, err := os.CreateTemp(dir, ".config-*.yml.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, m.filePath)
}
