package netsearch

import (
	"context"
	"fmt"
	"os"
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

// MountCIFS mounts //host/share read-only via mount.cifs, using a
// credentials file (0600, deleted immediately after the mount call
// returns) rather than putting the password in -o argv, where it would be
// visible to any local user via `ps`/`/proc/<pid>/cmdline` -- a real
// security issue in the original app.
func (m *MountManager) MountCIFS(ctx context.Context, host, share, user, pass string) (string, error) {
	mountPoint, err := os.MkdirTemp("", "searchboar-smb-")
	if err != nil {
		return "", err
	}

	var opts string
	if user != "" {
		credFile, err := writeCredentialsFile(user, pass)
		if err != nil {
			os.Remove(mountPoint)
			return "", err
		}
		defer os.Remove(credFile)
		opts = "credentials=" + credFile + ",ro,vers=3.0"
	} else {
		opts = "guest,ro,vers=3.0"
	}

	source := fmt.Sprintf("//%s/%s", host, share)
	out, err := m.elevator.Run(ctx, "mount", "-t", "cifs", source, mountPoint, "-o", opts)
	if err != nil {
		os.Remove(mountPoint)
		return "", fmt.Errorf("%w: %s", err, string(out))
	}

	m.track(mountPoint)
	return mountPoint, nil
}

// MountNFS mounts host:export read-only.
func (m *MountManager) MountNFS(ctx context.Context, host, export string) (string, error) {
	mountPoint, err := os.MkdirTemp("", "searchboar-nfs-")
	if err != nil {
		return "", err
	}

	source := host + ":" + export
	out, err := m.elevator.Run(ctx, "mount", "-t", "nfs", source, mountPoint, "-o", "ro")
	if err != nil {
		os.Remove(mountPoint)
		return "", fmt.Errorf("%w: %s", err, string(out))
	}

	m.track(mountPoint)
	return mountPoint, nil
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
