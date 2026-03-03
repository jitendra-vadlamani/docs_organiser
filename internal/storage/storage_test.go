package storage

import (
	"os"
	"testing"
)

func TestBadgerStore(t *testing.T) {
	tmpDir := "tmp_badger_test"
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	type TestData struct {
		Name  string
		Value int
	}

	key := "test_key"
	val := TestData{Name: "test", Value: 123}

	// Test Save
	if err := store.Save(key, val); err != nil {
		t.Fatalf("Failed to save data: %v", err)
	}

	// Test Load
	var loaded TestData
	exists, err := store.Load(key, &loaded)
	if err != nil {
		t.Fatalf("Failed to load data: %v", err)
	}
	if !exists {
		t.Fatal("Data should exist")
	}
	if loaded.Name != val.Name || loaded.Value != val.Value {
		t.Errorf("Loaded data mismatch: got %+v, want %+v", loaded, val)
	}

	// Test non-existent key
	var notFound TestData
	exists, err = store.Load("non_existent", &notFound)
	if err != nil {
		t.Fatalf("Failed to load non-existent key: %v", err)
	}
	if exists {
		t.Fatal("Key should not exist")
	}
}
