package email

import (
	"context"
	"strings"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

func TestCorreoValido(t *testing.T) {
	buenos := []string{"a@b.co", "nombre.apellido@dominio.com", "  con@espacios.es  ", "a+etiqueta@b.org"}
	for _, c := range buenos {
		if !CorreoValido(c) {
			t.Errorf("rechazó una dirección correcta: %q", c)
		}
	}

	malos := []string{"", "sinarroba", "sin@punto", "@sinnombre.com", "dos @espacios.com", "a@b"}
	for _, c := range malos {
		if CorreoValido(c) {
			t.Errorf("aceptó una dirección inválida: %q", c)
		}
	}
}

func TestSepararDestinatarios(t *testing.T) {
	validos, invalidos := SepararDestinatarios("uno@a.com, dos@b.com; tres@c.com\ncuatro@d.com")
	if len(validos) != 4 {
		t.Errorf("salieron %d válidos, quería 4: %v", len(validos), validos)
	}
	if len(invalidos) != 0 {
		t.Errorf("marcó como inválidos: %v", invalidos)
	}
}

func TestSepararApartaLosInvalidos(t *testing.T) {
	validos, invalidos := SepararDestinatarios("bueno@a.com, malo, otro@b.com")
	if len(validos) != 2 {
		t.Errorf("válidos = %v", validos)
	}
	if len(invalidos) != 1 || invalidos[0] != "malo" {
		t.Errorf("inválidos = %v", invalidos)
	}
}

// Escribir dos veces la misma dirección no debe mandar el correo dos veces.
func TestSepararQuitaRepetidos(t *testing.T) {
	validos, _ := SepararDestinatarios("a@b.com, a@b.com,  a@b.com ")
	if len(validos) != 1 {
		t.Errorf("salieron %d, quería 1: %v", len(validos), validos)
	}
}

func TestValidarMensaje(t *testing.T) {
	bueno := Mensaje{Para: []string{"a@b.com"}, Asunto: "Hola", Cuerpo: "Qué tal"}
	if res := ValidarMensaje(bueno); !res.OK {
		t.Errorf("rechazó un mensaje correcto: %s", res.Error)
	}

	casos := map[string]Mensaje{
		"sin destinatarios": {Asunto: "a", Cuerpo: "b"},
		"destinatario malo": {Para: []string{"malo"}, Asunto: "a", Cuerpo: "b"},
		"sin asunto":        {Para: []string{"a@b.com"}, Cuerpo: "b"},
		"asunto en blanco":  {Para: []string{"a@b.com"}, Asunto: "   ", Cuerpo: "b"},
		"sin cuerpo":        {Para: []string{"a@b.com"}, Asunto: "a"},
	}
	for nombre, m := range casos {
		if res := ValidarMensaje(m); res.OK {
			t.Errorf("%s: lo dio por bueno", nombre)
		}
	}
}

// ── Plantillas ─────────────────────────────────────────────────────────────

func TestVariables(t *testing.T) {
	vs := Variables("Hola {{nombre}}, tu pedido {{pedido.id}} de {{ nombre }} va en camino")
	if len(vs) != 2 {
		t.Fatalf("encontró %v, quería 2 sin repetir", vs)
	}
	if vs[0] != "nombre" || vs[1] != "pedido.id" {
		t.Errorf("variables = %v", vs)
	}
	if v := Variables("sin huecos"); len(v) != 0 {
		t.Errorf("inventó variables: %v", v)
	}
}

func TestRellenar(t *testing.T) {
	texto := "Hola {{nombre}}, son {{ cantidad }} euros."
	got := Rellenar(texto, map[string]string{"nombre": "Ana", "cantidad": "30"})
	if got != "Hola Ana, son 30 euros." {
		t.Errorf("Rellenar = %q", got)
	}
}

// Una variable sin valor se queda vacía: enseñarle {{nombre}} al
// destinatario sería peor que una frase a la que le falta algo.
func TestVariableSinValorNoSeFiltra(t *testing.T) {
	got := Rellenar("Hola {{nombre}}.", map[string]string{})
	if strings.Contains(got, "{{") {
		t.Errorf("filtró la plantilla al destinatario: %q", got)
	}
	if got != "Hola ." {
		t.Errorf("Rellenar = %q", got)
	}
}

// ── Componer el mensaje ────────────────────────────────────────────────────

// Lo importante: un asunto con un salto de línea permitiría inyectar
// cabeceras propias, por ejemplo un Bcc a quien quisiera el atacante.
func TestNoSePuedenInyectarCabeceras(t *testing.T) {
	m := Mensaje{
		Para:   []string{"victima@ejemplo.com"},
		Asunto: "Normal\r\nBcc: atacante@malo.com",
		Cuerpo: "texto",
	}

	crudo := string(Componer("yo@ejemplo.com", m))
	cabeceras, _, _ := strings.Cut(crudo, "\r\n\r\n")

	// Lo que importa no es que el texto "Bcc:" desaparezca —dentro del
	// asunto es inofensivo—, sino que no llegue a empezar una línea: solo
	// entonces sería una cabecera de verdad.
	for _, linea := range strings.Split(cabeceras, "\r\n") {
		if strings.HasPrefix(strings.ToLower(linea), "bcc:") {
			t.Errorf("se coló una cabecera Bcc:\n%s", cabeceras)
		}
	}
	if strings.Count(cabeceras, "Subject:") != 1 {
		t.Errorf("el asunto se partió en varias cabeceras:\n%s", cabeceras)
	}
	if lineas := strings.Count(cabeceras, "\r\n"); lineas != 4 {
		t.Errorf("salieron %d saltos de línea en las cabeceras, esperaba 4:\n%s", lineas, cabeceras)
	}
}

func TestComponerLlevaTextoYHTML(t *testing.T) {
	m := Mensaje{Para: []string{"a@b.com"}, Asunto: "Hola", Cuerpo: "línea 1\nlínea 2"}
	crudo := string(Componer("yo@ejemplo.com", m))

	for _, esperado := range []string{
		"From: yo@ejemplo.com",
		"To: a@b.com",
		"Subject: Hola",
		"text/plain; charset=UTF-8",
		"text/html; charset=UTF-8",
		"línea 1<br>línea 2",
	} {
		if !strings.Contains(crudo, esperado) {
			t.Errorf("falta %q en el mensaje", esperado)
		}
	}
}

// El cuerpo lo escribe una persona y puede llevar < o &.
func TestElHTMLSeEscapa(t *testing.T) {
	m := Mensaje{Para: []string{"a@b.com"}, Asunto: "x", Cuerpo: "<script>alert(1)</script> & más"}
	crudo := string(Componer("yo@ejemplo.com", m))

	// Hay que mirar solo la parte HTML: en la de texto plano el cuerpo va
	// literal, que es justo lo que debe pasar.
	_, html, hay := strings.Cut(crudo, "Content-Type: text/html; charset=UTF-8")
	if !hay {
		t.Fatal("el mensaje no trae parte HTML")
	}

	if strings.Contains(html, "<script>") {
		t.Errorf("no escapó el HTML del cuerpo:\n%s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") || !strings.Contains(html, "&amp;") {
		t.Errorf("no escapó como se esperaba:\n%s", html)
	}
}

// ── El servicio ────────────────────────────────────────────────────────────

type enviadorFalso struct {
	fallanEstos map[string]string
	enviados    []Mensaje
}

func (e *enviadorFalso) Enviar(_ context.Context, m Mensaje) kernel.Result[struct{}] {
	e.enviados = append(e.enviados, m)
	if motivo, falla := e.fallanEstos[m.Para[0]]; falla {
		return kernel.Err[struct{}](motivo)
	}
	return kernel.Ok(struct{}{})
}

type registroFalso struct {
	anotados []string
}

func (r *registroFalso) Anotar(_ context.Context, para, _, estado string, _ *string) error {
	r.anotados = append(r.anotados, para+":"+estado)
	return nil
}

func servicioConEnviador(e Enviador, reg Registro) *Service {
	return NewService(e, nil, nil, nil, reg)
}

// Un mensaje por destinatario, no uno con todos en copia: así nadie ve las
// direcciones de los demás.
func TestSeMandaUnMensajePorDestinatario(t *testing.T) {
	env := &enviadorFalso{}
	svc := servicioConEnviador(env, nil)

	res, err := svc.Enviar(context.Background(), "a@x.com, b@x.com, c@x.com", "Hola", "Qué tal", nil, "")
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if res.Value.Enviados != 3 {
		t.Errorf("enviados = %d, quería 3", res.Value.Enviados)
	}
	if len(env.enviados) != 3 {
		t.Fatalf("hizo %d envíos", len(env.enviados))
	}
	for i, m := range env.enviados {
		if len(m.Para) != 1 {
			t.Errorf("el envío %d llevaba %d destinatarios: %v", i, len(m.Para), m.Para)
		}
	}
}

func TestLasVariablesSeRellenanAntesDeEnviar(t *testing.T) {
	env := &enviadorFalso{}
	svc := servicioConEnviador(env, nil)

	_, _ = svc.Enviar(context.Background(), "a@x.com", "Hola {{nombre}}", "Son {{total}}€",
		map[string]string{"nombre": "Ana", "total": "30"}, "")

	if env.enviados[0].Asunto != "Hola Ana" {
		t.Errorf("asunto = %q", env.enviados[0].Asunto)
	}
	if env.enviados[0].Cuerpo != "Son 30€" {
		t.Errorf("cuerpo = %q", env.enviados[0].Cuerpo)
	}
}

// Que falle uno es un envío parcial; que fallen todos es un fallo.
func TestEnvioParcialVsFalloTotal(t *testing.T) {
	t.Run("parcial", func(t *testing.T) {
		env := &enviadorFalso{fallanEstos: map[string]string{"b@x.com": "buzón lleno"}}
		reg := &registroFalso{}
		svc := servicioConEnviador(env, reg)

		res, _ := svc.Enviar(context.Background(), "a@x.com, b@x.com", "Hola", "Qué tal", nil, "")
		if !res.OK {
			t.Fatalf("un envío parcial no es un fallo: %s", res.Error)
		}
		if res.Value.Enviados != 1 || res.Value.Fallidos != 1 {
			t.Errorf("resumen = %+v", res.Value)
		}
		if len(reg.anotados) != 2 {
			t.Errorf("anotó %v, quería uno por destinatario", reg.anotados)
		}
	})

	t.Run("total", func(t *testing.T) {
		env := &enviadorFalso{fallanEstos: map[string]string{
			"a@x.com": "sin servidor", "b@x.com": "sin servidor",
		}}
		svc := servicioConEnviador(env, nil)

		res, _ := svc.Enviar(context.Background(), "a@x.com, b@x.com", "Hola", "Qué tal", nil, "")
		if res.OK {
			t.Error("dio por bueno un envío en el que no salió ninguno")
		}
	})
}

func TestDestinatarioInvalidoNoLlegaAEnviarNada(t *testing.T) {
	env := &enviadorFalso{}
	svc := servicioConEnviador(env, nil)

	res, _ := svc.Enviar(context.Background(), "bueno@x.com, esto-no-vale", "Hola", "Qué tal", nil, "")
	if res.OK {
		t.Error("envió pese a haber un destinatario inválido")
	}
	if len(env.enviados) != 0 {
		t.Errorf("llegó a enviar %d correos", len(env.enviados))
	}
}

func TestSinSMTPLoDiceClaro(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil)

	res, _ := svc.Enviar(context.Background(), "a@x.com", "Hola", "Qué tal", nil, "")
	if res.OK {
		t.Fatal("envió sin transporte")
	}
	if !strings.Contains(res.Error, "SMTP") {
		t.Errorf("el mensaje no explica el problema: %q", res.Error)
	}
}

func TestSMTPSinConfigurarNoEnvia(t *testing.T) {
	e := NewSMTPEnviador(SMTPConfig{})
	if res := e.Enviar(context.Background(), Mensaje{Para: []string{"a@b.com"}}); res.OK {
		t.Error("envió sin configuración de SMTP")
	}

	completa := SMTPConfig{Host: "smtp.x.com", Port: 587, From: "yo@x.com"}
	if !completa.Configurado() {
		t.Error("dio por incompleta una configuración que vale")
	}
}
