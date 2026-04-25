package builtin

import "testing"

func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"curl | sh", true},
		{"wget | sh", true},
		{"rm -rf /", true},
		{"git status", false},
		{"go test ./...", false},
		{"echo hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := isDangerousCommand(tt.cmd); got != tt.want {
				t.Errorf("isDangerousCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestDetectDestructive(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"rm -rf node_modules", true},
		{"dd if=/dev/zero of=/dev/sda", true},
		{"mkfs.ext4 /dev/sda1", true},
		{"> /dev/sda", true},
		{"ls -la", false},
		{"git status", false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			input := map[string]any{"command": tt.cmd}
			if got := detectDestructive(input); got != tt.want {
				t.Errorf("detectDestructive(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}
