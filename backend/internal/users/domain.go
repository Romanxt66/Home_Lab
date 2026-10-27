// Package users es la administración de cuentas: aprobar registros, revocar
// accesos y repartir roles. Solo lo toca un superadmin.
package users

import "time"

// Role distingue al administrador del resto. No hay más niveles.
type Role string

const (
	RoleUser       Role = "user"
	RoleSuperadmin Role = "superadmin"
)

// Status es el ciclo de vida de una cuenta. Quien se registra queda en
// pending hasta que un superadmin decide.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// User es la cuenta tal y como vive en la tabla usuarioslab.
type User struct {
	ID      string
	Email   string
	Name    *string
	Picture *string
	// Nulo en las cuentas que solo entran con Google.
	PasswordHash *string
	Role         Role
	Status       Status
	CreatedAt    time.Time
}

// AuthMethod deduce cómo entra la cuenta a partir de si guarda contraseña.
func (u User) AuthMethod() string {
	if u.PasswordHash != nil && *u.PasswordHash != "" {
		return "password"
	}
	return "google"
}

// RoleValido evita que llegue a la base un rol inventado.
func RoleValido(r Role) bool {
	return r == RoleUser || r == RoleSuperadmin
}
