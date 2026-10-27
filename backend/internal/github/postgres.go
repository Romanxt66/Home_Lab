package github

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const tabla = `lab_github_config`

const columnas = `id, username, token, active, "createdAt", "updatedAt"`

type fila struct {
	ID        string    `db:"id"`
	Username  string    `db:"username"`
	Token     *string   `db:"token"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"createdAt"`
	UpdatedAt time.Time `db:"updatedAt"`
}

func (f fila) aDominio() Config {
	return Config{
		ID: f.ID, Username: f.Username, Token: f.Token,
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

// Upsert guarda la conexión y apaga las demás, en una transacción: si la
// segunda escritura fallara sola, quedarían dos activas y se elegiría una
// al azar.
//
// Un token vacío deja el que ya había: así se puede corregir el usuario sin
// tener que volver a pegar el token, que es lo incómodo.
func (r *PostgresRepo) Upsert(ctx context.Context, username string, token *string) (*Config, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var f fila
	err = tx.GetContext(ctx, &f,
		`UPDATE `+tabla+` SET username = $1, token = COALESCE($2, token),
			active = true, "updatedAt" = NOW()
		 WHERE username = $1 RETURNING `+columnas, username, token)

	if errors.Is(err, sql.ErrNoRows) {
		err = tx.GetContext(ctx, &f,
			`INSERT INTO `+tabla+` (id, username, token, active, "createdAt", "updatedAt")
			 VALUES ($1, $2, $3, true, NOW(), NOW()) RETURNING `+columnas,
			uuid.NewString(), username, token)
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
