package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	tablaJob = `lab_scheduled_job`
	tablaRun = `lab_job_run`
)

const columnasJob = `id, name, cron, action, params, enabled, "createdAt"`

type filaJob struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Cron      string    `db:"cron"`
	Action    string    `db:"action"`
	Params    string    `db:"params"`
	Enabled   bool      `db:"enabled"`
	CreatedAt time.Time `db:"createdAt"`
}

func (f filaJob) aDominio() Job {
	return Job{
		ID: f.ID, Name: f.Name, Cron: f.Cron, Action: Accion(f.Action),
		Params: f.Params, Enabled: f.Enabled, CreatedAt: f.CreatedAt,
	}
}

type PostgresJobRepo struct {
	db *sqlx.DB
}

func NewPostgresJobRepo(db *sqlx.DB) *PostgresJobRepo {
	return &PostgresJobRepo{db: db}
}

func (r *PostgresJobRepo) List(ctx context.Context) ([]Job, error) {
	return r.consultar(ctx, `SELECT `+columnasJob+` FROM `+tablaJob+` ORDER BY "createdAt" DESC`)
}

func (r *PostgresJobRepo) ListEnabled(ctx context.Context) ([]Job, error) {
	return r.consultar(ctx, `SELECT `+columnasJob+` FROM `+tablaJob+` WHERE enabled = true`)
}

func (r *PostgresJobRepo) consultar(ctx context.Context, q string, args ...any) ([]Job, error) {
	var filas []filaJob
	if err := r.db.SelectContext(ctx, &filas, q, args...); err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(filas))
	for _, f := range filas {
		jobs = append(jobs, f.aDominio())
	}
	return jobs, nil
}

func (r *PostgresJobRepo) Get(ctx context.Context, id string) (*Job, error) {
	var f filaJob
	err := r.db.GetContext(ctx, &f, `SELECT `+columnasJob+` FROM `+tablaJob+` WHERE id = $1`, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j := f.aDominio()
	return &j, nil
}

func (r *PostgresJobRepo) Create(ctx context.Context, j Job) (*Job, error) {
	var f filaJob
	err := r.db.GetContext(ctx, &f,
		`INSERT INTO `+tablaJob+` (id, name, cron, action, params, enabled, "createdAt")
		 VALUES ($1, $2, $3, $4, $5, $6, NOW()) RETURNING `+columnasJob,
		uuid.NewString(), j.Name, j.Cron, string(j.Action), j.Params, j.Enabled)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresJobRepo) Update(ctx context.Context, j Job) (*Job, error) {
	var f filaJob
	err := r.db.GetContext(ctx, &f,
		`UPDATE `+tablaJob+` SET name = $2, cron = $3, action = $4, params = $5, enabled = $6
		 WHERE id = $1 RETURNING `+columnasJob,
		j.ID, j.Name, j.Cron, string(j.Action), j.Params, j.Enabled)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresJobRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaJob+` WHERE id = $1`, id)
	return err
}

// ── El historial ───────────────────────────────────────────────────────────

type PostgresRunRepo struct {
	db *sqlx.DB
}

func NewPostgresRunRepo(db *sqlx.DB) *PostgresRunRepo {
	return &PostgresRunRepo{db: db}
}

func (r *PostgresRunRepo) Record(ctx context.Context, jobID, estado string, salida *string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tablaRun+` (id, "jobId", status, output, "startedAt")
		 VALUES ($1, $2, $3, $4, NOW())`,
		uuid.NewString(), jobID, estado, salida)
	return err
}

type filaRun struct {
	ID        string    `db:"id"`
	JobID     string    `db:"jobId"`
	JobName   string    `db:"jobName"`
	Status    string    `db:"status"`
	Output    *string   `db:"output"`
	StartedAt time.Time `db:"startedAt"`
}

// ListRecent trae las últimas ejecuciones con el nombre de su tarea, en una
// sola consulta: pedir el nombre aparte por cada fila serían N+1 viajes.
func (r *PostgresRunRepo) ListRecent(ctx context.Context, limite int) ([]Run, error) {
	var filas []filaRun
	err := r.db.SelectContext(ctx, &filas, `
		SELECT e.id, e."jobId", j.name AS "jobName", e.status, e.output, e."startedAt"
		FROM `+tablaRun+` e
		JOIN `+tablaJob+` j ON j.id = e."jobId"
		ORDER BY e."startedAt" DESC
		LIMIT $1`, limite)
	if err != nil {
		return nil, err
	}

	runs := make([]Run, 0, len(filas))
	for _, f := range filas {
		runs = append(runs, Run{
			ID: f.ID, JobID: f.JobID, JobName: f.JobName,
			Status: f.Status, Output: f.Output, StartedAt: f.StartedAt,
		})
	}
	return runs, nil
}
