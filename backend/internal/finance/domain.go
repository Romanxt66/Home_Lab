package finance

import (
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ── Cuentas ────────────────────────────────────────────────────────────────

type TipoCuenta string

const (
	Efectivo   TipoCuenta = "cash"
	Banco      TipoCuenta = "bank"
	Tarjeta    TipoCuenta = "card"
	Ahorros    TipoCuenta = "savings"
	OtraCuenta TipoCuenta = "other"
)

var TiposDeCuenta = []TipoCuenta{Efectivo, Banco, Tarjeta, Ahorros, OtraCuenta}

var EtiquetasDeCuenta = map[TipoCuenta]string{
	Efectivo:   "Efectivo",
	Banco:      "Banco",
	Tarjeta:    "Tarjeta",
	Ahorros:    "Ahorros",
	OtraCuenta: "Otra",
}

type Cuenta struct {
	ID             string
	Name           string
	Kind           TipoCuenta
	Currency       string
	InitialBalance Dinero
	Archived       bool
	CreatedAt      time.Time
}

func ValidarCuenta(c Cuenta) kernel.Result[Cuenta] {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return kernel.Err[Cuenta]("La cuenta necesita un nombre.")
	}

	if !tipoDeCuentaValido(c.Kind) {
		return kernel.Err[Cuenta]("Ese tipo de cuenta no existe.")
	}

	moneda := strings.ToUpper(strings.TrimSpace(c.Currency))
	if moneda == "" {
		return kernel.Err[Cuenta]("La moneda es obligatoria.")
	}
	if len(moneda) > 6 {
		moneda = moneda[:6]
	}
	c.Currency = moneda

	return kernel.Ok(c)
}

func tipoDeCuentaValido(t TipoCuenta) bool {
	for _, v := range TiposDeCuenta {
		if v == t {
			return true
		}
	}
	return false
}

// ── Categorías ─────────────────────────────────────────────────────────────

type TipoMovimiento string

const (
	Ingreso TipoMovimiento = "income"
	Gasto   TipoMovimiento = "expense"
)

func tipoDeMovimientoValido(t TipoMovimiento) bool {
	return t == Ingreso || t == Gasto
}

type Categoria struct {
	ID        string
	Name      string
	Kind      TipoMovimiento
	Color     *string
	CreatedAt time.Time
}

func ValidarCategoria(c Categoria) kernel.Result[Categoria] {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return kernel.Err[Categoria]("La categoría necesita un nombre.")
	}
	if !tipoDeMovimientoValido(c.Kind) {
		return kernel.Err[Categoria]("Una categoría es de ingreso o de gasto.")
	}

	if c.Color != nil {
		color := strings.TrimSpace(*c.Color)
		if color == "" {
			c.Color = nil
		} else {
			c.Color = &color
		}
	}

	return kernel.Ok(c)
}

// ── Movimientos ────────────────────────────────────────────────────────────

type Movimiento struct {
	ID         string
	AccountID  string
	CategoryID *string
	Kind       TipoMovimiento
	Amount     Dinero
	OccurredAt time.Time
	Notes      *string
	CreatedAt  time.Time
}

func ValidarMovimiento(m Movimiento) kernel.Result[Movimiento] {
	if strings.TrimSpace(m.AccountID) == "" {
		return kernel.Err[Movimiento]("Elige una cuenta.")
	}
	if !tipoDeMovimientoValido(m.Kind) {
		return kernel.Err[Movimiento]("Un movimiento es un ingreso o un gasto.")
	}

	// El importe va siempre en positivo; el signo lo pone el tipo. Guardar
	// un gasto negativo lo restaría dos veces.
	if m.Amount <= 0 {
		return kernel.Err[Movimiento]("El importe tiene que ser mayor que cero.")
	}

	if m.OccurredAt.IsZero() {
		return kernel.Err[Movimiento]("La fecha no es válida.")
	}

	m.Notes = limpiarOpcional(m.Notes)
	return kernel.Ok(m)
}

// Firmado devuelve el importe con su signo: los gastos restan.
func (m Movimiento) Firmado() Dinero {
	if m.Kind == Ingreso {
		return m.Amount
	}
	return -m.Amount
}

// ── Presupuestos ───────────────────────────────────────────────────────────

type Presupuesto struct {
	ID         string
	CategoryID string
	Amount     Dinero
	CreatedAt  time.Time
}

func ValidarPresupuesto(p Presupuesto) kernel.Result[Presupuesto] {
	if strings.TrimSpace(p.CategoryID) == "" {
		return kernel.Err[Presupuesto]("Elige una categoría.")
	}
	if p.Amount <= 0 {
		return kernel.Err[Presupuesto]("El límite tiene que ser mayor que cero.")
	}
	return kernel.Ok(p)
}

type EstadoPresupuesto string

const (
	PresupuestoOK     EstadoPresupuesto = "ok"
	PresupuestoAviso  EstadoPresupuesto = "warning"
	PresupuestoPasado EstadoPresupuesto = "over"
)

type AvancePresupuesto struct {
	CategoryID string
	Limit      Dinero
	Spent      Dinero
	Remaining  Dinero
	Ratio      float64
	Status     EstadoPresupuesto
}

// Avance compara lo gastado con el límite. Aviso al 80 %, pasado al 100 %.
func Avance(categoriaID string, limite, gastado Dinero) AvancePresupuesto {
	if limite < 0 {
		limite = 0
	}

	ratio := 0.0
	if limite > 0 {
		ratio = float64(gastado) / float64(limite)
	}

	estado := PresupuestoOK
	switch {
	case ratio >= 1:
		estado = PresupuestoPasado
	case ratio >= 0.8:
		estado = PresupuestoAviso
	}

	return AvancePresupuesto{
		CategoryID: categoriaID,
		Limit:      limite,
		Spent:      gastado,
		Remaining:  limite - gastado,
		Ratio:      ratio,
		Status:     estado,
	}
}

// ── Saldos y resúmenes ─────────────────────────────────────────────────────

// Neto suma los movimientos: los ingresos suman, los gastos restan.
func Neto(movimientos []Movimiento) Dinero {
	var total Dinero
	for _, m := range movimientos {
		total += m.Firmado()
	}
	return total
}

// SaldoDeCuenta es el saldo inicial más los movimientos.
func SaldoDeCuenta(c Cuenta, movimientos []Movimiento) Dinero {
	return c.InitialBalance + Neto(movimientos)
}

// PorCategoria agrupa los totales. Los movimientos sin categoría van juntos
// bajo un identificador nulo.
type PorCategoria struct {
	CategoryID *string
	Amount     Dinero
}

func AgruparPorCategoria(movimientos []Movimiento) []PorCategoria {
	totales := map[string]Dinero{}
	const sinCategoria = ""

	// Se recorre dos veces para que el orden de salida sea estable: un mapa
	// de Go no lo garantiza, y una lista que baila en cada recarga es
	// desconcertante.
	var orden []string
	for _, m := range movimientos {
		clave := sinCategoria
		if m.CategoryID != nil {
			clave = *m.CategoryID
		}
		if _, visto := totales[clave]; !visto {
			orden = append(orden, clave)
		}
		totales[clave] += m.Firmado()
	}

	salida := make([]PorCategoria, 0, len(orden))
	for _, clave := range orden {
		fila := PorCategoria{Amount: totales[clave]}
		if clave != sinCategoria {
			id := clave
			fila.CategoryID = &id
		}
		salida = append(salida, fila)
	}

	return salida
}

// ResumenMensual son las cifras de un periodo.
type ResumenMensual struct {
	Income     Dinero
	Expense    Dinero
	Net        Dinero
	ByCategory []PorCategoria
}

func Resumir(movimientos []Movimiento) ResumenMensual {
	var ingresos, gastos Dinero
	for _, m := range movimientos {
		if m.Kind == Ingreso {
			ingresos += m.Amount
		} else {
			gastos += m.Amount
		}
	}

	return ResumenMensual{
		Income:     ingresos,
		Expense:    gastos,
		Net:        ingresos - gastos,
		ByCategory: AgruparPorCategoria(movimientos),
	}
}

func limpiarOpcional(s *string) *string {
	if s == nil {
		return nil
	}
	limpio := strings.TrimSpace(*s)
	if limpio == "" {
		return nil
	}
	return &limpio
}
