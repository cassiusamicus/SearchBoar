package netsearch

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var smbHostRe = regexp.MustCompile(`Host:\s+(\d+\.\d+\.\d+\.\d+)`)

// discoverSMBHosts scans cidr for hosts with port 445 open via nmap.
// nmap is optional (like the original app); if it's missing, discovery
// simply yields no hosts rather than erroring.
func discoverSMBHosts(ctx context.Context, cidr string) ([]string, error) {
	if _, err := exec.LookPath("nmap"); err != nil {
		return nil, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "nmap", "-p", "445", "--open", "-oG", "-", cidr).Output()
	if err != nil {
		return nil, err
	}

	var hosts []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "Status: Up") {
			continue
		}
		if m := smbHostRe.FindStringSubmatch(line); m != nil {
			hosts = append(hosts, m[1])
		}
	}
	return hosts, nil
}

// listSMBShares enumerates shares on host via `smbclient -L`, dropping the
// administrative IPC$/print$ shares, matching the original app.
func listSMBShares(ctx context.Context, host, user, pass string) ([]string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if user == "" {
		cmd = exec.CommandContext(cctx, "smbclient", "-L", "//"+host, "-N")
	} else {
		cmd = exec.CommandContext(cctx, "smbclient", "-L", "//"+host, "-U", user)
		cmd.Stdin = strings.NewReader(pass + "\n")
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var shares []string
	inShares := false
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Sharename") {
			inShares = true
			continue
		}
		if !inShares {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			break
		}
		name := fields[0]
		lower := strings.ToLower(name)
		if lower == "ipc$" || lower == "print$" {
			continue
		}
		shares = append(shares, name)
	}
	return shares, scanner.Err()
}

func (e *Engine) searchSMB(ctx context.Context, opts Options, cidr string, results chan<- Result, log func(level, msg string)) error {
	hosts, err := discoverSMBHosts(ctx, cidr)
	if err != nil {
		log("WARN", "SMB host discovery failed: "+err.Error())
	}
	if len(hosts) == 0 {
		log("INFO", "No SMB hosts found (install nmap for host discovery, or none have port 445 open)")
		return nil
	}

	for _, host := range hosts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		shares, err := listSMBShares(ctx, host, opts.Username, opts.Password)
		if err != nil {
			log("WARN", fmt.Sprintf("could not list shares on %s: %v", host, err))
			continue
		}
		for _, share := range shares {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			mountPoint, err := e.Mounts.MountCIFS(ctx, host, share, opts.Username, opts.Password)
			if err != nil {
				log("WARN", fmt.Sprintf("mount //%s/%s failed: %v", host, share, err))
				continue
			}
			log("INFO", fmt.Sprintf("Searching //%s/%s", host, share))
			prefix := fmt.Sprintf("//%s/%s", host, share)
			if err := walkAndMatch(ctx, mountPoint, opts.Pattern, prefix, results); err != nil && ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return nil
}
