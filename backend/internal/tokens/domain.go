// Package tokens son los tokens personales de acceso: la llave con la que un
// programa (por ejemplo Claude Code, por MCP) actúa en nombre de una persona sin
// usar su contraseña ni su sesión.
//
// El token solo se enseña una vez, al crearlo; en la base queda su huella
// (SHA-256), así que ni un volcado de la base permite usarlo. Hereda los
// permisos de quien lo creó y se puede revocar en cualquier momento.
package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Alcance es lo que un token puede hacer.
type Alcance string

const (
	// Lectura: leer lo que la persona ve. Es el valor por defecto.
	Lectura Alcance = "read"
	// Escritura: además, guardar documentos.
	Escritura Alcance = "write"
)

// PuedeEscribir dice si el alcance deja guardar cosas.
func (a Alcance) PuedeEscribir() bool { return a == Escritura }

const (
	// Prefijo reconocible: si alguien lo pega por error en un repositorio, los
	// buscadores de secretos (y una búsqueda a mano) lo encuentran.
	Prefijo = "hl_"
	// MaxActivos por persona: un token olvidado es una puerta abierta.
	MaxActivos = 10
	// largoClaro: «hl_» + 32 bytes en base64 sin relleno (43 caracteres).
	largoClaro = len(Prefijo) + 43
	// MaxLongitudNombre del nombre que se le pone a un token.
	MaxLongitudNombre = 60
)

// Token es lo que se lista; nunca lleva el secreto.
type Token struct {
	ID         string
	DuenoID    string
	Nombre     string
	Alcance    Alcance
	Prefijo    string // los primeros caracteres, para reconocerlo en la lista
	CreadoEn   time.Time
	UsadoEn    *time.Time
	RevocadoEn *time.Time
}

// Activo: no está revocado.
func (t Token) Activo() bool { return t.RevocadoEn == nil }

// Identidad es quién está detrás de un token válido.
type Identidad struct {
	TokenID   string
	UsuarioID string
	Nombre    string
	Admin     bool
	Alcance   Alcance
}

// Generar crea un token aleatorio de 256 bits. Devuelve el secreto (que se
// enseña una vez), su huella (lo que se guarda) y el prefijo visible.
func Generar() (claro, huella, visible string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	claro = Prefijo + base64.RawURLEncoding.EncodeToString(b)
	return claro, Huella(claro), claro[:len(Prefijo)+6], nil
}

// Huella es el SHA-256 en hexadecimal. No hace falta un hash lento (bcrypt,
// scrypt): el token no es una contraseña elegida por una persona sino 256 bits
// aleatorios, imposibles de adivinar por fuerza bruta.
func Huella(claro string) string {
	h := sha256.Sum256([]byte(claro))
	return hex.EncodeToString(h[:])
}

// FormaValida descarta de entrada lo que no puede ser un token nuestro, sin
// tocar la base de datos.
func FormaValida(claro string) bool {
	return len(claro) == largoClaro && strings.HasPrefix(claro, Prefijo)
}

// ValidarNombre comprueba el nombre con el que se reconocerá el token.
func ValidarNombre(n string) kernel.Result[string] {
	n = strings.TrimSpace(n)
	if n == "" {
		return kernel.Err[string]("Ponle un nombre al token, por ejemplo «Mi portátil».")
	}
	if len([]rune(n)) > MaxLongitudNombre {
		return kernel.Err[string]("El nombre es demasiado largo (máximo 60 caracteres).")
	}
	return kernel.Ok(n)
}

// ValidarAlcance acepta read y write; vacío es read, lo más prudente.
func ValidarAlcance(a string) kernel.Result[Alcance] {
	switch Alcance(a) {
	case "", Lectura:
		return kernel.Ok(Lectura)
	case Escritura:
		return kernel.Ok(Escritura)
	}
	return kernel.Err[Alcance]("El permiso debe ser «read» (solo leer) o «write» (leer y guardar).")
}

// ExtraerBearer saca el token de una cabecera «Authorization: Bearer …».
func ExtraerBearer(cabecera string) (string, bool) {
	const pref = "Bearer "
	if len(cabecera) <= len(pref) || !strings.EqualFold(cabecera[:len(pref)], pref) {
		return "", false
	}
	return strings.TrimSpace(cabecera[len(pref):]), true
}
