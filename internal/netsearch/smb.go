package netsearch

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

var smbHostRe = regexp.MustCompile(`Host:\s+(\d+\.\d+\.\d+\.\d+)`)

// discoverSMBHosts scans cidr for hosts with port 445 open. nmap (a full
// port scan) is used when available; when it's missing, this falls back to
// probeSMBHostsTCP instead of silently yielding zero hosts, so "no nmap"
// and "no SMB hosts on this network" are no longer indistinguishable to
// the caller (see usedFallback).
func discoverSMBHosts(ctx context.Context, cidr string) (hosts []string, usedFallback bool, err error) {
	if _, lookErr := exec.LookPath("nmap"); lookErr != nil {
		hosts, err := probeSMBHostsTCP(ctx, cidr)
		return hosts, true, err
	}

	cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "nmap", "-p", "445", "--open", "-oG", "-", cidr).Output()
	if err != nil {
		return nil, false, err
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "Status: Up") {
			continue
		}
		if m := smbHostRe.FindStringSubmatch(line); m != nil {
			hosts = append(hosts, m[1])
		}
	}
	return hosts, false, nil
}

// probeSMBHostsTCP is the no-nmap fallback: a bounded, concurrent
// TCP-connect probe of port 445 across cidr (capped at maxNFSHostsScanned
// hosts, the same bound nfs.go's brute-force NFS scan already uses -- nmap
// does a real port scan across the whole range, but a plain TCP dial per
// host doesn't scale to a full /16 the way nmap does).
func probeSMBHostsTCP(ctx context.Context, cidr string) ([]string, error) {
	candidates, err := hostsInCIDR(cidr, maxNFSHostsScanned)
	if err != nil {
		return nil, err
	}

	type probeResult struct {
		host string
		up   bool
	}
	results := make(chan probeResult, len(candidates))
	sem := make(chan struct{}, 32)
	for _, host := range candidates {
		host := host
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			d := net.Dialer{Timeout: 500 * time.Millisecond}
			conn, dialErr := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "445"))
			if dialErr == nil {
				conn.Close()
			}
			results <- probeResult{host: host, up: dialErr == nil}
		}()
	}

	var hosts []string
	for range candidates {
		r := <-results
		if r.up {
			hosts = append(hosts, r.host)
		}
	}
	sort.Strings(hosts)
	return hosts, nil
}

// smbShareTypes are the Type column values a real share table row can
// have -- used to reject the "---------  ----  -------" divider row right
// under the header (Type "----") and any informational line smbclient
// prints into the same stdout stream outside the table itself, e.g.
// "Reconnecting with SMB1 for workgroup listing." on newer smbclient
// versions (Type would be "with", also not a real type). A row's actual
// share name is free-form text and can't be filtered on directly, but the
// Type column only ever holds one of these.
var smbShareTypes = map[string]bool{"Disk": true, "IPC": true, "Printer": true, "Print": true}

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
	return parseSMBShareListOutput(out), nil
}

// parseSMBShareListOutput parses smbclient -L's share table, dropping the
// administrative IPC$/print$ shares. Filters each row's Type column
// against smbShareTypes rather than just taking the first field on any
// non-blank line: the "---------  ----  -------" divider directly under
// the header, and (on some smbclient versions) an informational
// "Reconnecting with SMB1 for workgroup listing." line printed into the
// same stdout stream, both have a first field that reads like a share name
// otherwise.
func parseSMBShareListOutput(out []byte) []string {
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
		if len(fields) < 2 || !smbShareTypes[fields[1]] {
			continue
		}
		name := fields[0]
		lower := strings.ToLower(name)
		if lower == "ipc$" || lower == "print$" {
			continue
		}
		shares = append(shares, name)
	}
	return shares
}

// smbEntry is one file or directory listed by smbclient's ls command.
type smbEntry struct {
	Name  string
	IsDir bool
}

// smbclient's `ls` output has drifted slightly in column width across
// Samba versions and locales, but the field order is stable: a name field,
// an attribute-letter run (some subset of A/D/H/S/R/N), a byte size, then
// a ctime-style date. Parsing right-to-left off that fixed order (rather
// than assuming fixed column offsets) survives that drift. This does mean
// a file whose name is long enough to overflow the name field's padding
// *and* happens to end in attribute letters with no separating space could
// be mis-split -- accepted as a rare edge case rather than something worth
// a fragile fixed-width parser to avoid.
var (
	smbLsDateRe = regexp.MustCompile(`\s+[A-Za-z]{3} [A-Za-z]{3}\s+\d{1,2} \d{2}:\d{2}:\d{2} \d{4}\s*$`)
	smbLsSizeRe = regexp.MustCompile(`\s*\d+\s*$`)
	smbLsAttrRe = regexp.MustCompile(`\s+([ADHSRN]{1,6})\s*$`)
)

// parseSMBLsOutput parses smbclient's `ls`/`dir` output into entries,
// skipping "."/".." and the blank lines and trailing "N blocks of size..."
// summary line that aren't real directory entries (none of those have a
// trailing date, which is what smbLsDateRe requires to consider a line at
// all).
func parseSMBLsOutput(out []byte) []smbEntry {
	var entries []smbEntry
	for _, line := range strings.Split(string(out), "\n") {
		if !smbLsDateRe.MatchString(line) {
			continue
		}
		rest := smbLsDateRe.ReplaceAllString(line, "")
		if !smbLsSizeRe.MatchString(rest) {
			continue
		}
		rest = smbLsSizeRe.ReplaceAllString(rest, "")

		isDir := false
		if m := smbLsAttrRe.FindStringSubmatchIndex(rest); m != nil {
			isDir = strings.Contains(rest[m[2]:m[3]], "D")
			rest = rest[:m[0]]
		}

		name := strings.TrimSpace(rest)
		if name == "" || name == "." || name == ".." {
			continue
		}
		entries = append(entries, smbEntry{Name: name, IsDir: isDir})
	}
	return entries
}

// listSMBDir lists the immediate contents of subpath within host/share via
// smbclient's non-interactive -c command mode. Unlike mounting, this needs
// no privilege escalation, so a share's subfolders can be browsed (e.g. to
// pick a specific one to search) before deciding to search anything at all.
func listSMBDir(ctx context.Context, host, share, subpath, user, pass string) ([]smbEntry, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	smbclientCmd := "ls"
	if subpath != "" {
		// smbclient paths use backslashes; our own share-relative paths
		// are "/"-joined (see sharepicker.go), so translate here.
		smbclientCmd = `cd "` + strings.ReplaceAll(subpath, "/", `\`) + `"; ls`
	}

	var cmd *exec.Cmd
	if user == "" {
		cmd = exec.CommandContext(cctx, "smbclient", "//"+host+"/"+share, "-N", "-c", smbclientCmd)
	} else {
		cmd = exec.CommandContext(cctx, "smbclient", "//"+host+"/"+share, "-U", user, "-c", smbclientCmd)
		cmd.Stdin = strings.NewReader(pass + "\n")
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseSMBLsOutput(out), nil
}
