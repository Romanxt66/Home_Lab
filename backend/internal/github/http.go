package github

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

// configDTO: el token no sale nunca, solo si hay uno guardado.
type configDTO struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	HasToken bool   `json:"hasToken"`
}

type repoDTO struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	FullName      string   `json:"fullName"`
	Description   *string  `json:"description"`
	URL           string   `json:"url"`
	Homepage      *string  `json:"homepage"`
	IsPrivate     bool     `json:"isPrivate"`
	IsFork        bool     `json:"isFork"`
	IsArchived    bool     `json:"isArchived"`
	Language      *string  `json:"language"`
	Stars         int      `json:"stars"`
	Forks         int      `json:"forks"`
	OpenIssues    int      `json:"openIssues"`
	Topics        []string `json:"topics"`
	PushedAt      *string  `json:"pushedAt"`
	DefaultBranch string   `json:"defaultBranch"`
}

type perfilDTO struct {
	Login       string  `json:"login"`
	Name        *string `json:"name"`
	AvatarURL   string  `json:"avatarUrl"`
	Bio         *string `json:"bio"`
	HTMLURL     string  `json:"htmlUrl"`
	PublicRepos int     `json:"publicRepos"`
	Followers   int     `json:"followers"`
	Following   int     `json:"following"`
	Company     *string `json:"company"`
	Location    *string `json:"location"`
	Blog        *string `json:"blog"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/config", h.config)
	g.POST("/config", h.guardar)
	g.DELETE("/config/:id", h.borrar)
	g.GET("/overview", h.resumen)
}

func (h *Handler) config(c *gin.Context) {
	cfg, err := h.svc.Config(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if cfg == nil {
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusOK, aConfigDTO(*cfg))
}

func (h *Handler) guardar(c *gin.Context) {
	var cuerpo struct {
		Username string `json:"username"`
		Token    string `json:"token"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Guardar(c.Request.Context(), cuerpo.Username, cuerpo.Token)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aConfigDTO(res.Value))
}

func (h *Handler) borrar(c *gin.Context) {
	if err := h.svc.Borrar(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// resumen acepta el orden y los filtros como parámetros, para que la
// interfaz no tenga que repetir esa lógica.
func (h *Handler) resumen(c *gin.Context) {
	res, err := h.svc.Resumen(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	repos := Filtrar(res.Value.Repos, Filtro{
		Texto:             c.Query("q"),
		IncluirForks:      c.DefaultQuery("forks", "true") == "true",
		IncluirArchivados: c.DefaultQuery("archived", "true") == "true",
	})
	repos = Ordenar(repos, Orden(c.DefaultQuery("sort", string(OrdenReciente))))

	salida := make([]repoDTO, 0, len(repos))
	for _, r := range repos {
		salida = append(salida, aRepoDTO(r))
	}

	c.JSON(http.StatusOK, gin.H{
		"profile": aPerfilDTO(res.Value.Perfil),
		"repos":   salida,
	})
}

func aConfigDTO(c Config) configDTO {
	return configDTO{ID: c.ID, Username: c.Username, HasToken: c.Token != nil}
}

func aPerfilDTO(p Perfil) perfilDTO {
	return perfilDTO{
		Login: p.Login, Name: p.Name, AvatarURL: p.AvatarURL, Bio: p.Bio,
		HTMLURL: p.HTMLURL, PublicRepos: p.PublicRepos,
		Followers: p.Followers, Following: p.Following,
		Company: p.Company, Location: p.Location, Blog: p.Blog,
	}
}

func aRepoDTO(r Repo) repoDTO {
	var empujado *string
	if r.PushedAt != nil {
		s := r.PushedAt.Format(iso)
		empujado = &s
	}
	return repoDTO{
		ID: r.ID, Name: r.Name, FullName: r.FullName, Description: r.Description,
		URL: r.URL, Homepage: r.Homepage, IsPrivate: r.IsPrivate,
		IsFork: r.IsFork, IsArchived: r.IsArchived, Language: r.Language,
		Stars: r.Stars, Forks: r.Forks, OpenIssues: r.OpenIssues,
		Topics: r.Topics, PushedAt: empujado, DefaultBranch: r.DefaultBranch,
	}
}
