package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadParsesFixture(t *testing.T) {
	cfg, err := Load("testdata/sample_config.ini")
	if err != nil {
		t.Fatal(err)
	}

	wantPaths := []string{"/home/user/Documents", "/home/user/Projects", "/home/user/Downloads"}
	if !reflect.DeepEqual(cfg.Recent.Paths, wantPaths) {
		t.Errorf("Recent.Paths = %v, want %v", cfg.Recent.Paths, wantPaths)
	}

	if cfg.Window.X != 159 || cfg.Window.Y != 179 || cfg.Window.Width != 1220 || cfg.Window.Height != 600 {
		t.Errorf("Window = %+v, unexpected", cfg.Window)
	}
	if !cfg.Window.HasPosition() {
		t.Error("expected HasPosition true for x,y >= 0")
	}

	if cfg.Columns["details_file"] != 308 {
		t.Errorf("Columns[details_file] = %d, want 308", cfg.Columns["details_file"])
	}

	if len(cfg.Favorites) != 3 {
		t.Fatalf("expected 3 favorites (6/5/4-field legacy formats), got %d", len(cfg.Favorites))
	}
	byPath := map[string]FavoriteRecord{}
	for _, f := range cfg.Favorites {
		byPath[f.Filepath] = f
	}

	six := byPath["/home/user/Documents/notes.md"]
	if six.SearchTerm != "hello world" || six.Category != "Uncategorized" || six.SizeBytes != 49290 {
		t.Errorf("6-field favorite parsed wrong: %+v", six)
	}
	five := byPath["/home/user/Documents/legacy5.txt"]
	if five.SearchTerm != "term" || five.Category != "Research" {
		t.Errorf("5-field favorite parsed wrong: %+v", five)
	}
	four := byPath["/home/user/Documents/legacy4.txt"]
	if four.SearchTerm != "" || four.Category != "Uncategorized" || four.SizeBytes != 200 {
		t.Errorf("4-field favorite parsed wrong: %+v", four)
	}

	wantCats := []string{"Uncategorized", "Research"}
	if !reflect.DeepEqual(cfg.FavoriteCategoryOrder, wantCats) {
		t.Errorf("FavoriteCategoryOrder = %v, want %v", cfg.FavoriteCategoryOrder, wantCats)
	}

	ss, ok := cfg.FavoriteSearches["MySearch"]
	if !ok || ss.FilePattern != "*.md" || ss.ContentPattern != "epicurus" {
		t.Errorf("FavoriteSearches[MySearch] = %+v ok=%v, unexpected", ss, ok)
	}

	if cfg.CustomPrograms["myeditor"] != "/usr/bin/myeditor" {
		t.Errorf("CustomPrograms[myeditor] = %q, unexpected", cfg.CustomPrograms["myeditor"])
	}
}

func TestSaveThenLoadPreservesValues(t *testing.T) {
	orig, err := Load("testdata/sample_config.ini")
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "config.ini")
	orig.path = dst
	if err := orig.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(dst)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(orig.Recent, reloaded.Recent) {
		t.Errorf("Recent mismatch after round trip: %+v vs %+v", orig.Recent, reloaded.Recent)
	}
	if !reflect.DeepEqual(orig.Window, reloaded.Window) {
		t.Errorf("Window mismatch after round trip: %+v vs %+v", orig.Window, reloaded.Window)
	}
	if !reflect.DeepEqual(orig.Columns, reloaded.Columns) {
		t.Errorf("Columns mismatch after round trip: %+v vs %+v", orig.Columns, reloaded.Columns)
	}
	if !reflect.DeepEqual(orig.Favorites, reloaded.Favorites) {
		t.Errorf("Favorites mismatch after round trip: %+v vs %+v", orig.Favorites, reloaded.Favorites)
	}
	if !reflect.DeepEqual(orig.FavoriteCategoryOrder, reloaded.FavoriteCategoryOrder) {
		t.Errorf("FavoriteCategoryOrder mismatch: %v vs %v", orig.FavoriteCategoryOrder, reloaded.FavoriteCategoryOrder)
	}
	if !reflect.DeepEqual(orig.FavoriteSearches, reloaded.FavoriteSearches) {
		t.Errorf("FavoriteSearches mismatch: %+v vs %+v", orig.FavoriteSearches, reloaded.FavoriteSearches)
	}
	if !reflect.DeepEqual(orig.CustomPrograms, reloaded.CustomPrograms) {
		t.Errorf("CustomPrograms mismatch: %+v vs %+v", orig.CustomPrograms, reloaded.CustomPrograms)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Window.HasPosition() {
		t.Error("expected no saved position for a fresh config")
	}
	if len(cfg.FavoriteCategoryOrder) != 1 || cfg.FavoriteCategoryOrder[0] != "Uncategorized" {
		t.Errorf("expected default category order, got %v", cfg.FavoriteCategoryOrder)
	}
}

func TestRecentConfigAddPathDedupsAndCaps(t *testing.T) {
	var r RecentConfig
	for i := 0; i < MaxRecentPaths+3; i++ {
		r.AddPath("/p")
	}
	if len(r.Paths) != 1 {
		t.Fatalf("expected dedup to collapse repeated adds to 1 entry, got %d", len(r.Paths))
	}

	r = RecentConfig{}
	r.AddPath("/a")
	r.AddPath("/b")
	r.AddPath("/c")
	r.AddPath("/d")
	r.AddPath("/e")
	r.AddPath("/f")
	if len(r.Paths) != MaxRecentPaths {
		t.Fatalf("expected cap at %d, got %d: %v", MaxRecentPaths, len(r.Paths), r.Paths)
	}
	if r.Paths[0] != "/f" {
		t.Errorf("expected most recent first, got %v", r.Paths)
	}
}

func TestRecentConfigAddContentPatternIgnoresEmpty(t *testing.T) {
	var r RecentConfig
	r.AddContentPattern("")
	if len(r.ContentPatterns) != 0 {
		t.Errorf("expected empty pattern to be ignored, got %v", r.ContentPatterns)
	}
}

func TestRecentResultsAndLastFilePatternRoundTrip(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.Recent.LastFilePattern = `\.go$`
	cfg.RecentResults = []RecentResult{
		{Path: "/a.go", Modified: "2026-01-01 00:00:00", SizeBytes: 100, SizeHuman: "100 B"},
		{Path: "/tmp/mnt/b.go", DisplayPath: "//host/share/b.go", Modified: "2026-01-02 00:00:00", SizeBytes: 200, SizeHuman: "200 B"},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Recent.LastFilePattern != `\.go$` {
		t.Errorf("LastFilePattern = %q, want %q", reloaded.Recent.LastFilePattern, `\.go$`)
	}
	if !reflect.DeepEqual(reloaded.RecentResults, cfg.RecentResults) {
		t.Errorf("RecentResults = %+v, want %+v", reloaded.RecentResults, cfg.RecentResults)
	}
}

func TestRecentResultsMatchesRoundTrip(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.RecentResults = []RecentResult{
		{
			Path: "/a.go", Modified: "2026-01-01 00:00:00", SizeBytes: 100, SizeHuman: "100 B",
			Matches: []RecentMatch{
				{LineNum: 5, ContextStartLine: 3, ContextLines: []string{"func Foo() {", "    x := 1", "    return x"}},
				{LineNum: 12, ContextStartLine: 12, ContextLines: []string{"// TODO: fix this | with a pipe in it"}},
			},
		},
		{Path: "/b.go", Modified: "2026-01-02 00:00:00", SizeBytes: 200, SizeHuman: "200 B"}, // no matches at all
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.RecentResults, cfg.RecentResults) {
		t.Errorf("RecentResults = %+v, want %+v", reloaded.RecentResults, cfg.RecentResults)
	}
}

func TestRecentResultsWithoutMatchesFieldStillLoads(t *testing.T) {
	// Simulates a config.ini written before Matches existed: only 5
	// pipe-delimited fields, no trailing Matches blob.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	if err := os.WriteFile(path, []byte("[RecentResults]\n0 = /a.go||2026-01-01 00:00:00|100|100 B\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RecentResults) != 1 {
		t.Fatalf("expected 1 recent result, got %d", len(cfg.RecentResults))
	}
	r := cfg.RecentResults[0]
	if r.Path != "/a.go" || r.SizeBytes != 100 || r.Matches != nil {
		t.Errorf("RecentResults[0] = %+v, unexpected", r)
	}
}

func TestThemeModeRoundTrip(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.ThemeMode = "light"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.ThemeMode != "light" {
		t.Errorf("ThemeMode = %q, want %q", reloaded.ThemeMode, "light")
	}
}

func TestAccentColorRoundTrip(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.AccentColor = "#FF8800"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AccentColor != "#FF8800" {
		t.Errorf("AccentColor = %q, want %q", reloaded.AccentColor, "#FF8800")
	}
}

func TestAccentColorEmptyIsOmitted(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AccentColor != "" {
		t.Errorf("AccentColor = %q, want empty", reloaded.AccentColor)
	}
}

func TestWorkspacesRoundTrip(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.Workspaces["Work"] = LocationWorkspace{
		Name:             "Work",
		SearchLocal:      true,
		SearchSMB:        true,
		SearchNFS:        false,
		LocalRoots:       []string{"/home/user/Work", "/home/user/Projects"},
		ExcludeDirs:      []string{"/home/user/Work/tmp"},
		SMBShares:        []string{"fileserver:projects", "fileserver:archive"},
		NFSExports:       nil,
		SMBShareExcludes: []string{"fileserver:projects:old-drafts"},
	}
	cfg.Workspaces["Personal"] = LocationWorkspace{
		Name:        "Personal",
		SearchLocal: true,
		LocalRoots:  []string{"/home/user/Personal"},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded.Workspaces, cfg.Workspaces) {
		t.Errorf("Workspaces = %+v, want %+v", reloaded.Workspaces, cfg.Workspaces)
	}
}

func TestWorkspacesEmptyListsRoundTripAsNil(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.Workspaces["Bare"] = LocationWorkspace{Name: "Bare", SearchLocal: true}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	w, ok := reloaded.Workspaces["Bare"]
	if !ok {
		t.Fatal("expected \"Bare\" workspace to load")
	}
	if w.LocalRoots != nil || w.ExcludeDirs != nil || w.SMBShares != nil || w.NFSExports != nil || w.SMBShareExcludes != nil {
		t.Errorf("expected all list fields nil for an empty workspace, got %+v", w)
	}
}

// TestWorkspacesPreSMBShareExcludesStillLoad guards backward compatibility:
// a workspace saved before SMBShareExcludes existed has exactly 7
// pipe-delimited parts on disk, not 8 -- it must still load cleanly, just
// with that field left empty, rather than being silently dropped.
func TestWorkspacesPreSMBShareExcludesStillLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	iniContent := "[Workspaces]\nOld = 1|1|0|/home/user/Work|" +
		"/home/user/Work/tmp|fileserver:projects|\n"
	if err := os.WriteFile(path, []byte(iniContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	w, ok := cfg.Workspaces["Old"]
	if !ok {
		t.Fatal("expected \"Old\" workspace to load from a pre-SMBShareExcludes 7-part value")
	}
	if w.SMBShareExcludes != nil {
		t.Errorf("SMBShareExcludes = %+v, want nil for a legacy 7-part workspace", w.SMBShareExcludes)
	}
	if !w.SearchLocal || !w.SearchSMB || w.SearchNFS {
		t.Errorf("flags = %+v, want SearchLocal/SearchSMB true, SearchNFS false", w)
	}
}

func TestLastScanCIDRRoundTrip(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	cfg.LastScanCIDR = "192.168.1.0/24"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.LastScanCIDR != "192.168.1.0/24" {
		t.Errorf("LastScanCIDR = %q, want %q", reloaded.LastScanCIDR, "192.168.1.0/24")
	}
}

func TestLastScanCIDREmptyIsOmitted(t *testing.T) {
	cfg := New(filepath.Join(t.TempDir(), "config.ini"))
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(cfg.path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.LastScanCIDR != "" {
		t.Errorf("LastScanCIDR = %q, want empty", reloaded.LastScanCIDR)
	}
}
