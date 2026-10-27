// Package notifications manda los avisos del sistema. Hoy solo por Telegram,
// pero el proveedor es un dato: añadir otro no toca a quien lo usa.
package notifications

import (
	"regexp"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

type Provider string

const ProviderTelegram Provider = "telegram"

// Config es la configuración de envío. La credencial NUNCA sale al navegador:
// para eso está el DTO, que solo dice si existe.
type Config struct {
	ID         string
	Provider   Provider
	Recipient  string // Telegram: chat_id, puede ser negativo en grupos
	Credential string // Telegram: bot_token
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

var (
	// Un bot token de Telegram es <dígitos>:<al menos 20 caracteres>.
	reToken  = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]{20,}$`)
	reChatID = regexp.MustCompile(`^-?\d+$`)
)

// ValidarToken comprueba la forma del bot token, no que funcione.
func ValidarToken(token string) bool {
	return reToken.MatchString(strings.TrimSpace(token))
}

// NormalizarChatID valida y limpia el identificador del chat.
func NormalizarChatID(crudo string) kernel.Result[string] {
	limpio := strings.TrimSpace(crudo)
	if !reChatID.MatchString(limpio) {
		return kernel.Err[string]("El chat ID debe ser un número (puede ser negativo).")
	}
	return kernel.Ok(limpio)
}
