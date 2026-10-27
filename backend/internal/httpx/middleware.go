// Package httpx reúne lo transversal de la capa HTTP: CORS, autenticación y
// la forma de responder los errores. Ningún módulo de negocio importa Gin
// fuera de su archivo http.go.
package httpx

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/auth"
)

const claveUsuario = "usuario"

// CORS permite al frontend, que corre en otro puerto, llamar a esta API.
// Solo se aceptan los orígenes de la lista: con "*" el navegador nunca
// enviaría la cookie de sesión, así que una lista abierta aquí no vale.
func CORS(origenes []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origen := c.GetHeader("Origin")

		if origen != "" && slices.Contains(origenes, origen) {
			c.Header("Access-Control-Allow-Origin", origen)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RequireAuth corta la petición si no hay una sesión válida en la cookie.
func RequireAuth(sessions *auth.Sessions) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.SessionCookie)
		if err != nil {
			Unauthorized(c)
			return
		}

		payload, err := sessions.Verify(token)
		if err != nil {
			Unauthorized(c)
			return
		}

		c.Set(claveUsuario, payload)
		c.Next()
	}
}

// RequireRole exige además un rol concreto (por ejemplo "superadmin").
func RequireRole(sessions *auth.Sessions, rol string) gin.HandlerFunc {
	return func(c *gin.Context) {
		usuario, ok := CurrentUser(c)
		if !ok || usuario.Role != rol {
			c.AbortWithStatusJSON(http.StatusForbidden,
				gin.H{"error": "No tienes permiso para esta acción."})
			return
		}
		c.Next()
	}
}

// CurrentUser devuelve la sesión que dejó RequireAuth.
func CurrentUser(c *gin.Context) (*auth.SessionPayload, bool) {
	valor, existe := c.Get(claveUsuario)
	if !existe {
		return nil, false
	}
	payload, ok := valor.(*auth.SessionPayload)
	return payload, ok
}

// ── Respuestas ─────────────────────────────────────────────────────────────

// Unauthorized responde 401 y detiene la cadena.
func Unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized,
		gin.H{"error": "Inicia sesión para continuar."})
}

// BadRequest se usa para los errores que puede corregir quien llama.
func BadRequest(c *gin.Context, mensaje string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": mensaje})
}

// NotFound para un recurso que no existe.
func NotFound(c *gin.Context, mensaje string) {
	c.JSON(http.StatusNotFound, gin.H{"error": mensaje})
}

// ServerError registra el detalle y devuelve un mensaje genérico: el error
// interno se queda en el log, nunca se expone al navegador.
func ServerError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.JSON(http.StatusInternalServerError,
		gin.H{"error": "Ocurrió un error inesperado."})
}
