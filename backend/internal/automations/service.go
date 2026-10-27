package automations

import (
	"context"
	"log"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Repo guarda las reglas y su historial.
type Repo interface {
	List(ctx context.Context) ([]Regla, error)
	ActivasPara(ctx context.Context, d Disparador) ([]Regla, error)
	Create(ctx context.Context, r Regla) (*Regla, error)
	Update(ctx context.Context, r Regla) (*Regla, error)
	Remove(ctx context.Context, id string) error
	Anotar(ctx context.Context, reglaID string, ok bool, fallo *string) error
	Ejecuciones(ctx context.Context, reglaID string, limite int) ([]Ejecucion, error)
}

// Avisador manda un mensaje de Telegram.
type Avisador interface {
	Send(ctx context.Context, mensaje string) error
}

// Agenda crea eventos en el calendario.
type Agenda interface {
	CrearEvento(ctx context.Context, titulo string, descripcion *string, cuando time.Time) error
}

// Webhook llama a una URL de fuera.
type Webhook interface {
	Post(ctx context.Context, url string, cuerpo map[string]string) error
}

// Rellenador sustituye los {{huecos}} de las plantillas. Se recibe como
// función para no tener que depender del módulo de correo, que es donde
// vive esa lógica.
type Rellenador func(texto string, valores map[string]string) string

type Service struct {
	repo     Repo
	avisador Avisador
	agenda   Agenda
	webhook  Webhook
	rellenar Rellenador
}

func NewService(repo Repo, avisador Avisador, agenda Agenda, webhook Webhook, rellenar Rellenador) *Service {
	return &Service{repo: repo, avisador: avisador, agenda: agenda, webhook: webhook, rellenar: rellenar}
}

func (s *Service) List(ctx context.Context) ([]Regla, error) {
	return s.repo.List(ctx)
}

func (s *Service) Ejecuciones(ctx context.Context, reglaID string, limite int) ([]Ejecucion, error) {
	if limite <= 0 || limite > 200 {
		limite = 20
	}
	return s.repo.Ejecuciones(ctx, reglaID, limite)
}

func (s *Service) Guardar(ctx context.Context, r Regla) (kernel.Result[Regla], error) {
	v := ValidarRegla(r)
	if !v.OK {
		return kernel.Err[Regla](v.Error), nil
	}

	var guardada *Regla
	var err error
	if v.Value.ID == "" {
		guardada, err = s.repo.Create(ctx, v.Value)
	} else {
		guardada, err = s.repo.Update(ctx, v.Value)
	}
	if err != nil {
		return kernel.Result[Regla]{}, err
	}
	if guardada == nil {
		return kernel.Err[Regla]("Esa regla ya no existe."), nil
	}

	return kernel.Ok(*guardada), nil
}

func (s *Service) Borrar(ctx context.Context, id string) error {
	return s.repo.Remove(ctx, id)
}

// Disparar lanza las reglas activas de ese suceso.
//
// No devuelve error a propósito: lo llaman otros módulos en mitad de cosas
// importantes —guardar una transacción, entrar al sistema, un tic del
// reloj—, y que una regla mal configurada tumbe esas operaciones sería
// mucho peor que no ejecutarla. El fallo queda anotado en el historial de la
// regla, que es donde se puede ver y corregir.
func (s *Service) Disparar(ctx context.Context, d Disparador, valores map[string]string) {
	reglas, err := s.repo.ActivasPara(ctx, d)
	if err != nil {
		log.Printf("automatizaciones: no se pudieron leer las reglas de %s: %v", d, err)
		return
	}

	for _, regla := range reglas {
		fallo := s.correrAcciones(ctx, regla, d, valores)

		if fallo != nil {
			log.Printf("automatizaciones: la regla %q falló: %s", regla.Name, *fallo)
		}
		if err := s.repo.Anotar(ctx, regla.ID, fallo == nil, fallo); err != nil {
			log.Printf("automatizaciones: no se pudo anotar la ejecución: %v", err)
		}
	}
}

// correrAcciones ejecuta las acciones de una regla en orden y para en la
// primera que falle: si la segunda depende de la primera, seguir sería peor.
func (s *Service) correrAcciones(ctx context.Context, r Regla, d Disparador, valores map[string]string) *string {
	for _, accion := range r.Actions {
		if err := s.correrUna(ctx, accion, d, valores); err != nil {
			msg := err.Error()
			return &msg
		}
	}
	return nil
}

func (s *Service) correrUna(ctx context.Context, a Accion, d Disparador, valores map[string]string) error {
	switch a.Type {
	case TelegramAccion:
		return s.avisador.Send(ctx, s.rellenar(a.Config["message"], valores))

	case EventoAccion:
		var descripcion *string
		if texto := a.Config["description"]; texto != "" {
			rellena := s.rellenar(texto, valores)
			descripcion = &rellena
		}
		cuando := time.Now().AddDate(0, 0, DiasDesdeHoy(a))
		return s.agenda.CrearEvento(ctx, s.rellenar(a.Config["title"], valores), descripcion, cuando)

	case WebhookAccion:
		cuerpo := map[string]string{"trigger": string(d)}
		for k, v := range valores {
			cuerpo[k] = v
		}
		return s.webhook.Post(ctx, a.Config["webhookUrl"], cuerpo)
	}

	return errAccionDesconocida
}

var errAccionDesconocida = errorSimple("ese tipo de acción no existe")

type errorSimple string

func (e errorSimple) Error() string { return string(e) }
