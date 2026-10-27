package deploygen

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

type deteccionDTO struct {
	Runtime        string   `json:"runtime"`
	Framework      string   `json:"framework"`
	PackageManager string   `json:"packageManager"`
	RuntimeVersion string   `json:"runtimeVersion"`
	InstallCommand string   `json:"installCommand"`
	BuildCommand   string   `json:"buildCommand"`
	StartCommand   string   `json:"startCommand"`
	Port           int      `json:"port"`
	IsStatic       bool     `json:"isStatic"`
	PublishDir     string   `json:"publishDir"`
	NeedsDatabase  bool     `json:"needsDatabase"`
	DatabaseType   string   `json:"databaseType"`
	HasDockerfile  bool     `json:"hasDockerfile"`
	HasCompose     bool     `json:"hasCompose"`
	BuildPack      string   `json:"recommendedBuildPack"`
	Notas          []string `json:"notes"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.POST("/analyze", h.analizar)
	g.POST("/commit", h.commitear)
	g.GET("/database-template", h.plantillaDeBase)
}

func (h *Handler) analizar(c *gin.Context) {
	var cuerpo struct {
		RepoURL string `json:"repoUrl"`
		Branch  string `json:"branch"`
		Subdir  string `json:"subdir"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res := h.svc.Analizar(c.Request.Context(), cuerpo.RepoURL, cuerpo.Branch, cuerpo.Subdir)
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	a := res.Value
	c.JSON(http.StatusOK, gin.H{
		"owner":      a.Owner,
		"repo":       a.Repo,
		"branch":     a.Branch,
		"baseDir":    a.BaseDir,
		"detection":  aDeteccionDTO(a.Deteccion),
		"dockerfile": a.Dockerfile,
		"compose":    vacioANulo(a.Compose),
	})
}

func (h *Handler) commitear(c *gin.Context) {
	var cuerpo struct {
		RepoURL  string `json:"repoUrl"`
		Branch   string `json:"branch"`
		BaseDir  string `json:"baseDir"`
		FileName string `json:"fileName"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res := h.svc.Commitear(c.Request.Context(), cuerpo.RepoURL, cuerpo.Branch,
		cuerpo.BaseDir, cuerpo.FileName, cuerpo.Content)
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, gin.H{"mensaje": res.Value})
}

func (h *Handler) plantillaDeBase(c *gin.Context) {
	tipo := c.Query("type")
	c.JSON(http.StatusOK, gin.H{"compose": PlantillaDeBase(tipo)})
}

func aDeteccionDTO(d Deteccion) deteccionDTO {
	notas := d.Notas
	if notas == nil {
		notas = []string{}
	}
	return deteccionDTO{
		Runtime: string(d.Runtime), Framework: d.Framework,
		PackageManager: d.PackageManager, RuntimeVersion: d.RuntimeVersion,
		InstallCommand: d.InstallCommand, BuildCommand: d.BuildCommand,
		StartCommand: d.StartCommand, Port: d.Port,
		IsStatic: d.IsStatic, PublishDir: d.PublishDir,
		NeedsDatabase: d.NeedsDatabase, DatabaseType: string(d.DatabaseType),
		HasDockerfile: d.HasDockerfile, HasCompose: d.HasCompose,
		BuildPack: string(d.BuildPack), Notas: notas,
	}
}

// vacioANulo deja el compose como null en el JSON cuando no hace falta, que
// es más claro para la interfaz que una cadena vacía.
func vacioANulo(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
