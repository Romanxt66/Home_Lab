package notifications

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

// ConfigDTO es lo que ve el navegador. No lleva la credencial: solo dice si
// hay una guardada, que es lo único que la interfaz necesita saber.
type ConfigDTO struct {
	ID            string `json:"id"`
	Provider      string `json:"provider"`
	Recipient     string `json:"recipient"`
	Active        bool   `json:"active"`
	HasCredential bool   `json:"hasCredential"`
}

func toDTO(c Config) ConfigDTO {
	return ConfigDTO{
		ID:            c.ID,
		Provider:      string(c.Provider),
		Recipient:     c.Recipient,
		Active:        c.Active,
		HasCredential: c.Credential != "",
	}
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/config", h.active)
	g.POST("/config", h.save)
	g.DELETE("/config/:id", h.remove)
	g.POST("/test", h.test)
}

func (h *Handler) active(c *gin.Context) {
	config, err := h.svc.Active(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if config == nil {
		// null, no 404: "todavía no has configurado nada" es un estado
		// normal de la pantalla, no un error.
		c.JSON(http.StatusOK, nil)
		return
	}
	c.JSON(http.StatusOK, toDTO(*config))
}

type guardarRequest struct {
	ID string `json:"id"`
	// chat_id de Telegram
	Recipient string `json:"recipient"`
	// bot token; si llega vacío se conserva el guardado
	Credential string `json:"credential"`
}

func (h *Handler) save(c *gin.Context) {
	var req guardarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "El formulario no se pudo leer.")
		return
	}

	res, err := h.svc.SaveTelegram(c.Request.Context(), req.ID, req.Recipient, req.Credential)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, toDTO(res.Value))
}

func (h *Handler) remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) test(c *gin.Context) {
	res := h.svc.Test(c.Request.Context())
	if !res.OK {
		// 400: el fallo es de configuración, algo que el usuario puede
		// corregir, no una avería del servidor.
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"enviado": true})
}
