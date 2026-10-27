package email

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// parametros son los que guarda una tarea programada de tipo "email".
type parametros struct {
	Recipients      string            `json:"recipients"`
	Subject         string            `json:"subject"`
	Body            string            `json:"body"`
	Variables       map[string]string `json:"variables"`
	GoogleAccountID string            `json:"googleAccountId"`
}

// EjecutorDeTarea adapta el envío a lo que espera el planificador. Se
// devuelve la función suelta para que este paquete no importe aquel.
func EjecutorDeTarea(svc *Service) func(context.Context, json.RawMessage) kernel.Result[string] {
	return func(ctx context.Context, crudos json.RawMessage) kernel.Result[string] {
		var p parametros
		if err := json.Unmarshal(crudos, &p); err != nil {
			return kernel.Err[string]("Los parámetros de la tarea no tienen la forma esperada.")
		}

		res, err := svc.Enviar(ctx, p.Recipients, p.Subject, p.Body, p.Variables, p.GoogleAccountID)
		if err != nil {
			return kernel.Err[string]("No se pudo enviar: fallo del servidor.")
		}
		if !res.OK {
			return kernel.Err[string](res.Error)
		}

		resumen := "Enviados " + strconv.Itoa(res.Value.Enviados)
		if res.Value.Fallidos > 0 {
			resumen += ", fallidos " + strconv.Itoa(res.Value.Fallidos)
		}
		return kernel.Ok(resumen + ".")
	}
}
