package dbadmin

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const (
	tiempoLimite = 30 * time.Second
	// Tope de filas que vuelven de una consulta libre. Sin él, un SELECT *
	// sobre una tabla de millones de filas se comería la memoria del
	// proceso entero.
	maxFilas = 5000
)

// PostgresEjecutor abre conexiones a las bases externas.
//
// Las conexiones se guardan en un mapa para no abrir una nueva en cada
// consulta: abrir una conexión a PostgreSQL cuesta decenas de milisegundos,
// y hojear una tabla son muchas consultas seguidas.
type PostgresEjecutor struct {
	mu       sync.Mutex
	abiertas map[string]*sqlx.DB
}

func NewPostgresEjecutor() *PostgresEjecutor {
	return &PostgresEjecutor{abiertas: map[string]*sqlx.DB{}}
}

// Cerrar suelta todas las conexiones. Se llama al apagar el proceso.
func (e *PostgresEjecutor) Cerrar() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for url, db := range e.abiertas {
		_ = db.Close()
		delete(e.abiertas, url)
	}
}

func (e *PostgresEjecutor) conexion(url string) (*sqlx.DB, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if db, hay := e.abiertas[url]; hay {
		return db, nil
	}

	db, err := sqlx.Open("postgres", url)
	if err != nil {
		return nil, err
	}

	// Pocas conexiones por base: esto es un panel personal, no un servicio
	// con carga, y dejar abiertas muchas conexiones a una base ajena es de
	// mala educación.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)

	e.abiertas[url] = db
	return db, nil
}

func (e *PostgresEjecutor) Probar(ctx context.Context, url string) kernel.Result[string] {
	db, err := e.conexion(url)
	if err != nil {
		return kernel.Err[string](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	var version string
	if err := db.GetContext(ctx, &version, "SELECT version()"); err != nil {
		return kernel.Err[string](explicar(err))
	}

	return kernel.Ok(version)
}

func (e *PostgresEjecutor) Consultar(ctx context.Context, url, consulta string, soloLectura bool) kernel.Result[Resultado] {
	return e.ejecutar(ctx, url, consulta, nil, soloLectura)
}

func (e *PostgresEjecutor) ConsultarConParametros(ctx context.Context, url, consulta string, params []any, soloLectura bool) kernel.Result[Resultado] {
	return e.ejecutar(ctx, url, consulta, params, soloLectura)
}

// ejecutar corre la consulta y devuelve las filas.
//
// En modo solo lectura va dentro de una transacción READ ONLY: eso es una
// garantía del propio PostgreSQL, no un análisis de texto que se pueda
// burlar. Es la barrera que de verdad cuenta.
func (e *PostgresEjecutor) ejecutar(ctx context.Context, url, consulta string, params []any, soloLectura bool) kernel.Result[Resultado] {
	db, err := e.conexion(url)
	if err != nil {
		return kernel.Err[Resultado](explicar(err))
	}

	ctx, cancelar := context.WithTimeout(ctx, tiempoLimite)
	defer cancelar()

	empezo := time.Now()

	tx, err := db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: soloLectura})
	if err != nil {
		return kernel.Err[Resultado](explicar(err))
	}
	// En solo lectura siempre se deshace: no hay nada que confirmar y así
	// no queda ninguna transacción abierta por descuido.
	defer func() { _ = tx.Rollback() }()

	filas, err := tx.QueryxContext(ctx, consulta, params...)
	if err != nil {
		return kernel.Err[Resultado](explicar(err))
	}
	defer filas.Close()

	columnas, err := filas.Columns()
	if err != nil {
		return kernel.Err[Resultado](explicar(err))
	}

	datos := make([][]any, 0)
	for filas.Next() {
		if len(datos) >= maxFilas {
			break
		}

		fila, err := filas.SliceScan()
		if err != nil {
			return kernel.Err[Resultado](explicar(err))
		}

		datos = append(datos, normalizarFila(fila))
	}
	if err := filas.Err(); err != nil {
		return kernel.Err[Resultado](explicar(err))
	}

	if !soloLectura {
		if err := tx.Commit(); err != nil {
			return kernel.Err[Resultado](explicar(err))
		}
	}

	return kernel.Ok(Resultado{
		Columns:    columnas,
		Rows:       datos,
		RowCount:   int64(len(datos)),
		DurationMs: time.Since(empezo).Milliseconds(),
	})
}

// normalizarFila convierte los valores a algo que el JSON entienda.
//
// El driver devuelve []byte para texto y para tipos que no conoce; dejarlo
// así saldría al navegador como base64, que no hay quien lo lea.
func normalizarFila(fila []any) []any {
	salida := make([]any, len(fila))
	for i, v := range fila {
		if b, es := v.([]byte); es {
			salida[i] = string(b)
			continue
		}
		salida[i] = v
	}
	return salida
}

// explicar traduce el error de PostgreSQL a algo accionable.
func explicar(err error) string {
	if err == nil {
		return ""
	}

	texto := err.Error()

	switch {
	case errors.Is(err, context.DeadlineExceeded),
		strings.Contains(texto, "context deadline exceeded"):
		return "La consulta tardó más de 30 segundos y se canceló."

	case strings.Contains(texto, "connection refused"):
		return "No se pudo conectar: el servidor no responde en esa dirección."

	case strings.Contains(texto, "password authentication failed"):
		return "Usuario o contraseña incorrectos en la cadena de conexión."

	case strings.Contains(texto, "does not exist"):
		// Lo dice PostgreSQL y suele venir con el nombre exacto, que es
		// justo lo que hace falta saber.
		return "PostgreSQL: " + texto

	case strings.Contains(texto, "read-only transaction"):
		return "La conexión está en modo solo lectura y esa sentencia escribe."
	}

	return "PostgreSQL: " + texto
}
