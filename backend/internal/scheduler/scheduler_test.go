package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

func TestValidarJob(t *testing.T) {
	casos := []struct {
		nombre string
		n      string
		cron   string
		accion Accion
		vale   bool
	}{
		{"todo correcto", "Aviso diario", "0 9 * * *", AccionEmail, true},
		{"scraping", "Precios", "*/30 * * * *", AccionScrape, true},
		{"sin nombre", "", "0 9 * * *", AccionEmail, false},
		{"nombre en blanco", "   ", "0 9 * * *", AccionEmail, false},
		{"cron imposible", "Algo", "99 * * * *", AccionEmail, false},
		{"acción inventada", "Algo", "0 9 * * *", Accion("borrar-todo"), false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res := ValidarJob(c.n, c.cron, c.accion)
			if res.OK != c.vale {
				t.Errorf("OK = %v, quería %v (%s)", res.OK, c.vale, res.Error)
			}
		})
	}
}

// ── El servicio ────────────────────────────────────────────────────────────

type jobsFalsos struct {
	jobs     map[string]*Job
	guardado *Job
	fallo    error
}

func nuevosJobs(js ...Job) *jobsFalsos {
	m := map[string]*Job{}
	for i := range js {
		copia := js[i]
		m[copia.ID] = &copia
	}
	return &jobsFalsos{jobs: m}
}

func (r *jobsFalsos) List(context.Context) ([]Job, error) { return r.todos(), nil }

func (r *jobsFalsos) ListEnabled(context.Context) ([]Job, error) {
	if r.fallo != nil {
		return nil, r.fallo
	}
	var activos []Job
	for _, j := range r.todos() {
		if j.Enabled {
			activos = append(activos, j)
		}
	}
	return activos, nil
}

func (r *jobsFalsos) todos() []Job {
	var salida []Job
	for _, j := range r.jobs {
		salida = append(salida, *j)
	}
	return salida
}

func (r *jobsFalsos) Get(_ context.Context, id string) (*Job, error) {
	if r.fallo != nil {
		return nil, r.fallo
	}
	j, hay := r.jobs[id]
	if !hay {
		return nil, nil
	}
	copia := *j
	return &copia, nil
}

func (r *jobsFalsos) Create(_ context.Context, j Job) (*Job, error) {
	j.ID = "nuevo"
	r.jobs[j.ID] = &j
	r.guardado = &j
	return &j, nil
}

func (r *jobsFalsos) Update(_ context.Context, j Job) (*Job, error) {
	if _, hay := r.jobs[j.ID]; !hay {
		return nil, nil
	}
	r.jobs[j.ID] = &j
	r.guardado = &j
	return &j, nil
}

func (r *jobsFalsos) Remove(_ context.Context, id string) error {
	delete(r.jobs, id)
	return nil
}

type runsFalsos struct {
	anotados []Run
}

func (r *runsFalsos) Record(_ context.Context, jobID, estado string, salida *string) error {
	r.anotados = append(r.anotados, Run{JobID: jobID, Status: estado, Output: salida})
	return nil
}

func (r *runsFalsos) ListRecent(context.Context, int) ([]Run, error) { return r.anotados, nil }

func ejecutorQueVaBien(texto string) Ejecutor {
	return func(context.Context, json.RawMessage) kernel.Result[string] {
		return kernel.Ok(texto)
	}
}

func ejecutorQueFalla(texto string) Ejecutor {
	return func(context.Context, json.RawMessage) kernel.Result[string] {
		return kernel.Err[string](texto)
	}
}

// Unos parámetros que no son JSON harían fallar la tarea en cada ejecución.
// Mejor rechazarlos al guardar, que es donde se pueden corregir.
func TestParametrosNoJSONSeRechazanAlGuardar(t *testing.T) {
	repo := nuevosJobs()
	svc := NewService(repo, &runsFalsos{}, nil)

	res, err := svc.Save(context.Background(), Job{
		Name: "Algo", Cron: "* * * * *", Action: AccionEmail, Params: "{esto no es json",
	})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK {
		t.Error("guardó unos parámetros que no son JSON")
	}
	if repo.guardado != nil {
		t.Error("llegó a escribir en la base")
	}
}

func TestParametrosVaciosSeGuardanComoObjetoVacio(t *testing.T) {
	repo := nuevosJobs()
	svc := NewService(repo, &runsFalsos{}, nil)

	res, _ := svc.Save(context.Background(), Job{
		Name: "  Algo  ", Cron: "  * * * * *  ", Action: AccionEmail, Params: "",
	})
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}
	if repo.guardado.Params != "{}" {
		t.Errorf("guardó %q, quería \"{}\"", repo.guardado.Params)
	}
	if repo.guardado.Name != "Algo" || repo.guardado.Cron != "* * * * *" {
		t.Errorf("no recortó los espacios: %+v", repo.guardado)
	}
}

func TestEjecutarAnotaElResultadoSalgaBienOMal(t *testing.T) {
	t.Run("bien", func(t *testing.T) {
		repo := nuevosJobs(Job{ID: "j1", Action: AccionEmail, Params: "{}", Enabled: true})
		runs := &runsFalsos{}
		svc := NewService(repo, runs, map[Accion]Ejecutor{AccionEmail: ejecutorQueVaBien("enviado")})

		res, err := svc.Ejecutar(context.Background(), "j1")
		if err != nil || !res.OK {
			t.Fatalf("falló: err=%v res=%+v", err, res)
		}
		if len(runs.anotados) != 1 || runs.anotados[0].Status != EstadoOK {
			t.Errorf("no anotó el éxito: %+v", runs.anotados)
		}
	})

	t.Run("mal", func(t *testing.T) {
		repo := nuevosJobs(Job{ID: "j1", Action: AccionEmail, Params: "{}", Enabled: true})
		runs := &runsFalsos{}
		svc := NewService(repo, runs, map[Accion]Ejecutor{AccionEmail: ejecutorQueFalla("sin servidor")})

		res, err := svc.Ejecutar(context.Background(), "j1")
		if err != nil {
			t.Fatalf("un fallo de la acción no es un error técnico: %v", err)
		}
		if res.OK {
			t.Error("dio por buena una ejecución que falló")
		}
		if len(runs.anotados) != 1 || runs.anotados[0].Status != EstadoError {
			t.Errorf("no anotó el fallo: %+v", runs.anotados)
		}
		if runs.anotados[0].Output == nil || *runs.anotados[0].Output != "sin servidor" {
			t.Error("no guardó el motivo del fallo")
		}
	})
}

func TestAccionSinEjecutorLoDice(t *testing.T) {
	repo := nuevosJobs(Job{ID: "j1", Action: Accion("telepatía"), Params: "{}"})
	svc := NewService(repo, &runsFalsos{}, map[Accion]Ejecutor{})

	res, err := svc.Ejecutar(context.Background(), "j1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK {
		t.Error("ejecutó una acción que nadie sabe hacer")
	}
}

func TestTareaQueYaNoExiste(t *testing.T) {
	svc := NewService(nuevosJobs(), &runsFalsos{}, nil)

	res, err := svc.Ejecutar(context.Background(), "fantasma")
	if err != nil {
		t.Fatalf("debía ser error de negocio: %v", err)
	}
	if res.OK {
		t.Error("ejecutó una tarea inexistente")
	}
}

// Solo se lanzan las tareas activas a las que les toca ahora.
func TestEjecutarPendientesFiltraPorCronYPorActiva(t *testing.T) {
	repo := nuevosJobs(
		Job{ID: "ahora", Cron: "* * * * *", Action: AccionEmail, Params: "{}", Enabled: true},
		Job{ID: "a-las-3", Cron: "0 3 * * *", Action: AccionEmail, Params: "{}", Enabled: true},
		Job{ID: "apagada", Cron: "* * * * *", Action: AccionEmail, Params: "{}", Enabled: false},
	)
	svc := NewService(repo, &runsFalsos{}, map[Accion]Ejecutor{AccionEmail: ejecutorQueVaBien("ok")})

	momento := time.Date(2025, 3, 3, 9, 30, 0, 0, time.Local)
	lanzadas, err := svc.EjecutarPendientes(context.Background(), momento)
	if err != nil {
		t.Fatalf("falló: %v", err)
	}
	if len(lanzadas) != 1 || lanzadas[0] != "ahora" {
		t.Errorf("lanzó %v, quería solo [ahora]", lanzadas)
	}
}

// Que una tarea falle no puede impedir que corran las demás.
func TestUnaTareaQueFallaNoParaAlResto(t *testing.T) {
	repo := nuevosJobs(
		Job{ID: "j1", Cron: "* * * * *", Action: AccionEmail, Params: "{}", Enabled: true},
		Job{ID: "j2", Cron: "* * * * *", Action: AccionEmail, Params: "{}", Enabled: true},
	)
	runs := &runsFalsos{}
	svc := NewService(repo, runs, map[Accion]Ejecutor{AccionEmail: ejecutorQueFalla("se rompió")})

	lanzadas, err := svc.EjecutarPendientes(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("falló: %v", err)
	}
	if len(lanzadas) != 2 {
		t.Errorf("lanzó %d de 2", len(lanzadas))
	}
	if len(runs.anotados) != 2 {
		t.Errorf("anotó %d ejecuciones de 2", len(runs.anotados))
	}
}

func TestToggleSoloCambiaElInterruptor(t *testing.T) {
	repo := nuevosJobs(Job{ID: "j1", Name: "Aviso", Cron: "0 9 * * *", Action: AccionEmail, Params: `{"a":1}`, Enabled: true})
	svc := NewService(repo, &runsFalsos{}, nil)

	res, err := svc.Toggle(context.Background(), "j1", false)
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if res.Value.Enabled {
		t.Error("no la apagó")
	}
	if res.Value.Name != "Aviso" || res.Value.Cron != "0 9 * * *" || res.Value.Params != `{"a":1}` {
		t.Errorf("cambió algo más que el interruptor: %+v", res.Value)
	}
}

func TestFalloDeBaseSubeComoError(t *testing.T) {
	repo := nuevosJobs()
	repo.fallo = errors.New("sin conexión")
	svc := NewService(repo, &runsFalsos{}, nil)

	if _, err := svc.EjecutarPendientes(context.Background(), time.Now()); err == nil {
		t.Error("se tragó un fallo de base")
	}
}

func TestLimiteDelHistorialSeAcota(t *testing.T) {
	// No es tanto comprobar el número como que no se pueda pedir un millón
	// de filas ni un número negativo.
	svc := NewService(nuevosJobs(), &runsFalsos{}, nil)
	for _, limite := range []int{0, -5, 999999} {
		if _, err := svc.Runs(context.Background(), limite); err != nil {
			t.Errorf("Runs(%d) falló: %v", limite, err)
		}
	}
}
