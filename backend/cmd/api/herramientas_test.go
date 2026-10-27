package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/assistant"
)

// Los servicios van a nil a propósito: estas pruebas solo ejercitan las
// barreras que cortan ANTES de tocar ningún servicio. Si alguna herramienta
// llegara a usarlo, el pánico lo delataría.

func herramienta(t *testing.T, nombre string) assistant.Herramienta {
	t.Helper()
	for _, h := range construirHerramientas(Servicios{Destinos: destinosDelAsistente}) {
		if h.Nombre == nombre {
			return h
		}
	}
	t.Fatalf("no existe la herramienta %q", nombre)
	return assistant.Herramienta{}
}

func contexto(rol string) assistant.Contexto {
	return assistant.Contexto{UID: "u1", Role: rol, Efectos: &assistant.Efectos{}}
}

// El rol que ve la herramienta sale de la sesión firmada: aquí se comprueba
// que, sin ser superadmin, ninguna herramienta de usuarios llega a hacer nada.
func TestLasHerramientasDeUsuariosExigenSuperadmin(t *testing.T) {
	for _, nombre := range []string{"list_users", "approve_pending_user", "reject_pending_user"} {
		t.Run(nombre, func(t *testing.T) {
			h := herramienta(t, nombre)
			salida := h.Ejecutar(context.Background(), map[string]any{"id": "x"}, contexto("user"))

			if !strings.Contains(salida, "No autorizado") {
				t.Errorf("dejó pasar a un usuario normal: %q", salida)
			}
		})
	}
}

// Más estricto que el Lab: allí solo se rechazaba lo destructivo, así que un
// INSERT o un UPDATE con WHERE pasaban en una conexión con escritura.
func TestLaConsultaDelAsistenteSoloLee(t *testing.T) {
	h := herramienta(t, "run_database_query")

	for _, sql := range []string{
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET a = 1 WHERE id = 1",
		"DELETE FROM t WHERE id = 1",
		"DROP TABLE t",
		"TRUNCATE t",
		"-- inocente\nDROP TABLE t",
	} {
		salida := h.Ejecutar(context.Background(),
			map[string]any{"connectionId": "c1", "sql": sql}, contexto("superadmin"))

		if !strings.Contains(salida, "solo ejecuta consultas de lectura") {
			t.Errorf("dejó pasar %q: %s", sql, salida)
		}
	}
}

func TestNavegarSoloAAlgoQueExiste(t *testing.T) {
	h := herramienta(t, "navigate_to_module")

	c := contexto("user")
	if salida := h.Ejecutar(context.Background(), map[string]any{"slug": "calendar"}, c); strings.Contains(salida, "Error") {
		t.Errorf("rechazó un destino válido: %s", salida)
	}
	if c.Efectos.Destino() != "calendar" {
		t.Errorf("no dejó el destino: %q", c.Efectos.Destino())
	}

	c2 := contexto("user")
	salida := h.Ejecutar(context.Background(), map[string]any{"slug": "/etc/passwd"}, c2)
	if !strings.Contains(salida, "Error") {
		t.Errorf("aceptó un destino inventado: %s", salida)
	}
	if c2.Efectos.Destino() != "" {
		t.Errorf("dejó un destino inválido: %q", c2.Efectos.Destino())
	}
}

func TestLaFechaInvalidaSeExplicaAlModelo(t *testing.T) {
	h := herramienta(t, "create_calendar_event")
	salida := h.Ejecutar(context.Background(),
		map[string]any{"title": "x", "start": "mañana por la tarde"}, contexto("user"))

	if !strings.Contains(salida, "Error") || !strings.Contains(salida, "ISO 8601") {
		t.Errorf("no explica el formato esperado: %s", salida)
	}
}

// Los modelos a veces mandan "5" donde se pedía 5.
func TestLeerLaEntradaSinTipo(t *testing.T) {
	in := map[string]any{"a": 5.0, "b": "7", "c": "x", "d": true, "e": "true", "f": nil, "g": ""}

	if numero(in, "a") != 5 || numero(in, "b") != 7 || numero(in, "c") != 0 {
		t.Error("numero no acepta número, texto numérico y basura")
	}
	if !booleano(in, "d") || !booleano(in, "e") || booleano(in, "c") {
		t.Error("booleano no acepta booleano y texto")
	}
	if hay(in, "f") || hay(in, "g") || hay(in, "no-existe") || !hay(in, "a") {
		t.Error("hay confunde ausente con presente")
	}
}

// Cada herramienta tiene que tener un esquema JSON válido, o Gemini
// rechazaría TODA la petición por culpa de una sola.
func TestTodosLosEsquemasSonJSONValido(t *testing.T) {
	for _, h := range construirHerramientas(Servicios{Destinos: destinosDelAsistente}) {
		if !jsonValido(h.Esquema) {
			t.Errorf("%s: esquema inválido: %s", h.Nombre, h.Esquema)
		}
		if strings.TrimSpace(h.Descripcion) == "" {
			t.Errorf("%s: sin descripción, el modelo no sabrá cuándo usarla", h.Nombre)
		}
	}
}

func TestNoHayHerramientasRepetidas(t *testing.T) {
	vistas := map[string]bool{}
	for _, h := range construirHerramientas(Servicios{Destinos: destinosDelAsistente}) {
		if vistas[h.Nombre] {
			t.Errorf("herramienta repetida: %s", h.Nombre)
		}
		vistas[h.Nombre] = true
	}
	if len(vistas) < 15 {
		t.Errorf("solo hay %d herramientas; quedan unas veinte tras quitar cuatro módulos", len(vistas))
	}
}

// Lo que el asistente NO debe poder hacer, aunque se lo pidan.
func TestNoHayHerramientasQueBorrenOCambienRoles(t *testing.T) {
	prohibidas := []string{"delete", "remove", "drop", "set_role", "role", "password", "token"}

	for _, h := range construirHerramientas(Servicios{Destinos: destinosDelAsistente}) {
		for _, p := range prohibidas {
			if strings.Contains(strings.ToLower(h.Nombre), p) {
				t.Errorf("la herramienta %q parece peligrosa (%q)", h.Nombre, p)
			}
		}
	}
}

func jsonValido(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}
