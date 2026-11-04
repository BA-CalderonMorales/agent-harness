package tui

// SettingsUpdatedMsg applies the authoritative state after a delegate callback.
// A queued update survives SettingsModel.Update returning its current copy.
type SettingsUpdatedMsg struct {
	Settings []Setting
}
