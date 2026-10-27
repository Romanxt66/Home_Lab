// Package uptime es el monitor de disponibilidad: vigila una lista de URLs,
// guarda cada comprobación y avisa cuando una se cae o se recupera.
//
// Este archivo es el dominio: lógica pura, sin base de datos y sin HTTP. Todo
// lo que hay aquí se puede probar sin levantar nada.
package uptime

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

type Status string

const (
	StatusUp      Status = "up"
	StatusDown    Status = "down"
	StatusUnknown Status = "unknown"
)

type Method string

const (
	MethodGET  Method = "GET"
	MethodHEAD Method = "HEAD"
)

// Límites de validación, con los mismos valores que el Lab original.
const (
	intervaloMinimo = 30
	timeoutMinimo   = 1000
	timeoutMaximo   = 60000
)

// Monitor es una URL vigilada, con su configuración y su estado actual.
type Monitor struct {
	ID                  string
	Name                string
	URL                 string
	Method              Method
	ExpectedStatus      int // 0 = vale cualquier 2xx o 3xx
	IntervalSeconds     int
	TimeoutMs           int
	Active              bool
	NotifyOnFailure     bool
	LastStatus          Status
	LastCheckedAt       *time.Time
	LastResponseMs      *int
	LastStatusCode      *int
	LastError           *string
	ConsecutiveFailures int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// MonitorInput son los campos que llegan del formulario.
type MonitorInput struct {
	Name            string
	URL             string
	Method          Method
	ExpectedStatus  int
	IntervalSeconds int
	TimeoutMs       int
	Active          bool
	NotifyOnFailure bool
}

// Validate comprueba la entrada y la devuelve normalizada (sin espacios
// sobrantes y sin barra final en la URL).
func Validate(in MonitorInput) kernel.Result[MonitorInput] {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return kernel.Err[MonitorInput]("El nombre no puede estar vacío.")
	}

	crudo := strings.TrimSpace(in.URL)
	parsed, err := url.Parse(crudo)
	if err != nil || parsed.Host == "" {
		return kernel.Err[MonitorInput]("La URL no es válida (ej. https://midominio.com).")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return kernel.Err[MonitorInput]("La URL debe empezar por http:// o https://")
	}

	if in.Method != MethodGET && in.Method != MethodHEAD {
		return kernel.Err[MonitorInput](fmt.Sprintf("Método no soportado: %s", in.Method))
	}
	if in.ExpectedStatus < 0 || in.ExpectedStatus > 599 {
		return kernel.Err[MonitorInput]("El código de estado esperado no es válido (0 = cualquiera).")
	}
	if in.IntervalSeconds < intervaloMinimo {
		return kernel.Err[MonitorInput](
			fmt.Sprintf("El intervalo mínimo es %d segundos.", intervaloMinimo))
	}
	if in.TimeoutMs < timeoutMinimo || in.TimeoutMs > timeoutMaximo {
		return kernel.Err[MonitorInput]("El timeout debe estar entre 1s y 60s.")
	}

	// Se quita la barra final, pero nunca se deja la URL vacía.
	sinBarra := strings.TrimRight(crudo, "/")
	if sinBarra == "" {
		sinBarra = crudo
	}
	in.URL = sinBarra

	return kernel.Ok(in)
}

// ProbeResult es lo que devuelve el adaptador que hace la petición HTTP.
type ProbeResult struct {
	OK         bool
	StatusCode *int
	ResponseMs *int
	Error      *string
}

// IsUp decide si una comprobación cuenta como "en línea".
// ExpectedStatus a 0 acepta cualquier 2xx o 3xx; si no, el código debe coincidir.
func IsUp(expectedStatus int, probe ProbeResult) bool {
	if !probe.OK || probe.StatusCode == nil {
		return false
	}
	if expectedStatus == 0 {
		return *probe.StatusCode >= 200 && *probe.StatusCode < 400
	}
	return *probe.StatusCode == expectedStatus
}

// IsDue dice si al monitor le toca comprobación en este instante.
func IsDue(m Monitor, now time.Time) bool {
	if !m.Active {
		return false
	}
	if m.LastCheckedAt == nil {
		return true
	}
	siguiente := m.LastCheckedAt.Add(time.Duration(m.IntervalSeconds) * time.Second)
	return !now.Before(siguiente)
}

// Alert es el aviso que dispara un cambio de estado.
type Alert string

const (
	AlertNone      Alert = ""
	AlertDown      Alert = "down"
	AlertRecovered Alert = "recovered"
)

// Transition calcula el estado nuevo y si hay que avisar. Solo se avisa en un
// cambio real: de "unknown" al primer estado conocido no se molesta a nadie,
// salvo que ese primer estado ya sea "caído".
func Transition(prev Status, nowUp bool) (Status, Alert) {
	nuevo := StatusDown
	if nowUp {
		nuevo = StatusUp
	}

	if prev == nuevo {
		return nuevo, AlertNone
	}
	if prev == StatusDown && nuevo == StatusUp {
		return nuevo, AlertRecovered
	}
	if prev == StatusUp && nuevo == StatusDown {
		return nuevo, AlertDown
	}
	if nuevo == StatusDown {
		return nuevo, AlertDown
	}
	return nuevo, AlertNone
}

// Check es una comprobación ya guardada.
type Check struct {
	OK         bool
	StatusCode *int
	ResponseMs *int
	Error      *string
	CheckedAt  time.Time
}

// UptimePercent calcula el porcentaje de disponibilidad con un decimal.
func UptimePercent(checks []Check) float64 {
	if len(checks) == 0 {
		return 0
	}
	correctas := 0
	for _, c := range checks {
		if c.OK {
			correctas++
		}
	}
	return math.Round(float64(correctas)/float64(len(checks))*1000) / 10
}

// StatusLabel traduce el estado para mostrarlo.
func StatusLabel(s Status) string {
	switch s {
	case StatusUp:
		return "En línea"
	case StatusDown:
		return "Caído"
	default:
		return "Sin datos"
	}
}
