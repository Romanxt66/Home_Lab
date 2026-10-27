package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Usuario tal como está en la tabla usuarioslab, compartida con el Lab.
type Usuario struct {
	ID           string  `db:"id"`
	Email        string  `db:"email"`
	Name         *string `db:"name"`
	PasswordHash *string `db:"passwordHash"`
	Role         string  `db:"role"`
	Status       string  `db:"status"`
}

// Repo busca usuarios para el inicio de sesión.
type Repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) PorEmail(ctx context.Context, email string) (*Usuario, error) {
	var u Usuario
	err := r.db.GetContext(ctx, &u, `
		SELECT id, email, name, "passwordHash", role, status
		FROM usuarioslab WHERE email = $1`, strings.ToLower(strings.TrimSpace(email)))

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Registrar crea una cuenta nueva a partir de una identidad de Google.
// Nace siempre pendiente y con rol de usuario: quién entra lo decide un
// superadmin, nunca quien se registra.
func (r *Repo) Registrar(ctx context.Context, email string, nombre, foto *string) (*Usuario, error) {
	var u Usuario
	err := r.db.GetContext(ctx, &u, `
		INSERT INTO usuarioslab (id, email, name, picture, "passwordHash", role, status, "createdAt")
		VALUES ($1, $2, $3, $4, NULL, 'user', 'pending', NOW())
		RETURNING id, email, name, "passwordHash", role, status`,
		uuid.NewString(), strings.ToLower(strings.TrimSpace(email)), nombre, foto)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Handler expone iniciar sesión, cerrarla y consultar quién soy.
type Handler struct {
	repo       *Repo
	sessions   *Sessions
	produccion bool
}

func NewHandler(repo *Repo, sessions *Sessions, produccion bool) *Handler {
	return &Handler{repo: repo, sessions: sessions, produccion: produccion}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	g.POST("/login", h.login)
	g.POST("/logout", h.logout)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type usuarioDTO struct {
	ID    string  `json:"id"`
	Email string  `json:"email"`
	Name  *string `json:"name"`
	Role  string  `json:"role"`
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "El formulario no se pudo leer."})
		return
	}

	// El mismo mensaje para "no existe" y "contraseña incorrecta": decir cuál
	// de las dos falló permite averiguar qué correos están registrados.
	const generico = "Correo o contraseña incorrectos."

	usuario, err := h.repo.PorEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ocurrió un error inesperado."})
		return
	}
	if usuario == nil || usuario.PasswordHash == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": generico})
		return
	}
	if !VerifyPassword(req.Password, *usuario.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": generico})
		return
	}

	// Una cuenta a la espera de aprobación sí merece explicación: el usuario
	// no puede hacer nada por su cuenta y repetir la contraseña no ayuda.
	if usuario.Status != "approved" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Tu cuenta todavía no está aprobada por un administrador.",
		})
		return
	}

	token, err := h.sessions.Create(usuario.ID, usuario.Email, usuario.Name, usuario.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear la sesión."})
		return
	}

	h.ponerCookie(c, token, SessionMaxAge)
	c.JSON(http.StatusOK, usuarioDTO{
		ID: usuario.ID, Email: usuario.Email, Name: usuario.Name, Role: usuario.Role,
	})
}

func (h *Handler) logout(c *gin.Context) {
	h.ponerCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}

// Me devuelve la sesión actual. Va detrás del middleware de autenticación.
func (h *Handler) Me(c *gin.Context) {
	valor, existe := c.Get("usuario")
	payload, ok := valor.(*SessionPayload)
	if !existe || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Inicia sesión para continuar."})
		return
	}

	c.JSON(http.StatusOK, usuarioDTO{
		ID: payload.UID, Email: payload.Email, Name: payload.Name, Role: payload.Role,
	})
}

// ponerCookie fija la cookie de sesión con los mismos atributos que el Lab.
// HttpOnly deja la cookie fuera del alcance de JavaScript, que es lo que
// convierte un XSS en un robo de sesión.
func (h *Handler) ponerCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.produccion,
		SameSite: http.SameSiteLaxMode,
	})
}
