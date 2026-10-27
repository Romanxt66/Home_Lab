package email

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	tablaPlantilla = `lab_email_template`
	tablaCuenta    = `lab_google_account`
	tablaRegistro  = `lab_email_log`
)

// ── Plantillas ─────────────────────────────────────────────────────────────

const columnasPlantilla = `id, name, subject, body, "createdAt", "updatedAt"`

type filaPlantilla struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Subject   string    `db:"subject"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"createdAt"`
	UpdatedAt time.Time `db:"updatedAt"`
}

func (f filaPlantilla) aDominio() Plantilla {
	return Plantilla{
		ID: f.ID, Name: f.Name, Subject: f.Subject, Body: f.Body,
		CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}

type PostgresPlantillaRepo struct {
	db *sqlx.DB
}

func NewPostgresPlantillaRepo(db *sqlx.DB) *PostgresPlantillaRepo {
	return &PostgresPlantillaRepo{db: db}
}

func (r *PostgresPlantillaRepo) List(ctx context.Context) ([]Plantilla, error) {
	var filas []filaPlantilla
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnasPlantilla+` FROM `+tablaPlantilla+` ORDER BY "createdAt" DESC`)
	if err != nil {
		return nil, err
	}

	salida := make([]Plantilla, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

func (r *PostgresPlantillaRepo) Get(ctx context.Context, id string) (*Plantilla, error) {
	var f filaPlantilla
	err := r.db.GetContext(ctx, &f,
		`SELECT `+columnasPlantilla+` FROM `+tablaPlantilla+` WHERE id = $1`, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p := f.aDominio()
	return &p, nil
}

func (r *PostgresPlantillaRepo) Create(ctx context.Context, p Plantilla) (*Plantilla, error) {
	var f filaPlantilla
	err := r.db.GetContext(ctx, &f,
		`INSERT INTO `+tablaPlantilla+` (id, name, subject, body, "createdAt", "updatedAt")
		 VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING `+columnasPlantilla,
		uuid.NewString(), p.Name, p.Subject, p.Body)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresPlantillaRepo) Update(ctx context.Context, p Plantilla) (*Plantilla, error) {
	var f filaPlantilla
	err := r.db.GetContext(ctx, &f,
		`UPDATE `+tablaPlantilla+` SET name = $2, subject = $3, body = $4, "updatedAt" = NOW()
		 WHERE id = $1 RETURNING `+columnasPlantilla,
		p.ID, p.Name, p.Subject, p.Body)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

func (r *PostgresPlantillaRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tablaPlantilla+` WHERE id = $1`, id)
	return err
}

// ── Buzones conectados ─────────────────────────────────────────────────────

const columnasCuenta = `id, email, name, picture, "refreshToken", "accessToken", "expiresAt", scope`

type filaCuenta struct {
	ID           string     `db:"id"`
	Email        string     `db:"email"`
	Name         *string    `db:"name"`
	Picture      *string    `db:"picture"`
	RefreshToken string     `db:"refreshToken"`
	AccessToken  *string    `db:"accessToken"`
	ExpiresAt    *time.Time `db:"expiresAt"`
	Scope        string     `db:"scope"`
}

func (f filaCuenta) aDominio() CuentaGoogle {
	return CuentaGoogle{
		ID: f.ID, Email: f.Email, Name: f.Name, Picture: f.Picture,
		RefreshToken: f.RefreshToken, AccessToken: f.AccessToken,
		ExpiresAt: f.ExpiresAt, Scope: f.Scope,
	}
}

type PostgresCuentaRepo struct {
	db *sqlx.DB
}

func NewPostgresCuentaRepo(db *sqlx.DB) *PostgresCuentaRepo {
	return &PostgresCuentaRepo{db: db}
}

func (r *PostgresCuentaRepo) List(ctx context.Context) ([]CuentaGoogle, error) {
	var filas []filaCuenta
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnasCuenta+` FROM `+tablaCuenta+` ORDER BY "createdAt" DESC`)
	if err != nil {
		return nil, err
	}

	salida := make([]CuentaGoogle, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida, nil
}

func (r *PostgresCuentaRepo) Get(ctx context.Context, id string) (*CuentaGoogle, error) {
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

// Upsert guarda el buzón por correo: reconectar la misma cuenta actualiza la
// fila en vez de crear una nueva.
func (r *PostgresCuentaRepo) Upsert(ctx context.Context, c CuentaGoogle) (*CuentaGoogle, error) {
	id := c.ID
	if id == "" {
		id = uuid.NewString()
	}

	var f filaCuenta
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tablaCuenta+`
			(id, email, name, picture, "refreshToken", "accessToken", "expiresAt", scope, "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (email) DO UPDATE SET
			name = EXCLUDED.name,
			picture = EXCLUDED.picture,
			"refreshToken" = EXCLUDED."refreshToken",
			"accessToken" = EXCLUDED."accessToken",
			"expiresAt" = EXCLUDED."expiresAt",
			scope = EXCLUDED.scope,
			"updatedAt" = NOW()
		RETURNING `+columnasCuenta,
		id, c.Email, c.Name, c.Picture, c.RefreshToken, c.AccessToken, c.ExpiresAt, c.Scope)
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

// ── Registro de envíos ─────────────────────────────────────────────────────

type PostgresRegistro struct {
	db *sqlx.DB
}

func NewPostgresRegistro(db *sqlx.DB) *PostgresRegistro {
	return &PostgresRegistro{db: db}
}

func (r *PostgresRegistro) Anotar(ctx context.Context, para, asunto, estado string, fallo *string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tablaRegistro+` (id, "to", subject, status, error, "sentAt")
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		uuid.NewString(), para, asunto, estado, fallo)
	return err
}
