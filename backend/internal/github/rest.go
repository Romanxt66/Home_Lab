package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const (
	api          = "https://api.github.com"
	tiempoLimite = 15 * time.Second
	porPagina    = 100
	// Tope de páginas: una cuenta enorme dejaría la petición colgada minutos.
	maxPaginas = 5
)

type RESTCliente struct {
	client *http.Client
}

func NewRESTCliente() *RESTCliente {
	return &RESTCliente{client: &http.Client{Timeout: tiempoLimite}}
}

func (g *RESTCliente) Perfil(ctx context.Context, c Credenciales) kernel.Result[Perfil] {
	cuerpo, res := g.pedir(ctx, api+"/users/"+url.PathEscape(c.Username), c.Token)
	if !res.OK {
		return kernel.Err[Perfil](res.Error)
	}

	var u usuarioCrudo
	if err := json.Unmarshal(cuerpo, &u); err != nil {
		return kernel.Err[Perfil]("GitHub devolvió una respuesta que no se entiende.")
	}

	return kernel.Ok(Perfil{
		Login: u.Login, Name: u.Name, AvatarURL: u.AvatarURL, Bio: u.Bio,
		HTMLURL: u.HTMLURL, PublicRepos: u.PublicRepos,
		Followers: u.Followers, Following: u.Following,
		Company: u.Company, Location: u.Location, Blog: vacioANulo(u.Blog),
	})
}

func (g *RESTCliente) Repos(ctx context.Context, c Credenciales) kernel.Result[[]Repo] {
	// Con token se puede listar lo del usuario autenticado, privados
	// incluidos; sin él solo queda lo público de ese nombre.
	var base string
	if c.Token != nil {
		base = api + "/user/repos?per_page=" + strconv.Itoa(porPagina) +
			"&sort=pushed&affiliation=owner,collaborator,organization_member"
	} else {
		base = api + "/users/" + url.PathEscape(c.Username) + "/repos?per_page=" +
			strconv.Itoa(porPagina) + "&sort=pushed&type=owner"
	}

	var todos []Repo
	for pagina := 1; pagina <= maxPaginas; pagina++ {
		cuerpo, res := g.pedir(ctx, base+"&page="+strconv.Itoa(pagina), c.Token)
		if !res.OK {
			return kernel.Err[[]Repo](res.Error)
		}

		var lote []repoCrudo
		if err := json.Unmarshal(cuerpo, &lote); err != nil {
			return kernel.Err[[]Repo]("GitHub devolvió una respuesta que no se entiende.")
		}

		for _, r := range lote {
			todos = append(todos, r.aDominio())
		}

		// Una página incompleta es la última.
		if len(lote) < porPagina {
			break
		}
	}

	if todos == nil {
		todos = []Repo{}
	}
	return kernel.Ok(todos)
}

// ── Las formas que manda GitHub ────────────────────────────────────────────

type usuarioCrudo struct {
	Login       string  `json:"login"`
	Name        *string `json:"name"`
	AvatarURL   string  `json:"avatar_url"`
	Bio         *string `json:"bio"`
	HTMLURL     string  `json:"html_url"`
	PublicRepos int     `json:"public_repos"`
	Followers   int     `json:"followers"`
	Following   int     `json:"following"`
	Company     *string `json:"company"`
	Location    *string `json:"location"`
	Blog        *string `json:"blog"`
}

type repoCrudo struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	FullName      string     `json:"full_name"`
	Description   *string    `json:"description"`
	HTMLURL       string     `json:"html_url"`
	Homepage      *string    `json:"homepage"`
	Private       bool       `json:"private"`
	Fork          bool       `json:"fork"`
	Archived      bool       `json:"archived"`
	Language      *string    `json:"language"`
	Stargazers    int        `json:"stargazers_count"`
	Forks         int        `json:"forks_count"`
	OpenIssues    int        `json:"open_issues_count"`
	Topics        []string   `json:"topics"`
	PushedAt      *time.Time `json:"pushed_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
	DefaultBranch string     `json:"default_branch"`
}

func (r repoCrudo) aDominio() Repo {
	temas := r.Topics
	if temas == nil {
		temas = []string{}
	}
	return Repo{
		ID: r.ID, Name: r.Name, FullName: r.FullName, Description: r.Description,
		URL: r.HTMLURL, Homepage: vacioANulo(r.Homepage),
		IsPrivate: r.Private, IsFork: r.Fork, IsArchived: r.Archived,
		Language: r.Language, Stars: r.Stargazers, Forks: r.Forks,
		OpenIssues: r.OpenIssues, Topics: temas,
		PushedAt: r.PushedAt, UpdatedAt: r.UpdatedAt,
		DefaultBranch: r.DefaultBranch,
	}
}

// vacioANulo trata "  " igual que ausente: GitHub devuelve cadenas vacías
// donde conceptualmente no hay dato.
func vacioANulo(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

// pedir hace la llamada y traduce el fallo.
func (g *RESTCliente) pedir(ctx context.Context, destino string, token *string) ([]byte, kernel.Result[struct{}]) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, destino, nil)
	if err != nil {
		return nil, kernel.Err[struct{}]("No se pudo preparar la petición a GitHub.")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+*token)
	}

	res, err := g.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return nil, kernel.Err[struct{}]("GitHub tardó más de 15 segundos en responder.")
		}
		return nil, kernel.Err[struct{}]("No se pudo conectar con GitHub.")
	}
	defer res.Body.Close()

	cuerpo, _ := io.ReadAll(io.LimitReader(res.Body, 16<<20))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, kernel.Err[struct{}](
			explicar(res.StatusCode, res.Header.Get("X-RateLimit-Remaining"), cuerpo))
	}

	return cuerpo, kernel.Ok(struct{}{})
}

// explicar convierte el estado en algo accionable.
func explicar(estado int, restantes string, cuerpo []byte) string {
	switch estado {
	case http.StatusUnauthorized:
		return "El token no vale o caducó. Revísalo en GitHub → Settings → Developer settings."
	case http.StatusNotFound:
		return "GitHub no encuentra ese usuario."
	case http.StatusForbidden:
		// Un 403 con el contador a cero es límite de peticiones, no falta
		// de permisos: la solución es distinta y conviene decirlo.
		if restantes == "0" {
			return "Se agotó el límite de peticiones de GitHub. Añade un token o espera un rato."
		}
	}

	var detalle struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(cuerpo, &detalle); err == nil && detalle.Message != "" {
		return "GitHub: " + detalle.Message
	}
	return fmt.Sprintf("GitHub respondió con un error (HTTP %d).", estado)
}
