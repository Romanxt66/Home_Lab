// Package scraper baja una página y saca de ella lo que pida un selector CSS.
package scraper

import (
	"net/url"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// LimiteCrudo es cuánto del cuerpo se devuelve cuando no hay selector.
// Sin tope, una página grande llenaría la respuesta de ruido.
const LimiteCrudo = 20_000

// MaxElementos acota cuántas coincidencias se devuelven.
const MaxElementos = 1000

// ValidarURL comprueba y normaliza una dirección http(s).
//
// Se rechaza cualquier otro esquema a propósito: file:// leería archivos del
// servidor y los esquemas raros no tienen sentido aquí.
func ValidarURL(crudo string) kernel.Result[string] {
	crudo = strings.TrimSpace(crudo)
	if crudo == "" {
		return kernel.Err[string]("Escribe una URL.")
	}

	u, err := url.Parse(crudo)
	if err != nil {
		return kernel.Err[string]("La URL no es válida.")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return kernel.Err[string]("Solo se admiten direcciones http y https.")
	}
	if u.Host == "" {
		return kernel.Err[string]("La URL no tiene servidor.")
	}

	return kernel.Ok(u.String())
}

// Normalizar deja el texto de un elemento en una sola línea: los saltos y la
// sangría del HTML son presentación, no contenido.
func Normalizar(texto string) string {
	return strings.Join(strings.Fields(texto), " ")
}

// Recortar corta por caracteres, no por bytes, para no partir una letra
// acentuada por la mitad.
func Recortar(s string, limite int) string {
	runas := []rune(s)
	if len(runas) <= limite {
		return s
	}
	return string(runas[:limite])
}
