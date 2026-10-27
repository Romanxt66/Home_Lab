package dbadmin

import (
	"regexp"
	"strings"
)

// Análisis estático de una sentencia SQL, a ojo.
//
// No es un analizador de verdad: sirve para que la interfaz pida
// confirmación antes de lanzar algo destructivo. La seguridad real viene de
// los permisos del usuario en la propia base de datos, no de esto.

type Riesgo string

const (
	Inofensiva  Riesgo = "safe"
	Modifica    Riesgo = "modifying"
	Destructiva Riesgo = "destructive"
)

type Analisis struct {
	Riesgo Riesgo
	// SinFiltro marca un DELETE o UPDATE sin WHERE, que toca todas las filas.
	SinFiltro bool
	// Palabra es la que levantó la bandera, para poder explicarlo.
	Palabra string
}

var (
	reComentarioLinea  = regexp.MustCompile(`(?m)--[^\n]*$`)
	reComentarioBloque = regexp.MustCompile(`(?s)/\*.*?\*/`)

	reDestructiva = regexp.MustCompile(`(?i)^(DROP|TRUNCATE|ALTER|CREATE|GRANT|REVOKE|RESET|VACUUM)\b`)
	reModifica    = regexp.MustCompile(`(?i)^(INSERT|UPDATE|DELETE|MERGE|UPSERT)\b`)
	reMasiva      = regexp.MustCompile(`(?i)^(UPDATE|DELETE)\b`)
	reWhere       = regexp.MustCompile(`(?i)\bWHERE\b`)
)

// QuitarComentarios limpia el SQL para que las palabras clave no se puedan
// esconder dentro de un comentario.
func QuitarComentarios(sql string) string {
	sql = reComentarioLinea.ReplaceAllString(sql, "")
	return reComentarioBloque.ReplaceAllString(sql, "")
}

// Analizar mira la primera sentencia ejecutable y decide el riesgo.
func Analizar(crudo string) Analisis {
	sql := strings.TrimSpace(QuitarComentarios(crudo))

	primera := ""
	for _, trozo := range strings.Split(sql, ";") {
		if strings.TrimSpace(trozo) != "" {
			primera = strings.TrimLeft(trozo, " \t\r\n")
			break
		}
	}

	if m := reDestructiva.FindStringSubmatch(primera); m != nil {
		return Analisis{Riesgo: Destructiva, Palabra: strings.ToUpper(m[1])}
	}

	if m := reModifica.FindStringSubmatch(primera); m != nil {
		palabra := strings.ToUpper(m[1])

		// Un UPDATE o un DELETE sin WHERE tocan todas las filas de la tabla:
		// eso es destructivo aunque la palabra clave no lo parezca.
		masiva := reMasiva.MatchString(primera) && !reWhere.MatchString(primera)
		if masiva {
			return Analisis{Riesgo: Destructiva, SinFiltro: true, Palabra: palabra}
		}

		return Analisis{Riesgo: Modifica, Palabra: palabra}
	}

	return Analisis{Riesgo: Inofensiva}
}
