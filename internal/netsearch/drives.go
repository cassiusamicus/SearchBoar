package netsearch

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Drive is one locally mounted filesystem worth offering in the drive
// picker.
type Drive struct {
	MountPoint string
	Device     string
	Kind       string // "Internal", "Removable", "SD Card"
	Label      string // "<mountpoint> (<Kind>)"
}

// pseudoFilesystems are mounts that are never real user data and should
// never appear in the drive picker.
var pseudoFilesystems = map[string]bool{
	"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true,
	"tmpfs": true, "cgroup": true, "cgroup2": true, "pstore": true,
	"securityfs": true, "debugfs": true, "tracefs": true, "configfs": true,
	"fusectl": true, "mqueue": true, "hugetlbfs": true, "autofs": true,
	"binfmt_misc": true, "rpc_pipefs": true, "overlay": true, "squashfs": true,
}

// excludedMountPrefixes are real-filesystem mounts that are still not
// useful search targets.
var excludedMountPrefixes = []string{
	"/proc", "/sys", "/dev", "/run", "/boot", "/snap", "/var/lib/docker",
}

// DetectLocalDrives enumerates real, user-relevant local mount points from
// /proc/mounts, classifying each as Internal/Removable/SD Card via
// /sys/class/block's "removable" attribute and device naming convention.
func DetectLocalDrives() ([]Drive, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var drives []Drive
	seen := map[string]bool{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mountPoint, fstype := fields[0], fields[1], fields[2]
		mountPoint = unescapeMountPath(mountPoint)

		if pseudoFilesystems[fstype] || !strings.HasPrefix(device, "/dev/") {
			continue
		}
		if isExcludedMount(mountPoint) || seen[mountPoint] {
			continue
		}
		seen[mountPoint] = true

		kind := classifyDevice(device)
		drives = append(drives, Drive{
			MountPoint: mountPoint,
			Device:     device,
			Kind:       kind,
			Label:      mountPoint + " (" + kind + ")",
		})
	}

	sort.Slice(drives, func(i, j int) bool { return drives[i].MountPoint < drives[j].MountPoint })
	return drives, scanner.Err()
}

func isExcludedMount(mountPoint string) bool {
	if mountPoint != "/" {
		for _, prefix := range excludedMountPrefixes {
			if mountPoint == prefix || strings.HasPrefix(mountPoint, prefix+"/") {
				return true
			}
		}
	}
	return false
}

// unescapeMountPath reverses /proc/mounts' octal escaping of spaces and
// other special characters (e.g. "\040" -> " ").
func unescapeMountPath(p string) string {
	if !strings.Contains(p, "\\") {
		return p
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' && i+3 < len(p) {
			if v := (int(p[i+1]-'0')*64 + int(p[i+2]-'0')*8 + int(p[i+3]-'0')); v >= 0 && v < 256 {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(p[i])
	}
	return b.String()
}

// classifyDevice labels a device as Internal, Removable, or SD Card using
// /sys/class/block/<name>/removable, walking up from a partition (e.g.
// sda1) to its parent disk (sda) if the partition itself has no such
// attribute.
func classifyDevice(device string) string {
	name := strings.TrimPrefix(device, "/dev/")
	if strings.HasPrefix(name, "mmcblk") {
		return "SD Card"
	}

	removable := readRemovableFlag(name)
	if removable == "" {
		parent := parentBlockDevice(name)
		removable = readRemovableFlag(parent)
	}
	if removable == "1" {
		return "Removable"
	}
	return "Internal"
}

func readRemovableFlag(name string) string {
	if name == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join("/sys/class/block", name, "removable"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// parentBlockDevice strips a trailing partition number, e.g. "sda1" -> "sda",
// "nvme0n1p1" -> "nvme0n1".
func parentBlockDevice(name string) string {
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i == len(name) || i == 0 {
		return ""
	}
	if name[i-1] == 'p' && i > 1 && name[i-2] >= '0' && name[i-2] <= '9' {
		// nvme-style "p<N>" partition suffix
		return name[:i-1]
	}
	if strings.HasPrefix(name, "nvme") {
		return "" // "p" already stripped above; anything else isn't a known partition scheme
	}
	return name[:i]
}
