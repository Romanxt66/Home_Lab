// Package database abre la conexión a PostgreSQL.
package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Connect abre el pool y comprueba que la base responde antes de devolverlo.
// Un fallo aquí es mejor que un fallo en la primera petición de un usuario.
func Connect(url string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir la conexión: %w", err)
	}

	// Límites modestos: este servicio comparte la base con el Lab en Next y
	// con otras aplicaciones, así que no conviene acaparar conexiones.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("la base de datos no responde: %w", err)
	}

	return db, nil
}
