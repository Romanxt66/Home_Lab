package scheduler

import (
	"strings"
	"testing"
	"time"
)

func TestCronValido(t *testing.T) {
	buenos := []string{
		"* * * * *",
		"*/5 * * * *",
		"0 0 * * *",
		"30 9 * * 1-5",
		"0,15,30,45 * * * *",
		"0 9-17 * * *",
		"0 0-23/2 * * *",
		"59 23 31 12 6",
		"0 0 1 1 0",
		"0 0 * * 7", // domingo escrito como 7
	}
	for _, c := range buenos {
		if !CronValido(c) {
			t.Errorf("rechazó un cron correcto: %q — %s", c, ParsearCron(c).Error)
		}
	}

	malos := []string{
		"",
		"* * * *",      // faltan campos
		"* * * * * *",  // sobra uno
		"60 * * * *",   // minuto fuera de rango
		"* 24 * * *",   // hora fuera de rango
		"* * 0 * *",    // no existe el día 0
		"* * 32 * *",   // ni el 32
		"* * * 13 *",   // ni el mes 13
		"* * * * 8",    // ni el día 8 de la semana
		"10-5 * * * *", // rango al revés
		"*/0 * * * *",  // paso cero
		"*/-1 * * * *", // paso negativo
		"a * * * *",    // no es un número
		"* * * * 1-",   // rango incompleto
	}
	for _, c := range malos {
		if CronValido(c) {
			t.Errorf("dio por bueno un cron inválido: %q", c)
		}
	}
}

// Un cron mal escrito debe decir QUÉ campo falla, no solo que falla.
func TestElErrorSeñalaElCampo(t *testing.T) {
	res := ParsearCron("* 99 * * *")
	if res.OK {
		t.Fatal("aceptó una hora imposible")
	}
	if !strings.Contains(res.Error, "hora") {
		t.Errorf("el mensaje no dice qué campo falla: %q", res.Error)
	}
}

func TestCronCoincide(t *testing.T) {
	// Lunes 3 de marzo de 2025, 09:30.
	lunes := time.Date(2025, 3, 3, 9, 30, 0, 0, time.Local)
	if lunes.Weekday() != time.Monday {
		t.Fatalf("la fecha de prueba no es lunes, es %v", lunes.Weekday())
	}

	casos := []struct {
		expr   string
		quiero bool
	}{
		{"* * * * *", true},
		{"30 9 * * *", true},
		{"30 9 3 3 1", true},
		{"*/30 * * * *", true},
		{"*/15 * * * *", true},
		{"31 9 * * *", false},
		{"30 10 * * *", false},
		{"30 9 * * 0", false}, // domingo, y es lunes
		{"30 9 * * 1-5", true},
		{"30 9 4 3 *", false}, // otro día del mes
		{"no es un cron", false},
	}

	for _, c := range casos {
		if got := CronCoincide(c.expr, lunes); got != c.quiero {
			t.Errorf("CronCoincide(%q, lunes 09:30) = %v, quería %v", c.expr, got, c.quiero)
		}
	}
}

// El domingo escrito como 7 tiene que funcionar igual que escrito como 0.
// En el Lab se validaba hasta 6, así que un cron con 7 se rechazaba y la
// comparación contra el 7 era código muerto.
func TestDomingoVale0Y7(t *testing.T) {
	domingo := time.Date(2025, 3, 2, 12, 0, 0, 0, time.Local)
	if domingo.Weekday() != time.Sunday {
		t.Fatalf("la fecha de prueba no es domingo, es %v", domingo.Weekday())
	}

	for _, expr := range []string{"0 12 * * 0", "0 12 * * 7"} {
		if !CronCoincide(expr, domingo) {
			t.Errorf("%q no coincidió en domingo", expr)
		}
	}

	lunes := time.Date(2025, 3, 3, 12, 0, 0, 0, time.Local)
	for _, expr := range []string{"0 12 * * 0", "0 12 * * 7"} {
		if CronCoincide(expr, lunes) {
			t.Errorf("%q coincidió en lunes y solo valía para domingo", expr)
		}
	}
}

func TestPasoConRango(t *testing.T) {
	res := ParsearCron("0 8-18/2 * * *")
	if !res.OK {
		t.Fatalf("rechazó un rango con paso: %s", res.Error)
	}
	horas := res.Value[1]
	for _, h := range []int{8, 10, 12, 14, 16, 18} {
		if !horas[h] {
			t.Errorf("faltó la hora %d", h)
		}
	}
	for _, h := range []int{9, 11, 7, 19} {
		if horas[h] {
			t.Errorf("sobró la hora %d", h)
		}
	}
}

func TestListaConComas(t *testing.T) {
	res := ParsearCron("0,15,30,45 * * * *")
	if !res.OK {
		t.Fatalf("rechazó una lista: %s", res.Error)
	}
	minutos := res.Value[0]
	if len(minutos) != 4 {
		t.Errorf("salieron %d minutos, quería 4", len(minutos))
	}
	for _, m := range []int{0, 15, 30, 45} {
		if !minutos[m] {
			t.Errorf("faltó el minuto %d", m)
		}
	}
}
