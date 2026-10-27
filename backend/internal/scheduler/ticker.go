package scheduler

import (
	"context"
	"log"
	"time"
)

// ahora existe para poder fijar el reloj en las pruebas.
var ahora = time.Now

// Tarea es algo que hay que lanzar cada minuto. Los recordatorios del calendario también entran aquí.
type Tarea struct {
	Nombre string
	Correr func(ctx context.Context) error
}

// Ticker lanza las tareas pendientes cada minuto, dentro del propio proceso.
//
// El Lab necesitaba un cron externo porque en serverless el proceso no vive
// entre peticiones. Aquí el binario está siempre levantado, así que una
// gorutina basta y desaparece la dependencia de que alguien llame al
// endpoint desde fuera.
//
// Ojo: mientras el Lab siga desplegado, su propio cron está ejecutando estas
// mismas tareas sobre la misma base. Por eso el ticker viene apagado y hay
// que encenderlo a propósito con SCHEDULER_INTERNO=1, para no acabar
// mandando cada correo dos veces.
type Ticker struct {
	tareas []Tarea
}

func NewTicker(tareas ...Tarea) *Ticker {
	return &Ticker{tareas: tareas}
}

// Arrancar corre hasta que se cancele el contexto.
func (t *Ticker) Arrancar(ctx context.Context) {
	// Se espera al siguiente minuto en punto antes de empezar: así las
	// expresiones cron coinciden con el reloj y no con el momento en que
	// arrancó el proceso.
	primera := time.Until(ahora().Truncate(time.Minute).Add(time.Minute))
	espera := time.NewTimer(primera)
	defer espera.Stop()

	select {
	case <-ctx.Done():
		return
	case <-espera.C:
	}

	t.pasada(ctx)

	reloj := time.NewTicker(time.Minute)
	defer reloj.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("planificador: parado")
			return
		case <-reloj.C:
			t.pasada(ctx)
		}
	}
}

// pasada lanza todas las tareas. Cada una va en su propia gorutina con su
// propio tiempo límite: una tarea lenta no puede retrasar a las demás ni
// comerse el minuto siguiente.
func (t *Ticker) pasada(padre context.Context) {
	for _, tarea := range t.tareas {
		go func(tarea Tarea) {
			ctx, cancelar := context.WithTimeout(padre, 55*time.Second)
			defer cancelar()

			if err := tarea.Correr(ctx); err != nil {
				log.Printf("planificador: %s falló: %v", tarea.Nombre, err)
			}
		}(tarea)
	}
}

// TareaDeJobs envuelve el servicio como una Tarea del ticker.
func (s *Service) TareaDeJobs() Tarea {
	return Tarea{
		Nombre: "tareas programadas",
		Correr: func(ctx context.Context) error {
			lanzadas, err := s.EjecutarPendientes(ctx, ahora())
			if err != nil {
				return err
			}
			if len(lanzadas) > 0 {
				log.Printf("planificador: lanzadas %d tareas", len(lanzadas))
			}
			return nil
		},
	}
}
