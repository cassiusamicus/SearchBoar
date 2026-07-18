package netsearch

import (
	"context"
	"os/exec"
)

// Elevator runs a single privileged command. Unlike the original LanSearch
// (which self-elevated the whole GUI process via pkexec/sudo before
// showing any results), only mount/umount go through this -- the GUI
// itself always runs as the normal user, which is safer and avoids
// running a GPU-accelerated GUI as root under Wayland.
type Elevator interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type cmdElevator struct{ bin string }

// NewElevator prefers pkexec (PolicyKit, gives a proper GUI password
// prompt) and falls back to sudo if pkexec isn't installed.
func NewElevator() Elevator {
	if p, err := exec.LookPath("pkexec"); err == nil {
		return &cmdElevator{bin: p}
	}
	return &cmdElevator{bin: "sudo"}
}

func (e *cmdElevator) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	full := append([]string{name}, args...)
	cmd := exec.CommandContext(ctx, e.bin, full...)
	return cmd.CombinedOutput()
}
