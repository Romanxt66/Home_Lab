package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

func herramientas() []Tool {
	return []Tool{
		{Name: "leer", Description: "lee", Schema: json.RawMessage(`{"type":"object","properties":{}}`), ReadOnly: true,
			Handler: func(_ context.Context, w Caller, a json.RawMessage) (string, error) {
				return "hola " + w.Name + " " + string(a), nil
			}},
		{Name: "escribir", Description: "escribe", Schema: json.RawMessage(`{"type":"object"}`),
			Handler: func(context.Context, Caller, json.RawMessage) (string, error) { return "hecho", nil }},
		{Name: "falla_de_negocio", Schema: json.RawMessage(`{}`), ReadOnly: true,
			Handler: func(context.Context, Caller, json.RawMessage) (string, error) { return "", ToolError("no existe eso") }},
		{Name: "falla_tecnica", Schema: json.RawMessage(`{}`), ReadOnly: true,
			Handler: func(context.Context, Caller, json.RawMessage) (string, error) {
				return "", errors.New("conexión rota con 10.0.0.5")
			}},
		{Name: "panico", Schema: json.RawMessage(`{}`), ReadOnly: true,
			Handler: func(context.Context, Caller, json.RawMessage) (string, error) { panic("ay") }},
		{Name: "enorme", Schema: json.RawMessage(`{}`), ReadOnly: true,
			Handler: func(context.Context, Caller, json.RawMessage) (string, error) {
				return strings.Repeat("é", MaxBytesRespuesta), nil // 2 bytes cada una: pasa del límite
			}},
	}
}

func llamar(t *testing.T, s *Server, who Caller, cuerpo string) map[string]any {
	t.Helper()
	b := s.Handle(context.Background(), who, []byte(cuerpo))
	if b == nil {
		return nil
	}
	var r map[string]any
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("respuesta que no es JSON: %s", b)
	}
	return r
}

var (
	lector   = Caller{UserID: "ana", Name: "Ana"}
	escritor = Caller{UserID: "ana", Name: "Ana", CanWrite: true}
)

func TestInitialize(t *testing.T) {
	s := NewServer("home-lab", "1.0.0", "instrucciones", herramientas())
	for pedida, esperada := range map[string]string{"2025-03-26": "2025-03-26", "2024-11-05": "2024-11-05", "1999-01-01": "2025-06-18", "": "2025-06-18"} {
		r := llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+pedida+`"}}`)
		res := r["result"].(map[string]any)
		if res["protocolVersion"] != esperada {
			t.Errorf("pidió %q: respondió %v, esperaba %s", pedida, res["protocolVersion"], esperada)
		}
		if res["serverInfo"].(map[string]any)["name"] != "home-lab" || res["instructions"] != "instrucciones" {
			t.Errorf("serverInfo/instructions mal: %v", res)
		}
		if _, ok := res["capabilities"].(map[string]any)["tools"]; !ok {
			t.Error("debe anunciar tools")
		}
	}
}

func TestPingYNotificaciones(t *testing.T) {
	s := NewServer("x", "1", "", herramientas())
	if r := llamar(t, s, lector, `{"jsonrpc":"2.0","id":"abc","method":"ping"}`); r["id"] != "abc" || r["error"] != nil {
		t.Errorf("ping: %v", r)
	}
	if r := llamar(t, s, lector, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); r != nil {
		t.Errorf("una notificación no se contesta: %v", r)
	}
}

func TestToolsList_segunPermiso(t *testing.T) {
	s := NewServer("x", "1", "", herramientas())
	nombres := func(w Caller) []string {
		r := llamar(t, s, w, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		var out []string
		for _, h := range r["result"].(map[string]any)["tools"].([]any) {
			out = append(out, h.(map[string]any)["name"].(string))
		}
		return out
	}
	if strings.Contains(strings.Join(nombres(lector), ","), "escribir") {
		t.Error("un token de lectura no debe ver las herramientas que escriben")
	}
	if !strings.Contains(strings.Join(nombres(escritor), ","), "escribir") {
		t.Error("uno de escritura sí")
	}
	r := llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	primera := r["result"].(map[string]any)["tools"].([]any)[0].(map[string]any)
	if primera["annotations"].(map[string]any)["readOnlyHint"] != true || primera["inputSchema"] == nil {
		t.Errorf("faltan annotations o esquema: %v", primera)
	}
}

func texto(r map[string]any) (string, bool) {
	res := r["result"].(map[string]any)
	return res["content"].([]any)[0].(map[string]any)["text"].(string), res["isError"].(bool)
}

func TestToolsCall(t *testing.T) {
	s := NewServer("x", "1", "", herramientas())

	r := llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"leer","arguments":{"a":1}}}`)
	if tx, e := texto(r); e || tx != `hola Ana {"a":1}` {
		t.Errorf("llamada normal: %q %v", tx, e)
	}
	// Sin «arguments» se pasan unas llaves vacías.
	r = llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"leer"}}`)
	if tx, _ := texto(r); tx != "hola Ana {}" {
		t.Errorf("sin argumentos: %q", tx)
	}

	// Escribir con un token de lectura: se le explica al modelo, no es un fallo del protocolo.
	r = llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"escribir"}}`)
	if tx, e := texto(r); !e || !strings.Contains(tx, "solo lectura") {
		t.Errorf("escribir con lectura: %q %v", tx, e)
	}
	if tx, e := texto(llamar(t, s, escritor, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"escribir"}}`)); e || tx != "hecho" {
		t.Errorf("escribir con escritura: %q %v", tx, e)
	}

	// Un fallo de negocio se le dice al modelo tal cual; uno técnico NO filtra detalles.
	if tx, e := texto(llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"falla_de_negocio"}}`)); !e || tx != "no existe eso" {
		t.Errorf("fallo de negocio: %q %v", tx, e)
	}
	tx, e := texto(llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"falla_tecnica"}}`))
	if !e || strings.Contains(tx, "10.0.0.5") {
		t.Errorf("un error técnico no debe filtrar detalles internos: %q", tx)
	}
	// Un pánico no tumba el servidor.
	if tx, e := texto(llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"panico"}}`)); !e || strings.Contains(tx, "ay") {
		t.Errorf("pánico: %q %v", tx, e)
	}

	// Tamaño: se recorta por UTF-8 válido y avisa.
	tx, _ = texto(llamar(t, s, lector, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"enorme"}}`))
	if len(tx) > MaxBytesRespuesta+200 || !strings.Contains(tx, "recortada") {
		t.Errorf("la respuesta enorme debía recortarse: %d bytes", len(tx))
	}
	if !utf8.ValidString(tx) {
		t.Error("el recorte no debe partir un carácter")
	}
}

func TestErroresDeProtocolo(t *testing.T) {
	s := NewServer("x", "1", "", herramientas())
	casos := map[string]struct {
		cuerpo string
		codigo float64
	}{
		"json roto":               {`{no es json`, codigoJSONInvalido},
		"sin jsonrpc":             {`{"id":1,"method":"ping"}`, codigoPeticionInvalida},
		"lote":                    {`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, codigoPeticionInvalida},
		"método desconocido":      {`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, codigoMetodoDesconocido},
		"herramienta inexistente": {`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"no_existe"}}`, codigoParametros},
		"sin nombre":              {`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`, codigoParametros},
	}
	for nombre, c := range casos {
		r := llamar(t, s, lector, c.cuerpo)
		e, ok := r["error"].(map[string]any)
		if !ok || e["code"] != c.codigo {
			t.Errorf("%s: %v", nombre, r)
		}
	}
}

// ── HTTP ───────────────────────────────────────────────────────────────────

type autFalsa struct{ identidad *tokens.Identidad }

func (a autFalsa) Autenticar(_ context.Context, claro string) (*tokens.Identidad, error) {
	if claro == "hl_valido" {
		return a.identidad, nil
	}
	if claro == "hl_error" {
		return nil, errors.New("base caída")
	}
	return nil, nil
}

func servidor(origenes ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	id := &tokens.Identidad{UsuarioID: "ana", Nombre: "Ana", Alcance: tokens.Lectura}
	NewHandler(NewServer("x", "1", "", herramientas()), autFalsa{id}, origenes, func(*gin.Context) string { return "https://lab.ejemplo/.well-known/oauth-protected-resource/mcp" }).Register(r)
	return r
}

func post(r *gin.Engine, cabeceras map[string]string, cuerpo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(cuerpo))
	for k, v := range cabeceras {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

const ping = `{"jsonrpc":"2.0","id":1,"method":"ping"}`

func TestHTTP_autenticacion(t *testing.T) {
	r := servidor()
	for nombre, cab := range map[string]map[string]string{
		"sin cabecera": {},
		"esquema malo": {"Authorization": "Basic hl_valido"},
		"token falso":  {"Authorization": "Bearer hl_otro"},
		"token vacío":  {"Authorization": "Bearer "},
	} {
		rec := post(r, cab, ping)
		if rec.Code != 401 || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer") ||
			!strings.Contains(rec.Header().Get("WWW-Authenticate"), `resource_metadata="https://lab.ejemplo/.well-known/oauth-protected-resource/mcp"`) {
			t.Errorf("%s: %d %v", nombre, rec.Code, rec.Header())
		}
	}
	if rec := post(r, map[string]string{"Authorization": "Bearer hl_error"}, ping); rec.Code != 500 || strings.Contains(rec.Body.String(), "base caída") {
		t.Errorf("un fallo técnico es 500 y no filtra el detalle: %d %s", rec.Code, rec.Body)
	}
	rec := post(r, map[string]string{"Authorization": "Bearer hl_valido"}, ping)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" || !strings.Contains(rec.Body.String(), `"result"`) {
		t.Errorf("con token válido: %d %s", rec.Code, rec.Body)
	}
}

func TestHTTP_notificacionYMetodos(t *testing.T) {
	r := servidor()
	auth := map[string]string{"Authorization": "Bearer hl_valido"}
	if rec := post(r, auth, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); rec.Code != 202 || rec.Body.Len() != 0 {
		t.Errorf("una notificación da 202 sin cuerpo: %d %q", rec.Code, rec.Body)
	}
	for _, metodo := range []string{http.MethodGet, http.MethodDelete} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(metodo, "/mcp", nil))
		if rec.Code != 405 || rec.Header().Get("Allow") != "POST" {
			t.Errorf("%s: %d %v", metodo, rec.Code, rec.Header())
		}
	}
}

func TestHTTP_origen(t *testing.T) {
	auth := func(origen string) map[string]string {
		c := map[string]string{"Authorization": "Bearer hl_valido"}
		if origen != "" {
			c["Origin"] = origen
		}
		return c
	}
	r := servidor("https://panel.ejemplo.com")
	// Sin Origin (Claude Code, un script): bien. Con uno permitido: bien.
	for _, o := range []string{"", "https://panel.ejemplo.com"} {
		if rec := post(r, auth(o), ping); rec.Code != 200 {
			t.Errorf("origen %q debía valer: %d", o, rec.Code)
		}
	}
	// Una página ajena en un navegador: no.
	if rec := post(r, auth("https://malo.example"), ping); rec.Code != 403 {
		t.Errorf("origen ajeno: esperaba 403 y fue %d", rec.Code)
	}
	// El propio sitio (mismo host) también vale.
	req := httptest.NewRequest(http.MethodPost, "http://lab.softlane.click/mcp", bytes.NewBufferString(ping))
	req.Header.Set("Authorization", "Bearer hl_valido")
	req.Header.Set("Origin", "https://lab.softlane.click")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("mismo origen: %d", rec.Code)
	}
}

func TestCombinar(t *testing.T) {
	uno := autFalsa{&tokens.Identidad{UsuarioID: "uno"}}
	vacio := autVacia{}
	// Se queda con el primero que reconoce el token.
	if id, _ := Combinar(vacio, uno).Autenticar(context.Background(), "hl_valido"); id == nil || id.UsuarioID != "uno" {
		t.Error("debía llegar al segundo autenticador")
	}
	if id, err := Combinar(vacio, vacio).Autenticar(context.Background(), "x"); id != nil || err != nil {
		t.Error("si ninguno lo reconoce, no es nadie y no es un error")
	}
	// Un fallo técnico no se confunde con «token inválido» ni se salta.
	if _, err := Combinar(uno, vacio).Autenticar(context.Background(), "hl_error"); err == nil {
		t.Error("un error técnico debe propagarse")
	}
}

type autVacia struct{}

func (autVacia) Autenticar(context.Context, string) (*tokens.Identidad, error) { return nil, nil }
