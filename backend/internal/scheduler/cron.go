// Package scheduler ejecuta tareas según una expresión cron.
package scheduler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Un cron de cinco campos, el de toda la vida:
//
//	minuto(0-59) hora(0-23) día(1-31) mes(1-12) día-semana(0-6, 0=domingo)
//
// Se admite "*", el paso "/n", un valor suelto, un rango "a-b", "a-b/n" y
// listas separadas por comas. Se escribe a mano, sin biblioteca: son treinta
// líneas y así el comportamiento es exactamente el mismo que el del Lab.
type rango struct{ min, max int }

var campos = [5]rango{
	{0, 59}, // minuto
	{0, 23}, // hora
	{1, 31}, // día del mes
	{1, 12}, // mes
	// Hasta 7, no hasta 6: el domingo se escribe 0 o 7 según la costumbre, y
	// las dos formas circulan. El Lab validaba hasta 6 pero luego comparaba
	// contra el 7, así que esa rama no se cumplía nunca y un cron escrito
	// con 7 se rechazaba sin explicar por qué. Aquí se acepta y se normaliza.
	{0, 7}, // día de la semana
}

var nombres = [5]string{"minuto", "hora", "día", "mes", "día de la semana"}

// parsearCampo convierte un campo en el conjunto de valores que acepta.
func parsearCampo(campo string, r rango) (map[int]bool, bool) {
	valores := make(map[int]bool)

	for _, parte := range strings.Split(campo, ",") {
		cuerpo, pasoTexto, hayPaso := strings.Cut(parte, "/")

		paso := 1
		if hayPaso {
			n, err := strconv.Atoi(pasoTexto)
			if err != nil || n <= 0 {
				return nil, false
			}
			paso = n
		}

		var desde, hasta int
		switch {
		case cuerpo == "*":
			desde, hasta = r.min, r.max

		case strings.Contains(cuerpo, "-"):
			a, b, _ := strings.Cut(cuerpo, "-")
			na, errA := strconv.Atoi(a)
			nb, errB := strconv.Atoi(b)
			if errA != nil || errB != nil {
				return nil, false
			}
			desde, hasta = na, nb

		default:
			n, err := strconv.Atoi(cuerpo)
			if err != nil {
				return nil, false
			}
			desde, hasta = n, n
		}

		if desde < r.min || hasta > r.max || desde > hasta {
			return nil, false
		}
		for v := desde; v <= hasta; v += paso {
			valores[v] = true
		}
	}

	return valores, true
}

// ParsearCron descompone la expresión en un conjunto de valores por campo.
func ParsearCron(expr string) kernel.Result[[]map[int]bool] {
	partes := strings.Fields(strings.TrimSpace(expr))
	if len(partes) != 5 {
		return kernel.Err[[]map[int]bool](
			"Un cron tiene 5 campos: minuto hora día mes día-semana (por ejemplo */5 * * * *).")
	}

	conjuntos := make([]map[int]bool, 5)
	for i, parte := range partes {
		valores, ok := parsearCampo(parte, campos[i])
		if !ok {
			return kernel.Err[[]map[int]bool]("El campo «" + nombres[i] + "» no es válido: " + parte)
		}
		conjuntos[i] = valores
	}

	return kernel.Ok(conjuntos)
}

// CronValido dice si la expresión se entiende.
func CronValido(expr string) bool {
	return ParsearCron(expr).OK
}

// CronCoincide responde si a esa hora toca ejecutar.
func CronCoincide(expr string, momento time.Time) bool {
	res := ParsearCron(expr)
	if !res.OK {
		return false
	}
	c := res.Value

	// time.Weekday() devuelve 0 para el domingo; el cron pudo escribirlo
	// como 0 o como 7.
	diaSemana := int(momento.Weekday())
	semanaOK := c[4][diaSemana] || (diaSemana == 0 && c[4][7])

	return c[0][momento.Minute()] &&
		c[1][momento.Hour()] &&
		c[2][momento.Day()] &&
		c[3][int(momento.Month())] &&
		semanaOK
}
