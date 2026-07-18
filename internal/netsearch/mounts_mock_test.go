package netsearch

import (
	"context"
	"os"
	"strings"
	"testing"
)

// capturingElevator records the script it was asked to run, but never
// actually mounts anything -- for testing MountJobs' request-building and
// cleanup behavior without needing root or real network shares.
type capturingElevator struct {
	scriptPath string
	scriptBody string
}

func (e *capturingElevator) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "sh" && len(args) == 1 {
		e.scriptPath = args[0]
		body, _ := os.ReadFile(args[0])
		e.scriptBody = string(body)
	}
	return nil, nil
}

func TestMountJobsEmptyIsNoOp(t *testing.T) {
	m := NewMountManager(&capturingElevator{})
	results, err := m.MountJobs(context.Background(), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil results for no jobs, got %v", results)
	}
}

func TestMountJobsBuildsScriptAndCleansUpOnFailure(t *testing.T) {
	elevator := &capturingElevator{}
	m := NewMountManager(elevator)

	jobs := []MountJob{
		{Kind: "cifs", Source: "//host/share1", Label: "//host/share1"},
		{Kind: "nfs", Source: "host:/export1", Label: "host:/export1"},
	}
	results, err := m.MountJobs(context.Background(), jobs, "alice", "s3cret")
	if err != nil {
		t.Fatal(err)
	}

	// The capturing elevator never actually mounts anything, so /proc/mounts
	// won't show our temp mount points as mounted -- MountJobs should treat
	// every job as failed and clean up (no leaked temp dirs), matching real
	// behavior when a mount genuinely fails.
	if len(results) != 0 {
		t.Errorf("expected no successful mounts against a non-mounting elevator, got %v", results)
	}

	if !strings.Contains(elevator.scriptBody, "mount -t cifs") {
		t.Errorf("script missing cifs mount line: %s", elevator.scriptBody)
	}
	if !strings.Contains(elevator.scriptBody, "mount -t nfs") {
		t.Errorf("script missing nfs mount line: %s", elevator.scriptBody)
	}
	if !strings.Contains(elevator.scriptBody, "'//host/share1'") {
		t.Errorf("script missing quoted cifs source: %s", elevator.scriptBody)
	}
	if !strings.Contains(elevator.scriptBody, "credentials=") {
		t.Errorf("script missing credentials= option for authenticated cifs mount: %s", elevator.scriptBody)
	}

	// The script file itself should have been cleaned up (it's created
	// under a defer os.Remove in MountJobs).
	if _, err := os.Stat(elevator.scriptPath); !os.IsNotExist(err) {
		t.Errorf("expected mount script to be removed after use, stat err = %v", err)
	}
}
