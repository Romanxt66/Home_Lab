package dbadmin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// conexionDTO es lo que ve el navegador. La URL de conexión NO sale nunca:
// lleva usuario y contraseña. En su lugar va solo el servidor, que es lo que
// hace falta para distinguir una conexión de otra en la lista.
type conexionDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	ReadOnly bool   `json:"readOnly"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/connections", h.conexiones)
	g.POST("/connections", h.guardar)
	g.PUT("/connections/:id", h.guardar)
	g.DELETE("/connections/:id", h.borrar)
	g.POST("/connections/:id/test", h.probar)

	g.GET("/connections/:id/schemas", h.esquemas)
	g.GET("/connections/:id/tables", h.tablas)
	g.GET("/connections/:id/table", h.detalleTabla)
	g.GET("/connections/:id/diagram", h.diagrama)

	g.GET("/connections/:id/rows", h.hojear)
	g.POST("/connections/:id/rows", h.insertar)
	g.PUT("/connections/:id/rows", h.actualizar)
	g.DELETE("/connections/:id/rows", h.borrarFila)

	g.POST("/connections/:id/query", h.consultar)
}

// ── Conexiones ─────────────────────────────────────────────────────────────

func (h *Handler) conexiones(c *gin.Context) {
	conexiones, err := h.svc.Conexiones(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]conexionDTO, 0, len(conexiones))
	for _, con := range conexiones {
		salida = append(salida, aConexionDTO(con))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) guardar(c *gin.Context) {
	var cuerpo struct {
		Name          string `json:"name"`
		ConnectionURL string `json:"connectionUrl"`
		ReadOnly      bool   `json:"readOnly"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Guardar(c.Request.Context(), Conexion{
		ID: c.Param("id"), Name: cuerpo.Name,
		ConnectionURL: cuerpo.ConnectionURL, ReadOnly: cuerpo.ReadOnly,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aConexionDTO(res.Value))
}

func (h *Handler) borrar(c *gin.Context) {
	if err := h.svc.Borrar(c.Request.Context(), c.Param("id")); err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) probar(c *gin.Context) {
	res, err := h.svc.Probar(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": res.Value})
}

// ── Estructura ─────────────────────────────────────────────────────────────

func (h *Handler) esquemas(c *gin.Context) {
	res, err := h.svc.Esquemas(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	salida := make([]gin.H, 0, len(res.Value))
	for _, e := range res.Value {
		salida = append(salida, gin.H{"name": e.Name, "isSystem": e.IsSystem})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) tablas(c *gin.Context) {
	res, err := h.svc.Tablas(c.Request.Context(), c.Param("id"), c.Query("schema"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	salida := make([]gin.H, 0, len(res.Value))
	for _, t := range res.Value {
		salida = append(salida, gin.H{
			"schema": t.Schema, "name": t.Name,
			"kind": t.Kind, "rowsApprox": t.RowsApprox,
		})
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) detalleTabla(c *gin.Context) {
	res, err := h.svc.DetalleTabla(c.Request.Context(), c.Param("id"),
		c.Query("schema"), c.Query("table"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"columns":      aColumnasDTO(res.Value.Columns),
		"indexes":      aIndicesDTO(res.Value.Indexes),
		"foreignKeys":  aClavesDTO(res.Value.ForeignKeys),
		"referencedBy": aClavesDTO(res.Value.ReferencedBy),
	})
}

func (h *Handler) diagrama(c *gin.Context) {
	res, err := h.svc.Diagrama(c.Request.Context(), c.Param("id"), c.Query("schema"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	tablas := make([]gin.H, 0, len(res.Value.Tables))
	for _, t := range res.Value.Tables {
		columnas := make([]gin.H, 0, len(t.Columns))
		for _, col := range t.Columns {
			columnas = append(columnas, gin.H{
				"name": col.Name, "dataType": col.DataType,
				"isPrimaryKey": col.IsPrimaryKey, "isForeignKey": col.IsForeignKey,
			})
		}
		tablas = append(tablas, gin.H{"schema": t.Schema, "name": t.Name, "columns": columnas})
	}

	c.JSON(http.StatusOK, gin.H{
		"tables":    tablas,
		"relations": aClavesDTO(res.Value.Relations),
	})
}

// ── Filas ──────────────────────────────────────────────────────────────────

func (h *Handler) hojear(c *gin.Context) {
	limite, _ := strconv.Atoi(c.Query("limit"))
	desde, _ := strconv.Atoi(c.Query("offset"))

	direccion := Ascendente
	if c.Query("direction") == "desc" {
		direccion = Descendente
	}

	res, err := h.svc.Hojear(c.Request.Context(), c.Param("id"), OpcionesHojear{
		Esquema: c.Query("schema"), Tabla: c.Query("table"),
		Limite: limite, Desde: desde,
		OrdenarPor: c.Query("orderBy"), Direccion: direccion,
	})
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"columns":    res.Value.Columns,
		"rows":       res.Value.Rows,
		"rowCount":   res.Value.RowCount,
		"durationMs": res.Value.DurationMs,
		"hasMore":    res.Value.HasMore,
		"limit":      res.Value.Limit,
		"offset":     res.Value.Offset,
	})
}

type cuerpoFila struct {
	Schema string         `json:"schema"`
	Table  string         `json:"table"`
	Values map[string]any `json:"values"`
	Key    map[string]any `json:"key"`
}

func (h *Handler) insertar(c *gin.Context) {
	var cuerpo cuerpoFila
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Insertar(c.Request.Context(), c.Param("id"),
		cuerpo.Schema, cuerpo.Table, cuerpo.Values)
	h.responderResultado(c, res, err)
}

func (h *Handler) actualizar(c *gin.Context) {
	var cuerpo cuerpoFila
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Actualizar(c.Request.Context(), c.Param("id"),
		cuerpo.Schema, cuerpo.Table, cuerpo.Values, cuerpo.Key)
	h.responderResultado(c, res, err)
}

func (h *Handler) borrarFila(c *gin.Context) {
	var cuerpo cuerpoFila
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.BorrarFila(c.Request.Context(), c.Param("id"),
		cuerpo.Schema, cuerpo.Table, cuerpo.Key)
	h.responderResultado(c, res, err)
}

// ── Consulta libre ─────────────────────────────────────────────────────────

func (h *Handler) consultar(c *gin.Context) {
	var cuerpo struct {
		SQL     string `json:"sql"`
		Confirm bool   `json:"confirmDestructive"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.Consultar(c.Request.Context(), c.Param("id"), cuerpo.SQL, cuerpo.Confirm)
	h.responderResultado(c, res, err)
}

// ── Auxiliares ─────────────────────────────────────────────────────────────

func (h *Handler) responderResultado(c *gin.Context, res kernel.Result[Resultado], err error) {
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"columns":    res.Value.Columns,
		"rows":       res.Value.Rows,
		"rowCount":   res.Value.RowCount,
		"durationMs": res.Value.DurationMs,
	})
}

func aConexionDTO(c Conexion) conexionDTO {
	return conexionDTO{ID: c.ID, Name: c.Name, Host: Host(c.ConnectionURL), ReadOnly: c.ReadOnly}
}

func aColumnasDTO(cs []Columna) []gin.H {
	salida := make([]gin.H, 0, len(cs))
	for _, c := range cs {
		fila := gin.H{
			"name": c.Name, "dataType": c.DataType,
			"isNullable": c.IsNullable, "isPrimaryKey": c.IsPrimaryKey,
			"default": c.Default, "ordinalPosition": c.OrdinalPosition,
			"references": nil,
		}
		if c.References != nil {
			fila["references"] = gin.H{
				"schema": c.References.Schema,
				"table":  c.References.Table,
				"column": c.References.Column,
			}
		}
		salida = append(salida, fila)
	}
	return salida
}

func aIndicesDTO(is []Indice) []gin.H {
	salida := make([]gin.H, 0, len(is))
	for _, i := range is {
		salida = append(salida, gin.H{
			"name": i.Name, "isUnique": i.IsUnique,
			"isPrimary": i.IsPrimary, "definition": i.Definition,
		})
	}
	return salida
}

func aClavesDTO(ks []ClaveAjena) []gin.H {
	salida := make([]gin.H, 0, len(ks))
	for _, k := range ks {
		salida = append(salida, gin.H{
			"constraintName": k.ConstraintName,
			"schema":         k.Schema, "table": k.Table, "columns": k.Columns,
			"refSchema": k.RefSchema, "refTable": k.RefTable, "refColumns": k.RefColumns,
		})
	}
	return salida
}
