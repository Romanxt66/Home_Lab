package email

import (
	"context"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Enviador entrega un mensaje. Hay dos implementaciones: el SMTP del sistema
// y el envío desde una cuenta de Gmail conectada.
type Enviador interface {
	Enviar(ctx context.Context, m Mensaje) kernel.Result[struct{}]
}

// PlantillaRepo guarda las plantillas.
type PlantillaRepo interface {
	List(ctx context.Context) ([]Plantilla, error)
	Get(ctx context.Context, id string) (*Plantilla, error)
	Create(ctx context.Context, p Plantilla) (*Plantilla, error)
	Update(ctx context.Context, p Plantilla) (*Plantilla, error)
	Remove(ctx context.Context, id string) error
}

// Registro anota cada intento de envío.
type Registro interface {
	Anotar(ctx context.Context, para, asunto, estado string, fallo *string) error
}

// CuentaRepo guarda los buzones conectados.
type CuentaRepo interface {
	List(ctx context.Context) ([]CuentaGoogle, error)
	Get(ctx context.Context, id string) (*CuentaGoogle, error)
	Upsert(ctx context.Context, c CuentaGoogle) (*CuentaGoogle, error)
	Remove(ctx context.Context, id string) error
}

// EnviadorPorCuenta entrega el mensaje desde un buzón concreto.
type EnviadorPorCuenta interface {
	EnviarDesde(ctx context.Context, cuentaID string, m Mensaje) kernel.Result[struct{}]
}

// Resumen de un envío.
type Resumen struct {
	Enviados int
	Fallidos int
}

type Service struct {
	smtp       Enviador
	porCuenta  EnviadorPorCuenta
	plantillas PlantillaRepo
	cuentas    CuentaRepo
	registro   Registro
}

func NewService(smtp Enviador, porCuenta EnviadorPorCuenta, plantillas PlantillaRepo, cuentas CuentaRepo, registro Registro) *Service {
	return &Service{
		smtp: smtp, porCuenta: porCuenta,
		plantillas: plantillas, cuentas: cuentas, registro: registro,
	}
}

// Enviar manda el correo, rellenando antes las variables.
//
// Se manda un mensaje por destinatario y no uno con todos en copia: así
// nadie ve las direcciones de los demás y el registro queda por persona,
// que es lo que sirve para saber a quién no le llegó.
func (s *Service) Enviar(ctx context.Context, crudos, asunto, cuerpo string, valores map[string]string, cuentaID string) (kernel.Result[Resumen], error) {
	validos, invalidos := SepararDestinatarios(crudos)
	if len(invalidos) > 0 {
		return kernel.Err[Resumen]("Estos destinatarios no son válidos: " + strings.Join(invalidos, ", ")), nil
	}

	asunto = Rellenar(asunto, valores)
	cuerpo = Rellenar(cuerpo, valores)

	if v := ValidarMensaje(Mensaje{Para: validos, Asunto: asunto, Cuerpo: cuerpo}); !v.OK {
		return kernel.Err[Resumen](v.Error), nil
	}

	enviar := s.elegirEnviador(cuentaID)

	var resumen Resumen
	var fallos []string

	for _, destinatario := range validos {
		uno := Mensaje{Para: []string{destinatario}, Asunto: asunto, Cuerpo: cuerpo}

		if res := enviar(ctx, uno); res.OK {
			resumen.Enviados++
			s.anotar(ctx, destinatario, asunto, "sent", nil)
		} else {
			resumen.Fallidos++
			fallos = append(fallos, destinatario+": "+res.Error)
			motivo := res.Error
			s.anotar(ctx, destinatario, asunto, "failed", &motivo)
		}
	}

	// Que falle alguno es un envío parcial; que fallen todos es un fallo.
	if resumen.Enviados == 0 {
		return kernel.Err[Resumen]("No se pudo enviar ningún correo. " + strings.Join(fallos, "; ")), nil
	}

	return kernel.Ok(resumen), nil
}

// elegirEnviador devuelve el transporte que toque: el buzón de Gmail si se
// pidió uno, y si no el SMTP del sistema.
func (s *Service) elegirEnviador(cuentaID string) func(context.Context, Mensaje) kernel.Result[struct{}] {
	if cuentaID != "" && s.porCuenta != nil {
		return func(ctx context.Context, m Mensaje) kernel.Result[struct{}] {
			return s.porCuenta.EnviarDesde(ctx, cuentaID, m)
		}
	}
	return func(ctx context.Context, m Mensaje) kernel.Result[struct{}] {
		if s.smtp == nil {
			return kernel.Err[struct{}]("No hay SMTP configurado ni cuenta de Gmail elegida.")
		}
		return s.smtp.Enviar(ctx, m)
	}
}

func (s *Service) anotar(ctx context.Context, para, asunto, estado string, fallo *string) {
	if s.registro == nil {
		return
	}
	// Perder la anotación es menos grave que deshacer un envío que ya salió.
	_ = s.registro.Anotar(ctx, para, asunto, estado, fallo)
}

// ── Plantillas ─────────────────────────────────────────────────────────────

func (s *Service) Plantillas(ctx context.Context) ([]Plantilla, error) {
	return s.plantillas.List(ctx)
}

func (s *Service) GuardarPlantilla(ctx context.Context, p Plantilla) (kernel.Result[Plantilla], error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return kernel.Err[Plantilla]("La plantilla necesita un nombre."), nil
	}

	var guardada *Plantilla
	var err error
	if p.ID == "" {
		guardada, err = s.plantillas.Create(ctx, p)
	} else {
		guardada, err = s.plantillas.Update(ctx, p)
	}
	if err != nil {
		return kernel.Result[Plantilla]{}, err
	}
	if guardada == nil {
		return kernel.Err[Plantilla]("Esa plantilla ya no existe."), nil
	}

	return kernel.Ok(*guardada), nil
}

func (s *Service) BorrarPlantilla(ctx context.Context, id string) error {
	return s.plantillas.Remove(ctx, id)
}

// ── Buzones conectados ─────────────────────────────────────────────────────

func (s *Service) Cuentas(ctx context.Context) ([]CuentaGoogle, error) {
	return s.cuentas.List(ctx)
}

func (s *Service) DesconectarCuenta(ctx context.Context, id string) error {
	return s.cuentas.Remove(ctx, id)
}
