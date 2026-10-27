package calendar

import (
	"context"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Repo guarda los eventos.
type Repo interface {
	ListBetween(ctx context.Context, desde, hasta time.Time) ([]Evento, error)
	Get(ctx context.Context, id string) (*Evento, error)
	Create(ctx context.Context, e Evento) (*Evento, error)
	Update(ctx context.Context, e Evento) (*Evento, error)
	Remove(ctx context.Context, id string) error
	// Candidatos trae los eventos con aviso configurado y sin mandar. El
	// filtro fino lo hace el dominio, que es donde está la regla.
	Candidatos(ctx context.Context, desde time.Time) ([]Evento, error)
	MarcarAvisado(ctx context.Context, id string, cuando time.Time) error
}

// Avisador manda el recordatorio.
type Avisador interface {
	Send(ctx context.Context, mensaje string) error
}

type Service struct {
	repo     Repo
	avisador Avisador
}

func NewService(repo Repo, avisador Avisador) *Service {
	return &Service{repo: repo, avisador: avisador}
}

// Mes devuelve los eventos que se ven en la rejilla de ese mes.
func (s *Service) Mes(ctx context.Context, año, mes int) ([]Evento, error) {
	if mes < 1 || mes > 12 {
		return nil, nil
	}
	desde, hasta := RangoDelMes(año, mes, time.Local)
	return s.repo.ListBetween(ctx, desde, hasta)
}

func (s *Service) Guardar(ctx context.Context, e Evento) (kernel.Result[Evento], error) {
	v := Validar(e)
	if !v.OK {
		return kernel.Err[Evento](v.Error), nil
	}

	var guardado *Evento
	var err error
	if v.Value.ID == "" {
		guardado, err = s.repo.Create(ctx, v.Value)
	} else {
		guardado, err = s.repo.Update(ctx, v.Value)
	}
	if err != nil {
		return kernel.Result[Evento]{}, err
	}
	if guardado == nil {
		return kernel.Err[Evento]("Ese evento ya no existe."), nil
	}

	return kernel.Ok(*guardado), nil
}

func (s *Service) Borrar(ctx context.Context, id string) error {
	return s.repo.Remove(ctx, id)
}

// ResumenAvisos cuenta cómo fue una pasada de recordatorios.
type ResumenAvisos struct {
	Revisados int
	Enviados  int
	Fallidos  int
	Errores   []string
}

// ProcesarAvisos manda los recordatorios que tocan y los marca.
//
// Marcarlos es lo que lo hace idempotente: si el ticker pasa dos veces en el
// mismo minuto, el aviso no sale dos veces. Solo se marca cuando el envío
// salió bien, para que un fallo se reintente en la siguiente pasada.
func (s *Service) ProcesarAvisos(ctx context.Context, ahora time.Time) (ResumenAvisos, error) {
	candidatos, err := s.repo.Candidatos(ctx, ahora.Add(-24*time.Hour))
	if err != nil {
		return ResumenAvisos{}, err
	}

	var resumen ResumenAvisos
	for _, evento := range candidatos {
		if !evento.AvisoPendiente(ahora) {
			continue
		}
		resumen.Revisados++

		if err := s.avisador.Send(ctx, TextoDelAviso(evento)); err != nil {
			resumen.Fallidos++
			resumen.Errores = append(resumen.Errores, evento.Title+": "+err.Error())
			continue
		}

		resumen.Enviados++
		if err := s.repo.MarcarAvisado(ctx, evento.ID, ahora); err != nil {
			// Se mandó pero no se pudo marcar: se avisará otra vez en la
			// siguiente pasada. Es molesto, pero menos grave que no avisar.
			resumen.Errores = append(resumen.Errores,
				evento.Title+": se avisó pero no se pudo marcar")
		}
	}

	return resumen, nil
}

// Crear es lo que usan otros módulos (las automatizaciones) para añadir un
// evento sin pasar por HTTP.
func (s *Service) Crear(ctx context.Context, e Evento) (kernel.Result[Evento], error) {
	e.ID = ""
	return s.Guardar(ctx, e)
}
