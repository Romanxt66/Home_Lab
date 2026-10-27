package notifications

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const tabla = `lab_notification_config`

type fila struct {
	ID         string    `db:"id"`
	Provider   string    `db:"provider"`
	Recipient  string    `db:"recipient"`
	Credential string    `db:"credential"`
	Active     bool      `db:"active"`
	CreatedAt  time.Time `db:"createdAt"`
	UpdatedAt  time.Time `db:"updatedAt"`
}

func (f fila) aDominio() Config {
	return Config{
		ID:         f.ID,
		Provider:   Provider(f.Provider),
		Recipient:  f.Recipient,
		Credential: f.Credential,
		Active:     f.Active,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}
}

const columnas = `id, provider, recipient, credential, active, "createdAt", "updatedAt"`

type PostgresRepo struct {
	db *sqlx.DB
}

func NewPostgresRepo(db *sqlx.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

// GetActive devuelve la configuración vigente. Si hubiera varias activas por
// un error pasado, gana la más reciente.
func (r *PostgresRepo) GetActive(ctx context.Context) (*Config, error) {
	var f fila
	err := r.db.GetContext(ctx, &f, `SELECT `+columnas+` FROM `+tabla+`
		WHERE active = true ORDER BY "updatedAt" DESC LIMIT 1`)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	c := f.aDominio()
	return &c, nil
}

// Save inserta o actualiza según venga o no un id.
func (r *PostgresRepo) Save(ctx context.Context, c Config) (*Config, error) {
	ahora := time.Now().UTC()
	var f fila

	if c.ID == "" {
		err := r.db.GetContext(ctx, &f, `
			INSERT INTO `+tabla+` (id, provider, recipient, credential, active, "createdAt", "updatedAt")
			VALUES ($1,$2,$3,$4,$5,$6,$6) RETURNING `+columnas,
			uuid.NewString(), string(c.Provider), c.Recipient, c.Credential, c.Active, ahora)
		if err != nil {
			return nil, err
		}
	} else {
		err := r.db.GetContext(ctx, &f, `
			UPDATE `+tabla+` SET provider = $2, recipient = $3, credential = $4,
				active = $5, "updatedAt" = $6
			WHERE id = $1 RETURNING `+columnas,
			c.ID, string(c.Provider), c.Recipient, c.Credential, c.Active, ahora)
		if err != nil {
			return nil, err
		}
	}

	guardado := f.aDominio()
	return &guardado, nil
}

func (r *PostgresRepo) Remove(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tabla+` WHERE id = $1`, id)
	return err
}
