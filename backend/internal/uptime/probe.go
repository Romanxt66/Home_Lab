package uptime

import (
	"context"
	"net/http"
	"time"
)

// HTTPProber hace la petición real contra la URL vigilada.
type HTTPProber struct {
	client *http.Client
}

func NewHTTPProber() *HTTPProber {
	return &HTTPProber{
		client: &http.Client{
			// No seguir redirecciones: un 301 es información, no un fallo, y
			// el usuario puede querer exigir un 200 exacto.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Probe devuelve el resultado sin lanzar errores: un fallo de red es un dato
// del monitor ("está caído"), no una avería del programa.
func (p *HTTPProber) Probe(ctx context.Context, url string, method Method, timeoutMs int) ProbeResult {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, string(method), url, nil)
	if err != nil {
		return fallo("URL inválida: " + err.Error())
	}
	req.Header.Set("User-Agent", "Home_Lab-uptime/1.0")

	inicio := time.Now()
	resp, err := p.client.Do(req)
	transcurrido := int(time.Since(inicio).Milliseconds())

	if err != nil {
		// Se distingue el timeout porque es el caso más común y el mensaje
		// genérico de Go no se entiende fuera de un terminal.
		if ctx.Err() == context.DeadlineExceeded {
			r := fallo("No respondió a tiempo")
			r.ResponseMs = &transcurrido
			return r
		}
		r := fallo(err.Error())
		r.ResponseMs = &transcurrido
		return r
	}
	defer resp.Body.Close()

	codigo := resp.StatusCode
	return ProbeResult{OK: true, StatusCode: &codigo, ResponseMs: &transcurrido}
}

func fallo(mensaje string) ProbeResult {
	return ProbeResult{OK: false, Error: &mensaje}
}

// LogNotifier es el avisador provisional: deja el mensaje en el registro.
// Se sustituirá al portar el módulo de notificaciones.
type LogNotifier struct {
	Log func(format string, args ...any)
}

func (n *LogNotifier) Send(_ context.Context, message string) error {
	if n.Log != nil {
		n.Log("aviso de uptime: %s", message)
	}
	return nil
}
