package users

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Romanxt66/Home_Lab/backend/internal/httpx"
	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// usuarioDTO es lo que ve el navegador. No sale el hash de la contraseña:
// en su lugar va authMethod, que es lo único que la interfaz necesita saber.
type usuarioDTO struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	Name       *string `json:"name"`
	Picture    *string `json:"picture"`
	Role       string  `json:"role"`
	Status     string  `json:"status"`
	AuthMethod string  `json:"authMethod"`
	CreatedAt  string  `json:"createdAt"`
	IsSelf     bool    `json:"isSelf"`
}

func aDTO(u User, yo string) usuarioDTO {
	return usuarioDTO{
		ID:         u.ID,
		Email:      u.Email,
		Name:       u.Name,
		Picture:    u.Picture,
		Role:       string(u.Role),
		Status:     string(u.Status),
		AuthMethod: u.AuthMethod(),
		CreatedAt:  u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		IsSelf:     u.ID == yo,
	}
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register cuelga las rutas. El grupo que se le pasa ya exige superadmin.
func (h *Handler) Register(g *gin.RouterGroup) {
	g.GET("", h.list)
	g.POST("/:id/approve", h.approve)
	g.POST("/:id/reject", h.reject)
	g.PUT("/:id/role", h.setRole)
}

// yo devuelve el id de quien hace la petición, para marcar su propia fila y
// para impedir que se aplique a sí mismo los cambios peligrosos.
func yo(c *gin.Context) string {
	if u, ok := httpx.CurrentUser(c); ok {
		return u.UID
	}
	return ""
}

func (h *Handler) list(c *gin.Context) {
	usuarios, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpx.ServerError(c, err)
		return
	}

	salida := make([]usuarioDTO, 0, len(usuarios))
	for _, u := range usuarios {
		salida = append(salida, aDTO(u, yo(c)))
	}
	c.JSON(http.StatusOK, salida)
}

func (h *Handler) approve(c *gin.Context) {
	res, err := h.svc.Approve(c.Request.Context(), c.Param("id"))
	h.responder(c, res, err)
}

func (h *Handler) reject(c *gin.Context) {
	res, err := h.svc.Reject(c.Request.Context(), c.Param("id"), yo(c))
	h.responder(c, res, err)
}

func (h *Handler) setRole(c *gin.Context) {
	var cuerpo struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&cuerpo); err != nil {
		httpx.BadRequest(c, "No se entendió la petición.")
		return
	}

	res, err := h.svc.SetRole(c.Request.Context(), c.Param("id"), Role(cuerpo.Role), yo(c))
	h.responder(c, res, err)
}

// responder traduce el Result: un error de negocio es un 400 con su mensaje,
// uno técnico un 500 genérico.
func (h *Handler) responder(c *gin.Context, res kernel.Result[User], err error) {
	if err != nil {
		httpx.ServerError(c, err)
		return
	}
	if !res.OK {
		httpx.BadRequest(c, res.Error)
		return
	}
	c.JSON(http.StatusOK, aDTO(res.Value, yo(c)))
}
