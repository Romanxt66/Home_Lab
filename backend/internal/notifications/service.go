package notifications

import (
	"context"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ConfigRepo guarda la configuración de notificaciones.
type ConfigRepo interface {
	GetActive(ctx context.Context) (*Config, error)
	Save(ctx context.Context, c Config) (*Config, error)
	Remove(ctx context.Context, id string) error
}

// Sender entrega el mensaje al proveedor.
type Sender interface {
	Send(ctx context.Context, provider Provider, recipient, credential, message string) kernel.Result[struct{}]
}

// Service decide a quién y cómo se avisa.
type Service struct {
	repo   ConfigRepo
	sender Sender
}

func NewService(repo ConfigRepo, sender Sender) *Service {
	return &Service{repo: repo, sender: sender}
}

// Send es el caso de uso que usan los demás módulos (uptime, automatizaciones).
// Si no hay configuración activa lo dice con un mensaje accionable en vez de
// fallar en silencio.
func (s *Service) Send(ctx context.Context, message string) kernel.Result[struct{}] {
	if strings.TrimSpace(message) == "" {
		return kernel.Err[struct{}]("El mensaje está vacío.")
	}

	config, err := s.repo.GetActive(ctx)
	if err != nil {
		return kernel.Err[struct{}]("No se pudo leer la configuración de avisos.")
	}
	if config == nil {
		return kernel.Err[struct{}](
			"No hay una configuración de notificaciones activa. Configúrala en Notificaciones.")
	}
	if config.Credential == "" {
		return kernel.Err[struct{}]("La configuración no tiene credencial.")
	}
	if config.Recipient == "" {
		return kernel.Err[struct{}]("La configuración no tiene destinatario.")
	}

	return s.sender.Send(ctx, config.Provider, config.Recipient, config.Credential, message)
}

// Active devuelve la configuración vigente, si la hay.
func (s *Service) Active(ctx context.Context) (*Config, error) {
	return s.repo.GetActive(ctx)
}

// SaveTelegram valida y guarda la configuración de Telegram.
// Si no llega credencial nueva, se conserva la que ya había: así se puede
// cambiar el chat sin tener que volver a escribir el token.
func (s *Service) SaveTelegram(ctx context.Context, id, recipient, credential string) (kernel.Result[Config], error) {
	chat := NormalizarChatID(recipient)
	if !chat.OK {
		return kernel.Err[Config](chat.Error), nil
	}

	credential = strings.TrimSpace(credential)
	if credential == "" {
		actual, err := s.repo.GetActive(ctx)
		if err != nil {
			return kernel.Result[Config]{}, err
		}
		if actual == nil || actual.Credential == "" {
			return kernel.Err[Config]("Falta el bot token de Telegram."), nil
		}
		credential = actual.Credential
		if id == "" {
			id = actual.ID
		}
	} else if !ValidarToken(credential) {
		return kernel.Err[Config](
			"El bot token no tiene el formato correcto (debe ser como 123456:AA...)."), nil
	}

	guardado, err := s.repo.Save(ctx, Config{
		ID:         id,
		Provider:   ProviderTelegram,
		Recipient:  chat.Value,
		Credential: credential,
		Active:     true,
	})
	if err != nil {
		return kernel.Result[Config]{}, err
	}

	return kernel.Ok(*guardado), nil
}

func (s *Service) Remove(ctx context.Context, id string) error {
	return s.repo.Remove(ctx, id)
}

// Test manda un mensaje de prueba para comprobar la configuración.
func (s *Service) Test(ctx context.Context) kernel.Result[struct{}] {
	return s.Send(ctx, "✅ Prueba desde el Home Lab. Si lees esto, los avisos funcionan.")
}
