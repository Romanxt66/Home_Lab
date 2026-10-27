package automations

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

type reglaDTO struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Trigger   string   `json:"trigger"`
	Active    bool     `json:"active"`
	Actions   []Accion `json:"actions"`
	CreatedAt string   `json:"createdAt"`
}

type ejecucionDTO struct {
	OK    bool    `json:"ok"`
	Error *string `json:"error"`
	RanAt string  `json:"ranAt"`
}

type disparadorDTO struct {
	Tipo        string   `json:"type"`
	Etiqueta    string   `json:"label"`
	Descripcion string   `json:"description"`
	Variables   []string `json:"variables"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/triggers", h.disparadores)
	g.GET("/rules", h.list)
	g.POST("/rules", h.guardar)
	g.PUT("/rules/:id", h.guardar)
	g.DELETE("/rules/:id", h.borrar)
	g.GET("/rules/:id/runs", h.ejecuciones)
}

// disparadores publica el catálogo para que la interfaz no tenga que
// repetirlo por su cuenta y se desincronice.
func (h *Handler) disparadores(c *gin.Context) {
	salida := make([]disparadorDTO, 0, len(Disparadores))
	// Se recorre en orden fijo: un mapa de Go no garantiza el orden y la
	// lista bailaría en cada recarga.
	for _, d := range []Disparador{UptimeCaido, UptimeRecuperado, PresupuestoPasado, UsuarioRegistrado} {
		info := Disparadores[d]
		salida = append(salida, disparadorDTO{
			Tipo: string(d), Etiqueta: info.Etiqueta,
			Descripcion: info.Descripcion, Variables: info.Variables,
		})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) list(c *gin.Context) {
	reglas, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]reglaDTO, 0, len(reglas))
	for _, r := range reglas {
		salida = append(salida, aDTO(r))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardar(c *gin.Context) {
	var cuerpo struct {
		Name    string   `json:"name"`
		Trigger string   `json:"trigger"`
		Active  bool     `json:"active"`
		Actions []Accion `json:"actions"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Guardar(c.Request.Context(), Regla{
		ID: c.Param("id"), Name: cuerpo.Name, Trigger: Disparador(cuerpo.Trigger),
		Active: cuerpo.Active, Actions: cuerpo.Actions,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aDTO(res.Value))
}

func (h *Handler) borrar(c *gin.Context) {
	if err := h.svc.Borrar(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ejecuciones(c *gin.Context) {
	limite, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	runs, err := h.svc.Ejecuciones(c.Request.Context(), c.Param("id"), limite)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]ejecucionDTO, 0, len(runs))
	for _, r := range runs {
		salida = append(salida, ejecucionDTO{OK: r.OK, Error: r.Error, RanAt: r.RanAt.Format(iso)})
	}
	c.JSON(http.StatusOK, salida)
}

func aDTO(r Regla) reglaDTO {
	acciones := r.Actions
	if acciones == nil {
		acciones = []Accion{}
	}
	return reglaDTO{
		ID: r.ID, Name: r.Name, Trigger: string(r.Trigger), Active: r.Active,
		Actions: acciones, CreatedAt: r.CreatedAt.Format(iso),
	}
}
