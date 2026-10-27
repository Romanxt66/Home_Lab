// Package automations son reglas «cuando pase X, haz Y». Otros módulos
// disparan los eventos; aquí se decide qué hacer con ellos.
package automations

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Disparador es un suceso al que se puede reaccionar.
type Disparador string

const (
	UsuarioRegistrado Disparador = "user_registered"
)

// InfoDisparador describe el suceso y qué variables trae, para que la
// interfaz pueda enseñarlas sin tenerlas codificadas por su cuenta.
type InfoDisparador struct {
	Etiqueta    string
	Descripcion string
	Variables   []string
}

var Disparadores = map[Disparador]InfoDisparador{
	UsuarioRegistrado: {
		Etiqueta: "Nuevo registro pendiente",
		Descripcion: "Alguien se registró y espera aprobación. El aviso de Telegram ya " +
			"sale solo.",
		Variables: []string{"nombre", "email"},
	},
}

func DisparadorValido(d Disparador) bool {
	_, hay := Disparadores[d]
	return hay
}

// TipoAccion es lo que una regla puede hacer.
type TipoAccion string

const (
	TelegramAccion TipoAccion = "telegram"
	EventoAccion   TipoAccion = "calendar_event"
	WebhookAccion  TipoAccion = "n8n_webhook"
)

// Accion es una cosa que hacer, con su configuración. Se guarda como JSON,
// así que la configuración viaja como mapa en vez de como unión de tipos.
type Accion struct {
	Type   TipoAccion        `json:"type"`
	Config map[string]string `json:"config"`
}

// Regla es «cuando pase esto, haz esto otro».
type Regla struct {
	ID        string
	Name      string
	Trigger   Disparador
	Active    bool
	Actions   []Accion
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Ejecucion es una vez que saltó la regla.
type Ejecucion struct {
	OK    bool
	Error *string
	RanAt time.Time
}

// ValidarRegla comprueba la regla entera antes de guardarla.
func ValidarRegla(r Regla) kernel.Result[Regla] {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return kernel.Err[Regla]("La regla necesita un nombre.")
	}

	if !DisparadorValido(r.Trigger) {
		return kernel.Err[Regla]("Ese disparador no existe.")
	}

	if len(r.Actions) == 0 {
		return kernel.Err[Regla]("Añade al menos una acción.")
	}

	for i, a := range r.Actions {
		if res := validarAccion(a, i); !res.OK {
			return kernel.Err[Regla](res.Error)
		}
	}

	return kernel.Ok(r)
}

// validarAccion revisa una acción. El número va en el mensaje porque una
// regla puede tener varias y hay que saber cuál falla.
func validarAccion(a Accion, indice int) kernel.Result[struct{}] {
	prefijo := "Acción " + strconv.Itoa(indice+1) + ": "

	switch a.Type {
	case TelegramAccion:
		if strings.TrimSpace(a.Config["message"]) == "" {
			return kernel.Err[struct{}](prefijo + "el mensaje de Telegram no puede estar vacío.")
		}

	case EventoAccion:
		if strings.TrimSpace(a.Config["title"]) == "" {
			return kernel.Err[struct{}](prefijo + "el evento necesita un título.")
		}
		dias := strings.TrimSpace(a.Config["daysFromNow"])
		if dias != "" {
			n, err := strconv.Atoi(dias)
			if err != nil || n < 0 {
				return kernel.Err[struct{}](prefijo + "los días desde hoy deben ser 0 o más.")
			}
		}

	case WebhookAccion:
		crudo := strings.TrimSpace(a.Config["webhookUrl"])
		if crudo == "" {
			return kernel.Err[struct{}](prefijo + "escribe la URL del webhook de n8n.")
		}
		u, err := url.Parse(crudo)
		if err != nil || u.Host == "" {
			return kernel.Err[struct{}](prefijo + "la URL del webhook no es válida.")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return kernel.Err[struct{}](prefijo + "la URL del webhook debe empezar por http:// o https://")
		}

	default:
		return kernel.Err[struct{}](prefijo + "ese tipo de acción no existe.")
	}

	return kernel.Ok(struct{}{})
}

// DiasDesdeHoy lee el desplazamiento de una acción de calendario.
func DiasDesdeHoy(a Accion) int {
	n, err := strconv.Atoi(strings.TrimSpace(a.Config["daysFromNow"]))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
