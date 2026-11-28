package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveSettings writes an explicit set of values to a configuration layer,
// deep-merging with any existing file so unrelated keys survive. It persists
// only the keys the caller passes, keeping user settings a delta over the
// tracked project defaults instead of a frozen snapshot. A key passed
// with a nil value is removed from the file.
func (ll *LayeredLoader) SaveSettings(source ConfigSource, values map[string]interface{}) error {
	var path string
	switch source {
	case SourceUser:
		path = filepath.Join(ll.configHome, "settings.json")
	case SourceProject:
		path = filepath.Join(ll.cwd, ".agent-harness", "settings.json")
	case SourceLocal:
		path = filepath.Join(ll.cwd, ".agent-harness", "settings.local.json")
	}

	merged := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &merged); err != nil {
			return fmt.Errorf("failed to parse existing config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config %s: %w", path, err)
	}
	ll.deepMerge(merged, values)
	// A nil value clears its key: the setting returns to whatever the
	// lower layers or the defaults say.
	for key, value := range values {
		if value == nil {
			delete(merged, key)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomically with secure permissions so a crash mid-save cannot
	// truncate or corrupt the settings file.
	if err := writeFileSecure(path, append(jsonData, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
