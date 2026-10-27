package email

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

// cuentaDTO: ni el refresh token ni el de acceso salen al navegador.
type cuentaDTO struct {
	ID      string  `json:"id"`
	Email   string  `json:"email"`
	Name    *string `json:"name"`
	Picture *string `json:"picture"`
}

type plantillaDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	Variables []string `json:"variables"`
	UpdatedAt string   `json:"updatedAt"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.POST("/send", h.enviar)
	g.GET("/templates", h.plantillas)
	g.POST("/templates", h.guardarPlantilla)
	g.PUT("/templates/:id", h.guardarPlantilla)
	g.DELETE("/templates/:id", h.borrarPlantilla)
	g.GET("/accounts", h.cuentas)
	g.DELETE("/accounts/:id", h.desconectarCuenta)
}

func (h *Handler) enviar(c *gin.Context) {
	var cuerpo struct {
		Recipients      string            `json:"recipients"`
		Subject         string            `json:"subject"`
		Body            string            `json:"body"`
		Variables       map[string]string `json:"variables"`
		GoogleAccountID string            `json:"googleAccountId"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Enviar(c.Request.Context(), cuerpo.Recipients, cuerpo.Subject,
		cuerpo.Body, cuerpo.Variables, cuerpo.GoogleAccountID)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, gin.H{"sent": res.Value.Enviados, "failed": res.Value.Fallidos})
}

func (h *Handler) plantillas(c *gin.Context) {
	ps, err := h.svc.Plantillas(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]plantillaDTO, 0, len(ps))
	for _, p := range ps {
		salida = append(salida, aPlantillaDTO(p))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardarPlantilla(c *gin.Context) {
	var cuerpo struct {
		Name    string `json:"name"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.GuardarPlantilla(c.Request.Context(), Plantilla{
		ID: c.Param("id"), Name: cuerpo.Name, Subject: cuerpo.Subject, Body: cuerpo.Body,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aPlantillaDTO(res.Value))
}

func (h *Handler) borrarPlantilla(c *gin.Context) {
	if err := h.svc.BorrarPlantilla(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) cuentas(c *gin.Context) {
	cs, err := h.svc.Cuentas(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]cuentaDTO, 0, len(cs))
	for _, cu := range cs {
		salida = append(salida, cuentaDTO{ID: cu.ID, Email: cu.Email, Name: cu.Name, Picture: cu.Picture})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) desconectarCuenta(c *gin.Context) {
	if err := h.svc.DesconectarCuenta(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// aPlantillaDTO añade la lista de variables que usa la plantilla, para que la
// interfaz pueda pedir sus valores sin tener que buscarlas ella misma.
func aPlantillaDTO(p Plantilla) plantillaDTO {
	variables := Variables(p.Subject + " " + p.Body)
	if variables == nil {
		variables = []string{}
	}
	return plantillaDTO{
		ID: p.ID, Name: p.Name, Subject: p.Subject, Body: p.Body,
		Variables: variables,
		UpdatedAt: p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
