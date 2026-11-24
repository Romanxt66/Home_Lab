package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Romanxt66/Home_Lab/backend/internal/tokens"
)

// ErrorOAuth es un error con el vocabulario del protocolo (RFC 6749, apartado 5.2).
type ErrorOAuth struct {
	Codigo      string
	Descripcion string
	Estado      int
	// NoRedirigir: el fallo es del propio cliente o de su dirección de retorno, y
	// mandar a la persona allí sería justo lo que hay que evitar. Se le enseña el
	// error en pantalla.
	NoRedirigir bool
}

func (e *ErrorOAuth) Error() string { return e.Codigo + ": " + e.Descripcion }

func nuevoError(codigo, desc string) *ErrorOAuth {
	return &ErrorOAuth{Codigo: codigo, Descripcion: desc, Estado: 400}
}

// Codigo es un código de autorización pendiente de canjear.
type Codigo struct {
	Huella      string
	ClienteID   string
	UsuarioID   string
	RedirectURI string
	Challenge   string
	Alcance     Alcance
	Recurso     string
	Caduca      time.Time
}

// Concesion es lo que una persona ha autorizado a una aplicación.
type Concesion struct {
	ID            string
	UsuarioID     string
	ClienteID     string
	ClienteNombre string
	Alcance       Alcance
	CreadaEn      time.Time
	UsadaEn       *time.Time
}

// RegistroToken es la huella de un token y cuándo caduca.
type RegistroToken struct {
	Huella string
	Caduca time.Time
}

// ResultadoRotacion dice qué pasó al intentar usar un token de refresco.
type ResultadoRotacion int

const (
	Rotado ResultadoRotacion = iota
	Invalido
	// Reutilizado: ese refresco ya se había usado. Quien lo presenta es o el
	// cliente legítimo o alguien que lo robó, y no se puede saber cuál: se
	// revoca toda la concesión y los dos tienen que volver a autorizar.
	Reutilizado
)

// Repo guarda clientes, códigos, concesiones y tokens.
type Repo interface {
	CrearCliente(ctx context.Context, c Cliente) error
	ObtenerCliente(ctx context.Context, id string) (*Cliente, error)
	ContarClientes(ctx context.Context) (int, error)
	LimpiarClientesSinUso(ctx context.Context, antes time.Time) error

	GuardarCodigo(ctx context.Context, c Codigo) error
	// CanjearCodigo marca el código como usado y lo devuelve; nil si no existe,
	// ya se usó o caducó. Tiene que ser atómico: dos canjes a la vez no pueden
	// ganar los dos.
	CanjearCodigo(ctx context.Context, huella string, ahora time.Time) (*Codigo, error)

	CrearConcesion(ctx context.Context, c Concesion, acceso, refresco RegistroToken) error
	Rotar(ctx context.Context, huellaRefresco string, ahora time.Time, acceso, refresco RegistroToken) (*Concesion, ResultadoRotacion, error)
	// BuscarAcceso devuelve quién hay detrás de un token de acceso vigente, o nil.
	BuscarAcceso(ctx context.Context, huella string, ahora time.Time) (*tokens.Identidad, error)

	ListarConcesiones(ctx context.Context, usuarioID string) ([]Concesion, error)
	RevocarConcesion(ctx context.Context, usuarioID, id string, ahora time.Time) (bool, error)
	RevocarPorToken(ctx context.Context, huella, clienteID string, ahora time.Time) error
}

type Service struct {
	repo          Repo
	hostsRedirect []string
	ahora         func() time.Time
}

// NewService: hostsRedirect son los hosts https a los que se permite volver
// (por ejemplo claude.ai); «*» los permite todos.
func NewService(repo Repo, hostsRedirect []string) *Service {
	return &Service{repo: repo, hostsRedirect: hostsRedirect, ahora: func() time.Time { return time.Now().UTC() }}
}

// ── Registro ───────────────────────────────────────────────────────────────

type RegistroEntrada struct {
	Nombre       string
	RedirectURIs []string
}

func (s *Service) Registrar(ctx context.Context, in RegistroEntrada) (*Cliente, *ErrorOAuth) {
	if len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > MaxRedirectURIs {
		return nil, nuevoError("invalid_redirect_uri", "Indica entre 1 y 5 direcciones de retorno (redirect_uris).")
	}
	vistas := map[string]bool{}
	var limpias []string
	for _, r := range in.RedirectURIs {
		u, ok := ValidarRedirectURI(r, s.hostsRedirect)
		if !ok {
			return nil, nuevoError("invalid_redirect_uri", "Una dirección de retorno no está permitida. Se admite https en hosts autorizados y http solo hacia localhost.")
		}
		if !vistas[u] {
			vistas[u] = true
			limpias = append(limpias, u)
		}
	}

	ahora := s.ahora()
	// Los registros que nadie llegó a usar se descartan al cabo de una semana, para
	// que un registro público no pueda llenar la base ni agotar el cupo.
	_ = s.repo.LimpiarClientesSinUso(ctx, ahora.Add(-7*24*time.Hour))
	n, err := s.repo.ContarClientes(ctx)
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo registrar el cliente.", Estado: 500}
	}
	if n >= MaxClientes {
		return nil, &ErrorOAuth{Codigo: "temporarily_unavailable", Descripcion: "Hay demasiados clientes registrados ahora mismo. Inténtalo más tarde.", Estado: 503}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo registrar el cliente.", Estado: 500}
	}
	c := Cliente{ID: "hlc_" + hex.EncodeToString(b), Nombre: LimpiarNombre(in.Nombre), RedirectURIs: limpias, CreadoEn: ahora}
	if err := s.repo.CrearCliente(ctx, c); err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo registrar el cliente.", Estado: 500}
	}
	return &c, nil
}

// ── Autorización ───────────────────────────────────────────────────────────

// Peticion es lo que llega a /oauth/authorize.
type Peticion struct {
	TipoRespuesta string
	ClienteID     string
	RedirectURI   string
	Estado        string
	Challenge     string
	Metodo        string
	Scope         string
	Recurso       string
}

// ValidarPeticion comprueba una petición de autorización. Primero lo que, si
// falla, impide volver al cliente (cliente y dirección); después lo demás.
func (s *Service) ValidarPeticion(ctx context.Context, p Peticion, recursoEsperado string) (*Cliente, *ErrorOAuth) {
	if p.ClienteID == "" {
		return nil, &ErrorOAuth{Codigo: "invalid_request", Descripcion: "Falta client_id.", Estado: 400, NoRedirigir: true}
	}
	c, err := s.repo.ObtenerCliente(ctx, p.ClienteID)
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo comprobar la aplicación.", Estado: 500, NoRedirigir: true}
	}
	if c == nil {
		return nil, &ErrorOAuth{Codigo: "invalid_request", Descripcion: "La aplicación no está registrada.", Estado: 400, NoRedirigir: true}
	}
	coincide := false
	for _, r := range c.RedirectURIs {
		if CoincideRedirect(r, p.RedirectURI) {
			coincide = true
		}
	}
	if !coincide {
		return nil, &ErrorOAuth{Codigo: "invalid_request", Descripcion: "La dirección de retorno no coincide con la registrada.", Estado: 400, NoRedirigir: true}
	}

	if p.TipoRespuesta != "code" {
		return c, nuevoError("unsupported_response_type", "Solo se admite response_type=code.")
	}
	if !ValidarChallenge(p.Challenge) || p.Metodo != "S256" {
		return c, nuevoError("invalid_request", "Hace falta PKCE con code_challenge_method=S256.")
	}
	if p.Recurso != "" && strings.TrimRight(p.Recurso, "/") != strings.TrimRight(recursoEsperado, "/") {
		return c, nuevoError("invalid_target", "El recurso pedido no es este servidor.")
	}
	return c, nil
}

// Aprobar cierra la autorización con la decisión de la persona y devuelve la
// dirección a la que hay que mandarla. Vuelve a validar todo: lo que llega del
// navegador no se da por bueno aunque ya se comprobara al entrar.
func (s *Service) Aprobar(ctx context.Context, usuarioID string, p Peticion, recursoEsperado string, aprobado bool, permiso Alcance, issuer string) (string, *ErrorOAuth) {
	c, e := s.ValidarPeticion(ctx, p, recursoEsperado)
	if e != nil && e.NoRedirigir {
		return "", e
	}
	if e != nil {
		return redireccion(p.RedirectURI, map[string]string{"error": e.Codigo, "error_description": e.Descripcion, "state": p.Estado, "iss": issuer}), nil
	}
	if !aprobado {
		return redireccion(p.RedirectURI, map[string]string{"error": "access_denied", "error_description": "La persona no autorizó el acceso.", "state": p.Estado, "iss": issuer}), nil
	}
	if permiso != Lectura && permiso != Escritura {
		permiso = Lectura // ante la duda, el permiso menor
	}

	claro, huella, err := NuevoToken("")
	if err != nil {
		return "", &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo crear el código.", Estado: 500}
	}
	if err := s.repo.GuardarCodigo(ctx, Codigo{
		Huella: huella, ClienteID: c.ID, UsuarioID: usuarioID, RedirectURI: p.RedirectURI,
		Challenge: p.Challenge, Alcance: permiso, Recurso: p.Recurso, Caduca: s.ahora().Add(DuracionCodigo),
	}); err != nil {
		return "", &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo crear el código.", Estado: 500}
	}
	return redireccion(p.RedirectURI, map[string]string{"code": claro, "state": p.Estado, "iss": issuer}), nil
}

// redireccion añade parámetros a la dirección de retorno sin tocar los que ya llevara.
func redireccion(base string, params map[string]string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ── Tokens ─────────────────────────────────────────────────────────────────

type TokenEntrada struct {
	GrantType   string
	Codigo      string
	RedirectURI string
	ClienteID   string
	Verifier    string
	Refresco    string
	Recurso     string
}

type Tokens struct {
	Acceso   string
	Refresco string
	ExpiraEn int
	Alcance  Alcance
}

func (s *Service) Canjear(ctx context.Context, in TokenEntrada, recursoEsperado string) (*Tokens, *ErrorOAuth) {
	switch in.GrantType {
	case "authorization_code":
		return s.canjearCodigo(ctx, in, recursoEsperado)
	case "refresh_token":
		return s.refrescar(ctx, in)
	}
	return nil, nuevoError("unsupported_grant_type", "Solo se admite authorization_code y refresh_token.")
}

func (s *Service) emitir() (acceso, refresco string, ra, rr RegistroToken, err error) {
	ahora := s.ahora()
	acceso, ha, err := NuevoToken(PrefijoAcceso)
	if err != nil {
		return
	}
	refresco, hr, err := NuevoToken(PrefijoRefresco)
	if err != nil {
		return
	}
	return acceso, refresco, RegistroToken{ha, ahora.Add(DuracionAcceso)}, RegistroToken{hr, ahora.Add(DuracionRefresco)}, nil
}

func (s *Service) canjearCodigo(ctx context.Context, in TokenEntrada, recursoEsperado string) (*Tokens, *ErrorOAuth) {
	if in.Codigo == "" || in.RedirectURI == "" || in.ClienteID == "" || in.Verifier == "" {
		return nil, nuevoError("invalid_request", "Faltan parámetros: code, redirect_uri, client_id y code_verifier son obligatorios.")
	}
	cl, err := s.repo.ObtenerCliente(ctx, in.ClienteID)
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo comprobar la aplicación.", Estado: 500}
	}
	if cl == nil {
		return nil, &ErrorOAuth{Codigo: "invalid_client", Descripcion: "La aplicación no está registrada.", Estado: 401}
	}

	// El código se quema nada más intentarlo: un intento fallido no da una segunda oportunidad.
	cod, err := s.repo.CanjearCodigo(ctx, Huella(in.Codigo), s.ahora())
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo canjear el código.", Estado: 500}
	}
	// Un único mensaje para todo lo que sea «este código no vale», sin dar pistas.
	malo := nuevoError("invalid_grant", "El código no es válido, ya se usó o caducó.")
	if cod == nil || cod.ClienteID != in.ClienteID || cod.RedirectURI != in.RedirectURI || !VerificarPKCE(in.Verifier, cod.Challenge) {
		return nil, malo
	}
	if in.Recurso != "" && strings.TrimRight(in.Recurso, "/") != strings.TrimRight(recursoEsperado, "/") {
		return nil, nuevoError("invalid_target", "El recurso pedido no es este servidor.")
	}

	acceso, refresco, ra, rr, err := s.emitir()
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudieron crear los tokens.", Estado: 500}
	}
	g := Concesion{ID: uuid.NewString(), UsuarioID: cod.UsuarioID, ClienteID: cod.ClienteID, Alcance: cod.Alcance, CreadaEn: s.ahora()}
	if err := s.repo.CrearConcesion(ctx, g, ra, rr); err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudieron crear los tokens.", Estado: 500}
	}
	return &Tokens{Acceso: acceso, Refresco: refresco, ExpiraEn: int(DuracionAcceso.Seconds()), Alcance: cod.Alcance}, nil
}

func (s *Service) refrescar(ctx context.Context, in TokenEntrada) (*Tokens, *ErrorOAuth) {
	if in.Refresco == "" || in.ClienteID == "" {
		return nil, nuevoError("invalid_request", "Faltan parámetros: refresh_token y client_id son obligatorios.")
	}
	malo := nuevoError("invalid_grant", "El token de refresco no es válido, caducó o fue revocado.")
	if !FormaValida(in.Refresco, PrefijoRefresco) {
		return nil, malo
	}
	acceso, refresco, ra, rr, err := s.emitir()
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudieron crear los tokens.", Estado: 500}
	}
	g, res, err := s.repo.Rotar(ctx, Huella(in.Refresco), s.ahora(), ra, rr)
	if err != nil {
		return nil, &ErrorOAuth{Codigo: "server_error", Descripcion: "No se pudo renovar el acceso.", Estado: 500}
	}
	if res != Rotado || g == nil || g.ClienteID != in.ClienteID {
		return nil, malo
	}
	return &Tokens{Acceso: acceso, Refresco: refresco, ExpiraEn: int(DuracionAcceso.Seconds()), Alcance: g.Alcance}, nil
}

// ── Uso y gestión ──────────────────────────────────────────────────────────

// Autenticar resuelve un token de acceso a la persona que hay detrás.
func (s *Service) Autenticar(ctx context.Context, claro string) (*tokens.Identidad, error) {
	if !FormaValida(claro, PrefijoAcceso) {
		return nil, nil
	}
	return s.repo.BuscarAcceso(ctx, Huella(claro), s.ahora())
}

func (s *Service) Concesiones(ctx context.Context, usuarioID string) ([]Concesion, error) {
	return s.repo.ListarConcesiones(ctx, usuarioID)
}

func (s *Service) RevocarConcesion(ctx context.Context, usuarioID, id string) (bool, error) {
	return s.repo.RevocarConcesion(ctx, usuarioID, id, s.ahora())
}

// RevocarToken atiende /oauth/revoke: quita la concesión a la que pertenece el
// token, si es de ese cliente. Siempre «funciona» de cara al exterior (RFC 7009).
func (s *Service) RevocarToken(ctx context.Context, claro, clienteID string) error {
	if !FormaValida(claro, PrefijoAcceso) && !FormaValida(claro, PrefijoRefresco) {
		return nil
	}
	return s.repo.RevocarPorToken(ctx, Huella(claro), clienteID, s.ahora())
}
