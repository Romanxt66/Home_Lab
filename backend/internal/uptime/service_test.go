package uptime

import (
	"context"
	"testing"
	"time"
)

// ── Dobles ─────────────────────────────────────────────────────────────────

type monitorsFalsos struct {
	activos []Monitor
	parches map[string]CheckPatch
}

func (m *monitorsFalsos) List(context.Context) ([]Monitor, error) { return m.activos, nil }
func (m *monitorsFalsos) ListActive(context.Context) ([]Monitor, error) {
	return m.activos, nil
}
func (m *monitorsFalsos) Get(context.Context, string) (*Monitor, error) { return nil, nil }
func (m *monitorsFalsos) Create(context.Context, MonitorInput) (*Monitor, error) {
	return nil, nil
}
func (m *monitorsFalsos) Update(context.Context, string, MonitorInput) (*Monitor, error) {
	return nil, nil
}
func (m *monitorsFalsos) Remove(context.Context, string) error { return nil }
func (m *monitorsFalsos) ApplyCheck(_ context.Context, id string, p CheckPatch) error {
	if m.parches == nil {
		m.parches = map[string]CheckPatch{}
	}
	m.parches[id] = p
	return nil
}

type checksFalsos struct{ guardados int }

func (c *checksFalsos) Add(context.Context, string, Check) error { c.guardados++; return nil }
func (c *checksFalsos) ListForMonitor(context.Context, string, int) ([]Check, error) {
	return nil, nil
}

type proberFalso struct{ resultado ProbeResult }

func (p *proberFalso) Probe(context.Context, string, Method, int) ProbeResult {
	return p.resultado
}

type notifierFalso struct{ mensajes []string }

func (n *notifierFalso) Send(_ context.Context, m string) error {
	n.mensajes = append(n.mensajes, m)
	return nil
}

type automatizadorFalso struct {
	disparados []string
	valores    []map[string]string
}

func (a *automatizadorFalso) Disparar(_ context.Context, d string, v map[string]string) {
	a.disparados = append(a.disparados, d)
	a.valores = append(a.valores, v)
}

func punt[T any](v T) *T { return &v }

// monitorQueSeCae: estaba en línea y la sonda ahora falla.
func monitorQueSeCae() Monitor {
	return Monitor{
		ID: "m1", Name: "La web", URL: "https://x.com",
		Method: MethodGET, ExpectedStatus: 200, IntervalSeconds: 60,
		Active: true, NotifyOnFailure: true, LastStatus: StatusUp,
	}
}

func sondaCaida() ProbeResult {
	return ProbeResult{Error: punt("sin respuesta"), ResponseMs: punt(0)}
}

func sondaOK() ProbeResult {
	// OK a true: sin eso IsUp la da por caída, por mucho que el código sea 200.
	return ProbeResult{OK: true, StatusCode: punt(200), ResponseMs: punt(42)}
}

// ── Las pruebas ────────────────────────────────────────────────────────────

func TestUnaCaidaAvisaYDisparaLaAutomatizacion(t *testing.T) {
	monitor := monitorQueSeCae()
	monitors := &monitorsFalsos{activos: []Monitor{monitor}}
	avisos := &notifierFalso{}
	autos := &automatizadorFalso{}

	svc := NewService(monitors, &checksFalsos{}, &proberFalso{resultado: sondaCaida()}, avisos, autos)
	if _, err := svc.runOne(context.Background(), monitor, time.Now()); err != nil {
		t.Fatalf("falló: %v", err)
	}

	if len(avisos.mensajes) != 1 {
		t.Errorf("mandó %d avisos, quería 1", len(avisos.mensajes))
	}
	if len(autos.disparados) != 1 || autos.disparados[0] != "uptime_down" {
		t.Fatalf("disparó %v", autos.disparados)
	}
	if autos.valores[0]["monitor"] != "La web" || autos.valores[0]["url"] != "https://x.com" {
		t.Errorf("variables = %v", autos.valores[0])
	}
	if autos.valores[0]["motivo"] != "sin respuesta" {
		t.Errorf("motivo = %q", autos.valores[0]["motivo"])
	}
}

func TestUnaRecuperacionDisparaSuPropioEvento(t *testing.T) {
	monitor := monitorQueSeCae()
	monitor.LastStatus = StatusDown

	autos := &automatizadorFalso{}
	svc := NewService(&monitorsFalsos{}, &checksFalsos{}, &proberFalso{resultado: sondaOK()},
		&notifierFalso{}, autos)

	if _, err := svc.runOne(context.Background(), monitor, time.Now()); err != nil {
		t.Fatalf("falló: %v", err)
	}

	if len(autos.disparados) != 1 || autos.disparados[0] != "uptime_recovered" {
		t.Errorf("disparó %v", autos.disparados)
	}
}

// Sin cambio de estado no hay alerta, así que tampoco automatización: una
// web caída desde hace una hora no debe disparar la regla cada minuto.
func TestSinCambioDeEstadoNoSeDisparaNada(t *testing.T) {
	monitor := monitorQueSeCae()
	monitor.LastStatus = StatusDown

	avisos := &notifierFalso{}
	autos := &automatizadorFalso{}
	svc := NewService(&monitorsFalsos{}, &checksFalsos{}, &proberFalso{resultado: sondaCaida()},
		avisos, autos)

	if _, err := svc.runOne(context.Background(), monitor, time.Now()); err != nil {
		t.Fatalf("falló: %v", err)
	}

	if len(autos.disparados) != 0 {
		t.Errorf("disparó %v sin que cambiara el estado", autos.disparados)
	}
	if len(avisos.mensajes) != 0 {
		t.Errorf("avisó sin que cambiara el estado: %v", avisos.mensajes)
	}
}

// Diferencia deliberada con el Lab: el aviso de Telegram respeta
// notifyOnFailure, pero las automatizaciones saltan igual. Son dos cosas
// distintas, y se puede querer reaccionar sin recibir el mensaje.
func TestLasAutomatizacionesSaltanAunqueElAvisoEsteApagado(t *testing.T) {
	monitor := monitorQueSeCae()
	monitor.NotifyOnFailure = false

	avisos := &notifierFalso{}
	autos := &automatizadorFalso{}
	svc := NewService(&monitorsFalsos{}, &checksFalsos{}, &proberFalso{resultado: sondaCaida()},
		avisos, autos)

	if _, err := svc.runOne(context.Background(), monitor, time.Now()); err != nil {
		t.Fatalf("falló: %v", err)
	}

	if len(avisos.mensajes) != 0 {
		t.Errorf("avisó con notifyOnFailure apagado: %v", avisos.mensajes)
	}
	if len(autos.disparados) != 1 {
		t.Errorf("no disparó la automatización: %v", autos.disparados)
	}
}

// La comprobación debe guardarse aunque no haya a quién avisar.
func TestSinAutomatizadorNiAvisadorSeGuardaIgual(t *testing.T) {
	monitor := monitorQueSeCae()
	monitors := &monitorsFalsos{}
	checks := &checksFalsos{}

	svc := NewService(monitors, checks, &proberFalso{resultado: sondaCaida()}, nil, nil)
	if _, err := svc.runOne(context.Background(), monitor, time.Now()); err != nil {
		t.Fatalf("falló: %v", err)
	}

	if checks.guardados != 1 {
		t.Errorf("guardó %d comprobaciones", checks.guardados)
	}
	if p, hay := monitors.parches["m1"]; !hay || p.LastStatus != StatusDown {
		t.Errorf("no actualizó el estado del monitor: %+v", monitors.parches)
	}
}

func TestMotivoDeLaAlerta(t *testing.T) {
	casos := []struct {
		nombre string
		sonda  ProbeResult
		quiero string
	}{
		{"con error de red", ProbeResult{Error: punt("sin respuesta")}, "sin respuesta"},
		{"con código HTTP", ProbeResult{StatusCode: punt(503)}, "HTTP 503"},
		{"error manda sobre el código", ProbeResult{Error: punt("timeout"), StatusCode: punt(500)}, "timeout"},
		{"sin nada", ProbeResult{}, ""},
		{"error vacío cuenta como sin error", ProbeResult{Error: punt(""), StatusCode: punt(404)}, "HTTP 404"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := motivoDeLaAlerta(c.sonda); got != c.quiero {
				t.Errorf("motivoDeLaAlerta = %q, quería %q", got, c.quiero)
			}
		})
	}
}
