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

// Combinar prueba varios autenticadores en orden y se queda con el primero que
// reconozca el token (aquí: tokens personales y tokens de OAuth). Un error
// técnico de cualquiera corta la cadena: no se sigue como si el token no valiera.
func Combinar(autenticadores ...Autenticador) Autenticador {
	return combinado(autenticadores)
}

type combinado []Autenticador

func (c combinado) Autenticar(ctx context.Context, claro string) (*tokens.Identidad, error) {
	for _, a := range c {
		id, err := a.Autenticar(ctx, claro)
		if err != nil || id != nil {
			return id, err
		}
	}
	return nil, nil
}

// Handler monta el servidor en una ruta de Gin.
type Handler struct {
	servidor *Server
	auth     Autenticador
	// origenOK decide si una cabecera Origin es aceptable.
	origenOK func(origen, host string) bool
	// metadatos da la dirección de los metadatos OAuth del recurso, que se anuncia
	// en el 401 para que el cliente sepa dónde autorizarse. Nil: no hay OAuth.
	metadatos func(*gin.Context) string
}

func NewHandler(s *Server, auth Autenticador, origenesPermitidos []string, metadatos func(*gin.Context) string) *Handler {
	return &Handler{servidor: s, auth: auth, origenOK: origenAceptable(origenesPermitidos), metadatos: metadatos}
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
		h.sinAutorizar(c)
		return
	}
	id, err := h.auth.Autenticar(c.Request.Context(), claro)
	if err != nil {
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ocurrió un error inesperado."})
		return
	}
	if id == nil {
		h.sinAutorizar(c)
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

func (h *Handler) sinAutorizar(c *gin.Context) {
	cabecera := `Bearer realm="homelab"`
	if h.metadatos != nil {
		// Así un cliente OAuth (claude.ai) descubre dónde y cómo autorizarse (RFC 9728).
		cabecera += `, resource_metadata="` + h.metadatos(c) + `"`
	}
	c.Header("WWW-Authenticate", cabecera)
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Falta un token válido. Conéctalo por OAuth o crea uno en Espacio de trabajo → Claude."})
}
