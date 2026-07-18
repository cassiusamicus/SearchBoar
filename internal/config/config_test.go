package config

import (
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
