package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Extremos de Google. Son constantes públicas y documentadas por Google.
const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

	// Para entrar basta con saber quién es: ni Gmail ni nada más. Es el
	// permiso mínimo que resuelve el caso, y pedir de más sería gratuito.
	GoogleLoginScope = "openid email profile"
)

// GoogleConfig son las credenciales de la aplicación en Google Cloud.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configurado dice si se puede ofrecer el botón de Google.
func (c GoogleConfig) Configurado() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURI != ""
}

// GoogleClient habla con Google. Se le pasa el http.Client para poder
// sustituirlo en las pruebas sin salir a internet.
type GoogleClient struct {
	cfg    GoogleConfig
	client *http.Client
}

func NewGoogleClient(cfg GoogleConfig) *GoogleClient {
	return &GoogleClient{
		cfg: cfg,
		// Con tiempo límite: sin él, una petición a Google que se quede
		// colgada bloquearía la goroutine para siempre.
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// AuthURL es la dirección a la que se manda al usuario para que Google le
// pida permiso.
//
// access_type=online y sin forzar consentimiento: para entrar no hace falta
// refresh token ni volver a preguntar cada vez. prompt=select_account sí se
// deja, para que quien tenga varias cuentas pueda elegir.
func (g *GoogleClient) AuthURL(state string) string {
	p := url.Values{
		"client_id":              {g.cfg.ClientID},
		"redirect_uri":           {g.cfg.RedirectURI},
		"response_type":          {"code"},
		"scope":                  {GoogleLoginScope},
		"access_type":            {"online"},
		"prompt":                 {"select_account"},
		"include_granted_scopes": {"true"},
		"state":                  {state},
	}
	return googleAuthURL + "?" + p.Encode()
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// GoogleProfile es lo que Google cuenta del usuario.
type GoogleProfile struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	EmailVerified bool   `json:"email_verified"`
}

// Exchange canjea el código de la vuelta por un token de acceso.
func (g *GoogleClient) Exchange(ctx context.Context, code string) (string, error) {
	cuerpo := url.Values{
		"client_id":     {g.cfg.ClientID},
		"client_secret": {g.cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {g.cfg.RedirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(cuerpo.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var token tokenResponse
	if err := g.pedir(req, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("google no devolvió token de acceso")
	}
	return token.AccessToken, nil
}

// UserInfo lee el perfil con el token recién obtenido.
func (g *GoogleClient) UserInfo(ctx context.Context, accessToken string) (*GoogleProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	var perfil GoogleProfile
	if err := g.pedir(req, &perfil); err != nil {
		return nil, err
	}
	return &perfil, nil
}

// pedir ejecuta la petición y vuelca el JSON. El detalle del fallo se
// devuelve recortado: entero podría arrastrar el token a los registros.
func (g *GoogleClient) pedir(req *http.Request, destino any) error {
	res, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	cuerpo, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detalle := string(cuerpo)
		if len(detalle) > 200 {
			detalle = detalle[:200]
		}
		return fmt.Errorf("google respondió %d: %s", res.StatusCode, detalle)
	}

	return json.Unmarshal(cuerpo, destino)
}
