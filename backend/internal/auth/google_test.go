package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

// ── El state firmado ───────────────────────────────────────────────────────

func TestStateIdaYVuelta(t *testing.T) {
	s := NewSessions("un-secreto")

	state, err := s.CreateState()
	if err != nil {
		t.Fatalf("CreateState: %v", err)
	}
	if !s.VerifyState(state) {
		t.Error("no reconoció un state que acababa de firmar")
	}
}

func TestStateManipuladoSeRechaza(t *testing.T) {
	s := NewSessions("un-secreto")
	state, _ := s.CreateState()
	nonce, firma, _ := strings.Cut(state, ".")

	casos := map[string]string{
		"vacío":               "",
		"sin firma":           nonce,
		"sin punto":           nonce + firma,
		"firma cambiada":      nonce + ".0000000000000000000000000000000000000000000000000000000000000000",
		"nonce cambiado":      "ffffffffffffffffffffffffffffffff." + firma,
		"firma más corta":     nonce + "." + firma[:10],
		"solo el separador":   ".",
		"nonce de otra gente": "abc.def",
	}

	for nombre, malo := range casos {
		t.Run(nombre, func(t *testing.T) {
			if s.VerifyState(malo) {
				t.Errorf("aceptó un state manipulado: %q", malo)
			}
		})
	}
}

// Un state firmado con otro secreto no vale: es justo el ataque del que
// protege, alguien que fabrica su propia vuelta de OAuth.
func TestStateDeOtroSecretoSeRechaza(t *testing.T) {
	ajeno, _ := NewSessions("otro-secreto").CreateState()
	if NewSessions("un-secreto").VerifyState(ajeno) {
		t.Error("aceptó un state firmado con otro secreto")
	}
}

// ── La URL de consentimiento ───────────────────────────────────────────────

func TestAuthURLPideLoMinimo(t *testing.T) {
	g := NewGoogleClient(GoogleConfig{
		ClientID:     "id-de-cliente",
		ClientSecret: "secreto",
		RedirectURI:  "https://ejemplo.com/api/auth/google/login/callback",
	})

	u, err := url.Parse(g.AuthURL("el-state"))
	if err != nil {
		t.Fatalf("la URL no es válida: %v", err)
	}
	q := u.Query()

	if q.Get("scope") != "openid email profile" {
		t.Errorf("pide permisos de más: %q", q.Get("scope"))
	}
	// Para entrar no hace falta refresh token; pedirlo sería pedir de más.
	if q.Get("access_type") != "online" {
		t.Errorf("access_type = %q, quería online", q.Get("access_type"))
	}
	if q.Get("state") != "el-state" {
		t.Errorf("no viajó el state: %q", q.Get("state"))
	}
	if q.Get("redirect_uri") != "https://ejemplo.com/api/auth/google/login/callback" {
		t.Errorf("redirect_uri mal: %q", q.Get("redirect_uri"))
	}
}

func TestConfigurado(t *testing.T) {
	completa := GoogleConfig{ClientID: "a", ClientSecret: "b", RedirectURI: "c"}
	if !completa.Configurado() {
		t.Error("una configuración completa debería valer")
	}
	for _, falta := range []GoogleConfig{
		{ClientSecret: "b", RedirectURI: "c"},
		{ClientID: "a", RedirectURI: "c"},
		{ClientID: "a", ClientSecret: "b"},
		{},
	} {
		if falta.Configurado() {
			t.Errorf("dio por buena una configuración incompleta: %+v", falta)
		}
	}
}

// ── El caso de uso ─────────────────────────────────────────────────────────

type cuentasFalsas struct {
	existente  *Usuario
	registrado *Usuario
	fallo      error
}

func (c *cuentasFalsas) PorEmail(context.Context, string) (*Usuario, error) {
	if c.fallo != nil {
		return nil, c.fallo
	}
	return c.existente, nil
}

func (c *cuentasFalsas) Registrar(_ context.Context, email string, nombre, _ *string) (*Usuario, error) {
	if c.fallo != nil {
		return nil, c.fallo
	}
	c.registrado = &Usuario{ID: "nuevo", Email: email, Name: nombre, Role: "user", Status: "pending"}
	return c.registrado, nil
}

type avisadorFalso struct {
	mensajes []string
	fallo    error
}

func (a *avisadorFalso) Send(_ context.Context, m string) error {
	a.mensajes = append(a.mensajes, m)
	return a.fallo
}

type automatizadorFalso struct {
	disparados []string
	valores    []map[string]string
}

func (a *automatizadorFalso) Disparar(_ context.Context, d string, v map[string]string) {
	a.disparados = append(a.disparados, d)
	a.valores = append(a.valores, v)
}

func perfil(email string) GoogleProfile {
	return GoogleProfile{Email: email, Name: "Quien Sea", EmailVerified: true}
}

func TestCuentaNuevaSeRegistraPendienteYAvisa(t *testing.T) {
	repo := &cuentasFalsas{}
	avisos := &avisadorFalso{}

	res, err := NewGoogleLogin(repo, avisos, nil).Ejecutar(context.Background(), perfil("nuevo@ejemplo.com"))
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}

	if res.Value.Tipo != ResultadoPendiente || !res.Value.Nuevo {
		t.Errorf("esperaba pendiente y nuevo, salió %+v", res.Value)
	}
	if repo.registrado == nil || repo.registrado.Status != "pending" {
		t.Error("no creó la cuenta como pendiente")
	}
	if len(avisos.mensajes) != 1 {
		t.Errorf("mandó %d avisos, esperaba 1", len(avisos.mensajes))
	}
}

func TestCuentaAprobadaRecibeSesion(t *testing.T) {
	repo := &cuentasFalsas{existente: &Usuario{
		ID: "u1", Email: "yo@ejemplo.com", Role: "superadmin", Status: "approved",
	}}

	res, err := NewGoogleLogin(repo, &avisadorFalso{}, nil).Ejecutar(context.Background(), perfil("yo@ejemplo.com"))
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if res.Value.Tipo != ResultadoSesion {
		t.Fatalf("esperaba sesión, salió %q", res.Value.Tipo)
	}
	if res.Value.User.ID != "u1" {
		t.Errorf("devolvió otro usuario: %+v", res.Value.User)
	}
}

func TestCuentaPendienteYaExistenteNoEsNueva(t *testing.T) {
	repo := &cuentasFalsas{existente: &Usuario{ID: "u1", Status: "pending"}}
	avisos := &avisadorFalso{}

	res, _ := NewGoogleLogin(repo, avisos, nil).Ejecutar(context.Background(), perfil("yo@ejemplo.com"))
	if res.Value.Tipo != ResultadoPendiente || res.Value.Nuevo {
		t.Errorf("esperaba pendiente y no nueva, salió %+v", res.Value)
	}
	if len(avisos.mensajes) != 0 {
		t.Error("volvió a avisar de un registro que ya existía")
	}
}

func TestCuentaRechazadaNoEntra(t *testing.T) {
	repo := &cuentasFalsas{existente: &Usuario{ID: "u1", Status: "rejected"}}

	res, _ := NewGoogleLogin(repo, &avisadorFalso{}, nil).Ejecutar(context.Background(), perfil("yo@ejemplo.com"))
	if res.Value.Tipo != ResultadoRechazado {
		t.Errorf("dejó entrar a una cuenta revocada: %+v", res.Value)
	}
}

func TestCorreoSinVerificarSeRechaza(t *testing.T) {
	repo := &cuentasFalsas{}
	p := perfil("sinverificar@ejemplo.com")
	p.EmailVerified = false

	res, err := NewGoogleLogin(repo, &avisadorFalso{}, nil).Ejecutar(context.Background(), p)
	if err != nil {
		t.Fatalf("debía ser error de negocio: %v", err)
	}
	if res.OK {
		t.Error("aceptó un correo que Google no ha verificado")
	}
	if repo.registrado != nil {
		t.Error("llegó a crear la cuenta pese a rechazar el correo")
	}
}

func TestCorreoVacioSeRechaza(t *testing.T) {
	res, _ := NewGoogleLogin(&cuentasFalsas{}, &avisadorFalso{}, nil).
		Ejecutar(context.Background(), GoogleProfile{Email: "   ", EmailVerified: true})
	if res.OK {
		t.Error("aceptó un perfil sin correo")
	}
}

// El aviso es accesorio: que falle no puede impedir el registro.
func TestUnAvisoQueFallaNoRompeElRegistro(t *testing.T) {
	repo := &cuentasFalsas{}
	avisos := &avisadorFalso{fallo: errors.New("telegram caído")}

	res, err := NewGoogleLogin(repo, avisos, nil).Ejecutar(context.Background(), perfil("nuevo@ejemplo.com"))
	if err != nil || !res.OK {
		t.Fatalf("el fallo del aviso tumbó el registro: err=%v res=%+v", err, res)
	}
	if repo.registrado == nil {
		t.Error("no creó la cuenta")
	}
}

// Sin notificaciones configuradas, entrar debe seguir funcionando.
func TestSinAvisadorNoPetaraNada(t *testing.T) {
	repo := &cuentasFalsas{}
	res, err := NewGoogleLogin(repo, nil, nil).Ejecutar(context.Background(), perfil("nuevo@ejemplo.com"))
	if err != nil || !res.OK {
		t.Fatalf("falló sin avisador: err=%v res=%+v", err, res)
	}
}

// Un fallo de base sí es técnico y debe subir.
func TestFalloDeBaseSubeComoErrorEnGoogle(t *testing.T) {
	repo := &cuentasFalsas{fallo: errors.New("sin conexión")}
	if _, err := NewGoogleLogin(repo, nil, nil).Ejecutar(context.Background(), perfil("a@b.com")); err == nil {
		t.Error("se tragó un fallo de base")
	}
}

// El correo se normaliza: Google puede devolverlo con mayúsculas.
func TestCorreoSeNormaliza(t *testing.T) {
	repo := &cuentasFalsas{}
	_, _ = NewGoogleLogin(repo, nil, nil).Ejecutar(context.Background(), perfil("  Yo@Ejemplo.COM  "))
	if repo.registrado == nil || repo.registrado.Email != "yo@ejemplo.com" {
		t.Errorf("no normalizó el correo: %+v", repo.registrado)
	}
}

// Un registro nuevo también debe dejar que salten las reglas del usuario.
func TestUnRegistroNuevoDisparaLasAutomatizaciones(t *testing.T) {
	repo := &cuentasFalsas{}
	autos := &automatizadorFalso{}

	_, _ = NewGoogleLogin(repo, &avisadorFalso{}, autos).
		Ejecutar(context.Background(), perfil("nuevo@ejemplo.com"))

	if len(autos.disparados) != 1 || autos.disparados[0] != "user_registered" {
		t.Fatalf("disparó %v", autos.disparados)
	}
	if autos.valores[0]["email"] != "nuevo@ejemplo.com" {
		t.Errorf("no pasó el correo: %v", autos.valores[0])
	}
	if autos.valores[0]["nombre"] != "Quien Sea" {
		t.Errorf("no pasó el nombre: %v", autos.valores[0])
	}
}

// Entrar con una cuenta que ya existe no es un registro nuevo.
func TestEntrarNoDisparaElRegistro(t *testing.T) {
	repo := &cuentasFalsas{existente: &Usuario{ID: "u1", Status: "approved"}}
	autos := &automatizadorFalso{}

	_, _ = NewGoogleLogin(repo, &avisadorFalso{}, autos).
		Ejecutar(context.Background(), perfil("yo@ejemplo.com"))

	if len(autos.disparados) != 0 {
		t.Errorf("disparó %v al entrar con una cuenta existente", autos.disparados)
	}
}

// Sin nombre en el perfil, se usa el correo: un aviso que diga "se registró"
// sin decir quién no sirve de nada.
func TestSinNombreSeUsaElCorreo(t *testing.T) {
	repo := &cuentasFalsas{}
	autos := &automatizadorFalso{}
	p := GoogleProfile{Email: "anonimo@ejemplo.com", EmailVerified: true}

	_, _ = NewGoogleLogin(repo, &avisadorFalso{}, autos).Ejecutar(context.Background(), p)

	if autos.valores[0]["nombre"] != "anonimo@ejemplo.com" {
		t.Errorf("nombre = %q", autos.valores[0]["nombre"])
	}
}
