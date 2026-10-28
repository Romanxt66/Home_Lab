package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// GoogleHandler atiende las dos patas del flujo: la ida a Google y la vuelta.
type GoogleHandler struct {
	google     *GoogleClient
	caso       *GoogleLogin
	sessions   *Sessions
	produccion bool
}

func NewGoogleHandler(google *GoogleClient, caso *GoogleLogin, sessions *Sessions, produccion bool) *GoogleHandler {
	return &GoogleHandler{google: google, caso: caso, sessions: sessions, produccion: produccion}
}

func (h *GoogleHandler) Register(g *gin.RouterGroup) {
	g.GET("/google/login/start", h.start)
	g.GET("/google/login/callback", h.callback)
}

// start manda al usuario a la pantalla de consentimiento de Google, dejando
// antes el state en una cookie para poder comprobarlo en la vuelta.
func (h *GoogleHandler) start(c *gin.Context) {
	state, err := h.sessions.CreateState()
	if err != nil {
		h.volverAlAcceso(c, "error", "No se pudo iniciar el acceso con Google.")
		return
	}

	h.ponerCookieState(c, state, OAuthStateMaxAge)
	c.Redirect(http.StatusFound, h.google.AuthURL(state))
}

// callback es la dirección a la que Google devuelve al usuario.
func (h *GoogleHandler) callback(c *gin.Context) {
	defer h.ponerCookieState(c, "", -1) // el state es de un solo uso

	if c.Query("error") != "" {
		h.volverAlAcceso(c, "error", "No se pudo continuar con Google.")
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	cookie, _ := c.Cookie(OAuthStateCookie)

	// Tres comprobaciones, no una: que venga state, que sea el mismo que
	// dejamos en la cookie, y que lleve nuestra firma.
	if code == "" || state == "" || state != cookie || !h.sessions.VerifyState(state) {
		h.volverAlAcceso(c, "error", "La sesión con Google caducó, inténtalo de nuevo.")
		return
	}

	ctx := c.Request.Context()

	token, err := h.google.Exchange(ctx, code)
	if err != nil {
		_ = c.Error(err)
		h.volverAlAcceso(c, "error", "No se pudo completar el acceso con Google.")
		return
	}

	perfil, err := h.google.UserInfo(ctx, token)
	if err != nil {
		_ = c.Error(err)
		h.volverAlAcceso(c, "error", "No se pudo leer tu perfil de Google.")
		return
	}

	res, err := h.caso.Ejecutar(ctx, *perfil)
	if err != nil {
		_ = c.Error(err)
		h.volverAlAcceso(c, "error", "Ocurrió un error inesperado.")
		return
	}
	if !res.OK {
		h.volverAlAcceso(c, "error", res.Error)
		return
	}

	switch res.Value.Tipo {
	case ResultadoPendiente:
		estado := "pending"
		if res.Value.Nuevo {
			estado = "registered"
		}
		h.volverAlAcceso(c, "estado", estado)

	case ResultadoRechazado:
		h.volverAlAcceso(c, "estado", "rejected")

	default:
		u := res.Value.User
		sesion, err := h.sessions.Create(u.ID, u.Email, u.Name, u.Role)
		if err != nil {
			_ = c.Error(err)
			h.volverAlAcceso(c, "error", "No se pudo abrir la sesión.")
			return
		}
		h.ponerCookieSesion(c, sesion)
		c.Redirect(http.StatusFound, OrigenPublico(c)+"/")
	}
}

// volverAlAcceso devuelve al usuario a la pantalla de acceso con un mensaje.
// Esperar aprobación no es un error, por eso viaja como "estado" y no como
// "error": la interfaz lo cuenta con otro tono.
func (h *GoogleHandler) volverAlAcceso(c *gin.Context, clave, valor string) {
	destino := OrigenPublico(c) + "/acceso?" + url.Values{clave: {valor}}.Encode()
	c.Redirect(http.StatusFound, destino)
}

func (h *GoogleHandler) ponerCookieState(c *gin.Context, valor string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(OAuthStateCookie, valor, maxAge, "/", "", h.produccion, true)
}

func (h *GoogleHandler) ponerCookieSesion(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, token, SessionMaxAge, "/", "", h.produccion, true)
}

// OrigenPublico reconstruye la dirección real del sitio.
//
// Detrás del proxy de Coolify, el host con el que llega la petición es el
// interno del contenedor (localhost:8080); construir la redirección con él
// sacaría al usuario fuera del sitio. El proxy nos dice el host de verdad
// en las cabeceras X-Forwarded-*, así que se miran primero.
func OrigenPublico(c *gin.Context) string {
	esquema := primero(c.GetHeader("X-Forwarded-Proto"), "http")
	if c.Request.TLS != nil {
		esquema = "https"
	}
	// Puede venir una lista, "https,http": vale la primera.
	if coma := strings.IndexByte(esquema, ','); coma > 0 {
		esquema = esquema[:coma]
	}

	host := primero(c.GetHeader("X-Forwarded-Host"), c.Request.Host)
	if coma := strings.IndexByte(host, ','); coma > 0 {
		host = host[:coma]
	}

	return esquema + "://" + strings.TrimSpace(host)
}

func primero(valores ...string) string {
	for _, v := range valores {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
