package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestItemCRUD(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	items, err := store.ListItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(items))
	}

	created, err := store.CreateItem("hello")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Title != "hello" || created.CreatedAt == "" {
		t.Fatalf("unexpected item: %+v", created)
	}

	items, err = store.ListItems()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list: %+v", items)
	}

	if err := store.DeleteItem(created.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteItem(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOpenInMemory(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.CreateItem("ephemeral"); err != nil {
		t.Fatal(err)
	}
}
