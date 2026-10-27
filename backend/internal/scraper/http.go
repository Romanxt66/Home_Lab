package scraper

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.POST("", h.ejecutar)
}

type peticion struct {
	URL      string `json:"url"`
	Selector string `json:"selector"`
	Atributo string `json:"attribute"`
}

type respuesta struct {
	Status      int      `json:"status"`
	ContentType string   `json:"contentType"`
	Elementos   []string `json:"items,omitempty"`
	Crudo       string   `json:"raw,omitempty"`
}

func (h *Handler) ejecutar(c *gin.Context) {
	var p peticion
	if err := c.ShouldBindJSON(&p); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res := h.svc.Ejecutar(c.Request.Context(), p.URL, p.Selector, p.Atributo)
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, respuesta{
		Status:      res.Value.Status,
		ContentType: res.Value.ContentType,
		Elementos:   res.Value.Elementos,
		Crudo:       res.Value.Crudo,
	})
}
