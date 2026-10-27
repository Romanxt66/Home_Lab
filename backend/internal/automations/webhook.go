package automations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const tiempoLimite = 15 * time.Second

// HTTPWebhook llama al webhook de n8n con un POST de JSON.
type HTTPWebhook struct {
	client *http.Client
}

func NewHTTPWebhook() *HTTPWebhook {
	return &HTTPWebhook{client: &http.Client{Timeout: tiempoLimite}}
}

func (w *HTTPWebhook) Post(ctx context.Context, destino string, cuerpo map[string]string) error {
	datos, err := json.Marshal(cuerpo)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(datos))
	if err != nil {
		return fmt.Errorf("la URL del webhook no es válida")
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := w.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return fmt.Errorf("el webhook tardó más de %d segundos", int(tiempoLimite.Seconds()))
		}
		return fmt.Errorf("no se pudo llamar al webhook")
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("el webhook respondió con un error (HTTP %d)", res.StatusCode)
	}
	return nil
}
