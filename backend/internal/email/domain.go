// Package email manda correos, con plantillas de variables y, opcionalmente,
// desde una cuenta de Gmail conectada en vez del SMTP del sistema.
package email

import (
	"regexp"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Mensaje es un correo ya listo para salir.
type Mensaje struct {
	Para   []string
	Asunto string
	Cuerpo string
}

// Plantilla es un asunto y un cuerpo con huecos {{variable}}.
type Plantilla struct {
	ID        string
	Name      string
	Subject   string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CuentaGoogle es un buzón conectado por OAuth desde el que se puede enviar.
type CuentaGoogle struct {
	ID           string
	Email        string
	Name         *string
	Picture      *string
	RefreshToken string
	AccessToken  *string
	ExpiresAt    *time.Time
	Scope        string
}

// Permiso que hace falta para enviar como el usuario.
const GmailSendScope = "openid email profile https://www.googleapis.com/auth/gmail.send"

// Comprobación pragmática, no el RFC entero: atrapa los errores evidentes
// sin rechazar direcciones raras pero legítimas.
var reCorreo = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func CorreoValido(v string) bool {
	return reCorreo.MatchString(strings.TrimSpace(v))
}

// SepararDestinatarios parte el texto que escribe el usuario, que puede usar
// comas, puntos y comas o saltos de línea, y aparta los que no valen para
// poder decirle cuáles son.
func SepararDestinatarios(crudo string) (validos, invalidos []string) {
	partes := strings.FieldsFunc(crudo, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})

	vistos := make(map[string]bool)
	for _, p := range partes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !CorreoValido(p) {
			invalidos = append(invalidos, p)
			continue
		}
		// Sin repetidos: escribir dos veces la misma dirección no debe
		// mandar el correo dos veces.
		if !vistos[p] {
			vistos[p] = true
			validos = append(validos, p)
		}
	}

	return validos, invalidos
}

// ValidarMensaje comprueba el correo antes de intentar enviarlo.
func ValidarMensaje(m Mensaje) kernel.Result[Mensaje] {
	if len(m.Para) == 0 {
		return kernel.Err[Mensaje]("Añade al menos un destinatario válido.")
	}

	var malos []string
	for _, d := range m.Para {
		if !CorreoValido(d) {
			malos = append(malos, d)
		}
	}
	if len(malos) > 0 {
		return kernel.Err[Mensaje]("Estos destinatarios no son válidos: " + strings.Join(malos, ", "))
	}

	if strings.TrimSpace(m.Asunto) == "" {
		return kernel.Err[Mensaje]("El asunto no puede estar vacío.")
	}
	if strings.TrimSpace(m.Cuerpo) == "" {
		return kernel.Err[Mensaje]("El cuerpo no puede estar vacío.")
	}

	return kernel.Ok(m)
}

// ── Plantillas ─────────────────────────────────────────────────────────────

var reVariable = regexp.MustCompile(`\{\{\s*([\w.]+)\s*\}\}`)

// Variables lista los huecos que usa una plantilla, sin repetir.
func Variables(texto string) []string {
	var nombres []string
	vistos := make(map[string]bool)

	for _, m := range reVariable.FindAllStringSubmatch(texto, -1) {
		if !vistos[m[1]] {
			vistos[m[1]] = true
			nombres = append(nombres, m[1])
		}
	}

	return nombres
}

// Rellenar sustituye los huecos por sus valores.
//
// Una variable sin valor se queda vacía, no como {{nombre}}: es preferible
// una frase a la que le falta algo que enseñarle al destinatario las tripas
// de la plantilla.
func Rellenar(texto string, valores map[string]string) string {
	return reVariable.ReplaceAllStringFunc(texto, func(hueco string) string {
		nombre := reVariable.FindStringSubmatch(hueco)[1]
		return valores[nombre]
	})
}
