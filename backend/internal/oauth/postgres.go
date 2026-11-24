package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

const migracion = `
CREATE TABLE IF NOT EXISTS hl_oauth_client (
	id            text PRIMARY KEY,
	nombre        text NOT NULL,
	redirect_uris text NOT NULL,
	created_at    timestamp NOT NULL
);
CREATE TABLE IF NOT EXISTS hl_oauth_code (
	hash         text PRIMARY KEY,
	client_id    text NOT NULL,
	user_id      text NOT NULL,
	redirect_uri text NOT NULL,
	challenge    text NOT NULL,
	alcance      text NOT NULL,
	recurso      text NOT NULL DEFAULT '',
	expires_at   timestamp NOT NULL,
	used_at      timestamp
);
CREATE TABLE IF NOT EXISTS hl_oauth_grant (
	id           uuid PRIMARY KEY,
	user_id      text NOT NULL,
	client_id    text NOT NULL,
	alcance      text NOT NULL,
	created_at   timestamp NOT NULL,
	last_used_at timestamp,
	revoked_at   timestamp
);
CREATE INDEX IF NOT EXISTS hl_oauth_grant_user ON hl_oauth_grant (user_id);
CREATE TABLE IF NOT EXISTS hl_oauth_token (
	hash       text PRIMARY KEY,
	grant_id   uuid NOT NULL REFERENCES hl_oauth_grant(id) ON DELETE CASCADE,
	tipo       text NOT NULL,
	expires_at timestamp NOT NULL,
	used_at    timestamp,
	created_at timestamp NOT NULL
);
CREATE INDEX IF NOT EXISTS hl_oauth_token_grant ON hl_oauth_token (grant_id);
`

// Migrar crea las tablas si no existen (idempotente; se llama al arrancar).
func Migrar(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, migracion)
	return err
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) CrearCliente(ctx context.Context, c Cliente) error {
	uris, _ := json.Marshal(c.RedirectURIs)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO hl_oauth_client (id, nombre, redirect_uris, created_at) VALUES ($1,$2,$3,$4::timestamp)`,
		c.ID, c.Nombre, string(uris), c.CreadoEn)
	return err
}

func (r *PostgresRepo) ObtenerCliente(ctx context.Context, id string) (*Cliente, error) {
	var f struct {
		ID        string    `db:"id"`
		Nombre    string    `db:"nombre"`
		URIs      string    `db:"redirect_uris"`
		CreatedAt time.Time `db:"created_at"`
	}
	err := r.db.GetContext(ctx, &f, `SELECT id, nombre, redirect_uris, created_at FROM hl_oauth_client WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := Cliente{ID: f.ID, Nombre: f.Nombre, CreadoEn: f.CreatedAt}
	if err := json.Unmarshal([]byte(f.URIs), &c.RedirectURIs); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PostgresRepo) ContarClientes(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, `SELECT count(*) FROM hl_oauth_client`)
	return n, err
}

func (r *PostgresRepo) LimpiarClientesSinUso(ctx context.Context, antes time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM hl_oauth_client c
		 WHERE c.created_at < $1::timestamp
		   AND NOT EXISTS (SELECT 1 FROM hl_oauth_grant g WHERE g.client_id = c.id)`, antes)
	return err
}

func (r *PostgresRepo) GuardarCodigo(ctx context.Context, c Codigo) error {
	// De paso se tiran los códigos viejos: caducan a los 5 minutos y no sirven de nada después.
	_, _ = r.db.ExecContext(ctx, `DELETE FROM hl_oauth_code WHERE expires_at < $1::timestamp`, c.Caduca.Add(-DuracionCodigo-24*time.Hour))
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO hl_oauth_code (hash, client_id, user_id, redirect_uri, challenge, alcance, recurso, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::timestamp)`,
		c.Huella, c.ClienteID, c.UsuarioID, c.RedirectURI, c.Challenge, string(c.Alcance), c.Recurso, c.Caduca)
	return err
}

func (r *PostgresRepo) CanjearCodigo(ctx context.Context, huella string, ahora time.Time) (*Codigo, error) {
	var f struct {
		ClienteID   string    `db:"client_id"`
		UsuarioID   string    `db:"user_id"`
		RedirectURI string    `db:"redirect_uri"`
		Challenge   string    `db:"challenge"`
		Alcance     string    `db:"alcance"`
		Recurso     string    `db:"recurso"`
		Caduca      time.Time `db:"expires_at"`
	}
	// Un único UPDATE ... RETURNING: dos canjes a la vez no pueden ganar los dos.
	err := r.db.GetContext(ctx, &f, `
		UPDATE hl_oauth_code SET used_at = $2::timestamp
		 WHERE hash = $1 AND used_at IS NULL AND expires_at > $2::timestamp
		RETURNING client_id, user_id, redirect_uri, challenge, alcance, recurso, expires_at`, huella, ahora)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Codigo{
		Huella: huella, ClienteID: f.ClienteID, UsuarioID: f.UsuarioID, RedirectURI: f.RedirectURI,
		Challenge: f.Challenge, Alcance: Alcance(f.Alcance), Recurso: f.Recurso, Caduca: f.Caduca,
	}, nil
}

func (r *PostgresRepo) CrearConcesion(ctx context.Context, c Concesion, acceso, refresco RegistroToken) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO hl_oauth_grant (id, user_id, client_id, alcance, created_at)
		VALUES ($1::uuid,$2,$3,$4,$5::timestamp)`,
		c.ID, c.UsuarioID, c.ClienteID, string(c.Alcance), c.CreadaEn); err != nil {
		return err
	}
	if err := insertarTokens(ctx, tx, c.ID, c.CreadaEn, acceso, refresco); err != nil {
		return err
	}
	// Se aprovecha para tirar los tokens caducados hace más de una semana.
	_, _ = tx.ExecContext(ctx, `DELETE FROM hl_oauth_token WHERE expires_at < $1::timestamp`, c.CreadaEn.Add(-7*24*time.Hour))
	return tx.Commit()
}

func insertarTokens(ctx context.Context, tx *sqlx.Tx, grantID string, ahora time.Time, acceso, refresco RegistroToken) error {
	for _, t := range []struct {
		reg  RegistroToken
		tipo string
	}{{acceso, "access"}, {refresco, "refresh"}} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO hl_oauth_token (hash, grant_id, tipo, expires_at, created_at)
			VALUES ($1,$2::uuid,$3,$4::timestamp,$5::timestamp)`, t.reg.Huella, grantID, t.tipo, t.reg.Caduca, ahora); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepo) Rotar(ctx context.Context, huella string, ahora time.Time, acceso, refresco RegistroToken) (*Concesion, ResultadoRotacion, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, Invalido, err
	}
	defer func() { _ = tx.Rollback() }()

	var f struct {
		GrantID   string     `db:"grant_id"`
		UsedAt    *time.Time `db:"used_at"`
		ExpiresAt time.Time  `db:"expires_at"`
		Revoked   *time.Time `db:"revoked_at"`
		UserID    string     `db:"user_id"`
		ClientID  string     `db:"client_id"`
		Alcance   string     `db:"alcance"`
		Status    string     `db:"status"`
	}
	// FOR UPDATE: dos refrescos a la vez con el mismo token se ordenan, y el
	// segundo ve el token ya usado (y se trata como reutilización).
	err = tx.GetContext(ctx, &f, `
		SELECT t.grant_id::text AS grant_id, t.used_at, t.expires_at, g.revoked_at, g.user_id, g.client_id, g.alcance, COALESCE(u.status, '') AS status
		  FROM hl_oauth_token t
		  JOIN hl_oauth_grant g ON g.id = t.grant_id
		  LEFT JOIN usuarioslab u ON u.id = g.user_id
		 WHERE t.hash = $1 AND t.tipo = 'refresh'
		   FOR UPDATE OF t`, huella)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, Invalido, nil
	}
	if err != nil {
		return nil, Invalido, err
	}
	if f.Revoked != nil {
		return nil, Invalido, nil
	}
	if f.UsedAt != nil {
		// Reutilización: se revoca toda la concesión y se confirma el cambio.
		if _, err := tx.ExecContext(ctx, `UPDATE hl_oauth_grant SET revoked_at = $2::timestamp WHERE id = $1::uuid AND revoked_at IS NULL`, f.GrantID, ahora); err != nil {
			return nil, Invalido, err
		}
		if err := tx.Commit(); err != nil {
			return nil, Invalido, err
		}
		return nil, Reutilizado, nil
	}
	if !f.ExpiresAt.After(ahora) || f.Status != "approved" {
		return nil, Invalido, nil
	}

	if _, err := tx.ExecContext(ctx, `UPDATE hl_oauth_token SET used_at = $2::timestamp WHERE hash = $1`, huella, ahora); err != nil {
		return nil, Invalido, err
	}
	if err := insertarTokens(ctx, tx, f.GrantID, ahora, acceso, refresco); err != nil {
		return nil, Invalido, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE hl_oauth_grant SET last_used_at = $2::timestamp WHERE id = $1::uuid`, f.GrantID, ahora); err != nil {
		return nil, Invalido, err
	}
	if err := tx.Commit(); err != nil {
		return nil, Invalido, err
	}
	return &Concesion{ID: f.GrantID, UsuarioID: f.UserID, ClienteID: f.ClientID, Alcance: Alcance(f.Alcance)}, Rotado, nil
}

func (r *PostgresRepo) BuscarAcceso(ctx context.Context, huella string, ahora time.Time) (*tokens.Identidad, error) {
	var f struct {
		GrantID string `db:"grant_id"`
		UserID  string `db:"user_id"`
		Alcance string `db:"alcance"`
		Nombre  string `db:"nombre"`
		Rol     string `db:"role"`
	}
	err := r.db.GetContext(ctx, &f, `
		SELECT g.id::text AS grant_id, g.user_id, g.alcance, COALESCE(NULLIF(u.name, ''), u.email, '') AS nombre, u.role
		  FROM hl_oauth_token t
		  JOIN hl_oauth_grant g ON g.id = t.grant_id
		  JOIN usuarioslab u ON u.id = g.user_id
		 WHERE t.hash = $1 AND t.tipo = 'access' AND t.expires_at > $2::timestamp
		   AND g.revoked_at IS NULL AND u.status = 'approved'`, huella, ahora)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Como mucho una anotación por minuto: no se convierte cada llamada en una escritura.
	_, _ = r.db.ExecContext(ctx, `
		UPDATE hl_oauth_grant SET last_used_at = $2::timestamp
		 WHERE id = $1::uuid AND (last_used_at IS NULL OR last_used_at < $2::timestamp - interval '1 minute')`, f.GrantID, ahora)
	return &tokens.Identidad{
		TokenID: f.GrantID, UsuarioID: f.UserID, Nombre: f.Nombre,
		Admin: f.Rol == "superadmin", Alcance: tokens.Alcance(f.Alcance),
	}, nil
}

func (r *PostgresRepo) ListarConcesiones(ctx context.Context, usuarioID string) ([]Concesion, error) {
	var filas []struct {
		ID        string     `db:"id"`
		ClientID  string     `db:"client_id"`
		Nombre    string     `db:"nombre"`
		Alcance   string     `db:"alcance"`
		CreatedAt time.Time  `db:"created_at"`
		LastUsed  *time.Time `db:"last_used_at"`
	}
	err := r.db.SelectContext(ctx, &filas, `
		SELECT g.id::text AS id, g.client_id, COALESCE(c.nombre, 'Aplicación eliminada') AS nombre, g.alcance, g.created_at, g.last_used_at
		  FROM hl_oauth_grant g LEFT JOIN hl_oauth_client c ON c.id = g.client_id
		 WHERE g.user_id = $1 AND g.revoked_at IS NULL
		 ORDER BY g.created_at DESC`, usuarioID)
	if err != nil {
		return nil, err
	}
	salida := make([]Concesion, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Concesion{
			ID: f.ID, UsuarioID: usuarioID, ClienteID: f.ClientID, ClienteNombre: f.Nombre,
			Alcance: Alcance(f.Alcance), CreadaEn: f.CreatedAt, UsadaEn: f.LastUsed,
		})
	}
	return salida, nil
}

func (r *PostgresRepo) RevocarConcesion(ctx context.Context, usuarioID, id string, ahora time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE hl_oauth_grant SET revoked_at = $3::timestamp
		 WHERE user_id = $1 AND id::text = $2 AND revoked_at IS NULL`, usuarioID, id, ahora)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *PostgresRepo) RevocarPorToken(ctx context.Context, huella, clienteID string, ahora time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE hl_oauth_grant SET revoked_at = $3::timestamp
		 WHERE revoked_at IS NULL AND client_id = $2
		   AND id IN (SELECT grant_id FROM hl_oauth_token WHERE hash = $1)`, huella, clienteID, ahora)
	return err
}
