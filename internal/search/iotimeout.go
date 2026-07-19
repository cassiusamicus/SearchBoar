package search

import (
	"context"
	"fmt"
	"time"
)

// fileIOTimeout bounds how long a single file's content read/extraction may
// take before it's treated as a failure (same value already used for the
// pdftotext subprocess in extractPDFText) -- long enough for a legitimately
// large or slow-but-working file, short enough that one bad file can't tie
// up a worker indefinitely.
const fileIOTimeout = 30 * time.Second

// withIOTimeout runs fn (a blocking filesystem call -- os.Open, ReadFile,
// zip.OpenReader, etc.) and gives up waiting for it after fileIOTimeout or
// ctx cancellation, whichever comes first.
//
// This matters because Go's os/io APIs have no way to cancel an in-flight
// blocking syscall via context: a read() that stalls (the classic case is a
// degraded or wedged network mount -- an SMB/NFS share that's still
// mounted but not responding) simply never returns, and neither ctx
// cancellation nor the app's Stop button can unstick it once a worker is
// inside that call. Without this wrapper, one such file freezes that
// worker forever, and if enough workers hit the same stalled mount the
// whole search (and the app, since there's no other way to interrupt it)
// has to be force-killed.
//
// fn's own goroutine is deliberately left running if it never returns --
// there is no way in Go to forcibly abort a blocked syscall, so the
// alternative would be blocking the caller just the same. Giving up
// promptly and letting the search move on to the next file is strictly
// better than freezing everything, even though it leaks one goroutine per
// timed-out file.
func withIOTimeout[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	return withTimeout(ctx, fileIOTimeout, fn)
}

// withTimeout is withIOTimeout with an explicit duration, split out so
// tests can use a short timeout instead of waiting out the real
// fileIOTimeout.
func withTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error)) (T, error) {
	ch := make(chan struct {
		val T
		err error
	}, 1)
	go func() {
		v, err := fn()
		ch <- struct {
			val T
			err error
		}{v, err}
	}()
	select {
	case r := <-ch:
		return r.val, r.err
	case <-time.After(timeout):
		var zero T
		return zero, fmt.Errorf("timed out after %s reading file (possibly a stalled network mount)", timeout)
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
