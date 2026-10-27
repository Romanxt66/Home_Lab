package inventory

import (
	"context"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

type LocationRepo interface {
	List(ctx context.Context) ([]Location, error)
	Get(ctx context.Context, id string) (*Location, error)
	Create(ctx context.Context, in LocationInput) (*Location, error)
	Update(ctx context.Context, id string, in LocationInput) (*Location, error)
	Remove(ctx context.Context, id string) error
	ItemCounts(ctx context.Context) (map[string]int, error)
}

type ItemRepo interface {
	List(ctx context.Context, locationID string) ([]Item, error)
	Get(ctx context.Context, id string) (*Item, error)
	Create(ctx context.Context, in ItemInput) (*Item, error)
	Update(ctx context.Context, id string, in ItemInput) (*Item, error)
	Remove(ctx context.Context, id string) error
	Categories(ctx context.Context) ([]string, error)
}

type MovementRepo interface {
	// Record guarda el movimiento y ajusta la cantidad del artículo en una
	// sola transacción: si una de las dos fallara, la otra no debe quedar.
	Record(ctx context.Context, m ValidatedMovement) (*Movement, error)
	ListForItem(ctx context.Context, itemID string, limit int) ([]Movement, error)
	ListRecent(ctx context.Context, limit int) ([]Movement, error)
}

type Service struct {
	locations LocationRepo
	items     ItemRepo
	movements MovementRepo
}

func NewService(l LocationRepo, i ItemRepo, m MovementRepo) *Service {
	return &Service{locations: l, items: i, movements: m}
}

// ── Ubicaciones ────────────────────────────────────────────────────────────

func (s *Service) ListLocations(ctx context.Context) ([]Location, error) {
	return s.locations.List(ctx)
}

// LocationTree devuelve el árbol con el número de artículos de cada ubicación.
func (s *Service) LocationTree(ctx context.Context) ([]*LocationNode, error) {
	locations, err := s.locations.List(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.locations.ItemCounts(ctx)
	if err != nil {
		return nil, err
	}
	return BuildTree(locations, counts), nil
}

func (s *Service) SaveLocation(ctx context.Context, id string, in LocationInput) (kernel.Result[Location], error) {
	valido := ValidateLocation(in)
	if !valido.OK {
		return kernel.Err[Location](valido.Error), nil
	}

	// Evitar ciclos: una ubicación no puede colgar de sí misma ni de una de
	// sus propias descendientes, o se perdería esa rama del árbol.
	if id != "" && valido.Value.ParentID != nil {
		todas, err := s.locations.List(ctx)
		if err != nil {
			return kernel.Result[Location]{}, err
		}
		if DescendantIDs(id, todas)[*valido.Value.ParentID] {
			return kernel.Err[Location](
				"No puedes mover una ubicación dentro de sí misma o de una sub-ubicación suya."), nil
		}
	}

	var (
		l   *Location
		err error
	)
	if id == "" {
		l, err = s.locations.Create(ctx, valido.Value)
	} else {
		l, err = s.locations.Update(ctx, id, valido.Value)
	}
	if err != nil {
		return kernel.Result[Location]{}, err
	}

	return kernel.Ok(*l), nil
}

func (s *Service) RemoveLocation(ctx context.Context, id string) error {
	return s.locations.Remove(ctx, id)
}

// ── Artículos ──────────────────────────────────────────────────────────────

func (s *Service) ListItems(ctx context.Context, locationID string) ([]Item, error) {
	return s.items.List(ctx, locationID)
}

func (s *Service) GetItem(ctx context.Context, id string) (*Item, error) {
	return s.items.Get(ctx, id)
}

func (s *Service) Categories(ctx context.Context) ([]string, error) {
	return s.items.Categories(ctx)
}

func (s *Service) SaveItem(ctx context.Context, id string, in ItemInput) (kernel.Result[Item], error) {
	valido := ValidateItem(in)
	if !valido.OK {
		return kernel.Err[Item](valido.Error), nil
	}

	var (
		i   *Item
		err error
	)
	if id == "" {
		i, err = s.items.Create(ctx, valido.Value)
	} else {
		i, err = s.items.Update(ctx, id, valido.Value)
	}
	if err != nil {
		return kernel.Result[Item]{}, err
	}

	return kernel.Ok(*i), nil
}

func (s *Service) RemoveItem(ctx context.Context, id string) error {
	return s.items.Remove(ctx, id)
}

// ── Movimientos ────────────────────────────────────────────────────────────

func (s *Service) ListMovements(ctx context.Context, itemID string, limit int) ([]Movement, error) {
	return s.movements.ListForItem(ctx, itemID, limit)
}

func (s *Service) RecentMovements(ctx context.Context, limit int) ([]Movement, error) {
	return s.movements.ListRecent(ctx, limit)
}

// RecordMovement comprueba contra el stock actual y lo aplica.
func (s *Service) RecordMovement(ctx context.Context, in MovementInput) (kernel.Result[Movement], error) {
	item, err := s.items.Get(ctx, in.ItemID)
	if err != nil {
		return kernel.Result[Movement]{}, err
	}
	if item == nil {
		return kernel.Err[Movement]("El artículo ya no existe."), nil
	}

	valido := ValidateMovement(in, item.Quantity)
	if !valido.OK {
		return kernel.Err[Movement](valido.Error), nil
	}

	m, err := s.movements.Record(ctx, valido.Value)
	if err != nil {
		return kernel.Result[Movement]{}, err
	}

	return kernel.Ok(*m), nil
}
