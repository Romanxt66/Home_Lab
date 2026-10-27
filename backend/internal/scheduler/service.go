package scheduler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// JobRepo guarda las tareas.
type JobRepo interface {
	List(ctx context.Context) ([]Job, error)
	ListEnabled(ctx context.Context) ([]Job, error)
	Get(ctx context.Context, id string) (*Job, error)
	Create(ctx context.Context, j Job) (*Job, error)
	Update(ctx context.Context, j Job) (*Job, error)
	Remove(ctx context.Context, id string) error
}

// RunRepo guarda el historial de ejecuciones.
type RunRepo interface {
	Record(ctx context.Context, jobID, estado string, salida *string) error
	ListRecent(ctx context.Context, limite int) ([]Run, error)
}

// Ejecutor hace el trabajo de una acción concreta. El planificador no sabe
// nada de correo ni de scraping: solo despacha y anota el resultado.
type Ejecutor func(ctx context.Context, params json.RawMessage) kernel.Result[string]

type Service struct {
	jobs       JobRepo
	runs       RunRepo
	ejecutores map[Accion]Ejecutor
}

func NewService(jobs JobRepo, runs RunRepo, ejecutores map[Accion]Ejecutor) *Service {
	return &Service{jobs: jobs, runs: runs, ejecutores: ejecutores}
}

func (s *Service) List(ctx context.Context) ([]Job, error) {
	return s.jobs.List(ctx)
}

func (s *Service) Runs(ctx context.Context, limite int) ([]Run, error) {
	if limite <= 0 || limite > 200 {
		limite = 20
	}
	return s.runs.ListRecent(ctx, limite)
}

// Save crea o actualiza según venga o no el id.
func (s *Service) Save(ctx context.Context, j Job) (kernel.Result[Job], error) {
	if v := ValidarJob(j.Name, j.Cron, j.Action); !v.OK {
		return kernel.Err[Job](v.Error), nil
	}

	// Los parámetros se guardan como texto, pero tienen que ser JSON válido:
	// si no, la tarea fallaría en cada ejecución y el error aparecería en el
	// historial en vez de aquí, que es donde se puede corregir.
	if strings.TrimSpace(j.Params) == "" {
		j.Params = "{}"
	}
	if !json.Valid([]byte(j.Params)) {
		return kernel.Err[Job]("Los parámetros no son un JSON válido."), nil
	}

	j.Name = strings.TrimSpace(j.Name)
	j.Cron = strings.TrimSpace(j.Cron)

	var guardado *Job
	var err error
	if j.ID == "" {
		guardado, err = s.jobs.Create(ctx, j)
	} else {
		guardado, err = s.jobs.Update(ctx, j)
	}
	if err != nil {
		return kernel.Result[Job]{}, err
	}
	if guardado == nil {
		return kernel.Err[Job]("Esa tarea ya no existe."), nil
	}

	return kernel.Ok(*guardado), nil
}

func (s *Service) Toggle(ctx context.Context, id string, activa bool) (kernel.Result[Job], error) {
	job, err := s.jobs.Get(ctx, id)
	if err != nil {
		return kernel.Result[Job]{}, err
	}
	if job == nil {
		return kernel.Err[Job]("Esa tarea ya no existe."), nil
	}

	job.Enabled = activa
	actualizado, err := s.jobs.Update(ctx, *job)
	if err != nil {
		return kernel.Result[Job]{}, err
	}
	return kernel.Ok(*actualizado), nil
}

func (s *Service) Remove(ctx context.Context, id string) error {
	return s.jobs.Remove(ctx, id)
}

// Ejecutar lanza una tarea y anota el resultado, salga bien o mal.
func (s *Service) Ejecutar(ctx context.Context, id string) (kernel.Result[string], error) {
	job, err := s.jobs.Get(ctx, id)
	if err != nil {
		return kernel.Result[string]{}, err
	}
	if job == nil {
		return kernel.Err[string]("Esa tarea ya no existe."), nil
	}

	ejecutor, hay := s.ejecutores[job.Action]
	if !hay {
		return kernel.Err[string]("No hay quien ejecute la acción «" + string(job.Action) + "»."), nil
	}

	if !json.Valid([]byte(job.Params)) {
		const msg = "Los parámetros de la tarea no son un JSON válido."
		s.anotar(ctx, job.ID, EstadoError, msg)
		return kernel.Err[string](msg), nil
	}

	res := ejecutor(ctx, json.RawMessage(job.Params))

	estado, salida := EstadoOK, res.Value
	if !res.OK {
		estado, salida = EstadoError, res.Error
	}
	s.anotar(ctx, job.ID, estado, salida)

	return res, nil
}

// EjecutarPendientes lanza las tareas activas a las que les toca ahora.
func (s *Service) EjecutarPendientes(ctx context.Context, ahora time.Time) ([]string, error) {
	activas, err := s.jobs.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}

	var lanzadas []string
	for _, job := range activas {
		if !CronCoincide(job.Cron, ahora) {
			continue
		}
		if _, err := s.Ejecutar(ctx, job.ID); err != nil {
			// Que una tarea falle no puede impedir que corran las demás.
			continue
		}
		lanzadas = append(lanzadas, job.ID)
	}

	return lanzadas, nil
}

// anotar guarda el resultado. Si falla el registro no se interrumpe nada: la
// tarea ya se ejecutó y perder la anotación es menos grave que fingir que no
// se ejecutó.
func (s *Service) anotar(ctx context.Context, jobID, estado, salida string) {
	var puntero *string
	if salida != "" {
		puntero = &salida
	}
	_ = s.runs.Record(ctx, jobID, estado, puntero)
}
