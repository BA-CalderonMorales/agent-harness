package config

// The local layer (.agent-harness/settings.local.json) sits above the user
// layer. A lab it sets belongs to that file: its value is never copied into
// the user layer, and the app never writes it. It outranks any SetLab or
// ClearLab, and the Settings row refuses the toggle (settings_labs.go), so
// only editing the file changes it.

// noteLocalLab records a lab value loaded from the local layer.
func noteLocalLab(config *LayeredConfig, key string, v bool) {
	if config.labsLocal == nil {
		config.labsLocal = map[string]bool{}
	}
	config.labsLocal[key] = v
}

// LabSetByLocal reports whether settings.local.json sets the lab, which
// locks it against in-app changes.
func (c *LayeredConfig) LabSetByLocal(key string) bool {
	_, ok := c.labsLocal[key]
	return ok
}
