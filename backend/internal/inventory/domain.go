// Package inventory lleva la cuenta de qué hay y dónde está: ubicaciones
// anidadas, artículos y el histórico de movimientos de cada uno.
package inventory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ── Ubicaciones ────────────────────────────────────────────────────────────

type LocationKind string

const (
	KindRoom  LocationKind = "room"
	KindBox   LocationKind = "box"
	KindShelf LocationKind = "shelf"
	KindPlace LocationKind = "place"
)

var EtiquetasUbicacion = map[LocationKind]string{
	KindRoom:  "Habitación",
	KindBox:   "Caja",
	KindShelf: "Estante",
	KindPlace: "Lugar",
}

func kindValido(k LocationKind) bool {
	_, ok := EtiquetasUbicacion[k]
	return ok
}

type Location struct {
	ID        string
	Name      string
	Kind      LocationKind
	ParentID  *string
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LocationInput struct {
	Name     string
	Kind     LocationKind
	ParentID *string
	Notes    *string
}

func ValidateLocation(in LocationInput) kernel.Result[LocationInput] {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return kernel.Err[LocationInput]("El nombre no puede estar vacío.")
	}
	if !kindValido(in.Kind) {
		return kernel.Err[LocationInput](
			fmt.Sprintf("Tipo de ubicación desconocido: %s", in.Kind))
	}
	in.Notes = limpiarOpcional(in.Notes)
	return kernel.Ok(in)
}

// LocationNode es una ubicación con sus hijas, para pintar el árbol.
type LocationNode struct {
	Location
	Children  []*LocationNode
	ItemCount int
}

// BuildTree arma el árbol a partir de la lista plana. Una ubicación cuya madre
// no existe pasa a ser raíz, así un dato inconsistente no hace desaparecer
// ramas enteras de la pantalla.
func BuildTree(locations []Location, counts map[string]int) []*LocationNode {
	nodos := make(map[string]*LocationNode, len(locations))
	for _, l := range locations {
		nodos[l.ID] = &LocationNode{Location: l, ItemCount: counts[l.ID]}
	}

	var raices []*LocationNode
	for _, l := range locations {
		nodo := nodos[l.ID]
		// Una ubicación que se declara madre de sí misma sería un ciclo:
		// se trata como raíz.
		if l.ParentID != nil && *l.ParentID != l.ID {
			if madre, existe := nodos[*l.ParentID]; existe {
				madre.Children = append(madre.Children, nodo)
				continue
			}
		}
		raices = append(raices, nodo)
	}

	ordenarRama(raices)
	return raices
}

func ordenarRama(lista []*LocationNode) {
	sort.Slice(lista, func(i, j int) bool {
		return strings.ToLower(lista[i].Name) < strings.ToLower(lista[j].Name)
	})
	for _, n := range lista {
		ordenarRama(n.Children)
	}
}

// DescendantIDs devuelve los identificadores de una ubicación y toda su
// descendencia. Sirve para impedir que alguien meta una ubicación dentro de
// sí misma, que dejaría una rama huérfana imposible de recuperar.
func DescendantIDs(raiz string, locations []Location) map[string]bool {
	hijas := make(map[string][]string)
	for _, l := range locations {
		if l.ParentID != nil {
			hijas[*l.ParentID] = append(hijas[*l.ParentID], l.ID)
		}
	}

	resultado := map[string]bool{raiz: true}
	pendientes := []string{raiz}
	for len(pendientes) > 0 {
		actual := pendientes[0]
		pendientes = pendientes[1:]
		for _, hija := range hijas[actual] {
			if !resultado[hija] {
				resultado[hija] = true
				pendientes = append(pendientes, hija)
			}
		}
	}
	return resultado
}

// ── Artículos ──────────────────────────────────────────────────────────────

type Item struct {
	ID         string
	Name       string
	Category   *string
	Quantity   int
	Unit       *string
	LocationID *string
	Notes      *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ItemInput struct {
	Name     string
	Category *string
	// Cantidad inicial al crear; al editar no se toca, para eso están los
	// movimientos, que dejan rastro de por qué cambió.
	Quantity   int
	Unit       *string
	LocationID *string
	Notes      *string
}

func ValidateItem(in ItemInput) kernel.Result[ItemInput] {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return kernel.Err[ItemInput]("El nombre no puede estar vacío.")
	}
	if in.Quantity < 0 {
		return kernel.Err[ItemInput]("La cantidad no puede ser negativa.")
	}
	in.Category = limpiarOpcional(in.Category)
	in.Unit = limpiarOpcional(in.Unit)
	in.Notes = limpiarOpcional(in.Notes)
	return kernel.Ok(in)
}

// ── Movimientos ────────────────────────────────────────────────────────────

type MovementKind string

const (
	MovBought   MovementKind = "bought"
	MovSold     MovementKind = "sold"
	MovGaveAway MovementKind = "gave_away"
	MovReceived MovementKind = "received"
	MovMoved    MovementKind = "moved"
	MovAdjust   MovementKind = "adjust"
)

var EtiquetasMovimiento = map[MovementKind]string{
	MovBought:   "Comprado",
	MovSold:     "Vendido",
	MovGaveAway: "Regalado",
	MovReceived: "Recibido",
	MovMoved:    "Movido",
	MovAdjust:   "Ajuste",
}

// signo es lo que cada tipo de movimiento hace con la cantidad.
// "moved" cambia de sitio sin alterar el total; "adjust" trae su propio signo
// porque es una corrección manual.
var signo = map[MovementKind]int{
	MovBought:   1,
	MovReceived: 1,
	MovSold:     -1,
	MovGaveAway: -1,
	MovMoved:    0,
	MovAdjust:   1,
}

type Movement struct {
	ID           string
	ItemID       string
	Kind         MovementKind
	Delta        int
	ToLocationID *string
	Notes        *string
	OccurredAt   time.Time
	CreatedAt    time.Time
}

type MovementInput struct {
	ItemID string
	Kind   MovementKind
	// Lo que escribió el usuario. Siempre positivo salvo en "adjust",
	// donde puede venir con signo.
	Quantity     int
	ToLocationID *string
	Notes        *string
	OccurredAt   time.Time
}

// MovementDelta traduce el tipo y la cantidad a la variación con signo.
func MovementDelta(kind MovementKind, quantity int) int {
	switch kind {
	case MovMoved:
		return 0
	case MovAdjust:
		return quantity // puede ser negativo
	default:
		if quantity < 0 {
			quantity = -quantity
		}
		return signo[kind] * quantity
	}
}

// ValidatedMovement es un movimiento ya comprobado, listo para guardar.
type ValidatedMovement struct {
	ItemID       string
	Kind         MovementKind
	Delta        int
	ToLocationID *string
	Notes        *string
	OccurredAt   time.Time
}

// ValidateMovement comprueba el movimiento contra la cantidad actual.
// Rechaza lo que dejaría el stock en negativo, salvo "adjust", que es
// justamente la salida para corregir a mano cuando la cuenta no cuadra.
func ValidateMovement(in MovementInput, cantidadActual int) kernel.Result[ValidatedMovement] {
	if in.ItemID == "" {
		return kernel.Err[ValidatedMovement]("Falta el artículo.")
	}
	if _, ok := EtiquetasMovimiento[in.Kind]; !ok {
		return kernel.Err[ValidatedMovement](
			fmt.Sprintf("Tipo de movimiento desconocido: %s", in.Kind))
	}
	if in.Kind == MovMoved && (in.ToLocationID == nil || *in.ToLocationID == "") {
		return kernel.Err[ValidatedMovement](
			"Elige la ubicación de destino para un movimiento.")
	}
	if in.Kind != MovAdjust && in.Kind != MovMoved && in.Quantity <= 0 {
		return kernel.Err[ValidatedMovement]("La cantidad debe ser mayor que 0.")
	}

	delta := MovementDelta(in.Kind, in.Quantity)
	if cantidadActual+delta < 0 {
		falta := delta
		if falta < 0 {
			falta = -falta
		}
		return kernel.Err[ValidatedMovement](fmt.Sprintf(
			"No hay stock suficiente: quedan %d y el movimiento resta %d.",
			cantidadActual, falta))
	}

	destino := in.ToLocationID
	if in.Kind != MovMoved {
		destino = nil
	}

	return kernel.Ok(ValidatedMovement{
		ItemID:       in.ItemID,
		Kind:         in.Kind,
		Delta:        delta,
		ToLocationID: destino,
		Notes:        limpiarOpcional(in.Notes),
		OccurredAt:   in.OccurredAt,
	})
}

// limpiarOpcional quita espacios y convierte la cadena vacía en nulo, para no
// guardar "" donde lo correcto es NULL.
func limpiarOpcional(v *string) *string {
	if v == nil {
		return nil
	}
	limpio := strings.TrimSpace(*v)
	if limpio == "" {
		return nil
	}
	return &limpio
}
