package n8n

import (
	"context"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Credenciales es lo que necesita el cliente para hablar con n8n.
type Credenciales struct {
	BaseURL string
	APIKey  string
}

// Cliente habla con la API pública de n8n.
type Cliente interface {
	Ping(ctx context.Context, c Credenciales) kernel.Result[struct{}]
	ListarWorkflows(ctx context.Context, c Credenciales) kernel.Result[[]Workflow]
	Activar(ctx context.Context, c Credenciales, id string, activo bool) kernel.Result[struct{}]
}

// ConfigRepo guarda la conexión.
type ConfigRepo interface {
	GetActive(ctx context.Context) (*Config, error)
	Upsert(ctx context.Context, baseURL, apiKey string) (*Config, error)
	Remove(ctx context.Context, id string) error
}

type Service struct {
	cliente Cliente
	configs ConfigRepo
}

func NewService(cliente Cliente, configs ConfigRepo) *Service {
	return &Service{cliente: cliente, configs: configs}
}

func (s *Service) Config(ctx context.Context) (*Config, error) {
	return s.configs.GetActive(ctx)
}

// Conectar valida los datos y comprueba que funcionan ANTES de guardarlos.
// Guardar una credencial que no sirve solo retrasaría el error hasta la
// primera vez que alguien intentara usarla.
func (s *Service) Conectar(ctx context.Context, baseURL, apiKey string) (kernel.Result[Config], error) {
	url := ValidarBaseURL(baseURL)
	if !url.OK {
		return kernel.Err[Config](url.Error), nil
	}
	clave := ValidarAPIKey(apiKey)
	if !clave.OK {
		return kernel.Err[Config](clave.Error), nil
	}

	cred := Credenciales{BaseURL: url.Value, APIKey: clave.Value}
	if ping := s.cliente.Ping(ctx, cred); !ping.OK {
		return kernel.Err[Config](ping.Error), nil
	}

	guardada, err := s.configs.Upsert(ctx, cred.BaseURL, cred.APIKey)
	if err != nil {
		return kernel.Result[Config]{}, err
	}
	return kernel.Ok(*guardada), nil
}

func (s *Service) Desconectar(ctx context.Context) error {
	cfg, err := s.configs.GetActive(ctx)
	if err != nil || cfg == nil {
		return err
	}
	return s.configs.Remove(ctx, cfg.ID)
}

// credenciales resuelve la conexión activa. Si no hay, lo dice con una
// instrucción concreta en vez de un error técnico.
func (s *Service) credenciales(ctx context.Context) (Credenciales, string, error) {
	cfg, err := s.configs.GetActive(ctx)
	if err != nil {
		return Credenciales{}, "", err
	}
	if cfg == nil {
		return Credenciales{}, "No hay conexión con n8n. Conéctala primero.", nil
	}
	return Credenciales{BaseURL: cfg.BaseURL, APIKey: cfg.APIKey}, "", nil
}

func (s *Service) ListarWorkflows(ctx context.Context) (kernel.Result[[]Workflow], error) {
	cred, falta, err := s.credenciales(ctx)
	if err != nil {
		return kernel.Result[[]Workflow]{}, err
	}
	if falta != "" {
		return kernel.Err[[]Workflow](falta), nil
	}
	return s.cliente.ListarWorkflows(ctx, cred), nil
}

func (s *Service) Activar(ctx context.Context, id string, activo bool) (kernel.Result[struct{}], error) {
	cred, falta, err := s.credenciales(ctx)
	if err != nil {
		return kernel.Result[struct{}]{}, err
	}
	if falta != "" {
		return kernel.Err[struct{}](falta), nil
	}
	return s.cliente.Activar(ctx, cred, id, activo), nil
}
