package scheduler

import (
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Accion es lo que hace una tarea cuando le toca.
type Accion string

const (
	AccionEmail  Accion = "email"
	AccionScrape Accion = "scrape"
)

// Job es una tarea programada.
type Job struct {
	ID   string
	Name string
	Cron string
	// Accion y sus parámetros, guardados como JSON: cada acción espera unos
	// distintos, y una columna por cada uno no escalaría.
	Action    Accion
	Params    string
	Enabled   bool
	CreatedAt time.Time
}

// Run es el resultado de una ejecución.
type Run struct {
	ID        string
	JobID     string
	JobName   string
	Status    string // "success" | "error"
	Output    *string
	StartedAt time.Time
}

const (
	EstadoOK    = "success"
	EstadoError = "error"
)

// ValidarJob comprueba lo que se puede comprobar sin tocar la base.
func ValidarJob(nombre, expr string, accion Accion) kernel.Result[struct{}] {
	if strings.TrimSpace(nombre) == "" {
		return kernel.Err[struct{}]("La tarea necesita un nombre.")
	}

	if res := ParsearCron(expr); !res.OK {
		return kernel.Err[struct{}](res.Error)
	}

	if accion != AccionEmail && accion != AccionScrape {
		return kernel.Err[struct{}]("Esa acción no existe. Las hay de correo y de scraping.")
	}

	return kernel.Ok(struct{}{})
}
