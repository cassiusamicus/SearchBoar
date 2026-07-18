package netsearch

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// MountManager tracks live CIFS/NFS mounts so they can be unmounted as a
// batch. Mounts intentionally persist until UnmountAll is called (on
// Clear Results, before starting a new search, or at app exit) rather
// than immediately after each search completes -- the original app tore
// mounts down right after searching, so "Open File" on a result could
// fail because the share was already unmounted by the time the user
// clicked it.
type MountManager struct {
	elevator Elevator

	mu    sync.Mutex
	unmnt map[string]func(context.Context) error
}

func NewMountManager(elevator Elevator) *MountManager {
	return &MountManager{elevator: elevator, unmnt: map[string]func(context.Context) error{}}
}

// MountJob is one share/export to mount, identified by Label (used both as
// the map key in MountJobs' result and as the display prefix results found
// under it should show).
type MountJob struct {
	Kind   string // "cifs" or "nfs"
	Source string // "//host/share" for cifs, "host:export" for nfs
	Label  string
}

// MountJobs mounts every job in a single elevated batch -- one pkexec/sudo
// prompt covers the whole set, no matter how many shares/exports there
// are, instead of one prompt per share (which is unworkable on a LAN with
// even a modest number of discovered shares). Returns mount points for
// jobs that succeeded, keyed by Label; failed jobs are silently omitted
// (a share going away between discovery and search, or a bad password, is
// routine and shouldn't abort the rest of the batch).
func (m *MountManager) MountJobs(ctx context.Context, jobs []MountJob, cifsUser, cifsPass string) (map[string]string, error) {
	if len(jobs) == 0 {
		return nil, nil
	}

	type prepared struct {
		job        MountJob
		mountPoint string
		credFile   string
	}
	var preparedJobs []prepared
	defer func() {
		for _, p := range preparedJobs {
			if p.credFile != "" {
				os.Remove(p.credFile)
			}
		}
	}()

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	for _, job := range jobs {
		prefix := "searchboar-nfs-"
		if job.Kind == "cifs" {
			prefix = "searchboar-smb-"
		}
		mountPoint, err := os.MkdirTemp("", prefix)
		if err != nil {
			continue
		}
		p := prepared{job: job, mountPoint: mountPoint}

		var opts string
		switch job.Kind {
		case "cifs":
			if cifsUser != "" {
				credFile, err := writeCredentialsFile(cifsUser, cifsPass)
				if err != nil {
					os.Remove(mountPoint)
					continue
				}
				p.credFile = credFile
				opts = "credentials=" + credFile + ",ro,vers=3.0"
			} else {
				opts = "guest,ro,vers=3.0"
			}
		case "nfs":
			opts = "ro"
		default:
			os.Remove(mountPoint)
			continue
		}

		fmt.Fprintf(&script, "mount -t %s %s %s -o %s\n",
			job.Kind, shellQuote(job.Source), shellQuote(mountPoint), shellQuote(opts))
		preparedJobs = append(preparedJobs, p)
	}
	if len(preparedJobs) == 0 {
		return nil, nil
	}

	scriptFile, err := os.CreateTemp("", "searchboar-mount-*.sh")
	if err != nil {
		for _, p := range preparedJobs {
			os.Remove(p.mountPoint)
		}
		return nil, err
	}
	scriptPath := scriptFile.Name()
	defer os.Remove(scriptPath)
	if _, err := scriptFile.WriteString(script.String()); err != nil {
		scriptFile.Close()
		for _, p := range preparedJobs {
			os.Remove(p.mountPoint)
		}
		return nil, err
	}
	scriptFile.Close()

	// Best-effort: even if the script's overall exit code is non-zero
	// (one bad mount among many), successful mounts are still verified
	// individually below via /proc/mounts.
	m.elevator.Run(ctx, "sh", scriptPath)

	mounted := readMountedPaths()
	results := make(map[string]string, len(preparedJobs))
	for _, p := range preparedJobs {
		if mounted[p.mountPoint] {
			results[p.job.Label] = p.mountPoint
			m.track(p.mountPoint)
		} else {
			os.Remove(p.mountPoint)
		}
	}
	return results, nil
}

// readMountedPaths reads /proc/mounts (world-readable, no privilege
// needed) to verify which of our candidate mount points actually mounted,
// since the batch script's own exit code doesn't distinguish which
// individual mount(s) within it failed.
func readMountedPaths() map[string]bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			out[fields[1]] = true
		}
	}
	return out
}

// shellQuote wraps s in single quotes for safe use in a POSIX shell
// command, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (m *MountManager) track(mountPoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unmnt[mountPoint] = func(ctx context.Context) error {
		_, err := m.elevator.Run(ctx, "umount", mountPoint)
		os.Remove(mountPoint)
		return err
	}
}

// UnmountAll unmounts and cleans up every currently tracked mount.
func (m *MountManager) UnmountAll(ctx context.Context) {
	m.mu.Lock()
	pending := m.unmnt
	m.unmnt = map[string]func(context.Context) error{}
	m.mu.Unlock()

	for _, unmount := range pending {
		unmount(ctx)
	}
}

func writeCredentialsFile(user, pass string) (string, error) {
	f, err := os.CreateTemp("", "searchboar-cred-")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return "", err
	}
	if _, err := f.WriteString("username=" + user + "\npassword=" + pass + "\n"); err != nil {
		return "", err
	}
	return f.Name(), nil
}
