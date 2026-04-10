package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

type SQLiteConfigRepository struct {
	db *sql.DB
}

func NewConfigRepository(db *sql.DB) ConfigRepository {
	return &SQLiteConfigRepository{db: db}
}

func (r *SQLiteConfigRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO config (key, value)
VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key,
		value,
	)
	if err != nil {
		return fmt.Errorf("configRepo.Set: %w", err)
	}

	return nil
}

func (r *SQLiteConfigRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, "SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("configRepo.Get: %w", err)
	}

	return value, nil
}

func (r *SQLiteConfigRepository) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT key, value FROM config")
	if err != nil {
		return nil, fmt.Errorf("configRepo.GetAll: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("configRepo.GetAll scan: %w", err)
		}
		result[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("configRepo.GetAll rows: %w", err)
	}

	return result, nil
}

func (r *SQLiteConfigRepository) GetAppConfig(ctx context.Context) (*AppConfig, error) {
	var version int
	var rawValue string
	err := r.db.QueryRowContext(
		ctx,
		"SELECT version, value FROM app_config WHERE id = 1",
	).Scan(&version, &rawValue)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("configRepo.GetAppConfig query: %w", err)
		}

		legacyValues, legacyErr := r.GetAll(ctx)
		if legacyErr != nil {
			return nil, fmt.Errorf("configRepo.GetAppConfig load legacy config: %w", legacyErr)
		}

		cfg := mapLegacyConfig(legacyValues)
		return &cfg, nil
	}

	cfg, err := mapLegacyAppConfigJSON([]byte(rawValue))
	if err != nil {
		return nil, fmt.Errorf("configRepo.GetAppConfig unmarshal: %w", err)
	}
	if cfg.Version <= 0 {
		cfg.Version = version
	}

	return &cfg, nil
}

func (r *SQLiteConfigRepository) SaveAppConfig(ctx context.Context, value *AppConfig) error {
	if value == nil {
		return fmt.Errorf("configRepo.SaveAppConfig: value is nil")
	}

	normalized, err := normalizeAppConfigForSave(*value)
	if err != nil {
		return err
	}
	rawValue, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig marshal: %w", err)
	}

	checksum := checksumHex(rawValue)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO app_config (id, version, value, checksum, updated_at)
VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	version = excluded.version,
	value = excluded.value,
	checksum = excluded.checksum,
	updated_at = CURRENT_TIMESTAMP`,
		normalized.Version,
		string(rawValue),
		checksum,
	); err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig upsert: %w", err)
	}

	scanDirsJSON, err := json.Marshal(normalized.ScanInputDirs)
	if err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig marshal scan_input_dirs: %w", err)
	}

	if err := setConfigValue(ctx, tx, "scan_input_dirs", string(scanDirsJSON)); err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig set scan_input_dirs: %w", err)
	}
	if err := setConfigValue(ctx, tx, "scan_cron", normalized.ScanCron); err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig set scan_cron: %w", err)
	}
	outputDirsJSON, err := json.Marshal(normalized.OutputDirs)
	if err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig marshal output_dirs: %w", err)
	}
	if err := setConfigValue(ctx, tx, "output_dirs", string(outputDirsJSON)); err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig set output_dirs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("configRepo.SaveAppConfig commit: %w", err)
	}

	return nil
}

func (r *SQLiteConfigRepository) EnsureAppConfig(ctx context.Context) error {
	var exists int
	err := r.db.QueryRowContext(ctx, "SELECT 1 FROM app_config WHERE id = 1").Scan(&exists)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("configRepo.EnsureAppConfig query: %w", err)
	}

	legacyValues, err := r.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("configRepo.EnsureAppConfig load legacy config: %w", err)
	}

	defaultConfig := mapLegacyConfig(legacyValues)
	if err := r.SaveAppConfig(ctx, &defaultConfig); err != nil {
		return fmt.Errorf("configRepo.EnsureAppConfig save mapped config: %w", err)
	}

	return nil
}

func mapLegacyConfig(values map[string]string) AppConfig {
	cfg := defaultAppConfig()

	if value, ok := values["scan_cron"]; ok {
		cfg.ScanCron = strings.TrimSpace(value)
	}

	rawScanInputDirs, hasScanInputDirs := values["scan_input_dirs"]
	if hasScanInputDirs && strings.TrimSpace(rawScanInputDirs) != "" {
		var dirs []string
		if err := json.Unmarshal([]byte(rawScanInputDirs), &dirs); err == nil {
			cfg.ScanInputDirs = cleanPathList(dirs)
		}
	}
	var legacySourceDir string
	if value, ok := values["source_dir"]; ok {
		legacySourceDir = strings.TrimSpace(value)
	}
	if len(cfg.ScanInputDirs) == 0 && legacySourceDir != "" {
		cfg.ScanInputDirs = []string{legacySourceDir}
	}

	var legacyTargetDir string
	if value, ok := values["target_dir"]; ok {
		legacyTargetDir = strings.TrimSpace(value)
	}
	if rawOutputDirs, ok := values["output_dirs"]; ok && strings.TrimSpace(rawOutputDirs) != "" {
		var outputDirs AppConfigOutputDirs
		if err := json.Unmarshal([]byte(rawOutputDirs), &outputDirs); err == nil {
			cfg.OutputDirs = outputDirs
		}
	}

	cfg.OutputDirs = fillDefaultOutputDirs(cfg.OutputDirs, legacyTargetDir)

	return cfg
}

func defaultAppConfig() AppConfig {
	return AppConfig{
		Version:       1,
		ScanInputDirs: []string{},
		ScanCron:      "",
		OutputDirs: AppConfigOutputDirs{
			Video: "",
			Manga: "",
			Photo: "",
			Other: "",
			Mixed: "",
		},
	}
}

func normalizeAppConfig(value AppConfig) AppConfig {
	normalized := defaultAppConfig()

	if value.Version > 0 {
		normalized.Version = value.Version
	}

	normalized.ScanCron = strings.TrimSpace(value.ScanCron)
	normalized.ScanInputDirs = cleanPathList(value.ScanInputDirs)

	normalized.OutputDirs = AppConfigOutputDirs{
		Video: strings.TrimSpace(value.OutputDirs.Video),
		Manga: strings.TrimSpace(value.OutputDirs.Manga),
		Photo: strings.TrimSpace(value.OutputDirs.Photo),
		Other: strings.TrimSpace(value.OutputDirs.Other),
		Mixed: strings.TrimSpace(value.OutputDirs.Mixed),
	}

	return normalized
}

func normalizeAppConfigForSave(value AppConfig) (AppConfig, error) {
	normalized := normalizeAppConfig(value)
	var err error

	normalized.ScanInputDirs = cleanPathList(normalized.ScanInputDirs)
	for index, item := range normalized.ScanInputDirs {
		normalizedItem, normalizeErr := normalizeOptionalAbsPath(item)
		if normalizeErr != nil {
			return AppConfig{}, fmt.Errorf("%w: scan_input_dirs[%d]: %v", ErrInvalidConfig, index, normalizeErr)
		}
		normalized.ScanInputDirs[index] = normalizedItem
	}

	normalized.OutputDirs.Video, err = normalizeOptionalAbsPath(normalized.OutputDirs.Video)
	if err != nil {
		return AppConfig{}, fmt.Errorf("%w: output_dirs.video: %v", ErrInvalidConfig, err)
	}
	normalized.OutputDirs.Manga, err = normalizeOptionalAbsPath(normalized.OutputDirs.Manga)
	if err != nil {
		return AppConfig{}, fmt.Errorf("%w: output_dirs.manga: %v", ErrInvalidConfig, err)
	}
	normalized.OutputDirs.Photo, err = normalizeOptionalAbsPath(normalized.OutputDirs.Photo)
	if err != nil {
		return AppConfig{}, fmt.Errorf("%w: output_dirs.photo: %v", ErrInvalidConfig, err)
	}
	normalized.OutputDirs.Other, err = normalizeOptionalAbsPath(normalized.OutputDirs.Other)
	if err != nil {
		return AppConfig{}, fmt.Errorf("%w: output_dirs.other: %v", ErrInvalidConfig, err)
	}
	normalized.OutputDirs.Mixed, err = normalizeOptionalAbsPath(normalized.OutputDirs.Mixed)
	if err != nil {
		return AppConfig{}, fmt.Errorf("%w: output_dirs.mixed: %v", ErrInvalidConfig, err)
	}

	return normalized, nil
}

func fillDefaultOutputDirs(dirs AppConfigOutputDirs, targetDir string) AppConfigOutputDirs {
	baseDir := strings.TrimSpace(targetDir)
	if baseDir == "" {
		return dirs
	}

	if dirs.Video == "" {
		dirs.Video = filepath.Join(baseDir, "video")
	}
	if dirs.Manga == "" {
		dirs.Manga = filepath.Join(baseDir, "manga")
	}
	if dirs.Photo == "" {
		dirs.Photo = filepath.Join(baseDir, "photo")
	}
	if dirs.Other == "" {
		dirs.Other = filepath.Join(baseDir, "other")
	}
	if dirs.Mixed == "" {
		dirs.Mixed = filepath.Join(baseDir, "mixed")
	}

	return dirs
}

func cleanPathList(raw []string) []string {
	if len(raw) == 0 {
		return []string{}
	}

	cleaned := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}

	return cleaned
}

func mapLegacyAppConfigJSON(raw []byte) (AppConfig, error) {
	type legacyAppConfig struct {
		Version       int                 `json:"version"`
		ScanInputDirs []string            `json:"scan_input_dirs"`
		ScanCron      string              `json:"scan_cron"`
		OutputDirs    AppConfigOutputDirs `json:"output_dirs"`
		SourceDir     string              `json:"source_dir"`
		TargetDir     string              `json:"target_dir"`
	}

	var payload legacyAppConfig
	if err := json.Unmarshal(raw, &payload); err != nil {
		return AppConfig{}, err
	}

	cfg := AppConfig{
		Version:       payload.Version,
		ScanInputDirs: payload.ScanInputDirs,
		ScanCron:      payload.ScanCron,
		OutputDirs:    payload.OutputDirs,
	}
	cfg = normalizeAppConfig(cfg)
	if len(cfg.ScanInputDirs) == 0 && strings.TrimSpace(payload.SourceDir) != "" {
		cfg.ScanInputDirs = []string{strings.TrimSpace(payload.SourceDir)}
	}
	cfg.OutputDirs = fillDefaultOutputDirs(cfg.OutputDirs, payload.TargetDir)
	return cfg, nil
}

func normalizeOptionalAbsPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	if !isAbsoluteConfigPath(trimmed) {
		return "", fmt.Errorf("path must be absolute")
	}

	return trimmed, nil
}

func isAbsoluteConfigPath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	if runtime.GOOS == "windows" && (strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`)) {
		return true
	}

	return false
}

type configValueSetter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func setConfigValue(ctx context.Context, setter configValueSetter, key, value string) error {
	_, err := setter.ExecContext(
		ctx,
		`INSERT INTO config (key, value)
VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key,
		value,
	)
	if err != nil {
		return fmt.Errorf("set config key %q: %w", key, err)
	}

	return nil
}

func checksumHex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
