// Package testenv isolates environment-dependent test fixtures.
package testenv

// SetHome redirects both Unix and Windows home-directory lookups. Setenv
// registers cleanup with the test, restoring the original environment.
func SetHome(t interface{ Setenv(string, string) }, dir string) {
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
