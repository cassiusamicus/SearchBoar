package netsearch

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// maxNFSHostsScanned caps a brute-force `showmount -e` scan of a CIDR
// range, matching the original app's hard cap of 20 hosts.
const maxNFSHostsScanned = 20

func discoverNFSExports(ctx context.Context, host string) ([]string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cctx, "showmount", "-e", host).Output()
	if err != nil {
		return nil, err
	}

	var exports []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue // header line, e.g. "Export list for host:"
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		exports = append(exports, fields[0])
	}
	return exports, scanner.Err()
}

func hostsInCIDR(cidr string, max int) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var hosts []string
	ip := ipnet.IP.Mask(ipnet.Mask)
	for ; ipnet.Contains(ip) && len(hosts) < max; incIP(ip) {
		hosts = append(hosts, ip.String())
	}
	return hosts, nil
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func (e *Engine) searchNFS(ctx context.Context, opts Options, cidr string, results chan<- Result, log func(level, msg string)) error {
	hosts, err := hostsInCIDR(cidr, maxNFSHostsScanned)
	if err != nil {
		log("ERROR", "invalid network range for NFS scan: "+err.Error())
		return err
	}

	for _, host := range hosts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		exports, err := discoverNFSExports(ctx, host)
		if err != nil {
			continue // most scanned hosts won't run NFS at all; not worth a log line each
		}
		for _, export := range exports {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			mountPoint, err := e.Mounts.MountNFS(ctx, host, export)
			if err != nil {
				log("WARN", fmt.Sprintf("mount %s:%s failed: %v", host, export, err))
				continue
			}
			log("INFO", fmt.Sprintf("Searching %s:%s", host, export))
			prefix := fmt.Sprintf("%s:%s", host, export)
			if err := walkAndMatch(ctx, mountPoint, opts.Pattern, prefix, results); err != nil && ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return nil
}
