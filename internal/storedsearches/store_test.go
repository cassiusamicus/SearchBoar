package storedsearches

import (
	"testing"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

func TestAddGetListDelete(t *testing.T) {
	cfg := config.New("/dev/null")
	s := LoadStore(cfg)

	if err := s.Add("MySearch", config.StoredSearch{FilePattern: "*.md", ContentPattern: "epicurus", Directory: "/docs"}); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Get("MySearch")
	if !ok || got.FilePattern != "*.md" || got.Directory != "/docs" {
		t.Errorf("Get returned %+v ok=%v", got, ok)
	}

	if err := s.Add("MySearch", config.StoredSearch{}); err != ErrExists {
		t.Errorf("expected ErrExists on duplicate name, got %v", err)
	}

	s.Add("Another", config.StoredSearch{FilePattern: "*.txt"})
	list := s.List()
	if len(list) != 2 || list[0].Name != "Another" || list[1].Name != "MySearch" {
		t.Errorf("List() = %+v, expected sorted [Another, MySearch]", list)
	}

	if err := s.Delete("MySearch"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("MySearch"); ok {
		t.Error("expected MySearch to be gone after Delete")
	}
	if err := s.Delete("MySearch"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound deleting an already-deleted search, got %v", err)
	}
}
