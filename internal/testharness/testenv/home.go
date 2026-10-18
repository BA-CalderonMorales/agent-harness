// Package testenv isolates environment-dependent test fixtures.
package testenv

// SetHome isolates Unix/Windows home lookups and inherited application/XDG
// roots. Redirecting HOME alone still lets fixtures touch the real credential
// store and audit log on desktops that export XDG_*_HOME. Tests may set explicit
// overrides after this call. Every value is restored by test cleanup.
func SetHome(t interface{ Setenv(string, string) }, dir string) {
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	for _, name := range []string{"AGENT_HARNESS_CONFIG_HOME", "AGENT_HARNESS_DATA_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "AGENT_HARNESS_SESSION_DIR"} {
		t.Setenv(name, "")
	}
}
