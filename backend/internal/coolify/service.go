package coolify

import (
	"context"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Credenciales es lo que necesita el cliente para hablar con la instancia.
type Credenciales struct {
	BaseURL string
	Token   string
}

// Accion es lo que se le puede hacer a algo que corre.
type Accion string

const (
	Arrancar  Accion = "start"
	Parar     Accion = "stop"
	Reiniciar Accion = "restart"
)

func AccionValida(a Accion) bool {
	return a == Arrancar || a == Parar || a == Reiniciar
}

// TipoRecurso es el tramo de la ruta en la API de Coolify.
type TipoRecurso string

const (
	Bases     TipoRecurso = "databases"
	Servicios TipoRecurso = "services"
)

func TipoRecursoValido(t TipoRecurso) bool {
	return t == Bases || t == Servicios
}

// TipoBorrable amplía lo anterior con lo que además se puede borrar.
type TipoBorrable string

const (
	BorrarApp      TipoBorrable = "applications"
	BorrarBase     TipoBorrable = "databases"
	BorrarServicio TipoBorrable = "services"
	BorrarProyecto TipoBorrable = "projects"
)

func TipoBorrableValido(t TipoBorrable) bool {
	switch t {
	case BorrarApp, BorrarBase, BorrarServicio, BorrarProyecto:
		return true
	}
	return false
}

// ── Entradas ───────────────────────────────────────────────────────────────

type NuevaApp struct {
	Source          string // "public" o "dockerfile"
	Name            string
	ProjectUUID     string
	EnvironmentName string
	EnvironmentUUID string
	ServerUUID      string
	Domains         string
	PortsExposes    string
	InstantDeploy   bool
	BaseDirectory   string
	GitRepository   string
	GitBranch       string
	BuildPack       string
	Dockerfile      string
}

type ParcheApp struct {
	Name           string
	Description    string
	Domains        string
	GitBranch      string
	BuildPack      string
	PortsExposes   string
	InstallCommand string
	BuildCommand   string
	StartCommand   string
	BaseDirectory  string
}

type NuevaBase struct {
	Type            string
	Name            string
	ProjectUUID     string
	EnvironmentName string
	EnvironmentUUID string
	ServerUUID      string
	Image           string
	InstantDeploy   bool
}

type NuevoServicio struct {
	Name            string
	ProjectUUID     string
	EnvironmentName string
	EnvironmentUUID string
	ServerUUID      string
	DockerCompose   string
	InstantDeploy   bool
}

type NuevaVariable struct {
	Key       string
	Value     string
	IsPreview bool
	IsLiteral bool
}

// ── Puertos ────────────────────────────────────────────────────────────────

type Cliente interface {
	Ping(ctx context.Context, c Credenciales) kernel.Result[string]
	Resumen(ctx context.Context, c Credenciales) kernel.Result[Resumen]
	Entornos(ctx context.Context, c Credenciales, proyectoUUID string) kernel.Result[[]Entorno]

	CrearApp(ctx context.Context, c Credenciales, in NuevaApp) kernel.Result[string]
	App(ctx context.Context, c Credenciales, uuid string) kernel.Result[App]
	ActualizarApp(ctx context.Context, c Credenciales, uuid string, p ParcheApp) kernel.Result[string]
	Desplegar(ctx context.Context, c Credenciales, uuid string, forzar bool) kernel.Result[string]
	Controlar(ctx context.Context, c Credenciales, uuid string, a Accion) kernel.Result[string]
	Registros(ctx context.Context, c Credenciales, uuid string, lineas int) kernel.Result[string]

	Variables(ctx context.Context, c Credenciales, uuid string) kernel.Result[[]Variable]
	CrearVariable(ctx context.Context, c Credenciales, uuid string, in NuevaVariable) kernel.Result[string]
	ActualizarVariable(ctx context.Context, c Credenciales, uuid string, in NuevaVariable) kernel.Result[string]
	BorrarVariable(ctx context.Context, c Credenciales, uuid, varUUID string) kernel.Result[string]

	Despliegues(ctx context.Context, c Credenciales) kernel.Result[[]Despliegue]
	CancelarDespliegue(ctx context.Context, c Credenciales, uuid string) kernel.Result[string]

	ControlarRecurso(ctx context.Context, c Credenciales, tipo TipoRecurso, uuid string, a Accion) kernel.Result[string]
	RegistrosRecurso(ctx context.Context, c Credenciales, tipo TipoRecurso, uuid string, lineas int) kernel.Result[string]

	CrearBase(ctx context.Context, c Credenciales, in NuevaBase) kernel.Result[string]
	CrearServicio(ctx context.Context, c Credenciales, in NuevoServicio) kernel.Result[string]
	CrearProyecto(ctx context.Context, c Credenciales, nombre, descripcion string) kernel.Result[string]
	ActualizarProyecto(ctx context.Context, c Credenciales, uuid, nombre, descripcion string) kernel.Result[string]
	BorrarRecurso(ctx context.Context, c Credenciales, tipo TipoBorrable, uuid string) kernel.Result[string]
}

type ConfigRepo interface {
	GetActive(ctx context.Context) (*Config, error)
	Upsert(ctx context.Context, baseURL, token string) (*Config, error)
	Remove(ctx context.Context, id string) error
}

// ── Servicio ───────────────────────────────────────────────────────────────

type Service struct {
	cliente Cliente
	configs ConfigRepo
}

func NewService(cliente Cliente, configs ConfigRepo) *Service {
	return &Service{cliente: cliente, configs: configs}
}

func (s *Service) Config(ctx context.Context) (*Config, error) {
	return s.configs.GetActive(ctx)
}

// Conectar comprueba la credencial antes de guardarla.
func (s *Service) Conectar(ctx context.Context, baseURL, token string) (kernel.Result[Config], error) {
	url := ValidarBaseURL(baseURL)
	if !url.OK {
		return kernel.Err[Config](url.Error), nil
	}
	tok := ValidarToken(token)
	if !tok.OK {
		return kernel.Err[Config](tok.Error), nil
	}

	cred := Credenciales{BaseURL: url.Value, Token: tok.Value}
	if ping := s.cliente.Ping(ctx, cred); !ping.OK {
		return kernel.Err[Config](ping.Error), nil
	}

	guardada, err := s.configs.Upsert(ctx, cred.BaseURL, cred.Token)
	if err != nil {
		return kernel.Result[Config]{}, err
	}
	return kernel.Ok(*guardada), nil
}

func (s *Service) Desconectar(ctx context.Context) error {
	cfg, err := s.configs.GetActive(ctx)
	if err != nil || cfg == nil {
		return err
	}
	return s.configs.Remove(ctx, cfg.ID)
}

// credenciales resuelve la conexión activa, o explica que falta.
func (s *Service) credenciales(ctx context.Context) (Credenciales, string, error) {
	cfg, err := s.configs.GetActive(ctx)
	if err != nil {
		return Credenciales{}, "", err
	}
	if cfg == nil {
		return Credenciales{}, "No hay conexión con Coolify. Conéctala primero.", nil
	}
	return Credenciales{BaseURL: cfg.BaseURL, Token: cfg.Token}, "", nil
}

// conCredenciales ahorra repetir en cada operación el mismo preámbulo de
// resolver la conexión y decidir qué hacer si no hay.
func conCredenciales[T any](ctx context.Context, s *Service, f func(Credenciales) kernel.Result[T]) (kernel.Result[T], error) {
	cred, falta, err := s.credenciales(ctx)
	if err != nil {
		return kernel.Result[T]{}, err
	}
	if falta != "" {
		return kernel.Err[T](falta), nil
	}
	return f(cred), nil
}

// ── Lecturas ───────────────────────────────────────────────────────────────

// Resumen trae todo el panel, con las aplicaciones ya ordenadas para que las
// que tienen problemas salgan arriba.
func (s *Service) Resumen(ctx context.Context) (kernel.Result[Resumen], error) {
	res, err := conCredenciales(ctx, s, func(c Credenciales) kernel.Result[Resumen] {
		return s.cliente.Resumen(ctx, c)
	})
	if err != nil || !res.OK {
		return res, err
	}

	res.Value.Apps = OrdenarApps(res.Value.Apps)
	return res, nil
}

func (s *Service) Entornos(ctx context.Context, proyectoUUID string) (kernel.Result[[]Entorno], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[[]Entorno] {
		return s.cliente.Entornos(ctx, c, proyectoUUID)
	})
}

func (s *Service) App(ctx context.Context, uuid string) (kernel.Result[App], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[App] {
		return s.cliente.App(ctx, c, uuid)
	})
}

func (s *Service) Variables(ctx context.Context, uuid string) (kernel.Result[[]Variable], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[[]Variable] {
		return s.cliente.Variables(ctx, c, uuid)
	})
}

func (s *Service) Despliegues(ctx context.Context) (kernel.Result[[]Despliegue], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[[]Despliegue] {
		return s.cliente.Despliegues(ctx, c)
	})
}

// Registros acota las líneas que se piden: sin tope, un contenedor hablador
// devolvería megas de texto a una pantalla.
func (s *Service) Registros(ctx context.Context, uuid string, lineas int) (kernel.Result[string], error) {
	lineas = acotarLineas(lineas)
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.Registros(ctx, c, uuid, lineas)
	})
}

func (s *Service) RegistrosRecurso(ctx context.Context, tipo TipoRecurso, uuid string, lineas int) (kernel.Result[string], error) {
	if !TipoRecursoValido(tipo) {
		return kernel.Err[string]("Ese tipo de recurso no existe."), nil
	}

	lineas = acotarLineas(lineas)
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.RegistrosRecurso(ctx, c, tipo, uuid, lineas)
	})
}

func acotarLineas(n int) int {
	if n <= 0 || n > 2000 {
		return 200
	}
	return n
}

// ── Acciones ───────────────────────────────────────────────────────────────

func (s *Service) Desplegar(ctx context.Context, uuid string, forzar bool) (kernel.Result[string], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.Desplegar(ctx, c, uuid, forzar)
	})
}

func (s *Service) Controlar(ctx context.Context, uuid string, a Accion) (kernel.Result[string], error) {
	if !AccionValida(a) {
		return kernel.Err[string]("Esa acción no existe: arrancar, parar o reiniciar."), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.Controlar(ctx, c, uuid, a)
	})
}

func (s *Service) ControlarRecurso(ctx context.Context, tipo TipoRecurso, uuid string, a Accion) (kernel.Result[string], error) {
	if !TipoRecursoValido(tipo) {
		return kernel.Err[string]("Ese tipo de recurso no existe."), nil
	}
	if !AccionValida(a) {
		return kernel.Err[string]("Esa acción no existe: arrancar, parar o reiniciar."), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.ControlarRecurso(ctx, c, tipo, uuid, a)
	})
}

func (s *Service) CancelarDespliegue(ctx context.Context, uuid string) (kernel.Result[string], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.CancelarDespliegue(ctx, c, uuid)
	})
}

// ── Creación ───────────────────────────────────────────────────────────────

// ValidarNuevaApp comprueba lo que se puede comprobar sin llamar a Coolify.
func ValidarNuevaApp(in NuevaApp) kernel.Result[NuevaApp] {
	if in.ProjectUUID == "" {
		return kernel.Err[NuevaApp]("Elige un proyecto.")
	}
	if in.EnvironmentUUID == "" {
		return kernel.Err[NuevaApp]("Elige un entorno.")
	}
	if in.ServerUUID == "" {
		return kernel.Err[NuevaApp]("Elige un servidor.")
	}

	switch in.Source {
	case "public":
		if strings.TrimSpace(in.GitRepository) == "" {
			return kernel.Err[NuevaApp]("Falta el repositorio de Git.")
		}
		if strings.TrimSpace(in.GitBranch) == "" {
			return kernel.Err[NuevaApp]("Falta la rama.")
		}
		if in.BuildPack != "" && !BuildPackValido(in.BuildPack) {
			return kernel.Err[NuevaApp]("Ese build pack no existe.")
		}
	case "dockerfile":
		if strings.TrimSpace(in.Dockerfile) == "" {
			return kernel.Err[NuevaApp]("Pega el contenido del Dockerfile.")
		}
	default:
		return kernel.Err[NuevaApp]("El origen debe ser un repositorio público o un Dockerfile.")
	}

	return kernel.Ok(in)
}

func (s *Service) CrearApp(ctx context.Context, in NuevaApp) (kernel.Result[string], error) {
	if v := ValidarNuevaApp(in); !v.OK {
		return kernel.Err[string](v.Error), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.CrearApp(ctx, c, in)
	})
}

func (s *Service) ActualizarApp(ctx context.Context, uuid string, p ParcheApp) (kernel.Result[string], error) {
	if strings.TrimSpace(p.Name) == "" {
		return kernel.Err[string]("El nombre no puede estar vacío."), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.ActualizarApp(ctx, c, uuid, p)
	})
}

func (s *Service) CrearBase(ctx context.Context, in NuevaBase) (kernel.Result[string], error) {
	switch {
	case in.ProjectUUID == "":
		return kernel.Err[string]("Elige un proyecto."), nil
	case in.EnvironmentUUID == "":
		return kernel.Err[string]("Elige un entorno."), nil
	case in.ServerUUID == "":
		return kernel.Err[string]("Elige un servidor."), nil
	case in.Type == "":
		return kernel.Err[string]("Elige el tipo de base de datos."), nil
	case !TipoDeBaseValido(in.Type):
		return kernel.Err[string]("Coolify no sabe crear ese tipo de base de datos."), nil
	}

	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.CrearBase(ctx, c, in)
	})
}

func (s *Service) CrearServicio(ctx context.Context, in NuevoServicio) (kernel.Result[string], error) {
	switch {
	case in.ProjectUUID == "":
		return kernel.Err[string]("Elige un proyecto."), nil
	case in.EnvironmentUUID == "":
		return kernel.Err[string]("Elige un entorno."), nil
	case in.ServerUUID == "":
		return kernel.Err[string]("Elige un servidor."), nil
	case strings.TrimSpace(in.DockerCompose) == "":
		return kernel.Err[string]("Pega el docker-compose."), nil
	}

	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.CrearServicio(ctx, c, in)
	})
}

func (s *Service) CrearProyecto(ctx context.Context, nombre, descripcion string) (kernel.Result[string], error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return kernel.Err[string]("El proyecto necesita un nombre."), nil
	}

	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.CrearProyecto(ctx, c, nombre, strings.TrimSpace(descripcion))
	})
}

func (s *Service) ActualizarProyecto(ctx context.Context, uuid, nombre, descripcion string) (kernel.Result[string], error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return kernel.Err[string]("El proyecto necesita un nombre."), nil
	}

	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.ActualizarProyecto(ctx, c, uuid, nombre, strings.TrimSpace(descripcion))
	})
}

func (s *Service) BorrarRecurso(ctx context.Context, tipo TipoBorrable, uuid string) (kernel.Result[string], error) {
	if !TipoBorrableValido(tipo) {
		return kernel.Err[string]("Ese tipo de recurso no se puede borrar desde aquí."), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.BorrarRecurso(ctx, c, tipo, uuid)
	})
}

// ── Variables de entorno ───────────────────────────────────────────────────

func (s *Service) CrearVariable(ctx context.Context, uuid string, in NuevaVariable) (kernel.Result[string], error) {
	in.Key = strings.TrimSpace(in.Key)
	if in.Key == "" {
		return kernel.Err[string]("La clave no puede estar vacía."), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.CrearVariable(ctx, c, uuid, in)
	})
}

func (s *Service) ActualizarVariable(ctx context.Context, uuid string, in NuevaVariable) (kernel.Result[string], error) {
	in.Key = strings.TrimSpace(in.Key)
	if in.Key == "" {
		return kernel.Err[string]("La clave no puede estar vacía."), nil
	}
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.ActualizarVariable(ctx, c, uuid, in)
	})
}

func (s *Service) BorrarVariable(ctx context.Context, uuid, varUUID string) (kernel.Result[string], error) {
	return conCredenciales(ctx, s, func(c Credenciales) kernel.Result[string] {
		return s.cliente.BorrarVariable(ctx, c, uuid, varUUID)
	})
}
