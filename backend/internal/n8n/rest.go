package n8n

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const tiempoLimite = 20 * time.Second

// RESTCliente habla con la API pública de n8n (v1), autenticándose con la
// cabecera X-N8N-API-KEY.
type RESTCliente struct {
	client *http.Client
}

func NewRESTCliente() *RESTCliente {
	return &RESTCliente{client: &http.Client{Timeout: tiempoLimite}}
}

func (r *RESTCliente) Ping(ctx context.Context, c Credenciales) kernel.Result[struct{}] {
	_, res := r.pedir(ctx, c, http.MethodGet, "/workflows?limit=1")
	if !res.OK {
		return kernel.Err[struct{}](res.Error)
	}
	return kernel.Ok(struct{}{})
}

func (r *RESTCliente) ListarWorkflows(ctx context.Context, c Credenciales) kernel.Result[[]Workflow] {
	cuerpo, res := r.pedir(ctx, c, http.MethodGet, "/workflows?limit=250")
	if !res.OK {
		return kernel.Err[[]Workflow](res.Error)
	}

	// n8n devuelve {"data": [...]}, pero en algunas versiones es la lista
	// directamente. Se aceptan las dos formas.
	var envoltorio struct {
		Data []workflowCrudo `json:"data"`
	}
	if err := json.Unmarshal(cuerpo, &envoltorio); err == nil && envoltorio.Data != nil {
		return kernel.Ok(convertir(envoltorio.Data))
	}

	var lista []workflowCrudo
	if err := json.Unmarshal(cuerpo, &lista); err != nil {
		return kernel.Err[[]Workflow]("n8n devolvió una respuesta que no se entiende.")
	}
	return kernel.Ok(convertir(lista))
}

func (r *RESTCliente) Activar(ctx context.Context, c Credenciales, id string, activo bool) kernel.Result[struct{}] {
	accion := "deactivate"
	if activo {
		accion = "activate"
	}

	ruta := "/workflows/" + url.PathEscape(id) + "/" + accion
	if _, res := r.pedir(ctx, c, http.MethodPost, ruta); !res.OK {
		return kernel.Err[struct{}](res.Error)
	}
	return kernel.Ok(struct{}{})
}

// workflowCrudo refleja lo que manda n8n. El id llega unas veces como texto
// y otras como número, así que se lee sin tipo y se normaliza después.
type workflowCrudo struct {
	ID        any     `json:"id"`
	Name      string  `json:"name"`
	Active    bool    `json:"active"`
	UpdatedAt *string `json:"updatedAt"`
}

func convertir(crudos []workflowCrudo) []Workflow {
	salida := make([]Workflow, 0, len(crudos))
	for _, w := range crudos {
		nombre := w.Name
		if strings.TrimSpace(nombre) == "" {
			nombre = "(sin nombre)"
		}
		salida = append(salida, Workflow{
			ID:        comoTexto(w.ID),
			Name:      nombre,
			Active:    w.Active,
			UpdatedAt: w.UpdatedAt,
		})
	}
	return salida
}

func comoTexto(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64: // todo número en JSON llega como float64
		return fmt.Sprintf("%.0f", t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

// pedir ejecuta la llamada y traduce el fallo a un mensaje que se pueda leer.
func (r *RESTCliente) pedir(ctx context.Context, c Credenciales, metodo, ruta string) ([]byte, kernel.Result[struct{}]) {
	req, err := http.NewRequestWithContext(ctx, metodo, c.BaseURL+"/api/v1"+ruta, nil)
	if err != nil {
		return nil, kernel.Err[struct{}]("No se pudo preparar la petición a n8n.")
	}
	req.Header.Set("X-N8N-API-KEY", c.APIKey)
	req.Header.Set("Accept", "application/json")

	res, err := r.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return nil, kernel.Err[struct{}]("n8n tardó más de 20 segundos en responder.")
		}
		return nil, kernel.Err[struct{}]("No se pudo conectar con n8n.")
	}
	defer res.Body.Close()

	cuerpo, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, kernel.Err[struct{}](explicar(res.StatusCode, cuerpo))
	}
	return cuerpo, kernel.Ok(struct{}{})
}

// explicar convierte el código de estado en algo accionable.
func explicar(estado int, cuerpo []byte) string {
	switch estado {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "El API key no vale o no tiene permisos. Revísalo en n8n → Settings → n8n API."
	case http.StatusNotFound:
		return "n8n no encontró eso. ¿Sigue existiendo el flujo?"
	}

	var detalle struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(cuerpo, &detalle); err == nil && detalle.Message != "" {
		return "n8n: " + detalle.Message
	}
	return fmt.Sprintf("n8n respondió con un error (HTTP %d).", estado)
}
