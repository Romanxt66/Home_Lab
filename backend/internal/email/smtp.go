package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// SMTPConfig son los datos del servidor de correo.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func (c SMTPConfig) Configurado() bool {
	return c.Host != "" && c.Port > 0 && c.From != ""
}

// SMTPEnviador manda por el servidor de correo del sistema.
type SMTPEnviador struct {
	cfg SMTPConfig
}

func NewSMTPEnviador(cfg SMTPConfig) *SMTPEnviador {
	return &SMTPEnviador{cfg: cfg}
}

func (e *SMTPEnviador) Enviar(ctx context.Context, m Mensaje) kernel.Result[struct{}] {
	if !e.cfg.Configurado() {
		return kernel.Err[struct{}](
			"No hay servidor de correo configurado. Revisa SMTP_HOST, SMTP_PORT y SMTP_FROM.")
	}

	mensaje := Componer(e.cfg.From, m)
	direccion := e.cfg.Host + ":" + strconv.Itoa(e.cfg.Port)

	var auth smtp.Auth
	if e.cfg.User != "" {
		auth = smtp.PlainAuth("", e.cfg.User, e.cfg.Pass, e.cfg.Host)
	}

	// net/smtp no entiende de contextos, así que se ejecuta aparte y se
	// espera o al resultado o a que el contexto se cancele.
	hecho := make(chan error, 1)
	go func() {
		hecho <- smtp.SendMail(direccion, auth, e.cfg.From, m.Para, mensaje)
	}()

	select {
	case <-ctx.Done():
		return kernel.Err[struct{}]("Se agotó el tiempo al enviar el correo.")
	case err := <-hecho:
		if err != nil {
			return kernel.Err[struct{}]("El servidor de correo rechazó el envío: " + err.Error())
		}
		return kernel.Ok(struct{}{})
	}
}

// Componer arma el mensaje RFC 5322 en texto plano y HTML.
//
// Se monta a mano en vez de con una biblioteca porque son veinte líneas y
// evita una dependencia más, pero hay un detalle que no es opcional: las
// cabeceras se limpian de saltos de línea. Un asunto con un \n permitiría
// inyectar cabeceras propias (un Bcc, por ejemplo) y mandar copia a quien
// quisiera el atacante.
func Componer(desde string, m Mensaje) []byte {
	frontera := "frontera-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	var b strings.Builder
	b.WriteString("From: " + limpiarCabecera(desde) + "\r\n")
	b.WriteString("To: " + limpiarCabecera(strings.Join(m.Para, ", ")) + "\r\n")
	b.WriteString("Subject: " + limpiarCabecera(m.Asunto) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + frontera + "\"\r\n")
	b.WriteString("\r\n")

	b.WriteString("--" + frontera + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(m.Cuerpo + "\r\n")

	b.WriteString("--" + frontera + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(aHTML(m.Cuerpo) + "\r\n")

	b.WriteString("--" + frontera + "--\r\n")

	return []byte(b.String())
}

// limpiarCabecera quita lo que permitiría inyectar cabeceras nuevas.
func limpiarCabecera(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(v)
}

// aHTML pasa el texto a HTML conservando los saltos de línea. Se escapa
// primero: el cuerpo lo escribe una persona y podría llevar < o &.
func aHTML(texto string) string {
	escapado := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(texto)
	return fmt.Sprintf("<div>%s</div>", strings.ReplaceAll(escapado, "\n", "<br>"))
}
