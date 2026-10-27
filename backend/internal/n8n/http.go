package n8n

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

// configDTO es lo que ve el navegador. El API key no sale nunca: solo se
// dice si hay uno guardado, que es lo único que la interfaz necesita.
type configDTO struct {
	ID        string `json:"id"`
	BaseURL   string `json:"baseUrl"`
	HasAPIKey bool   `json:"hasApiKey"`
}

type workflowDTO struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Active    bool    `json:"active"`
	UpdatedAt *string `json:"updatedAt"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/config", h.config)
	g.POST("/config", h.conectar)
	g.DELETE("/config", h.desconectar)
	g.GET("/workflows", h.workflows)
	g.PUT("/workflows/:id/active", h.activar)
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

func (h *Handler) conectar(c *gin.Context) {
	var cuerpo struct {
		BaseURL string `json:"baseUrl"`
		APIKey  string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Conectar(c.Request.Context(), cuerpo.BaseURL, cuerpo.APIKey)
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

func (h *Handler) desconectar(c *gin.Context) {
	if err := h.svc.Desconectar(c.Request.Context()); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) workflows(c *gin.Context) {
	res, err := h.svc.ListarWorkflows(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	salida := make([]workflowDTO, 0, len(res.Value))
	for _, w := range res.Value {
		salida = append(salida, workflowDTO{ID: w.ID, Name: w.Name, Active: w.Active, UpdatedAt: w.UpdatedAt})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) activar(c *gin.Context) {
	var cuerpo struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Activar(c.Request.Context(), c.Param("id"), cuerpo.Active)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.Status(http.StatusNoContent)
}

func aConfigDTO(c Config) configDTO {
	return configDTO{ID: c.ID, BaseURL: c.BaseURL, HasAPIKey: c.APIKey != ""}
}
