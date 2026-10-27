package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
const googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

// OAuthHandler conecta un buzón de Gmail del que poder enviar.
//
// Es un flujo distinto del de "entrar con Google", aunque use el mismo
// cliente de Google Cloud: aquí sí hace falta permiso duradero, porque hay
// que poder enviar correos más tarde sin el usuario delante.
type OAuthHandler struct {
	cfg        OAuthConfig
	cuentas    CuentaRepo
	sessions   *auth.Sessions
	produccion bool
	client     *http.Client
}

func NewOAuthHandler(cfg OAuthConfig, cuentas CuentaRepo, sessions *auth.Sessions, produccion bool) *OAuthHandler {
	return &OAuthHandler{
		cfg: cfg, cuentas: cuentas, sessions: sessions, produccion: produccion,
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (h *OAuthHandler) Register(g *gin.RouterGroup) {
	g.GET("/google/start", h.start)
	g.GET("/google/callback", h.callback)
}

func (h *OAuthHandler) start(c *gin.Context) {
	state, err := h.sessions.CreateState()
	if err != nil {
		h.volver(c, "error", "No se pudo iniciar la conexión con Google.")
		return
	}

	h.ponerState(c, state, auth.OAuthStateMaxAge)

	// access_type=offline y prompt=consent: sin forzar el consentimiento,
	// Google no devuelve refresh_token si el usuario ya autorizó antes, y
	// sin él no se podría enviar nada pasada una hora.
	p := url.Values{
		"client_id":              {h.cfg.ClientID},
		"redirect_uri":           {h.cfg.RedirectURI},
		"response_type":          {"code"},
		"scope":                  {GmailSendScope},
		"access_type":            {"offline"},
		"prompt":                 {"select_account consent"},
		"include_granted_scopes": {"true"},
		"state":                  {state},
	}
	c.Redirect(http.StatusFound, googleAuthURL+"?"+p.Encode())
}

func (h *OAuthHandler) callback(c *gin.Context) {
	defer h.ponerState(c, "", -1)

	if c.Query("error") != "" {
		h.volver(c, "error", "No se pudo conectar la cuenta de Google.")
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	cookie, _ := c.Cookie(auth.OAuthStateCookie)

	if code == "" || state == "" || state != cookie || !h.sessions.VerifyState(state) {
		h.volver(c, "error", "La conexión con Google caducó, inténtalo de nuevo.")
		return
	}

	res := h.conectar(c.Request.Context(), code)
	if !res.OK {
		h.volver(c, "error", res.Error)
		return
	}

	h.volver(c, "conectada", res.Value)
}

// conectar canjea el código, lee el perfil y guarda el buzón.
func (h *OAuthHandler) conectar(ctx context.Context, code string) kernel.Result[string] {
	datos := url.Values{
		"client_id":     {h.cfg.ClientID},
		"client_secret": {h.cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {h.cfg.RedirectURI},
	}

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := h.postJSON(ctx, googleTokenURL, datos, &tokens); err != nil {
		return kernel.Err[string]("No se pudo completar la conexión con Google.")
	}

	// Sin refresh token el buzón serviría una hora y luego dejaría de
	// funcionar sin explicación. Mejor decirlo ahora.
	if tokens.RefreshToken == "" {
		return kernel.Err[string](
			"Google no concedió permiso duradero. Quita el acceso del Home Lab en tu cuenta de Google y vuelve a conectarla.")
	}

	perfil, err := h.perfil(ctx, tokens.AccessToken)
	if err != nil {
		return kernel.Err[string]("No se pudo leer el perfil de Google.")
	}

	caduca := time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	cuenta := CuentaGoogle{
		Email:        strings.ToLower(strings.TrimSpace(perfil.Email)),
		RefreshToken: tokens.RefreshToken,
		AccessToken:  &tokens.AccessToken,
		ExpiresAt:    &caduca,
		Scope:        tokens.Scope,
	}
	if perfil.Name != "" {
		cuenta.Name = &perfil.Name
	}
	if perfil.Picture != "" {
		cuenta.Picture = &perfil.Picture
	}

	if _, err := h.cuentas.Upsert(ctx, cuenta); err != nil {
		return kernel.Err[string]("No se pudo guardar la cuenta.")
	}

	return kernel.Ok(cuenta.Email)
}

type perfilGoogle struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (h *OAuthHandler) perfil(ctx context.Context, accessToken string) (*perfilGoogle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	cuerpo, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return nil, err
	}

	var p perfilGoogle
	if err := json.Unmarshal(cuerpo, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (h *OAuthHandler) postJSON(ctx context.Context, destino string, datos url.Values, salida any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, strings.NewReader(datos.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	cuerpo, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &errorHTTP{estado: res.StatusCode}
	}

	return json.Unmarshal(cuerpo, salida)
}

type errorHTTP struct{ estado int }

func (e *errorHTTP) Error() string {
	return "google respondió " + http.StatusText(e.estado)
}

func (h *OAuthHandler) ponerState(c *gin.Context, valor string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.OAuthStateCookie, valor, maxAge, "/", "", h.produccion, true)
}

// volver devuelve a la pantalla de correo con el resultado. El origen lo
// resuelve auth, que ya sabe leer las cabeceras del proxy.
func (h *OAuthHandler) volver(c *gin.Context, clave, valor string) {
	c.Redirect(http.StatusFound,
		auth.OrigenPublico(c)+"/email-automation?"+url.Values{clave: {valor}}.Encode())
}
