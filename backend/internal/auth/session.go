// Package auth reproduce el esquema de sesiones y contraseñas del Lab en Next,
// byte a byte, para que las dos aplicaciones puedan convivir durante la
// migración: una sesión abierta en el Lab vale en esta API y al revés.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	// Mismo nombre de cookie que el Lab en Next.
	SessionCookie = "lab_session"
	// Siete días, igual que SESSION_MAX_AGE del original.
	SessionMaxAge = 60 * 60 * 24 * 7
)

var (
	ErrTokenInvalido   = errors.New("sesión inválida")
	ErrTokenExpirado   = errors.New("sesión expirada")
	ErrTokenMalformado = errors.New("sesión malformada")
)

// SessionPayload es exactamente el objeto que firma el Lab. Los nombres de los
// campos JSON tienen que coincidir o el token no se entiende entre las dos
// aplicaciones.
type SessionPayload struct {
	UID   string  `json:"uid"`
	Email string  `json:"email"`
	Name  *string `json:"name"`
	Role  string  `json:"role"`
	Exp   int64   `json:"exp"` // segundos unix
}

// Sessions firma y verifica tokens con HMAC-SHA256.
type Sessions struct {
	secret []byte
}

func NewSessions(secret string) *Sessions {
	return &Sessions{secret: []byte(secret)}
}

// Create firma un payload y devuelve "<cuerpo>.<firma>" en base64url sin
// relleno, que es el formato que emite y espera el Lab.
func (s *Sessions) Create(uid, email string, name *string, role string) (string, error) {
	payload := SessionPayload{
		UID:   uid,
		Email: email,
		Name:  name,
		Role:  role,
		Exp:   time.Now().Unix() + SessionMaxAge,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	body := base64.RawURLEncoding.EncodeToString(raw)
	return body + "." + base64.RawURLEncoding.EncodeToString(s.sign(body)), nil
}

// Verify comprueba la firma y la caducidad. Devuelve el payload o un error.
func (s *Sessions) Verify(token string) (*SessionPayload, error) {
	if token == "" {
		return nil, ErrTokenInvalido
	}

	body, sig, found := strings.Cut(token, ".")
	if !found || body == "" || sig == "" {
		return nil, ErrTokenMalformado
	}

	firma, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return nil, ErrTokenMalformado
	}

	// Comparación en tiempo constante: comparar firmas con == filtra
	// información sobre cuántos bytes coinciden.
	if !hmac.Equal(firma, s.sign(body)) {
		return nil, ErrTokenInvalido
	}

	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, ErrTokenMalformado
	}

	var payload SessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrTokenMalformado
	}

	if payload.Exp*1000 < time.Now().UnixMilli() {
		return nil, ErrTokenExpirado
	}

	return &payload, nil
}

func (s *Sessions) sign(body string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return mac.Sum(nil)
}
