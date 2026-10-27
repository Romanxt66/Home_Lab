package auth

import (
	"context"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// CuentaRepo es lo que el acceso con Google necesita de la base: buscar la
// cuenta por correo y, si no existe, crearla.
type CuentaRepo interface {
	PorEmail(ctx context.Context, email string) (*Usuario, error)
	Registrar(ctx context.Context, email string, nombre, foto *string) (*Usuario, error)
}

// Avisador manda el aviso de que hay un registro nuevo esperando.
// Es opcional: si no hay notificaciones configuradas, entrar debe seguir
// funcionando igual.
type Avisador interface {
	Send(ctx context.Context, mensaje string) error
}

// ResultadoGoogle dice qué pasó una vez identificada la cuenta.
//
// Quedarse esperando aprobación no es un error, es un desenlace normal: por
// eso viaja como valor y no como fallo, y así la interfaz puede explicarlo
// con calma en vez de enseñar una pantalla de error.
type ResultadoGoogle struct {
	Tipo  TipoResultado
	Nuevo bool // solo para Pendiente: ¿se acaba de crear la cuenta?
	User  *Usuario
}

type TipoResultado string

const (
	ResultadoSesion    TipoResultado = "session"
	ResultadoPendiente TipoResultado = "pending"
	ResultadoRechazado TipoResultado = "rejected"
)

// GoogleLogin es el caso de uso completo.
type GoogleLogin struct {
	repo     CuentaRepo
	avisador Avisador
}

func NewGoogleLogin(repo CuentaRepo, avisador Avisador) *GoogleLogin {
	return &GoogleLogin{repo: repo, avisador: avisador}
}

// Ejecutar identifica a quien vuelve de Google. Si es la primera vez, crea
// la cuenta como pendiente y avisa al superadmin. Solo las cuentas aprobadas
// reciben sesión.
func (g *GoogleLogin) Ejecutar(ctx context.Context, perfil GoogleProfile) (kernel.Result[ResultadoGoogle], error) {
	email := strings.ToLower(strings.TrimSpace(perfil.Email))
	if email == "" {
		return kernel.Err[ResultadoGoogle]("Google no devolvió un correo válido."), nil
	}

	// Un correo que Google no ha verificado no identifica a nadie: podría
	// ser de otra persona.
	if !perfil.EmailVerified {
		return kernel.Err[ResultadoGoogle](
			"Google no ha verificado ese correo, así que no se puede usar para entrar."), nil
	}

	usuario, err := g.repo.PorEmail(ctx, email)
	if err != nil {
		return kernel.Result[ResultadoGoogle]{}, err
	}

	nuevo := usuario == nil
	if nuevo {
		usuario, err = g.repo.Registrar(ctx, email, opcional(perfil.Name), opcional(perfil.Picture))
		if err != nil {
			return kernel.Result[ResultadoGoogle]{}, err
		}
		g.avisarDeRegistro(ctx, perfil.Name, email)
	}

	switch usuario.Status {
	case "pending":
		return kernel.Ok(ResultadoGoogle{Tipo: ResultadoPendiente, Nuevo: nuevo}), nil
	case "rejected":
		return kernel.Ok(ResultadoGoogle{Tipo: ResultadoRechazado}), nil
	default:
		return kernel.Ok(ResultadoGoogle{Tipo: ResultadoSesion, User: usuario}), nil
	}
}

// avisarDeRegistro intenta avisar, pero no deja que un fallo del aviso
// impida el registro: son dos cosas independientes y la cuenta ya está
// creada.
func (g *GoogleLogin) avisarDeRegistro(ctx context.Context, nombre, email string) {
	if g.avisador == nil {
		return
	}
	if nombre == "" {
		nombre = email
	}
	_ = g.avisador.Send(ctx, "🆕 Nuevo registro pendiente de aprobación\n"+nombre+"\n"+email)
}

func opcional(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
