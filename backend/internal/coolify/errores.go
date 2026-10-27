package coolify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// cuerpoDeError es lo que devuelve Coolify cuando algo falla.
//
// Coolify corre sobre Laravel, así que un 422 llega como
// {"message": "Validation failed.", "errors": {"campo": ["motivo"]}}.
// El message suelto no dice nada útil: lo accionable está en el saco de
// errores por campo, y por eso se aplanan los dos en una línea.
type cuerpoDeError struct {
	Message string                     `json:"message"`
	Error   string                     `json:"error"`
	Errors  map[string]json.RawMessage `json:"errors"`
}

// ExplicarError arma un mensaje legible a partir de una respuesta fallida.
func ExplicarError(estado int, crudo []byte) string {
	switch estado {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "El token no vale o no tiene permisos. Revísalo en Coolify → Keys & Tokens."
	case http.StatusNotFound:
		return "Coolify no encuentra eso. ¿Sigue existiendo el recurso?"
	}

	var cuerpo cuerpoDeError
	if err := json.Unmarshal(crudo, &cuerpo); err != nil {
		return fmt.Sprintf("Coolify respondió con un error (HTTP %d).", estado)
	}

	titular := cuerpo.Message
	if titular == "" {
		titular = cuerpo.Error
	}
	if titular == "" {
		titular = fmt.Sprintf("HTTP %d", estado)
	}

	campos := aplanarCampos(cuerpo.Errors)
	if campos == "" {
		return "Coolify: " + titular
	}
	return "Coolify: " + titular + " — " + campos
}

// aplanarCampos junta los errores por campo en una sola línea. Cada campo
// puede traer una lista de motivos o un texto suelto, según el caso.
func aplanarCampos(errores map[string]json.RawMessage) string {
	if len(errores) == 0 {
		return ""
	}

	// En orden alfabético: un mapa de Go no garantiza el orden y el mensaje
	// cambiaría de una llamada a otra.
	campos := make([]string, 0, len(errores))
	for campo := range errores {
		campos = append(campos, campo)
	}
	sort.Strings(campos)

	partes := make([]string, 0, len(campos))
	for _, campo := range campos {
		partes = append(partes, campo+": "+motivosDe(errores[campo]))
	}

	return strings.Join(partes, " · ")
}

func motivosDe(crudo json.RawMessage) string {
	var lista []string
	if err := json.Unmarshal(crudo, &lista); err == nil {
		return strings.Join(lista, " ")
	}

	var texto string
	if err := json.Unmarshal(crudo, &texto); err == nil {
		return texto
	}

	return string(crudo)
}
