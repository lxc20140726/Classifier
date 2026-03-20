package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type SQLiteFolderRepository struct {
	db *sql.DB
}

func NewFolderRepository(db *sql.DB) FolderRepository {
	return &SQLiteFolderRepository{db: db}
}

func (r *SQLiteFolderRepository) Upsert(ctx context.Context, f *Folder) error {
	query := `
INSERT INTO folders (
	id, path, name, category, category_source, status,
	image_count, video_count, total_files, total_size, marked_for_move,
	deleted_at, delete_staging_path, scanned_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	path = excluded.path,
	name = excluded.name,
	category = excluded.category,
	category_source = excluded.category_source,
	status = excluded.status,
	image_count = excluded.image_count,
	video_count = excluded.video_count,
	total_files = excluded.total_files,
	total_size = excluded.total_size,
	marked_for_move = excluded.marked_for_move,
	deleted_at = excluded.deleted_at,
	delete_staging_path = excluded.delete_staging_path,
	updated_at = CURRENT_TIMESTAMP
`

	_, err := r.db.ExecContext(
		ctx,
		query,
		f.ID,
		f.Path,
		f.Name,
		f.Category,
		f.CategorySource,
		f.Status,
		f.ImageCount,
		f.VideoCount,
		f.TotalFiles,
		f.TotalSize,
		boolToInt(f.MarkedForMove),
		nullableTime(f.DeletedAt),
		nullableString(f.DeleteStagingPath),
	)
	if err != nil {
		return fmt.Errorf("folderRepo.Upsert: %w", err)
	}

	return nil
}

func (r *SQLiteFolderRepository) GetByID(ctx context.Context, id string) (*Folder, error) {
	folder, err := scanFolder(
		r.db.QueryRowContext(ctx, `
SELECT id, path, name, category, category_source, status,
	image_count, video_count, total_files, total_size, marked_for_move,
	deleted_at, delete_staging_path, scanned_at, updated_at
FROM folders
WHERE id = ?
`, id),
	)
	if err != nil {
		return nil, fmt.Errorf("folderRepo.GetByID: %w", err)
	}

	return folder, nil
}

func (r *SQLiteFolderRepository) GetByPath(ctx context.Context, path string) (*Folder, error) {
	folder, err := scanFolder(
		r.db.QueryRowContext(ctx, `
SELECT id, path, name, category, category_source, status,
	image_count, video_count, total_files, total_size, marked_for_move,
	deleted_at, delete_staging_path, scanned_at, updated_at
FROM folders
WHERE path = ? AND deleted_at IS NULL
`, path),
	)
	if err != nil {
		return nil, fmt.Errorf("folderRepo.GetByPath: %w", err)
	}

	return folder, nil
}

func (r *SQLiteFolderRepository) List(ctx context.Context, filter FolderListFilter) ([]*Folder, int, error) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 4)

	if filter.OnlyDeleted {
		where = append(where, "deleted_at IS NOT NULL")
	} else if !filter.IncludeDeleted {
		where = append(where, "deleted_at IS NULL")
	}

	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}

	if filter.Category != "" {
		where = append(where, "category = ?")
		args = append(args, filter.Category)
	}

	if filter.Q != "" {
		where = append(where, "(path LIKE ? OR name LIKE ?)")
		term := "%" + filter.Q + "%"
		args = append(args, term, term)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM folders" + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("folderRepo.List count: %w", err)
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	offset := (page - 1) * limit
	listArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, path, name, category, category_source, status,
	image_count, video_count, total_files, total_size, marked_for_move,
	deleted_at, delete_staging_path, scanned_at, updated_at
FROM folders`+whereClause+`
ORDER BY updated_at DESC
LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("folderRepo.List query: %w", err)
	}
	defer rows.Close()

	folders := make([]*Folder, 0)
	for rows.Next() {
		folder, err := scanFolder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("folderRepo.List scan: %w", err)
		}
		folders = append(folders, folder)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("folderRepo.List rows: %w", err)
	}

	return folders, total, nil
}

func (r *SQLiteFolderRepository) UpdateCategory(ctx context.Context, id, category, source string) error {
	res, err := r.db.ExecContext(
		ctx,
		"UPDATE folders SET category = ?, category_source = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		category,
		source,
		id,
	)
	if err != nil {
		return fmt.Errorf("folderRepo.UpdateCategory: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("folderRepo.UpdateCategory: %w", err)
	}

	return nil
}

func (r *SQLiteFolderRepository) UpdateStatus(ctx context.Context, id, status string) error {
	res, err := r.db.ExecContext(
		ctx,
		"UPDATE folders SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		status,
		id,
	)
	if err != nil {
		return fmt.Errorf("folderRepo.UpdateStatus: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("folderRepo.UpdateStatus: %w", err)
	}

	return nil
}

func (r *SQLiteFolderRepository) UpdatePath(ctx context.Context, id, newPath string) error {
	res, err := r.db.ExecContext(
		ctx,
		"UPDATE folders SET path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		newPath,
		id,
	)
	if err != nil {
		return fmt.Errorf("folderRepo.UpdatePath: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("folderRepo.UpdatePath: %w", err)
	}

	return nil
}

func (r *SQLiteFolderRepository) SoftDelete(ctx context.Context, id, currentPath, originalPath string) error {
	res, err := r.db.ExecContext(
		ctx,
		"UPDATE folders SET path = ?, deleted_at = CURRENT_TIMESTAMP, delete_staging_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL",
		currentPath,
		originalPath,
		id,
	)
	if err != nil {
		return fmt.Errorf("folderRepo.SoftDelete: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("folderRepo.SoftDelete: %w", err)
	}

	return nil
}

func (r *SQLiteFolderRepository) Restore(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(
		ctx,
		"UPDATE folders SET path = delete_staging_path, deleted_at = NULL, delete_staging_path = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NOT NULL",
		id,
	)
	if err != nil {
		return fmt.Errorf("folderRepo.Restore: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("folderRepo.Restore: %w", err)
	}

	return nil
}

func (r *SQLiteFolderRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM folders WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("folderRepo.Delete: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("folderRepo.Delete: %w", err)
	}

	return nil
}

func scanFolder(scanner interface{ Scan(dest ...any) error }) (*Folder, error) {
	folder := &Folder{}
	var markedForMove int
	var deletedAt any
	var deleteStagingPath sql.NullString
	var scannedAt any
	var updatedAt any

	err := scanner.Scan(
		&folder.ID,
		&folder.Path,
		&folder.Name,
		&folder.Category,
		&folder.CategorySource,
		&folder.Status,
		&folder.ImageCount,
		&folder.VideoCount,
		&folder.TotalFiles,
		&folder.TotalSize,
		&markedForMove,
		&deletedAt,
		&deleteStagingPath,
		&scannedAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	folder.MarkedForMove = intToBool(markedForMove)
	if folder.DeletedAt, err = parseNullableTime(deletedAt); err != nil {
		return nil, fmt.Errorf("scanFolder parse deleted_at: %w", err)
	}
	if deleteStagingPath.Valid {
		folder.DeleteStagingPath = deleteStagingPath.String
	}

	folder.ScannedAt, err = parseDBTime(scannedAt)
	if err != nil {
		return nil, fmt.Errorf("scanFolder parse scanned_at: %w", err)
	}

	folder.UpdatedAt, err = parseDBTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanFolder parse updated_at: %w", err)
	}

	return folder, nil
}

func assertRowsAffected(res sql.Result) error {
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
