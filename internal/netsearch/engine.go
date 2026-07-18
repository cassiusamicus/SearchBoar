package netsearch

import (
	"context"
	"fmt"
)

// Engine discovers and mounts search locations (local drives, SMB shares,
// NFS exports).
type Engine struct {
	Mounts *MountManager
}

func NewEngine() *Engine {
	return &Engine{Mounts: NewMountManager(NewElevator())}
}

// ResolveRoots discovers every location opts asks for, mounting SMB/NFS
// shares as it goes (mounts are NOT unmounted here -- see MountManager's
// doc comment), and returns one ResolvedRoot per location for the caller
// to search with internal/search's engine.
func (e *Engine) ResolveRoots(ctx context.Context, opts LocationOptions, log func(level, msg string)) ([]ResolvedRoot, error) {
	var roots []ResolvedRoot

	if opts.SearchLocal {
		localRoots := opts.LocalRoots
		if len(localRoots) == 0 {
			drives, err := DetectLocalDrives()
			if err != nil {
				log("ERROR", "local drive detection failed: "+err.Error())
			}
			for _, d := range drives {
				localRoots = append(localRoots, d.MountPoint)
			}
		}
		for _, r := range localRoots {
			roots = append(roots, ResolvedRoot{Path: r})
		}
	}

	if !opts.SearchSMB && !opts.SearchNFS {
		return roots, nil
	}
	if ctx.Err() != nil {
		return roots, ctx.Err()
	}

	cidr := opts.CIDR
	if cidr == "" {
		detected, err := DetectLocalCIDR()
		if err != nil {
			log("ERROR", "could not autodetect network range: "+err.Error())
			return roots, nil
		}
		log("INFO", "Autodetected network range: "+detected)
		cidr = detected
	}

	if opts.SearchSMB {
		smbRoots, err := e.resolveSMB(ctx, opts, cidr, log)
		if err != nil && ctx.Err() != nil {
			return roots, ctx.Err()
		}
		roots = append(roots, smbRoots...)
	}
	if opts.SearchNFS {
		nfsRoots, err := e.resolveNFS(ctx, cidr, log)
		if err != nil && ctx.Err() != nil {
			return roots, ctx.Err()
		}
		roots = append(roots, nfsRoots...)
	}

	return roots, nil
}

func (e *Engine) resolveSMB(ctx context.Context, opts LocationOptions, cidr string, log func(level, msg string)) ([]ResolvedRoot, error) {
	hosts, err := discoverSMBHosts(ctx, cidr)
	if err != nil {
		log("WARN", "SMB host discovery failed: "+err.Error())
	}
	if len(hosts) == 0 {
		log("INFO", "No SMB hosts found (install nmap for host discovery, or none have port 445 open)")
		return nil, nil
	}

	var roots []ResolvedRoot
	for _, host := range hosts {
		if ctx.Err() != nil {
			return roots, ctx.Err()
		}
		shares, err := listSMBShares(ctx, host, opts.Username, opts.Password)
		if err != nil {
			log("WARN", fmt.Sprintf("could not list shares on %s: %v", host, err))
			continue
		}
		for _, share := range shares {
			if ctx.Err() != nil {
				return roots, ctx.Err()
			}
			mountPoint, err := e.Mounts.MountCIFS(ctx, host, share, opts.Username, opts.Password)
			if err != nil {
				log("WARN", fmt.Sprintf("mount //%s/%s failed: %v", host, share, err))
				continue
			}
			log("INFO", fmt.Sprintf("Mounted //%s/%s", host, share))
			roots = append(roots, ResolvedRoot{Path: mountPoint, DisplayPrefix: fmt.Sprintf("//%s/%s", host, share)})
		}
	}
	return roots, nil
}

func (e *Engine) resolveNFS(ctx context.Context, cidr string, log func(level, msg string)) ([]ResolvedRoot, error) {
	hosts, err := hostsInCIDR(cidr, maxNFSHostsScanned)
	if err != nil {
		log("ERROR", "invalid network range for NFS scan: "+err.Error())
		return nil, err
	}

	var roots []ResolvedRoot
	for _, host := range hosts {
		if ctx.Err() != nil {
			return roots, ctx.Err()
		}
		exports, err := discoverNFSExports(ctx, host)
		if err != nil {
			continue // most scanned hosts won't run NFS at all; not worth a log line each
		}
		for _, export := range exports {
			if ctx.Err() != nil {
				return roots, ctx.Err()
			}
			mountPoint, err := e.Mounts.MountNFS(ctx, host, export)
			if err != nil {
				log("WARN", fmt.Sprintf("mount %s:%s failed: %v", host, export, err))
				continue
			}
			log("INFO", fmt.Sprintf("Mounted %s:%s", host, export))
			roots = append(roots, ResolvedRoot{Path: mountPoint, DisplayPrefix: fmt.Sprintf("%s:%s", host, export)})
		}
	}
	return roots, nil
}
