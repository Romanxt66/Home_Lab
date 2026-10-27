package users

import (
	"context"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Repo es lo que el servicio necesita de la base de datos.
type Repo interface {
	List(ctx context.Context) ([]User, error)
	UpdateStatus(ctx context.Context, id string, status Status) (*User, error)
	UpdateRole(ctx context.Context, id string, role Role) (*User, error)
}

// Service administra las cuentas.
//
// Todas las operaciones que cambian algo comprueban que el superadmin no se
// la esté aplicando a sí mismo: si pudiera revocarse o degradarse, se
// quedaría fuera y no habría quien lo deshiciera.
type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]User, error) {
	return s.repo.List(ctx)
}

// Approve admite un registro pendiente, o devuelve el acceso a quien se le
// había revocado.
func (s *Service) Approve(ctx context.Context, id string) (kernel.Result[User], error) {
	u, err := s.repo.UpdateStatus(ctx, id, StatusApproved)
	if err != nil {
		return kernel.Result[User]{}, err
	}
	if u == nil {
		return kernel.Err[User]("Esa cuenta ya no existe."), nil
	}
	return kernel.Ok(*u), nil
}

// Reject rechaza un registro pendiente o revoca el acceso de una cuenta ya
// aprobada.
func (s *Service) Reject(ctx context.Context, id, yo string) (kernel.Result[User], error) {
	if id == yo {
		return kernel.Err[User]("No puedes rechazar o revocar tu propia cuenta."), nil
	}

	u, err := s.repo.UpdateStatus(ctx, id, StatusRejected)
	if err != nil {
		return kernel.Result[User]{}, err
	}
	if u == nil {
		return kernel.Err[User]("Esa cuenta ya no existe."), nil
	}
	return kernel.Ok(*u), nil
}

func (s *Service) SetRole(ctx context.Context, id string, role Role, yo string) (kernel.Result[User], error) {
	if id == yo {
		return kernel.Err[User]("No puedes cambiar tu propio rol."), nil
	}
	if !RoleValido(role) {
		return kernel.Err[User]("Ese rol no existe."), nil
	}

	u, err := s.repo.UpdateRole(ctx, id, role)
	if err != nil {
		return kernel.Result[User]{}, err
	}
	if u == nil {
		return kernel.Err[User]("Esa cuenta ya no existe."), nil
	}
	return kernel.Ok(*u), nil
}
