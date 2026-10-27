// Package finance lleva las cuentas: movimientos, categorías, presupuestos y
// pagos recurrentes.
package finance

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// Dinero son céntimos, en un entero.
//
// El Lab guardaba los importes como números de coma flotante de JavaScript, y
// con eso 0.1 + 0.2 no da 0.3. En un cuaderno de cuentas ese error se va
// acumulando movimiento a movimiento hasta que los totales dejan de cuadrar
// por unos céntimos. Con enteros no hay nada que redondear: 1050 son diez
// euros con cincuenta, exactamente, siempre.
//
// La base guarda Decimal(14,2), así que la conversión es exacta en los dos
// sentidos y los datos siguen valiendo para el Lab.
type Dinero int64

// ErrImporte se devuelve cuando el texto no es una cantidad.
var ErrImporte = errors.New("importe no válido")

// ParsearDinero lee una cantidad escrita por una persona. Acepta la coma y el
// punto como separador decimal, porque en español se usan las dos.
func ParsearDinero(texto string) (Dinero, error) {
	limpio := strings.ReplaceAll(strings.TrimSpace(texto), ",", ".")
	if limpio == "" {
		return 0, ErrImporte
	}

	f, err := strconv.ParseFloat(limpio, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrImporte
	}

	return DesdeFloat(f), nil
}

// DesdeFloat convierte un número decimal a céntimos.
//
// Se redondea al céntimo más cercano con math.Round: truncar perdería medio
// céntimo en cada conversión y siempre hacia abajo, que es un sesgo.
func DesdeFloat(f float64) Dinero {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return Dinero(math.Round(f * 100))
}

// Float devuelve la cantidad como decimal, para el JSON.
func (d Dinero) Float() float64 {
	return float64(d) / 100
}

// String la escribe con dos decimales y punto, sin símbolo de moneda.
func (d Dinero) String() string {
	negativo := d < 0
	if negativo {
		d = -d
	}

	entero := int64(d) / 100
	centimos := int64(d) % 100

	texto := strconv.FormatInt(entero, 10) + "." +
		strings.Repeat("0", 2-len(strconv.FormatInt(centimos, 10))) +
		strconv.FormatInt(centimos, 10)

	if negativo {
		return "-" + texto
	}
	return texto
}

// Abs devuelve el valor absoluto.
func (d Dinero) Abs() Dinero {
	if d < 0 {
		return -d
	}
	return d
}
