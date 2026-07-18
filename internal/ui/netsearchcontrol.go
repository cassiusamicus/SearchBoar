package ui

import (
	"context"
	"strconv"

	"codeberg.org/cassiusamicus/Utilities/internal/netsearch"
)

func (t *networkTab) startSearch() {
	if t.cancelSearch != nil {
		t.cancelSearch()
	}
	// Unmount before starting a new search (not after the previous one
	// finished) -- mounts are kept alive so results stay openable right up
	// until the next search or an explicit Clear Results/app exit.
	t.engine.Mounts.UnmountAll(context.Background())

	if t.patternEntry.Text == "" {
		t.setStatus("No search pattern specified")
		return
	}
	if !t.localCheck.Checked && !t.smbCheck.Checked && !t.nfsCheck.Checked {
		t.setStatus("No search location selected")
		return
	}

	roots, excludes := t.picker.selectedRootsAndExcludes()
	opts := netsearch.Options{
		Pattern:     t.patternEntry.Text,
		LocalRoots:  roots,
		ExcludeDirs: excludes,
		SearchLocal: t.localCheck.Checked,
		SearchSMB:   t.smbCheck.Checked,
		SearchNFS:   t.nfsCheck.Checked,
		CIDR:        t.cidrEntry.Text,
		Username:    t.userEntry.Text,
		Password:    t.passEntry.Text,
	}

	t.results = nil
	t.order = nil
	t.logs = nil
	t.table.Refresh()
	t.logView.Refresh()

	t.searchBtn.Disable()
	t.stopBtn.Enable()
	t.progressBar.Show()
	t.progressBar.SetValue(0)
	t.setStatus("Searching...")

	ctx, cancel := context.WithCancel(context.Background())
	t.cancelSearch = cancel

	results := make(chan netsearch.Result)
	logs := make(chan netsearch.LogLine)
	done := make(chan error, 1)

	go func() { done <- t.engine.Run(ctx, opts, results, logs) }()
	go t.consumeSearch(results, logs, done)
}

func (t *networkTab) stopSearch() {
	if t.cancelSearch != nil {
		t.cancelSearch()
	}
}

func (t *networkTab) consumeSearch(results <-chan netsearch.Result, logs <-chan netsearch.LogLine, done <-chan error) {
	resultsOpen, logsOpen := true, true
	for resultsOpen || logsOpen {
		select {
		case r, ok := <-results:
			if !ok {
				resultsOpen = false
				continue
			}
			runOnUI(func() {
				t.results = append(t.results, r)
				t.resort()
			})
		case l, ok := <-logs:
			if !ok {
				logsOpen = false
				continue
			}
			runOnUI(func() {
				t.logs = append(t.logs, l)
				t.logView.Refresh()
				t.logView.ScrollToBottom()
			})
		}
	}

	err := <-done
	runOnUI(func() {
		t.searchBtn.Enable()
		t.stopBtn.Disable()
		t.progressBar.Hide()
		t.cancelSearch = nil
		suffix := " file(s) found"
		switch {
		case err != nil && err != context.Canceled:
			t.setStatus("Search error: " + err.Error())
		case err == context.Canceled:
			t.setStatus("Search stopped: " + strconv.Itoa(len(t.results)) + suffix)
		default:
			t.setStatus("Search complete: " + strconv.Itoa(len(t.results)) + suffix)
		}
	})
}
