package calendar

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const tabla = `calendario`

const columnas = `id, title, description, start, "end", "allDay", location, color,
	"remindMinutesBefore", "reminderSentAt", "createdAt"`

type fila struct {
	ID                  string     `db:"id"`
	Title               string     `db:"title"`
	Description         *string    `db:"description"`
	Start               time.Time  `db:"start"`
	End                 *time.Time `db:"end"`
	AllDay              bool       `db:"allDay"`
	Location            *string    `db:"location"`
	Color               *string    `db:"color"`
	RemindMinutesBefore *int       `db:"remindMinutesBefore"`
	ReminderSentAt      *time.Time `db:"reminderSentAt"`
	CreatedAt           time.Time  `db:"createdAt"`
}

func (f fila) aDominio() Evento {
	return Evento{
		ID: f.ID, Title: f.Title, Description: f.Description,
		Start: f.Start, End: f.End, AllDay: f.AllDay,
		Location: f.Location, Color: f.Color,
		RemindMinutesBefore: f.RemindMinutesBefore,
		ReminderSentAt:      f.ReminderSentAt,
		CreatedAt:           f.CreatedAt,
	}
}

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) ListBetween(ctx context.Context, desde, hasta time.Time) ([]Evento, error) {
	var filas []fila
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnas+` FROM `+tabla+`
		 WHERE start >= $1 AND start < $2 ORDER BY start ASC`, desde, hasta)
	if err != nil {
		return nil, err
	}
	return convertir(filas), nil
}

func (r *PostgresRepo) Get(ctx context.Context, id string) (*Evento, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `SELECT `+columnas+` FROM `+tabla+` WHERE id = $1`, id)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	e := f.aDominio()
	return &e, nil
}

func (r *PostgresRepo) Create(ctx context.Context, e Evento) (*Evento, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `
		INSERT INTO `+tabla+`
			(id, title, description, start, "end", "allDay", location, color,
			 "remindMinutesBefore", "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING `+columnas,
		uuid.NewString(), e.Title, e.Description, e.Start, e.End, e.AllDay,
		e.Location, e.Color, e.RemindMinutesBefore)
	if err != nil {
		return nil, err
	}
	d := f.aDominio()
	return &d, nil
}

// Update reinicia el aviso SOLO si cambió cuándo tocaba avisar.
//
// Si se mueve el evento o se cambian los minutos de antelación, el aviso ya
// mandado no vale para la hora nueva y hay que volver a avisar. Pero si solo
// se corrige el título, reiniciarlo mandaría el recordatorio dos veces. En
// la condición, los nombres de columna sin $ son los valores de ANTES de la
// actualización, que es justo lo que hay que comparar.
func (r *PostgresRepo) Update(ctx context.Context, e Evento) (*Evento, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `
		UPDATE `+tabla+` SET
			title = $2, description = $3, start = $4, "end" = $5, "allDay" = $6,
			location = $7, color = $8, "remindMinutesBefore" = $9,
			"reminderSentAt" = CASE
				WHEN start <> $4 OR "remindMinutesBefore" IS DISTINCT FROM $9
				THEN NULL ELSE "reminderSentAt"
			END,
			"updatedAt" = NOW()
		WHERE id = $1 RETURNING `+columnas,
		e.ID, e.Title, e.Description, e.Start, e.End, e.AllDay,
		e.Location, e.Color, e.RemindMinutesBefore)

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

// Candidatos trae los que podrían tocar. El corte exacto lo hace el dominio:
// aquí solo se descarta lo que no hace falta ni mirar.
func (r *PostgresRepo) Candidatos(ctx context.Context, desde time.Time) ([]Evento, error) {
	var filas []fila
	err := r.db.SelectContext(ctx, &filas,
		`SELECT `+columnas+` FROM `+tabla+`
		 WHERE "remindMinutesBefore" IS NOT NULL
		   AND "reminderSentAt" IS NULL
		   AND start >= $1
		 ORDER BY start ASC`, desde)
	if err != nil {
		return nil, err
	}
	return convertir(filas), nil
}

func (r *PostgresRepo) MarcarAvisado(ctx context.Context, id string, cuando time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE `+tabla+` SET "reminderSentAt" = $2, "updatedAt" = NOW() WHERE id = $1`,
		id, cuando)
	return err
}

func convertir(filas []fila) []Evento {
	salida := make([]Evento, 0, len(filas))
	for _, f := range filas {
		salida = append(salida, f.aDominio())
	}
	return salida
}
