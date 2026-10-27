// Package github enseña los repositorios de una cuenta de GitHub.
package github

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Config es la conexión guardada. El token es opcional: sin él solo se ven
// los repositorios públicos y el límite de peticiones es mucho más bajo.
type Config struct {
	ID        string
	Username  string
	Token     *string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Repo es un repositorio ya normalizado desde la forma que usa la API.
type Repo struct {
	ID            int64
	Name          string
	FullName      string
	Description   *string
	URL           string
	Homepage      *string
	IsPrivate     bool
	IsFork        bool
	IsArchived    bool
	Language      *string
	Stars         int
	Forks         int
	OpenIssues    int
	Topics        []string
	PushedAt      *time.Time
	UpdatedAt     *time.Time
	DefaultBranch string
}

// Perfil es la cuenta cuyos repositorios se enseñan.
type Perfil struct {
	Login       string
	Name        *string
	AvatarURL   string
	Bio         *string
	HTMLURL     string
	PublicRepos int
	Followers   int
	Following   int
	Company     *string
	Location    *string
	Blog        *string
}

// Las reglas de GitHub para los nombres de usuario: de 1 a 39 caracteres,
// letras, números y guiones sueltos, sin empezar ni acabar en guion.
var reUsuario = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-[A-Za-z0-9]){0,38}$`)

func ValidarUsuario(crudo string) kernel.Result[string] {
	u := strings.TrimSpace(crudo)
	if u == "" {
		return kernel.Err[string]("Escribe tu usuario de GitHub.")
	}
	if !reUsuario.MatchString(u) {
		return kernel.Err[string](
			"Ese usuario no es válido: letras, números y guiones sueltos, como mucho 39 caracteres.")
	}
	return kernel.Ok(u)
}

// NormalizarToken deja nulo un token vacío: así «sin token» es un solo valor
// y no hay que distinguir entre nulo y cadena vacía en el resto del código.
//
// No se comprueba el formato a propósito. Los clásicos empiezan por ghp_ y
// los finos por github_pat_, pero GitHub puede inventarse otro mañana; quien
// de verdad valida el token es GitHub respondiendo 401.
func NormalizarToken(crudo string) *string {
	t := strings.TrimSpace(crudo)
	if t == "" {
		return nil
	}
	return &t
}

// ── Ordenar y filtrar ──────────────────────────────────────────────────────

type Orden string

const (
	OrdenReciente  Orden = "recent"
	OrdenEstrellas Orden = "stars"
	OrdenNombre    Orden = "name"
)

func empujado(r Repo) int64 {
	if r.PushedAt == nil {
		return 0
	}
	return r.PushedAt.Unix()
}

// Ordenar devuelve una copia ordenada; la lista original no se toca.
func Ordenar(repos []Repo, orden Orden) []Repo {
	copia := slices.Clone(repos)

	switch orden {
	case OrdenEstrellas:
		// A igualdad de estrellas, primero el más reciente: así el orden es
		// estable y predecible en vez de depender del azar.
		slices.SortStableFunc(copia, func(a, b Repo) int {
			if c := cmp.Compare(b.Stars, a.Stars); c != 0 {
				return c
			}
			return cmp.Compare(empujado(b), empujado(a))
		})

	case OrdenNombre:
		slices.SortStableFunc(copia, func(a, b Repo) int {
			return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})

	default:
		slices.SortStableFunc(copia, func(a, b Repo) int {
			return cmp.Compare(empujado(b), empujado(a))
		})
	}

	return copia
}

// Filtro son los filtros que aplica la interfaz.
type Filtro struct {
	Texto             string
	IncluirForks      bool
	IncluirArchivados bool
}

// Filtrar busca en el nombre, la descripción y los temas, sin distinguir
// mayúsculas.
func Filtrar(repos []Repo, f Filtro) []Repo {
	texto := strings.ToLower(strings.TrimSpace(f.Texto))

	salida := make([]Repo, 0, len(repos))
	for _, r := range repos {
		if !f.IncluirForks && r.IsFork {
			continue
		}
		if !f.IncluirArchivados && r.IsArchived {
			continue
		}
		if texto != "" && !coincide(r, texto) {
			continue
		}
		salida = append(salida, r)
	}

	return salida
}

func coincide(r Repo, texto string) bool {
	trozos := []string{r.Name}
	if r.Description != nil {
		trozos = append(trozos, *r.Description)
	}
	trozos = append(trozos, r.Topics...)

	return strings.Contains(strings.ToLower(strings.Join(trozos, " ")), texto)
}
