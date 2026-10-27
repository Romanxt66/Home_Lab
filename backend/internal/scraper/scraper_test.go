package scraper

import (
	"context"
	"strings"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

func TestValidarURL(t *testing.T) {
	casos := []struct {
		nombre string
		dado   string
		vale   bool
	}{
		{"https normal", "https://ejemplo.com/pagina", true},
		{"http normal", "http://ejemplo.com", true},
		{"con espacios alrededor", "  https://ejemplo.com  ", true},
		{"vacía", "", false},
		{"solo espacios", "   ", false},
		{"sin esquema", "ejemplo.com", false},
		{"file leería el disco del servidor", "file:///etc/passwd", false},
		{"javascript", "javascript:alert(1)", false},
		{"ftp", "ftp://ejemplo.com", false},
		{"data", "data:text/html,<p>hola</p>", false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res := ValidarURL(c.dado)
			if res.OK != c.vale {
				t.Errorf("ValidarURL(%q).OK = %v, quería %v (%s)", c.dado, res.OK, c.vale, res.Error)
			}
		})
	}
}

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"  hola   mundo  ":    "hola mundo",
		"con\nsaltos\ty tabs": "con saltos y tabs",
		"":                    "",
		"     ":               "",
		"una":                 "una",
	}
	for dado, quiero := range casos {
		if got := Normalizar(dado); got != quiero {
			t.Errorf("Normalizar(%q) = %q, quería %q", dado, got, quiero)
		}
	}
}

// Recortar cuenta caracteres, no bytes: con acentos lo segundo partiría una
// letra por la mitad.
func TestRecortarNoPartePorLaMitadUnAcento(t *testing.T) {
	if got := Recortar("ñññññ", 3); got != "ñññ" {
		t.Errorf("Recortar = %q, quería %q", got, "ñññ")
	}
	if got := Recortar("corto", 99); got != "corto" {
		t.Errorf("recortó algo que cabía: %q", got)
	}
}

// ── El caso de uso ─────────────────────────────────────────────────────────

type descargadorFalso struct {
	pagina   Pagina
	error    string
	pedidaA  string
	llamadas int
}

func (d *descargadorFalso) Descargar(_ context.Context, url string) kernel.Result[Pagina] {
	d.llamadas++
	d.pedidaA = url
	if d.error != "" {
		return kernel.Err[Pagina](d.error)
	}
	return kernel.Ok(d.pagina)
}

const htmlDePrueba = `<html><body>
	<a href="/uno" class="enlace">  Uno
	  con salto </a>
	<a href="/dos" class="enlace">Dos</a>
	<p>Otra cosa</p>
</body></html>`

func servicioDePrueba(d *descargadorFalso) *Service {
	return NewService(d, NewGoqueryExtractor())
}

func TestSinSelectorDevuelveElCuerpo(t *testing.T) {
	d := &descargadorFalso{pagina: Pagina{Status: 200, ContentType: "text/html", Body: htmlDePrueba}}

	res := servicioDePrueba(d).Ejecutar(context.Background(), "https://ejemplo.com", "", "")
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}
	if res.Value.Crudo != htmlDePrueba {
		t.Error("no devolvió el cuerpo tal cual")
	}
	if res.Value.Elementos != nil {
		t.Error("devolvió elementos sin que se pidiera selector")
	}
}

func TestConSelectorSacaElTextoNormalizado(t *testing.T) {
	d := &descargadorFalso{pagina: Pagina{Status: 200, Body: htmlDePrueba}}

	res := servicioDePrueba(d).Ejecutar(context.Background(), "https://ejemplo.com", "a.enlace", "")
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}
	quiero := []string{"Uno con salto", "Dos"}
	if len(res.Value.Elementos) != len(quiero) {
		t.Fatalf("sacó %v, quería %v", res.Value.Elementos, quiero)
	}
	for i := range quiero {
		if res.Value.Elementos[i] != quiero[i] {
			t.Errorf("elemento %d = %q, quería %q", i, res.Value.Elementos[i], quiero[i])
		}
	}
}

func TestConAtributoLeeElAtributo(t *testing.T) {
	d := &descargadorFalso{pagina: Pagina{Status: 200, Body: htmlDePrueba}}

	res := servicioDePrueba(d).Ejecutar(context.Background(), "https://ejemplo.com", "a.enlace", "href")
	if !res.OK {
		t.Fatalf("falló: %s", res.Error)
	}
	if len(res.Value.Elementos) != 2 || res.Value.Elementos[0] != "/uno" {
		t.Errorf("sacó %v, quería los href", res.Value.Elementos)
	}
}

// Lo importante: un selector inválido debe decirlo, no fingir que no hubo
// resultados. goquery.Find devolvería vacío en silencio.
func TestSelectorInvalidoSeDistingueDeSinResultados(t *testing.T) {
	d := &descargadorFalso{pagina: Pagina{Status: 200, Body: htmlDePrueba}}
	svc := servicioDePrueba(d)

	malo := svc.Ejecutar(context.Background(), "https://ejemplo.com", "a[[[", "")
	if malo.OK {
		t.Error("dio por bueno un selector inválido")
	}
	if !strings.Contains(malo.Error, "selector") {
		t.Errorf("el mensaje no explica el problema: %q", malo.Error)
	}

	vacio := svc.Ejecutar(context.Background(), "https://ejemplo.com", "table.inexistente", "")
	if !vacio.OK {
		t.Errorf("un selector válido sin coincidencias debería salir bien: %s", vacio.Error)
	}
	if len(vacio.Value.Elementos) != 0 {
		t.Errorf("sacó algo de la nada: %v", vacio.Value.Elementos)
	}
}

// Una URL inválida no debe llegar a provocar una petición de red.
func TestURLInvalidaNoLlegaADescargar(t *testing.T) {
	d := &descargadorFalso{}

	res := servicioDePrueba(d).Ejecutar(context.Background(), "file:///etc/passwd", "", "")
	if res.OK {
		t.Error("aceptó un esquema file")
	}
	if d.llamadas != 0 {
		t.Errorf("hizo %d descargas; no debía hacer ninguna", d.llamadas)
	}
}

func TestFalloDeRedSeCuentaTalCual(t *testing.T) {
	d := &descargadorFalso{error: "No se pudo conectar con esa dirección."}

	res := servicioDePrueba(d).Ejecutar(context.Background(), "https://ejemplo.com", "a", "")
	if res.OK {
		t.Fatal("ignoró el fallo de red")
	}
	if res.Error != d.error {
		t.Errorf("cambió el mensaje: %q", res.Error)
	}
}

func TestElCuerpoSeRecortaAlLimite(t *testing.T) {
	largo := strings.Repeat("x", LimiteCrudo+500)
	d := &descargadorFalso{pagina: Pagina{Status: 200, Body: largo}}

	res := servicioDePrueba(d).Ejecutar(context.Background(), "https://ejemplo.com", "", "")
	if len([]rune(res.Value.Crudo)) != LimiteCrudo {
		t.Errorf("devolvió %d caracteres, el tope son %d", len([]rune(res.Value.Crudo)), LimiteCrudo)
	}
}

func TestNoSeDevuelvenMasDeMaxElementos(t *testing.T) {
	var b strings.Builder
	b.WriteString("<ul>")
	for i := 0; i < MaxElementos+50; i++ {
		b.WriteString("<li>x</li>")
	}
	b.WriteString("</ul>")

	d := &descargadorFalso{pagina: Pagina{Status: 200, Body: b.String()}}
	res := servicioDePrueba(d).Ejecutar(context.Background(), "https://ejemplo.com", "li", "")

	if len(res.Value.Elementos) != MaxElementos {
		t.Errorf("devolvió %d elementos, el tope son %d", len(res.Value.Elementos), MaxElementos)
	}
}
