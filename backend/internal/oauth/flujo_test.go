package oauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

// ── Repositorio en memoria ─────────────────────────────────────────────────

type tokenFalso struct {
	grant  string
	tipo   string
	caduca time.Time
	usado  bool
}

type repoFalso struct {
	mu        sync.Mutex
	clientes  map[string]Cliente
	codigos   map[string]*Codigo
	usados    map[string]bool
	concs     map[string]*Concesion
	revocadas map[string]bool
	tokens    map[string]*tokenFalso
	estado    string // estado de la cuenta del usuario
}

func nuevoRepo() *repoFalso {
	return &repoFalso{clientes: map[string]Cliente{}, codigos: map[string]*Codigo{}, usados: map[string]bool{}, concs: map[string]*Concesion{},
		revocadas: map[string]bool{}, tokens: map[string]*tokenFalso{}, estado: "approved"}
}

func (r *repoFalso) CrearCliente(_ context.Context, c Cliente) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientes[c.ID] = c
	return nil
}
func (r *repoFalso) ObtenerCliente(_ context.Context, id string) (*Cliente, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.clientes[id]; ok {
		return &c, nil
	}
	return nil, nil
}
func (r *repoFalso) ContarClientes(context.Context) (int, error) { return len(r.clientes), nil }
func (r *repoFalso) LimpiarClientesSinUso(_ context.Context, antes time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, c := range r.clientes {
		if c.CreadoEn.Before(antes) {
			usado := false
			for _, g := range r.concs {
				usado = usado || g.ClienteID == id
			}
			if !usado {
				delete(r.clientes, id)
			}
		}
	}
	return nil
}
func (r *repoFalso) GuardarCodigo(_ context.Context, c Codigo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codigos[c.Huella] = &c
	return nil
}
func (r *repoFalso) CanjearCodigo(_ context.Context, h string, ahora time.Time) (*Codigo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.codigos[h]
	if !ok || r.usados[h] || !c.Caduca.After(ahora) {
		return nil, nil
	}
	r.usados[h] = true
	return c, nil
}
func (r *repoFalso) CrearConcesion(_ context.Context, c Concesion, a, rf RegistroToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.concs[c.ID] = &c
	r.tokens[a.Huella] = &tokenFalso{c.ID, "access", a.Caduca, false}
	r.tokens[rf.Huella] = &tokenFalso{c.ID, "refresh", rf.Caduca, false}
	return nil
}
func (r *repoFalso) Rotar(_ context.Context, h string, ahora time.Time, a, rf RegistroToken) (*Concesion, ResultadoRotacion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[h]
	if !ok || t.tipo != "refresh" || r.revocadas[t.grant] {
		return nil, Invalido, nil
	}
	if t.usado {
		r.revocadas[t.grant] = true
		return nil, Reutilizado, nil
	}
	if !t.caduca.After(ahora) || r.estado != "approved" {
		return nil, Invalido, nil
	}
	t.usado = true
	r.tokens[a.Huella] = &tokenFalso{t.grant, "access", a.Caduca, false}
	r.tokens[rf.Huella] = &tokenFalso{t.grant, "refresh", rf.Caduca, false}
	g := *r.concs[t.grant]
	return &g, Rotado, nil
}
func (r *repoFalso) BuscarAcceso(_ context.Context, h string, ahora time.Time) (*tokens.Identidad, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[h]
	if !ok || t.tipo != "access" || !t.caduca.After(ahora) || r.revocadas[t.grant] || r.estado != "approved" {
		return nil, nil
	}
	g := r.concs[t.grant]
	return &tokens.Identidad{TokenID: g.ID, UsuarioID: g.UsuarioID, Alcance: tokens.Alcance(g.Alcance)}, nil
}
func (r *repoFalso) ListarConcesiones(_ context.Context, uid string) ([]Concesion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Concesion
	for id, g := range r.concs {
		if g.UsuarioID == uid && !r.revocadas[id] {
			c := *g
			c.ClienteNombre = r.clientes[g.ClienteID].Nombre
			out = append(out, c)
		}
	}
	return out, nil
}
func (r *repoFalso) RevocarConcesion(_ context.Context, uid, id string, _ time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.concs[id]; ok && g.UsuarioID == uid && !r.revocadas[id] {
		r.revocadas[id] = true
		return true, nil
	}
	return false, nil
}
func (r *repoFalso) RevocarPorToken(_ context.Context, h, cliente string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[h]; ok && r.concs[t.grant].ClienteID == cliente {
		r.revocadas[t.grant] = true
	}
	return nil
}

// ── Montaje ────────────────────────────────────────────────────────────────

const (
	verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	callback  = "https://claude.ai/api/mcp/auth_callback"
	origen    = "http://lab.test"
)

type entorno struct {
	t      *testing.T
	repo   *repoFalso
	svc    *Service
	motor  *gin.Engine
	ahora  time.Time
	sesion string // uid con sesión; vacío = sin sesión
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := &entorno{t: t, repo: nuevoRepo(), ahora: time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)}
	e.svc = NewService(e.repo, []string{"claude.ai"})
	e.svc.ahora = func() time.Time { return e.ahora }
	h := NewHandler(e.svc)

	e.motor = gin.New()
	h.RegistrarPublico(e.motor)
	// La sesión: un middleware de prueba en el grupo, como haría RequireAuth.
	sesion := e.motor.Group("/api/oauth", func(c *gin.Context) {
		if e.sesion != "" {
			c.Set("usuario", &auth.SessionPayload{UID: e.sesion, Email: e.sesion + "@x.com", Role: "user"})
		}
	})
	h.RegistrarSesion(sesion)
	return e
}

func (e *entorno) llamar(metodo, ruta string, cuerpo string, tipo string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, origen+ruta, strings.NewReader(cuerpo))
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}
	rec := httptest.NewRecorder()
	e.motor.ServeHTTP(rec, req)
	return rec
}

func (e *entorno) json(rec *httptest.ResponseRecorder) map[string]any {
	e.t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		e.t.Fatalf("no es JSON: %d %s", rec.Code, rec.Body)
	}
	return m
}

func (e *entorno) registrar(nombre string, uris ...string) string {
	cuerpo, _ := json.Marshal(map[string]any{"client_name": nombre, "redirect_uris": uris})
	rec := e.llamar("POST", "/oauth/register", string(cuerpo), "application/json")
	if rec.Code != 201 {
		e.t.Fatalf("registrar: %d %s", rec.Code, rec.Body)
	}
	return e.json(rec)["client_id"].(string)
}

func consulta(cliente string, extra map[string]string) url.Values {
	q := url.Values{"response_type": {"code"}, "client_id": {cliente}, "redirect_uri": {callback}, "state": {"estado-123"},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "scope": {"mcp:read mcp:write"}, "resource": {origen + "/mcp"}}
	for k, v := range extra {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	return q
}

// aprobar hace lo que la pantalla: decide y devuelve la dirección de retorno.
func (e *entorno) aprobar(q url.Values, aprobar bool, permiso string) *url.URL {
	cuerpo, _ := json.Marshal(map[string]any{
		"response_type": q.Get("response_type"), "client_id": q.Get("client_id"), "redirect_uri": q.Get("redirect_uri"), "state": q.Get("state"),
		"code_challenge": q.Get("code_challenge"), "code_challenge_method": q.Get("code_challenge_method"), "scope": q.Get("scope"),
		"resource": q.Get("resource"), "approve": aprobar, "permission": permiso})
	rec := e.llamar("POST", "/api/oauth/approve", string(cuerpo), "application/json")
	if rec.Code != 200 {
		e.t.Fatalf("aprobar: %d %s", rec.Code, rec.Body)
	}
	u, err := url.Parse(e.json(rec)["redirect"].(string))
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

func (e *entorno) token(campos map[string]string) *httptest.ResponseRecorder {
	f := url.Values{}
	for k, v := range campos {
		f.Set(k, v)
	}
	return e.llamar("POST", "/oauth/token", f.Encode(), "application/x-www-form-urlencoded")
}

// completo recorre registro → autorización → token y devuelve (cliente, tokens).
func (e *entorno) completo(permiso string) (string, map[string]any) {
	e.sesion = "ana"
	cliente := e.registrar("Claude", callback)
	u := e.aprobar(consulta(cliente, nil), true, permiso)
	rec := e.token(map[string]string{"grant_type": "authorization_code", "code": u.Query().Get("code"), "redirect_uri": callback, "client_id": cliente, "code_verifier": verifier})
	if rec.Code != 200 {
		e.t.Fatalf("token: %d %s", rec.Code, rec.Body)
	}
	return cliente, e.json(rec)
}

// ── Pruebas ────────────────────────────────────────────────────────────────

func TestMetadatos(t *testing.T) {
	e := nuevoEntorno(t)
	rec := e.llamar("GET", "/.well-known/oauth-protected-resource/mcp", "", "")
	m := e.json(rec)
	if rec.Code != 200 || m["resource"] != origen+"/mcp" || m["authorization_servers"].([]any)[0] != origen {
		t.Errorf("recurso protegido: %v", m)
	}
	rec = e.llamar("GET", "/.well-known/oauth-protected-resource", "", "")
	if rec.Code != 200 {
		t.Errorf("también sin sufijo: %d", rec.Code)
	}
	m = e.json(e.llamar("GET", "/.well-known/oauth-authorization-server", "", ""))
	if m["issuer"] != origen || m["token_endpoint"] != origen+"/oauth/token" || m["registration_endpoint"] != origen+"/oauth/register" ||
		m["authorization_endpoint"] != origen+"/oauth/authorize" {
		t.Errorf("servidor de autorización: %v", m)
	}
	if got := m["code_challenge_methods_supported"].([]any); len(got) != 1 || got[0] != "S256" {
		t.Errorf("solo S256: %v", got)
	}
	if got := m["token_endpoint_auth_methods_supported"].([]any); len(got) != 1 || got[0] != "none" {
		t.Errorf("solo clientes públicos: %v", got)
	}
}

func TestRegistro(t *testing.T) {
	e := nuevoEntorno(t)
	cliente := e.registrar("  Claude‮  ", callback, callback, "http://localhost:3000/cb")
	c, _ := e.repo.ObtenerCliente(context.Background(), cliente)
	if !strings.HasPrefix(cliente, "hlc_") || c.Nombre != "Claude" || len(c.RedirectURIs) != 2 {
		t.Errorf("registro: %+v", c)
	}
	mal := []string{
		`{"client_name":"x"}`, `{"client_name":"x","redirect_uris":[]}`,
		`{"client_name":"x","redirect_uris":["https://malo.example/cb"]}`,
		`{"client_name":"x","redirect_uris":["https://claude.ai/cb","javascript:alert(1)"]}`,
		`{"client_name":"x","redirect_uris":["https://claude.ai/1","https://claude.ai/2","https://claude.ai/3","https://claude.ai/4","https://claude.ai/5","https://claude.ai/6"]}`,
		`{"client_name":"x","redirect_uris":["https://claude.ai/cb"],"grant_types":["password"]}`,
		`no es json`,
	}
	for _, cuerpo := range mal {
		rec := e.llamar("POST", "/oauth/register", cuerpo, "application/json")
		if rec.Code != 400 || e.json(rec)["error"] == nil {
			t.Errorf("%s → %d %s", cuerpo, rec.Code, rec.Body)
		}
	}
	// Tope de clientes registrados.
	for i := len(e.repo.clientes); i < MaxClientes; i++ {
		e.repo.clientes["x"+string(rune(i+1000))] = Cliente{ID: "x" + string(rune(i+1000)), CreadoEn: e.ahora}
	}
	if rec := e.llamar("POST", "/oauth/register", `{"client_name":"x","redirect_uris":["https://claude.ai/cb"]}`, "application/json"); rec.Code != 503 {
		t.Errorf("con el cupo lleno: esperaba 503 y fue %d", rec.Code)
	}
	// Los que nadie usó en una semana se liberan.
	e.ahora = e.ahora.Add(8 * 24 * time.Hour)
	if rec := e.llamar("POST", "/oauth/register", `{"client_name":"x","redirect_uris":["https://claude.ai/cb"]}`, "application/json"); rec.Code != 201 {
		t.Errorf("tras limpiar los sin uso: %d %s", rec.Code, rec.Body)
	}
}

func TestAutorizar_validaAntesDeRedirigir(t *testing.T) {
	e := nuevoEntorno(t)
	cliente := e.registrar("Claude", callback)

	// Bien: va a la pantalla de consentimiento con los parámetros.
	rec := e.llamar("GET", "/oauth/authorize?"+consulta(cliente, nil).Encode(), "", "")
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if rec.Code != 302 || loc.Path != "/oauth/autorizar" || loc.Query().Get("client_id") != cliente || loc.Query().Get("state") != "estado-123" {
		t.Fatalf("autorizar bien: %d %v", rec.Code, rec.Header())
	}

	// Cliente desconocido o redirección que no coincide: NO se redirige a ninguna parte, se enseña el error.
	for nombre, q := range map[string]url.Values{
		"cliente desconocido":    consulta("hlc_inventado", nil),
		"sin client_id":          consulta("", map[string]string{"client_id": ""}),
		"redirect distinto":      consulta(cliente, map[string]string{"redirect_uri": "https://claude.ai/otra"}),
		"redirect de otro sitio": consulta(cliente, map[string]string{"redirect_uri": "https://malo.example/cb"}),
		"sin redirect":           consulta(cliente, map[string]string{"redirect_uri": ""}),
	} {
		rec := e.llamar("GET", "/oauth/authorize?"+q.Encode(), "", "")
		if rec.Code != 400 || rec.Header().Get("Location") != "" || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("%s: esperaba 400 sin redirección: %d %v", nombre, rec.Code, rec.Header())
		}
	}

	// El resto de fallos vuelven al cliente, con su state y el emisor.
	for nombre, q := range map[string]url.Values{
		"sin PKCE":          consulta(cliente, map[string]string{"code_challenge": "", "code_challenge_method": ""}),
		"método plain":      consulta(cliente, map[string]string{"code_challenge_method": "plain"}),
		"challenge corto":   consulta(cliente, map[string]string{"code_challenge": "abc"}),
		"tipo de respuesta": consulta(cliente, map[string]string{"response_type": "token"}),
		"recurso ajeno":     consulta(cliente, map[string]string{"resource": "https://otro.example/mcp"}),
	} {
		rec := e.llamar("GET", "/oauth/authorize?"+q.Encode(), "", "")
		l, _ := url.Parse(rec.Header().Get("Location"))
		if rec.Code != 302 || l.Host != "claude.ai" || l.Query().Get("error") == "" || l.Query().Get("state") != "estado-123" || l.Query().Get("iss") != origen {
			t.Errorf("%s: %d %v", nombre, rec.Code, rec.Header())
		}
	}
}

func TestPaginaDeErrorEscapaElTexto(t *testing.T) {
	e := nuevoEntorno(t)
	rec := e.llamar("GET", "/oauth/authorize?client_id=<script>alert(1)</script>&redirect_uri=x", "", "")
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Errorf("el HTML de error no debe reflejar la entrada sin escapar: %s", rec.Body)
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Error("la página de error lleva una política que prohíbe scripts")
	}
}

func TestFlujoCompleto(t *testing.T) {
	e := nuevoEntorno(t)
	cliente, tk := e.completo("write")

	if tk["token_type"] != "Bearer" || tk["expires_in"].(float64) != 3600 || tk["scope"] != "mcp:read mcp:write" {
		t.Errorf("respuesta del token: %v", tk)
	}
	acceso, refresco := tk["access_token"].(string), tk["refresh_token"].(string)
	if !strings.HasPrefix(acceso, "hlo_") || !strings.HasPrefix(refresco, "hlr_") {
		t.Errorf("prefijos: %q %q", acceso, refresco)
	}

	// El token de acceso identifica a la persona, con el permiso concedido.
	id, err := e.svc.Autenticar(context.Background(), acceso)
	if err != nil || id == nil || id.UsuarioID != "ana" || !id.Alcance.PuedeEscribir() {
		t.Fatalf("autenticar: %+v %v", id, err)
	}
	// Caduca a la hora.
	e.ahora = e.ahora.Add(61 * time.Minute)
	if id, _ := e.svc.Autenticar(context.Background(), acceso); id != nil {
		t.Error("el token de acceso caduca a la hora")
	}

	// Refrescar da tokens nuevos y el refresco viejo ya no sirve.
	rec := e.token(map[string]string{"grant_type": "refresh_token", "refresh_token": refresco, "client_id": cliente})
	if rec.Code != 200 {
		t.Fatalf("refrescar: %d %s", rec.Code, rec.Body)
	}
	nuevo := e.json(rec)
	if nuevo["access_token"] == acceso || nuevo["refresh_token"] == refresco {
		t.Error("los tokens se rotan")
	}
	if id, _ := e.svc.Autenticar(context.Background(), nuevo["access_token"].(string)); id == nil {
		t.Error("el acceso nuevo vale")
	}
}

func TestPermisoLoDecideLaPersona(t *testing.T) {
	e := nuevoEntorno(t)
	// Pide escribir, pero ella concede solo leer.
	_, tk := e.completo("read")
	if tk["scope"] != "mcp:read" {
		t.Errorf("scope: %v", tk["scope"])
	}
	id, _ := e.svc.Autenticar(context.Background(), tk["access_token"].(string))
	if id.Alcance.PuedeEscribir() {
		t.Error("lo concedido manda, no lo pedido")
	}
	// Un permiso raro se queda en el menor.
	_, tk = e.completo("admin")
	if tk["scope"] != "mcp:read" {
		t.Errorf("un permiso desconocido es solo lectura: %v", tk["scope"])
	}
}

func TestAprobar_sesionYDenegar(t *testing.T) {
	e := nuevoEntorno(t)
	e.sesion = ""
	if rec := e.llamar("POST", "/api/oauth/approve", `{}`, "application/json"); rec.Code != 401 {
		t.Errorf("sin sesión: %d", rec.Code)
	}
	if rec := e.llamar("GET", "/api/oauth/request", "", ""); rec.Code != 401 {
		t.Errorf("sin sesión: %d", rec.Code)
	}
	e.sesion = "ana"
	cliente := e.registrar("Claude", callback)

	// Denegar devuelve access_denied al cliente y no crea ningún código.
	u := e.aprobar(consulta(cliente, nil), false, "")
	if u.Query().Get("error") != "access_denied" || u.Query().Get("code") != "" || u.Query().Get("state") != "estado-123" {
		t.Errorf("denegar: %v", u)
	}
	if len(e.repo.codigos) != 0 {
		t.Error("no se crea código si se deniega")
	}

	// Aprobar con una petición manipulada (redirect de otro sitio) se rechaza aunque se haya mostrado una buena.
	cuerpo, _ := json.Marshal(map[string]any{"client_id": cliente, "redirect_uri": "https://malo.example/cb", "code_challenge": challenge, "code_challenge_method": "S256", "response_type": "code", "approve": true})
	rec := e.llamar("POST", "/api/oauth/approve", string(cuerpo), "application/json")
	if rec.Code != 400 || len(e.repo.codigos) != 0 {
		t.Errorf("redirect manipulado: %d %s", rec.Code, rec.Body)
	}

	// La pantalla de consentimiento sabe quién pide y a dónde volverá.
	q := consulta(cliente, nil)
	m := e.json(e.llamar("GET", "/api/oauth/request?"+q.Encode(), "", ""))
	if m["clientName"] != "Claude" || m["redirectHost"] != "claude.ai" || m["loopback"] != false || m["requestedWrite"] != true {
		t.Errorf("request: %v", m)
	}
	if rec := e.llamar("GET", "/api/oauth/request?"+consulta("hlc_nada", nil).Encode(), "", ""); rec.Code != 400 {
		t.Errorf("request de un cliente falso: %d", rec.Code)
	}
}

func TestCanje_rechazos(t *testing.T) {
	e := nuevoEntorno(t)
	e.sesion = "ana"
	cliente := e.registrar("Claude", callback)
	otro := e.registrar("Otro", callback)
	obtener := func() string { return e.aprobar(consulta(cliente, nil), true, "read").Query().Get("code") }
	ok := func(code string) map[string]string {
		return map[string]string{"grant_type": "authorization_code", "code": code, "redirect_uri": callback, "client_id": cliente, "code_verifier": verifier}
	}
	cambiar := func(m map[string]string, k, v string) map[string]string {
		c := map[string]string{}
		for a, b := range m {
			c[a] = b
		}
		if v == "" {
			delete(c, k)
		} else {
			c[k] = v
		}
		return c
	}

	casos := map[string]struct {
		campos map[string]string
		codigo int
		error  string
	}{
		"verifier distinto":   {cambiar(ok(obtener()), "code_verifier", strings.Repeat("a", 43)), 400, "invalid_grant"},
		"sin verifier":        {cambiar(ok(obtener()), "code_verifier", ""), 400, "invalid_request"},
		"redirect distinto":   {cambiar(ok(obtener()), "redirect_uri", "https://claude.ai/otra"), 400, "invalid_grant"},
		"código inventado":    {cambiar(ok(obtener()), "code", "inventado"), 400, "invalid_grant"},
		"otro cliente":        {cambiar(ok(obtener()), "client_id", otro), 400, "invalid_grant"},
		"cliente desconocido": {cambiar(ok(obtener()), "client_id", "hlc_nada"), 401, "invalid_client"},
		"sin código":          {cambiar(ok(obtener()), "code", ""), 400, "invalid_request"},
		"grant_type raro":     {cambiar(ok(obtener()), "grant_type", "password"), 400, "unsupported_grant_type"},
		"recurso ajeno":       {cambiar(ok(obtener()), "resource", "https://otro.example/mcp"), 400, "invalid_target"},
	}
	for nombre, c := range casos {
		rec := e.token(c.campos)
		m := e.json(rec)
		if rec.Code != c.codigo || m["error"] != c.error || m["access_token"] != nil {
			t.Errorf("%s: esperaba %d/%s y fue %d %v", nombre, c.codigo, c.error, rec.Code, m)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: la respuesta del token no debe cachearse", nombre)
		}
	}

	// El código se quema aunque el intento fallara: no hay segunda oportunidad.
	code := obtener()
	e.token(cambiar(ok(code), "code_verifier", strings.Repeat("a", 43)))
	if rec := e.token(ok(code)); rec.Code != 400 {
		t.Errorf("tras un intento fallido, el código ya no sirve: %d", rec.Code)
	}

	// Un solo uso.
	code = obtener()
	if rec := e.token(ok(code)); rec.Code != 200 {
		t.Fatalf("primer canje: %d", rec.Code)
	}
	if rec := e.token(ok(code)); rec.Code != 400 {
		t.Errorf("segundo canje: %d", rec.Code)
	}

	// Caduca a los 5 minutos.
	code = obtener()
	e.ahora = e.ahora.Add(5*time.Minute + time.Second)
	if rec := e.token(ok(code)); rec.Code != 400 {
		t.Errorf("código caducado: %d", rec.Code)
	}
}

func TestRefresco_reutilizacionRevocaTodo(t *testing.T) {
	e := nuevoEntorno(t)
	cliente, tk := e.completo("write")
	r1 := tk["refresh_token"].(string)

	rec := e.token(map[string]string{"grant_type": "refresh_token", "refresh_token": r1, "client_id": cliente})
	if rec.Code != 200 {
		t.Fatalf("primer refresco: %d", rec.Code)
	}
	nuevo := e.json(rec)

	// Reusar el refresco ya gastado: se revoca TODA la concesión (también lo emitido después).
	rec = e.token(map[string]string{"grant_type": "refresh_token", "refresh_token": r1, "client_id": cliente})
	if rec.Code != 400 || e.json(rec)["error"] != "invalid_grant" {
		t.Errorf("reutilizar: %d %s", rec.Code, rec.Body)
	}
	if id, _ := e.svc.Autenticar(context.Background(), nuevo["access_token"].(string)); id != nil {
		t.Error("tras detectar la reutilización el acceso de esa concesión deja de valer")
	}
	if rec := e.token(map[string]string{"grant_type": "refresh_token", "refresh_token": nuevo["refresh_token"].(string), "client_id": cliente}); rec.Code != 400 {
		t.Errorf("y el refresco nuevo tampoco: %d", rec.Code)
	}
}

func TestRefresco_rechazos(t *testing.T) {
	e := nuevoEntorno(t)
	cliente, tk := e.completo("write")
	otro := e.registrar("Otro", callback)
	r := tk["refresh_token"].(string)
	refrescar := func(campos map[string]string) int {
		campos["grant_type"] = "refresh_token"
		return e.token(campos).Code
	}
	if c := refrescar(map[string]string{"refresh_token": r, "client_id": otro}); c != 400 {
		t.Errorf("el refresco de un cliente no sirve a otro: %d", c)
	}
	if c := refrescar(map[string]string{"refresh_token": tk["access_token"].(string), "client_id": cliente}); c != 400 {
		t.Errorf("un token de acceso no es un refresco: %d", c)
	}
	if c := refrescar(map[string]string{"refresh_token": "hlr_corto", "client_id": cliente}); c != 400 {
		t.Errorf("mala forma: %d", c)
	}
	if c := refrescar(map[string]string{"client_id": cliente}); c != 400 {
		t.Errorf("sin token: %d", c)
	}
	// Cuenta que deja de estar aprobada: no se puede renovar ni usar.
	e.repo.estado = "pending"
	if c := refrescar(map[string]string{"refresh_token": r, "client_id": cliente}); c != 400 {
		t.Errorf("cuenta no aprobada: %d", c)
	}
	if id, _ := e.svc.Autenticar(context.Background(), tk["access_token"].(string)); id != nil {
		t.Error("y su acceso deja de valer")
	}
	e.repo.estado = "approved"
	// Caduca a los 30 días.
	e.ahora = e.ahora.Add(31 * 24 * time.Hour)
	if c := refrescar(map[string]string{"refresh_token": r, "client_id": cliente}); c != 400 {
		t.Errorf("refresco caducado: %d", c)
	}
}

func TestRevocar(t *testing.T) {
	e := nuevoEntorno(t)
	cliente, tk := e.completo("write")
	acceso := tk["access_token"].(string)

	// /oauth/revoke siempre responde 200, también con basura.
	if rec := e.llamar("POST", "/oauth/revoke", "token=basura&client_id="+cliente, "application/x-www-form-urlencoded"); rec.Code != 200 {
		t.Errorf("revocar basura: %d", rec.Code)
	}
	if id, _ := e.svc.Autenticar(context.Background(), acceso); id == nil {
		t.Fatal("revocar basura no toca nada")
	}
	// Otro cliente no puede revocar la concesión ajena.
	otro := e.registrar("Otro", callback)
	e.llamar("POST", "/oauth/revoke", "token="+acceso+"&client_id="+otro, "application/x-www-form-urlencoded")
	if id, _ := e.svc.Autenticar(context.Background(), acceso); id == nil {
		t.Error("otro cliente no puede revocar lo que no es suyo")
	}
	// El suyo, sí.
	e.llamar("POST", "/oauth/revoke", "token="+acceso+"&client_id="+cliente, "application/x-www-form-urlencoded")
	if id, _ := e.svc.Autenticar(context.Background(), acceso); id != nil {
		t.Error("revocado")
	}
}

func TestConcesionesDeLaPersona(t *testing.T) {
	e := nuevoEntorno(t)
	_, tk := e.completo("read")
	e.sesion = "ana"
	var lista []map[string]any
	_ = json.Unmarshal(e.llamar("GET", "/api/oauth/grants", "", "").Body.Bytes(), &lista)
	if len(lista) != 1 || lista[0]["clientName"] != "Claude" || lista[0]["scope"] != "read" {
		t.Fatalf("lista: %v", lista)
	}
	id := lista[0]["id"].(string)

	// Otra persona no ve ni revoca la concesión de Ana.
	e.sesion = "luis"
	_ = json.Unmarshal(e.llamar("GET", "/api/oauth/grants", "", "").Body.Bytes(), &lista)
	if len(lista) != 0 {
		t.Error("Luis no ve las de Ana")
	}
	if rec := e.llamar("DELETE", "/api/oauth/grants/"+id, "", ""); rec.Code != 404 {
		t.Errorf("Luis revocando lo de Ana: %d", rec.Code)
	}
	// Ana sí, y el token deja de valer al instante.
	e.sesion = "ana"
	if rec := e.llamar("DELETE", "/api/oauth/grants/"+id, "", ""); rec.Code != 204 {
		t.Fatalf("revocar: %d", rec.Code)
	}
	if got, _ := e.svc.Autenticar(context.Background(), tk["access_token"].(string)); got != nil {
		t.Error("revocar la conexión invalida el token")
	}
}

func TestPKCE_extremoAExtremo(t *testing.T) {
	// Con un verifier propio (no el de la RFC) para comprobar el cálculo de punta a punta.
	e := nuevoEntorno(t)
	e.sesion = "ana"
	cliente := e.registrar("Claude", callback)
	v := "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MC1fLn5BQkNERUZHSA"
	h := sha256.Sum256([]byte(v))
	ch := base64.RawURLEncoding.EncodeToString(h[:])
	u := e.aprobar(consulta(cliente, map[string]string{"code_challenge": ch}), true, "read")
	campos := map[string]string{"grant_type": "authorization_code", "code": u.Query().Get("code"), "redirect_uri": callback, "client_id": cliente, "code_verifier": v}
	if rec := e.token(campos); rec.Code != 200 {
		t.Errorf("con el verifier correcto: %d %s", rec.Code, rec.Body)
	}
}

func TestLimite_deEnvios(t *testing.T) {
	// El cuerpo del registro está acotado: uno enorme se rechaza sin leerse entero.
	e := nuevoEntorno(t)
	grande := `{"client_name":"` + strings.Repeat("a", 40<<10) + `","redirect_uris":["https://claude.ai/cb"]}`
	if rec := e.llamar("POST", "/oauth/register", grande, "application/json"); rec.Code != 400 {
		t.Errorf("cuerpo enorme: %d", rec.Code)
	}
	_ = bytes.NewBuffer
	_ = http.StatusOK
}

// El servicio no se fía de que quien lo llame haya filtrado el permiso: ante uno
// desconocido concede el menor. (Desde HTTP no se llega, pero otro adaptador sí podría.)
func TestServicio_permisoDesconocidoEsLectura(t *testing.T) {
	e := nuevoEntorno(t)
	cliente := e.registrar("Claude", callback)
	p := Peticion{TipoRespuesta: "code", ClienteID: cliente, RedirectURI: callback, Challenge: challenge, Metodo: "S256"}
	destino, err := e.svc.Aprobar(context.Background(), "ana", p, origen+"/mcp", true, Alcance("root"), origen)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(destino)
	tk, e2 := e.svc.Canjear(context.Background(), TokenEntrada{GrantType: "authorization_code", Codigo: u.Query().Get("code"),
		RedirectURI: callback, ClienteID: cliente, Verifier: verifier}, origen+"/mcp")
	if e2 != nil || tk.Alcance != Lectura {
		t.Errorf("un permiso desconocido debe quedar en lectura: %+v %v", tk, e2)
	}
}

func TestAprobar_rechazaPeticionesDeOtroSitio(t *testing.T) {
	e := nuevoEntorno(t)
	e.sesion = "ana"
	cliente := e.registrar("Claude", callback)
	q := consulta(cliente, nil)
	cuerpo, _ := json.Marshal(map[string]any{"response_type": "code", "client_id": cliente, "redirect_uri": callback, "code_challenge": challenge,
		"code_challenge_method": "S256", "approve": true})

	// Un envío «simple» (text/plain) no pide permiso CORS: se rechaza aunque lleve JSON válido.
	if rec := e.llamar("POST", "/api/oauth/approve", string(cuerpo), "text/plain"); rec.Code != 415 || len(e.repo.codigos) != 0 {
		t.Errorf("text/plain: esperaba 415 y fue %d", rec.Code)
	}
	if rec := e.llamar("POST", "/api/oauth/approve", string(cuerpo), ""); rec.Code != 415 {
		t.Errorf("sin tipo: esperaba 415 y fue %d", rec.Code)
	}
	if rec := e.llamar("POST", "/api/oauth/approve", string(cuerpo), "application/x-www-form-urlencoded"); rec.Code != 415 {
		t.Errorf("formulario: esperaba 415 y fue %d", rec.Code)
	}

	// Lo que el navegador declara lanzado desde otro sitio, tampoco.
	req := httptest.NewRequest("POST", origen+"/api/oauth/approve", strings.NewReader(string(cuerpo)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	e.motor.ServeHTTP(rec, req)
	if rec.Code != 403 || len(e.repo.codigos) != 0 {
		t.Errorf("cross-site: esperaba 403 y fue %d", rec.Code)
	}

	// Desde el propio sitio (lo que hace la pantalla) sí.
	req = httptest.NewRequest("POST", origen+"/api/oauth/approve", strings.NewReader(string(cuerpo)))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec = httptest.NewRecorder()
	e.motor.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("same-origin: %d %s", rec.Code, rec.Body)
	}
	_ = q
}
