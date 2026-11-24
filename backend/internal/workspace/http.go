package workspace

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

type elementoDTO struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	Version     int    `json:"version"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	OwnerName   string `json:"ownerName"`
	Mine        bool   `json:"mine"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type archivoDTO struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
	Text  bool   `json:"text"`
}

type versionDTO struct {
	Number      int    `json:"number"`
	Message     string `json:"message"`
	AuthorName  string `json:"authorName"`
	Description string `json:"description"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	CreatedAt   string `json:"createdAt"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("/items", h.listar)
	g.GET("/items/:id", h.detalle)
	g.GET("/items/:id/file", h.archivo)
	g.GET("/items/:id/download", h.descargar)
	g.PUT("/items/:id/visibility", h.visibilidad)
	g.DELETE("/items/:id", h.borrar)

	g.POST("/skills", h.subirSkill)
	g.POST("/docs", h.guardarDoc)

	g.GET("/download", h.descargarTodo)
}

func quien(c *gin.Context) (Quien, bool) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return Quien{}, false
	}
	nombre := u.Email
	if u.Name != nil && *u.Name != "" {
		nombre = *u.Name
	}
	return Quien{ID: u.UID, Nombre: nombre, Admin: u.Role == "superadmin"}, true
}

func aDTO(e Elemento, uid string) elementoDTO {
	return elementoDTO{
		ID: e.ID, Kind: string(e.Tipo), Name: e.Nombre, Description: e.Descripcion,
		Visibility: string(e.Visibilidad), Version: e.Version, Files: e.Archivos, Bytes: e.Bytes,
		OwnerName: e.DuenoNombre, Mine: e.DuenoID == uid,
		CreatedAt: e.CreadoEn.UTC().Format(iso), UpdatedAt: e.ActualizadoEn.UTC().Format(iso),
	}
}

// responderError traduce los errores del servicio a respuestas HTTP.
func responderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNoEncontrado):
		httpx.NotFound(c, "No se encontró.")
	case errors.Is(err, ErrSinPermiso):
		c.JSON(http.StatusForbidden, gin.H{"error": "Solo quien lo creó puede hacer eso."})
	default:
		httpx.ServerError(c, err)
	}
}

func (h *Handler) listar(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	tipo := Tipo(c.DefaultQuery("kind", string(Skill)))
	if tipo != Skill && tipo != Doc {
		httpx.BadRequest(c, "El tipo debe ser skill o doc.")
		return
	}
	lista, err := h.svc.Listar(c.Request.Context(), q, tipo)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	salida := make([]elementoDTO, 0, len(lista))
	for _, e := range lista {
		salida = append(salida, aDTO(e, q.ID))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) detalle(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	d, err := h.svc.Detalle(c.Request.Context(), q, c.Param("id"))
	if err != nil {
		responderError(c, err)
		return
	}
	archivos := make([]archivoDTO, 0, len(d.Archivos))
	for _, a := range d.Archivos {
		archivos = append(archivos, archivoDTO{Path: a.Ruta, Bytes: a.Bytes, Text: a.Texto})
	}
	versiones := make([]versionDTO, 0, len(d.Versiones))
	for _, v := range d.Versiones {
		versiones = append(versiones, versionDTO{
			Number: v.Numero, Message: v.Mensaje, AuthorName: v.AutorNombre, Description: v.Descripcion,
			Files: v.Archivos, Bytes: v.Bytes, CreatedAt: v.CreadaEn.UTC().Format(iso),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"item": aDTO(d.Elemento, q.ID), "files": archivos, "versions": versiones,
	})
}

// archivo devuelve el texto de un archivo, para previsualizarlo. Lo binario no
// se enseña: se descarga con el zip.
func (h *Handler) archivo(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	version, _ := strconv.Atoi(c.DefaultQuery("version", "0"))
	a, err := h.svc.Archivo(c.Request.Context(), q, c.Param("id"), version, c.Query("path"))
	if err != nil {
		responderError(c, err)
		return
	}
	if !EsTexto(a.Contenido) {
		httpx.BadRequest(c, "Este archivo no es de texto: descarga la skill para verlo.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": a.Ruta, "content": string(a.Contenido)})
}

func (h *Handler) descargar(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	version, _ := strconv.Atoi(c.DefaultQuery("version", "0"))
	d, err := h.svc.Descargar(c.Request.Context(), q, c.Param("id"), version)
	if err != nil {
		responderError(c, err)
		return
	}
	enviar(c, d)
}

func (h *Handler) descargarTodo(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	alcance := Alcance(c.DefaultQuery("scope", string(LoMio)))
	if alcance != LoMio && alcance != LoCompartido && alcance != Todo {
		httpx.BadRequest(c, "El alcance debe ser mine, shared o all.")
		return
	}
	d, err := h.svc.DescargarTodo(c.Request.Context(), q, alcance)
	if err != nil {
		if errors.Is(err, ErrNoEncontrado) {
			httpx.BadRequest(c, "Todavía no hay nada que descargar.")
			return
		}
		httpx.ServerError(c, err)
		return
	}
	enviar(c, d)
}

func enviar(c *gin.Context, d *Descarga) {
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(d.Nombre))
	// El navegador no debe interpretar lo descargado como una página.
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, d.Tipo, d.Contenido)
}

func (h *Handler) subirSkill(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	// Se corta la lectura en el límite: un cuerpo enorme no llega a la memoria.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBytesZipSubido+(1<<20))
	f, _, err := c.Request.FormFile("file")
	if err != nil {
		httpx.BadRequest(c, "Elige un archivo .skill o .zip (de hasta 12 MB).")
		return
	}
	defer f.Close()
	datos, err := io.ReadAll(io.LimitReader(f, MaxBytesZipSubido+1))
	if err != nil {
		httpx.BadRequest(c, "No se pudo leer el archivo.")
		return
	}

	res, err := h.svc.SubirSkill(c.Request.Context(), q, datos, c.PostForm("message"), Visibilidad(c.PostForm("visibility")))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aDTO(res.Value, q.ID))
}

func (h *Handler) guardarDoc(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	var cuerpo struct {
		Path       string `json:"path"`
		Content    string `json:"content"`
		Message    string `json:"message"`
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}
	res, err := h.svc.GuardarDoc(c.Request.Context(), q, cuerpo.Path, cuerpo.Content, cuerpo.Message, Visibilidad(cuerpo.Visibility))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aDTO(res.Value, q.ID))
}

func (h *Handler) visibilidad(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	var cuerpo struct {
		Visibility string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil || (cuerpo.Visibility != "private" && cuerpo.Visibility != "shared") {
		httpx.BadRequest(c, "La visibilidad debe ser private o shared.")
		return
	}
	if err := h.svc.CambiarVisibilidad(c.Request.Context(), q, c.Param("id"), Visibilidad(cuerpo.Visibility)); err != nil {
		responderError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) borrar(c *gin.Context) {
	q, ok := quien(c)
	if !ok {
		return
	}
	if err := h.svc.Borrar(c.Request.Context(), q, c.Param("id")); err != nil {
		responderError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
