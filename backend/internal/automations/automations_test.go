package automations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func accionTelegram(msg string) Accion {
	return Accion{Type: TelegramAccion, Config: map[string]string{"message": msg}}
}

func TestValidarRegla(t *testing.T) {
	buena := Regla{Name: "  Avisar  ", Trigger: UptimeCaido, Actions: []Accion{accionTelegram("hola")}}
	res := ValidarRegla(buena)
	if !res.OK {
		t.Fatalf("rechazó una regla correcta: %s", res.Error)
	}
	if res.Value.Name != "Avisar" {
		t.Errorf("no recortó el nombre: %q", res.Value.Name)
	}

	casos := map[string]Regla{
		"sin nombre":           {Trigger: UptimeCaido, Actions: []Accion{accionTelegram("x")}},
		"disparador inventado": {Name: "A", Trigger: Disparador("lluvia"), Actions: []Accion{accionTelegram("x")}},
		"sin acciones":         {Name: "A", Trigger: UptimeCaido},
		"acción vacía":         {Name: "A", Trigger: UptimeCaido, Actions: []Accion{accionTelegram("  ")}},
		"acción desconocida":   {Name: "A", Trigger: UptimeCaido, Actions: []Accion{{Type: TipoAccion("bailar")}}},
	}
	for nombre, r := range casos {
		if ValidarRegla(r).OK {
			t.Errorf("%s: la dio por buena", nombre)
		}
	}
}

// Una regla puede tener varias acciones, así que el error debe decir cuál.
func TestElErrorDiceQueAccionFalla(t *testing.T) {
	r := Regla{Name: "A", Trigger: UptimeCaido, Actions: []Accion{
		accionTelegram("vale"),
		accionTelegram("   "),
	}}

	res := ValidarRegla(r)
	if res.OK {
		t.Fatal("la dio por buena")
	}
	if !strings.Contains(res.Error, "2") {
		t.Errorf("no dice qué acción falla: %q", res.Error)
	}
}

func TestValidarAccionDeWebhook(t *testing.T) {
	webhook := func(u string) Regla {
		return Regla{Name: "A", Trigger: UptimeCaido, Actions: []Accion{
			{Type: WebhookAccion, Config: map[string]string{"webhookUrl": u}},
		}}
	}

	if !ValidarRegla(webhook("https://n8n.test/webhook/abc")).OK {
		t.Error("rechazó un webhook válido")
	}
	for _, malo := range []string{"", "   ", "no-es-url", "ftp://n8n.test/x", "file:///etc/passwd"} {
		if ValidarRegla(webhook(malo)).OK {
			t.Errorf("aceptó un webhook inválido: %q", malo)
		}
	}
}

func TestValidarAccionDeCalendario(t *testing.T) {
	evento := func(cfg map[string]string) Regla {
		return Regla{Name: "A", Trigger: UptimeCaido, Actions: []Accion{{Type: EventoAccion, Config: cfg}}}
	}

	if !ValidarRegla(evento(map[string]string{"title": "Revisar", "daysFromNow": "3"})).OK {
		t.Error("rechazó un evento válido")
	}
	if !ValidarRegla(evento(map[string]string{"title": "Revisar"})).OK {
		t.Error("rechazó un evento sin días, que debería valer como 0")
	}
	if ValidarRegla(evento(map[string]string{"title": "  "})).OK {
		t.Error("aceptó un evento sin título")
	}
	if ValidarRegla(evento(map[string]string{"title": "A", "daysFromNow": "-3"})).OK {
		t.Error("aceptó días negativos")
	}
	if ValidarRegla(evento(map[string]string{"title": "A", "daysFromNow": "mañana"})).OK {
		t.Error("aceptó días que no son un número")
	}
}

func TestDiasDesdeHoy(t *testing.T) {
	casos := map[string]int{"3": 3, "0": 0, "": 0, "-5": 0, "hoy": 0}
	for dado, quiero := range casos {
		a := Accion{Config: map[string]string{"daysFromNow": dado}}
		if got := DiasDesdeHoy(a); got != quiero {
			t.Errorf("DiasDesdeHoy(%q) = %d, quería %d", dado, got, quiero)
		}
	}
}

// ── Disparar ───────────────────────────────────────────────────────────────

type repoFalso struct {
	reglas   []Regla
	anotados []bool
	errores  []*string
	fallo    error
}

func (r *repoFalso) List(context.Context) ([]Regla, error) { return r.reglas, r.fallo }
func (r *repoFalso) ActivasPara(context.Context, Disparador) ([]Regla, error) {
	return r.reglas, r.fallo
}
func (r *repoFalso) Create(_ context.Context, g Regla) (*Regla, error) { return &g, r.fallo }
func (r *repoFalso) Update(_ context.Context, g Regla) (*Regla, error) { return &g, r.fallo }
func (r *repoFalso) Remove(context.Context, string) error              { return r.fallo }
func (r *repoFalso) Anotar(_ context.Context, _ string, ok bool, fallo *string) error {
	r.anotados = append(r.anotados, ok)
	r.errores = append(r.errores, fallo)
	return nil
}
func (r *repoFalso) Ejecuciones(context.Context, string, int) ([]Ejecucion, error) {
	return nil, r.fallo
}

type avisadorFalso struct {
	mensajes []string
	fallo    error
}

func (a *avisadorFalso) Send(_ context.Context, m string) error {
	a.mensajes = append(a.mensajes, m)
	return a.fallo
}

type agendaFalsa struct {
	titulos []string
	cuando  []time.Time
}

func (a *agendaFalsa) CrearEvento(_ context.Context, titulo string, _ *string, cuando time.Time) error {
	a.titulos = append(a.titulos, titulo)
	a.cuando = append(a.cuando, cuando)
	return nil
}

type webhookFalso struct {
	cuerpos []map[string]string
}

func (w *webhookFalso) Post(_ context.Context, _ string, cuerpo map[string]string) error {
	w.cuerpos = append(w.cuerpos, cuerpo)
	return nil
}

// rellenar de mentira, suficiente para las pruebas.
func rellenarSimple(texto string, valores map[string]string) string {
	for k, v := range valores {
		texto = strings.ReplaceAll(texto, "{{"+k+"}}", v)
	}
	return texto
}

func TestDispararRellenaLasVariables(t *testing.T) {
	repo := &repoFalso{reglas: []Regla{{
		ID: "r1", Name: "Avisar", Trigger: UptimeCaido, Active: true,
		Actions: []Accion{accionTelegram("Se cayó {{monitor}} ({{url}})")},
	}}}
	avisos := &avisadorFalso{}

	svc := NewService(repo, avisos, &agendaFalsa{}, &webhookFalso{}, rellenarSimple)
	svc.Disparar(context.Background(), UptimeCaido, map[string]string{
		"monitor": "La web", "url": "https://x.com",
	})

	if len(avisos.mensajes) != 1 {
		t.Fatalf("mandó %d mensajes", len(avisos.mensajes))
	}
	if avisos.mensajes[0] != "Se cayó La web (https://x.com)" {
		t.Errorf("mensaje = %q", avisos.mensajes[0])
	}
	if len(repo.anotados) != 1 || !repo.anotados[0] {
		t.Errorf("no anotó el éxito: %v", repo.anotados)
	}
}

// Lo importante: Disparar lo llaman otros módulos en mitad de operaciones
// importantes, así que una regla rota no puede tumbarlos.
func TestUnaReglaQueFallaNoRompeAQuienLaDispara(t *testing.T) {
	repo := &repoFalso{reglas: []Regla{
		{ID: "r1", Name: "Rota", Trigger: UptimeCaido, Actions: []Accion{accionTelegram("x")}},
		{ID: "r2", Name: "Buena", Trigger: UptimeCaido, Actions: []Accion{accionTelegram("y")}},
	}}
	avisos := &avisadorFalso{fallo: errors.New("telegram caído")}

	svc := NewService(repo, avisos, &agendaFalsa{}, &webhookFalso{}, rellenarSimple)

	// No devuelve error: si lo hiciera, quien lo llama tendría que decidir
	// qué hacer en mitad de un login o de un tic del reloj.
	svc.Disparar(context.Background(), UptimeCaido, nil)

	if len(repo.anotados) != 2 {
		t.Fatalf("anotó %d ejecuciones, quería 2 (las dos reglas)", len(repo.anotados))
	}
	for i, ok := range repo.anotados {
		if ok {
			t.Errorf("la regla %d salió bien y debía fallar", i)
		}
		if repo.errores[i] == nil {
			t.Errorf("la regla %d no guardó el motivo", i)
		}
	}
}

// Las acciones van en orden y se para en la primera que falla: si la segunda
// depende de la primera, seguir sería peor.
func TestSeParaEnLaPrimeraAccionQueFalla(t *testing.T) {
	repo := &repoFalso{reglas: []Regla{{
		ID: "r1", Trigger: UptimeCaido,
		Actions: []Accion{
			accionTelegram("primera"),
			{Type: EventoAccion, Config: map[string]string{"title": "no debería crearse"}},
		},
	}}}
	avisos := &avisadorFalso{fallo: errors.New("telegram caído")}
	agenda := &agendaFalsa{}

	NewService(repo, avisos, agenda, &webhookFalso{}, rellenarSimple).
		Disparar(context.Background(), UptimeCaido, nil)

	if len(agenda.titulos) != 0 {
		t.Errorf("siguió tras el fallo y creó %v", agenda.titulos)
	}
}

// Si no se pueden leer las reglas, no se anota nada ni se rompe nada.
func TestSinReglasLegiblesNoPasaNada(t *testing.T) {
	repo := &repoFalso{fallo: errors.New("sin conexión")}
	svc := NewService(repo, &avisadorFalso{}, &agendaFalsa{}, &webhookFalso{}, rellenarSimple)

	svc.Disparar(context.Background(), UptimeCaido, nil)

	if len(repo.anotados) != 0 {
		t.Error("anotó ejecuciones sin haber leído reglas")
	}
}

func TestElWebhookLlevaElDisparadorYLasVariables(t *testing.T) {
	repo := &repoFalso{reglas: []Regla{{
		ID: "r1", Trigger: UsuarioRegistrado,
		Actions: []Accion{{Type: WebhookAccion, Config: map[string]string{"webhookUrl": "https://n8n.test/x"}}},
	}}}
	wh := &webhookFalso{}

	NewService(repo, &avisadorFalso{}, &agendaFalsa{}, wh, rellenarSimple).
		Disparar(context.Background(), UsuarioRegistrado, map[string]string{"email": "a@b.com"})

	if len(wh.cuerpos) != 1 {
		t.Fatalf("hizo %d llamadas", len(wh.cuerpos))
	}
	if wh.cuerpos[0]["trigger"] != "user_registered" {
		t.Errorf("no mandó el disparador: %v", wh.cuerpos[0])
	}
	if wh.cuerpos[0]["email"] != "a@b.com" {
		t.Errorf("no mandó las variables: %v", wh.cuerpos[0])
	}
}

func TestElEventoSeCreaConLosDiasDeDesplazamiento(t *testing.T) {
	repo := &repoFalso{reglas: []Regla{{
		ID: "r1", Trigger: UptimeCaido,
		Actions: []Accion{{Type: EventoAccion, Config: map[string]string{
			"title": "Revisar {{monitor}}", "daysFromNow": "3",
		}}},
	}}}
	agenda := &agendaFalsa{}

	antes := time.Now()
	NewService(repo, &avisadorFalso{}, agenda, &webhookFalso{}, rellenarSimple).
		Disparar(context.Background(), UptimeCaido, map[string]string{"monitor": "La web"})

	if len(agenda.titulos) != 1 || agenda.titulos[0] != "Revisar La web" {
		t.Fatalf("títulos = %v", agenda.titulos)
	}
	dias := agenda.cuando[0].Sub(antes).Hours() / 24
	if dias < 2.9 || dias > 3.1 {
		t.Errorf("el evento quedó a %.1f días, quería 3", dias)
	}
}
