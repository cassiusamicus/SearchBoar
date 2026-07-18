package netsearch

import "context"

// Engine runs network searches (local drives, SMB, NFS).
type Engine struct {
	Mounts *MountManager
}

func NewEngine() *Engine {
	return &Engine{Mounts: NewMountManager(NewElevator())}
}

// Run executes opts' selected search types in order (Local, then SMB, then
// NFS), streaming results and log lines, closing both channels when done.
// Mounts made along the way are NOT unmounted here -- see MountManager's
// doc comment; the caller decides when to call Mounts.UnmountAll.
func (e *Engine) Run(ctx context.Context, opts Options, results chan<- Result, logs chan<- LogLine) error {
	defer close(results)
	if logs != nil {
		defer close(logs)
	}

	log := func(level, msg string) {
		if logs == nil {
			return
		}
		select {
		case logs <- LogLine{Level: level, Message: msg}:
		case <-ctx.Done():
		}
	}

	if opts.SearchLocal {
		if err := e.searchLocal(ctx, opts, results, log); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log("ERROR", "local search error: "+err.Error())
		}
	}

	if opts.SearchSMB || opts.SearchNFS {
		cidr := opts.CIDR
		if cidr == "" {
			detected, err := DetectLocalCIDR()
			if err != nil {
				log("ERROR", "could not autodetect network range: "+err.Error())
				cidr = ""
			} else {
				log("INFO", "Autodetected network range: "+detected)
				cidr = detected
			}
		}
		if cidr != "" {
			if opts.SearchSMB {
				if err := e.searchSMB(ctx, opts, cidr, results, log); err != nil && ctx.Err() != nil {
					return ctx.Err()
				}
			}
			if opts.SearchNFS {
				if err := e.searchNFS(ctx, opts, cidr, results, log); err != nil && ctx.Err() != nil {
					return ctx.Err()
				}
			}
		}
	}

	log("INFO", "Search complete")
	return ctx.Err()
}
