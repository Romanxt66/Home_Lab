package coolify

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ── Interpretar el estado ──────────────────────────────────────────────────

func TestInterpretarEstado(t *testing.T) {
	casos := []struct {
		crudo  string
		estado Estado
		sano   *bool
	}{
		{"running:healthy", EnMarcha, punt(true)},
		// Lo delicado: "unhealthy" contiene "healthy", así que hay que
		// descartarlo antes de dar nada por sano.
		{"running:unhealthy", ConProblemas, punt(false)},
		{"running", EnMarcha, nil},
		{"exited:unhealthy", Detenida, punt(false)},
		{"stopped", Detenida, nil},
		{"dead", Detenida, nil},
		{"restarting", ConProblemas, nil},
		{"starting:unhealthy", ConProblemas, punt(false)},
		{"", Desconocido, nil},
		{"   ", Desconocido, nil},
		{"algo-que-no-conocemos", Desconocido, nil},
		{"RUNNING:HEALTHY", EnMarcha, punt(true)}, // sin distinguir mayúsculas
	}

	for _, c := range casos {
		t.Run(c.crudo, func(t *testing.T) {
			estado, sano := InterpretarEstado(c.crudo)
			if estado != c.estado {
				t.Errorf("estado = %q, quería %q", estado, c.estado)
			}
			switch {
			case c.sano == nil && sano != nil:
				t.Errorf("salud = %v, quería desconocida", *sano)
			case c.sano != nil && sano == nil:
				t.Errorf("salud desconocida, quería %v", *c.sano)
			case c.sano != nil && *sano != *c.sano:
				t.Errorf("salud = %v, quería %v", *sano, *c.sano)
			}
		})
	}
}

func punt[T any](v T) *T { return &v }

func TestInterpretarDespliegue(t *testing.T) {
	casos := map[string]EstadoDespliegue{
		"in_progress":       EnCurso,
		"running":           EnCurso,
		"queued":            EnCola,
		"failed":            Fallido,
		"error":             Fallido,
		"cancelled-by-user": Cancelado,
		"finished":          Terminado,
		"success":           Terminado,
		"done":              Terminado,
		"":                  EnCola, // lo prudente ante algo desconocido
		"cualquier cosa":    EnCola,
	}

	for crudo, quiero := range casos {
		if got := InterpretarDespliegue(crudo); got != quiero {
			t.Errorf("InterpretarDespliegue(%q) = %q, quería %q", crudo, got, quiero)
		}
	}
}

// ── Mensajes de error ──────────────────────────────────────────────────────

// Coolify corre sobre Laravel: el "message" suelto no dice nada útil, lo
// accionable está en el saco de errores por campo.
func TestExplicarErrorAplanaLosCamposDeLaravel(t *testing.T) {
	cuerpo := []byte(`{
		"message": "Validation failed.",
		"errors": {
			"ports_exposes": ["The ports exposes field is required."],
			"name": ["The name has already been taken."]
		}
	}`)

	msg := ExplicarError(422, cuerpo)

	for _, esperado := range []string{"ports_exposes", "required", "name", "already been taken"} {
		if !strings.Contains(msg, esperado) {
			t.Errorf("falta %q en el mensaje: %s", esperado, msg)
		}
	}
}

// El orden de los campos debe ser estable: un mapa de Go no lo garantiza y
// el mensaje cambiaría de una llamada a otra.
func TestElOrdenDeLosCamposEsEstable(t *testing.T) {
	cuerpo := []byte(`{"message":"x","errors":{"zeta":["a"],"alfa":["b"],"media":["c"]}}`)

	primero := ExplicarError(422, cuerpo)
	for i := 0; i < 20; i++ {
		if ExplicarError(422, cuerpo) != primero {
			t.Fatalf("el mensaje cambió en la pasada %d", i)
		}
	}
	if strings.Index(primero, "alfa") > strings.Index(primero, "zeta") {
		t.Errorf("no salió en orden alfabético: %s", primero)
	}
}

func TestExplicarErrorPorEstado(t *testing.T) {
	if ExplicarError(401, nil) != ExplicarError(403, nil) {
		t.Error("401 y 403 son el mismo problema para quien lo lee")
	}
	if !strings.Contains(ExplicarError(404, nil), "encuentra") {
		t.Errorf("404: %s", ExplicarError(404, nil))
	}
	if ExplicarError(500, []byte("no es json")) == "" {
		t.Error("sin JSON debería dar igual un mensaje")
	}
	// Un campo que llega como texto suelto en vez de lista.
	msg := ExplicarError(422, []byte(`{"message":"x","errors":{"campo":"un motivo"}}`))
	if !strings.Contains(msg, "un motivo") {
		t.Errorf("no leyó un motivo suelto: %s", msg)
	}
}

// ── Ordenar ────────────────────────────────────────────────────────────────

// Lo que hay que mirar primero al abrir el panel es lo que está roto.
func TestOrdenarAppsPoneLosProblemasArriba(t *testing.T) {
	apps := []App{
		{Name: "zeta", State: EnMarcha},
		{Name: "alfa", State: EnMarcha},
		{Name: "rota", State: ConProblemas},
		{Name: "parada", State: Detenida},
		{Name: "rara", State: Desconocido},
	}

	ordenadas := OrdenarApps(apps)
	quiero := []string{"rota", "parada", "rara", "alfa", "zeta"}

	for i, n := range quiero {
		if ordenadas[i].Name != n {
			t.Errorf("posición %d = %q, quería %q", i, ordenadas[i].Name, n)
		}
	}

	// La lista original no se toca.
	if apps[0].Name != "zeta" {
		t.Error("modificó la lista original")
	}
}

// ── Validaciones ───────────────────────────────────────────────────────────

func TestValidarBaseURL(t *testing.T) {
	buenas := map[string]string{
		"https://coolify.midominio.com": "https://coolify.midominio.com",
		"  https://coolify.test/  ":     "https://coolify.test",
		"http://localhost:8000///":      "http://localhost:8000",
	}
	for dada, quiero := range buenas {
		res := ValidarBaseURL(dada)
		if !res.OK {
			t.Errorf("rechazó %q: %s", dada, res.Error)
			continue
		}
		if res.Value != quiero {
			t.Errorf("ValidarBaseURL(%q) = %q, quería %q", dada, res.Value, quiero)
		}
	}

	for _, mala := range []string{"", "  ", "coolify.test", "ftp://coolify.test"} {
		if ValidarBaseURL(mala).OK {
			t.Errorf("aceptó %q", mala)
		}
	}
}

func TestValidarNuevaApp(t *testing.T) {
	base := NuevaApp{
		ProjectUUID: "p", EnvironmentUUID: "e", ServerUUID: "s",
		Source: "public", GitRepository: "https://github.com/x/y", GitBranch: "main",
		BuildPack: "nixpacks",
	}
	if res := ValidarNuevaApp(base); !res.OK {
		t.Errorf("rechazó una app correcta: %s", res.Error)
	}

	sinProyecto := base
	sinProyecto.ProjectUUID = ""
	if ValidarNuevaApp(sinProyecto).OK {
		t.Error("aceptó una app sin proyecto")
	}

	sinRepo := base
	sinRepo.GitRepository = "  "
	if ValidarNuevaApp(sinRepo).OK {
		t.Error("aceptó un origen público sin repositorio")
	}

	buildPackRaro := base
	buildPackRaro.BuildPack = "magia"
	if ValidarNuevaApp(buildPackRaro).OK {
		t.Error("aceptó un build pack que no existe")
	}

	dockerfile := base
	dockerfile.Source = "dockerfile"
	dockerfile.Dockerfile = ""
	if ValidarNuevaApp(dockerfile).OK {
		t.Error("aceptó un origen dockerfile sin contenido")
	}
	dockerfile.Dockerfile = "FROM alpine"
	if !ValidarNuevaApp(dockerfile).OK {
		t.Error("rechazó un dockerfile válido")
	}

	origenRaro := base
	origenRaro.Source = "telepatia"
	if ValidarNuevaApp(origenRaro).OK {
		t.Error("aceptó un origen que no existe")
	}
}

// ── El servicio ────────────────────────────────────────────────────────────

type clienteFalso struct {
	pingFalla string
	resumen   Resumen
	llamadas  []string
}

func (c *clienteFalso) anotar(q string) { c.llamadas = append(c.llamadas, q) }

func (c *clienteFalso) Ping(context.Context, Credenciales) kernel.Result[string] {
	c.anotar("ping")
	if c.pingFalla != "" {
		return kernel.Err[string](c.pingFalla)
	}
	return kernel.Ok("4.5.0")
}

func (c *clienteFalso) Resumen(context.Context, Credenciales) kernel.Result[Resumen] {
	c.anotar("resumen")
	return kernel.Ok(c.resumen)
}

func (c *clienteFalso) Entornos(context.Context, Credenciales, string) kernel.Result[[]Entorno] {
	return kernel.Ok([]Entorno{})
}
func (c *clienteFalso) CrearApp(context.Context, Credenciales, NuevaApp) kernel.Result[string] {
	c.anotar("crearApp")
	return kernel.Ok("creada")
}
func (c *clienteFalso) App(context.Context, Credenciales, string) kernel.Result[App] {
	return kernel.Ok(App{})
}
func (c *clienteFalso) ActualizarApp(context.Context, Credenciales, string, ParcheApp) kernel.Result[string] {
	return kernel.Ok("ok")
}
func (c *clienteFalso) Desplegar(context.Context, Credenciales, string, bool) kernel.Result[string] {
	return kernel.Ok("ok")
}
func (c *clienteFalso) Controlar(context.Context, Credenciales, string, Accion) kernel.Result[string] {
	c.anotar("controlar")
	return kernel.Ok("ok")
}
func (c *clienteFalso) Registros(_ context.Context, _ Credenciales, _ string, lineas int) kernel.Result[string] {
	c.anotar("registros:" + strconv.Itoa(lineas))
	return kernel.Ok("")
}
func (c *clienteFalso) Variables(context.Context, Credenciales, string) kernel.Result[[]Variable] {
	return kernel.Ok([]Variable{})
}
func (c *clienteFalso) CrearVariable(context.Context, Credenciales, string, NuevaVariable) kernel.Result[string] {
	c.anotar("crearVariable")
	return kernel.Ok("ok")
}
func (c *clienteFalso) ActualizarVariable(context.Context, Credenciales, string, NuevaVariable) kernel.Result[string] {
	return kernel.Ok("ok")
}
func (c *clienteFalso) BorrarVariable(context.Context, Credenciales, string, string) kernel.Result[string] {
	return kernel.Ok("ok")
}
func (c *clienteFalso) Despliegues(context.Context, Credenciales) kernel.Result[[]Despliegue] {
	return kernel.Ok([]Despliegue{})
}
func (c *clienteFalso) CancelarDespliegue(context.Context, Credenciales, string) kernel.Result[string] {
	return kernel.Ok("ok")
}
func (c *clienteFalso) ControlarRecurso(context.Context, Credenciales, TipoRecurso, string, Accion) kernel.Result[string] {
	c.anotar("controlarRecurso")
	return kernel.Ok("ok")
}
func (c *clienteFalso) RegistrosRecurso(context.Context, Credenciales, TipoRecurso, string, int) kernel.Result[string] {
	return kernel.Ok("")
}
func (c *clienteFalso) CrearBase(context.Context, Credenciales, NuevaBase) kernel.Result[string] {
	c.anotar("crearBase")
	return kernel.Ok("ok")
}
func (c *clienteFalso) CrearServicio(context.Context, Credenciales, NuevoServicio) kernel.Result[string] {
	c.anotar("crearServicio")
	return kernel.Ok("ok")
}
func (c *clienteFalso) CrearProyecto(context.Context, Credenciales, string, string) kernel.Result[string] {
	c.anotar("crearProyecto")
	return kernel.Ok("ok")
}
func (c *clienteFalso) ActualizarProyecto(context.Context, Credenciales, string, string, string) kernel.Result[string] {
	return kernel.Ok("ok")
}
func (c *clienteFalso) BorrarRecurso(context.Context, Credenciales, TipoBorrable, string) kernel.Result[string] {
	c.anotar("borrar")
	return kernel.Ok("ok")
}

type repoFalso struct {
	activa   *Config
	guardada *Config
	fallo    error
}

func (r *repoFalso) GetActive(context.Context) (*Config, error) { return r.activa, r.fallo }
func (r *repoFalso) Upsert(_ context.Context, u, t string) (*Config, error) {
	if r.fallo != nil {
		return nil, r.fallo
	}
	r.guardada = &Config{ID: "c1", BaseURL: u, Token: t, Active: true}
	return r.guardada, nil
}
func (r *repoFalso) Remove(context.Context, string) error { return r.fallo }

func conexionActiva() *repoFalso {
	return &repoFalso{activa: &Config{ID: "c1", BaseURL: "https://coolify.test", Token: "t"}}
}

func TestNoSeGuardaUnaCredencialQueNoFunciona(t *testing.T) {
	cliente := &clienteFalso{pingFalla: "El token no vale."}
	repo := &repoFalso{}

	res, err := NewService(cliente, repo).Conectar(context.Background(), "https://coolify.test", "malo")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK {
		t.Error("dio la conexión por buena")
	}
	if repo.guardada != nil {
		t.Error("guardó una credencial que no funciona")
	}
}

func TestSinConexionSeExplicaQueHayQueConectar(t *testing.T) {
	svc := NewService(&clienteFalso{}, &repoFalso{})

	res, err := svc.Resumen(context.Background())
	if err != nil {
		t.Fatalf("debía ser error de negocio: %v", err)
	}
	if res.OK || res.Error == "" {
		t.Errorf("no explicó que falta conectar: %+v", res)
	}
}

// Una acción inventada no debe llegar a provocar una llamada de red.
func TestUnaAccionInventadaNoLlegaAColify(t *testing.T) {
	cliente := &clienteFalso{}
	svc := NewService(cliente, conexionActiva())

	res, _ := svc.Controlar(context.Background(), "u1", Accion("autodestruir"))
	if res.OK {
		t.Error("aceptó una acción que no existe")
	}
	if len(cliente.llamadas) != 0 {
		t.Errorf("llamó a Coolify: %v", cliente.llamadas)
	}
}

func TestUnTipoDeRecursoInventadoNoLlegaAColify(t *testing.T) {
	cliente := &clienteFalso{}
	svc := NewService(cliente, conexionActiva())

	if res, _ := svc.ControlarRecurso(context.Background(), TipoRecurso("naves"), "u1", Arrancar); res.OK {
		t.Error("aceptó un tipo de recurso que no existe")
	}
	if res, _ := svc.BorrarRecurso(context.Background(), TipoBorrable("usuarios"), "u1"); res.OK {
		t.Error("aceptó borrar un tipo que no está permitido")
	}
	if len(cliente.llamadas) != 0 {
		t.Errorf("llamó a Coolify: %v", cliente.llamadas)
	}
}

// Sin tope, un contenedor hablador devolvería megas de texto a una pantalla.
func TestLasLineasDeRegistroSeAcotan(t *testing.T) {
	cliente := &clienteFalso{}
	svc := NewService(cliente, conexionActiva())

	for _, lineas := range []int{0, -5, 999999} {
		cliente.llamadas = nil
		_, _ = svc.Registros(context.Background(), "u1", lineas)
		if len(cliente.llamadas) != 1 || cliente.llamadas[0] != "registros:200" {
			t.Errorf("con %d pidió %v, esperaba el valor por defecto", lineas, cliente.llamadas)
		}
	}

	cliente.llamadas = nil
	_, _ = svc.Registros(context.Background(), "u1", 50)
	if cliente.llamadas[0] != "registros:50" {
		t.Errorf("no respetó un valor razonable: %v", cliente.llamadas)
	}
}

// El resumen llega ya ordenado, para que la interfaz no tenga que hacerlo.
func TestElResumenLlegaOrdenado(t *testing.T) {
	cliente := &clienteFalso{resumen: Resumen{Apps: []App{
		{Name: "buena", State: EnMarcha},
		{Name: "rota", State: ConProblemas},
	}}}

	res, err := NewService(cliente, conexionActiva()).Resumen(context.Background())
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if res.Value.Apps[0].Name != "rota" {
		t.Errorf("no ordenó: %v", res.Value.Apps)
	}
}

func TestLoQueFaltaNoLlegaAColify(t *testing.T) {
	cliente := &clienteFalso{}
	svc := NewService(cliente, conexionActiva())
	ctx := context.Background()

	casos := map[string]func() (kernel.Result[string], error){
		"base sin proyecto": func() (kernel.Result[string], error) { return svc.CrearBase(ctx, NuevaBase{}) },
		"base de tipo raro": func() (kernel.Result[string], error) {
			return svc.CrearBase(ctx, NuevaBase{ProjectUUID: "p", EnvironmentUUID: "e", ServerUUID: "s", Type: "oracle"})
		},
		"servicio sin compose": func() (kernel.Result[string], error) {
			return svc.CrearServicio(ctx, NuevoServicio{ProjectUUID: "p", EnvironmentUUID: "e", ServerUUID: "s"})
		},
		"proyecto sin nombre": func() (kernel.Result[string], error) { return svc.CrearProyecto(ctx, "   ", "") },
		"variable sin clave": func() (kernel.Result[string], error) {
			return svc.CrearVariable(ctx, "u1", NuevaVariable{Key: "  "})
		},
		"app sin nombre": func() (kernel.Result[string], error) {
			return svc.ActualizarApp(ctx, "u1", ParcheApp{Name: "  "})
		},
	}

	for nombre, f := range casos {
		t.Run(nombre, func(t *testing.T) {
			cliente.llamadas = nil
			res, err := f()
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if res.OK {
				t.Error("lo dio por bueno")
			}
			if len(cliente.llamadas) != 0 {
				t.Errorf("llamó a Coolify igualmente: %v", cliente.llamadas)
			}
		})
	}
}

func TestFalloDeBaseSubeComoError(t *testing.T) {
	repo := &repoFalso{fallo: errors.New("sin conexión")}
	if _, err := NewService(&clienteFalso{}, repo).Resumen(context.Background()); err == nil {
		t.Error("se tragó un fallo de base")
	}
}
