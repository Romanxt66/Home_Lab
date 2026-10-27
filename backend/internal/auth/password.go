package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// Parámetros de scrypt. No son arbitrarios: son los que usa por defecto
// `crypto.scryptSync` de Node, que es con lo que el Lab cifró las contraseñas
// que ya están en la base. Cambiarlos invalidaría todas las cuentas
// existentes, así que se quedan fijos.
const (
	scryptN      = 16384
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 64
	saltLen      = 16
)

// HashPassword devuelve el formato almacenado del Lab: scrypt$<sal>$<hash>.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash, err := scrypt.Key([]byte(plain), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("scrypt$%s$%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// VerifyPassword comprueba una contraseña contra el valor almacenado.
// Entiende los hashes que ya escribió el Lab en Node sin tocarlos.
func VerifyPassword(plain, stored string) bool {
	partes := strings.Split(stored, "$")
	if len(partes) != 3 || partes[0] != "scrypt" {
		return false
	}

	salt, err := hex.DecodeString(partes[1])
	if err != nil {
		return false
	}
	esperado, err := hex.DecodeString(partes[2])
	if err != nil || len(esperado) == 0 {
		return false
	}

	// La longitud se toma del hash guardado, no de la constante: si alguna
	// cuenta antigua se cifró con otro tamaño, sigue validando.
	actual, err := scrypt.Key([]byte(plain), salt, scryptN, scryptR, scryptP, len(esperado))
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(actual, esperado) == 1
}
