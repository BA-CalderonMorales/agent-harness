package config

import (
	"strconv"
	"strings"
)

// Lab is one experimental, behavior-changing feature that ships behind a
// flag. Key is also the settings.json key the choice is saved under, so
// it must not collide with any other settings key.
type Lab struct {
	Key         string
	Label       string
	Description string
	// EnvVar pins the flag for one session (AH_*); the AGENT_HARNESS_*
	// spelling of the same name is accepted too, like every other
	// setting.
	EnvVar  string
	Default bool
}

// LabLoopTrack shows the Ask/Plan/Review/Try beat track in the chat chrome.
const LabLoopTrack = "loop_track"

// Labs is the only list of experimental flags. New behavior-changing
// features ship behind an entry here; the Settings tab, the environment
// overrides, and persistence are all generated from it.
//
// Graduation rule: a flag graduates when it has a named use, documented
// behavior, and tests. Graduating deletes the entry and the branch it
// guards; a graduated flag is never left here switched on.
//
// Record compaction (session record, PR #63) is the next planned entry;
// add it here when that feature lands on main, not before.
var Labs = []Lab{
	{
		Key:         LabLoopTrack,
		Label:       "Loop Track",
		Description: "Show the Ask/Plan/Review/Try beat track in the chat while a turn runs.",
		EnvVar:      "AH_LOOP_TRACK",
		Default:     false,
	},
}

// LookupLab returns the registry entry for key.
func LookupLab(key string) (Lab, bool) {
	for _, lab := range Labs {
		if lab.Key == key {
			return lab, true
		}
	}
	return Lab{}, false
}

// labEnvNames lists the environment names that pin lab, AH_* first.
func labEnvNames(lab Lab) []string {
	names := []string{lab.EnvVar}
	if rest, ok := strings.CutPrefix(lab.EnvVar, "AH_"); ok {
		names = append(names, "AGENT_HARNESS_"+rest)
	}
	return names
}

// applyLabEnvOverrides records every lab the environment pins. A value
// that does not parse as a bool is ignored, the same rule the other bool
// environment settings follow.
func applyLabEnvOverrides(config *LayeredConfig) {
	for _, lab := range Labs {
		v := firstEnv(labEnvNames(lab)...)
		if v == "" {
			continue
		}
		if parsed, err := strconv.ParseBool(v); err == nil {
			if config.LabsPinned == nil {
				config.LabsPinned = map[string]bool{}
			}
			config.LabsPinned[lab.Key] = parsed
		}
	}
}

// extractLabs reads every lab one settings layer sets. A project layer
// sits below the user layer, so its value is inherited, not the user's
// choice: it is never copied into the user layer on save. The user and
// local layers hold the user's own choices.
func extractLabs(config *LayeredConfig, source ConfigSource, values map[string]interface{}) {
	for _, lab := range Labs {
		v, ok := boolValue(values, lab.Key)
		if !ok {
			continue
		}
		target := &config.LabsSaved
		if source == SourceProject {
			target = &config.labsInherited
		}
		if *target == nil {
			*target = map[string]bool{}
		}
		(*target)[lab.Key] = v
	}
}

// LabEnabled reports whether the lab is on: an environment pin wins, then
// the user's saved choice, then a value inherited from a project layer,
// then the registry Default. An unregistered key is off.
func (c *LayeredConfig) LabEnabled(key string) bool {
	lab, ok := LookupLab(key)
	if !ok {
		return false
	}
	if v, ok := c.LabsPinned[key]; ok {
		return v
	}
	if v, ok := c.LabsSaved[key]; ok {
		return v
	}
	if v, ok := c.labsInherited[key]; ok {
		return v
	}
	return lab.Default
}

// LabPinned reports whether the environment pins the lab this session.
func (c *LayeredConfig) LabPinned(key string) bool {
	_, ok := c.LabsPinned[key]
	return ok
}

// SetLab records the user's choice for a lab. With no lower layer setting
// the lab, choosing the Default clears the choice instead of storing it,
// so the lab keeps following the registry. When a project layer sets it,
// the choice is always stored, so it outlasts the inherited value.
func (c *LayeredConfig) SetLab(key string, enabled bool) {
	lab, ok := LookupLab(key)
	if !ok {
		return
	}
	if _, inherited := c.labsInherited[key]; !inherited && enabled == lab.Default {
		c.ClearLab(key)
		return
	}
	if c.LabsSaved == nil {
		c.LabsSaved = map[string]bool{}
	}
	c.LabsSaved[key] = enabled
}

// ClearLab drops the saved choice so the lab returns to the value a
// project layer sets, or else its Default.
func (c *LayeredConfig) ClearLab(key string) {
	delete(c.LabsSaved, key)
}

// LabSettingsValues is the Labs part of a settings save: a saved choice
// is written, a cleared one is nil (SaveSettings removes the key), and an
// environment-pinned lab is left out so the pin does not outlive the
// variable and the user's own saved choice survives it.
func (c *LayeredConfig) LabSettingsValues() map[string]interface{} {
	values := make(map[string]interface{}, len(Labs))
	for _, lab := range Labs {
		if c.LabPinned(lab.Key) {
			continue
		}
		if v, ok := c.LabsSaved[lab.Key]; ok {
			values[lab.Key] = v
		} else {
			values[lab.Key] = nil
		}
	}
	return values
}
