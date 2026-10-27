package github

import (
	"context"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Credenciales es lo que necesita el cliente. Token nulo quiere decir sin
// autenticar: solo repositorios públicos.
type Credenciales struct {
	Username string
	Token    *string
}

// Cliente habla con la API de GitHub.
type Cliente interface {
	Perfil(ctx context.Context, c Credenciales) kernel.Result[Perfil]
	Repos(ctx context.Context, c Credenciales) kernel.Result[[]Repo]
}

// ConfigRepo guarda la conexión.
type ConfigRepo interface {
	GetActive(ctx context.Context) (*Config, error)
	Upsert(ctx context.Context, username string, token *string) (*Config, error)
	Remove(ctx context.Context, id string) error
}

// Resumen es lo que enseña la pantalla: la cuenta y sus repositorios.
type Resumen struct {
	Perfil Perfil
	Repos  []Repo
}

type Service struct {
	configs ConfigRepo
	cliente Cliente
}

func NewService(configs ConfigRepo, cliente Cliente) *Service {
	return &Service{configs: configs, cliente: cliente}
}

func (s *Service) Config(ctx context.Context) (*Config, error) {
	return s.configs.GetActive(ctx)
}

func (s *Service) Guardar(ctx context.Context, username, token string) (kernel.Result[Config], error) {
	u := ValidarUsuario(username)
	if !u.OK {
		return kernel.Err[Config](u.Error), nil
	}

	guardada, err := s.configs.Upsert(ctx, u.Value, NormalizarToken(token))
	if err != nil {
		return kernel.Result[Config]{}, err
	}
	return kernel.Ok(*guardada), nil
}

func (s *Service) Borrar(ctx context.Context, id string) error {
	return s.configs.Remove(ctx, id)
}

// Resumen trae el perfil y los repositorios.
//
// Las dos peticiones van en paralelo: son independientes y hacerlas en serie
// duplicaría la espera sin ganar nada.
func (s *Service) Resumen(ctx context.Context) (kernel.Result[Resumen], error) {
	config, err := s.configs.GetActive(ctx)
	if err != nil {
		return kernel.Result[Resumen]{}, err
	}
	if config == nil {
		return kernel.Err[Resumen](
			"Configura tu usuario de GitHub para ver tus repositorios."), nil
	}

	cred := Credenciales{Username: config.Username, Token: config.Token}

	var perfil kernel.Result[Perfil]
	hecho := make(chan struct{})
	go func() {
		defer close(hecho)
		perfil = s.cliente.Perfil(ctx, cred)
	}()

	repos := s.cliente.Repos(ctx, cred)
	<-hecho

	if !perfil.OK {
		return kernel.Err[Resumen](perfil.Error), nil
	}
	if !repos.OK {
		return kernel.Err[Resumen](repos.Error), nil
	}

	return kernel.Ok(Resumen{Perfil: perfil.Value, Repos: repos.Value}), nil
}
