package finance

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

// Los importes salen al navegador como número decimal, no como céntimos: es
// lo que espera la interfaz y lo que ya producía el Lab.

type cuentaDTO struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	Currency       string  `json:"currency"`
	InitialBalance float64 `json:"initialBalance"`
	Archived       bool    `json:"archived"`
	Balance        float64 `json:"balance,omitempty"`
}

type categoriaDTO struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Kind  string  `json:"kind"`
	Color *string `json:"color"`
}

type movimientoDTO struct {
	ID         string  `json:"id"`
	AccountID  string  `json:"accountId"`
	CategoryID *string `json:"categoryId"`
	Kind       string  `json:"kind"`
	Amount     float64 `json:"amount"`
	OccurredAt string  `json:"occurredAt"`
	Notes      *string `json:"notes"`
}

type presupuestoDTO struct {
	ID         string  `json:"id"`
	CategoryID string  `json:"categoryId"`
	Amount     float64 `json:"amount"`
	Spent      float64 `json:"spent,omitempty"`
	Remaining  float64 `json:"remaining,omitempty"`
	Ratio      float64 `json:"ratio,omitempty"`
	Status     string  `json:"status,omitempty"`
}

type recurrenteDTO struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	AccountID        string  `json:"accountId"`
	CategoryID       *string `json:"categoryId"`
	Kind             string  `json:"kind"`
	Amount           float64 `json:"amount"`
	Frequency        string  `json:"frequency"`
	NextRunAt        string  `json:"nextRunAt"`
	RemindDaysBefore *int    `json:"remindDaysBefore"`
	Active           bool    `json:"active"`
	Notes            *string `json:"notes"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/accounts", h.cuentas)
	g.POST("/accounts", h.guardarCuenta)
	g.PUT("/accounts/:id", h.guardarCuenta)
	g.DELETE("/accounts/:id", h.borrarCuenta)

	g.GET("/categories", h.categorias)
	g.POST("/categories", h.guardarCategoria)
	g.PUT("/categories/:id", h.guardarCategoria)
	g.DELETE("/categories/:id", h.borrarCategoria)

	g.GET("/transactions", h.movimientos)
	g.POST("/transactions", h.guardarMovimiento)
	g.PUT("/transactions/:id", h.guardarMovimiento)
	g.DELETE("/transactions/:id", h.borrarMovimiento)

	g.GET("/summary", h.resumen)

	g.GET("/budgets", h.presupuestos)
	g.PUT("/budgets", h.guardarPresupuesto)
	g.DELETE("/budgets/:categoryId", h.borrarPresupuesto)

	g.GET("/recurring", h.recurrentes)
	g.POST("/recurring", h.guardarRecurrente)
	g.PUT("/recurring/:id", h.guardarRecurrente)
	g.DELETE("/recurring/:id", h.borrarRecurrente)
}

// ── Cuentas ────────────────────────────────────────────────────────────────

func (h *Handler) cuentas(c *gin.Context) {
	archivadas := c.Query("archived") == "true"

	conSaldo, err := h.svc.CuentasConSaldo(c.Request.Context(), archivadas)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]cuentaDTO, 0, len(conSaldo))
	for _, f := range conSaldo {
		dto := aCuentaDTO(f.Cuenta)
		dto.Balance = f.Balance.Float()
		salida = append(salida, dto)
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardarCuenta(c *gin.Context) {
	var cuerpo struct {
		Name           string  `json:"name"`
		Kind           string  `json:"kind"`
		Currency       string  `json:"currency"`
		InitialBalance float64 `json:"initialBalance"`
		Archived       bool    `json:"archived"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.GuardarCuenta(c.Request.Context(), Cuenta{
		ID: c.Param("id"), Name: cuerpo.Name, Kind: TipoCuenta(cuerpo.Kind),
		Currency: cuerpo.Currency, InitialBalance: DesdeFloat(cuerpo.InitialBalance),
		Archived: cuerpo.Archived,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aCuentaDTO(res.Value))
}

func (h *Handler) borrarCuenta(c *gin.Context) {
	if err := h.svc.BorrarCuenta(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Categorías ─────────────────────────────────────────────────────────────

func (h *Handler) categorias(c *gin.Context) {
	cats, err := h.svc.Categorias(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]categoriaDTO, 0, len(cats))
	for _, cat := range cats {
		salida = append(salida, categoriaDTO{
			ID: cat.ID, Name: cat.Name, Kind: string(cat.Kind), Color: cat.Color,
		})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardarCategoria(c *gin.Context) {
	var cuerpo struct {
		Name  string  `json:"name"`
		Kind  string  `json:"kind"`
		Color *string `json:"color"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.GuardarCategoria(c.Request.Context(), Categoria{
		ID: c.Param("id"), Name: cuerpo.Name,
		Kind: TipoMovimiento(cuerpo.Kind), Color: cuerpo.Color,
	})
	if err != nil {
		// Un nombre repetido es culpa de quien lo escribe, no del servidor.
		httpx.BadRequest(c, err.Error())
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, categoriaDTO{
		ID: res.Value.ID, Name: res.Value.Name,
		Kind: string(res.Value.Kind), Color: res.Value.Color,
	})
}

func (h *Handler) borrarCategoria(c *gin.Context) {
	if err := h.svc.BorrarCategoria(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Movimientos ────────────────────────────────────────────────────────────

func (h *Handler) movimientos(c *gin.Context) {
	filtro := FiltroMovimientos{AccountID: c.Query("accountId")}

	if d := c.Query("from"); d != "" {
		if t, err := time.Parse(iso, d); err == nil {
			filtro.Desde = &t
		}
	}
	if d := c.Query("to"); d != "" {
		if t, err := time.Parse(iso, d); err == nil {
			filtro.Hasta = &t
		}
	}
	filtro.Limite, _ = strconv.Atoi(c.Query("limit"))

	movs, err := h.svc.Movimientos(c.Request.Context(), filtro)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]movimientoDTO, 0, len(movs))
	for _, m := range movs {
		salida = append(salida, aMovimientoDTO(m))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardarMovimiento(c *gin.Context) {
	var cuerpo struct {
		AccountID  string  `json:"accountId"`
		CategoryID *string `json:"categoryId"`
		Kind       string  `json:"kind"`
		Amount     float64 `json:"amount"`
		OccurredAt string  `json:"occurredAt"`
		Notes      *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	cuando, err := time.Parse(iso, cuerpo.OccurredAt)
	if err != nil {
		httpx.BadRequest(c, "La fecha no tiene un formato válido.")
		return
	}

	res, errSvc := h.svc.GuardarMovimiento(c.Request.Context(), Movimiento{
		ID: c.Param("id"), AccountID: cuerpo.AccountID, CategoryID: cuerpo.CategoryID,
		Kind: TipoMovimiento(cuerpo.Kind), Amount: DesdeFloat(cuerpo.Amount),
		OccurredAt: cuando, Notes: cuerpo.Notes,
	})
	if errSvc != nil {
		httpx.ServerError(c, errSvc)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aMovimientoDTO(res.Value))
}

func (h *Handler) borrarMovimiento(c *gin.Context) {
	if err := h.svc.BorrarMovimiento(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Resumen ────────────────────────────────────────────────────────────────

func (h *Handler) resumen(c *gin.Context) {
	año, mes, ok := añoYMes(c)
	if !ok {
		return
	}

	resumen, err := h.svc.ResumenMensual(c.Request.Context(), año, mes)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	porCategoria := make([]gin.H, 0, len(resumen.ByCategory))
	for _, fila := range resumen.ByCategory {
		porCategoria = append(porCategoria, gin.H{
			"categoryId": fila.CategoryID,
			"amount":     fila.Amount.Float(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"income":     resumen.Income.Float(),
		"expense":    resumen.Expense.Float(),
		"net":        resumen.Net.Float(),
		"byCategory": porCategoria,
	})
}

// ── Presupuestos ───────────────────────────────────────────────────────────

func (h *Handler) presupuestos(c *gin.Context) {
	año, mes, ok := añoYMes(c)
	if !ok {
		return
	}

	conAvance, err := h.svc.PresupuestosConAvance(c.Request.Context(), año, mes)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]presupuestoDTO, 0, len(conAvance))
	for _, p := range conAvance {
		salida = append(salida, presupuestoDTO{
			ID: p.Presupuesto.ID, CategoryID: p.Presupuesto.CategoryID,
			Amount:    p.Presupuesto.Amount.Float(),
			Spent:     p.Avance.Spent.Float(),
			Remaining: p.Avance.Remaining.Float(),
			Ratio:     p.Avance.Ratio,
			Status:    string(p.Avance.Status),
		})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardarPresupuesto(c *gin.Context) {
	var cuerpo struct {
		CategoryID string  `json:"categoryId"`
		Amount     float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.GuardarPresupuesto(c.Request.Context(), Presupuesto{
		CategoryID: cuerpo.CategoryID, Amount: DesdeFloat(cuerpo.Amount),
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, presupuestoDTO{
		ID: res.Value.ID, CategoryID: res.Value.CategoryID,
		Amount: res.Value.Amount.Float(),
	})
}

func (h *Handler) borrarPresupuesto(c *gin.Context) {
	if err := h.svc.BorrarPresupuesto(c.Request.Context(), c.Param("categoryId")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Recurrentes ────────────────────────────────────────────────────────────

func (h *Handler) recurrentes(c *gin.Context) {
	recs, err := h.svc.Recurrentes(c.Request.Context(), c.Query("active") == "true")
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]recurrenteDTO, 0, len(recs))
	for _, r := range recs {
		salida = append(salida, aRecurrenteDTO(r))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardarRecurrente(c *gin.Context) {
	var cuerpo struct {
		Name             string  `json:"name"`
		AccountID        string  `json:"accountId"`
		CategoryID       *string `json:"categoryId"`
		Kind             string  `json:"kind"`
		Amount           float64 `json:"amount"`
		Frequency        string  `json:"frequency"`
		NextRunAt        string  `json:"nextRunAt"`
		RemindDaysBefore *int    `json:"remindDaysBefore"`
		Active           bool    `json:"active"`
		Notes            *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	cuando, err := time.Parse(iso, cuerpo.NextRunAt)
	if err != nil {
		httpx.BadRequest(c, "La fecha del próximo cargo no tiene un formato válido.")
		return
	}

	res, errSvc := h.svc.GuardarRecurrente(c.Request.Context(), Recurrente{
		ID: c.Param("id"), Name: cuerpo.Name, AccountID: cuerpo.AccountID,
		CategoryID: cuerpo.CategoryID, Kind: TipoMovimiento(cuerpo.Kind),
		Amount: DesdeFloat(cuerpo.Amount), Frequency: Frecuencia(cuerpo.Frequency),
		NextRunAt: cuando, RemindDaysBefore: cuerpo.RemindDaysBefore,
		Active: cuerpo.Active, Notes: cuerpo.Notes,
	})
	if errSvc != nil {
		httpx.ServerError(c, errSvc)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aRecurrenteDTO(res.Value))
}

func (h *Handler) borrarRecurrente(c *gin.Context) {
	if err := h.svc.BorrarRecurrente(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── Auxiliares ─────────────────────────────────────────────────────────────

// añoYMes lee el periodo de la consulta, con el mes actual por defecto.
func añoYMes(c *gin.Context) (int, int, bool) {
	ahora := time.Now()

	año, err := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(ahora.Year())))
	if err != nil {
		httpx.BadRequest(c, "El año no es válido.")
		return 0, 0, false
	}

	mes, err := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(ahora.Month()))))
	if err != nil || mes < 1 || mes > 12 {
		httpx.BadRequest(c, "El mes debe estar entre 1 y 12.")
		return 0, 0, false
	}

	return año, mes, true
}

func aCuentaDTO(c Cuenta) cuentaDTO {
	return cuentaDTO{
		ID: c.ID, Name: c.Name, Kind: string(c.Kind), Currency: c.Currency,
		InitialBalance: c.InitialBalance.Float(), Archived: c.Archived,
	}
}

func aMovimientoDTO(m Movimiento) movimientoDTO {
	return movimientoDTO{
		ID: m.ID, AccountID: m.AccountID, CategoryID: m.CategoryID,
		Kind: string(m.Kind), Amount: m.Amount.Float(),
		OccurredAt: m.OccurredAt.Format(iso), Notes: m.Notes,
	}
}

func aRecurrenteDTO(r Recurrente) recurrenteDTO {
	return recurrenteDTO{
		ID: r.ID, Name: r.Name, AccountID: r.AccountID, CategoryID: r.CategoryID,
		Kind: string(r.Kind), Amount: r.Amount.Float(),
		Frequency: string(r.Frequency), NextRunAt: r.NextRunAt.Format(iso),
		RemindDaysBefore: r.RemindDaysBefore, Active: r.Active, Notes: r.Notes,
	}
}
