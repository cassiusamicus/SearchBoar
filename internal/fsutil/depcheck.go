package fsutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// PackageManager identifies the detected system package manager.
type PackageManager string

const (
	PkgApt     PackageManager = "apt"
	PkgPacman  PackageManager = "pacman"
	PkgDnf     PackageManager = "dnf"
	PkgZypper  PackageManager = "zypper"
	PkgUnknown PackageManager = "unknown"
)

// DetectPackageManager probes PATH for a known package manager, in the
// same priority order the original app used.
func DetectPackageManager() (PackageManager, string) {
	if _, err := exec.LookPath("apt-get"); err == nil {
		return PkgApt, "Debian/Ubuntu/MX Linux"
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return PkgApt, "Debian/Ubuntu/MX Linux"
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return PkgPacman, "Arch/Artix/EndeavourOS"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return PkgDnf, "Fedora"
	}
	if _, err := exec.LookPath("zypper"); err == nil {
		return PkgZypper, "openSUSE"
	}
	return PkgUnknown, "Unknown distribution"
}

var packageNames = map[PackageManager]map[string]string{
	PkgApt:    {"ripgrep": "ripgrep", "pdftotext": "poppler-utils"},
	PkgPacman: {"ripgrep": "ripgrep", "pdftotext": "poppler"},
	PkgDnf:    {"ripgrep": "ripgrep", "pdftotext": "poppler-utils"},
	PkgZypper: {"ripgrep": "ripgrep", "pdftotext": "poppler-tools"},
}

var installCommandTemplates = map[PackageManager]string{
	PkgApt:    "sudo apt update && sudo apt install %s",
	PkgPacman: "sudo pacman -S %s",
	PkgDnf:    "sudo dnf install %s",
	PkgZypper: "sudo zypper install %s",
}

// OptionalDependency is one optional external tool the app can use if
// present. Unlike the original Python app, PDF and DOCX text extraction
// are always available (statically linked pure-Go fallbacks), so there is
// no "missing Python library" dependency to report -- only these two
// genuinely optional speed/quality boosts remain.
type OptionalDependency struct {
	Key         string // "ripgrep" or "pdftotext"
	Label       string
	Description string
	Present     bool
}

// CheckOptionalDependencies probes for ripgrep and pdftotext on PATH.
func CheckOptionalDependencies() []OptionalDependency {
	_, rgErr := exec.LookPath("rg")
	_, ptErr := exec.LookPath("pdftotext")
	return []OptionalDependency{
		{Key: "ripgrep", Label: "ripgrep", Description: "much faster content searches", Present: rgErr == nil},
		{Key: "pdftotext", Label: "Poppler/pdftotext", Description: "fastest PDF text extraction", Present: ptErr == nil},
	}
}

// BuildInstallCommand returns a copyable install command for the given
// missing dependency keys, and a human-readable distro label.
func BuildInstallCommand(missingKeys []string) (command, distroLabel string) {
	manager, label := DetectPackageManager()
	names, ok := packageNames[manager]
	if !ok {
		return "Install ripgrep and Poppler with your system package manager.", label
	}

	seen := map[string]bool{}
	var packages []string
	for _, key := range missingKeys {
		pkg, ok := names[key]
		if !ok || seen[pkg] {
			continue
		}
		seen[pkg] = true
		packages = append(packages, pkg)
	}
	if len(packages) == 0 {
		return "", label
	}
	return fmt.Sprintf(installCommandTemplates[manager], strings.Join(packages, " ")), label
}
