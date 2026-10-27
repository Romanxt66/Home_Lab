package deploygen

import (
	"context"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Lector trae los archivos del repositorio y escribe en él.
type Lector interface {
	Leer(ctx context.Context, repoURL, rama, subdir string, token *string) kernel.Result[RepoLeido]
	Escribir(ctx context.Context, repoURL, rama, ruta, contenido, mensaje string, token *string) kernel.Result[string]
}

// TokenDeGitHub resuelve el token guardado, si lo hay. Se recibe como
// función para que este módulo no dependa del de GitHub.
type TokenDeGitHub func(ctx context.Context) *string

// Analisis es el resultado de mirar un repositorio.
type Analisis struct {
	Owner      string
	Repo       string
	Branch     string
	BaseDir    string
	Deteccion  Deteccion
	Dockerfile string
	Compose    string
}

type Service struct {
	lector Lector
	token  TokenDeGitHub
}

func NewService(lector Lector, token TokenDeGitHub) *Service {
	return &Service{lector: lector, token: token}
}

// Analizar mira el repositorio y genera el Dockerfile.
func (s *Service) Analizar(ctx context.Context, repoURL, rama, subdir string) kernel.Result[Analisis] {
	leido := s.lector.Leer(ctx, repoURL, rama, subdir, s.token(ctx))
	if !leido.OK {
		return kernel.Err[Analisis](leido.Error)
	}

	deteccion := Detectar(leido.Value.Archivos)

	return kernel.Ok(Analisis{
		Owner:      leido.Value.Owner,
		Repo:       leido.Value.Repo,
		Branch:     leido.Value.Branch,
		BaseDir:    strings.Trim(strings.TrimSpace(subdir), "/"),
		Deteccion:  deteccion,
		Dockerfile: GenerarDockerfile(deteccion),
		Compose:    GenerarCompose(deteccion),
	})
}

// Commitear sube el archivo generado al repositorio, para poder desplegarlo
// con el build pack de Docker.
func (s *Service) Commitear(ctx context.Context, repoURL, rama, baseDir, nombre, contenido string) kernel.Result[string] {
	if strings.TrimSpace(nombre) == "" {
		return kernel.Err[string]("Falta el nombre del archivo.")
	}
	if strings.TrimSpace(contenido) == "" {
		return kernel.Err[string]("El archivo está vacío; no se sube nada.")
	}
	if strings.TrimSpace(rama) == "" {
		return kernel.Err[string]("Falta la rama en la que escribir.")
	}

	dir := strings.Trim(strings.TrimSpace(baseDir), "/")
	ruta := nombre
	if dir != "" {
		ruta = dir + "/" + nombre
	}

	return s.lector.Escribir(ctx, repoURL, rama, ruta, contenido,
		"chore: añadir "+nombre+" (generado por Home Lab)", s.token(ctx))
}
