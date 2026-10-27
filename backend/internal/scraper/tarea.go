package scraper

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// parametros son los que guarda una tarea programada de tipo "scrape".
type parametros struct {
	URL      string `json:"url"`
	Selector string `json:"selector"`
	Atributo string `json:"attribute"`
}

// EjecutorDeTarea adapta el scraping a lo que espera el planificador.
//
// Devuelve la función suelta en vez de un tipo del planificador para que este
// paquete no tenga que importarlo: la dependencia va en un solo sentido.
func EjecutorDeTarea(svc *Service) func(context.Context, json.RawMessage) kernel.Result[string] {
	return func(ctx context.Context, crudos json.RawMessage) kernel.Result[string] {
		var p parametros
		if err := json.Unmarshal(crudos, &p); err != nil {
			return kernel.Err[string]("Los parámetros de la tarea no tienen la forma esperada.")
		}

		res := svc.Ejecutar(ctx, p.URL, p.Selector, p.Atributo)
		if !res.OK {
			return kernel.Err[string](res.Error)
		}

		return kernel.Ok(resumir(res.Value))
	}
}

// resumir deja un texto corto para el historial: guardar ahí veinte mil
// caracteres de HTML llenaría la base sin aportar nada.
func resumir(r Resultado) string {
	if r.Elementos == nil {
		return "HTTP " + strconv.Itoa(r.Status) + ", " +
			strconv.Itoa(len([]rune(r.Crudo))) + " caracteres."
	}

	cabecera := "HTTP " + strconv.Itoa(r.Status) + ", " +
		strconv.Itoa(len(r.Elementos)) + " coincidencias."
	if len(r.Elementos) == 0 {
		return cabecera
	}

	muestra := r.Elementos
	if len(muestra) > 5 {
		muestra = muestra[:5]
	}
	return cabecera + " Por ejemplo: " + Recortar(strings.Join(muestra, " · "), 500)
}
