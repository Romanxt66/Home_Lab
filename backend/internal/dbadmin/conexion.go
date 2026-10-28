package dbadmin

import (
	"net/url"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Conexion es una base de datos externa que se puede mirar y editar.
//
// La URL se guarda en claro, como en el Lab: es un panel personal al que
// solo entra su dueño. Nunca sale al navegador.
type Conexion struct {
	ID            string
	Name          string
	ConnectionURL string
	ReadOnly      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Host saca el servidor de la URL, para poder distinguir conexiones en la
// lista sin enseñar la credencial.
func Host(crudo string) string {
	u, err := url.Parse(limpiarComillas(crudo))
	if err != nil || u.Host == "" {
		return "?"
	}
	return u.Host
}

// ValidarURL comprueba que parezca una cadena de conexión de PostgreSQL.
func ValidarURL(crudo string) kernel.Result[string] {
	limpio := limpiarComillas(crudo)
	if limpio == "" {
		return kernel.Err[string]("Escribe una URL de conexión.")
	}

	if !strings.HasPrefix(limpio, "postgres://") && !strings.HasPrefix(limpio, "postgresql://") {
		return kernel.Err[string](
			"Tiene que empezar por postgres:// o postgresql:// (el formato de libpq).")
	}

	u, err := url.Parse(limpio)
	if err != nil || u.Host == "" {
		return kernel.Err[string]("La URL no tiene un formato válido.")
	}

	return kernel.Ok(limpio)
}

// limpiarComillas quita las que se cuelan al copiar y pegar de un .env.
func limpiarComillas(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}

// ── Estructura de la base ──────────────────────────────────────────────────

type Esquema struct {
	Name     string
	IsSystem bool
}

type Tabla struct {
	Schema     string
	Name       string
	Kind       string
	RowsApprox int64
}

type Referencia struct {
	Schema string
	Table  string
	Column string
}

type Columna struct {
	Name            string
	DataType        string
	IsNullable      bool
	IsPrimaryKey    bool
	Default         *string
	OrdinalPosition int
	References      *Referencia
}

type ClaveAjena struct {
	ConstraintName string
	Schema         string
	Table          string
	Columns        []string
	RefSchema      string
	RefTable       string
	RefColumns     []string
}

type Indice struct {
	Name       string
	IsUnique   bool
	IsPrimary  bool
	Definition string
}

// DetalleTabla es lo que enseña la pestaña de estructura.
type DetalleTabla struct {
	Columns      []Columna
	Indexes      []Indice
	ForeignKeys  []ClaveAjena // de esta tabla hacia otras
	ReferencedBy []ClaveAjena // de otras hacia esta
}

// TablaDelDiagrama es una tabla con lo justo para dibujarla.
type TablaDelDiagrama struct {
	Schema  string
	Name    string
	Columns []ColumnaDelDiagrama
}

type ColumnaDelDiagrama struct {
	Name         string
	DataType     string
	IsPrimaryKey bool
	IsForeignKey bool
}

type Diagrama struct {
	Tables    []TablaDelDiagrama
	Relations []ClaveAjena
}

// Resultado es lo que devuelve una consulta.
type Resultado struct {
	Columns  []string
	Rows     [][]any
	RowCount int64
	// Duración en milisegundos, para que se vea qué tarda.
	DurationMs int64
}

// Pagina es un trozo de una tabla con la pista de si hay más.
type Pagina struct {
	Resultado
	HasMore bool
	Limit   int
	Offset  int
}

// ParaLibPq adapta una cadena de conexión pegada de otras herramientas a lo
// que acepta lib/pq. Dos diferencias muerden en la práctica:
//
//   - Prisma escribe `?schema=lab`. PostgreSQL no tiene ese parámetro y lo
//     rechaza («unrecognized configuration parameter»); el equivalente es
//     search_path.
//   - lib/pq exige TLS si no se dice otra cosa, mientras que libpq lo prueba y
//     cae a texto plano. Las bases internas (un nombre de contenedor, sin
//     puntos, o localhost) no suelen tener TLS, así que ahí se desactiva.
func ParaLibPq(crudo string) string {
	u, err := url.Parse(limpiarComillas(crudo))
	if err != nil || u.Host == "" {
		return crudo
	}

	q := u.Query()
	if esquema := q.Get("schema"); esquema != "" {
		q.Del("schema")
		if q.Get("search_path") == "" {
			q.Set("search_path", esquema)
		}
	}
	if q.Get("sslmode") == "" {
		if h := u.Hostname(); h == "localhost" || !strings.Contains(h, ".") {
			q.Set("sslmode", "disable")
		}
	}

	u.RawQuery = q.Encode()
	return u.String()
}
