package netsearch

import (
	"bufio"
	"bytes"
	"context"
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
