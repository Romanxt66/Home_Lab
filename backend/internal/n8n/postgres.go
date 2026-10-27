package n8n

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const tabla = `lab_n8n_config`

const columnas = `id, "baseUrl", "apiKey", active, "createdAt", "updatedAt"`

type fila struct {
	ID        string    `db:"id"`
	BaseURL   string    `db:"baseUrl"`
	APIKey    string    `db:"apiKey"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"createdAt"`
	UpdatedAt time.Time `db:"updatedAt"`
}

func (f fila) aDominio() Config {
	return Config{
		ID: f.ID, BaseURL: f.BaseURL, APIKey: f.APIKey,
		Active: f.Active, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) GetActive(ctx context.Context) (*Config, error) {
	var f fila
	err := r.db.GetContext(ctx, &f,
		`SELECT `+columnas+` FROM `+tabla+` WHERE active = true ORDER BY "createdAt" DESC LIMIT 1`)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := f.aDominio()
	return &c, nil
}

// Upsert guarda la conexión y apaga las demás, dentro de una transacción.
//
// Son dos escrituras que deben cuadrar: si la segunda fallara por su cuenta,
// quedarían dos conexiones activas y GetActive elegiría una al azar.
func (r *PostgresRepo) Upsert(ctx context.Context, baseURL, apiKey string) (*Config, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var f fila

	// Se reutiliza la fila de esa misma dirección si ya existe, para no
	// acumular una por cada vez que se reconecta.
	err = tx.GetContext(ctx, &f,
		`UPDATE `+tabla+` SET "apiKey" = $2, active = true, "updatedAt" = NOW()
		 WHERE "baseUrl" = $1 RETURNING `+columnas, baseURL, apiKey)

	if errors.Is(err, sql.ErrNoRows) {
		err = tx.GetContext(ctx, &f,
			`INSERT INTO `+tabla+` (id, "baseUrl", "apiKey", active, "createdAt", "updatedAt")
			 VALUES ($1, $2, $3, true, NOW(), NOW()) RETURNING `+columnas,
			uuid.NewString(), baseURL, apiKey)
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE `+tabla+` SET active = false, "updatedAt" = NOW()
		 WHERE id <> $1 AND active = true`, f.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	c := f.aDominio()
	return &c, nil
}

func (r *PostgresRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tabla+` WHERE id = $1`, id)
	return err
}
