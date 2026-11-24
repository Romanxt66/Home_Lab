package workspace

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/mcp"
	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

// SyncHandler deja a un programa (el plugin de Claude Code) bajar las skills de
// una persona con su token, en lugar de la cookie de sesión. Es solo lectura y
// solo de lo suyo: ni lo compartido por otras personas ni nada que escriba.
// El MCP no basta para esto porque no puede llevar archivos binarios (fuentes,
// imágenes), y una skill sin ellos queda rota.
type SyncHandler struct {
	svc  *Service
	auth mcp.Autenticador
}

func NewSyncHandler(svc *Service, auth mcp.Autenticador) *SyncHandler {
	return &SyncHandler{svc: svc, auth: auth}
}

func (h *SyncHandler) Register(g gin.IRoutes) {
	g.GET("/manifest", h.manifiesto)
	g.GET("/skills/:id", h.descargar)
}

// quien resuelve el token Bearer; si no vale, responde 401 y devuelve false.
func (h *SyncHandler) quien(c *gin.Context) (Quien, bool) {
	claro, ok := tokens.ExtraerBearer(c.GetHeader("Authorization"))
	if !ok {
		c.Header("WWW-Authenticate", `Bearer realm="homelab"`)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Falta un token. Créalo en Espacio de trabajo → Claude."})
		return Quien{}, false
	}
	id, err := h.auth.Autenticar(c.Request.Context(), claro)
	if err != nil {
		httpx.ServerError(c, err)
		return Quien{}, false
	}
	if id == nil {
		c.Header("WWW-Authenticate", `Bearer realm="homelab"`)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "El token no es válido o fue revocado."})
		return Quien{}, false
	}
	return Quien{ID: id.UsuarioID, Nombre: id.Nombre, Admin: id.Admin}, true
}

type itemSync struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   int    `json:"version"`
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	UpdatedAt string `json:"updatedAt"`
}

// manifiesto lista las skills propias con su versión: el cliente compara y solo
// baja las que cambiaron.
func (h *SyncHandler) manifiesto(c *gin.Context) {
	q, ok := h.quien(c)
	if !ok {
		return
	}
	lista, err := h.svc.Listar(c.Request.Context(), q, Skill)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	salida := []itemSync{}
	for _, e := range lista {
		if e.DuenoID != q.ID {
			continue
		}
		salida = append(salida, itemSync{e.ID, e.Nombre, e.Version, e.Archivos, e.Bytes, e.ActualizadoEn.UTC().Format(iso)})
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"skills": salida})
}

func (h *SyncHandler) descargar(c *gin.Context) {
	q, ok := h.quien(c)
	if !ok {
		return
	}
	// Buscar con compartido=false: solo se baja lo propio, aunque se conozca el id de otra.
	e, err := h.svc.Buscar(c.Request.Context(), q, Skill, c.Param("id"), false)
	if errors.Is(err, ErrNoEncontrado) {
		httpx.NotFound(c, "No se encontró esa skill entre las tuyas.")
		return
	}
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	d, err := h.svc.Descargar(c.Request.Context(), q, e.ID, 0)
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	c.Header("X-Skill-Version", itoa(e.Version))
	enviar(c, d)
}

func itoa(n int) string { return strconv.Itoa(n) }
