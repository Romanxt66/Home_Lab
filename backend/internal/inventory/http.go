package inventory

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

// ── DTOs ───────────────────────────────────────────────────────────────────

type LocationDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	ParentID *string `json:"parentId"`
	Notes    *string `json:"notes"`
}

type LocationNodeDTO struct {
	LocationDTO
	Children  []LocationNodeDTO `json:"children"`
	ItemCount int               `json:"itemCount"`
}

type ItemDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Category   *string `json:"category"`
	Quantity   int     `json:"quantity"`
	Unit       *string `json:"unit"`
	LocationID *string `json:"locationId"`
	Notes      *string `json:"notes"`
}

type MovementDTO struct {
	ID           string  `json:"id"`
	ItemID       string  `json:"itemId"`
	Kind         string  `json:"kind"`
	Delta        int     `json:"delta"`
	ToLocationID *string `json:"toLocationId"`
	Notes        *string `json:"notes"`
	OccurredAt   string  `json:"occurredAt"`
}

func toLocationDTO(l Location) LocationDTO {
	return LocationDTO{
		ID: l.ID, Name: l.Name, Kind: string(l.Kind),
		ParentID: l.ParentID, Notes: l.Notes,
	}
}

func toNodeDTO(n *LocationNode) LocationNodeDTO {
	hijas := make([]LocationNodeDTO, 0, len(n.Children))
	for _, h := range n.Children {
		hijas = append(hijas, toNodeDTO(h))
	}
	return LocationNodeDTO{
		LocationDTO: toLocationDTO(n.Location),
		Children:    hijas,
		ItemCount:   n.ItemCount,
	}
}

func toItemDTO(i Item) ItemDTO {
	return ItemDTO{
		ID: i.ID, Name: i.Name, Category: i.Category, Quantity: i.Quantity,
		Unit: i.Unit, LocationID: i.LocationID, Notes: i.Notes,
	}
}

func toMovementDTO(m Movement) MovementDTO {
	return MovementDTO{
		ID: m.ID, ItemID: m.ItemID, Kind: string(m.Kind), Delta: m.Delta,
		ToLocationID: m.ToLocationID, Notes: m.Notes,
		OccurredAt: m.OccurredAt.UTC().Format(time.RFC3339),
	}
}

// ── Peticiones ─────────────────────────────────────────────────────────────

type locationRequest struct {
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	ParentID *string `json:"parentId"`
	Notes    *string `json:"notes"`
}

type itemRequest struct {
	Name       string  `json:"name"`
	Category   *string `json:"category"`
	Quantity   int     `json:"quantity"`
	Unit       *string `json:"unit"`
	LocationID *string `json:"locationId"`
	Notes      *string `json:"notes"`
}

type movementRequest struct {
	ItemID       string  `json:"itemId"`
	Kind         string  `json:"kind"`
	Quantity     int     `json:"quantity"`
	ToLocationID *string `json:"toLocationId"`
	Notes        *string `json:"notes"`
	OccurredAt   *string `json:"occurredAt"`
}

// ── Rutas ──────────────────────────────────────────────────────────────────

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/locations", h.listLocations)
	g.GET("/locations/tree", h.locationTree)
	g.POST("/locations", h.createLocation)
	g.PUT("/locations/:id", h.updateLocation)
	g.DELETE("/locations/:id", h.removeLocation)

	g.GET("/items", h.listItems)
	g.GET("/items/categories", h.categories)
	g.GET("/items/:id", h.getItem)
	g.POST("/items", h.createItem)
	g.PUT("/items/:id", h.updateItem)
	g.DELETE("/items/:id", h.removeItem)
	g.GET("/items/:id/movements", h.itemMovements)

	g.GET("/movements", h.recentMovements)
	g.POST("/movements", h.recordMovement)
}

func (h *Handler) listLocations(c *gin.Context) {
	lista, err := h.svc.ListLocations(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	dtos := make([]LocationDTO, 0, len(lista))
	for _, l := range lista {
		dtos = append(dtos, toLocationDTO(l))
	}
	c.JSON(http.StatusOK, dtos)
}

func (h *Handler) locationTree(c *gin.Context) {
	arbol, err := h.svc.LocationTree(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	dtos := make([]LocationNodeDTO, 0, len(arbol))
	for _, n := range arbol {
		dtos = append(dtos, toNodeDTO(n))
	}
	c.JSON(http.StatusOK, dtos)
}

func (h *Handler) createLocation(c *gin.Context) { h.guardarUbicacion(c, "") }

func (h *Handler) updateLocation(c *gin.Context) { h.guardarUbicacion(c, c.Param("id")) }

func (h *Handler) guardarUbicacion(c *gin.Context, id string) {
	var req locationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "El formulario no se pudo leer.")
		return
	}

	res, err := h.svc.SaveLocation(c.Request.Context(), id, LocationInput{
		Name: req.Name, Kind: LocationKind(req.Kind),
		ParentID: req.ParentID, Notes: req.Notes,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	estado := http.StatusOK
	if id == "" {
		estado = http.StatusCreated
	}
	c.JSON(estado, toLocationDTO(res.Value))
}

func (h *Handler) removeLocation(c *gin.Context) {
	if err := h.svc.RemoveLocation(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) listItems(c *gin.Context) {
	lista, err := h.svc.ListItems(c.Request.Context(), c.Query("locationId"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	dtos := make([]ItemDTO, 0, len(lista))
	for _, i := range lista {
		dtos = append(dtos, toItemDTO(i))
	}
	c.JSON(http.StatusOK, dtos)
}

func (h *Handler) getItem(c *gin.Context) {
	item, err := h.svc.GetItem(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if item == nil {
		httpx.NotFound(c, "El artículo no existe.")
		return
	}
	c.JSON(http.StatusOK, toItemDTO(*item))
}

func (h *Handler) categories(c *gin.Context) {
	cats, err := h.svc.Categories(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, cats)
}

func (h *Handler) createItem(c *gin.Context) { h.guardarArticulo(c, "") }

func (h *Handler) updateItem(c *gin.Context) { h.guardarArticulo(c, c.Param("id")) }

func (h *Handler) guardarArticulo(c *gin.Context, id string) {
	var req itemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "El formulario no se pudo leer.")
		return
	}

	res, err := h.svc.SaveItem(c.Request.Context(), id, ItemInput{
		Name: req.Name, Category: req.Category, Quantity: req.Quantity,
		Unit: req.Unit, LocationID: req.LocationID, Notes: req.Notes,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	estado := http.StatusOK
	if id == "" {
		estado = http.StatusCreated
	}
	c.JSON(estado, toItemDTO(res.Value))
}

func (h *Handler) removeItem(c *gin.Context) {
	if err := h.svc.RemoveItem(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) itemMovements(c *gin.Context) {
	lista, err := h.svc.ListMovements(c.Request.Context(), c.Param("id"), 100)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	h.responderMovimientos(c, lista)
}

func (h *Handler) recentMovements(c *gin.Context) {
	lista, err := h.svc.RecentMovements(c.Request.Context(), 50)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	h.responderMovimientos(c, lista)
}

func (h *Handler) responderMovimientos(c *gin.Context, lista []Movement) {
	dtos := make([]MovementDTO, 0, len(lista))
	for _, m := range lista {
		dtos = append(dtos, toMovementDTO(m))
	}
	c.JSON(http.StatusOK, dtos)
}

func (h *Handler) recordMovement(c *gin.Context) {
	var req movementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "El formulario no se pudo leer.")
		return
	}

	// Sin fecha, se asume ahora: lo normal es registrar lo que acaba de pasar.
	cuando := time.Now().UTC()
	if req.OccurredAt != nil && *req.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.OccurredAt)
		if err != nil {
			httpx.BadRequest(c, "La fecha del movimiento no es válida.")
			return
		}
		cuando = parsed.UTC()
	}

	res, err := h.svc.RecordMovement(c.Request.Context(), MovementInput{
		ItemID: req.ItemID, Kind: MovementKind(req.Kind), Quantity: req.Quantity,
		ToLocationID: req.ToLocationID, Notes: req.Notes, OccurredAt: cuando,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusCreated, toMovementDTO(res.Value))
}
