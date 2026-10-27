package automations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	tablaRegla = `lab_automation_rule`
	tablaRun   = `lab_automation_run`
)

const columnas = `id, name, trigger, active, actions, "createdAt", "updatedAt"`

type fila struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Trigger   string    `db:"trigger"`
	Active    bool      `db:"active"`
	Actions   []byte    `db:"actions"`
	CreatedAt time.Time `db:"createdAt"`
	UpdatedAt time.Time `db:"updatedAt"`
}

func (f fila) aDominio() Regla {
	var acciones []Accion
	// Una regla con el JSON corrompido se devuelve sin acciones en vez de
	// tumbar el listado entero: así se puede ver y arreglar desde la
	// interfaz.
	_ = json.Unmarshal(f.Actions, &acciones)

	return Regla{
		ID: f.ID, Name: f.Name, Trigger: Disparador(f.Trigger),
		Active: f.Active, Actions: acciones,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) List(ctx context.Context) ([]Regla, error) {
	return r.consultar(ctx,
		`SELECT `+columnas+` FROM `+tablaRegla+` ORDER BY "createdAt" DESC`)
}

func (r *PostgresRepo) ActivasPara(ctx context.Context, d Disparador) ([]Regla, error) {
	return r.consultar(ctx,
		`SELECT `+columnas+` FROM `+tablaRegla+` WHERE trigger = $1 AND active = true`, string(d))
}

func (r *PostgresRepo) consultar(ctx context.Context, q string, args ...any) ([]Regla, error) {
	var filas []fila
	if err := r.db.SelectContext(ctx, &filas, q, args...); err != nil {
		return nil, err
	}

	reglas := make([]Regla, 0, len(filas))
	for _, f := range filas {
		reglas = append(reglas, f.aDominio())
	}
	return reglas, nil
}

func (r *PostgresRepo) Create(ctx context.Context, regla Regla) (*Regla, error) {
	acciones, err := json.Marshal(regla.Actions)
	if err != nil {
		return nil, err
	}

	var f fila
	err = r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaRegla+` (id, name, trigger, active, actions, "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) RETURNING `+columnas,
		uuid.NewString(), regla.Name, string(regla.Trigger), regla.Active, acciones)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresRepo) Update(ctx context.Context, regla Regla) (*Regla, error) {
	acciones, err := json.Marshal(regla.Actions)
	if err != nil {
		return nil, err
	}

	var f fila
	err = r.db.GetContext(ctx, &f, `
		UPDATE `+tablaRegla+` SET name = $2, trigger = $3, active = $4, actions = $5,
			"updatedAt" = NOW()
		WHERE id = $1 RETURNING `+columnas,
		regla.ID, regla.Name, string(regla.Trigger), regla.Active, acciones)

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
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaRegla+` WHERE id = $1`, id)
	return err
}

func (r *PostgresRepo) Anotar(ctx context.Context, reglaID string, ok bool, fallo *string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tablaRun+` (id, "ruleId", ok, error, "ranAt") VALUES ($1, $2, $3, $4, NOW())`,
		uuid.NewString(), reglaID, ok, fallo)
	return err
}

type filaRun struct {
	OK    bool      `db:"ok"`
	Error *string   `db:"error"`
	RanAt time.Time `db:"ranAt"`
}

func (r *PostgresRepo) Ejecuciones(ctx context.Context, reglaID string, limite int) ([]Ejecucion, error) {
	var filas []filaRun
	err := r.db.SelectContext(ctx, &filas,
		`SELECT ok, error, "ranAt" FROM `+tablaRun+`
		 WHERE "ruleId" = $1 ORDER BY "ranAt" DESC LIMIT $2`, reglaID, limite)
	if err != nil {
		return nil, err
	}

	salida := make([]Ejecucion, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, Ejecucion{OK: f.OK, Error: f.Error, RanAt: f.RanAt})
	}
	return salida, nil
}
