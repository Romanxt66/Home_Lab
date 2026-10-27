package dbadmin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

type repoFalso struct {
	conexion *Conexion
	guardada *Conexion
	fallo    error
}

func (r *repoFalso) List(context.Context) ([]Conexion, error) { return nil, r.fallo }
func (r *repoFalso) Get(context.Context, string) (*Conexion, error) {
	return r.conexion, r.fallo
}
func (r *repoFalso) Create(_ context.Context, c Conexion) (*Conexion, error) {
	r.guardada = &c
	return &c, nil
}
func (r *repoFalso) Update(_ context.Context, c Conexion) (*Conexion, error) {
	r.guardada = &c
	return &c, nil
}
func (r *repoFalso) Remove(context.Context, string) error { return nil }

type ejecutorFalso struct {
	llamadas    []string
	soloLectura []bool
	filas       [][]any
}

func (e *ejecutorFalso) Probar(context.Context, string) kernel.Result[string] {
	return kernel.Ok("PostgreSQL 16")
}

func (e *ejecutorFalso) Consultar(_ context.Context, _, sql string, soloLectura bool) kernel.Result[Resultado] {
	e.llamadas = append(e.llamadas, sql)
	e.soloLectura = append(e.soloLectura, soloLectura)
	return kernel.Ok(Resultado{Rows: e.filas, RowCount: int64(len(e.filas))})
}

func (e *ejecutorFalso) ConsultarConParametros(_ context.Context, _, sql string, _ []any, soloLectura bool) kernel.Result[Resultado] {
	e.llamadas = append(e.llamadas, sql)
	e.soloLectura = append(e.soloLectura, soloLectura)
	return kernel.Ok(Resultado{Rows: e.filas, RowCount: int64(len(e.filas))})
}

type introspectorVacio struct{}

func (introspectorVacio) Esquemas(context.Context, string) kernel.Result[[]Esquema] {
	return kernel.Ok([]Esquema{})
}
func (introspectorVacio) Tablas(context.Context, string, string) kernel.Result[[]Tabla] {
	return kernel.Ok([]Tabla{})
}
func (introspectorVacio) Columnas(context.Context, string, string, string) kernel.Result[[]Columna] {
	return kernel.Ok([]Columna{})
}
func (introspectorVacio) Indices(context.Context, string, string, string) kernel.Result[[]Indice] {
	return kernel.Ok([]Indice{})
}
func (introspectorVacio) ClavesAjenas(context.Context, string, string) kernel.Result[[]ClaveAjena] {
	return kernel.Ok([]ClaveAjena{})
}

func servicio(c *Conexion) (*Service, *ejecutorFalso) {
	ej := &ejecutorFalso{}
	return NewService(&repoFalso{conexion: c}, ej, introspectorVacio{}), ej
}

func lectura() *Conexion {
	return &Conexion{ID: "c1", Name: "Prod", ConnectionURL: "postgres://x@h/db", ReadOnly: true}
}

func escritura() *Conexion {
	return &Conexion{ID: "c1", Name: "Dev", ConnectionURL: "postgres://x@h/db", ReadOnly: false}
}

// ── Conexión ───────────────────────────────────────────────────────────────

func TestValidarURL(t *testing.T) {
	for _, buena := range []string{
		"postgres://u:p@host:5432/db",
		"postgresql://u@host/db?sslmode=disable",
		`  "postgres://u:p@host/db"  `, // comillas que se cuelan al pegar de un .env
	} {
		if res := ValidarURL(buena); !res.OK {
			t.Errorf("rechazó %q: %s", buena, res.Error)
		}
	}

	for _, mala := range []string{"", "  ", "mysql://u@h/db", "host:5432", "postgres://"} {
		if ValidarURL(mala).OK {
			t.Errorf("aceptó %q", mala)
		}
	}
}

func TestHostNoDejaSalirLaContrasena(t *testing.T) {
	h := Host("postgres://admin:secreto@db.ejemplo.com:5432/master")
	if h != "db.ejemplo.com:5432" {
		t.Errorf("Host = %q", h)
	}
	if strings.Contains(h, "secreto") || strings.Contains(h, "admin") {
		t.Errorf("se coló una credencial: %q", h)
	}
	if Host("basura") != "?" {
		t.Error("una URL ilegible debería dar ?")
	}
}

// ── Modo solo lectura ──────────────────────────────────────────────────────

// Las tres escrituras de fila se cortan en una conexión de solo lectura.
func TestLasEscriturasDeFilaSeCortanEnSoloLectura(t *testing.T) {
	svc, ej := servicio(lectura())
	ctx := context.Background()

	pruebas := map[string]func() (kernel.Result[Resultado], error){
		"insertar": func() (kernel.Result[Resultado], error) {
			return svc.Insertar(ctx, "c1", "lab", "t", map[string]any{"a": 1})
		},
		"actualizar": func() (kernel.Result[Resultado], error) {
			return svc.Actualizar(ctx, "c1", "lab", "t", map[string]any{"a": 1}, map[string]any{"id": 1})
		},
		"borrar": func() (kernel.Result[Resultado], error) {
			return svc.BorrarFila(ctx, "c1", "lab", "t", map[string]any{"id": 1})
		},
	}

	for nombre, f := range pruebas {
		t.Run(nombre, func(t *testing.T) {
			res, err := f()
			if err != nil {
				t.Fatalf("debía ser error de negocio: %v", err)
			}
			if res.OK {
				t.Error("escribió en una conexión de solo lectura")
			}
			if !strings.Contains(res.Error, "solo lectura") {
				t.Errorf("no explica el motivo: %q", res.Error)
			}
		})
	}

	if len(ej.llamadas) != 0 {
		t.Errorf("llegó a ejecutar algo contra la base: %v", ej.llamadas)
	}
}

func TestEnSoloLecturaNoPasaNadaQueModifique(t *testing.T) {
	svc, ej := servicio(lectura())

	for _, sql := range []string{
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET a = 1 WHERE id = 1",
		"DELETE FROM t WHERE id = 1",
	} {
		res, _ := svc.Consultar(context.Background(), "c1", sql, true)
		if res.OK {
			t.Errorf("dejó pasar %q en solo lectura", sql)
		}
	}
	if len(ej.llamadas) != 0 {
		t.Errorf("ejecutó algo: %v", ej.llamadas)
	}
}

func TestEnSoloLecturaUnSelectPasaYSeEjecutaEnLecturaReal(t *testing.T) {
	svc, ej := servicio(lectura())

	res, err := svc.Consultar(context.Background(), "c1", "SELECT 1", false)
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}

	// Lo que de verdad cuenta: se le dice al ejecutor que abra una
	// transacción READ ONLY, que es una garantía del propio PostgreSQL y no
	// un análisis de texto que se pueda burlar.
	if len(ej.soloLectura) != 1 || !ej.soloLectura[0] {
		t.Errorf("no pidió transacción de solo lectura: %v", ej.soloLectura)
	}
}

// Hojear no puede modificar nada, diga lo que diga la configuración.
func TestHojearSiempreVaEnSoloLectura(t *testing.T) {
	svc, ej := servicio(escritura())

	res, err := svc.Hojear(context.Background(), "c1", OpcionesHojear{Esquema: "lab", Tabla: "t"})
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if len(ej.soloLectura) != 1 || !ej.soloLectura[0] {
		t.Errorf("hojear abrió una transacción de escritura: %v", ej.soloLectura)
	}
}

// ── Confirmación de lo destructivo ─────────────────────────────────────────

func TestLoDestructivoPideConfirmacion(t *testing.T) {
	svc, ej := servicio(escritura())

	for _, sql := range []string{"DROP TABLE t", "TRUNCATE t", "DELETE FROM t", "UPDATE t SET a = 1"} {
		res, err := svc.Consultar(context.Background(), "c1", sql, false)
		if err != nil {
			t.Fatalf("debía ser error de negocio: %v", err)
		}
		if res.OK {
			t.Errorf("ejecutó %q sin confirmar", sql)
		}
		if !strings.Contains(res.Error, "Confírmalo") {
			t.Errorf("no pide confirmación: %q", res.Error)
		}
	}

	if len(ej.llamadas) != 0 {
		t.Errorf("ejecutó algo sin confirmar: %v", ej.llamadas)
	}
}

func TestConfirmadoSeEjecuta(t *testing.T) {
	svc, ej := servicio(escritura())

	res, _ := svc.Consultar(context.Background(), "c1", "DROP TABLE t", true)
	if !res.OK {
		t.Fatalf("no ejecutó pese a estar confirmado: %s", res.Error)
	}
	if len(ej.llamadas) != 1 {
		t.Errorf("llamadas = %v", ej.llamadas)
	}
}

// Un DELETE sin WHERE debe decir que afecta a todas las filas, no solo
// "sentencia DELETE": es lo que hace pensárselo.
func TestElMensajeDiceCuandoAfectaATodasLasFilas(t *testing.T) {
	svc, _ := servicio(escritura())

	res, _ := svc.Consultar(context.Background(), "c1", "DELETE FROM t", false)
	if !strings.Contains(res.Error, "TODAS") {
		t.Errorf("no avisa de que toca todas las filas: %q", res.Error)
	}

	res, _ = svc.Consultar(context.Background(), "c1", "DROP TABLE t", false)
	if strings.Contains(res.Error, "TODAS") {
		t.Errorf("un DROP no es un borrado sin filtro: %q", res.Error)
	}
}

// En solo lectura, lo destructivo pide confirmación ANTES de decir que no se
// puede: quien lo escribió se entera de las dos cosas.
func TestEnSoloLecturaLoDestructivoPideConfirmacionPrimero(t *testing.T) {
	svc, _ := servicio(lectura())

	sinConfirmar, _ := svc.Consultar(context.Background(), "c1", "DROP TABLE t", false)
	if !strings.Contains(sinConfirmar.Error, "Confírmalo") {
		t.Errorf("sin confirmar debería pedir confirmación: %q", sinConfirmar.Error)
	}

	confirmado, _ := svc.Consultar(context.Background(), "c1", "DROP TABLE t", true)
	if !strings.Contains(confirmado.Error, "solo lectura") {
		t.Errorf("confirmado debería decir que es de solo lectura: %q", confirmado.Error)
	}
}

// ── Validaciones que cortan antes de llegar a la base ──────────────────────

func TestUnaConsultaVaciaNoLlegaAEjecutarse(t *testing.T) {
	svc, ej := servicio(escritura())

	for _, sql := range []string{"", "   ", "\n\t"} {
		if res, _ := svc.Consultar(context.Background(), "c1", sql, false); res.OK {
			t.Errorf("aceptó una consulta vacía: %q", sql)
		}
	}
	if len(ej.llamadas) != 0 {
		t.Errorf("ejecutó algo: %v", ej.llamadas)
	}
}

func TestUnNombreInvalidoNoLlegaALaBase(t *testing.T) {
	svc, ej := servicio(escritura())
	ctx := context.Background()

	casos := map[string]func() (kernel.Result[Resultado], error){
		"tabla vacía": func() (kernel.Result[Resultado], error) {
			return svc.Insertar(ctx, "c1", "lab", "  ", map[string]any{"a": 1})
		},
		"esquema vacío": func() (kernel.Result[Resultado], error) {
			return svc.Insertar(ctx, "c1", "", "t", map[string]any{"a": 1})
		},
		"byte nulo": func() (kernel.Result[Resultado], error) {
			return svc.Insertar(ctx, "c1", "lab", "t\x00x", map[string]any{"a": 1})
		},
		"sin clave": func() (kernel.Result[Resultado], error) { return svc.BorrarFila(ctx, "c1", "lab", "t", nil) },
	}

	for nombre, f := range casos {
		t.Run(nombre, func(t *testing.T) {
			res, _ := f()
			if res.OK {
				t.Error("lo dio por bueno")
			}
		})
	}
	if len(ej.llamadas) != 0 {
		t.Errorf("llegó a la base: %v", ej.llamadas)
	}
}

func TestUnaConexionInexistenteSeDice(t *testing.T) {
	svc, _ := servicio(nil)

	res, err := svc.Consultar(context.Background(), "fantasma", "SELECT 1", false)
	if err != nil {
		t.Fatalf("debía ser error de negocio: %v", err)
	}
	if res.OK {
		t.Error("ejecutó contra una conexión inexistente")
	}
}

// ── Paginación ─────────────────────────────────────────────────────────────

// Se pide una fila de más: si llega, hay página siguiente, sin COUNT(*).
func TestHojearDetectaSiHayMasPaginas(t *testing.T) {
	svc, ej := servicio(escritura())
	ej.filas = make([][]any, 11) // límite 10 + 1

	res, err := svc.Hojear(context.Background(), "c1", OpcionesHojear{Esquema: "lab", Tabla: "t", Limite: 10})
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}

	if !res.Value.HasMore {
		t.Error("no detectó que hay otra página")
	}
	if len(res.Value.Rows) != 10 {
		t.Errorf("devolvió %d filas, el límite son 10 (la de más no debe salir)", len(res.Value.Rows))
	}
}

func TestHojearEnLaUltimaPagina(t *testing.T) {
	svc, ej := servicio(escritura())
	ej.filas = make([][]any, 4)

	res, _ := svc.Hojear(context.Background(), "c1", OpcionesHojear{Esquema: "lab", Tabla: "t", Limite: 10})
	if res.Value.HasMore {
		t.Error("dijo que había más páginas en la última")
	}
	if len(res.Value.Rows) != 4 {
		t.Errorf("filas = %d", len(res.Value.Rows))
	}
}

func TestFalloDeBaseSubeComoError(t *testing.T) {
	svc := NewService(&repoFalso{fallo: errors.New("sin conexión")}, &ejecutorFalso{}, introspectorVacio{})

	if _, err := svc.Consultar(context.Background(), "c1", "SELECT 1", false); err == nil {
		t.Error("se tragó un fallo de base")
	}
}

func TestGuardarValida(t *testing.T) {
	svc := NewService(&repoFalso{}, &ejecutorFalso{}, introspectorVacio{})
	ctx := context.Background()

	if res, _ := svc.Guardar(ctx, Conexion{Name: "  ", ConnectionURL: "postgres://x@h/db"}); res.OK {
		t.Error("aceptó una conexión sin nombre")
	}
	if res, _ := svc.Guardar(ctx, Conexion{Name: "A", ConnectionURL: "mysql://x@h/db"}); res.OK {
		t.Error("aceptó una URL que no es de PostgreSQL")
	}
	if res, _ := svc.Guardar(ctx, Conexion{Name: "A", ConnectionURL: "postgres://x@h/db"}); !res.OK {
		t.Errorf("rechazó una conexión válida: %s", res.Error)
	}
}
