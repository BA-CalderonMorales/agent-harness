package commands

import "fmt"

// slashGate ties a command to a Labs flag. A gated command is hidden from
// /help, completion, and the command palette while its flag is off, and
// answers with a pointer to the flag instead of running.
type slashGate struct {
	lab     string
	enabled func() bool
}

// RegisterLabs registers a command that only runs while its Labs flag is
// on. lab is the flag's key, named in the off-state answer; enabled is
// read at call time, so a flag changed in Settings takes effect without
// re-registering.
func (sr *SlashRegistry) RegisterLabs(name, description, lab string, enabled func() bool, handler SlashHandler) {
	sr.Register(name, description, handler)
	sr.gates[name] = slashGate{lab: lab, enabled: enabled}
}

// visible reports whether a command is shown to the user. Ungated
// commands always are; a gated one only while its flag is on.
func (sr *SlashRegistry) visible(name string) bool {
	gate, gated := sr.gates[name]
	return !gated || gate.enabled()
}

// gatedOffMessage is the answer to a gated command whose flag is off.
func (sr *SlashRegistry) gatedOffMessage(name string) (string, bool) {
	gate, gated := sr.gates[name]
	if !gated || gate.enabled() {
		return "", false
	}
	return fmt.Sprintf("Labs: enable %s in Settings > Labs to use /%s.", gate.lab, name), true
}
