package dbadmin

import (
	"strings"
	"testing"
)

// ── Citado de identificadores ──────────────────────────────────────────────

// La razón de ser de esta función: los identificadores no se pueden
// parametrizar, hay que interpolarlos, y hacerlo mal es inyección de SQL.
func TestCitarNeutralizaLaInyeccion(t *testing.T) {
	casos := map[string]string{
		"usuarios":                   `"usuarios"`,
		"con espacio":                `"con espacio"`,
		"MAYÚSCULAS":                 `"MAYÚSCULAS"`,
		`con"comilla`:                `"con""comilla"`,
		`"; DROP TABLE x; --`:        `"""; DROP TABLE x; --"`,
		`a"; DELETE FROM b WHERE "1`: `"a""; DELETE FROM b WHERE ""1"`,
	}

	for dado, quiero := range casos {
		if got := Citar(dado); got != quiero {
			t.Errorf("Citar(%q) = %s, quería %s", dado, got, quiero)
		}
	}
}

// El intento clásico: cerrar las comillas y colar otra sentencia. Tras citar,
// el texto completo queda dentro de un solo identificador.
func TestUnIntentoDeInyeccionQuedaDentroDeLasComillas(t *testing.T) {
	malicioso := `x" ; DROP TABLE usuarios; SELECT "1`
	citado := Citar(malicioso)

	if !strings.HasPrefix(citado, `"`) || !strings.HasSuffix(citado, `"`) {
		t.Fatalf("no quedó entre comillas: %s", citado)
	}

	// Dentro no puede quedar ninguna comilla sin duplicar: eso es lo que
	// cerraría el identificador antes de tiempo.
	dentro := citado[1 : len(citado)-1]
	for i := 0; i < len(dentro); i++ {
		if dentro[i] == '"' {
			if i+1 >= len(dentro) || dentro[i+1] != '"' {
				t.Fatalf("hay una comilla suelta en la posición %d: %s", i, citado)
			}
			i++ // saltar la pareja
		}
	}
}

func TestCitarCualificado(t *testing.T) {
	if got := CitarCualificado("lab", "usuarios"); got != `"lab"."usuarios"` {
		t.Errorf("CitarCualificado = %s", got)
	}
}

func TestValidarIdentificador(t *testing.T) {
	if res := ValidarIdentificador("  usuarios  ", "tabla"); !res.OK || res.Value != "usuarios" {
		t.Errorf("no recortó: %+v", res)
	}

	casos := map[string]string{
		"vacío":           "",
		"solo espacios":   "   ",
		"con byte nulo":   "usua\x00rios",
		"demasiado largo": strings.Repeat("a", 64),
	}
	for nombre, dado := range casos {
		if ValidarIdentificador(dado, "tabla").OK {
			t.Errorf("%s: lo dio por bueno", nombre)
		}
	}

	// Justo 63 bytes sí vale.
	if !ValidarIdentificador(strings.Repeat("a", 63), "tabla").OK {
		t.Error("rechazó un nombre de exactamente 63 bytes")
	}
}

// ── Análisis de SQL ────────────────────────────────────────────────────────

func TestAnalizar(t *testing.T) {
	casos := []struct {
		sql       string
		riesgo    Riesgo
		sinFiltro bool
	}{
		{"SELECT * FROM usuarios", Inofensiva, false},
		{"  select 1  ", Inofensiva, false},
		{"WITH x AS (SELECT 1) SELECT * FROM x", Inofensiva, false},

		{"INSERT INTO t VALUES (1)", Modifica, false},
		{"UPDATE t SET a = 1 WHERE id = 2", Modifica, false},
		{"DELETE FROM t WHERE id = 2", Modifica, false},

		// Sin WHERE tocan toda la tabla: eso es destructivo.
		{"UPDATE t SET a = 1", Destructiva, true},
		{"DELETE FROM t", Destructiva, true},
		{"delete from t", Destructiva, true},

		{"DROP TABLE t", Destructiva, false},
		{"TRUNCATE t", Destructiva, false},
		{"ALTER TABLE t ADD COLUMN c int", Destructiva, false},
		{"CREATE TABLE t (id int)", Destructiva, false},
		{"GRANT ALL ON t TO alguien", Destructiva, false},
	}

	for _, c := range casos {
		t.Run(c.sql, func(t *testing.T) {
			a := Analizar(c.sql)
			if a.Riesgo != c.riesgo {
				t.Errorf("riesgo = %q, quería %q", a.Riesgo, c.riesgo)
			}
			if a.SinFiltro != c.sinFiltro {
				t.Errorf("sinFiltro = %v, quería %v", a.SinFiltro, c.sinFiltro)
			}
		})
	}
}

// Esconder un DROP tras un comentario no debe colarlo como inofensivo.
func TestLosComentariosNoEsconden(t *testing.T) {
	casos := []string{
		"-- solo miro\nDROP TABLE usuarios",
		"/* nada que ver */ DROP TABLE usuarios",
		"\n\n  /* varias\nlíneas */\n DROP TABLE usuarios",
	}

	for _, sql := range casos {
		if a := Analizar(sql); a.Riesgo != Destructiva {
			t.Errorf("se coló como %q: %q", a.Riesgo, sql)
		}
	}
}

// Un comentario delante no debe hacer que se pierda la primera sentencia.
func TestLaPrimeraSentenciaEsLaQueCuenta(t *testing.T) {
	a := Analizar("SELECT 1; DROP TABLE usuarios")
	if a.Riesgo != Inofensiva {
		t.Errorf("miró más allá de la primera sentencia: %+v", a)
	}

	b := Analizar("; ; DROP TABLE usuarios")
	if b.Riesgo != Destructiva {
		t.Errorf("los puntos y coma sueltos despistaron al análisis: %+v", b)
	}
}

func TestQuitarComentarios(t *testing.T) {
	got := QuitarComentarios("SELECT 1 -- nota\nFROM t /* otra */ WHERE x = 1")
	if strings.Contains(got, "nota") || strings.Contains(got, "otra") {
		t.Errorf("quedaron comentarios: %q", got)
	}
	if !strings.Contains(got, "SELECT 1") || !strings.Contains(got, "WHERE x = 1") {
		t.Errorf("se llevó por delante código: %q", got)
	}
}

// ── Construcción de sentencias ─────────────────────────────────────────────

func TestConstruirHojear(t *testing.T) {
	s := ConstruirHojear(OpcionesHojear{Esquema: "lab", Tabla: "usuarios", Limite: 10, Desde: 20})

	if !strings.Contains(s.Texto, `"lab"."usuarios"`) {
		t.Errorf("no citó la tabla: %s", s.Texto)
	}
	// Una fila de más, para saber si hay página siguiente sin un COUNT(*).
	if s.Parametros[0] != 11 {
		t.Errorf("pidió %v filas, quería 11 (10 + 1)", s.Parametros[0])
	}
	if s.Parametros[1] != 20 {
		t.Errorf("desplazamiento = %v", s.Parametros[1])
	}
	if strings.Contains(s.Texto, "ORDER BY") {
		t.Error("ordenó sin que se pidiera")
	}
}

func TestConstruirHojearConOrden(t *testing.T) {
	s := ConstruirHojear(OpcionesHojear{
		Esquema: "lab", Tabla: "t", OrdenarPor: `raro"col`, Direccion: Descendente,
	})

	if !strings.Contains(s.Texto, `ORDER BY "raro""col" DESC`) {
		t.Errorf("no citó la columna de orden: %s", s.Texto)
	}
}

func TestAcotarLimite(t *testing.T) {
	casos := map[int]int{0: PaginaPorDefecto, -5: PaginaPorDefecto, 10: 10, 999999: MaxPagina}
	for dado, quiero := range casos {
		if got := AcotarLimite(dado); got != quiero {
			t.Errorf("AcotarLimite(%d) = %d, quería %d", dado, got, quiero)
		}
	}
}

// El orden de las columnas tiene que ser estable: un mapa de Go no lo
// garantiza, y sin esto la sentencia cambiaría en cada llamada.
func TestElOrdenDeLasColumnasEsEstable(t *testing.T) {
	valores := map[string]any{"zeta": 1, "alfa": 2, "media": 3}

	primera := ConstruirInsertar("lab", "t", valores)
	if !primera.OK {
		t.Fatalf("falló: %s", primera.Error)
	}

	for i := 0; i < 20; i++ {
		otra := ConstruirInsertar("lab", "t", valores)
		if otra.Value.Texto != primera.Value.Texto {
			t.Fatalf("la sentencia cambió en la pasada %d:\n%s\n%s",
				i, primera.Value.Texto, otra.Value.Texto)
		}
	}

	if !strings.Contains(primera.Value.Texto, `("alfa", "media", "zeta")`) {
		t.Errorf("no salió en orden alfabético: %s", primera.Value.Texto)
	}
}

func TestConstruirInsertar(t *testing.T) {
	res := ConstruirInsertar("lab", "usuarios", map[string]any{"nombre": "Ana", "edad": 30})
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}

	if !strings.Contains(res.Value.Texto, "RETURNING *") {
		t.Error("no devuelve la fila insertada")
	}
	if len(res.Value.Parametros) != 2 {
		t.Errorf("parámetros = %v", res.Value.Parametros)
	}
	// Los valores van parametrizados, nunca en el texto.
	if strings.Contains(res.Value.Texto, "Ana") {
		t.Errorf("metió un valor en el texto de la consulta: %s", res.Value.Texto)
	}

	if ConstruirInsertar("lab", "t", nil).OK {
		t.Error("aceptó un insert sin valores")
	}
}

// Lo importante: sin clave, un UPDATE tocaría toda la tabla y un DELETE la
// vaciaría. Se rechaza antes de construir nada.
func TestSinClaveNoSeConstruyeNadaQueToqueTodaLaTabla(t *testing.T) {
	valores := map[string]any{"a": 1}

	if ConstruirActualizar("lab", "t", valores, nil).OK {
		t.Error("construyó un UPDATE sin clave")
	}
	if ConstruirActualizar("lab", "t", valores, map[string]any{}).OK {
		t.Error("construyó un UPDATE con clave vacía")
	}
	if ConstruirBorrar("lab", "t", nil).OK {
		t.Error("construyó un DELETE sin clave")
	}
	if ConstruirBorrar("lab", "t", map[string]any{}).OK {
		t.Error("construyó un DELETE con clave vacía")
	}
}

func TestConstruirActualizar(t *testing.T) {
	res := ConstruirActualizar("lab", "usuarios",
		map[string]any{"nombre": "Ana"},
		map[string]any{"id": 7})
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}

	if !strings.Contains(res.Value.Texto, `SET "nombre" = $1`) {
		t.Errorf("asignación mal: %s", res.Value.Texto)
	}
	if !strings.Contains(res.Value.Texto, `WHERE "id" = $2`) {
		t.Errorf("condición mal: %s", res.Value.Texto)
	}
	// Los marcadores van en orden: primero los valores, luego la clave.
	if len(res.Value.Parametros) != 2 || res.Value.Parametros[0] != "Ana" || res.Value.Parametros[1] != 7 {
		t.Errorf("parámetros = %v", res.Value.Parametros)
	}
}

func TestConstruirActualizarConClaveCompuesta(t *testing.T) {
	res := ConstruirActualizar("lab", "t",
		map[string]any{"v": 1},
		map[string]any{"a": "x", "b": "y"})
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}

	if !strings.Contains(res.Value.Texto, `"a" = $2 AND "b" = $3`) {
		t.Errorf("clave compuesta mal: %s", res.Value.Texto)
	}
}

// Un nombre de columna malicioso tampoco debe colarse por aquí.
func TestUnaColumnaMaliciosaSeCita(t *testing.T) {
	res := ConstruirActualizar("lab", "t",
		map[string]any{`a" = 1; DROP TABLE x; --`: 1},
		map[string]any{"id": 1})
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}

	// Todo el nombre, punto y coma incluido, queda DENTRO de un solo
	// identificador: la comilla interna va duplicada, así que no cierra.
	if !strings.Contains(res.Value.Texto, `"a"" = 1; DROP TABLE x; --" = $1`) {
		t.Errorf("no citó la columna: %s", res.Value.Texto)
	}

	// Y el valor sigue yendo parametrizado, no en el texto.
	if len(res.Value.Parametros) != 2 {
		t.Errorf("parámetros = %v", res.Value.Parametros)
	}
}

// Una columna con byte nulo se rechaza en vez de citarse: truncaría la
// sentencia en el servidor.
func TestUnaColumnaConByteNuloSeRechaza(t *testing.T) {
	res := ConstruirInsertar("lab", "t", map[string]any{"a\x00b": 1})
	if res.OK {
		t.Error("aceptó una columna con byte nulo")
	}
}
