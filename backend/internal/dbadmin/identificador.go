// Package dbadmin permite mirar y editar bases de datos PostgreSQL externas.
package dbadmin

import (
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Citado de identificadores de SQL.
//
// Los VALORES de una consulta se pueden parametrizar ($1, $2…), pero los
// IDENTIFICADORES —esquema, tabla, columna— no: hay que interpolarlos en el
// texto. Hacerlo a lo bruto es un agujero de inyección de SQL, así que todo
// identificador que llegue a una sentencia pasa antes por aquí.
//
// La regla de PostgreSQL: entre comillas dobles, duplicando las que lleve
// dentro. Un byte nulo trunca la sentencia en el servidor, así que se
// rechaza en vez de intentar escaparlo.

// MaxBytesIdentificador es el límite de PostgreSQL. Pasarse no da error:
// trunca en silencio, y entonces la sentencia apuntaría a otro objeto.
const MaxBytesIdentificador = 63

// Citar devuelve el identificador listo para meter en una sentencia.
// Solo debe llamarse con algo que haya pasado por ValidarIdentificador.
func Citar(nombre string) string {
	return `"` + strings.ReplaceAll(nombre, `"`, `""`) + `"`
}

// CitarCualificado devuelve "esquema"."tabla".
func CitarCualificado(esquema, tabla string) string {
	return Citar(esquema) + "." + Citar(tabla)
}

// ValidarIdentificador comprueba el nombre antes de usarlo para construir
// una sentencia.
func ValidarIdentificador(nombre, etiqueta string) kernel.Result[string] {
	limpio := strings.TrimSpace(nombre)

	if limpio == "" {
		return kernel.Err[string]("El " + etiqueta + " no puede estar vacío.")
	}
	if strings.ContainsRune(limpio, 0) {
		return kernel.Err[string]("El " + etiqueta + " lleva caracteres que no se admiten.")
	}
	if len(limpio) > MaxBytesIdentificador {
		return kernel.Err[string]("El " + etiqueta + " pasa de los 63 bytes que admite PostgreSQL.")
	}

	return kernel.Ok(limpio)
}
