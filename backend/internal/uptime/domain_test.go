package uptime

import (
	"testing"
	"time"
)

func entradaValida() MonitorInput {
	return MonitorInput{
		Name:            "Mi sitio",
		URL:             "https://ejemplo.com",
		Method:          MethodGET,
		ExpectedStatus:  0,
		IntervalSeconds: 300,
		TimeoutMs:       10000,
		Active:          true,
		NotifyOnFailure: true,
	}
}

func TestValidate(t *testing.T) {
	casos := []struct {
		nombre   string
		ajuste   func(*MonitorInput)
		valido   bool
		contiene string
	}{
		{nombre: "entrada correcta", ajuste: func(*MonitorInput) {}, valido: true},
		{
			nombre:   "nombre vacío",
			ajuste:   func(i *MonitorInput) { i.Name = "   " },
			contiene: "nombre no puede estar vacío",
		},
		{
			nombre:   "url sin esquema",
			ajuste:   func(i *MonitorInput) { i.URL = "ejemplo.com" },
			contiene: "no es válida",
		},
		{
			nombre:   "esquema no permitido",
			ajuste:   func(i *MonitorInput) { i.URL = "ftp://ejemplo.com" },
			contiene: "http:// o https://",
		},
		{
			nombre:   "método no soportado",
			ajuste:   func(i *MonitorInput) { i.Method = "DELETE" },
			contiene: "Método no soportado",
		},
		{
			nombre:   "estado fuera de rango",
			ajuste:   func(i *MonitorInput) { i.ExpectedStatus = 700 },
			contiene: "código de estado esperado",
		},
		{
			nombre:   "intervalo por debajo del mínimo",
			ajuste:   func(i *MonitorInput) { i.IntervalSeconds = 10 },
			contiene: "intervalo mínimo es 30",
		},
		{
			nombre:   "timeout demasiado alto",
			ajuste:   func(i *MonitorInput) { i.TimeoutMs = 90000 },
			contiene: "entre 1s y 60s",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			entrada := entradaValida()
			caso.ajuste(&entrada)

			res := Validate(entrada)
			if res.OK != caso.valido {
				t.Fatalf("OK = %v, se esperaba %v (error: %q)", res.OK, caso.valido, res.Error)
			}
			if !caso.valido && !contiene(res.Error, caso.contiene) {
				t.Errorf("el mensaje %q no menciona %q", res.Error, caso.contiene)
			}
		})
	}
}

func TestValidateNormaliza(t *testing.T) {
	entrada := entradaValida()
	entrada.Name = "  Mi sitio  "
	entrada.URL = "https://ejemplo.com///"

	res := Validate(entrada)
	if !res.OK {
		t.Fatalf("debería ser válida: %s", res.Error)
	}
	if res.Value.Name != "Mi sitio" {
		t.Errorf("nombre = %q, se esperaba sin espacios", res.Value.Name)
	}
	if res.Value.URL != "https://ejemplo.com" {
		t.Errorf("url = %q, se esperaba sin barras finales", res.Value.URL)
	}
}

func TestIsUp(t *testing.T) {
	codigo := func(n int) *int { return &n }

	casos := []struct {
		nombre   string
		esperado int
		probe    ProbeResult
		quiere   bool
	}{
		{"200 con cualquiera", 0, ProbeResult{OK: true, StatusCode: codigo(200)}, true},
		{"301 con cualquiera", 0, ProbeResult{OK: true, StatusCode: codigo(301)}, true},
		{"404 con cualquiera", 0, ProbeResult{OK: true, StatusCode: codigo(404)}, false},
		{"500 con cualquiera", 0, ProbeResult{OK: true, StatusCode: codigo(500)}, false},
		{"código exacto que coincide", 204, ProbeResult{OK: true, StatusCode: codigo(204)}, true},
		{"código exacto que no coincide", 204, ProbeResult{OK: true, StatusCode: codigo(200)}, false},
		{"sin respuesta", 0, ProbeResult{OK: false}, false},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if got := IsUp(caso.esperado, caso.probe); got != caso.quiere {
				t.Errorf("IsUp = %v, se esperaba %v", got, caso.quiere)
			}
		})
	}
}

func TestIsDue(t *testing.T) {
	ahora := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	hace10min := ahora.Add(-10 * time.Minute)
	hace1min := ahora.Add(-1 * time.Minute)

	casos := []struct {
		nombre  string
		monitor Monitor
		quiere  bool
	}{
		{
			"inactivo nunca toca",
			Monitor{Active: false, IntervalSeconds: 60, LastCheckedAt: &hace10min},
			false,
		},
		{
			"sin comprobar todavía, toca",
			Monitor{Active: true, IntervalSeconds: 300},
			true,
		},
		{
			"ya pasó el intervalo",
			Monitor{Active: true, IntervalSeconds: 300, LastCheckedAt: &hace10min},
			true,
		},
		{
			"aún no toca",
			Monitor{Active: true, IntervalSeconds: 300, LastCheckedAt: &hace1min},
			false,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if got := IsDue(caso.monitor, ahora); got != caso.quiere {
				t.Errorf("IsDue = %v, se esperaba %v", got, caso.quiere)
			}
		})
	}
}

func TestTransition(t *testing.T) {
	casos := []struct {
		nombre       string
		previo       Status
		enLinea      bool
		quiereEstado Status
		quiereAlerta Alert
	}{
		{"sigue en línea, sin aviso", StatusUp, true, StatusUp, AlertNone},
		{"sigue caído, sin aviso", StatusDown, false, StatusDown, AlertNone},
		{"se cae, avisa", StatusUp, false, StatusDown, AlertDown},
		{"se recupera, avisa", StatusDown, true, StatusUp, AlertRecovered},
		{"primera vez en línea, sin aviso", StatusUnknown, true, StatusUp, AlertNone},
		{"primera vez caído, sí avisa", StatusUnknown, false, StatusDown, AlertDown},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			estado, alerta := Transition(caso.previo, caso.enLinea)
			if estado != caso.quiereEstado {
				t.Errorf("estado = %v, se esperaba %v", estado, caso.quiereEstado)
			}
			if alerta != caso.quiereAlerta {
				t.Errorf("alerta = %v, se esperaba %v", alerta, caso.quiereAlerta)
			}
		})
	}
}

func TestUptimePercent(t *testing.T) {
	casos := []struct {
		nombre string
		checks []Check
		quiere float64
	}{
		{"sin datos", nil, 0},
		{"todo en línea", []Check{{OK: true}, {OK: true}}, 100},
		{"todo caído", []Check{{OK: false}, {OK: false}}, 0},
		{"la mitad", []Check{{OK: true}, {OK: false}}, 50},
		{"dos de tres, un decimal", []Check{{OK: true}, {OK: true}, {OK: false}}, 66.7},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if got := UptimePercent(caso.checks); got != caso.quiere {
				t.Errorf("UptimePercent = %v, se esperaba %v", got, caso.quiere)
			}
		})
	}
}

func contiene(texto, fragmento string) bool {
	return fragmento == "" || len(texto) >= len(fragmento) &&
		(texto == fragmento || indiceDe(texto, fragmento) >= 0)
}

func indiceDe(texto, fragmento string) int {
	for i := 0; i+len(fragmento) <= len(texto); i++ {
		if texto[i:i+len(fragmento)] == fragmento {
			return i
		}
	}
	return -1
}
