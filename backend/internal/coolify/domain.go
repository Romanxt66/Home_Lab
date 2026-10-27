// Package coolify es el panel de despliegues: ver qué hay levantado,
// arrancarlo, pararlo y crear recursos nuevos.
package coolify

import (
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Config es la conexión guardada. El token se queda en el servidor.
type Config struct {
	ID        string
	BaseURL   string
	Token     string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func ValidarBaseURL(crudo string) kernel.Result[string] {
	limpio := strings.TrimRight(strings.TrimSpace(crudo), "/")
	if limpio == "" {
		return kernel.Err[string]("Escribe la dirección de tu Coolify.")
	}

	u, err := url.Parse(limpio)
	if err != nil {
		return kernel.Err[string]("La dirección no es válida (por ejemplo https://coolify.midominio.com).")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return kernel.Err[string]("La dirección debe empezar por http:// o https://")
	}
	if u.Host == "" {
		return kernel.Err[string]("La dirección no tiene servidor.")
	}

	return kernel.Ok(limpio)
}

func ValidarToken(crudo string) kernel.Result[string] {
	t := strings.TrimSpace(crudo)
	if t == "" {
		return kernel.Err[string]("Escribe el API token de Coolify.")
	}
	return kernel.Ok(t)
}

// ── Estado de los recursos ─────────────────────────────────────────────────

type Estado string

const (
	EnMarcha     Estado = "running"
	Detenida     Estado = "stopped"
	ConProblemas Estado = "degraded"
	Desconocido  Estado = "unknown"
)

var EtiquetasDeEstado = map[Estado]string{
	EnMarcha:     "En marcha",
	Detenida:     "Detenida",
	ConProblemas: "Con problemas",
	Desconocido:  "Desconocido",
}

// InterpretarEstado traduce la cadena "fase:salud" que devuelve Coolify.
//
// Ejemplos reales: "running:healthy", "running:unhealthy", "exited:unhealthy",
// "stopped", "restarting". Se escribe a la defensiva porque Coolify ha
// cambiado ese formato entre versiones.
func InterpretarEstado(crudo string) (Estado, *bool) {
	texto := strings.ToLower(strings.TrimSpace(crudo))
	if texto == "" {
		return Desconocido, nil
	}

	fase, salud, haySalud := strings.Cut(texto, ":")

	var sano *bool
	if haySalud {
		// Ojo con el orden: "unhealthy" contiene "healthy", así que hay que
		// descartarlo antes de dar nada por sano.
		v := strings.Contains(salud, "healthy") && !strings.Contains(salud, "unhealthy")
		sano = &v
	}

	switch {
	case strings.HasPrefix(fase, "running"):
		if sano != nil && !*sano {
			return ConProblemas, sano
		}
		return EnMarcha, sano

	case strings.HasPrefix(fase, "exited"),
		strings.HasPrefix(fase, "stopped"),
		strings.HasPrefix(fase, "dead"):
		return Detenida, sano

	case strings.HasPrefix(fase, "restarting"), strings.HasPrefix(fase, "starting"):
		return ConProblemas, sano
	}

	return Desconocido, sano
}

// ── Despliegues ────────────────────────────────────────────────────────────

type EstadoDespliegue string

const (
	EnCola    EstadoDespliegue = "queued"
	EnCurso   EstadoDespliegue = "in_progress"
	Terminado EstadoDespliegue = "finished"
	Fallido   EstadoDespliegue = "failed"
	Cancelado EstadoDespliegue = "cancelled"
)

var EtiquetasDeDespliegue = map[EstadoDespliegue]string{
	EnCola:    "En cola",
	EnCurso:   "En curso",
	Terminado: "Completado",
	Fallido:   "Fallido",
	Cancelado: "Cancelado",
}

// InterpretarDespliegue normaliza el estado que devuelve Coolify, que varía
// entre versiones y a veces trae texto libre.
func InterpretarDespliegue(crudo string) EstadoDespliegue {
	s := strings.ToLower(crudo)

	switch {
	case strings.Contains(s, "progress"), strings.Contains(s, "running"):
		return EnCurso
	case strings.Contains(s, "queue"):
		return EnCola
	case strings.Contains(s, "fail"), strings.Contains(s, "error"):
		return Fallido
	case strings.Contains(s, "cancel"):
		return Cancelado
	case strings.Contains(s, "finish"), strings.Contains(s, "success"), strings.Contains(s, "done"):
		return Terminado
	}

	return EnCola
}

// ── Catálogos ──────────────────────────────────────────────────────────────

type Opcion struct {
	ID    string
	Label string
}

// TiposDeBase son los motores que Coolify sabe crear. El id es el tramo de
// la ruta en su API.
var TiposDeBase = []Opcion{
	{"postgresql", "PostgreSQL"},
	{"mysql", "MySQL"},
	{"mariadb", "MariaDB"},
	{"mongodb", "MongoDB"},
	{"redis", "Redis"},
	{"keydb", "KeyDB"},
	{"dragonfly", "Dragonfly"},
	{"clickhouse", "ClickHouse"},
}

var BuildPacks = []Opcion{
	{"nixpacks", "Nixpacks (automático)"},
	{"static", "Estático"},
	{"dockerfile", "Dockerfile"},
	{"dockercompose", "Docker Compose"},
}

func TipoDeBaseValido(id string) bool {
	for _, o := range TiposDeBase {
		if o.ID == id {
			return true
		}
	}
	return false
}

func BuildPackValido(id string) bool {
	for _, o := range BuildPacks {
		if o.ID == id {
			return true
		}
	}
	return false
}

// ── Recursos ───────────────────────────────────────────────────────────────

type App struct {
	UUID            string
	Name            string
	Description     *string
	FQDN            *string
	Status          string
	State           Estado
	Healthy         *bool
	GitRepository   *string
	GitBranch       *string
	BuildPack       *string
	ProjectName     *string
	EnvironmentName *string
	PortsExposes    string
	InstallCommand  string
	BuildCommand    string
	StartCommand    string
	BaseDirectory   string
}

type Proyecto struct {
	UUID        string
	Name        string
	Description *string
}

type Entorno struct {
	UUID string
	Name string
}

type Servidor struct {
	UUID      string
	Name      string
	IP        *string
	Reachable *bool
}

type Recurso struct {
	UUID   string
	Name   string
	Type   string
	Status string
	State  Estado
}

type Resumen struct {
	Apps       []App
	Proyectos  []Proyecto
	Servidores []Servidor
	Bases      []Recurso
	Servicios  []Recurso
}

type Despliegue struct {
	UUID            string
	ApplicationName string
	ServerName      *string
	Status          EstadoDespliegue
	RawStatus       string
	Commit          *string
	CommitMessage   *string
	CreatedAt       *string
}

type Variable struct {
	UUID        string
	Key         string
	Value       string
	IsBuildTime bool
	IsPreview   bool
	IsLiteral   bool
}

// OrdenarApps deja las que tienen problemas arriba: es lo que hay que mirar
// primero al abrir el panel.
func OrdenarApps(apps []App) []App {
	copia := make([]App, len(apps))
	copy(copia, apps)

	prioridad := map[Estado]int{
		ConProblemas: 0,
		Detenida:     1,
		Desconocido:  2,
		EnMarcha:     3,
	}

	sort.SliceStable(copia, func(i, j int) bool {
		pi, pj := prioridad[copia[i].State], prioridad[copia[j].State]
		if pi != pj {
			return pi < pj
		}
		return strings.ToLower(copia[i].Name) < strings.ToLower(copia[j].Name)
	})

	return copia
}
