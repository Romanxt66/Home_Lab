package finance

import (
	"context"
	"errors"
	"testing"
	"time"
)

func punt[T any](v T) *T { return &v }

// ── Dinero ─────────────────────────────────────────────────────────────────

// La razón de ser del tipo: con coma flotante esto no cuadra.
func TestSumarCentimosNoAcumulaError(t *testing.T) {
	var total Dinero
	for i := 0; i < 1000; i++ {
		total += DesdeFloat(0.1)
	}

	if total != Dinero(10000) {
		t.Errorf("mil veces diez céntimos dieron %s, quería 100.00", total)
	}

	// En coma flotante, el mismo bucle se desvía.
	var flotante float64
	for i := 0; i < 1000; i++ {
		flotante += 0.1
	}
	if flotante == 100.0 {
		t.Log("nota: en esta máquina la suma flotante sí cuadró; el entero cuadra siempre")
	}
}

func TestParsearDinero(t *testing.T) {
	casos := map[string]Dinero{
		"10":      1000,
		"10.50":   1050,
		"10,50":   1050, // coma decimal, como se escribe en español
		"0.01":    1,
		"-5.25":   -525,
		"  3.3  ": 330,
		"1000000": 100000000,
	}
	for dado, quiero := range casos {
		got, err := ParsearDinero(dado)
		if err != nil {
			t.Errorf("ParsearDinero(%q) falló: %v", dado, err)
			continue
		}
		if got != quiero {
			t.Errorf("ParsearDinero(%q) = %d, quería %d", dado, got, quiero)
		}
	}

	for _, malo := range []string{"", "   ", "diez", "1.2.3", "€5"} {
		if _, err := ParsearDinero(malo); err == nil {
			t.Errorf("aceptó un importe inválido: %q", malo)
		}
	}
}

// Truncar perdería medio céntimo en cada conversión, y siempre hacia abajo.
func TestDesdeFloatRedondeaAlCentimoMasCercano(t *testing.T) {
	casos := map[float64]Dinero{
		10.555: 1056,
		10.554: 1055,
		0.005:  1,
		0.004:  0,
		-1.005: -100, // math.Round aleja del cero en el .5
	}
	for dado, quiero := range casos {
		if got := DesdeFloat(dado); got != quiero {
			t.Errorf("DesdeFloat(%v) = %d, quería %d", dado, got, quiero)
		}
	}
}

func TestDineroString(t *testing.T) {
	casos := map[Dinero]string{
		1050: "10.50",
		1000: "10.00",
		5:    "0.05",
		0:    "0.00",
		-525: "-5.25",
	}
	for dado, quiero := range casos {
		if got := dado.String(); got != quiero {
			t.Errorf("Dinero(%d).String() = %q, quería %q", dado, got, quiero)
		}
	}
}

// ── Validaciones ───────────────────────────────────────────────────────────

func TestValidarMovimiento(t *testing.T) {
	ahora := time.Now()
	bueno := Movimiento{AccountID: "c1", Kind: Gasto, Amount: 1050, OccurredAt: ahora}

	if res := ValidarMovimiento(bueno); !res.OK {
		t.Errorf("rechazó un movimiento correcto: %s", res.Error)
	}

	casos := map[string]Movimiento{
		"sin cuenta":       {Kind: Gasto, Amount: 100, OccurredAt: ahora},
		"tipo inventado":   {AccountID: "c1", Kind: TipoMovimiento("regalo"), Amount: 100, OccurredAt: ahora},
		"importe cero":     {AccountID: "c1", Kind: Gasto, Amount: 0, OccurredAt: ahora},
		"importe negativo": {AccountID: "c1", Kind: Gasto, Amount: -100, OccurredAt: ahora},
		"sin fecha":        {AccountID: "c1", Kind: Gasto, Amount: 100},
	}
	for nombre, m := range casos {
		if ValidarMovimiento(m).OK {
			t.Errorf("%s: lo dio por bueno", nombre)
		}
	}
}

// El signo lo pone el tipo, no el importe: un gasto guardado en negativo se
// restaría dos veces.
func TestFirmado(t *testing.T) {
	ingreso := Movimiento{Kind: Ingreso, Amount: 1000}
	gasto := Movimiento{Kind: Gasto, Amount: 1000}

	if ingreso.Firmado() != 1000 {
		t.Errorf("ingreso firmado = %d", ingreso.Firmado())
	}
	if gasto.Firmado() != -1000 {
		t.Errorf("gasto firmado = %d", gasto.Firmado())
	}
}

func TestValidarCuenta(t *testing.T) {
	res := ValidarCuenta(Cuenta{Name: "  Banco  ", Kind: Banco, Currency: "  eur  "})
	if !res.OK {
		t.Fatalf("rechazó una cuenta correcta: %s", res.Error)
	}
	if res.Value.Name != "Banco" {
		t.Errorf("no recortó el nombre: %q", res.Value.Name)
	}
	if res.Value.Currency != "EUR" {
		t.Errorf("no normalizó la moneda: %q", res.Value.Currency)
	}

	for nombre, c := range map[string]Cuenta{
		"sin nombre":     {Kind: Banco, Currency: "EUR"},
		"tipo inventado": {Name: "A", Kind: TipoCuenta("cripto"), Currency: "EUR"},
		"sin moneda":     {Name: "A", Kind: Banco},
	} {
		if ValidarCuenta(c).OK {
			t.Errorf("%s: la dio por buena", nombre)
		}
	}
}

// ── Saldos ─────────────────────────────────────────────────────────────────

func TestSaldoDeCuenta(t *testing.T) {
	cuenta := Cuenta{InitialBalance: 10000} // 100.00
	movimientos := []Movimiento{
		{Kind: Ingreso, Amount: 5050}, // +50.50
		{Kind: Gasto, Amount: 2025},   // -20.25
		{Kind: Gasto, Amount: 1},      // -0.01
	}

	if got := SaldoDeCuenta(cuenta, movimientos); got != 13024 {
		t.Errorf("saldo = %s, quería 130.24", got)
	}
}

func TestResumir(t *testing.T) {
	cat := "c1"
	movimientos := []Movimiento{
		{Kind: Ingreso, Amount: 10000, CategoryID: &cat},
		{Kind: Gasto, Amount: 3000, CategoryID: &cat},
		{Kind: Gasto, Amount: 2000}, // sin categoría
	}

	r := Resumir(movimientos)
	if r.Income != 10000 {
		t.Errorf("ingresos = %s", r.Income)
	}
	if r.Expense != 5000 {
		t.Errorf("gastos = %s", r.Expense)
	}
	if r.Net != 5000 {
		t.Errorf("neto = %s", r.Net)
	}
	if len(r.ByCategory) != 2 {
		t.Fatalf("agrupó en %d categorías, quería 2", len(r.ByCategory))
	}
}

// Un mapa de Go no garantiza el orden; una lista que baila en cada recarga
// es desconcertante.
func TestAgruparPorCategoriaMantieneElOrden(t *testing.T) {
	a, b := "a", "b"
	movimientos := []Movimiento{
		{Kind: Gasto, Amount: 100, CategoryID: &a},
		{Kind: Gasto, Amount: 100, CategoryID: &b},
		{Kind: Gasto, Amount: 100, CategoryID: &a},
	}

	for i := 0; i < 20; i++ {
		filas := AgruparPorCategoria(movimientos)
		if len(filas) != 2 || *filas[0].CategoryID != "a" || *filas[1].CategoryID != "b" {
			t.Fatalf("el orden cambió en la pasada %d: %+v", i, filas)
		}
	}
}

// ── Presupuestos ───────────────────────────────────────────────────────────

func TestAvance(t *testing.T) {
	casos := []struct {
		nombre  string
		limite  Dinero
		gastado Dinero
		estado  EstadoPresupuesto
	}{
		{"muy por debajo", 10000, 1000, PresupuestoOK},
		{"justo antes del aviso", 10000, 7999, PresupuestoOK},
		{"en el 80 por ciento", 10000, 8000, PresupuestoAviso},
		{"justo antes de pasarse", 10000, 9999, PresupuestoAviso},
		{"clavado en el límite", 10000, 10000, PresupuestoPasado},
		{"pasado", 10000, 12000, PresupuestoPasado},
		{"límite cero", 0, 100, PresupuestoOK},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			a := Avance("cat", c.limite, c.gastado)
			if a.Status != c.estado {
				t.Errorf("estado = %q, quería %q (ratio %.2f)", a.Status, c.estado, a.Ratio)
			}
			if a.Remaining != c.limite-c.gastado {
				t.Errorf("restante = %s", a.Remaining)
			}
		})
	}
}

// ── Pagos recurrentes ──────────────────────────────────────────────────────

func TestAvanzar(t *testing.T) {
	base := time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local)

	if got := Avanzar(base, Semanal); got.Day() != 22 {
		t.Errorf("semanal = %v", got)
	}
	if got := Avanzar(base, Mensual); got.Month() != time.February || got.Day() != 15 {
		t.Errorf("mensual = %v", got)
	}
	if got := Avanzar(base, Anual); got.Year() != 2026 {
		t.Errorf("anual = %v", got)
	}
}

// Si el servidor estuvo caído una semana, al volver hay que crear los cargos
// que se perdieron, no saltárselos.
func TestVencidosRecuperaLoQueSePerdio(t *testing.T) {
	desde := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	ahora := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)

	ocurrencias, siguiente := Vencidos(desde, Mensual, ahora)

	if len(ocurrencias) != 3 { // enero, febrero, marzo
		t.Errorf("salieron %d cargos, quería 3: %v", len(ocurrencias), ocurrencias)
	}
	if !siguiente.After(ahora) {
		t.Errorf("la fecha siguiente (%v) no quedó por delante de ahora", siguiente)
	}
}

// Sin tope, una fecha de hace años dejaría el bucle corriendo sin fin.
func TestVencidosTieneTope(t *testing.T) {
	desde := time.Date(1990, 1, 1, 0, 0, 0, 0, time.Local)
	ocurrencias, _ := Vencidos(desde, Semanal, time.Now())

	if len(ocurrencias) != MaxRecuperacion {
		t.Errorf("generó %d cargos, el tope son %d", len(ocurrencias), MaxRecuperacion)
	}
}

func TestVencidosSinNadaPendiente(t *testing.T) {
	futuro := time.Now().AddDate(0, 1, 0)
	ocurrencias, siguiente := Vencidos(futuro, Mensual, time.Now())

	if len(ocurrencias) != 0 {
		t.Errorf("generó cargos de una fecha futura: %v", ocurrencias)
	}
	if !siguiente.Equal(futuro) {
		t.Errorf("movió la fecha sin haber nada pendiente: %v", siguiente)
	}
}

func TestTocaAvisar(t *testing.T) {
	ahora := time.Date(2025, 3, 10, 12, 0, 0, 0, time.Local)
	cargo := time.Date(2025, 3, 13, 0, 0, 0, 0, time.Local)

	casos := []struct {
		nombre string
		pago   Recurrente
		quiero bool
	}{
		{"dentro de la ventana", Recurrente{Active: true, NextRunAt: cargo, RemindDaysBefore: punt(5)}, true},
		{"todavía lejos", Recurrente{Active: true, NextRunAt: cargo, RemindDaysBefore: punt(1)}, false},
		{"sin aviso configurado", Recurrente{Active: true, NextRunAt: cargo}, false},
		{"ya avisado", Recurrente{Active: true, NextRunAt: cargo, RemindDaysBefore: punt(5), ReminderSentAt: &ahora}, false},
		{"pago desactivado", Recurrente{Active: false, NextRunAt: cargo, RemindDaysBefore: punt(5)}, false},
		{
			"el cargo ya pasó: avisar de que viene no tiene sentido",
			Recurrente{Active: true, NextRunAt: ahora.Add(-time.Hour), RemindDaysBefore: punt(5)},
			false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := TocaAvisar(c.pago, ahora); got != c.quiero {
				t.Errorf("TocaAvisar = %v, quería %v", got, c.quiero)
			}
		})
	}
}

// ── El procesador ──────────────────────────────────────────────────────────

type recurrentesFalsos struct {
	pagos      []Recurrente
	procesados map[string]time.Time
	avisados   []string
}

func (r *recurrentesFalsos) List(context.Context, bool) ([]Recurrente, error) {
	return r.pagos, nil
}
func (r *recurrentesFalsos) Create(_ context.Context, p Recurrente) (*Recurrente, error) {
	return &p, nil
}
func (r *recurrentesFalsos) Update(_ context.Context, p Recurrente) (*Recurrente, error) {
	return &p, nil
}
func (r *recurrentesFalsos) Remove(context.Context, string) error { return nil }
func (r *recurrentesFalsos) MarcarProcesado(_ context.Context, id string, siguiente time.Time) error {
	if r.procesados == nil {
		r.procesados = map[string]time.Time{}
	}
	r.procesados[id] = siguiente
	return nil
}
func (r *recurrentesFalsos) MarcarAvisado(_ context.Context, id string, _ time.Time) error {
	r.avisados = append(r.avisados, id)
	return nil
}

type movimientosFalsos struct {
	creados []Movimiento
	fallo   error
}

func (m *movimientosFalsos) List(context.Context, FiltroMovimientos) ([]Movimiento, error) {
	return nil, nil
}
func (m *movimientosFalsos) Create(_ context.Context, mov Movimiento) (*Movimiento, error) {
	if m.fallo != nil {
		return nil, m.fallo
	}
	m.creados = append(m.creados, mov)
	return &mov, nil
}
func (m *movimientosFalsos) Update(_ context.Context, mov Movimiento) (*Movimiento, error) {
	return &mov, nil
}
func (m *movimientosFalsos) Remove(context.Context, string) error { return nil }
func (m *movimientosFalsos) GastadoEnCategoria(context.Context, string, time.Time, time.Time) (Dinero, error) {
	return 0, nil
}

type cuentasFalsas struct{}

func (cuentasFalsas) List(context.Context, bool) ([]Cuenta, error) { return nil, nil }
func (cuentasFalsas) ConSaldos(context.Context, bool) ([]CuentaConSaldo, error) {
	return nil, nil
}
func (cuentasFalsas) Get(_ context.Context, id string) (*Cuenta, error) {
	return &Cuenta{ID: id, Currency: "EUR"}, nil
}
func (cuentasFalsas) Create(_ context.Context, c Cuenta) (*Cuenta, error) { return &c, nil }
func (cuentasFalsas) Update(_ context.Context, c Cuenta) (*Cuenta, error) { return &c, nil }
func (cuentasFalsas) Remove(context.Context, string) error                { return nil }

type avisadorFalso struct {
	mensajes []string
	fallo    error
}

func (a *avisadorFalso) Send(_ context.Context, m string) error {
	a.mensajes = append(a.mensajes, m)
	return a.fallo
}

func servicioDePrueba(recs *recurrentesFalsos, movs *movimientosFalsos, avisos *avisadorFalso) *Service {
	return NewService(cuentasFalsas{}, nil, movs, nil, recs, avisos, nil)
}

func TestProcesarRecurrentesCreaLosCargosVencidos(t *testing.T) {
	ahora := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	recs := &recurrentesFalsos{pagos: []Recurrente{{
		ID: "p1", Name: "Alquiler", AccountID: "c1", Kind: Gasto, Amount: 80000,
		Frequency: Mensual, NextRunAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
		Active: true,
	}}}
	movs := &movimientosFalsos{}

	resumen, err := servicioDePrueba(recs, movs, &avisadorFalso{}).
		ProcesarRecurrentes(context.Background(), ahora)
	if err != nil {
		t.Fatalf("falló: %v", err)
	}

	if resumen.Creados != 3 {
		t.Errorf("creó %d cargos, quería 3", resumen.Creados)
	}
	if len(movs.creados) != 3 {
		t.Fatalf("movimientos creados = %d", len(movs.creados))
	}
	if movs.creados[0].Amount != 80000 || movs.creados[0].Kind != Gasto {
		t.Errorf("el movimiento no copió el pago: %+v", movs.creados[0])
	}
	if _, avanzado := recs.procesados["p1"]; !avanzado {
		t.Error("no movió la fecha del próximo cargo")
	}
}

// Si falla crear un movimiento, la fecha NO debe avanzar: si avanzara, los
// cargos que faltaban se perderían para siempre.
func TestSiFallaUnCargoLaFechaNoAvanza(t *testing.T) {
	ahora := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	recs := &recurrentesFalsos{pagos: []Recurrente{{
		ID: "p1", Name: "Alquiler", AccountID: "c1", Kind: Gasto, Amount: 100,
		Frequency: Mensual, NextRunAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local),
		Active: true,
	}}}
	movs := &movimientosFalsos{fallo: errors.New("sin conexión")}

	resumen, err := servicioDePrueba(recs, movs, &avisadorFalso{}).
		ProcesarRecurrentes(context.Background(), ahora)
	if err != nil {
		t.Fatalf("falló: %v", err)
	}

	if len(resumen.Errores) == 0 {
		t.Error("no anotó el fallo")
	}
	if _, avanzado := recs.procesados["p1"]; avanzado {
		t.Error("movió la fecha pese a no haber creado los cargos")
	}
}

func TestElAvisoSaleAntesDeMoverLaFecha(t *testing.T) {
	ahora := time.Date(2025, 3, 10, 12, 0, 0, 0, time.Local)
	recs := &recurrentesFalsos{pagos: []Recurrente{{
		ID: "p1", Name: "Netflix", AccountID: "c1", Kind: Gasto, Amount: 1299,
		Frequency: Mensual, NextRunAt: time.Date(2025, 3, 13, 0, 0, 0, 0, time.Local),
		RemindDaysBefore: punt(5), Active: true,
	}}}
	avisos := &avisadorFalso{}

	resumen, _ := servicioDePrueba(recs, &movimientosFalsos{}, avisos).
		ProcesarRecurrentes(context.Background(), ahora)

	if resumen.Avisados != 1 {
		t.Errorf("mandó %d avisos", resumen.Avisados)
	}
	if len(recs.avisados) != 1 {
		t.Error("no marcó el aviso como enviado")
	}
	if resumen.Creados != 0 {
		t.Error("creó un cargo que todavía no tocaba")
	}
}

// Si el aviso falla no se marca, para reintentarlo en la pasada siguiente.
func TestUnAvisoQueFallaNoSeMarca(t *testing.T) {
	ahora := time.Date(2025, 3, 10, 12, 0, 0, 0, time.Local)
	recs := &recurrentesFalsos{pagos: []Recurrente{{
		ID: "p1", Name: "Netflix", AccountID: "c1", Kind: Gasto, Amount: 100,
		Frequency: Mensual, NextRunAt: time.Date(2025, 3, 13, 0, 0, 0, 0, time.Local),
		RemindDaysBefore: punt(5), Active: true,
	}}}
	avisos := &avisadorFalso{fallo: errors.New("telegram caído")}

	resumen, _ := servicioDePrueba(recs, &movimientosFalsos{}, avisos).
		ProcesarRecurrentes(context.Background(), ahora)

	if resumen.Avisados != 0 {
		t.Error("contó como enviado un aviso que falló")
	}
	if len(recs.avisados) != 0 {
		t.Error("marcó un aviso que no se pudo enviar")
	}
	if len(resumen.Errores) == 0 {
		t.Error("no anotó el fallo")
	}
}
