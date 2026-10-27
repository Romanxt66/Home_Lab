package finance

import (
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

type Frecuencia string

const (
	Semanal Frecuencia = "weekly"
	Mensual Frecuencia = "monthly"
	Anual   Frecuencia = "yearly"
)

var Frecuencias = []Frecuencia{Semanal, Mensual, Anual}

var EtiquetasDeFrecuencia = map[Frecuencia]string{
	Semanal: "Semanal",
	Mensual: "Mensual",
	Anual:   "Anual",
}

func frecuenciaValida(f Frecuencia) bool {
	return f == Semanal || f == Mensual || f == Anual
}

// Recurrente es un pago que se repite: el alquiler, una suscripción.
type Recurrente struct {
	ID               string
	Name             string
	AccountID        string
	CategoryID       *string
	Kind             TipoMovimiento
	Amount           Dinero
	Frequency        Frecuencia
	NextRunAt        time.Time
	RemindDaysBefore *int
	ReminderSentAt   *time.Time
	Active           bool
	Notes            *string
	CreatedAt        time.Time
}

func ValidarRecurrente(r Recurrente) kernel.Result[Recurrente] {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return kernel.Err[Recurrente]("El pago recurrente necesita un nombre.")
	}
	if strings.TrimSpace(r.AccountID) == "" {
		return kernel.Err[Recurrente]("Elige una cuenta.")
	}
	if !tipoDeMovimientoValido(r.Kind) {
		return kernel.Err[Recurrente]("Un pago recurrente es un ingreso o un gasto.")
	}
	if !frecuenciaValida(r.Frequency) {
		return kernel.Err[Recurrente]("Esa frecuencia no existe: semanal, mensual o anual.")
	}
	if r.Amount <= 0 {
		return kernel.Err[Recurrente]("El importe tiene que ser mayor que cero.")
	}
	if r.NextRunAt.IsZero() {
		return kernel.Err[Recurrente]("La fecha del próximo cargo no es válida.")
	}
	if r.RemindDaysBefore != nil && *r.RemindDaysBefore < 0 {
		return kernel.Err[Recurrente]("Los días de aviso no pueden ser negativos.")
	}

	r.Notes = limpiarOpcional(r.Notes)
	return kernel.Ok(r)
}

// Avanzar mueve la fecha un periodo.
//
// En los meses se usa aritmética de calendario, con la deriva conocida: el 31
// de enero más un mes cae en marzo, porque febrero no tiene 31. Para un
// cuaderno personal es aceptable, y es exactamente lo que hacía el Lab.
func Avanzar(fecha time.Time, f Frecuencia) time.Time {
	switch f {
	case Semanal:
		return fecha.AddDate(0, 0, 7)
	case Anual:
		return fecha.AddDate(1, 0, 0)
	default:
		return fecha.AddDate(0, 1, 0)
	}
}

// MaxRecuperacion acota cuántos cargos atrasados se generan de una vez: si la
// fecha quedó años atrás, sin tope el bucle no terminaría nunca.
const MaxRecuperacion = 60

// Vencidos devuelve los cargos que tocaban hasta ahora y la fecha siguiente.
//
// Sirve para recuperar lo que no se procesó si el servidor estuvo caído: en
// vez de saltarse una semana de alquileres, los genera todos.
func Vencidos(desde time.Time, f Frecuencia, ahora time.Time) (ocurrencias []time.Time, siguiente time.Time) {
	cursor := desde

	for !cursor.After(ahora) && len(ocurrencias) < MaxRecuperacion {
		ocurrencias = append(ocurrencias, cursor)
		cursor = Avanzar(cursor, f)
	}

	return ocurrencias, cursor
}

// TocaAvisar dice si hay que mandar el recordatorio de este pago.
func TocaAvisar(r Recurrente, ahora time.Time) bool {
	if !r.Active || r.RemindDaysBefore == nil || r.ReminderSentAt != nil {
		return false
	}

	inicio := r.NextRunAt.AddDate(0, 0, -*r.RemindDaysBefore)

	// La ventana va del aviso al cargo: después de cobrarlo, avisar de que
	// va a llegar no tiene sentido.
	return !ahora.Before(inicio) && ahora.Before(r.NextRunAt)
}

// TextoDelAviso arma el recordatorio que se manda.
func TextoDelAviso(r Recurrente, moneda string) string {
	que := "Cobro"
	if r.Kind == Gasto {
		que = "Pago"
	}

	return "💸 " + que + " próximo: " + r.Name + "\n" +
		r.Amount.String() + " " + moneda + "\n" +
		"📅 " + r.NextRunAt.Format("02/01/2006")
}
