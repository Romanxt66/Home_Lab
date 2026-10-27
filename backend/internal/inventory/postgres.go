package inventory

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	tablaUbicacion  = `lab_inventory_location`
	tablaArticulo   = `lab_inventory_item`
	tablaMovimiento = `lab_inventory_movement`
)

// ── Ubicaciones ────────────────────────────────────────────────────────────

type filaUbicacion struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Kind      string    `db:"kind"`
	ParentID  *string   `db:"parentId"`
	Notes     *string   `db:"notes"`
	CreatedAt time.Time `db:"createdAt"`
	UpdatedAt time.Time `db:"updatedAt"`
}

func (f filaUbicacion) aDominio() Location {
	return Location{
		ID: f.ID, Name: f.Name, Kind: LocationKind(f.Kind),
		ParentID: f.ParentID, Notes: f.Notes,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

const colUbicacion = `id, name, kind, "parentId", notes, "createdAt", "updatedAt"`

type PostgresLocationRepo struct{ db *sqlx.DB }

func NewPostgresLocationRepo(db *sqlx.DB) *PostgresLocationRepo {
	return &PostgresLocationRepo{db: db}
}

func (r *PostgresLocationRepo) List(ctx context.Context) ([]Location, error) {
	var filas []filaUbicacion
	if err := r.db.SelectContext(ctx, &filas,
		`SELECT `+colUbicacion+` FROM `+tablaUbicacion+` ORDER BY name`); err != nil {
		return nil, err
	}

	lista := make([]Location, 0, len(filas))
	for _, f := range filas {
		lista = append(lista, f.aDominio())
	}
	return lista, nil
}

func (r *PostgresLocationRepo) Get(ctx context.Context, id string) (*Location, error) {
	var f filaUbicacion
	err := r.db.GetContext(ctx, &f,
		`SELECT `+colUbicacion+` FROM `+tablaUbicacion+` WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	l := f.aDominio()
	return &l, nil
}

func (r *PostgresLocationRepo) Create(ctx context.Context, in LocationInput) (*Location, error) {
	ahora := time.Now().UTC()
	var f filaUbicacion
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaUbicacion+` (id, name, kind, "parentId", notes, "createdAt", "updatedAt")
		VALUES ($1,$2,$3,$4,$5,$6,$6) RETURNING `+colUbicacion,
		uuid.NewString(), in.Name, string(in.Kind), in.ParentID, in.Notes, ahora)
	if err != nil {
		return nil, err
	}
	l := f.aDominio()
	return &l, nil
}

func (r *PostgresLocationRepo) Update(ctx context.Context, id string, in LocationInput) (*Location, error) {
	var f filaUbicacion
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tablaUbicacion+` SET name = $2, kind = $3, "parentId" = $4,
			notes = $5, "updatedAt" = $6
		WHERE id = $1 RETURNING `+colUbicacion,
		id, in.Name, string(in.Kind), in.ParentID, in.Notes, time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	l := f.aDominio()
	return &l, nil
}

func (r *PostgresLocationRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaUbicacion+` WHERE id = $1`, id)
	return err
}

// ItemCounts cuenta los artículos de cada ubicación en una sola consulta, en
// vez de una por ubicación, que es lo que convierte un árbol en un problema.
func (r *PostgresLocationRepo) ItemCounts(ctx context.Context) (map[string]int, error) {
	type par struct {
		LocationID string `db:"locationId"`
		Total      int    `db:"total"`
	}

	var filas []par
	if err := r.db.SelectContext(ctx, &filas, `
		SELECT "locationId", COUNT(*) AS total FROM `+tablaArticulo+`
		WHERE "locationId" IS NOT NULL GROUP BY "locationId"`); err != nil {
		return nil, err
	}

	conteos := make(map[string]int, len(filas))
	for _, f := range filas {
		conteos[f.LocationID] = f.Total
	}
	return conteos, nil
}

// ── Artículos ──────────────────────────────────────────────────────────────

type filaArticulo struct {
	ID         string    `db:"id"`
	Name       string    `db:"name"`
	Category   *string   `db:"category"`
	Quantity   int       `db:"quantity"`
	Unit       *string   `db:"unit"`
	LocationID *string   `db:"locationId"`
	Notes      *string   `db:"notes"`
	CreatedAt  time.Time `db:"createdAt"`
	UpdatedAt  time.Time `db:"updatedAt"`
}

func (f filaArticulo) aDominio() Item {
	return Item{
		ID: f.ID, Name: f.Name, Category: f.Category, Quantity: f.Quantity,
		Unit: f.Unit, LocationID: f.LocationID, Notes: f.Notes,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

const colArticulo = `id, name, category, quantity, unit, "locationId", notes,
	"createdAt", "updatedAt"`

type PostgresItemRepo struct{ db *sqlx.DB }

func NewPostgresItemRepo(db *sqlx.DB) *PostgresItemRepo {
	return &PostgresItemRepo{db: db}
}

func (r *PostgresItemRepo) List(ctx context.Context, locationID string) ([]Item, error) {
	query := `SELECT ` + colArticulo + ` FROM ` + tablaArticulo
	args := []any{}

	if locationID != "" {
		query += ` WHERE "locationId" = $1`
		args = append(args, locationID)
	}
	query += ` ORDER BY name`

	var filas []filaArticulo
	if err := r.db.SelectContext(ctx, &filas, query, args...); err != nil {
		return nil, err
	}

	lista := make([]Item, 0, len(filas))
	for _, f := range filas {
		lista = append(lista, f.aDominio())
	}
	return lista, nil
}

func (r *PostgresItemRepo) Get(ctx context.Context, id string) (*Item, error) {
	var f filaArticulo
	err := r.db.GetContext(ctx, &f,
		`SELECT `+colArticulo+` FROM `+tablaArticulo+` WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i := f.aDominio()
	return &i, nil
}

func (r *PostgresItemRepo) Create(ctx context.Context, in ItemInput) (*Item, error) {
	ahora := time.Now().UTC()
	var f filaArticulo
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaArticulo+` (id, name, category, quantity, unit,
			"locationId", notes, "createdAt", "updatedAt")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8) RETURNING `+colArticulo,
		uuid.NewString(), in.Name, in.Category, in.Quantity, in.Unit,
		in.LocationID, in.Notes, ahora)
	if err != nil {
		return nil, err
	}
	i := f.aDominio()
	return &i, nil
}

// Update no toca la cantidad: esa solo cambia por movimientos, que dejan
// rastro de quién, cuándo y por qué.
func (r *PostgresItemRepo) Update(ctx context.Context, id string, in ItemInput) (*Item, error) {
	var f filaArticulo
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tablaArticulo+` SET name = $2, category = $3, unit = $4,
			"locationId" = $5, notes = $6, "updatedAt" = $7
		WHERE id = $1 RETURNING `+colArticulo,
		id, in.Name, in.Category, in.Unit, in.LocationID, in.Notes, time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i := f.aDominio()
	return &i, nil
}

func (r *PostgresItemRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaArticulo+` WHERE id = $1`, id)
	return err
}

func (r *PostgresItemRepo) Categories(ctx context.Context) ([]string, error) {
	var categorias []string
	err := r.db.SelectContext(ctx, &categorias, `
		SELECT DISTINCT category FROM `+tablaArticulo+`
		WHERE category IS NOT NULL AND category <> '' ORDER BY category`)
	if err != nil {
		return nil, err
	}
	if categorias == nil {
		categorias = []string{}
	}
	return categorias, nil
}

// ── Movimientos ────────────────────────────────────────────────────────────

type filaMovimiento struct {
	ID           string    `db:"id"`
	ItemID       string    `db:"itemId"`
	Kind         string    `db:"kind"`
	Delta        int       `db:"delta"`
	ToLocationID *string   `db:"toLocationId"`
	Notes        *string   `db:"notes"`
	OccurredAt   time.Time `db:"occurredAt"`
	CreatedAt    time.Time `db:"createdAt"`
}

func (f filaMovimiento) aDominio() Movement {
	return Movement{
		ID: f.ID, ItemID: f.ItemID, Kind: MovementKind(f.Kind), Delta: f.Delta,
		ToLocationID: f.ToLocationID, Notes: f.Notes,
		OccurredAt: f.OccurredAt, CreatedAt: f.CreatedAt,
	}
}

const colMovimiento = `id, "itemId", kind, delta, "toLocationId", notes,
	"occurredAt", "createdAt"`

type PostgresMovementRepo struct{ db *sqlx.DB }

func NewPostgresMovementRepo(db *sqlx.DB) *PostgresMovementRepo {
	return &PostgresMovementRepo{db: db}
}

// Record guarda el movimiento y actualiza el artículo DENTRO DE UNA
// TRANSACCIÓN. Son dos escrituras que solo tienen sentido juntas: si quedara
// el movimiento sin el ajuste, el inventario mentiría.
func (r *PostgresMovementRepo) Record(ctx context.Context, m ValidatedMovement) (*Movement, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Si algo falla antes del Commit, esto deshace todo. Tras el Commit no
	// tiene efecto, así que es seguro dejarlo siempre.
	defer func() { _ = tx.Rollback() }()

	ahora := time.Now().UTC()
	var f filaMovimiento
	err = tx.GetContext(ctx, &f, `
		INSERT INTO `+tablaMovimiento+` (id, "itemId", kind, delta,
			"toLocationId", notes, "occurredAt", "createdAt")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+colMovimiento,
		uuid.NewString(), m.ItemID, string(m.Kind), m.Delta,
		m.ToLocationID, m.Notes, m.OccurredAt, ahora)
	if err != nil {
		return nil, err
	}

	// La cantidad se ajusta en la propia base (quantity + $2) y no leyendo y
	// escribiendo desde Go: así dos movimientos a la vez no se pisan.
	if _, err = tx.ExecContext(ctx, `
		UPDATE `+tablaArticulo+` SET quantity = quantity + $2, "updatedAt" = $3
		WHERE id = $1`, m.ItemID, m.Delta, ahora); err != nil {
		return nil, err
	}

	// Un movimiento de tipo "moved" también cambia dónde está el artículo.
	if m.Kind == MovMoved && m.ToLocationID != nil {
		if _, err = tx.ExecContext(ctx, `
			UPDATE `+tablaArticulo+` SET "locationId" = $2, "updatedAt" = $3
			WHERE id = $1`, m.ItemID, *m.ToLocationID, ahora); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	mov := f.aDominio()
	return &mov, nil
}

func (r *PostgresMovementRepo) ListForItem(ctx context.Context, itemID string, limit int) ([]Movement, error) {
	return r.consultar(ctx, `SELECT `+colMovimiento+` FROM `+tablaMovimiento+`
		WHERE "itemId" = $1 ORDER BY "occurredAt" DESC LIMIT $2`, itemID, limit)
}

func (r *PostgresMovementRepo) ListRecent(ctx context.Context, limit int) ([]Movement, error) {
	return r.consultar(ctx, `SELECT `+colMovimiento+` FROM `+tablaMovimiento+`
		ORDER BY "occurredAt" DESC LIMIT $1`, limit)
}

func (r *PostgresMovementRepo) consultar(ctx context.Context, query string, args ...any) ([]Movement, error) {
	var filas []filaMovimiento
	if err := r.db.SelectContext(ctx, &filas, query, args...); err != nil {
		return nil, err
	}

	lista := make([]Movement, 0, len(filas))
	for _, f := range filas {
		lista = append(lista, f.aDominio())
	}
	return lista, nil
}
