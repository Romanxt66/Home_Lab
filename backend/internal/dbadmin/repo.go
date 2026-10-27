package dbadmin

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const tabla = `lab_db_connection`

const columnas = `id, name, "connectionUrl", "readOnly", "createdAt", "updatedAt"`

type fila struct {
	ID            string    `db:"id"`
	Name          string    `db:"name"`
	ConnectionURL string    `db:"connectionUrl"`
	ReadOnly      bool      `db:"readOnly"`
	CreatedAt     time.Time `db:"createdAt"`
	UpdatedAt     time.Time `db:"updatedAt"`
}

func (f fila) aDominio() Conexion {
	return Conexion{
		ID: f.ID, Name: f.Name, ConnectionURL: f.ConnectionURL,
		ReadOnly: f.ReadOnly, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) List(ctx context.Context) ([]Conexion, error) {
	var filas []fila
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnas+` FROM `+tabla+` ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}

	salida := make([]Conexion, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

func (r *PostgresRepo) Get(ctx context.Context, id string) (*Conexion, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `SELECT `+columnas+` FROM `+tabla+` WHERE id = $1`, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := f.aDominio()
	return &c, nil
}

func (r *PostgresRepo) Create(ctx context.Context, c Conexion) (*Conexion, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tabla+` (id, name, "connectionUrl", "readOnly", "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING `+columnas,
		uuid.NewString(), c.Name, c.ConnectionURL, c.ReadOnly)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresRepo) Update(ctx context.Context, c Conexion) (*Conexion, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tabla+` SET name = $2, "connectionUrl" = $3, "readOnly" = $4, "updatedAt" = NOW()
		WHERE id = $1 RETURNING `+columnas,
		c.ID, c.Name, c.ConnectionURL, c.ReadOnly)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tabla+` WHERE id = $1`, id)
	return err
}
