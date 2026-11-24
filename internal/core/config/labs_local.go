package config

// The local layer (.agent-harness/settings.local.json) sits above the user
// layer. A lab it sets belongs to that file: its value is never copied into
// the user layer, and a Settings toggle over it is written back to the
// local file, the only place a lasting change can live. Otherwise the next
// boot would read the local value again and undo the toggle.

// noteLocalLab records a lab value loaded from the local layer.
func noteLocalLab(config *LayeredConfig, key string, v bool) {
	if config.labsLocal == nil {
		config.labsLocal = map[string]bool{}
		config.labsLocalLoaded = map[string]bool{}
	}
	config.labsLocal[key] = v
	config.labsLocalLoaded[key] = v
}

// labLocalOwned reports whether the local layer set the lab at load.
func (c *LayeredConfig) labLocalOwned(key string) bool {
	_, ok := c.labsLocalLoaded[key]
	return ok
}

// LabLocalValues is the local-layer part of a settings save: each lab the
// local file owns and this session changed. A cleared lab is nil, which
// removes its key. An untouched or env-pinned lab is left out, so a save
// with nothing to change leaves the local file alone.
func (c *LayeredConfig) LabLocalValues() map[string]interface{} {
	values := map[string]interface{}{}
	for key, loaded := range c.labsLocalLoaded {
		if c.LabPinned(key) {
			continue
		}
		current, ok := c.labsLocal[key]
		switch {
		case !ok:
			values[key] = nil
		case current != loaded:
			values[key] = current
		}
	}
	return values
}
