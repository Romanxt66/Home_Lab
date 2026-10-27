package calendar

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

type eventoDTO struct {
	ID                  string  `json:"id"`
	Title               string  `json:"title"`
	Description         *string `json:"description"`
	Start               string  `json:"start"`
	End                 *string `json:"end"`
	AllDay              bool    `json:"allDay"`
	Location            *string `json:"location"`
	Color               *string `json:"color"`
	RemindMinutesBefore *int    `json:"remindMinutesBefore"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/events", h.mes)
	g.POST("/events", h.guardar)
	g.PUT("/events/:id", h.guardar)
	g.DELETE("/events/:id", h.borrar)
}

func (h *Handler) mes(c *gin.Context) {
	ahora := time.Now()
	año, err := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(ahora.Year())))
	if err != nil {
		httpx.BadRequest(c, "El año no es válido.")
		return
	}
	mes, err := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(ahora.Month()))))
	if err != nil || mes < 1 || mes > 12 {
		httpx.BadRequest(c, "El mes debe estar entre 1 y 12.")
		return
	}

	eventos, err := h.svc.Mes(c.Request.Context(), año, mes)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]eventoDTO, 0, len(eventos))
	for _, e := range eventos {
		salida = append(salida, aDTO(e))
	}
	c.JSON(http.StatusOK, salida)
}

type cuerpoEvento struct {
	Title               string  `json:"title"`
	Description         *string `json:"description"`
	Start               string  `json:"start"`
	End                 *string `json:"end"`
	AllDay              bool    `json:"allDay"`
	Location            *string `json:"location"`
	Color               *string `json:"color"`
	RemindMinutesBefore *int    `json:"remindMinutesBefore"`
}

func (h *Handler) guardar(c *gin.Context) {
	var cuerpo cuerpoEvento
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	inicio, err := time.Parse(iso, cuerpo.Start)
	if err != nil {
		httpx.BadRequest(c, "La fecha de inicio no tiene un formato válido.")
		return
	}

	var fin *time.Time
	if cuerpo.End != nil && *cuerpo.End != "" {
		f, err := time.Parse(iso, *cuerpo.End)
		if err != nil {
			httpx.BadRequest(c, "La fecha de fin no tiene un formato válido.")
			return
		}
		fin = &f
	}

	res, err := h.svc.Guardar(c.Request.Context(), Evento{
		ID: c.Param("id"), Title: cuerpo.Title, Description: cuerpo.Description,
		Start: inicio, End: fin, AllDay: cuerpo.AllDay,
		Location: cuerpo.Location, Color: cuerpo.Color,
		RemindMinutesBefore: cuerpo.RemindMinutesBefore,
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

func aDTO(e Evento) eventoDTO {
	var fin *string
	if e.End != nil {
		s := e.End.Format(iso)
		fin = &s
	}
	return eventoDTO{
		ID: e.ID, Title: e.Title, Description: e.Description,
		Start: e.Start.Format(iso), End: fin, AllDay: e.AllDay,
		Location: e.Location, Color: e.Color,
		RemindMinutesBefore: e.RemindMinutesBefore,
	}
}
