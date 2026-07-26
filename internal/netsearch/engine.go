package netsearch

import (
	"context"
	"fmt"
	"sort"
)

// Engine discovers and mounts search locations (local drives, SMB shares,
// NFS exports).
type Engine struct {
	Mounts *MountManager
}

func NewEngine() *Engine {
	return &Engine{Mounts: NewMountManager(NewElevator())}
}

// ResolveRoots discovers local drive locations and mounts every explicitly
// selected SMB share / NFS export (see LocationOptions -- there is no
// blind "mount everything discovered" fallback for network locations; use
// DiscoverSMB/DiscoverNFS to populate a picker first). All network mounts
// happen in a single batch (one privilege prompt) via MountManager.MountJobs.
// Mounts are NOT unmounted here -- see MountManager's doc comment.
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

	var jobs []MountJob
	if opts.SearchSMB {
		for _, s := range opts.SelectedSMBShares {
			label := fmt.Sprintf("//%s/%s", s.Host, s.Share)
			jobs = append(jobs, MountJob{Kind: "cifs", Source: label, Label: label})
		}
	}
	if opts.SearchNFS {
		for _, ex := range opts.SelectedNFSExports {
			label := fmt.Sprintf("%s:%s", ex.Host, ex.Export)
			jobs = append(jobs, MountJob{Kind: "nfs", Source: label, Label: label})
		}
	}
	if len(jobs) == 0 {
		return roots, nil
	}
	if ctx.Err() != nil {
		return roots, ctx.Err()
	}

	log("INFO", fmt.Sprintf("Mounting %d network location(s)...", len(jobs)))
	mounted, err := e.Mounts.MountJobs(ctx, jobs, opts.Username, opts.Password)
	if err != nil {
		log("ERROR", "mount failed: "+err.Error())
	}
	for label, mountPoint := range mounted {
		roots = append(roots, ResolvedRoot{Path: mountPoint, DisplayPrefix: label})
	}
	log("INFO", fmt.Sprintf("%d of %d network location(s) mounted", len(mounted), len(jobs)))

	return roots, ctx.Err()
}

// DiscoverSMB scans cidr for SMB hosts and lists their shares, for
// populating the Workspace Builder tab's share picker. It does not mount
// anything. usedFallback reports whether nmap was unavailable and a
// slower TCP-connect probe was used instead (see discoverSMBHosts), so the
// caller can surface that distinction rather than leaving "0 found"
// ambiguous between "no shares" and "no nmap."
func (e *Engine) DiscoverSMB(ctx context.Context, cidr, user, pass string) (shares []SMBShare, usedFallback bool, err error) {
	hosts, usedFallback, err := discoverSMBHosts(ctx, cidr)
	if err != nil {
		return nil, usedFallback, err
	}

	for _, host := range hosts {
		if ctx.Err() != nil {
			return shares, usedFallback, ctx.Err()
		}
		list, err := listSMBShares(ctx, host, user, pass)
		if err != nil {
			continue // one unreachable/unauthenticated host shouldn't abort the whole scan
		}
		for _, s := range list {
			shares = append(shares, SMBShare{Host: host, Share: s})
		}
	}
	return shares, usedFallback, nil
}

// ListSMBDir lists the immediate subdirectories of subpath within an SMB
// share, for the Workspace Builder tab's expandable share tree. Unlike
// ResolveRoots, this never mounts anything (see listSMBDir), so browsing a
// share's folder structure needs no privilege escalation.
func (e *Engine) ListSMBDir(ctx context.Context, host, share, subpath, user, pass string) ([]string, error) {
	entries, err := listSMBDir(ctx, host, share, subpath, user, pass)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir {
			dirs = append(dirs, entry.Name)
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// DiscoverNFS scans cidr for NFS exports, for populating the Search
// Locations tab's share picker. It does not mount anything.
func (e *Engine) DiscoverNFS(ctx context.Context, cidr string) ([]NFSExport, error) {
	hosts, err := hostsInCIDR(cidr, maxNFSHostsScanned)
	if err != nil {
		return nil, err
	}

	var exports []NFSExport
	for _, host := range hosts {
		if ctx.Err() != nil {
			return exports, ctx.Err()
		}
		list, err := discoverNFSExports(ctx, host)
		if err != nil {
			continue // most scanned hosts won't run NFS at all
		}
		for _, ex := range list {
			exports = append(exports, NFSExport{Host: host, Export: ex})
		}
	}
	return exports, nil
}
