package uptime

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

// ── DTOs ───────────────────────────────────────────────────────────────────
// Lo que viaja al navegador. Son los mismos campos que devolvían las Server
// Actions del Lab, así los componentes de React apenas cambian.

type MonitorDTO struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	URL                 string  `json:"url"`
	Method              string  `json:"method"`
	ExpectedStatus      int     `json:"expectedStatus"`
	IntervalSeconds     int     `json:"intervalSeconds"`
	TimeoutMs           int     `json:"timeoutMs"`
	Active              bool    `json:"active"`
	NotifyOnFailure     bool    `json:"notifyOnFailure"`
	LastStatus          string  `json:"lastStatus"`
	LastCheckedAt       *string `json:"lastCheckedAt"`
	LastResponseMs      *int    `json:"lastResponseMs"`
	LastStatusCode      *int    `json:"lastStatusCode"`
	LastError           *string `json:"lastError"`
	ConsecutiveFailures int     `json:"consecutiveFailures"`
}

type CheckDTO struct {
	OK         bool    `json:"ok"`
	StatusCode *int    `json:"statusCode"`
	ResponseMs *int    `json:"responseMs"`
	Error      *string `json:"error"`
	CheckedAt  string  `json:"checkedAt"`
}

type DetailDTO struct {
	Monitor   MonitorDTO `json:"monitor"`
	Checks    []CheckDTO `json:"checks"`
	Uptime24h float64    `json:"uptime24h"`
}

func toMonitorDTO(m Monitor) MonitorDTO {
	dto := MonitorDTO{
		ID:                  m.ID,
		Name:                m.Name,
		URL:                 m.URL,
		Method:              string(m.Method),
		ExpectedStatus:      m.ExpectedStatus,
		IntervalSeconds:     m.IntervalSeconds,
		TimeoutMs:           m.TimeoutMs,
		Active:              m.Active,
		NotifyOnFailure:     m.NotifyOnFailure,
		LastStatus:          string(m.LastStatus),
		LastResponseMs:      m.LastResponseMs,
		LastStatusCode:      m.LastStatusCode,
		LastError:           m.LastError,
		ConsecutiveFailures: m.ConsecutiveFailures,
	}
	if m.LastCheckedAt != nil {
		iso := m.LastCheckedAt.UTC().Format(time.RFC3339)
		dto.LastCheckedAt = &iso
	}
	return dto
}

func toCheckDTO(c Check) CheckDTO {
	return CheckDTO{
		OK:         c.OK,
		StatusCode: c.StatusCode,
		ResponseMs: c.ResponseMs,
		Error:      c.Error,
		CheckedAt:  c.CheckedAt.UTC().Format(time.RFC3339),
	}
}

// monitorRequest es el cuerpo que manda el formulario.
type monitorRequest struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Method          string `json:"method"`
	ExpectedStatus  int    `json:"expectedStatus"`
	IntervalSeconds int    `json:"intervalSeconds"`
	TimeoutMs       int    `json:"timeoutMs"`
	Active          bool   `json:"active"`
	NotifyOnFailure bool   `json:"notifyOnFailure"`
}

func (r monitorRequest) aDominio() MonitorInput {
	return MonitorInput{
		Name:            r.Name,
		URL:             r.URL,
		Method:          Method(r.Method),
		ExpectedStatus:  r.ExpectedStatus,
		IntervalSeconds: r.IntervalSeconds,
		TimeoutMs:       r.TimeoutMs,
		Active:          r.Active,
		NotifyOnFailure: r.NotifyOnFailure,
	}
}

// ── Rutas ──────────────────────────────────────────────────────────────────

// Handler expone el servicio por HTTP.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register cuelga las rutas del grupo que se le pase (ya autenticado).
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/monitors", h.list)
	g.POST("/monitors", h.create)
	g.GET("/monitors/:id", h.detail)
	g.PUT("/monitors/:id", h.update)
	g.DELETE("/monitors/:id", h.remove)
	g.POST("/monitors/:id/check", h.checkNow)
}

func (h *Handler) list(c *gin.Context) {
	monitores, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	// Lista vacía, no null: así el frontend no tiene que defenderse.
	dtos := make([]MonitorDTO, 0, len(monitores))
	for _, m := range monitores {
		dtos = append(dtos, toMonitorDTO(m))
	}
	c.JSON(http.StatusOK, dtos)
}

func (h *Handler) detail(c *gin.Context) {
	detalle, err := h.svc.Detail(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if detalle == nil {
		httpx.NotFound(c, "El monitor no existe.")
		return
	}

	checks := make([]CheckDTO, 0, len(detalle.Checks))
	for _, ch := range detalle.Checks {
		checks = append(checks, toCheckDTO(ch))
	}

	c.JSON(http.StatusOK, DetailDTO{
		Monitor:   toMonitorDTO(detalle.Monitor),
		Checks:    checks,
		Uptime24h: detalle.Uptime24h,
	})
}

func (h *Handler) create(c *gin.Context) { h.guardar(c, "") }

func (h *Handler) update(c *gin.Context) { h.guardar(c, c.Param("id")) }

func (h *Handler) guardar(c *gin.Context, id string) {
	var req monitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "El formulario no se pudo leer.")
		return
	}

	resultado, err := h.svc.Save(c.Request.Context(), id, req.aDominio())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	// Un fallo de validación es culpa de quien llama: 400, no 500.
	if !resultado.OK {
		httpx.BadRequest(c, resultado.Error)
		return
	}

	estado := http.StatusOK
	if id == "" {
		estado = http.StatusCreated
	}
	c.JSON(estado, toMonitorDTO(resultado.Value))
}

func (h *Handler) remove(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) checkNow(c *gin.Context) {
	resultado, err := h.svc.CheckNow(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !resultado.OK {
		httpx.NotFound(c, resultado.Error)
		return
	}
	c.JSON(http.StatusOK, toMonitorDTO(resultado.Value))
}

// RunDue es el disparador del cron. Se registra fuera del grupo autenticado
// porque lo llama una máquina, no una persona: va protegido por CRON_SECRET.
func (h *Handler) RunDue(c *gin.Context) {
	resumen, err := h.svc.RunDueChecks(c.Request.Context(), time.Now())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, resumen)
}
