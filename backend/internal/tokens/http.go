package tokens

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
)

const iso = "2006-01-02T15:04:05Z07:00"

type tokenDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Scope      string  `json:"scope"`
	Prefix     string  `json:"prefix"`
	CreatedAt  string  `json:"createdAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	RevokedAt  *string `json:"revokedAt"`
}

func aDTO(t Token) tokenDTO {
	d := tokenDTO{ID: t.ID, Name: t.Nombre, Scope: string(t.Alcance), Prefix: t.Prefijo, CreatedAt: t.CreadoEn.UTC().Format(iso)}
	if t.UsadoEn != nil {
		s := t.UsadoEn.UTC().Format(iso)
		d.LastUsedAt = &s
	}
	if t.RevocadoEn != nil {
		s := t.RevocadoEn.UTC().Format(iso)
		d.RevokedAt = &s
	}
	return d
}

// Handler gestiona los tokens de quien tiene la sesión abierta. Entrar con un
// token no sirve para crear otros: esto va con la cookie, no con Bearer.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("", h.listar)
	g.POST("", h.crear)
	g.DELETE("/:id", h.revocar)
}

func (h *Handler) listar(c *gin.Context) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}
	lista, err := h.svc.Listar(c.Request.Context(), u.UID)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	salida := make([]tokenDTO, 0, len(lista))
	for _, t := range lista {
		salida = append(salida, aDTO(t))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) crear(c *gin.Context) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}
	var cuerpo struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}
	res, err := h.svc.Crear(c.Request.Context(), u.UID, cuerpo.Name, cuerpo.Scope)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	// El secreto sale aquí y en ningún otro sitio; que nadie lo guarde en caché.
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"token": res.Value.Claro, "item": aDTO(res.Value.Token)})
}

func (h *Handler) revocar(c *gin.Context) {
	u, ok := httpx.CurrentUser(c)
	if !ok {
		httpx.Unauthorized(c)
		return
	}
	hecho, err := h.svc.Revocar(c.Request.Context(), u.UID, c.Param("id"))
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !hecho {
		httpx.NotFound(c, "No se encontró ese token.")
		return
	}
	c.Status(http.StatusNoContent)
}
