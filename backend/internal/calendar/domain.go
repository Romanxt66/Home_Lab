// Package calendar son los eventos del calendario y sus recordatorios.
package calendar

import (
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Evento es una cita del calendario.
type Evento struct {
	ID          string
	Title       string
	Description *string
	Start       time.Time
	End         *time.Time
	AllDay      bool
	Location    *string
	Color       *string
	// Minutos antes del inicio para avisar. Nulo: sin recordatorio.
	RemindMinutesBefore *int
	// Se rellena al mandar el aviso, para no mandarlo dos veces.
	ReminderSentAt *time.Time
	CreatedAt      time.Time
}

// Validar comprueba el evento antes de guardarlo.
func Validar(e Evento) kernel.Result[Evento] {
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return kernel.Err[Evento]("El evento necesita un título.")
	}

	if e.Start.IsZero() {
		return kernel.Err[Evento]("La fecha de inicio no es válida.")
	}

	if e.End != nil {
		if e.End.IsZero() {
			return kernel.Err[Evento]("La fecha de fin no es válida.")
		}
		if e.End.Before(e.Start) {
			return kernel.Err[Evento]("El fin no puede ser anterior al inicio.")
		}
	}

	if e.RemindMinutesBefore != nil && *e.RemindMinutesBefore < 0 {
		return kernel.Err[Evento]("Los minutos de aviso no pueden ser negativos.")
	}

	return kernel.Ok(e)
}

// MomentoDelAviso es cuándo toca avisar de este evento.
func (e Evento) MomentoDelAviso() (time.Time, bool) {
	if e.RemindMinutesBefore == nil {
		return time.Time{}, false
	}
	return e.Start.Add(-time.Duration(*e.RemindMinutesBefore) * time.Minute), true
}

// AvisoPendiente dice si hay que avisar ya de este evento.
//
// Se descartan los eventos que empezaron hace más de un día: si el servidor
// estuvo caído una semana, al volver no debe soltar de golpe todos los
// recordatorios atrasados.
func (e Evento) AvisoPendiente(ahora time.Time) bool {
	if e.ReminderSentAt != nil {
		return false
	}

	momento, hay := e.MomentoDelAviso()
	if !hay {
		return false
	}
	if momento.After(ahora) {
		return false
	}

	return !e.Start.Before(ahora.Add(-24 * time.Hour))
}

// TextoDelAviso arma el mensaje que se manda.
func TextoDelAviso(e Evento) string {
	hora := "todo el día"
	if !e.AllDay {
		hora = e.Start.Format("15:04")
	}

	partes := []string{"⏰ Recordatorio: " + e.Title, "🕒 " + hora}

	if e.Location != nil && *e.Location != "" {
		partes = append(partes, "📍 "+*e.Location)
	}
	if e.Description != nil && *e.Description != "" {
		partes = append(partes, "📝 "+recortar(*e.Description, 200))
	}

	return strings.Join(partes, "\n")
}

func recortar(s string, limite int) string {
	runas := []rune(s)
	if len(runas) <= limite {
		return s
	}
	return string(runas[:limite]) + "…"
}

// RangoDelMes devuelve el intervalo que hay que pedir para pintar un mes.
//
// Se añade una semana por cada lado: la rejilla del mes enseña los últimos
// días del anterior y los primeros del siguiente, y sin ese margen esos
// eventos no aparecerían.
func RangoDelMes(año, mes int, zona *time.Location) (desde, hasta time.Time) {
	primero := time.Date(año, time.Month(mes), 1, 0, 0, 0, 0, zona)
	return primero.AddDate(0, 0, -7), primero.AddDate(0, 1, 7)
}
