package tokens

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

const migracion = `
CREATE TABLE IF NOT EXISTS hl_api_token (
	id            uuid PRIMARY KEY,
	owner_id      text NOT NULL,
	nombre        text NOT NULL,
	alcance       text NOT NULL DEFAULT 'read',
	token_hash    text NOT NULL UNIQUE,
	prefijo       text NOT NULL,
	created_at    timestamp NOT NULL,
	last_used_at  timestamp,
	revoked_at    timestamp
);
CREATE INDEX IF NOT EXISTS hl_api_token_owner ON hl_api_token (owner_id);
`

// Migrar crea la tabla si no existe (idempotente; se llama al arrancar).
func Migrar(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, migracion)
	return err
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) Crear(ctx context.Context, t Token, huella string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hl_api_token (id, owner_id, nombre, alcance, token_hash, prefijo, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		t.ID, t.DuenoID, t.Nombre, string(t.Alcance), huella, t.Prefijo, t.CreadoEn)
	return err
}

type fila struct {
	ID         string     `db:"id"`
	OwnerID    string     `db:"owner_id"`
	Nombre     string     `db:"nombre"`
	Alcance    string     `db:"alcance"`
	Prefijo    string     `db:"prefijo"`
	CreatedAt  time.Time  `db:"created_at"`
	LastUsedAt *time.Time `db:"last_used_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
}

func (r *PostgresRepo) Listar(ctx context.Context, duenoID string) ([]Token, error) {
	var filas []fila
	err := r.db.SelectContext(ctx, &filas, `
		SELECT id, owner_id, nombre, alcance, prefijo, created_at, last_used_at, revoked_at
		  FROM hl_api_token WHERE owner_id = $1
		 ORDER BY (revoked_at IS NULL) DESC, created_at DESC`, duenoID)
	if err != nil {
		return nil, err
	}
	salida := make([]Token, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Token{
			ID: f.ID, DuenoID: f.OwnerID, Nombre: f.Nombre, Alcance: Alcance(f.Alcance),
			Prefijo: f.Prefijo, CreadoEn: f.CreatedAt, UsadoEn: f.LastUsedAt, RevocadoEn: f.RevokedAt,
		})
	}
	return salida, nil
}

func (r *PostgresRepo) ContarActivos(ctx context.Context, duenoID string) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n,
		`SELECT count(*) FROM hl_api_token WHERE owner_id = $1 AND revoked_at IS NULL`, duenoID)
	return n, err
}

func (r *PostgresRepo) Revocar(ctx context.Context, duenoID, id string, cuando time.Time) (bool, error) {
	// Un id que no es uuid no existe: se evita el error de conversión de Postgres.
	res, err := r.db.ExecContext(ctx, `
		UPDATE hl_api_token SET revoked_at = $3
		 WHERE owner_id = $1 AND revoked_at IS NULL AND id::text = $2`, duenoID, id, cuando)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *PostgresRepo) BuscarPorHuella(ctx context.Context, huella string) (*Identidad, error) {
	var f struct {
		ID      string `db:"id"`
		Dueno   string `db:"owner_id"`
		Alcance string `db:"alcance"`
		Nombre  string `db:"nombre"`
		Rol     string `db:"role"`
	}
	err := r.db.GetContext(ctx, &f, `
		SELECT t.id, t.owner_id, t.alcance, COALESCE(NULLIF(u.name, ''), u.email, '') AS nombre, u.role
		  FROM hl_api_token t JOIN usuarioslab u ON u.id = t.owner_id
		 WHERE t.token_hash = $1 AND t.revoked_at IS NULL AND u.status = 'approved'`, huella)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &Identidad{
		TokenID: f.ID, UsuarioID: f.Dueno, Nombre: f.Nombre,
		Admin: f.Rol == "superadmin", Alcance: Alcance(f.Alcance),
	}, nil
}

// MarcarUso anota la última vez que se usó, como mucho una vez por minuto: así
// una ráfaga de llamadas no se convierte en una ráfaga de escrituras.
func (r *PostgresRepo) MarcarUso(ctx context.Context, id string, cuando time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE hl_api_token SET last_used_at = $2::timestamp
		 WHERE id = $1::uuid AND (last_used_at IS NULL OR last_used_at < $2::timestamp - interval '1 minute')`, id, cuando)
	return err
}
