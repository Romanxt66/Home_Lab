// Package oauth es el servidor de autorización de Home Lab, para que clientes
// MCP como claude.ai puedan conectarse sin que nadie copie un token a mano.
//
// Sigue lo que pide la especificación de autorización de MCP: registro dinámico
// de clientes (RFC 7591), código de autorización con PKCE obligatorio (RFC 7636,
// solo S256), tokens de acceso de una hora y de refresco rotatorios con
// detección de reutilización, metadatos (RFC 8414 y 9728) y revocación
// (RFC 7009). Solo hay clientes públicos: sin secreto, y PKCE hace de prueba.
//
// La persona decide en una pantalla de consentimiento qué permiso concede
// (solo leer, o leer y guardar documentos); lo concedido se puede revocar.
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	DuracionCodigo   = 5 * time.Minute
	DuracionAcceso   = time.Hour
	DuracionRefresco = 30 * 24 * time.Hour

	// MaxClientes registrados a la vez: el registro es público, así que se acota
	// para que nadie pueda llenar la base con él.
	MaxClientes     = 500
	MaxRedirectURIs = 5
	MaxLongitudURI  = 500

	PrefijoAcceso   = "hlo_"
	PrefijoRefresco = "hlr_"
	// largoToken: prefijo de 4 + 32 bytes en base64 sin relleno (43 caracteres).
	largoToken = 4 + 43

	ScopeLectura   = "mcp:read"
	ScopeEscritura = "mcp:write"
)

// Alcance concedido por la persona.
type Alcance string

const (
	Lectura   Alcance = "read"
	Escritura Alcance = "write"
)

// Cliente es una aplicación registrada.
type Cliente struct {
	ID           string
	Nombre       string
	RedirectURIs []string
	CreadoEn     time.Time
}

// ── Redirecciones ──────────────────────────────────────────────────────────

func esLoopback(host string) bool {
	h := strings.Trim(host, "[]")
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// ValidarRedirectURI comprueba una dirección de retorno al registrar un cliente.
// Acepta https de un host permitido y http solo hacia el propio equipo (los
// clientes de escritorio abren un puerto local). Rechaza fragmentos, datos de
// acceso en la URL y todo lo demás: es lo que impide que un código acabe en
// manos ajenas. permitidos puede llevar «*» para admitir cualquier host https.
func ValidarRedirectURI(crudo string, permitidos []string) (string, bool) {
	if crudo == "" || len(crudo) > MaxLongitudURI || strings.ContainsAny(crudo, "\r\n\x00 ") {
		return "", false
	}
	u, err := url.Parse(crudo)
	if err != nil || u.Host == "" || u.Fragment != "" || u.User != nil || u.Opaque != "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	switch u.Scheme {
	case "http":
		if !esLoopback(host) {
			return "", false
		}
	case "https":
		if esLoopback(host) {
			return "", false // https a localhost no tiene sentido y casi siempre es un error
		}
		ok := false
		for _, p := range permitidos {
			if p == "*" || strings.EqualFold(p, host) {
				ok = true
			}
		}
		if !ok {
			return "", false
		}
	default:
		return "", false
	}
	return u.String(), true
}

// CoincideRedirect compara la dirección pedida con la registrada. Es una
// comparación exacta; solo en loopback se ignora el puerto, porque un cliente de
// escritorio abre uno distinto cada vez (RFC 8252, apartado 7.3).
func CoincideRedirect(registrada, pedida string) bool {
	if registrada == pedida {
		return true
	}
	r, err1 := url.Parse(registrada)
	p, err2 := url.Parse(pedida)
	if err1 != nil || err2 != nil || r.Scheme != "http" || p.Scheme != "http" {
		return false
	}
	return esLoopback(r.Hostname()) && r.Hostname() == p.Hostname() && r.Path == p.Path && r.RawQuery == p.RawQuery
}

// ── PKCE ───────────────────────────────────────────────────────────────────

var reVerifier = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)

// ValidarVerifier: de 43 a 128 caracteres sin reservar (RFC 7636, apartado 4.1).
func ValidarVerifier(v string) bool { return reVerifier.MatchString(v) }

// ValidarChallenge: el SHA-256 de 32 bytes en base64url sin relleno son 43 caracteres.
func ValidarChallenge(c string) bool {
	if len(c) != 43 {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	return err == nil && len(b) == 32
}

// VerificarPKCE comprueba, en tiempo constante, que el verifier corresponde al
// challenge con el método S256. «plain» no se admite: no protege de nada.
func VerificarPKCE(verifier, challenge string) bool {
	if !ValidarVerifier(verifier) {
		return false
	}
	h := sha256.Sum256([]byte(verifier))
	esperado := base64.RawURLEncoding.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(esperado), []byte(challenge)) == 1
}

// ── Permisos ───────────────────────────────────────────────────────────────

// ParseScope interpreta el parámetro scope: dice si pide leer y si pide escribir.
func ParseScope(s string) (lectura, escritura bool) {
	for _, parte := range strings.Fields(s) {
		switch parte {
		case ScopeLectura:
			lectura = true
		case ScopeEscritura:
			escritura = true
		}
	}
	return lectura, escritura
}

// ScopeDe devuelve el scope que se informa al cliente según lo concedido.
func ScopeDe(a Alcance) string {
	if a == Escritura {
		return ScopeLectura + " " + ScopeEscritura
	}
	return ScopeLectura
}

// ── Tokens ─────────────────────────────────────────────────────────────────

// NuevoToken genera un token opaco de 256 bits con el prefijo dado y su huella.
func NuevoToken(prefijo string) (claro, huella string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	claro = prefijo + base64.RawURLEncoding.EncodeToString(b)
	return claro, Huella(claro), nil
}

// Huella es el SHA-256 en hexadecimal: lo único que se guarda de un token.
func Huella(claro string) string {
	h := sha256.Sum256([]byte(claro))
	return hex.EncodeToString(h[:])
}

// FormaValida: ¿tiene pinta de ser un token de este prefijo? Evita ir a la base
// de datos con basura.
func FormaValida(claro, prefijo string) bool {
	return len(claro) == largoToken && strings.HasPrefix(claro, prefijo)
}

// ── Textos ─────────────────────────────────────────────────────────────────

// LimpiarNombre deja el nombre del cliente listo para enseñarlo en la pantalla
// de consentimiento: sin caracteres de control (que podrían disfrazar el texto)
// y de longitud acotada. Es lo que lee la persona al decidir, así que importa.
func LimpiarNombre(n string) string {
	var sb strings.Builder
	for _, r := range strings.TrimSpace(n) {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue // incluye los de dirección de texto, usados para falsear nombres
		}
		sb.WriteRune(r)
	}
	nombre := strings.Join(strings.Fields(sb.String()), " ")
	if r := []rune(nombre); len(r) > 80 {
		nombre = string(r[:80]) + "…"
	}
	if nombre == "" {
		return "Aplicación sin nombre"
	}
	return nombre
}
