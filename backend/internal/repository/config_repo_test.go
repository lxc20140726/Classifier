package repository

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
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

func TestConfigRepositorySaveAndGetAppConfig(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)
	ctx := context.Background()

	err := repo.SaveAppConfig(ctx, &AppConfig{
		Version:       2,
		ScanInputDirs: []string{"/mnt/source", "/mnt/source-2"},
		ScanCron:      "0 * * * *",
		SourceDir:     "/mnt/source",
		TargetDir:     "/mnt/target",
		OutputDirs: AppConfigOutputDirs{
			Video: "/mnt/out/video",
			Manga: "/mnt/out/manga",
			Photo: "/mnt/out/photo",
			Other: "/mnt/out/other",
			Mixed: "/mnt/out/mixed",
		},
	})
	if err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}

	got, err := repo.GetAppConfig(ctx)
	if err != nil {
		t.Fatalf("GetAppConfig() error = %v", err)
	}

	if got.Version != 2 {
		t.Fatalf("Version = %d, want 2", got.Version)
	}
	if !reflect.DeepEqual(got.ScanInputDirs, []string{"/mnt/source", "/mnt/source-2"}) {
		t.Fatalf("ScanInputDirs = %#v, want [/mnt/source /mnt/source-2]", got.ScanInputDirs)
	}
	if got.ScanCron != "0 * * * *" {
		t.Fatalf("ScanCron = %q, want 0 * * * *", got.ScanCron)
	}

	rawScanInputDirs, err := repo.Get(ctx, "scan_input_dirs")
	if err != nil {
		t.Fatalf("Get(scan_input_dirs) error = %v", err)
	}
	if rawScanInputDirs != `["/mnt/source","/mnt/source-2"]` {
		t.Fatalf("scan_input_dirs = %q, want %q", rawScanInputDirs, `["/mnt/source","/mnt/source-2"]`)
	}

	rawScanCron, err := repo.Get(ctx, "scan_cron")
	if err != nil {
		t.Fatalf("Get(scan_cron) error = %v", err)
	}
	if rawScanCron != "0 * * * *" {
		t.Fatalf("scan_cron = %q, want 0 * * * *", rawScanCron)
	}
}

func TestConfigRepositoryGetAppConfigFallsBackToLegacyKV(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)
	ctx := context.Background()

	if err := repo.Set(ctx, "source_dir", "/legacy/source"); err != nil {
		t.Fatalf("Set(source_dir) error = %v", err)
	}
	if err := repo.Set(ctx, "target_dir", "/legacy/target"); err != nil {
		t.Fatalf("Set(target_dir) error = %v", err)
	}
	if err := repo.Set(ctx, "scan_input_dirs", `["/legacy/source","/legacy/source-2"]`); err != nil {
		t.Fatalf("Set(scan_input_dirs) error = %v", err)
	}

	got, err := repo.GetAppConfig(ctx)
	if err != nil {
		t.Fatalf("GetAppConfig() error = %v", err)
	}

	if got.SourceDir != "/legacy/source" {
		t.Fatalf("SourceDir = %q, want /legacy/source", got.SourceDir)
	}
	if got.TargetDir != "/legacy/target" {
		t.Fatalf("TargetDir = %q, want /legacy/target", got.TargetDir)
	}
	expectedVideoDir := filepath.Join("/legacy/target", "video")
	if got.OutputDirs.Video != expectedVideoDir {
		t.Fatalf("OutputDirs.Video = %q, want %q", got.OutputDirs.Video, expectedVideoDir)
	}
	if !reflect.DeepEqual(got.ScanInputDirs, []string{"/legacy/source", "/legacy/source-2"}) {
		t.Fatalf("ScanInputDirs = %#v, want [/legacy/source /legacy/source-2]", got.ScanInputDirs)
	}
}

func TestConfigRepositorySaveAppConfigRejectsRelativePath(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)

	err := repo.SaveAppConfig(context.Background(), &AppConfig{
		SourceDir: "relative/source",
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("SaveAppConfig() error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigRepositoryEnsureAppConfig(t *testing.T) {
	t.Parallel()

	database := newTestDB(t)
	repo := NewConfigRepository(database)
	ctx := context.Background()

	if err := repo.Set(ctx, "source_dir", "/legacy/source"); err != nil {
		t.Fatalf("Set(source_dir) error = %v", err)
	}

	if err := repo.EnsureAppConfig(ctx); err != nil {
		t.Fatalf("EnsureAppConfig() error = %v", err)
	}

	var rowCount int
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM app_config WHERE id = 1").Scan(&rowCount); err != nil {
		t.Fatalf("query app_config count error = %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("app_config row count = %d, want 1", rowCount)
	}

	var rawValue string
	if err := database.QueryRowContext(ctx, "SELECT value FROM app_config WHERE id = 1").Scan(&rawValue); err != nil {
		t.Fatalf("query app_config value error = %v", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal([]byte(rawValue), &cfg); err != nil {
		t.Fatalf("json.Unmarshal(app_config.value) error = %v", err)
	}
	if cfg.SourceDir != "/legacy/source" {
		t.Fatalf("cfg.SourceDir = %q, want /legacy/source", cfg.SourceDir)
	}
}
