package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// defaultConfigRoot is where the config home resolves when no environment
// override selects another location. It is also the literal root the
// credential store used before it followed ConfigHome().
func defaultConfigRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "agent-harness")
}

// legacySecureConfigPath is the store's pre-ConfigHome location.
// SecureConfigPath no longer resolves here once an override is set;
// AdoptLegacyCredentialStore relocates a store found here so switching
// profiles never strands keys already entered.
func legacySecureConfigPath() string {
	return filepath.Join(defaultConfigRoot(), "credentials.enc")
}

// AdoptLegacyCredentialStore moves a credential store written at the
// default config root into the resolved config home when an override
// selects another home and no store exists there yet.
//
// The store ignored AGENT_HARNESS_CONFIG_HOME and XDG_CONFIG_HOME, so a
// machine pointed at a profile kept writing keys to the shared default
// root and profiles clobbered each other. Adopting once preserves the
// credentials already entered while giving the first profile its own
// store. Idempotent and best-effort: a store at the resolved home always
// wins, a missing legacy store is a no-op, and nothing is deleted that
// was not moved. Returns a boot-notice summary when it relocates a store.
func AdoptLegacyCredentialStore() string {
	resolved := SecureConfigPath()
	legacy := legacySecureConfigPath()
	if filepath.Clean(resolved) == filepath.Clean(legacy) {
		return "" // no override: the store already lives here
	}
	if _, err := os.Stat(resolved); err == nil {
		return "" // a profile store exists; never overwrite it
	}
	if _, err := os.Stat(legacy); err != nil {
		return "" // nothing to adopt
	}
	if err := os.MkdirAll(filepath.Dir(resolved), secureDirPerms); err != nil {
		return ""
	}
	if err := relocateFile(legacy, resolved); err != nil {
		// Move failed (for example across filesystems and a read-only
		// target): leave the legacy store where it is and let the normal
		// login path recover, rather than half-moving the pair.
		return ""
	}
	// The machine-local key is useless apart from its store, so it must
	// travel too or the relocated store cannot decrypt. A store derived
	// from a passphrase has no key file.
	keySrc := filepath.Join(filepath.Dir(legacy), "master.key")
	keyDst := filepath.Join(filepath.Dir(resolved), "master.key")
	if _, err := os.Stat(keySrc); err == nil {
		if _, err := os.Stat(keyDst); os.IsNotExist(err) {
			_ = relocateFile(keySrc, keyDst)
		}
	}
	return fmt.Sprintf("Credential store moved to the configured config home (%s).", filepath.Dir(resolved))
}

// relocateFile moves src to dst, falling back to copy-and-remove when a
// plain rename cannot cross a device boundary (for example a Windows
// config home on another drive). The destination inherits owner-only
// permissions.
func relocateFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, secureFilePerms)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return os.Remove(src)
}
