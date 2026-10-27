package users

import (
	"context"
	"errors"
	"testing"
)

// repoFalso devuelve siempre el mismo usuario y apunta qué se le pidió, para
// comprobar que el servicio ni siquiera llega a la base cuando debe cortar.
type repoFalso struct {
	usuario  *User
	llamadas int
	fallo    error
}

func (r *repoFalso) List(context.Context) ([]User, error) {
	r.llamadas++
	if r.fallo != nil {
		return nil, r.fallo
	}
	return []User{*r.usuario}, nil
}

func (r *repoFalso) UpdateStatus(_ context.Context, _ string, s Status) (*User, error) {
	r.llamadas++
	if r.fallo != nil {
		return nil, r.fallo
	}
	if r.usuario == nil {
		return nil, nil
	}
	copia := *r.usuario
	copia.Status = s
	return &copia, nil
}

func (r *repoFalso) UpdateRole(_ context.Context, _ string, rol Role) (*User, error) {
	r.llamadas++
	if r.fallo != nil {
		return nil, r.fallo
	}
	if r.usuario == nil {
		return nil, nil
	}
	copia := *r.usuario
	copia.Role = rol
	return &copia, nil
}

func usuarioDePrueba() *User {
	return &User{ID: "u1", Email: "alguien@ejemplo.com", Role: RoleUser, Status: StatusPending}
}

func TestAuthMethod(t *testing.T) {
	hash := "scrypt$aa$bb"
	vacio := ""

	casos := []struct {
		nombre string
		hash   *string
		quiero string
	}{
		{"con contraseña guardada", &hash, "password"},
		{"sin contraseña: entra con Google", nil, "google"},
		{"hash vacío cuenta como sin contraseña", &vacio, "google"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			u := User{PasswordHash: c.hash}
			if got := u.AuthMethod(); got != c.quiero {
				t.Errorf("AuthMethod() = %q, quería %q", got, c.quiero)
			}
		})
	}
}

// Lo que de verdad importa: un superadmin no puede dejarse fuera a sí mismo.
func TestNoPuedeActuarSobreSuPropiaCuenta(t *testing.T) {
	t.Run("revocarse", func(t *testing.T) {
		repo := &repoFalso{usuario: usuarioDePrueba()}
		svc := NewService(repo)

		res, err := svc.Reject(context.Background(), "u1", "u1")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if res.OK {
			t.Error("dejó que el superadmin se revocara a sí mismo")
		}
		if repo.llamadas != 0 {
			t.Errorf("llegó a la base %d veces; debía cortar antes", repo.llamadas)
		}
	})

	t.Run("degradarse", func(t *testing.T) {
		repo := &repoFalso{usuario: usuarioDePrueba()}
		svc := NewService(repo)

		res, err := svc.SetRole(context.Background(), "u1", RoleUser, "u1")
		if err != nil {
			t.Fatalf("error inesperado: %v", err)
		}
		if res.OK {
			t.Error("dejó que el superadmin se cambiara su propio rol")
		}
		if repo.llamadas != 0 {
			t.Errorf("llegó a la base %d veces; debía cortar antes", repo.llamadas)
		}
	})
}

func TestApruebaYRevocaACualquierOtro(t *testing.T) {
	repo := &repoFalso{usuario: usuarioDePrueba()}
	svc := NewService(repo)

	res, err := svc.Approve(context.Background(), "u1")
	if err != nil || !res.OK {
		t.Fatalf("Approve falló: err=%v res=%+v", err, res)
	}
	if res.Value.Status != StatusApproved {
		t.Errorf("quedó en %q, quería approved", res.Value.Status)
	}

	res, err = svc.Reject(context.Background(), "u1", "otro")
	if err != nil || !res.OK {
		t.Fatalf("Reject falló: err=%v res=%+v", err, res)
	}
	if res.Value.Status != StatusRejected {
		t.Errorf("quedó en %q, quería rejected", res.Value.Status)
	}
}

func TestRolInventadoSeRechaza(t *testing.T) {
	repo := &repoFalso{usuario: usuarioDePrueba()}
	svc := NewService(repo)

	res, err := svc.SetRole(context.Background(), "u1", Role("dios"), "otro")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK {
		t.Error("aceptó un rol que no existe")
	}
	if repo.llamadas != 0 {
		t.Error("llegó a la base con un rol inválido")
	}
}

// Una cuenta borrada mientras tanto es un caso normal, no un error técnico.
func TestCuentaQueYaNoExiste(t *testing.T) {
	repo := &repoFalso{usuario: nil}
	svc := NewService(repo)

	res, err := svc.Approve(context.Background(), "fantasma")
	if err != nil {
		t.Fatalf("debía ser un error de negocio, no técnico: %v", err)
	}
	if res.OK {
		t.Error("dijo que aprobó una cuenta inexistente")
	}
}

// Un fallo de base sí es técnico y debe subir como error.
func TestFalloDeBaseSubeComoError(t *testing.T) {
	repo := &repoFalso{usuario: usuarioDePrueba(), fallo: errors.New("conexión perdida")}
	svc := NewService(repo)

	if _, err := svc.Approve(context.Background(), "u1"); err == nil {
		t.Error("se tragó un fallo de base")
	}
}
