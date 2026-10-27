package uptime

import (
	"context"
	"fmt"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ── Puertos ────────────────────────────────────────────────────────────────
// Las interfaces se declaran aquí, en el lado que las consume, que es lo
// idiomático en Go. Las implementaciones viven en postgres.go y probe.go.

// MonitorRepo guarda y recupera monitores.
type MonitorRepo interface {
	List(ctx context.Context) ([]Monitor, error)
	ListActive(ctx context.Context) ([]Monitor, error)
	Get(ctx context.Context, id string) (*Monitor, error)
	Create(ctx context.Context, in MonitorInput) (*Monitor, error)
	Update(ctx context.Context, id string, in MonitorInput) (*Monitor, error)
	Remove(ctx context.Context, id string) error
	ApplyCheck(ctx context.Context, id string, patch CheckPatch) error
}

// CheckRepo guarda el histórico de comprobaciones.
type CheckRepo interface {
	Add(ctx context.Context, monitorID string, c Check) error
	ListForMonitor(ctx context.Context, monitorID string, limit int) ([]Check, error)
}

// Prober hace la petición HTTP de verdad.
type Prober interface {
	Probe(ctx context.Context, url string, method Method, timeoutMs int) ProbeResult
}

// Notifier manda el aviso cuando algo se cae o se recupera. De momento solo
// se registra en el log; cuando se porte el módulo de notificaciones, se
// cambia la implementación sin tocar este servicio.
type Notifier interface {
	Send(ctx context.Context, message string) error
}

// CheckPatch son los campos que se actualizan tras una comprobación.
type CheckPatch struct {
	LastStatus          Status
	LastCheckedAt       time.Time
	LastResponseMs      *int
	LastStatusCode      *int
	LastError           *string
	ConsecutiveFailures int
}

// ── Servicio ───────────────────────────────────────────────────────────────

// Detail es un monitor con su histórico y su disponibilidad de 24 horas.
type Detail struct {
	Monitor   Monitor
	Checks    []Check
	Uptime24h float64
}

// RunSummary resume una pasada del cron.
type RunSummary struct {
	Checked   int      `json:"checked"`
	Down      int      `json:"down"`
	Recovered int      `json:"recovered"`
	Errors    []string `json:"errors"`
}

// Service orquesta el monitor: el alta y baja de monitores, y la comprobación
// periódica. La decisión de si algo está caído y de si hay que avisar es
// dominio puro (IsUp y Transition); aquí solo se encadenan los efectos.
type Service struct {
	monitors MonitorRepo
	checks   CheckRepo
	prober   Prober
	notifier Notifier
}

func NewService(m MonitorRepo, c CheckRepo, p Prober, n Notifier) *Service {
	return &Service{monitors: m, checks: c, prober: p, notifier: n}
}

func (s *Service) List(ctx context.Context) ([]Monitor, error) {
	return s.monitors.List(ctx)
}

func (s *Service) Detail(ctx context.Context, id string) (*Detail, error) {
	monitor, err := s.monitors.Get(ctx, id)
	if err != nil || monitor == nil {
		return nil, err
	}

	checks, err := s.checks.ListForMonitor(ctx, id, 100)
	if err != nil {
		return nil, err
	}

	desde := time.Now().Add(-24 * time.Hour)
	var ultimas []Check
	for _, c := range checks {
		if !c.CheckedAt.Before(desde) {
			ultimas = append(ultimas, c)
		}
	}

	return &Detail{Monitor: *monitor, Checks: checks, Uptime24h: UptimePercent(ultimas)}, nil
}

// Save crea o actualiza según venga o no un id.
func (s *Service) Save(ctx context.Context, id string, in MonitorInput) (kernel.Result[Monitor], error) {
	valido := Validate(in)
	if !valido.OK {
		return kernel.Err[Monitor](valido.Error), nil
	}

	var (
		m   *Monitor
		err error
	)
	if id == "" {
		m, err = s.monitors.Create(ctx, valido.Value)
	} else {
		m, err = s.monitors.Update(ctx, id, valido.Value)
	}
	if err != nil {
		return kernel.Result[Monitor]{}, err
	}

	return kernel.Ok(*m), nil
}

func (s *Service) Remove(ctx context.Context, id string) error {
	return s.monitors.Remove(ctx, id)
}

// CheckNow comprueba un monitor al instante, sin esperar a su turno.
func (s *Service) CheckNow(ctx context.Context, id string) (kernel.Result[Monitor], error) {
	monitor, err := s.monitors.Get(ctx, id)
	if err != nil {
		return kernel.Result[Monitor]{}, err
	}
	if monitor == nil {
		return kernel.Err[Monitor]("El monitor ya no existe."), nil
	}

	if _, err := s.runOne(ctx, *monitor, time.Now()); err != nil {
		return kernel.Result[Monitor]{}, err
	}

	actualizado, err := s.monitors.Get(ctx, id)
	if err != nil {
		return kernel.Result[Monitor]{}, err
	}
	if actualizado == nil {
		return kernel.Err[Monitor]("No se pudo releer el monitor."), nil
	}

	return kernel.Ok(*actualizado), nil
}

// RunDueChecks comprueba todos los monitores activos a los que les toca.
// Un monitor que falle no interrumpe a los demás: se anota y se sigue.
func (s *Service) RunDueChecks(ctx context.Context, now time.Time) (RunSummary, error) {
	resumen := RunSummary{Errors: []string{}}

	activos, err := s.monitors.ListActive(ctx)
	if err != nil {
		return resumen, err
	}

	for _, monitor := range activos {
		if !IsDue(monitor, now) {
			continue
		}
		alerta, err := s.runOne(ctx, monitor, now)
		if err != nil {
			resumen.Errors = append(resumen.Errors,
				fmt.Sprintf("%s: %v", monitor.Name, err))
			continue
		}
		resumen.Checked++
		switch alerta {
		case AlertDown:
			resumen.Down++
		case AlertRecovered:
			resumen.Recovered++
		}
	}

	return resumen, nil
}

// runOne comprueba un monitor, guarda el resultado y avisa si hubo cambio.
func (s *Service) runOne(ctx context.Context, monitor Monitor, now time.Time) (Alert, error) {
	probe := s.prober.Probe(ctx, monitor.URL, monitor.Method, monitor.TimeoutMs)

	enLinea := IsUp(monitor.ExpectedStatus, probe)
	estado, alerta := Transition(monitor.LastStatus, enLinea)

	fallos := monitor.ConsecutiveFailures + 1
	if enLinea {
		fallos = 0
	}

	patch := CheckPatch{
		LastStatus:          estado,
		LastCheckedAt:       now,
		LastResponseMs:      probe.ResponseMs,
		LastStatusCode:      probe.StatusCode,
		LastError:           probe.Error,
		ConsecutiveFailures: fallos,
	}
	if err := s.monitors.ApplyCheck(ctx, monitor.ID, patch); err != nil {
		return AlertNone, err
	}

	registro := Check{
		OK:         enLinea,
		StatusCode: probe.StatusCode,
		ResponseMs: probe.ResponseMs,
		Error:      probe.Error,
		CheckedAt:  now,
	}
	if err := s.checks.Add(ctx, monitor.ID, registro); err != nil {
		return AlertNone, err
	}

	if alerta != AlertNone && monitor.NotifyOnFailure && s.notifier != nil {
		// Que falle el aviso no invalida la comprobación, que ya está guardada.
		_ = s.notifier.Send(ctx, alertMessage(monitor, alerta, probe))
	}

	return alerta, nil
}

func alertMessage(m Monitor, alerta Alert, probe ProbeResult) string {
	if alerta == AlertRecovered {
		msg := fmt.Sprintf("✅ *%s* se ha recuperado\n%s", m.Name, m.URL)
		if probe.StatusCode != nil {
			msg += fmt.Sprintf("\nHTTP %d", *probe.StatusCode)
		}
		if probe.ResponseMs != nil {
			msg += fmt.Sprintf(" · %d ms", *probe.ResponseMs)
		}
		return msg
	}

	motivo := "sin respuesta"
	switch {
	case probe.Error != nil:
		motivo = *probe.Error
	case probe.StatusCode != nil:
		motivo = fmt.Sprintf("HTTP %d", *probe.StatusCode)
	}

	return fmt.Sprintf("🔴 *%s* está caído\n%s\nMotivo: %s", m.Name, m.URL, motivo)
}
