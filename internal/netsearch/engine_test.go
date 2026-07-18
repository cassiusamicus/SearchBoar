package netsearch

import (
	"context"
	"testing"
)

func TestResolveRootsLocalExplicit(t *testing.T) {
	e := NewEngine()
	var logs []string
	roots, err := e.ResolveRoots(context.Background(), LocationOptions{
		LocalRoots:  []string{"/a", "/b"},
		SearchLocal: true,
	}, func(level, msg string) { logs = append(logs, level+": "+msg) })
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 || roots[0].Path != "/a" || roots[1].Path != "/b" {
		t.Errorf("got %+v, want explicit roots /a and /b", roots)
	}
	for _, r := range roots {
		if r.DisplayPrefix != "" {
			t.Errorf("local root %+v should have no DisplayPrefix", r)
		}
	}
}

func TestResolveRootsNoLocationsRequested(t *testing.T) {
	e := NewEngine()
	roots, err := e.ResolveRoots(context.Background(), LocationOptions{}, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 0 {
		t.Errorf("expected no roots when nothing is requested, got %+v", roots)
	}
}
