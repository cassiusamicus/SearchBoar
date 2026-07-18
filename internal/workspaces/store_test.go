package workspaces

import (
	"testing"

	"codeberg.org/cassiusamicus/Utilities/internal/config"
)

func TestPutGetListDelete(t *testing.T) {
	cfg := config.New("/dev/null")
	s := LoadStore(cfg)

	s.Put("Work", config.LocationWorkspace{SearchLocal: true, LocalRoots: []string{"/home/user/Work"}})

	got, ok := s.Get("Work")
	if !ok || got.Name != "Work" || !got.SearchLocal || len(got.LocalRoots) != 1 || got.LocalRoots[0] != "/home/user/Work" {
		t.Errorf("Get returned %+v ok=%v", got, ok)
	}

	s.Put("Another", config.LocationWorkspace{SearchLocal: true})
	list := s.List()
	if len(list) != 2 || list[0].Name != "Another" || list[1].Name != "Work" {
		t.Errorf("List() = %+v, expected sorted [Another, Work]", list)
	}

	if err := s.Delete("Work"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("Work"); ok {
		t.Error("expected Work to be gone after Delete")
	}
	if err := s.Delete("Work"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound deleting an already-deleted workspace, got %v", err)
	}
}

func TestPutOverwritesExistingName(t *testing.T) {
	cfg := config.New("/dev/null")
	s := LoadStore(cfg)

	s.Put("Work", config.LocationWorkspace{SearchLocal: true, LocalRoots: []string{"/old"}})
	s.Put("Work", config.LocationWorkspace{SearchLocal: true, LocalRoots: []string{"/new"}})

	got, ok := s.Get("Work")
	if !ok || len(got.LocalRoots) != 1 || got.LocalRoots[0] != "/new" {
		t.Errorf("Get after overwrite = %+v ok=%v, want LocalRoots=[/new]", got, ok)
	}
	if len(s.List()) != 1 {
		t.Errorf("expected overwrite to not create a duplicate entry, got %d", len(s.List()))
	}
}
