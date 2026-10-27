// Package n8n es el panel de la instancia de n8n: guardar la conexión, listar
// los flujos y encenderlos o apagarlos.
package n8n

import (
	"net/url"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Config es la conexión guardada. La clave se queda en el servidor y no sale
// nunca al navegador.
type Config struct {
	ID        string
	BaseURL   string
	APIKey    string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Workflow es un flujo de n8n, con lo poco que hace falta para listarlo.
type Workflow struct {
	ID        string
	Name      string
	Active    bool
	UpdatedAt *string
}

// ValidarBaseURL normaliza la dirección de la instancia: exige http(s) y
// quita la barra final, porque luego se le concatena la ruta de la API y
// una barra de más daría una URL con dos.
func ValidarBaseURL(crudo string) kernel.Result[string] {
	limpio := strings.TrimRight(strings.TrimSpace(crudo), "/")
	if limpio == "" {
		return kernel.Err[string]("Escribe la dirección de tu instancia de n8n.")
	}

	u, err := url.Parse(limpio)
	if err != nil {
		return kernel.Err[string]("La dirección no es válida (por ejemplo https://n8n.midominio.com).")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return kernel.Err[string]("La dirección debe empezar por http:// o https://")
	}
	if u.Host == "" {
		return kernel.Err[string]("La dirección no tiene servidor.")
	}

	return kernel.Ok(limpio)
}

func ValidarAPIKey(crudo string) kernel.Result[string] {
	k := strings.TrimSpace(crudo)
	if k == "" {
		return kernel.Err[string]("Escribe el API key de n8n.")
	}
	return kernel.Ok(k)
}
