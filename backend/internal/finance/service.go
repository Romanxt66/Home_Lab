package finance

import (
	"context"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ── Puertos ────────────────────────────────────────────────────────────────

type CuentaRepo interface {
	List(ctx context.Context, incluirArchivadas bool) ([]Cuenta, error)
	// ConSaldos devuelve cada cuenta con su saldo ya calculado en la base,
	// en una sola consulta.
	ConSaldos(ctx context.Context, incluirArchivadas bool) ([]CuentaConSaldo, error)
	Get(ctx context.Context, id string) (*Cuenta, error)
	Create(ctx context.Context, c Cuenta) (*Cuenta, error)
	Update(ctx context.Context, c Cuenta) (*Cuenta, error)
	Remove(ctx context.Context, id string) error
}

type CategoriaRepo interface {
	List(ctx context.Context) ([]Categoria, error)
	Get(ctx context.Context, id string) (*Categoria, error)
	Create(ctx context.Context, c Categoria) (*Categoria, error)
	Update(ctx context.Context, c Categoria) (*Categoria, error)
	Remove(ctx context.Context, id string) error
}

// FiltroMovimientos acota qué movimientos se piden.
type FiltroMovimientos struct {
	AccountID string
	Desde     *time.Time
	Hasta     *time.Time
	Limite    int
}

type MovimientoRepo interface {
	List(ctx context.Context, f FiltroMovimientos) ([]Movimiento, error)
	Create(ctx context.Context, m Movimiento) (*Movimiento, error)
	Update(ctx context.Context, m Movimiento) (*Movimiento, error)
	Remove(ctx context.Context, id string) error
	// GastadoEnCategoria suma los gastos de una categoría en un periodo,
	// en la base: traerse todos los movimientos para sumarlos en Go sería
	// mover datos para nada.
	GastadoEnCategoria(ctx context.Context, categoriaID string, desde, hasta time.Time) (Dinero, error)
}

type PresupuestoRepo interface {
	List(ctx context.Context) ([]Presupuesto, error)
	Upsert(ctx context.Context, p Presupuesto) (*Presupuesto, error)
	Remove(ctx context.Context, categoriaID string) error
}

type RecurrenteRepo interface {
	List(ctx context.Context, soloActivos bool) ([]Recurrente, error)
	Create(ctx context.Context, r Recurrente) (*Recurrente, error)
	Update(ctx context.Context, r Recurrente) (*Recurrente, error)
	Remove(ctx context.Context, id string) error
	MarcarProcesado(ctx context.Context, id string, siguiente time.Time) error
	MarcarAvisado(ctx context.Context, id string, cuando time.Time) error
}

// Avisador manda los recordatorios de los pagos recurrentes.
type Avisador interface {
	Send(ctx context.Context, mensaje string) error
}

// Automatizador deja saltar la regla de presupuesto superado.
type Automatizador interface {
	Disparar(ctx context.Context, disparador string, valores map[string]string)
}

// CuentaConSaldo es una cuenta con lo que tiene ahora mismo.
type CuentaConSaldo struct {
	Cuenta  Cuenta
	Balance Dinero
}

// PresupuestoConAvance empareja el límite con lo gastado este mes.
type PresupuestoConAvance struct {
	Presupuesto Presupuesto
	Avance      AvancePresupuesto
}

// ── Servicio ───────────────────────────────────────────────────────────────

type Service struct {
	cuentas       CuentaRepo
	categorias    CategoriaRepo
	movimientos   MovimientoRepo
	presupuestos  PresupuestoRepo
	recurrentes   RecurrenteRepo
	avisador      Avisador
	automatizador Automatizador
}

func NewService(
	cuentas CuentaRepo, categorias CategoriaRepo, movimientos MovimientoRepo,
	presupuestos PresupuestoRepo, recurrentes RecurrenteRepo,
	avisador Avisador, automatizador Automatizador,
) *Service {
	return &Service{
		cuentas: cuentas, categorias: categorias, movimientos: movimientos,
		presupuestos: presupuestos, recurrentes: recurrentes,
		avisador: avisador, automatizador: automatizador,
	}
}

// ── Cuentas ────────────────────────────────────────────────────────────────

func (s *Service) Cuentas(ctx context.Context, incluirArchivadas bool) ([]Cuenta, error) {
	return s.cuentas.List(ctx, incluirArchivadas)
}

// CuentasConSaldo delega el cálculo a la base.
//
// El Lab pedía los movimientos de cada cuenta por separado y los sumaba en
// memoria: con diez cuentas eran once viajes a la base para una pantalla.
func (s *Service) CuentasConSaldo(ctx context.Context, incluirArchivadas bool) ([]CuentaConSaldo, error) {
	return s.cuentas.ConSaldos(ctx, incluirArchivadas)
}

func (s *Service) GuardarCuenta(ctx context.Context, c Cuenta) (kernel.Result[Cuenta], error) {
	v := ValidarCuenta(c)
	if !v.OK {
		return kernel.Err[Cuenta](v.Error), nil
	}

	var guardada *Cuenta
	var err error
	if v.Value.ID == "" {
		guardada, err = s.cuentas.Create(ctx, v.Value)
	} else {
		guardada, err = s.cuentas.Update(ctx, v.Value)
	}
	if err != nil {
		return kernel.Result[Cuenta]{}, err
	}
	if guardada == nil {
		return kernel.Err[Cuenta]("Esa cuenta ya no existe."), nil
	}

	return kernel.Ok(*guardada), nil
}

func (s *Service) BorrarCuenta(ctx context.Context, id string) error {
	return s.cuentas.Remove(ctx, id)
}

// ── Categorías ─────────────────────────────────────────────────────────────

func (s *Service) Categorias(ctx context.Context) ([]Categoria, error) {
	return s.categorias.List(ctx)
}

func (s *Service) GuardarCategoria(ctx context.Context, c Categoria) (kernel.Result[Categoria], error) {
	v := ValidarCategoria(c)
	if !v.OK {
		return kernel.Err[Categoria](v.Error), nil
	}

	var guardada *Categoria
	var err error
	if v.Value.ID == "" {
		guardada, err = s.categorias.Create(ctx, v.Value)
	} else {
		guardada, err = s.categorias.Update(ctx, v.Value)
	}
	if err != nil {
		return kernel.Result[Categoria]{}, err
	}
	if guardada == nil {
		return kernel.Err[Categoria]("Esa categoría ya no existe."), nil
	}

	return kernel.Ok(*guardada), nil
}

func (s *Service) BorrarCategoria(ctx context.Context, id string) error {
	return s.categorias.Remove(ctx, id)
}

// ── Movimientos ────────────────────────────────────────────────────────────

func (s *Service) Movimientos(ctx context.Context, f FiltroMovimientos) ([]Movimiento, error) {
	return s.movimientos.List(ctx, f)
}

func (s *Service) GuardarMovimiento(ctx context.Context, m Movimiento) (kernel.Result[Movimiento], error) {
	v := ValidarMovimiento(m)
	if !v.OK {
		return kernel.Err[Movimiento](v.Error), nil
	}

	var guardado *Movimiento
	var err error
	if v.Value.ID == "" {
		guardado, err = s.movimientos.Create(ctx, v.Value)
	} else {
		guardado, err = s.movimientos.Update(ctx, v.Value)
	}
	if err != nil {
		return kernel.Result[Movimiento]{}, err
	}
	if guardado == nil {
		return kernel.Err[Movimiento]("Ese movimiento ya no existe."), nil
	}

	if guardado.Kind == Gasto && guardado.CategoryID != nil {
		s.revisarPresupuesto(ctx, *guardado.CategoryID, guardado.Amount, guardado.OccurredAt)
	}

	return kernel.Ok(*guardado), nil
}

func (s *Service) BorrarMovimiento(ctx context.Context, id string) error {
	return s.movimientos.Remove(ctx, id)
}

// revisarPresupuesto dispara la automatización justo cuando el gasto cruza el
// límite, no en cada movimiento posterior mientras siga pasado: si no, el mes
// entero sería una sucesión de avisos del mismo problema.
func (s *Service) revisarPresupuesto(ctx context.Context, categoriaID string, acabaDeGastar Dinero, cuando time.Time) {
	if s.automatizador == nil {
		return
	}

	presupuestos, err := s.presupuestos.List(ctx)
	if err != nil {
		return
	}

	var limite Dinero
	var hay bool
	for _, p := range presupuestos {
		if p.CategoryID == categoriaID {
			limite, hay = p.Amount, true
			break
		}
	}
	if !hay {
		return
	}

	desde := time.Date(cuando.Year(), cuando.Month(), 1, 0, 0, 0, 0, cuando.Location())
	hasta := desde.AddDate(0, 1, 0)

	despues, err := s.movimientos.GastadoEnCategoria(ctx, categoriaID, desde, hasta)
	if err != nil {
		return
	}
	antes := despues - acabaDeGastar

	yaEstaba := Avance(categoriaID, limite, antes).Status == PresupuestoPasado
	ahoraSi := Avance(categoriaID, limite, despues).Status == PresupuestoPasado
	if yaEstaba || !ahoraSi {
		return
	}

	nombre := "Sin categoría"
	if cat, err := s.categorias.Get(ctx, categoriaID); err == nil && cat != nil {
		nombre = cat.Name
	}

	s.automatizador.Disparar(ctx, "budget_exceeded", map[string]string{
		"categoria": nombre,
		"limite":    limite.String(),
		"gastado":   despues.String(),
	})
}

// ── Resumen ────────────────────────────────────────────────────────────────

func (s *Service) ResumenMensual(ctx context.Context, año, mes int) (ResumenMensual, error) {
	if mes < 1 || mes > 12 {
		return ResumenMensual{}, nil
	}

	desde := time.Date(año, time.Month(mes), 1, 0, 0, 0, 0, time.Local)
	hasta := desde.AddDate(0, 1, 0)

	movimientos, err := s.movimientos.List(ctx, FiltroMovimientos{Desde: &desde, Hasta: &hasta})
	if err != nil {
		return ResumenMensual{}, err
	}

	return Resumir(movimientos), nil
}

// ── Presupuestos ───────────────────────────────────────────────────────────

func (s *Service) Presupuestos(ctx context.Context) ([]Presupuesto, error) {
	return s.presupuestos.List(ctx)
}

func (s *Service) GuardarPresupuesto(ctx context.Context, p Presupuesto) (kernel.Result[Presupuesto], error) {
	v := ValidarPresupuesto(p)
	if !v.OK {
		return kernel.Err[Presupuesto](v.Error), nil
	}

	guardado, err := s.presupuestos.Upsert(ctx, v.Value)
	if err != nil {
		return kernel.Result[Presupuesto]{}, err
	}
	return kernel.Ok(*guardado), nil
}

func (s *Service) BorrarPresupuesto(ctx context.Context, categoriaID string) error {
	return s.presupuestos.Remove(ctx, categoriaID)
}

// PresupuestosConAvance empareja cada límite con lo gastado ese mes.
func (s *Service) PresupuestosConAvance(ctx context.Context, año, mes int) ([]PresupuestoConAvance, error) {
	if mes < 1 || mes > 12 {
		return nil, nil
	}

	presupuestos, err := s.presupuestos.List(ctx)
	if err != nil {
		return nil, err
	}

	desde := time.Date(año, time.Month(mes), 1, 0, 0, 0, 0, time.Local)
	hasta := desde.AddDate(0, 1, 0)

	movimientos, err := s.movimientos.List(ctx, FiltroMovimientos{Desde: &desde, Hasta: &hasta})
	if err != nil {
		return nil, err
	}

	// Se suma una vez por categoría en lugar de recorrer la lista entera
	// por cada presupuesto.
	gastado := map[string]Dinero{}
	for _, m := range movimientos {
		if m.Kind == Gasto && m.CategoryID != nil {
			gastado[*m.CategoryID] += m.Amount
		}
	}

	salida := make([]PresupuestoConAvance, 0, len(presupuestos))
	for _, p := range presupuestos {
		salida = append(salida, PresupuestoConAvance{
			Presupuesto: p,
			Avance:      Avance(p.CategoryID, p.Amount, gastado[p.CategoryID]),
		})
	}

	return salida, nil
}

// ── Pagos recurrentes ──────────────────────────────────────────────────────

func (s *Service) Recurrentes(ctx context.Context, soloActivos bool) ([]Recurrente, error) {
	return s.recurrentes.List(ctx, soloActivos)
}

func (s *Service) GuardarRecurrente(ctx context.Context, r Recurrente) (kernel.Result[Recurrente], error) {
	v := ValidarRecurrente(r)
	if !v.OK {
		return kernel.Err[Recurrente](v.Error), nil
	}

	var guardado *Recurrente
	var err error
	if v.Value.ID == "" {
		guardado, err = s.recurrentes.Create(ctx, v.Value)
	} else {
		guardado, err = s.recurrentes.Update(ctx, v.Value)
	}
	if err != nil {
		return kernel.Result[Recurrente]{}, err
	}
	if guardado == nil {
		return kernel.Err[Recurrente]("Ese pago recurrente ya no existe."), nil
	}

	return kernel.Ok(*guardado), nil
}

func (s *Service) BorrarRecurrente(ctx context.Context, id string) error {
	return s.recurrentes.Remove(ctx, id)
}

// ResumenRecurrentes cuenta cómo fue una pasada.
type ResumenRecurrentes struct {
	Creados  int
	Avisados int
	Errores  []string
}

// ProcesarRecurrentes crea los movimientos que tocaban y manda los avisos.
//
// Es idempotente por ciclo: el aviso se marca con reminderSentAt y la fecha
// del próximo cargo solo avanza, así que pasar dos veces en el mismo minuto
// no duplica nada.
func (s *Service) ProcesarRecurrentes(ctx context.Context, ahora time.Time) (ResumenRecurrentes, error) {
	pagos, err := s.recurrentes.List(ctx, true)
	if err != nil {
		return ResumenRecurrentes{}, err
	}

	var resumen ResumenRecurrentes
	monedas := map[string]string{}

	for _, p := range pagos {
		// Primero el aviso, antes de mover la fecha: si se avanzara antes,
		// la ventana del recordatorio ya habría pasado.
		if TocaAvisar(p, ahora) {
			moneda := s.monedaDe(ctx, p.AccountID, monedas)

			if err := s.avisador.Send(ctx, TextoDelAviso(p, moneda)); err != nil {
				resumen.Errores = append(resumen.Errores, "Aviso de «"+p.Name+"»: "+err.Error())
			} else {
				resumen.Avisados++
				// Solo se marca si el envío salió bien, para reintentarlo.
				_ = s.recurrentes.MarcarAvisado(ctx, p.ID, ahora)
			}
		}

		ocurrencias, siguiente := Vencidos(p.NextRunAt, p.Frequency, ahora)
		if len(ocurrencias) == 0 {
			continue
		}

		fallo := false
		for _, cuando := range ocurrencias {
			nota := "Automático · " + p.Name
			_, err := s.movimientos.Create(ctx, Movimiento{
				AccountID: p.AccountID, CategoryID: p.CategoryID,
				Kind: p.Kind, Amount: p.Amount, OccurredAt: cuando, Notes: &nota,
			})
			if err != nil {
				resumen.Errores = append(resumen.Errores, "Pago «"+p.Name+"»: "+err.Error())
				fallo = true
				break
			}
			resumen.Creados++
		}

		// La fecha solo avanza si se crearon todos los movimientos: si no,
		// se perderían los que faltaban.
		if !fallo {
			if err := s.recurrentes.MarcarProcesado(ctx, p.ID, siguiente); err != nil {
				resumen.Errores = append(resumen.Errores, "Pago «"+p.Name+"»: "+err.Error())
			}
		}
	}

	return resumen, nil
}

// monedaDe recuerda la moneda de cada cuenta durante la pasada, para no
// preguntarla una vez por pago.
func (s *Service) monedaDe(ctx context.Context, cuentaID string, cache map[string]string) string {
	if m, hay := cache[cuentaID]; hay {
		return m
	}

	moneda := "USD"
	if c, err := s.cuentas.Get(ctx, cuentaID); err == nil && c != nil && c.Currency != "" {
		moneda = c.Currency
	}

	cache[cuentaID] = moneda
	return moneda
}
