package repository

import (
	"context"
	"errors"
	"testing"
)

func TestConfigRepositorySetGet(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)
	ctx := context.Background()

	if err := repo.Set(ctx, "scan.path", "/media"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := repo.Set(ctx, "scan.path", "/media/new"); err != nil {
		t.Fatalf("Set(upsert) error = %v", err)
	}

	value, err := repo.Get(ctx, "scan.path")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "/media/new" {
		t.Fatalf("Get() value = %q, want %q", value, "/media/new")
	}
}

func TestConfigRepositoryGetAll(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)
	ctx := context.Background()

	fixtures := map[string]string{
		"scan.path":      "/media",
		"scan.recursive": "true",
		"move.target":    "/sorted",
	}

	for key, value := range fixtures {
		if err := repo.Set(ctx, key, value); err != nil {
			t.Fatalf("Set(%q) error = %v", key, err)
		}
	}

	all, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}

	if len(all) != len(fixtures) {
		t.Fatalf("GetAll() len = %d, want %d", len(all), len(fixtures))
	}

	for key, want := range fixtures {
		got, ok := all[key]
		if !ok {
			t.Fatalf("GetAll() missing key %q", key)
		}

		if got != want {
			t.Fatalf("GetAll()[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestConfigRepositoryGetNotFound(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)

	_, err := repo.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrNotFound", err)
	}
}
