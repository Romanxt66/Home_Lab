package assistant

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
	g.POST("/chat", h.conversar)
}

func (h *Handler) conversar(c *gin.Context) {
	usuario, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}

	var cuerpo struct {
		History []Mensaje `json:"history"`
		Message string    `json:"message"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	// El rol y el identificador salen de la sesión firmada, NUNCA del cuerpo:
	// lo que viene del navegador no se puede creer, y de este rol depende qué
	// herramientas puede usar el asistente en nombre del usuario.
	res := h.svc.Conversar(c.Request.Context(), cuerpo.History, cuerpo.Message,
		usuario.UID, usuario.Role)
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	usadas := res.Value.ToolsUsed
	if usadas == nil {
		usadas = []string{}
	}

	var navegarA *string
	if res.Value.NavigateTo != "" {
		navegarA = &res.Value.NavigateTo
	}

	c.JSON(http.StatusOK, gin.H{
		"reply":      res.Value.Reply,
		"toolsUsed":  usadas,
		"navigateTo": navegarA,
	})
}
