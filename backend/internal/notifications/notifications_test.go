package notifications

import (
	"context"
	"strings"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

func TestValidarToken(t *testing.T) {
	casos := []struct {
		token  string
		quiere bool
	}{
		{"123456:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", true},
		{"  123456:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw  ", true}, // con espacios
		{"123456:corto", false},
		{"sin-dos-puntos", false},
		{"abc:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", false}, // el id debe ser numérico
		{"", false},
	}

	for _, caso := range casos {
		t.Run(caso.token, func(t *testing.T) {
			if got := ValidarToken(caso.token); got != caso.quiere {
				t.Errorf("ValidarToken(%q) = %v", caso.token, got)
			}
		})
	}
}

func TestNormalizarChatID(t *testing.T) {
	t.Run("acepta negativos (son los grupos)", func(t *testing.T) {
		res := NormalizarChatID(" -1001234567890 ")
		if !res.OK || res.Value != "-1001234567890" {
			t.Errorf("resultado = %+v", res)
		}
	})

	for _, malo := range []string{"", "abc", "12.5", "12a"} {
		t.Run("rechaza "+malo, func(t *testing.T) {
			if res := NormalizarChatID(malo); res.OK {
				t.Errorf("%q no debería aceptarse", malo)
			}
		})
	}
}

// ── Dobles ─────────────────────────────────────────────────────────────────

type repoFalso struct {
	config   *Config
	guardado *Config
}

func (r *repoFalso) GetActive(context.Context) (*Config, error) { return r.config, nil }

func (r *repoFalso) Save(_ context.Context, c Config) (*Config, error) {
	if c.ID == "" {
		c.ID = "nuevo"
	}
	r.guardado = &c
	return &c, nil
}

func (r *repoFalso) Remove(context.Context, string) error { return nil }

type senderFalso struct {
	enviado   string
	resultado kernel.Result[struct{}]
}

func (s *senderFalso) Send(_ context.Context, _ Provider, _, _, message string) kernel.Result[struct{}] {
	s.enviado = message
	return s.resultado
}

// ── Casos de uso ───────────────────────────────────────────────────────────

func TestSendSinConfiguracion(t *testing.T) {
	svc := NewService(&repoFalso{config: nil}, &senderFalso{})

	res := svc.Send(context.Background(), "hola")
	if res.OK {
		t.Fatal("sin configuración no debería enviar")
	}
	// El mensaje tiene que decir qué hacer, no solo que falló.
	if !strings.Contains(res.Error, "Configúrala en Notificaciones") {
		t.Errorf("el error debería indicar dónde arreglarlo: %q", res.Error)
	}
}

func TestSendMensajeVacio(t *testing.T) {
	svc := NewService(&repoFalso{config: &Config{Recipient: "1", Credential: "x"}}, &senderFalso{})

	if res := svc.Send(context.Background(), "   "); res.OK {
		t.Error("un mensaje en blanco no debería enviarse")
	}
}

func TestSendCorrecto(t *testing.T) {
	sender := &senderFalso{resultado: kernel.Ok(struct{}{})}
	svc := NewService(&repoFalso{
		config: &Config{Provider: ProviderTelegram, Recipient: "123", Credential: "tok"},
	}, sender)

	res := svc.Send(context.Background(), "aviso")
	if !res.OK {
		t.Fatalf("debería enviarse: %s", res.Error)
	}
	if sender.enviado != "aviso" {
		t.Errorf("llegó %q al proveedor", sender.enviado)
	}
}

func TestSendSinCredencial(t *testing.T) {
	svc := NewService(&repoFalso{
		config: &Config{Provider: ProviderTelegram, Recipient: "123"},
	}, &senderFalso{})

	if res := svc.Send(context.Background(), "hola"); res.OK {
		t.Error("sin credencial no debería enviar")
	}
}

func TestSaveTelegramConservaElTokenGuardado(t *testing.T) {
	// Cambiar el chat sin volver a escribir el token es el caso habitual.
	repo := &repoFalso{config: &Config{
		ID: "c1", Provider: ProviderTelegram, Recipient: "111", Credential: "tok-viejo",
	}}
	svc := NewService(repo, &senderFalso{})

	res, err := svc.SaveTelegram(context.Background(), "", "222", "")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !res.OK {
		t.Fatalf("debería guardarse: %s", res.Error)
	}
	if repo.guardado.Credential != "tok-viejo" {
		t.Error("el token guardado debería conservarse al no enviar uno nuevo")
	}
	if repo.guardado.Recipient != "222" {
		t.Error("el chat sí debería actualizarse")
	}
}

func TestSaveTelegramRechazaTokenMalo(t *testing.T) {
	svc := NewService(&repoFalso{}, &senderFalso{})

	res, err := svc.SaveTelegram(context.Background(), "", "123", "esto-no-es-un-token")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK || !strings.Contains(res.Error, "formato") {
		t.Errorf("se esperaba rechazo por formato: %+v", res)
	}
}

func TestSaveTelegramSinTokenNiGuardado(t *testing.T) {
	svc := NewService(&repoFalso{config: nil}, &senderFalso{})

	res, _ := svc.SaveTelegram(context.Background(), "", "123", "")
	if res.OK {
		t.Error("sin token nuevo ni guardado no debería poder configurarse")
	}
}
