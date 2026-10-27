package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const (
	gmailEnvioURL  = "https://gmail.googleapis.com/gmail/v1/users/me/messages/send"
	googleTokenURL = "https://oauth2.googleapis.com/token"

	// Margen antes de dar por caducado el token: si faltara un segundo, la
	// petición podría llegar ya expirada.
	margenToken = time.Minute
)

// OAuthConfig son las credenciales de la aplicación en Google Cloud.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func (c OAuthConfig) Configurado() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURI != ""
}

// GmailEnviador manda desde un buzón conectado usando la API de Gmail.
//
// No se usa el SMTP de Gmail a propósito: con un token del permiso
// gmail.send lo rechaza con un error genérico de usuario y contraseña,
// porque para SMTP quiere el permiso ancho de https://mail.google.com/.
// Por la API funciona con el permiso mínimo.
type GmailEnviador struct {
	cfg     OAuthConfig
	cuentas CuentaRepo
	client  *http.Client
}

func NewGmailEnviador(cfg OAuthConfig, cuentas CuentaRepo) *GmailEnviador {
	return &GmailEnviador{
		cfg:     cfg,
		cuentas: cuentas,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *GmailEnviador) EnviarDesde(ctx context.Context, cuentaID string, m Mensaje) kernel.Result[struct{}] {
	if !g.cfg.Configurado() {
		return kernel.Err[struct{}]("Google OAuth no está configurado en el servidor.")
	}

	cuenta, err := g.cuentas.Get(ctx, cuentaID)
	if err != nil {
		return kernel.Err[struct{}]("No se pudo leer la cuenta de Google.")
	}
	if cuenta == nil {
		return kernel.Err[struct{}]("Esa cuenta de Google ya no está conectada.")
	}
	if cuenta.RefreshToken == "" {
		return kernel.Err[struct{}](
			"La cuenta no tiene permiso duradero. Desconéctala y vuelve a conectarla.")
	}

	token := g.tokenVigente(ctx, cuenta)
	if !token.OK {
		return kernel.Err[struct{}](token.Error)
	}

	crudo := base64.RawURLEncoding.EncodeToString(Componer(cuenta.Email, m))
	cuerpo, _ := json.Marshal(map[string]string{"raw": crudo})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gmailEnvioURL, bytes.NewReader(cuerpo))
	if err != nil {
		return kernel.Err[struct{}]("No se pudo preparar el envío.")
	}
	req.Header.Set("Authorization", "Bearer "+token.Value)
	req.Header.Set("Content-Type", "application/json")

	res, err := g.client.Do(req)
	if err != nil {
		return kernel.Err[struct{}]("No se pudo conectar con Gmail.")
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detalle, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		return kernel.Err[struct{}](explicarGmail(res.StatusCode, detalle))
	}

	return kernel.Ok(struct{}{})
}

// tokenVigente devuelve un token de acceso usable, renovándolo si hace falta
// y guardando el nuevo para no pedir uno en cada envío.
func (g *GmailEnviador) tokenVigente(ctx context.Context, c *CuentaGoogle) kernel.Result[string] {
	vigente := c.AccessToken != nil && *c.AccessToken != "" &&
		c.ExpiresAt != nil && time.Until(*c.ExpiresAt) > margenToken

	if vigente {
		return kernel.Ok(*c.AccessToken)
	}

	nuevo, caduca, err := g.renovar(ctx, c.RefreshToken)
	if err != nil {
		// invalid_grant quiere decir que el usuario revocó el permiso o que
		// caducó: no es un fallo pasajero, hay que reconectar.
		if strings.Contains(strings.ToLower(err.Error()), "invalid_grant") {
			return kernel.Err[string](
				"La autorización caducó o se revocó. Desconecta la cuenta y vuelve a conectarla.")
		}
		return kernel.Err[string]("No se pudo renovar el permiso de Google.")
	}

	c.AccessToken = &nuevo
	c.ExpiresAt = &caduca
	if _, err := g.cuentas.Upsert(ctx, *c); err != nil {
		// Guardar el token es una optimización: si falla, el envío sigue.
		// Solo significa que la próxima vez habrá que renovarlo otra vez.
		_ = err
	}

	return kernel.Ok(nuevo)
}

func (g *GmailEnviador) renovar(ctx context.Context, refresh string) (string, time.Time, error) {
	datos := url.Values{
		"client_id":     {g.cfg.ClientID},
		"client_secret": {g.cfg.ClientSecret},
		"refresh_token": {refresh},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(datos.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := g.client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer res.Body.Close()

	cuerpo, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("google respondió %d: %s", res.StatusCode, recortar(cuerpo))
	}

	var salida struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(cuerpo, &salida); err != nil {
		return "", time.Time{}, err
	}
	if salida.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("google no devolvió token")
	}

	return salida.AccessToken, time.Now().Add(time.Duration(salida.ExpiresIn) * time.Second), nil
}

func explicarGmail(estado int, cuerpo []byte) string {
	switch estado {
	case http.StatusUnauthorized:
		return "Gmail rechazó el permiso. Desconecta la cuenta y vuelve a conectarla."
	case http.StatusForbidden:
		return "La cuenta no tiene permiso para enviar. Revisa los permisos concedidos."
	case http.StatusTooManyRequests:
		return "Gmail está limitando los envíos. Inténtalo dentro de un rato."
	}

	var detalle struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(cuerpo, &detalle); err == nil && detalle.Error.Message != "" {
		return "Gmail: " + detalle.Error.Message
	}
	return fmt.Sprintf("Gmail devolvió un error (HTTP %d).", estado)
}

func recortar(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
