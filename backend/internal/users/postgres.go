package users

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// La tabla la comparte con el Lab, por eso los nombres en camelCase van
// entrecomillados: PostgreSQL los pasaría a minúsculas si no.
const columnas = `id, email, name, picture, "passwordHash", role, status, "createdAt"`

type fila struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	Name         *string   `db:"name"`
	Picture      *string   `db:"picture"`
	PasswordHash *string   `db:"passwordHash"`
	Role         string    `db:"role"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"createdAt"`
}

func (f fila) aDominio() User {
	return User{
		ID:           f.ID,
		Email:        f.Email,
		Name:         f.Name,
		Picture:      f.Picture,
		PasswordHash: f.PasswordHash,
		Role:         Role(f.Role),
		Status:       Status(f.Status),
		CreatedAt:    f.CreatedAt,
	}
}

// PostgresRepo implementa Repo sobre la tabla usuarioslab.
type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) List(ctx context.Context) ([]User, error) {
	var filas []fila
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnas+` FROM usuarioslab ORDER BY "createdAt" DESC`)
	if err != nil {
		return nil, err
	}

	usuarios := make([]User, 0, len(filas))
	for _, f := range filas {
		usuarios = append(usuarios, f.aDominio())
	}
	return usuarios, nil
}

func (r *PostgresRepo) UpdateStatus(ctx context.Context, id string, status Status) (*User, error) {
	return r.actualizar(ctx, `UPDATE usuarioslab SET status = $2 WHERE id = $1`, id, string(status))
}

func (r *PostgresRepo) UpdateRole(ctx context.Context, id string, role Role) (*User, error) {
	return r.actualizar(ctx, `UPDATE usuarioslab SET role = $2 WHERE id = $1`, id, string(role))
}

// actualizar aplica el cambio y devuelve la fila resultante. Un id que no
// existe devuelve nil, no un error: que el usuario se haya borrado mientras
// tanto es un caso normal, no un fallo técnico.
func (r *PostgresRepo) actualizar(ctx context.Context, consulta, id, valor string) (*User, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, consulta+` RETURNING `+columnas, id, valor)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	u := f.aDominio()
	return &u, nil
}
