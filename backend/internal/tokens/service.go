package tokens

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Repo guarda las huellas de los tokens.
type Repo interface {
	Crear(ctx context.Context, t Token, huella string) error
	Listar(ctx context.Context, duenoID string) ([]Token, error)
	ContarActivos(ctx context.Context, duenoID string) (int, error)
	// Revocar marca el token como revocado; false si no existe o no es suyo.
	Revocar(ctx context.Context, duenoID, id string, cuando time.Time) (bool, error)
	// BuscarPorHuella devuelve quién hay detrás, o nil si el token no existe,
	// está revocado o su dueño ya no tiene la cuenta aprobada.
	BuscarPorHuella(ctx context.Context, huella string) (*Identidad, error)
	MarcarUso(ctx context.Context, id string, cuando time.Time) error
}

type Service struct {
	repo  Repo
	ahora func() time.Time
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo, ahora: func() time.Time { return time.Now().UTC() }}
}

// Creado es un token recién hecho: el único momento en que se ve el secreto.
type Creado struct {
	Token Token
	Claro string
}

func (s *Service) Crear(ctx context.Context, duenoID, nombre, alcance string) (kernel.Result[Creado], error) {
	n := ValidarNombre(nombre)
	if !n.OK {
		return kernel.Err[Creado](n.Error), nil
	}
	a := ValidarAlcance(alcance)
	if !a.OK {
		return kernel.Err[Creado](a.Error), nil
	}
	activos, err := s.repo.ContarActivos(ctx, duenoID)
	if err != nil {
		return kernel.Result[Creado]{}, err
	}
	if activos >= MaxActivos {
		return kernel.Err[Creado]("Ya tienes 10 tokens activos. Revoca alguno que no uses antes de crear otro."), nil
	}

	claro, huella, visible, err := Generar()
	if err != nil {
		return kernel.Result[Creado]{}, err
	}
	t := Token{
		ID: uuid.NewString(), DuenoID: duenoID, Nombre: n.Value, Alcance: a.Value,
		Prefijo: visible, CreadoEn: s.ahora(),
	}
	if err := s.repo.Crear(ctx, t, huella); err != nil {
		return kernel.Result[Creado]{}, err
	}
	return kernel.Ok(Creado{Token: t, Claro: claro}), nil
}

func (s *Service) Listar(ctx context.Context, duenoID string) ([]Token, error) {
	return s.repo.Listar(ctx, duenoID)
}

// Revocar devuelve false si el token no existe o es de otra persona; no se
// distingue, para no revelar qué ids existen.
func (s *Service) Revocar(ctx context.Context, duenoID, id string) (bool, error) {
	return s.repo.Revocar(ctx, duenoID, id, s.ahora())
}

// Autenticar resuelve un token en claro a la persona que hay detrás. Un token
// que no existe, está revocado o tiene mala forma da nil, sin error: es una
// respuesta normal («no eres nadie»), no un fallo técnico.
func (s *Service) Autenticar(ctx context.Context, claro string) (*Identidad, error) {
	if !FormaValida(claro) {
		return nil, nil
	}
	id, err := s.repo.BuscarPorHuella(ctx, Huella(claro))
	if err != nil || id == nil {
		return nil, err
	}
	// Anotar el uso no debe tumbar la petición si falla, pero tampoco pasar
	// desapercibido: quedó así roto sin que nadie lo viera.
	if err := s.repo.MarcarUso(ctx, id.TokenID, s.ahora()); err != nil {
		log.Printf("tokens: no se pudo anotar el uso de %s: %v", id.TokenID, err)
	}
	return id, nil
}
