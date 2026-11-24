package workspace

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Las tablas son propias de este módulo y llevan el prefijo hl_ para no
// confundirse con las del Lab, con el que se comparte la base.
const migracion = `
CREATE TABLE IF NOT EXISTS hl_workspace_item (
	id            uuid PRIMARY KEY,
	tipo          text NOT NULL,
	owner_id      text NOT NULL,
	nombre        text NOT NULL,
	descripcion   text NOT NULL DEFAULT '',
	visibilidad   text NOT NULL DEFAULT 'private',
	version       integer NOT NULL DEFAULT 1,
	archivos      integer NOT NULL DEFAULT 0,
	bytes         bigint  NOT NULL DEFAULT 0,
	created_at    timestamp NOT NULL,
	updated_at    timestamp NOT NULL,
	UNIQUE (owner_id, tipo, nombre)
);
CREATE TABLE IF NOT EXISTS hl_workspace_version (
	item_id      uuid NOT NULL REFERENCES hl_workspace_item(id) ON DELETE CASCADE,
	numero       integer NOT NULL,
	mensaje      text NOT NULL DEFAULT '',
	autor_id     text NOT NULL,
	descripcion  text NOT NULL DEFAULT '',
	archivos     integer NOT NULL DEFAULT 0,
	bytes        bigint  NOT NULL DEFAULT 0,
	created_at   timestamp NOT NULL,
	PRIMARY KEY (item_id, numero)
);
CREATE TABLE IF NOT EXISTS hl_workspace_file (
	item_id    uuid NOT NULL,
	numero     integer NOT NULL,
	ruta       text NOT NULL,
	contenido  bytea NOT NULL,
	PRIMARY KEY (item_id, numero, ruta),
	FOREIGN KEY (item_id, numero) REFERENCES hl_workspace_version(item_id, numero) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS hl_workspace_item_visibilidad ON hl_workspace_item (visibilidad, tipo);
`

// Migrar crea las tablas si no existen. Es idempotente y se llama al arrancar:
// la base no se gestiona con migraciones (es la del Lab), y esto es lo único
// que este servicio añade a ella.
func Migrar(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, migracion)
	return err
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo { return &PostgresRepo{db: db} }

const columnasElemento = `e.id, e.tipo, e.owner_id, COALESCE(NULLIF(u.name, ''), u.email, '') AS owner_nombre,
	e.nombre, e.descripcion, e.visibilidad, e.version, e.archivos, e.bytes, e.created_at, e.updated_at`

const desde = ` FROM hl_workspace_item e LEFT JOIN usuarioslab u ON u.id = e.owner_id `

type filaElemento struct {
	ID          string    `db:"id"`
	Tipo        string    `db:"tipo"`
	OwnerID     string    `db:"owner_id"`
	OwnerNombre string    `db:"owner_nombre"`
	Nombre      string    `db:"nombre"`
	Descripcion string    `db:"descripcion"`
	Visibilidad string    `db:"visibilidad"`
	Version     int       `db:"version"`
	Archivos    int       `db:"archivos"`
	Bytes       int64     `db:"bytes"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (f filaElemento) aDominio() Elemento {
	return Elemento{
		ID: f.ID, Tipo: Tipo(f.Tipo), DuenoID: f.OwnerID, DuenoNombre: f.OwnerNombre,
		Nombre: f.Nombre, Descripcion: f.Descripcion, Visibilidad: Visibilidad(f.Visibilidad),
		Version: f.Version, Archivos: f.Archivos, Bytes: f.Bytes,
		CreadoEn: f.CreatedAt, ActualizadoEn: f.UpdatedAt,
	}
}

func (r *PostgresRepo) Listar(ctx context.Context, uid string, tipo Tipo) ([]Elemento, error) {
	var filas []filaElemento
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnasElemento+desde+`
		 WHERE e.tipo = $1 AND (e.owner_id = $2 OR e.visibilidad = 'shared')
		 ORDER BY (e.owner_id = $2) DESC, e.nombre ASC`, string(tipo), uid)
	if err != nil {
		return nil, err
	}
	salida := make([]Elemento, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

func (r *PostgresRepo) Obtener(ctx context.Context, id string) (*Elemento, error) {
	// Un id que no es un uuid no existe: se responde igual que si faltara.
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var f filaElemento
	err := r.db.GetContext(ctx, &f, `SELECT `+columnasElemento+desde+` WHERE e.id = $1`, id)
	return unico(f, err)
}

func (r *PostgresRepo) BuscarPorNombre(ctx context.Context, duenoID string, tipo Tipo, nombre string) (*Elemento, error) {
	var f filaElemento
	err := r.db.GetContext(ctx, &f,
		`SELECT `+columnasElemento+desde+` WHERE e.owner_id = $1 AND e.tipo = $2 AND e.nombre = $3`,
		duenoID, string(tipo), nombre)
	return unico(f, err)
}

func unico(f filaElemento, err error) (*Elemento, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e := f.aDominio()
	return &e, nil
}

func (r *PostgresRepo) Crear(ctx context.Context, e Elemento, v Version, archivos []Archivo) (*Elemento, error) {
	id := uuid.NewString()
	ahora := time.Now().UTC()
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO hl_workspace_item
			(id, tipo, owner_id, nombre, descripcion, visibilidad, version, archivos, bytes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,1,$7,$8,$9,$9)`,
		id, string(e.Tipo), e.DuenoID, e.Nombre, v.Descripcion, string(e.Visibilidad),
		v.Archivos, v.Bytes, ahora); err != nil {
		return nil, err
	}
	if err := insertarVersion(ctx, tx, id, 1, v, archivos, ahora); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Obtener(ctx, id)
}

func (r *PostgresRepo) NuevaVersion(ctx context.Context, id string, v Version, archivos []Archivo) (*Elemento, error) {
	ahora := time.Now().UTC()
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// FOR UPDATE: dos guardados a la vez no pueden elegir el mismo número.
	var actual int
	if err := tx.GetContext(ctx, &actual, `SELECT version FROM hl_workspace_item WHERE id = $1 FOR UPDATE`, id); err != nil {
		return nil, err
	}
	n := actual + 1
	if err := insertarVersion(ctx, tx, id, n, v, archivos, ahora); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE hl_workspace_item
		   SET version = $2, descripcion = $3, archivos = $4, bytes = $5, updated_at = $6
		 WHERE id = $1`, id, n, v.Descripcion, v.Archivos, v.Bytes, ahora); err != nil {
		return nil, err
	}
	// Se conservan las últimas MaxVersiones; los archivos de las viejas caen en cascada.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM hl_workspace_version WHERE item_id = $1 AND numero <= $2`, id, n-MaxVersiones); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Obtener(ctx, id)
}

func insertarVersion(ctx context.Context, tx *sqlx.Tx, id string, n int, v Version, archivos []Archivo, ahora time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO hl_workspace_version (item_id, numero, mensaje, autor_id, descripcion, archivos, bytes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id, n, v.Mensaje, v.AutorID, v.Descripcion, v.Archivos, v.Bytes, ahora); err != nil {
		return err
	}
	for _, a := range archivos {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO hl_workspace_file (item_id, numero, ruta, contenido) VALUES ($1,$2,$3,$4)`,
			id, n, a.Ruta, a.Contenido); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepo) CambiarVisibilidad(ctx context.Context, id string, v Visibilidad) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE hl_workspace_item SET visibilidad = $2 WHERE id = $1`, id, string(v))
	return err
}

type filaVersion struct {
	Numero      int       `db:"numero"`
	Mensaje     string    `db:"mensaje"`
	AutorID     string    `db:"autor_id"`
	AutorNombre string    `db:"autor_nombre"`
	Descripcion string    `db:"descripcion"`
	Archivos    int       `db:"archivos"`
	Bytes       int64     `db:"bytes"`
	CreatedAt   time.Time `db:"created_at"`
}

func (r *PostgresRepo) Versiones(ctx context.Context, id string) ([]Version, error) {
	var filas []filaVersion
	err := r.db.SelectContext(ctx, &filas, `
		SELECT v.numero, v.mensaje, v.autor_id, COALESCE(NULLIF(u.name, ''), u.email, '') AS autor_nombre,
		       v.descripcion, v.archivos, v.bytes, v.created_at
		  FROM hl_workspace_version v LEFT JOIN usuarioslab u ON u.id = v.autor_id
		 WHERE v.item_id = $1 ORDER BY v.numero DESC`, id)
	if err != nil {
		return nil, err
	}
	salida := make([]Version, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Version{
			Numero: f.Numero, Mensaje: f.Mensaje, AutorID: f.AutorID, AutorNombre: f.AutorNombre,
			Descripcion: f.Descripcion, Archivos: f.Archivos, Bytes: f.Bytes, CreadaEn: f.CreatedAt,
		})
	}
	return salida, nil
}

func (r *PostgresRepo) Archivos(ctx context.Context, id string, version int) ([]Archivo, error) {
	type fila struct {
		Ruta      string `db:"ruta"`
		Contenido []byte `db:"contenido"`
	}
	var filas []fila
	err := r.db.SelectContext(ctx, &filas, `
		SELECT f.ruta, f.contenido
		  FROM hl_workspace_file f
		  JOIN hl_workspace_item e ON e.id = f.item_id
		 WHERE f.item_id = $1 AND f.numero = CASE WHEN $2 = 0 THEN e.version ELSE $2 END
		 ORDER BY f.ruta`, id, version)
	if err != nil {
		return nil, err
	}
	salida := make([]Archivo, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Archivo{Ruta: f.Ruta, Contenido: f.Contenido})
	}
	return salida, nil
}

func (r *PostgresRepo) Borrar(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM hl_workspace_item WHERE id = $1`, id)
	return err
}
