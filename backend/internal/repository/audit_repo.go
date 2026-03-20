package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type SQLiteAuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) AuditRepository {
	return &SQLiteAuditRepository{db: db}
}

func (r *SQLiteAuditRepository) Write(ctx context.Context, log *AuditLog) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO audit_logs (
	id, job_id, folder_id, folder_path, action, level, detail, result, error_msg, duration_ms, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		log.ID,
		nullableString(log.JobID),
		nullableString(log.FolderID),
		log.FolderPath,
		log.Action,
		log.Level,
		nullableJSON(log.Detail),
		log.Result,
		nullableString(log.ErrorMsg),
		log.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("auditRepo.Write: %w", err)
	}

	return nil
}

func (r *SQLiteAuditRepository) List(ctx context.Context, filter AuditListFilter) ([]*AuditLog, int, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 6)

	if filter.Action != "" {
		where = append(where, "action = ?")
		args = append(args, filter.Action)
	}

	if filter.JobID != "" {
		where = append(where, "job_id = ?")
		args = append(args, filter.JobID)
	}

	if filter.Result != "" {
		where = append(where, "result = ?")
		args = append(args, filter.Result)
	}

	if filter.FolderID != "" {
		where = append(where, "folder_id = ?")
		args = append(args, filter.FolderID)
	}

	if !filter.From.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, filter.From.UTC().Format("2006-01-02 15:04:05"))
	}

	if !filter.To.IsZero() {
		where = append(where, "created_at <= ?")
		args = append(args, filter.To.UTC().Format("2006-01-02 15:04:05"))
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + whereClause
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("auditRepo.List count: %w", err)
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	offset := (page - 1) * limit
	listArgs := append(append([]any{}, args...), limit, offset)

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, job_id, folder_id, folder_path, action, level, detail, result, error_msg, duration_ms, created_at
FROM audit_logs`+whereClause+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`,
		listArgs...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("auditRepo.List query: %w", err)
	}
	defer rows.Close()

	items := make([]*AuditLog, 0)
	for rows.Next() {
		item, err := scanAuditLog(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("auditRepo.List scan: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("auditRepo.List rows: %w", err)
	}

	return items, total, nil
}

func (r *SQLiteAuditRepository) GetByID(ctx context.Context, id string) (*AuditLog, error) {
	item, err := scanAuditLog(
		r.db.QueryRowContext(ctx, `
SELECT id, job_id, folder_id, folder_path, action, level, detail, result, error_msg, duration_ms, created_at
FROM audit_logs
WHERE id = ?`, id),
	)
	if err != nil {
		return nil, fmt.Errorf("auditRepo.GetByID: %w", err)
	}

	return item, nil
}

func scanAuditLog(scanner interface{ Scan(dest ...any) error }) (*AuditLog, error) {
	item := &AuditLog{}
	var jobID sql.NullString
	var folderID sql.NullString
	var detail sql.NullString
	var errorMsg sql.NullString
	var createdAt any

	err := scanner.Scan(
		&item.ID,
		&jobID,
		&folderID,
		&item.FolderPath,
		&item.Action,
		&item.Level,
		&detail,
		&item.Result,
		&errorMsg,
		&item.DurationMs,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if jobID.Valid {
		item.JobID = jobID.String
	}

	if folderID.Valid {
		item.FolderID = folderID.String
	}

	if detail.Valid {
		item.Detail = json.RawMessage(detail.String)
	}

	if errorMsg.Valid {
		item.ErrorMsg = errorMsg.String
	}

	item.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("scanAuditLog parse created_at: %w", err)
	}

	return item, nil
}
