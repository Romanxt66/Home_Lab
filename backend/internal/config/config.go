// Package config lee la configuración del entorno una sola vez al arrancar.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Cadena de conexión a PostgreSQL. Apunta al esquema "lab", el mismo que
	// usa el Lab actual: no se migra ningún dato, se leen las mismas tablas.
	DatabaseURL string

	// Clave con la que se firman las sesiones. Debe ser la MISMA que usa el
	// Lab en Next, así las sesiones ya emitidas siguen siendo válidas.
	SessionSecret string

	Port string

	// Orígenes autorizados para CORS. El frontend corre en otro puerto, así
	// que sin esto el navegador bloquea las peticiones.
	AllowedOrigins []string

	// En producción la cookie viaja solo por HTTPS.
	Production bool

	// Credenciales de Google para "entrar con Google". Son opcionales: sin
	// ellas la aplicación funciona igual, solo que no se ofrece el botón.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	// Dirección de vuelta para conectar un buzón de Gmail. Es distinta de la
	// de entrar: Google exige que cada una esté dada de alta tal cual.
	GmailRedirectURI string

	// Servidor de correo del sistema. Opcional: sin él solo se puede enviar
	// desde un buzón de Gmail conectado.
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

// Load lee el entorno y falla pronto si falta algo imprescindible. Es mejor no
// arrancar que arrancar a medias y fallar en la primera petición.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		Port:          valueOr(os.Getenv("PORT"), "8080"),
		Production:    os.Getenv("APP_ENV") == "production",

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:  os.Getenv("GOOGLE_LOGIN_REDIRECT_URI"),
		GmailRedirectURI:   os.Getenv("GOOGLE_REDIRECT_URI"),

		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: IntOr("SMTP_PORT", 587),
		SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPass: os.Getenv("SMTP_PASS"),
		SMTPFrom: os.Getenv("SMTP_FROM"),
	}

	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("falta DATABASE_URL")
	}
	if cfg.SessionSecret == "" {
		if cfg.Production {
			return cfg, fmt.Errorf("falta SESSION_SECRET")
		}
		// El mismo valor por defecto que el Lab en desarrollo, para que las
		// dos aplicaciones se reconozcan la sesión mientras conviven.
		cfg.SessionSecret = "dev-insecure-session-secret-change-me"
	}

	origins := valueOr(os.Getenv("ALLOWED_ORIGINS"), "http://localhost:5173")
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}

	return cfg, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// IntOr lee un entero del entorno con valor por defecto.
func IntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
