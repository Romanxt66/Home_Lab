package calendar

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func punt[T any](v T) *T { return &v }

func TestValidar(t *testing.T) {
	inicio := time.Date(2025, 3, 3, 9, 0, 0, 0, time.Local)

	bueno := Evento{Title: "  Reunión  ", Start: inicio}
	res := Validar(bueno)
	if !res.OK {
		t.Fatalf("rechazó un evento correcto: %s", res.Error)
	}
	if res.Value.Title != "Reunión" {
		t.Errorf("no recortó el título: %q", res.Value.Title)
	}

	casos := map[string]Evento{
		"sin título":       {Start: inicio},
		"título en blanco": {Title: "   ", Start: inicio},
		"sin fecha":        {Title: "Algo"},
		"fin antes":        {Title: "Algo", Start: inicio, End: punt(inicio.Add(-time.Hour))},
		"aviso negativo":   {Title: "Algo", Start: inicio, RemindMinutesBefore: punt(-10)},
	}
	for nombre, e := range casos {
		if Validar(e).OK {
			t.Errorf("%s: lo dio por bueno", nombre)
		}
	}

	// El fin igual al inicio sí vale: es un evento instantáneo.
	igual := Evento{Title: "Algo", Start: inicio, End: punt(inicio)}
	if !Validar(igual).OK {
		t.Error("rechazó un evento que empieza y acaba a la vez")
	}
}

func TestAvisoPendiente(t *testing.T) {
	ahora := time.Date(2025, 3, 3, 9, 0, 0, 0, time.Local)

	casos := []struct {
		nombre string
		evento Evento
		quiero bool
	}{
		{
			"toca justo ahora",
			Evento{Start: ahora.Add(10 * time.Minute), RemindMinutesBefore: punt(10)},
			true,
		},
		{
			"ya pasó el momento de avisar",
			Evento{Start: ahora.Add(5 * time.Minute), RemindMinutesBefore: punt(10)},
			true,
		},
		{
			"todavía no toca",
			Evento{Start: ahora.Add(time.Hour), RemindMinutesBefore: punt(10)},
			false,
		},
		{
			"sin recordatorio configurado",
			Evento{Start: ahora.Add(5 * time.Minute)},
			false,
		},
		{
			"ya se avisó",
			Evento{Start: ahora.Add(5 * time.Minute), RemindMinutesBefore: punt(10), ReminderSentAt: &ahora},
			false,
		},
		{
			"evento de hace dos días: no se avisa con retraso",
			Evento{Start: ahora.Add(-48 * time.Hour), RemindMinutesBefore: punt(10)},
			false,
		},
		{
			"evento de hace dos horas: todavía vale",
			Evento{Start: ahora.Add(-2 * time.Hour), RemindMinutesBefore: punt(10)},
			true,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.evento.AvisoPendiente(ahora); got != c.quiero {
				t.Errorf("AvisoPendiente = %v, quería %v", got, c.quiero)
			}
		})
	}
}

func TestTextoDelAviso(t *testing.T) {
	e := Evento{
		Title:    "Dentista",
		Start:    time.Date(2025, 3, 3, 17, 30, 0, 0, time.Local),
		Location: punt("Calle Mayor 1"),
	}

	texto := TextoDelAviso(e)
	for _, trozo := range []string{"Dentista", "17:30", "Calle Mayor 1"} {
		if !strings.Contains(texto, trozo) {
			t.Errorf("falta %q en:\n%s", trozo, texto)
		}
	}

	todoElDia := Evento{Title: "Vacaciones", Start: e.Start, AllDay: true}
	if !strings.Contains(TextoDelAviso(todoElDia), "todo el día") {
		t.Error("un evento de todo el día debería decirlo en vez de dar una hora")
	}
}

func TestLaDescripcionLargaSeRecorta(t *testing.T) {
	larga := strings.Repeat("x", 500)
	texto := TextoDelAviso(Evento{Title: "A", Start: time.Now(), Description: &larga})
	if len([]rune(texto)) > 300 {
		t.Errorf("el aviso salió de %d caracteres", len([]rune(texto)))
	}
}

// La rejilla del mes enseña días del mes anterior y del siguiente; sin el
// margen, esos eventos no aparecerían.
func TestRangoDelMesLlevaMargen(t *testing.T) {
	desde, hasta := RangoDelMes(2025, 3, time.Local)

	if desde.After(time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("el rango empieza después del día 1: %v", desde)
	}
	if hasta.Before(time.Date(2025, 4, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("el rango acaba antes del día 1 del mes siguiente: %v", hasta)
	}
	if dias := hasta.Sub(desde).Hours() / 24; dias < 44 {
		t.Errorf("el rango cubre %.0f días, esperaba el mes más dos semanas", dias)
	}
}

// ── El servicio ────────────────────────────────────────────────────────────

type repoFalso struct {
	eventos     []Evento
	marcados    []string
	fallo       error
	falloMarcar error
}

func (r *repoFalso) ListBetween(context.Context, time.Time, time.Time) ([]Evento, error) {
	return r.eventos, r.fallo
}
func (r *repoFalso) Get(context.Context, string) (*Evento, error) { return nil, r.fallo }
func (r *repoFalso) Create(_ context.Context, e Evento) (*Evento, error) {
	e.ID = "nuevo"
	return &e, r.fallo
}
func (r *repoFalso) Update(_ context.Context, e Evento) (*Evento, error) { return &e, r.fallo }
func (r *repoFalso) Remove(context.Context, string) error                { return r.fallo }
func (r *repoFalso) Candidatos(context.Context, time.Time) ([]Evento, error) {
	return r.eventos, r.fallo
}
func (r *repoFalso) MarcarAvisado(_ context.Context, id string, _ time.Time) error {
	if r.falloMarcar != nil {
		return r.falloMarcar
	}
	r.marcados = append(r.marcados, id)
	return nil
}

type avisadorFalso struct {
	mensajes []string
	fallo    error
}

func (a *avisadorFalso) Send(_ context.Context, m string) error {
	a.mensajes = append(a.mensajes, m)
	return a.fallo
}

func TestProcesarAvisosSoloMandaLosQueTocan(t *testing.T) {
	ahora := time.Date(2025, 3, 3, 9, 0, 0, 0, time.Local)
	repo := &repoFalso{eventos: []Evento{
		{ID: "toca", Title: "Toca", Start: ahora.Add(5 * time.Minute), RemindMinutesBefore: punt(10)},
		{ID: "pronto", Title: "Pronto", Start: ahora.Add(5 * time.Hour), RemindMinutesBefore: punt(10)},
		{ID: "sin-aviso", Title: "Sin aviso", Start: ahora.Add(time.Minute)},
	}}
	avisos := &avisadorFalso{}

	resumen, err := NewService(repo, avisos).ProcesarAvisos(context.Background(), ahora)
	if err != nil {
		t.Fatalf("falló: %v", err)
	}
	if resumen.Enviados != 1 {
		t.Errorf("mandó %d avisos, quería 1", resumen.Enviados)
	}
	if len(repo.marcados) != 1 || repo.marcados[0] != "toca" {
		t.Errorf("marcó %v", repo.marcados)
	}
}

// Marcar es lo que impide que el mismo aviso salga dos veces.
func TestUnAvisoYaMandadoNoSeRepite(t *testing.T) {
	ahora := time.Now()
	repo := &repoFalso{eventos: []Evento{
		{ID: "e1", Start: ahora.Add(time.Minute), RemindMinutesBefore: punt(10), ReminderSentAt: &ahora},
	}}
	avisos := &avisadorFalso{}

	resumen, _ := NewService(repo, avisos).ProcesarAvisos(context.Background(), ahora)
	if resumen.Enviados != 0 || len(avisos.mensajes) != 0 {
		t.Errorf("repitió un aviso ya mandado: %+v", resumen)
	}
}

// Si el envío falla no se marca, para que se reintente en la pasada siguiente.
func TestUnAvisoQueFallaNoSeMarca(t *testing.T) {
	ahora := time.Now()
	repo := &repoFalso{eventos: []Evento{
		{ID: "e1", Title: "Algo", Start: ahora.Add(time.Minute), RemindMinutesBefore: punt(10)},
	}}
	avisos := &avisadorFalso{fallo: errors.New("telegram caído")}

	resumen, err := NewService(repo, avisos).ProcesarAvisos(context.Background(), ahora)
	if err != nil {
		t.Fatalf("un fallo de aviso no es error técnico: %v", err)
	}
	if resumen.Fallidos != 1 {
		t.Errorf("resumen = %+v", resumen)
	}
	if len(repo.marcados) != 0 {
		t.Error("marcó como avisado algo que no se pudo enviar")
	}
}

func TestFalloDeBaseSubeComoError(t *testing.T) {
	repo := &repoFalso{fallo: errors.New("sin conexión")}
	if _, err := NewService(repo, &avisadorFalso{}).ProcesarAvisos(context.Background(), time.Now()); err == nil {
		t.Error("se tragó un fallo de base")
	}
}

func TestMesInvalidoNoConsulta(t *testing.T) {
	svc := NewService(&repoFalso{}, &avisadorFalso{})
	for _, mes := range []int{0, 13, -1} {
		eventos, err := svc.Mes(context.Background(), 2025, mes)
		if err != nil || eventos != nil {
			t.Errorf("Mes(%d) devolvió %v, %v", mes, eventos, err)
		}
	}
}
