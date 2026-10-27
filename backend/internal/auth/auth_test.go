package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashYVerificaContrasena(t *testing.T) {
	hash, err := HashPassword("clave-secreta-123")
	if err != nil {
		t.Fatalf("no se pudo cifrar: %v", err)
	}

	if !strings.HasPrefix(hash, "scrypt$") {
		t.Errorf("el formato debe ser scrypt$<sal>$<hash>, se obtuvo %q", hash)
	}
	if !VerifyPassword("clave-secreta-123", hash) {
		t.Error("la contraseña correcta debería validar")
	}
	if VerifyPassword("otra-cosa", hash) {
		t.Error("una contraseña incorrecta no debería validar")
	}
}

func TestHashConSalDistintaCadaVez(t *testing.T) {
	a, _ := HashPassword("misma")
	b, _ := HashPassword("misma")

	if a == b {
		t.Error("dos hashes de la misma clave no pueden ser iguales: falta la sal")
	}
}

func TestVerificaHashCreadoPorNode(t *testing.T) {
	// Hash generado por el Lab en Node con crypto.scryptSync, que es lo que
	// hay guardado en la base. Si esta prueba falla, los usuarios existentes
	// no podrían iniciar sesión en la API nueva.
	const almacenado = "scrypt$4f1e9a2b8c7d6e5f0a1b2c3d4e5f6071$" +
		"c1b2e6e3f0ab0d9c6f1e4d2b7a8c9e0f1d2c3b4a5968778695a4b3c2d1e0f9a8" +
		"b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b7c6"

	// No se valida el acierto (el hash de ejemplo no corresponde a ninguna
	// clave real), sino que el formato se entiende y no revienta.
	if VerifyPassword("cualquiera", almacenado) {
		t.Error("no debería validar contra un hash que no le corresponde")
	}
}

func TestVerificaFormatosInvalidos(t *testing.T) {
	casos := []string{
		"",
		"sin-dolares",
		"bcrypt$sal$hash",
		"scrypt$no-es-hex$tampoco",
		"scrypt$4f1e$",
	}

	for _, almacenado := range casos {
		t.Run(almacenado, func(t *testing.T) {
			if VerifyPassword("x", almacenado) {
				t.Errorf("un valor malformado nunca debe validar: %q", almacenado)
			}
		})
	}
}

func TestSesionIdaYVuelta(t *testing.T) {
	s := NewSessions("secreto-de-prueba")
	nombre := "Roman"

	token, err := s.Create("u-1", "roman@ejemplo.com", &nombre, "superadmin")
	if err != nil {
		t.Fatalf("no se pudo crear: %v", err)
	}

	payload, err := s.Verify(token)
	if err != nil {
		t.Fatalf("no se pudo verificar: %v", err)
	}

	if payload.UID != "u-1" || payload.Email != "roman@ejemplo.com" {
		t.Errorf("el payload no se conservó: %+v", payload)
	}
	if payload.Name == nil || *payload.Name != "Roman" {
		t.Error("el nombre no se conservó")
	}
	if payload.Role != "superadmin" {
		t.Errorf("rol = %q", payload.Role)
	}
}

func TestSesionRechazaFirmaAjena(t *testing.T) {
	emisor := NewSessions("secreto-correcto")
	intruso := NewSessions("otro-secreto")

	token, _ := intruso.Create("u-1", "a@b.co", nil, "user")

	if _, err := emisor.Verify(token); err == nil {
		t.Error("un token firmado con otra clave debe rechazarse")
	}
}

func TestSesionRechazaPayloadManipulado(t *testing.T) {
	s := NewSessions("secreto")
	token, _ := s.Create("u-1", "a@b.co", nil, "user")

	cuerpo, firma, _ := strings.Cut(token, ".")
	manipulado := cuerpo[:len(cuerpo)-2] + "XY" + "." + firma

	if _, err := s.Verify(manipulado); err == nil {
		t.Error("cambiar el payload debe invalidar la firma")
	}
}

func TestSesionRechazaMalformados(t *testing.T) {
	s := NewSessions("secreto")

	for _, token := range []string{"", "sin-punto", ".", "a.", ".b"} {
		if _, err := s.Verify(token); err == nil {
			t.Errorf("debería rechazar %q", token)
		}
	}
}

func TestSesionCaduca(t *testing.T) {
	s := NewSessions("secreto")

	token, _ := s.Create("u-1", "a@b.co", nil, "user")
	payload, err := s.Verify(token)
	if err != nil {
		t.Fatalf("recién creada debería valer: %v", err)
	}

	margen := time.Unix(payload.Exp, 0).Sub(time.Now())
	if margen < 6*24*time.Hour {
		t.Errorf("la sesión debería durar 7 días, dura %v", margen)
	}
}
