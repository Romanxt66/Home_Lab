package oauth

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

// RutaMCP es el recurso protegido: el token de acceso es para este endpoint.
const RutaMCP = "/mcp"

// Handler expone el servidor de autorización.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegistrarPublico monta lo que no lleva sesión: metadatos y endpoints de OAuth.
func (h *Handler) RegistrarPublico(r gin.IRoutes) {
	r.GET("/.well-known/oauth-protected-resource", h.recursoProtegido)
	r.GET("/.well-known/oauth-protected-resource"+RutaMCP, h.recursoProtegido)
	r.GET("/.well-known/oauth-authorization-server", h.metadatos)

	r.POST("/oauth/register", httpx.LimitarPorIP(20, time.Hour), h.registrar)
	r.GET("/oauth/authorize", httpx.LimitarPorIP(120, time.Minute), h.autorizar)
	r.POST("/oauth/token", httpx.LimitarPorIP(120, time.Minute), h.token)
	r.POST("/oauth/revoke", httpx.LimitarPorIP(60, time.Minute), h.revocar)
}

// RegistrarSesion monta lo que usa la pantalla de consentimiento (cookie de sesión).
func (h *Handler) RegistrarSesion(g *gin.RouterGroup) {
	g.GET("/request", h.peticion)
	g.POST("/approve", h.aprobar)
	g.GET("/grants", h.concesiones)
	g.DELETE("/grants/:id", h.revocarConcesion)
}

// MetadatosRecurso es la dirección de los metadatos del recurso, para la
// cabecera WWW-Authenticate del MCP (RFC 9728).
func MetadatosRecurso(c *gin.Context) string {
	return auth.OrigenPublico(c) + "/.well-known/oauth-protected-resource" + RutaMCP
}

// ── Metadatos ──────────────────────────────────────────────────────────────

func (h *Handler) recursoProtegido(c *gin.Context) {
	origen := auth.OrigenPublico(c)
	c.JSON(http.StatusOK, gin.H{
		"resource":                 origen + RutaMCP,
		"authorization_servers":    []string{origen},
		"scopes_supported":         []string{ScopeLectura, ScopeEscritura},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Home Lab",
	})
}

func (h *Handler) metadatos(c *gin.Context) {
	o := auth.OrigenPublico(c)
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                         o,
		"authorization_endpoint":                         o + "/oauth/authorize",
		"token_endpoint":                                 o + "/oauth/token",
		"registration_endpoint":                          o + "/oauth/register",
		"revocation_endpoint":                            o + "/oauth/revoke",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"scopes_supported":                               []string{ScopeLectura, ScopeEscritura},
		"authorization_response_iss_parameter_supported": true,
	})
}

// ── Errores ────────────────────────────────────────────────────────────────

func sinCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func responderError(c *gin.Context, e *ErrorOAuth) {
	sinCache(c)
	if e.Estado == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", `Bearer realm="homelab"`)
	}
	c.JSON(e.Estado, gin.H{"error": e.Codigo, "error_description": e.Descripcion})
}

// ── Registro ───────────────────────────────────────────────────────────────

func (h *Handler) registrar(c *gin.Context) {
	var cuerpo struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
		GrantTypes   []string `json:"grant_types"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		responderError(c, nuevoError("invalid_client_metadata", "El cuerpo debe ser JSON con client_name y redirect_uris."))
		return
	}
	for _, g := range cuerpo.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			responderError(c, nuevoError("invalid_client_metadata", "Solo se admiten los grant_types authorization_code y refresh_token."))
			return
		}
	}
	cl, e := h.svc.Registrar(c.Request.Context(), RegistroEntrada{Nombre: cuerpo.ClientName, RedirectURIs: cuerpo.RedirectURIs})
	if e != nil {
		responderError(c, e)
		return
	}
	sinCache(c)
	c.JSON(http.StatusCreated, gin.H{
		"client_id":                  cl.ID,
		"client_id_issued_at":        cl.CreadoEn.Unix(),
		"client_name":                cl.Nombre,
		"redirect_uris":              cl.RedirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none", // cliente público: la prueba es PKCE
	})
}

// ── Autorización ───────────────────────────────────────────────────────────

func peticionDe(get func(string) string) Peticion {
	return Peticion{
		TipoRespuesta: get("response_type"),
		ClienteID:     get("client_id"),
		RedirectURI:   get("redirect_uri"),
		Estado:        get("state"),
		Challenge:     get("code_challenge"),
		Metodo:        get("code_challenge_method"),
		Scope:         get("scope"),
		Recurso:       get("resource"),
	}
}

// autorizar valida la petición y manda a la persona a la pantalla de
// consentimiento de la interfaz (que exige haber entrado). No concede nada.
func (h *Handler) autorizar(c *gin.Context) {
	p := peticionDe(c.Query)
	_, e := h.svc.ValidarPeticion(c.Request.Context(), p, auth.OrigenPublico(c)+RutaMCP)
	if e != nil {
		if e.NoRedirigir {
			paginaError(c, e)
			return
		}
		// El resto de fallos se devuelven al cliente, que es quien puede corregirlos.
		c.Redirect(http.StatusFound, redireccion(p.RedirectURI, map[string]string{
			"error": e.Codigo, "error_description": e.Descripcion, "state": p.Estado, "iss": auth.OrigenPublico(c),
		}))
		return
	}
	q := url.Values{}
	for _, k := range []string{"response_type", "client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method", "scope", "resource"} {
		if v := c.Query(k); v != "" {
			q.Set(k, v)
		}
	}
	sinCache(c)
	c.Redirect(http.StatusFound, "/oauth/autorizar?"+q.Encode())
}

// paginaError enseña el fallo en pantalla en vez de redirigir a una dirección
// que no se ha podido comprobar. Todo lo que se pinta va escapado.
func paginaError(c *gin.Context, e *ErrorOAuth) {
	sinCache(c)
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	c.Data(e.Estado, "text/html; charset=utf-8", []byte(`<!doctype html><html lang="es"><meta charset="utf-8"><title>No se pudo autorizar</title>
<body style="font:16px system-ui;max-width:32rem;margin:4rem auto;padding:0 1rem"><h1>No se pudo autorizar</h1>
<p>`+html.EscapeString(e.Descripcion)+`</p><p>Vuelve a la aplicación e inténtalo de nuevo.</p></body></html>`))
}

type consentimientoDTO struct {
	ClientName     string `json:"clientName"`
	RedirectHost   string `json:"redirectHost"`
	Loopback       bool   `json:"loopback"`
	RequestedWrite bool   `json:"requestedWrite"`
}

// peticion le dice a la pantalla de consentimiento quién pide acceso.
func (h *Handler) peticion(c *gin.Context) {
	if _, ok := httpx.CurrentUser(c); !ok {
		httpx.Unauthorized(c)
		return
	}
	p := peticionDe(c.Query)
	cl, e := h.svc.ValidarPeticion(c.Request.Context(), p, auth.OrigenPublico(c)+RutaMCP)
	if e != nil {
		httpx.BadRequest(c, e.Descripcion)
		return
	}
	u, _ := url.Parse(p.RedirectURI)
	_, escritura := ParseScope(p.Scope)
	c.JSON(http.StatusOK, consentimientoDTO{
		ClientName: cl.Nombre, RedirectHost: u.Host, Loopback: esLoopback(u.Hostname()), RequestedWrite: escritura,
	})
}

// aprobar recibe la decisión de la persona. Va con la cookie de sesión y JSON:
// una página ajena no puede lanzarlo (el navegador pediría permiso CORS antes).
func (h *Handler) aprobar(c *gin.Context) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}
	// Defensa en profundidad además de SameSite=Lax: solo JSON, y nada que el
	// navegador marque como lanzado desde otro sitio. Una página ajena podría
	// intentar un envío «simple» (text/plain) que no pide permiso CORS.
	if c.ContentType() != "application/json" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "La petición debe ser application/json."})
		return
	}
	if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Petición de otro sitio rechazada."})
		return
	}
	var cuerpo struct {
		ResponseType        string `json:"response_type"`
		ClientID            string `json:"client_id"`
		RedirectURI         string `json:"redirect_uri"`
		State               string `json:"state"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
		Scope               string `json:"scope"`
		Resource            string `json:"resource"`
		Approve             bool   `json:"approve"`
		Permission          string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}
	p := Peticion{
		TipoRespuesta: cuerpo.ResponseType, ClienteID: cuerpo.ClientID, RedirectURI: cuerpo.RedirectURI, Estado: cuerpo.State,
		Challenge: cuerpo.CodeChallenge, Metodo: cuerpo.CodeChallengeMethod, Scope: cuerpo.Scope, Recurso: cuerpo.Resource,
	}
	permiso := Lectura
	if cuerpo.Permission == "write" {
		permiso = Escritura
	}
	destino, e := h.svc.Aprobar(c.Request.Context(), u.UID, p, auth.OrigenPublico(c)+RutaMCP, cuerpo.Approve, permiso, auth.OrigenPublico(c))
	if e != nil {
		if e.Estado >= 500 {
			_ = c.Error(e)
			httpx.ServerError(c, e)
			return
		}
		httpx.BadRequest(c, e.Descripcion)
		return
	}
	sinCache(c)
	c.JSON(http.StatusOK, gin.H{"redirect": destino})
}

// ── Token ──────────────────────────────────────────────────────────────────

func (h *Handler) token(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.Request.ParseForm(); err != nil {
		responderError(c, nuevoError("invalid_request", "El cuerpo debe ser application/x-www-form-urlencoded."))
		return
	}
	// Solo se mira el cuerpo, no la URL: un secreto en la query acabaría en los registros.
	f := c.Request.PostForm
	tk, e := h.svc.Canjear(c.Request.Context(), TokenEntrada{
		GrantType: f.Get("grant_type"), Codigo: f.Get("code"), RedirectURI: f.Get("redirect_uri"),
		ClienteID: f.Get("client_id"), Verifier: f.Get("code_verifier"), Refresco: f.Get("refresh_token"),
		Recurso: f.Get("resource"),
	}, auth.OrigenPublico(c)+RutaMCP)
	if e != nil {
		if e.Estado >= 500 {
			_ = c.Error(e)
		}
		responderError(c, e)
		return
	}
	sinCache(c)
	c.JSON(http.StatusOK, gin.H{
		"access_token": tk.Acceso, "token_type": "Bearer", "expires_in": tk.ExpiraEn,
		"refresh_token": tk.Refresco, "scope": ScopeDe(tk.Alcance),
	})
}

func (h *Handler) revocar(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.Request.ParseForm(); err == nil {
		f := c.Request.PostForm
		_ = h.svc.RevocarToken(c.Request.Context(), strings.TrimSpace(f.Get("token")), f.Get("client_id"))
	}
	// RFC 7009: siempre 200, haya existido el token o no.
	sinCache(c)
	c.Status(http.StatusOK)
}

// ── Concesiones de la persona ──────────────────────────────────────────────

type concesionDTO struct {
	ID         string  `json:"id"`
	ClientName string  `json:"clientName"`
	Scope      string  `json:"scope"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
}

func (h *Handler) concesiones(c *gin.Context) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}
	lista, err := h.svc.Concesiones(c.Request.Context(), u.UID)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	salida := make([]concesionDTO, 0, len(lista))
	for _, g := range lista {
		d := concesionDTO{ID: g.ID, ClientName: g.ClienteNombre, Scope: string(g.Alcance), CreatedAt: g.CreadaEn.UTC().Format(iso)}
		if g.UsadaEn != nil {
			s := g.UsadaEn.UTC().Format(iso)
			d.LastUsedAt = &s
		}
		salida = append(salida, d)
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) revocarConcesion(c *gin.Context) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}
	hecho, err := h.svc.RevocarConcesion(c.Request.Context(), u.UID, c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !hecho {
		httpx.NotFound(c, "No se encontró esa conexión.")
		return
	}
	c.Status(http.StatusNoContent)
}
