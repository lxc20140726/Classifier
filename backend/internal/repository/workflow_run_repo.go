package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type SQLiteWorkflowRunRepository struct {
	db *sql.DB
}

func NewWorkflowRunRepository(db *sql.DB) WorkflowRunRepository {
	return &SQLiteWorkflowRunRepository{db: db}
}

func (r *SQLiteWorkflowRunRepository) Create(ctx context.Context, item *WorkflowRun) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO workflow_runs (
	id, job_id, folder_id, workflow_def_id, status, resume_node_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		item.ID,
		item.JobID,
		item.FolderID,
		item.WorkflowDefID,
		item.Status,
		nullableString(item.ResumeNodeID),
	)
	if err != nil {
		return fmt.Errorf("workflowRunRepo.Create: %w", err)
	}

	return nil
}

func (r *SQLiteWorkflowRunRepository) GetByID(ctx context.Context, id string) (*WorkflowRun, error) {
	item, err := scanWorkflowRun(r.db.QueryRowContext(ctx, `
SELECT id, job_id, folder_id, workflow_def_id, status, resume_node_id, created_at, updated_at
FROM workflow_runs
WHERE id = ?`, id))
	if err != nil {
		return nil, fmt.Errorf("workflowRunRepo.GetByID: %w", err)
	}

	return item, nil
}

func (r *SQLiteWorkflowRunRepository) List(ctx context.Context, filter WorkflowRunListFilter) ([]*WorkflowRun, int, error) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if filter.JobID != "" {
		where = append(where, "job_id = ?")
		args = append(args, filter.JobID)
	}
	if filter.FolderID != "" {
		where = append(where, "folder_id = ?")
		args = append(args, filter.FolderID)
	}
	if filter.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filter.Status)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM workflow_runs"+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("workflowRunRepo.List count: %w", err)
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

	rows, err := r.db.QueryContext(ctx, `
SELECT id, job_id, folder_id, workflow_def_id, status, resume_node_id, created_at, updated_at
FROM workflow_runs`+whereClause+`
ORDER BY created_at DESC
LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("workflowRunRepo.List query: %w", err)
	}
	defer rows.Close()

	items := make([]*WorkflowRun, 0)
	for rows.Next() {
		item, scanErr := scanWorkflowRun(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("workflowRunRepo.List scan: %w", scanErr)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("workflowRunRepo.List rows: %w", err)
	}

	return items, total, nil
}

func (r *SQLiteWorkflowRunRepository) UpdateStatus(ctx context.Context, id, status, resumeNodeID string) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE workflow_runs
SET status = ?, resume_node_id = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`,
		status,
		nullableString(resumeNodeID),
		id,
	)
	if err != nil {
		return fmt.Errorf("workflowRunRepo.UpdateStatus: %w", err)
	}

	if err := assertRowsAffected(res); err != nil {
		return fmt.Errorf("workflowRunRepo.UpdateStatus: %w", err)
	}

	return nil
}

func scanWorkflowRun(scanner interface{ Scan(dest ...any) error }) (*WorkflowRun, error) {
	item := &WorkflowRun{}
	var resumeNodeID sql.NullString
	var createdAt any
	var updatedAt any

	err := scanner.Scan(
		&item.ID,
		&item.JobID,
		&item.FolderID,
		&item.WorkflowDefID,
		&item.Status,
		&resumeNodeID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if resumeNodeID.Valid {
		item.ResumeNodeID = resumeNodeID.String
	}
	item.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("scanWorkflowRun parse created_at: %w", err)
	}
	item.UpdatedAt, err = parseDBTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanWorkflowRun parse updated_at: %w", err)
	}

	return item, nil
}
