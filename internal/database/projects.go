package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/devherd/devherd/internal/detector"
)

type ProjectRecord struct {
	ID        int64
	Name      string
	Path      string
	Stack     string
	Framework string
	Runtime   string
	Status    string
	Domain    string
}

func InsertPark(ctx context.Context, db *sql.DB, path string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO parks (path)
		VALUES (?)
		ON CONFLICT(path) DO NOTHING
	`, path)
	if err != nil {
		return fmt.Errorf("insert park: %w", err)
	}

	return nil
}

func UpsertProject(ctx context.Context, db *sql.DB, project detector.Project, domain string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO projects (name, path, stack, framework, runtime, status)
		VALUES (?, ?, ?, ?, ?, 'detected')
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			stack = excluded.stack,
			framework = excluded.framework,
			runtime = excluded.runtime,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP
	`, project.Name, project.Path, project.Stack, project.Framework, project.Runtime)
	if err != nil {
		return fmt.Errorf("upsert project: %w", err)
	}

	var projectID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE path = ?`, project.Path).Scan(&projectID); err != nil {
		return fmt.Errorf("lookup project id: %w", err)
	}

	existingDomain, err := currentPrimaryDomain(ctx, tx, projectID)
	if err != nil {
		return fmt.Errorf("lookup project domain: %w", err)
	}

	if existingDomain != "" {
		domain = existingDomain
	}

	if err := ensureDomainAvailable(ctx, tx, projectID, domain); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_domains WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("cleanup project domains: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO project_domains (project_id, domain, kind, is_primary)
		VALUES (?, ?, 'primary', 1)
	`, projectID, domain)
	if err != nil {
		return fmt.Errorf("upsert project domain: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func ListProjects(ctx context.Context, db *sql.DB) ([]ProjectRecord, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			p.id,
			p.name,
			p.path,
			p.stack,
			p.framework,
			p.runtime,
			p.status,
			COALESCE(d.domain, '')
		FROM projects p
		LEFT JOIN project_domains d
			ON d.project_id = p.id
			AND d.is_primary = 1
		ORDER BY p.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectRecord
	for rows.Next() {
		var project ProjectRecord
		if err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.Path,
			&project.Stack,
			&project.Framework,
			&project.Runtime,
			&project.Status,
			&project.Domain,
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func FindProjectByPath(ctx context.Context, db *sql.DB, path string) (ProjectRecord, bool, error) {
	var project ProjectRecord
	err := db.QueryRowContext(ctx, `
		SELECT
			p.id,
			p.name,
			p.path,
			p.stack,
			p.framework,
			p.runtime,
			p.status,
			COALESCE(d.domain, '')
		FROM projects p
		LEFT JOIN project_domains d
			ON d.project_id = p.id
			AND d.is_primary = 1
		WHERE p.path = ?
		LIMIT 1
	`, path).Scan(
		&project.ID,
		&project.Name,
		&project.Path,
		&project.Stack,
		&project.Framework,
		&project.Runtime,
		&project.Status,
		&project.Domain,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectRecord{}, false, nil
	}
	if err != nil {
		return ProjectRecord{}, false, fmt.Errorf("find project by path: %w", err)
	}

	return project, true, nil
}

func SetPrimaryDomain(ctx context.Context, db *sql.DB, projectName, domain string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var projectID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE name = ?`, projectName).Scan(&projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("project %q not found", projectName)
		}

		return fmt.Errorf("lookup project: %w", err)
	}

	if err := ensureDomainAvailable(ctx, tx, projectID, domain); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_domains WHERE project_id = ?`, projectID); err != nil {
		return fmt.Errorf("cleanup project domains: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_domains (project_id, domain, kind, is_primary)
		VALUES (?, ?, 'primary', 1)
	`, projectID, domain); err != nil {
		return fmt.Errorf("insert primary domain: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func currentPrimaryDomain(ctx context.Context, tx *sql.Tx, projectID int64) (string, error) {
	var domain string
	err := tx.QueryRowContext(ctx, `
		SELECT domain
		FROM project_domains
		WHERE project_id = ?
		  AND is_primary = 1
		LIMIT 1
	`, projectID).Scan(&domain)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return domain, nil
}

func ensureDomainAvailable(ctx context.Context, tx *sql.Tx, projectID int64, domain string) error {
	var ownerID int64
	err := tx.QueryRowContext(ctx, `
		SELECT project_id
		FROM project_domains
		WHERE domain = ?
		LIMIT 1
	`, domain).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("lookup domain owner: %w", err)
	}

	if ownerID != projectID {
		return fmt.Errorf("domain %q is already assigned to another project", domain)
	}

	return nil
}
