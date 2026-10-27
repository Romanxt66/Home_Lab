package dbadmin

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Sentencia es una consulta lista para el driver, con sus parámetros aparte.
type Sentencia struct {
	Texto      string
	Parametros []any
}

type Direccion string

const (
	Ascendente  Direccion = "asc"
	Descendente Direccion = "desc"
)

const (
	MaxPagina        = 200
	PaginaPorDefecto = 50
)

// AcotarLimite deja el tamaño de página en un rango razonable.
func AcotarLimite(limite int) int {
	if limite <= 0 {
		return PaginaPorDefecto
	}
	if limite > MaxPagina {
		return MaxPagina
	}
	return limite
}

// OpcionesHojear describe qué página de qué tabla se quiere.
type OpcionesHojear struct {
	Esquema    string
	Tabla      string
	Limite     int
	Desde      int
	OrdenarPor string
	Direccion  Direccion
}

// ConstruirHojear arma el SELECT de una página.
//
// Pide una fila de más para saber si hay página siguiente sin tener que
// hacer un COUNT(*) en cada paso, que en una tabla grande es caro.
func ConstruirHojear(o OpcionesHojear) Sentencia {
	limite := AcotarLimite(o.Limite)
	desde := o.Desde
	if desde < 0 {
		desde = 0
	}

	orden := ""
	if o.OrdenarPor != "" {
		direccion := "ASC"
		if o.Direccion == Descendente {
			direccion = "DESC"
		}
		orden = " ORDER BY " + Citar(o.OrdenarPor) + " " + direccion
	}

	return Sentencia{
		Texto: "SELECT * FROM " + CitarCualificado(o.Esquema, o.Tabla) +
			orden + " LIMIT $1 OFFSET $2",
		Parametros: []any{limite + 1, desde},
	}
}

func ConstruirContar(esquema, tabla string) Sentencia {
	return Sentencia{
		Texto: "SELECT COUNT(*)::bigint AS n FROM " + CitarCualificado(esquema, tabla),
	}
}

// ConstruirInsertar arma el INSERT con todos los valores parametrizados.
func ConstruirInsertar(esquema, tabla string, valores map[string]any) kernel.Result[Sentencia] {
	columnas := ordenadas(valores)
	if len(columnas) == 0 {
		return kernel.Err[Sentencia]("No hay valores que insertar.")
	}

	citadas := make([]string, 0, len(columnas))
	marcadores := make([]string, 0, len(columnas))
	parametros := make([]any, 0, len(columnas))

	for i, c := range columnas {
		if v := ValidarIdentificador(c, "nombre de columna"); !v.OK {
			return kernel.Err[Sentencia](v.Error)
		}
		citadas = append(citadas, Citar(c))
		marcadores = append(marcadores, "$"+strconv.Itoa(i+1))
		parametros = append(parametros, valores[c])
	}

	return kernel.Ok(Sentencia{
		Texto: "INSERT INTO " + CitarCualificado(esquema, tabla) +
			" (" + strings.Join(citadas, ", ") + ") VALUES (" +
			strings.Join(marcadores, ", ") + ") RETURNING *",
		Parametros: parametros,
	})
}

// ConstruirActualizar arma el UPDATE con su WHERE por clave primaria.
//
// La clave tiene que identificar exactamente una fila. Una clave vacía se
// rechaza en vez de actualizar la tabla entera en silencio.
func ConstruirActualizar(esquema, tabla string, valores, clave map[string]any) kernel.Result[Sentencia] {
	columnas := ordenadas(valores)
	if len(columnas) == 0 {
		return kernel.Err[Sentencia]("No hay cambios que guardar.")
	}
	if len(clave) == 0 {
		return kernel.Err[Sentencia](
			"No se puede editar una fila sin una clave primaria que la identifique.")
	}

	var parametros []any
	asignaciones := make([]string, 0, len(columnas))

	for _, c := range columnas {
		if v := ValidarIdentificador(c, "nombre de columna"); !v.OK {
			return kernel.Err[Sentencia](v.Error)
		}
		parametros = append(parametros, valores[c])
		asignaciones = append(asignaciones, Citar(c)+" = $"+strconv.Itoa(len(parametros)))
	}

	condiciones, parametros, res := condicionesDeClave(clave, parametros)
	if !res.OK {
		return kernel.Err[Sentencia](res.Error)
	}

	return kernel.Ok(Sentencia{
		Texto: "UPDATE " + CitarCualificado(esquema, tabla) +
			" SET " + strings.Join(asignaciones, ", ") +
			" WHERE " + strings.Join(condiciones, " AND ") + " RETURNING *",
		Parametros: parametros,
	})
}

// ConstruirBorrar arma el DELETE. Una clave vacía se rechaza: borraría toda
// la tabla.
func ConstruirBorrar(esquema, tabla string, clave map[string]any) kernel.Result[Sentencia] {
	if len(clave) == 0 {
		return kernel.Err[Sentencia](
			"No se puede borrar una fila sin una clave primaria que la identifique.")
	}

	condiciones, parametros, res := condicionesDeClave(clave, nil)
	if !res.OK {
		return kernel.Err[Sentencia](res.Error)
	}

	return kernel.Ok(Sentencia{
		Texto: "DELETE FROM " + CitarCualificado(esquema, tabla) +
			" WHERE " + strings.Join(condiciones, " AND "),
		Parametros: parametros,
	})
}

func condicionesDeClave(clave map[string]any, parametros []any) ([]string, []any, kernel.Result[struct{}]) {
	columnas := ordenadas(clave)
	condiciones := make([]string, 0, len(columnas))

	for _, c := range columnas {
		if v := ValidarIdentificador(c, "nombre de columna"); !v.OK {
			return nil, nil, kernel.Err[struct{}](v.Error)
		}
		parametros = append(parametros, clave[c])
		condiciones = append(condiciones, Citar(c)+" = $"+strconv.Itoa(len(parametros)))
	}

	return condiciones, parametros, kernel.Ok(struct{}{})
}

// ordenadas devuelve las claves del mapa en orden alfabético: un mapa de Go
// no lo garantiza, y sin esto la sentencia cambiaría en cada llamada, lo que
// vuelve imposible probarla y confunde al leer los registros.
func ordenadas(m map[string]any) []string {
	nombres := make([]string, 0, len(m))
	for k := range m {
		nombres = append(nombres, k)
	}
	sort.Strings(nombres)
	return nombres
}
