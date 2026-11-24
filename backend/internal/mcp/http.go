package mcp

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

// Autenticador resuelve el token Bearer a una persona.
type Autenticador interface {
	Autenticar(ctx context.Context, claro string) (*tokens.Identidad, error)
}

// Handler monta el servidor en una ruta de Gin.
type Handler struct {
	servidor *Server
	auth     Autenticador
	// origenOK decide si una cabecera Origin es aceptable.
	origenOK func(origen, host string) bool
}

func NewHandler(s *Server, auth Autenticador, origenesPermitidos []string) *Handler {
	return &Handler{servidor: s, auth: auth, origenOK: origenAceptable(origenesPermitidos)}
}

// origenAceptable: los clientes MCP de verdad (Claude Code, un script) no mandan
// Origin. Si llega uno es un navegador, y entonces solo se acepta el propio
// sitio o uno de la lista: así una página ajena no puede usar una sesión local
// contra este endpoint (el ataque de «DNS rebinding» que la especificación pide
// evitar).
func origenAceptable(permitidos []string) func(origen, host string) bool {
	return func(origen, host string) bool {
		if origen == "" {
			return true
		}
		for _, p := range permitidos {
			if p == origen {
				return true
			}
		}
		u, err := url.Parse(origen)
		return err == nil && u.Host == host
	}
}

func (h *Handler) Register(r gin.IRoutes) {
	r.POST("/mcp", h.post)
	// Este servidor no abre flujos hacia el cliente ni mantiene sesiones.
	r.GET("/mcp", noPermitido)
	r.DELETE("/mcp", noPermitido)
}

func noPermitido(c *gin.Context) {
	c.Header("Allow", http.MethodPost)
	c.Status(http.StatusMethodNotAllowed)
}

func (h *Handler) post(c *gin.Context) {
	if !h.origenOK(c.GetHeader("Origin"), c.Request.Host) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Origen no permitido."})
		return
	}

	claro, ok := tokens.ExtraerBearer(c.GetHeader("Authorization"))
	if !ok {
		sinAutorizar(c)
		return
	}
	id, err := h.auth.Autenticar(c.Request.Context(), claro)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ocurrió un error inesperado."})
		return
	}
	if id == nil {
		sinAutorizar(c)
		return
	}

	cuerpo, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "La petición es demasiado grande."})
		return
	}

	quien := Caller{UserID: id.UsuarioID, Name: id.Nombre, Admin: id.Admin, CanWrite: id.Alcance.PuedeEscribir()}
	resp := h.servidor.Handle(c.Request.Context(), quien, cuerpo)
	if resp == nil {
		c.Status(http.StatusAccepted) // era una notificación
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json", resp)
}

func sinAutorizar(c *gin.Context) {
	// Sin resource_metadata: no hay OAuth, solo un token personal.
	c.Header("WWW-Authenticate", `Bearer realm="homelab"`)
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Falta un token válido. Créalo en Espacio de trabajo → Conexión con Claude."})
}
