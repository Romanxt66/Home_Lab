package finance

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	tablaCuenta      = `lab_financial_account`
	tablaCategoria   = `lab_transaction_category`
	tablaMovimiento  = `lab_financial_transaction`
	tablaPresupuesto = `lab_budget`
	tablaRecurrente  = `lab_recurring_payment`
)

// Los importes viajan como céntimos enteros en los dos sentidos: se
// multiplican y redondean en SQL. Traerlos como coma flotante y convertirlos
// en Go reintroduciría justo el error que el tipo Dinero evita.

// ── Cuentas ────────────────────────────────────────────────────────────────

const columnasCuenta = `id, name, kind, currency,
	ROUND("initialBalance" * 100)::bigint AS initial_cents,
	archived, "createdAt"`

type filaCuenta struct {
	ID           string    `db:"id"`
	Name         string    `db:"name"`
	Kind         string    `db:"kind"`
	Currency     string    `db:"currency"`
	InitialCents int64     `db:"initial_cents"`
	Archived     bool      `db:"archived"`
	CreatedAt    time.Time `db:"createdAt"`
}

func (f filaCuenta) aDominio() Cuenta {
	return Cuenta{
		ID: f.ID, Name: f.Name, Kind: TipoCuenta(f.Kind), Currency: f.Currency,
		InitialBalance: Dinero(f.InitialCents), Archived: f.Archived,
		CreatedAt: f.CreatedAt,
	}
}

type PostgresCuentaRepo struct {
	db *sqlx.DB
}

func NewPostgresCuentaRepo(db *sqlx.DB) *PostgresCuentaRepo {
	return &PostgresCuentaRepo{db: db}
}

func (r *PostgresCuentaRepo) List(ctx context.Context, incluirArchivadas bool) ([]Cuenta, error) {
	q := `SELECT ` + columnasCuenta + ` FROM ` + tablaCuenta
	if !incluirArchivadas {
		q += ` WHERE archived = false`
	}
	q += ` ORDER BY name ASC`

	var filas []filaCuenta
	if err := r.db.SelectContext(ctx, &filas, q); err != nil {
		return nil, err
	}

	salida := make([]Cuenta, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

type filaCuentaSaldo struct {
	filaCuenta
	SaldoCents int64 `db:"saldo_cents"`
}

// ConSaldos calcula el saldo de todas las cuentas en una sola consulta.
//
// El Lab pedía los movimientos de cada cuenta por separado y los sumaba en
// memoria: con diez cuentas eran once viajes a la base para pintar una
// pantalla. Aquí lo suma PostgreSQL, que es su trabajo.
func (r *PostgresCuentaRepo) ConSaldos(ctx context.Context, incluirArchivadas bool) ([]CuentaConSaldo, error) {
	filtro := ""
	if !incluirArchivadas {
		filtro = ` WHERE c.archived = false`
	}

	var filas []filaCuentaSaldo
	err := r.db.SelectContext(ctx, &filas, `
		SELECT
			c.id, c.name, c.kind, c.currency,
			ROUND(c."initialBalance" * 100)::bigint AS initial_cents,
			c.archived, c."createdAt",
			ROUND((c."initialBalance" + COALESCE(m.neto, 0)) * 100)::bigint AS saldo_cents
		FROM `+tablaCuenta+` c
		LEFT JOIN (
			SELECT "accountId",
				SUM(CASE WHEN kind = 'income' THEN amount ELSE -amount END) AS neto
			FROM `+tablaMovimiento+`
			GROUP BY "accountId"
		) m ON m."accountId" = c.id`+filtro+`
		ORDER BY c.name ASC`)
	if err != nil {
		return nil, err
	}

	salida := make([]CuentaConSaldo, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, CuentaConSaldo{
			Cuenta:  f.filaCuenta.aDominio(),
			Balance: Dinero(f.SaldoCents),
		})
	}
	return salida, nil
}

func (r *PostgresCuentaRepo) Get(ctx context.Context, id string) (*Cuenta, error) {
	var f filaCuenta
	err := r.db.GetContext(ctx, &f, `SELECT `+columnasCuenta+` FROM `+tablaCuenta+` WHERE id = $1`, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := f.aDominio()
	return &c, nil
}

func (r *PostgresCuentaRepo) Create(ctx context.Context, c Cuenta) (*Cuenta, error) {
	var f filaCuenta
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaCuenta+` (id, name, kind, currency, "initialBalance", archived, "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, $5::numeric / 100, $6, NOW(), NOW())
		RETURNING `+columnasCuenta,
		uuid.NewString(), c.Name, string(c.Kind), c.Currency, int64(c.InitialBalance), c.Archived)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresCuentaRepo) Update(ctx context.Context, c Cuenta) (*Cuenta, error) {
	var f filaCuenta
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tablaCuenta+` SET name = $2, kind = $3, currency = $4,
			"initialBalance" = $5::numeric / 100, archived = $6, "updatedAt" = NOW()
		WHERE id = $1 RETURNING `+columnasCuenta,
		c.ID, c.Name, string(c.Kind), c.Currency, int64(c.InitialBalance), c.Archived)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresCuentaRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaCuenta+` WHERE id = $1`, id)
	return err
}

// ── Categorías ─────────────────────────────────────────────────────────────

const columnasCategoria = `id, name, kind, color, "createdAt"`

type filaCategoria struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Kind      string    `db:"kind"`
	Color     *string   `db:"color"`
	CreatedAt time.Time `db:"createdAt"`
}

func (f filaCategoria) aDominio() Categoria {
	return Categoria{
		ID: f.ID, Name: f.Name, Kind: TipoMovimiento(f.Kind),
		Color: f.Color, CreatedAt: f.CreatedAt,
	}
}

type PostgresCategoriaRepo struct {
	db *sqlx.DB
}

func NewPostgresCategoriaRepo(db *sqlx.DB) *PostgresCategoriaRepo {
	return &PostgresCategoriaRepo{db: db}
}

func (r *PostgresCategoriaRepo) List(ctx context.Context) ([]Categoria, error) {
	var filas []filaCategoria
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnasCategoria+` FROM `+tablaCategoria+` ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}

	salida := make([]Categoria, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

func (r *PostgresCategoriaRepo) Get(ctx context.Context, id string) (*Categoria, error) {
	var f filaCategoria
	err := r.db.GetContext(ctx, &f,
		`SELECT `+columnasCategoria+` FROM `+tablaCategoria+` WHERE id = $1`, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := f.aDominio()
	return &c, nil
}

func (r *PostgresCategoriaRepo) Create(ctx context.Context, c Categoria) (*Categoria, error) {
	var f filaCategoria
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaCategoria+` (id, name, kind, color, "createdAt")
		VALUES ($1, $2, $3, $4, NOW()) RETURNING `+columnasCategoria,
		uuid.NewString(), c.Name, string(c.Kind), c.Color)
	if err != nil {
		return nil, traducirDuplicado(err)
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresCategoriaRepo) Update(ctx context.Context, c Categoria) (*Categoria, error) {
	var f filaCategoria
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tablaCategoria+` SET name = $2, kind = $3, color = $4
		WHERE id = $1 RETURNING `+columnasCategoria,
		c.ID, c.Name, string(c.Kind), c.Color)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, traducirDuplicado(err)
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresCategoriaRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaCategoria+` WHERE id = $1`, id)
	return err
}

// traducirDuplicado convierte el choque con el índice único en algo legible:
// hay una restricción de nombre + tipo en esa tabla.
func traducirDuplicado(err error) error {
	if err != nil && strings.Contains(err.Error(), "duplicate key") {
		return errors.New("ya existe una categoría con ese nombre y tipo")
	}
	return err
}

// ── Movimientos ────────────────────────────────────────────────────────────

const columnasMovimiento = `id, "accountId", "categoryId", kind,
	ROUND(amount * 100)::bigint AS amount_cents,
	"occurredAt", notes, "createdAt"`

type filaMovimiento struct {
	ID          string    `db:"id"`
	AccountID   string    `db:"accountId"`
	CategoryID  *string   `db:"categoryId"`
	Kind        string    `db:"kind"`
	AmountCents int64     `db:"amount_cents"`
	OccurredAt  time.Time `db:"occurredAt"`
	Notes       *string   `db:"notes"`
	CreatedAt   time.Time `db:"createdAt"`
}

func (f filaMovimiento) aDominio() Movimiento {
	return Movimiento{
		ID: f.ID, AccountID: f.AccountID, CategoryID: f.CategoryID,
		Kind: TipoMovimiento(f.Kind), Amount: Dinero(f.AmountCents),
		OccurredAt: f.OccurredAt, Notes: f.Notes, CreatedAt: f.CreatedAt,
	}
}

type PostgresMovimientoRepo struct {
	db *sqlx.DB
}

func NewPostgresMovimientoRepo(db *sqlx.DB) *PostgresMovimientoRepo {
	return &PostgresMovimientoRepo{db: db}
}

func (r *PostgresMovimientoRepo) List(ctx context.Context, f FiltroMovimientos) ([]Movimiento, error) {
	// Las condiciones se arman con marcadores numerados; nunca se concatena
	// nada que venga del usuario.
	condiciones := []string{}
	args := []any{}

	if f.AccountID != "" {
		args = append(args, f.AccountID)
		condiciones = append(condiciones, `"accountId" = $`+pos(len(args)))
	}
	if f.Desde != nil {
		args = append(args, *f.Desde)
		condiciones = append(condiciones, `"occurredAt" >= $`+pos(len(args)))
	}
	if f.Hasta != nil {
		args = append(args, *f.Hasta)
		condiciones = append(condiciones, `"occurredAt" < $`+pos(len(args)))
	}

	q := `SELECT ` + columnasMovimiento + ` FROM ` + tablaMovimiento
	if len(condiciones) > 0 {
		q += ` WHERE ` + strings.Join(condiciones, " AND ")
	}
	q += ` ORDER BY "occurredAt" DESC, "createdAt" DESC`

	// Tope siempre: sin él, una cuenta con años de historia devolvería
	// decenas de miles de filas a una pantalla que enseña veinte.
	limite := f.Limite
	if limite <= 0 || limite > 1000 {
		limite = 500
	}
	args = append(args, limite)
	q += ` LIMIT $` + pos(len(args))

	var filas []filaMovimiento
	if err := r.db.SelectContext(ctx, &filas, q, args...); err != nil {
		return nil, err
	}

	salida := make([]Movimiento, 0, len(filas))
	for _, fila := range filas {
		salida = append(salida, fila.aDominio())
	}
	return salida, nil
}

func (r *PostgresMovimientoRepo) Create(ctx context.Context, m Movimiento) (*Movimiento, error) {
	var f filaMovimiento
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaMovimiento+` (id, "accountId", "categoryId", kind, amount, "occurredAt", notes, "createdAt")
		VALUES ($1, $2, $3, $4, $5::numeric / 100, $6, $7, NOW())
		RETURNING `+columnasMovimiento,
		uuid.NewString(), m.AccountID, m.CategoryID, string(m.Kind),
		int64(m.Amount), m.OccurredAt, m.Notes)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresMovimientoRepo) Update(ctx context.Context, m Movimiento) (*Movimiento, error) {
	var f filaMovimiento
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tablaMovimiento+` SET "accountId" = $2, "categoryId" = $3, kind = $4,
			amount = $5::numeric / 100, "occurredAt" = $6, notes = $7
		WHERE id = $1 RETURNING `+columnasMovimiento,
		m.ID, m.AccountID, m.CategoryID, string(m.Kind),
		int64(m.Amount), m.OccurredAt, m.Notes)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresMovimientoRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaMovimiento+` WHERE id = $1`, id)
	return err
}

func (r *PostgresMovimientoRepo) GastadoEnCategoria(ctx context.Context, categoriaID string, desde, hasta time.Time) (Dinero, error) {
	var centimos int64
	err := r.db.GetContext(ctx, &centimos, `
		SELECT COALESCE(ROUND(SUM(amount) * 100), 0)::bigint
		FROM `+tablaMovimiento+`
		WHERE "categoryId" = $1 AND kind = 'expense'
		  AND "occurredAt" >= $2 AND "occurredAt" < $3`,
		categoriaID, desde, hasta)
	if err != nil {
		return 0, err
	}
	return Dinero(centimos), nil
}

// ── Presupuestos ───────────────────────────────────────────────────────────

const columnasPresupuesto = `id, "categoryId", ROUND(amount * 100)::bigint AS amount_cents, "createdAt"`

type filaPresupuesto struct {
	ID          string    `db:"id"`
	CategoryID  string    `db:"categoryId"`
	AmountCents int64     `db:"amount_cents"`
	CreatedAt   time.Time `db:"createdAt"`
}

func (f filaPresupuesto) aDominio() Presupuesto {
	return Presupuesto{
		ID: f.ID, CategoryID: f.CategoryID,
		Amount: Dinero(f.AmountCents), CreatedAt: f.CreatedAt,
	}
}

type PostgresPresupuestoRepo struct {
	db *sqlx.DB
}

func NewPostgresPresupuestoRepo(db *sqlx.DB) *PostgresPresupuestoRepo {
	return &PostgresPresupuestoRepo{db: db}
}

func (r *PostgresPresupuestoRepo) List(ctx context.Context) ([]Presupuesto, error) {
	var filas []filaPresupuesto
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnasPresupuesto+` FROM `+tablaPresupuesto)
	if err != nil {
		return nil, err
	}

	salida := make([]Presupuesto, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

// Upsert aprovecha que hay una restricción única por categoría: un
// presupuesto por categoría, y guardar otra vez actualiza el que había.
func (r *PostgresPresupuestoRepo) Upsert(ctx context.Context, p Presupuesto) (*Presupuesto, error) {
	var f filaPresupuesto
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaPresupuesto+` (id, "categoryId", amount, "createdAt", "updatedAt")
		VALUES ($1, $2, $3::numeric / 100, NOW(), NOW())
		ON CONFLICT ("categoryId") DO UPDATE SET
			amount = EXCLUDED.amount, "updatedAt" = NOW()
		RETURNING `+columnasPresupuesto,
		uuid.NewString(), p.CategoryID, int64(p.Amount))
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresPresupuestoRepo) Remove(ctx context.Context, categoriaID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM `+tablaPresupuesto+` WHERE "categoryId" = $1`, categoriaID)
	return err
}

// ── Pagos recurrentes ──────────────────────────────────────────────────────

const columnasRecurrente = `id, name, "accountId", "categoryId", kind,
	ROUND(amount * 100)::bigint AS amount_cents,
	frequency, "nextRunAt", "remindDaysBefore", "reminderSentAt",
	active, notes, "createdAt"`

type filaRecurrente struct {
	ID               string     `db:"id"`
	Name             string     `db:"name"`
	AccountID        string     `db:"accountId"`
	CategoryID       *string    `db:"categoryId"`
	Kind             string     `db:"kind"`
	AmountCents      int64      `db:"amount_cents"`
	Frequency        string     `db:"frequency"`
	NextRunAt        time.Time  `db:"nextRunAt"`
	RemindDaysBefore *int       `db:"remindDaysBefore"`
	ReminderSentAt   *time.Time `db:"reminderSentAt"`
	Active           bool       `db:"active"`
	Notes            *string    `db:"notes"`
	CreatedAt        time.Time  `db:"createdAt"`
}

func (f filaRecurrente) aDominio() Recurrente {
	return Recurrente{
		ID: f.ID, Name: f.Name, AccountID: f.AccountID, CategoryID: f.CategoryID,
		Kind: TipoMovimiento(f.Kind), Amount: Dinero(f.AmountCents),
		Frequency: Frecuencia(f.Frequency), NextRunAt: f.NextRunAt,
		RemindDaysBefore: f.RemindDaysBefore, ReminderSentAt: f.ReminderSentAt,
		Active: f.Active, Notes: f.Notes, CreatedAt: f.CreatedAt,
	}
}

type PostgresRecurrenteRepo struct {
	db *sqlx.DB
}

func NewPostgresRecurrenteRepo(db *sqlx.DB) *PostgresRecurrenteRepo {
	return &PostgresRecurrenteRepo{db: db}
}

func (r *PostgresRecurrenteRepo) List(ctx context.Context, soloActivos bool) ([]Recurrente, error) {
	q := `SELECT ` + columnasRecurrente + ` FROM ` + tablaRecurrente
	if soloActivos {
		q += ` WHERE active = true`
	}
	q += ` ORDER BY "nextRunAt" ASC`

	var filas []filaRecurrente
	if err := r.db.SelectContext(ctx, &filas, q); err != nil {
		return nil, err
	}

	salida := make([]Recurrente, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

func (r *PostgresRecurrenteRepo) Create(ctx context.Context, rec Recurrente) (*Recurrente, error) {
	var f filaRecurrente
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaRecurrente+`
			(id, name, "accountId", "categoryId", kind, amount, frequency,
			 "nextRunAt", "remindDaysBefore", active, notes, "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, $5, $6::numeric / 100, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING `+columnasRecurrente,
		uuid.NewString(), rec.Name, rec.AccountID, rec.CategoryID, string(rec.Kind),
		int64(rec.Amount), string(rec.Frequency), rec.NextRunAt,
		rec.RemindDaysBefore, rec.Active, rec.Notes)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

// Update reinicia el aviso si cambia cuándo o cuánto se avisa, por la misma
// razón que en el calendario: el recordatorio ya mandado no vale para la
// fecha nueva.
func (r *PostgresRecurrenteRepo) Update(ctx context.Context, rec Recurrente) (*Recurrente, error) {
	var f filaRecurrente
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tablaRecurrente+` SET
			name = $2, "accountId" = $3, "categoryId" = $4, kind = $5,
			amount = $6::numeric / 100, frequency = $7, "nextRunAt" = $8,
			"remindDaysBefore" = $9, active = $10, notes = $11,
			"reminderSentAt" = CASE
				WHEN "nextRunAt" <> $8 OR "remindDaysBefore" IS DISTINCT FROM $9
				THEN NULL ELSE "reminderSentAt"
			END,
			"updatedAt" = NOW()
		WHERE id = $1 RETURNING `+columnasRecurrente,
		rec.ID, rec.Name, rec.AccountID, rec.CategoryID, string(rec.Kind),
		int64(rec.Amount), string(rec.Frequency), rec.NextRunAt,
		rec.RemindDaysBefore, rec.Active, rec.Notes)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresRecurrenteRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaRecurrente+` WHERE id = $1`, id)
	return err
}

// MarcarProcesado mueve la fecha y limpia el aviso: empieza un ciclo nuevo y
// hay que poder volver a avisar.
func (r *PostgresRecurrenteRepo) MarcarProcesado(ctx context.Context, id string, siguiente time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE `+tablaRecurrente+`
		SET "nextRunAt" = $2, "reminderSentAt" = NULL, "updatedAt" = NOW()
		WHERE id = $1`, id, siguiente)
	return err
}

func (r *PostgresRecurrenteRepo) MarcarAvisado(ctx context.Context, id string, cuando time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tablaRecurrente+` SET "reminderSentAt" = $2, "updatedAt" = NOW() WHERE id = $1`,
		id, cuando)
	return err
}

// pos devuelve el marcador $n de una consulta.
func pos(n int) string {
	return strconv.Itoa(n)
}
