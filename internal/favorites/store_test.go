package favorites

import (
	"testing"

	"github.com/cassiusamicus/SearchBoar/internal/config"
)

func newTestConfig() *config.Config {
	return config.New("/dev/null")
}

func TestAddAndCategories(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)

	if err := s.Add(config.FavoriteRecord{Filepath: "/a.md", Filename: "a.md"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(config.FavoriteRecord{Filepath: "/b.md", Filename: "b.md", Category: "Research"}); err != nil {
		t.Fatal(err)
	}

	cats := s.Categories()
	if len(cats) != 2 {
		t.Fatalf("expected 2 categories (Uncategorized, Research), got %d: %+v", len(cats), cats)
	}
	if cats[0].Name != DefaultCategory || len(cats[0].Items) != 1 {
		t.Errorf("Uncategorized category wrong: %+v", cats[0])
	}
	if cats[1].Name != "Research" || len(cats[1].Items) != 1 {
		t.Errorf("Research category wrong: %+v", cats[1])
	}
}

func TestAddDuplicateRejected(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	rec := config.FavoriteRecord{Filepath: "/a.md"}
	if err := s.Add(rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(rec); err != ErrDuplicate {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

func TestAddEnforcesCap(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	for i := 0; i < config.MaxFavorites; i++ {
		path := string(rune('a' + i%26))
		if err := s.Add(config.FavoriteRecord{Filepath: path + string(rune(i))}); err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}
	}
	if err := s.Add(config.FavoriteRecord{Filepath: "/overflow"}); err != ErrLimitReached {
		t.Errorf("expected ErrLimitReached, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	s.Add(config.FavoriteRecord{Filepath: "/a.md"})
	s.Add(config.FavoriteRecord{Filepath: "/b.md"})
	s.Remove("/a.md")
	if s.Contains("/a.md") {
		t.Error("expected /a.md to be removed")
	}
	if !s.Contains("/b.md") {
		t.Error("expected /b.md to remain")
	}
}

func TestCategoryCRUD(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)

	if err := s.AddCategory("Research"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddCategory("Research"); err != ErrCategoryExists {
		t.Errorf("expected ErrCategoryExists, got %v", err)
	}

	s.Add(config.FavoriteRecord{Filepath: "/a.md", Category: "Research"})

	if err := s.RenameCategory("Research", "Studies"); err != nil {
		t.Fatal(err)
	}
	cats := s.Categories()
	found := false
	for _, c := range cats {
		if c.Name == "Studies" {
			found = true
			if len(c.Items) != 1 || c.Items[0].Filepath != "/a.md" {
				t.Errorf("renamed category items wrong: %+v", c.Items)
			}
		}
		if c.Name == "Research" {
			t.Error("old category name should no longer exist")
		}
	}
	if !found {
		t.Error("renamed category not found")
	}
}

func TestMoveToCategoryRequiresExistingCategory(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	s.Add(config.FavoriteRecord{Filepath: "/a.md"})

	if err := s.MoveToCategory("/a.md", "DoesNotExist"); err != ErrCategoryNotFound {
		t.Errorf("expected ErrCategoryNotFound, got %v", err)
	}

	s.AddCategory("Research")
	if err := s.MoveToCategory("/a.md", "Research"); err != nil {
		t.Fatal(err)
	}
	for _, f := range cfg.Favorites {
		if f.Filepath == "/a.md" && f.Category != "Research" {
			t.Errorf("expected /a.md moved to Research, got %q", f.Category)
		}
	}
}

func TestDeleteCategoryMergesIntoUncategorized(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	s.AddCategory("Research")
	s.Add(config.FavoriteRecord{Filepath: "/a.md", Category: "Research"})

	if err := s.DeleteCategory("Research"); err != nil {
		t.Fatal(err)
	}
	for _, f := range cfg.Favorites {
		if f.Filepath == "/a.md" && f.Category != DefaultCategory {
			t.Errorf("expected /a.md merged into Uncategorized, got %q", f.Category)
		}
	}
	for _, c := range s.Categories() {
		if c.Name == "Research" {
			t.Error("Research category should have been removed")
		}
	}
}

func TestDeleteCategoryRefusesDefault(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	if err := s.DeleteCategory(DefaultCategory); err != ErrDefaultCategory {
		t.Errorf("expected ErrDefaultCategory, got %v", err)
	}
}

func TestMoveUpDownWithinCategory(t *testing.T) {
	cfg := newTestConfig()
	s := LoadStore(cfg)
	s.Add(config.FavoriteRecord{Filepath: "/1", Category: "Research"})
	s.Add(config.FavoriteRecord{Filepath: "/other", Category: "Uncategorized"}) // interleaved, different category
	s.Add(config.FavoriteRecord{Filepath: "/2", Category: "Research"})
	s.Add(config.FavoriteRecord{Filepath: "/3", Category: "Research"})

	order := func() []string {
		var out []string
		for _, c := range s.Categories() {
			if c.Name == "Research" {
				for _, i := range c.Items {
					out = append(out, i.Filepath)
				}
			}
		}
		return out
	}

	if got := order(); !equal(got, []string{"/1", "/2", "/3"}) {
		t.Fatalf("initial order = %v", got)
	}

	if err := s.MoveDown("/1"); err != nil {
		t.Fatal(err)
	}
	if got := order(); !equal(got, []string{"/2", "/1", "/3"}) {
		t.Errorf("after MoveDown(/1) = %v", got)
	}

	if err := s.MoveUp("/3"); err != nil {
		t.Fatal(err)
	}
	if got := order(); !equal(got, []string{"/2", "/3", "/1"}) {
		t.Errorf("after MoveUp(/3) = %v", got)
	}

	// Moving the top item up (or bottom item down) is a no-op, not an error.
	if err := s.MoveUp("/2"); err != nil {
		t.Fatal(err)
	}
	if got := order(); !equal(got, []string{"/2", "/3", "/1"}) {
		t.Errorf("MoveUp at top should be a no-op, got %v", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
