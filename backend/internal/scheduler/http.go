package scheduler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

type jobDTO struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Cron      string          `json:"cron"`
	Action    string          `json:"action"`
	Params    json.RawMessage `json:"params"`
	Enabled   bool            `json:"enabled"`
	CreatedAt string          `json:"createdAt"`
}

type runDTO struct {
	ID        string  `json:"id"`
	JobID     string  `json:"jobId"`
	JobName   string  `json:"jobName"`
	Status    string  `json:"status"`
	Output    *string `json:"output"`
	StartedAt string  `json:"startedAt"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/jobs", h.list)
	g.POST("/jobs", h.save)
	g.PUT("/jobs/:id", h.save)
	g.PUT("/jobs/:id/enabled", h.toggle)
	g.DELETE("/jobs/:id", h.remove)
	g.POST("/jobs/:id/run", h.run)
	g.GET("/runs", h.runs)
}

func (h *Handler) list(c *gin.Context) {
	jobs, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]jobDTO, 0, len(jobs))
	for _, j := range jobs {
		salida = append(salida, aJobDTO(j))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) save(c *gin.Context) {
	var cuerpo struct {
		Name    string          `json:"name"`
		Cron    string          `json:"cron"`
		Action  string          `json:"action"`
		Params  json.RawMessage `json:"params"`
		Enabled bool            `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	job := Job{
		ID:      c.Param("id"), // vacío al crear
		Name:    cuerpo.Name,
		Cron:    cuerpo.Cron,
		Action:  Accion(cuerpo.Action),
		Params:  string(cuerpo.Params),
		Enabled: cuerpo.Enabled,
	}

	res, err := h.svc.Save(c.Request.Context(), job)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aJobDTO(res.Value))
}

func (h *Handler) toggle(c *gin.Context) {
	var cuerpo struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Toggle(c.Request.Context(), c.Param("id"), cuerpo.Enabled)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aJobDTO(res.Value))
}

func (h *Handler) remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) run(c *gin.Context) {
	res, err := h.svc.Ejecutar(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": res.Value})
}

func (h *Handler) runs(c *gin.Context) {
	limite, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	runs, err := h.svc.Runs(c.Request.Context(), limite)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]runDTO, 0, len(runs))
	for _, r := range runs {
		salida = append(salida, runDTO{
			ID: r.ID, JobID: r.JobID, JobName: r.JobName,
			Status: r.Status, Output: r.Output,
			StartedAt: r.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	c.JSON(http.StatusOK, salida)
}

// RunDue es el disparador por HTTP, para quien prefiera un cron externo al
// ticker interno.
func (h *Handler) RunDue(c *gin.Context) {
	lanzadas, err := h.svc.EjecutarPendientes(c.Request.Context(), ahora())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"lanzadas": lanzadas})
}

func aJobDTO(j Job) jobDTO {
	params := json.RawMessage(j.Params)
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	return jobDTO{
		ID: j.ID, Name: j.Name, Cron: j.Cron, Action: string(j.Action),
		Params: params, Enabled: j.Enabled,
		CreatedAt: j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
