package uptime

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Las tablas son las MISMAS que usa el Lab en Next. No se migra nada: las dos
// aplicaciones leen y escriben en el esquema "lab" de la base de siempre.
const (
	tablaMonitor = `lab_uptime_monitor`
	tablaCheck   = `lab_uptime_check`
)

// filaMonitor refleja la tabla tal cual. Se mantiene aparte del Monitor del
// dominio a propósito: así un cambio de columnas no se filtra al dominio.
type filaMonitor struct {
	ID                  string     `db:"id"`
	Name                string     `db:"name"`
	URL                 string     `db:"url"`
	Method              string     `db:"method"`
	ExpectedStatus      int        `db:"expectedStatus"`
	IntervalSeconds     int        `db:"intervalSeconds"`
	TimeoutMs           int        `db:"timeoutMs"`
	Active              bool       `db:"active"`
	NotifyOnFailure     bool       `db:"notifyOnFailure"`
	LastStatus          string     `db:"lastStatus"`
	LastCheckedAt       *time.Time `db:"lastCheckedAt"`
	LastResponseMs      *int       `db:"lastResponseMs"`
	LastStatusCode      *int       `db:"lastStatusCode"`
	LastError           *string    `db:"lastError"`
	ConsecutiveFailures int        `db:"consecutiveFailures"`
	CreatedAt           time.Time  `db:"createdAt"`
	UpdatedAt           time.Time  `db:"updatedAt"`
}

func (f filaMonitor) aDominio() Monitor {
	return Monitor{
		ID:                  f.ID,
		Name:                f.Name,
		URL:                 f.URL,
		Method:              Method(f.Method),
		ExpectedStatus:      f.ExpectedStatus,
		IntervalSeconds:     f.IntervalSeconds,
		TimeoutMs:           f.TimeoutMs,
		Active:              f.Active,
		NotifyOnFailure:     f.NotifyOnFailure,
		LastStatus:          Status(f.LastStatus),
		LastCheckedAt:       f.LastCheckedAt,
		LastResponseMs:      f.LastResponseMs,
		LastStatusCode:      f.LastStatusCode,
		LastError:           f.LastError,
		ConsecutiveFailures: f.ConsecutiveFailures,
		CreatedAt:           f.CreatedAt,
		UpdatedAt:           f.UpdatedAt,
	}
}

// columnas lista los campos explícitamente en vez de usar SELECT *: si alguien
// añade una columna, esto sigue funcionando en lugar de romperse.
const columnas = `id, name, url, method, "expectedStatus", "intervalSeconds",
	"timeoutMs", active, "notifyOnFailure", "lastStatus", "lastCheckedAt",
	"lastResponseMs", "lastStatusCode", "lastError", "consecutiveFailures",
	"createdAt", "updatedAt"`

// PostgresMonitorRepo implementa MonitorRepo contra PostgreSQL.
type PostgresMonitorRepo struct {
	db *sqlx.DB
}

func NewPostgresMonitorRepo(db *sqlx.DB) *PostgresMonitorRepo {
	return &PostgresMonitorRepo{db: db}
}

func (r *PostgresMonitorRepo) List(ctx context.Context) ([]Monitor, error) {
	return r.consultar(ctx, `SELECT `+columnas+` FROM `+tablaMonitor+` ORDER BY name`)
}

func (r *PostgresMonitorRepo) ListActive(ctx context.Context) ([]Monitor, error) {
	return r.consultar(ctx, `SELECT `+columnas+` FROM `+tablaMonitor+`
		WHERE active = true ORDER BY name`)
}

func (r *PostgresMonitorRepo) consultar(ctx context.Context, query string, args ...any) ([]Monitor, error) {
	var filas []filaMonitor
	if err := r.db.SelectContext(ctx, &filas, query, args...); err != nil {
		return nil, err
	}

	monitores := make([]Monitor, 0, len(filas))
	for _, f := range filas {
		monitores = append(monitores, f.aDominio())
	}
	return monitores, nil
}

func (r *PostgresMonitorRepo) Get(ctx context.Context, id string) (*Monitor, error) {
	var fila filaMonitor
	err := r.db.GetContext(ctx, &fila,
		`SELECT `+columnas+` FROM `+tablaMonitor+` WHERE id = $1`, id)

	// "No existe" no es un error: es una respuesta legítima.
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	m := fila.aDominio()
	return &m, nil
}

func (r *PostgresMonitorRepo) Create(ctx context.Context, in MonitorInput) (*Monitor, error) {
	ahora := time.Now().UTC()
	var fila filaMonitor

	err := r.db.GetContext(ctx, &fila, `
		INSERT INTO `+tablaMonitor+` (
			id, name, url, method, "expectedStatus", "intervalSeconds", "timeoutMs",
			active, "notifyOnFailure", "lastStatus", "consecutiveFailures",
			"createdAt", "updatedAt")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'unknown',0,$10,$10)
		RETURNING `+columnas,
		uuid.NewString(), in.Name, in.URL, string(in.Method), in.ExpectedStatus,
		in.IntervalSeconds, in.TimeoutMs, in.Active, in.NotifyOnFailure, ahora)
	if err != nil {
		return nil, err
	}

	m := fila.aDominio()
	return &m, nil
}

func (r *PostgresMonitorRepo) Update(ctx context.Context, id string, in MonitorInput) (*Monitor, error) {
	var fila filaMonitor

	err := r.db.GetContext(ctx, &fila, `
		UPDATE `+tablaMonitor+` SET
			name = $2, url = $3, method = $4, "expectedStatus" = $5,
			"intervalSeconds" = $6, "timeoutMs" = $7, active = $8,
			"notifyOnFailure" = $9, "updatedAt" = $10
		WHERE id = $1
		RETURNING `+columnas,
		id, in.Name, in.URL, string(in.Method), in.ExpectedStatus,
		in.IntervalSeconds, in.TimeoutMs, in.Active, in.NotifyOnFailure,
		time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	m := fila.aDominio()
	return &m, nil
}

func (r *PostgresMonitorRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaMonitor+` WHERE id = $1`, id)
	return err
}

func (r *PostgresMonitorRepo) ApplyCheck(ctx context.Context, id string, p CheckPatch) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE `+tablaMonitor+` SET
			"lastStatus" = $2, "lastCheckedAt" = $3, "lastResponseMs" = $4,
			"lastStatusCode" = $5, "lastError" = $6, "consecutiveFailures" = $7,
			"updatedAt" = $3
		WHERE id = $1`,
		id, string(p.LastStatus), p.LastCheckedAt, p.LastResponseMs,
		p.LastStatusCode, p.LastError, p.ConsecutiveFailures)
	return err
}

// PostgresCheckRepo guarda el histórico de comprobaciones.
type PostgresCheckRepo struct {
	db *sqlx.DB
}

func NewPostgresCheckRepo(db *sqlx.DB) *PostgresCheckRepo {
	return &PostgresCheckRepo{db: db}
}

func (r *PostgresCheckRepo) Add(ctx context.Context, monitorID string, c Check) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO `+tablaCheck+` (id, "monitorId", ok, "statusCode", "responseMs", error, "checkedAt")
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		uuid.NewString(), monitorID, c.OK, c.StatusCode, c.ResponseMs, c.Error, c.CheckedAt)
	return err
}

func (r *PostgresCheckRepo) ListForMonitor(ctx context.Context, monitorID string, limit int) ([]Check, error) {
	type fila struct {
		OK         bool      `db:"ok"`
		StatusCode *int      `db:"statusCode"`
		ResponseMs *int      `db:"responseMs"`
		Error      *string   `db:"error"`
		CheckedAt  time.Time `db:"checkedAt"`
	}

	var filas []fila
	err := r.db.SelectContext(ctx, &filas, `
		SELECT ok, "statusCode", "responseMs", error, "checkedAt"
		FROM `+tablaCheck+`
		WHERE "monitorId" = $1
		ORDER BY "checkedAt" DESC
		LIMIT $2`, monitorID, limit)
	if err != nil {
		return nil, err
	}

	checks := make([]Check, 0, len(filas))
	for _, f := range filas {
		checks = append(checks, Check{
			OK:         f.OK,
			StatusCode: f.StatusCode,
			ResponseMs: f.ResponseMs,
			Error:      f.Error,
			CheckedAt:  f.CheckedAt,
		})
	}
	return checks, nil
}
