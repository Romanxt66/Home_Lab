package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// TelegramSender habla con la API de bots de Telegram.
// https://core.telegram.org/bots/api#sendmessage
type TelegramSender struct {
	client *http.Client
}

func NewTelegramSender() *TelegramSender {
	return &TelegramSender{client: &http.Client{Timeout: 15 * time.Second}}
}

var (
	reChatNoEncontrado = regexp.MustCompile(`(?i)chat not found`)
	reNoAutorizado     = regexp.MustCompile(`(?i)unauthorized`)
)

func (t *TelegramSender) Send(ctx context.Context, provider Provider, recipient, credential, message string) kernel.Result[struct{}] {
	if provider != ProviderTelegram {
		return kernel.Err[struct{}](fmt.Sprintf("Proveedor no soportado: %s", provider))
	}

	cuerpo, err := json.Marshal(map[string]any{
		"chat_id":                  recipient,
		"text":                     message,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return kernel.Err[struct{}]("No se pudo preparar el mensaje.")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", credential)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(cuerpo))
	if err != nil {
		return kernel.Err[struct{}]("No se pudo preparar la petición.")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return kernel.Err[struct{}]("No se pudo contactar a Telegram: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return kernel.Ok(struct{}{})
	}

	// Telegram responde { ok:false, error_code, description }. El `description`
	// es lo único aprovechable; se traduce a algo accionable.
	var respuesta struct {
		Description string `json:"description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&respuesta)

	descripcion := respuesta.Description
	if descripcion == "" {
		descripcion = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	switch {
	case reChatNoEncontrado.MatchString(descripcion):
		return kernel.Err[struct{}](
			"Telegram: chat no encontrado. Envía primero un mensaje a tu bot para " +
				"que pueda escribirte, y verifica el chat ID.")
	case reNoAutorizado.MatchString(descripcion):
		return kernel.Err[struct{}](
			"Telegram: el bot token no es válido. Créalo con @BotFather y cópialo exacto.")
	default:
		return kernel.Err[struct{}]("Telegram rechazó el envío: " + descripcion)
	}
}

// ServicioNotifier adapta el servicio de notificaciones al puerto que espera
// uptime. Así uptime no conoce este paquete más que por su interfaz.
type ServicioNotifier struct {
	svc *Service
}

func NewServicioNotifier(svc *Service) *ServicioNotifier {
	return &ServicioNotifier{svc: svc}
}

func (n *ServicioNotifier) Send(ctx context.Context, message string) error {
	res := n.svc.Send(ctx, message)
	if !res.OK {
		return fmt.Errorf("%s", res.Error)
	}
	return nil
}
