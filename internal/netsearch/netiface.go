package netsearch

import (
	"errors"
	"net"
)

// DetectLocalCIDR finds the CIDR of the first non-loopback, up IPv4
// interface -- a native-Go replacement for the original app's
// `ip route show default` / `ip addr show <iface>` shellouts.
func DetectLocalCIDR() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			return ipnet.String(), nil
		}
	}
	return "", errors.New("no active network interface found")
}
