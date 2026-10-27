package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// El state de OAuth: un número aleatorio más su firma, con el formato
// <nonce>.<firma>. Va firmado con el mismo secreto de sesión para poder
// comprobar en la vuelta que la petición la empezamos nosotros y no un
// tercero, que es la protección contra CSRF del flujo de OAuth.
//
// Mismo formato y misma cookie que el Lab, para que los dos puedan atender
// la misma redirección de Google.
const (
	OAuthStateCookie = "lab_oauth_state"
	OAuthStateMaxAge = 60 * 10 // diez minutos
)

// CreateState genera un state nuevo y firmado.
func (s *Sessions) CreateState() (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	texto := hex.EncodeToString(nonce)
	return texto + "." + s.firmarNonce(texto), nil
}

// VerifyState comprueba que el state lo firmamos nosotros.
func (s *Sessions) VerifyState(state string) bool {
	nonce, firma, hay := strings.Cut(state, ".")
	if !hay || nonce == "" || firma == "" {
		return false
	}

	// hmac.Equal compara en tiempo constante: comparar con == filtraría,
	// por lo que tarda, cuántos caracteres iniciales acertó quien lo intenta.
	return hmac.Equal([]byte(s.firmarNonce(nonce)), []byte(firma))
}

func (s *Sessions) firmarNonce(nonce string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}
