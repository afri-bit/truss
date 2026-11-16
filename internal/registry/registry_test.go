package registry

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistry_PersistsAcrossOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Add("nightly", "/work/nightly.yaml", now); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Add("build", "/work/truss.yaml", now); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetPaused("build", true); err != nil {
		t.Fatal(err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	list := again.List()
	if len(list) != 2 || list[0].Name != "build" || !list[0].Paused || list[1].Path != "/work/nightly.yaml" {
		t.Fatalf("list = %+v", list)
	}
	if !list[1].RegisteredAt.Equal(now) {
		t.Fatalf("registered at %s", list[1].RegisteredAt)
	}
}

func TestRegistry_AddIsIdempotentButRefusesAnotherPath(t *testing.T) {
	r, _ := Open(filepath.Join(t.TempDir(), "registry.json"))
	if _, err := r.Add("a", "/one.yaml", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Add("a", "/one.yaml", time.Now()); err != nil {
		t.Fatalf("same registration again: %v", err)
	}
	if _, err := r.Add("a", "/two.yaml", time.Now()); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v", err)
	}
	if _, err := r.Add("no spaces", "/x.yaml", time.Now()); err == nil {
		t.Fatal("bad name accepted")
	}
}

func TestRegistry_RemoveAndUnknownNames(t *testing.T) {
	r, _ := Open(filepath.Join(t.TempDir(), "registry.json"))
	_, _ = r.Add("a", "/one.yaml", time.Now())
	if err := r.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := r.SetPaused("a", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, ok := r.Get("a"); ok {
		t.Fatal("still registered")
	}
}

func TestRegistry_RemembersWhichNamesWereChosen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	r, _ := Open(path)
	if _, err := r.Add("from-file", "/one.yaml", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddChosen("mine", "/two.yaml", time.Now()); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := again.Get("from-file")
	b, _ := again.Get("mine")
	if a.Chosen || !b.Chosen {
		t.Fatalf("chosen = %t and %t, want false and true", a.Chosen, b.Chosen)
	}
}
