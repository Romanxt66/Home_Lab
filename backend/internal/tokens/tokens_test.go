package tokens

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGenerar(t *testing.T) {
	claro, huella, visible, err := Generar()
	if err != nil {
		t.Fatal(err)
	}
	if !FormaValida(claro) || !strings.HasPrefix(claro, Prefijo) {
		t.Errorf("el token generado debe tener la forma esperada: %q", claro)
	}
	if huella != Huella(claro) || huella == claro || len(huella) != 64 {
		t.Errorf("la huella es el SHA-256 en hexadecimal y no el secreto: %q", huella)
	}
	if !strings.HasPrefix(claro, visible) || len(visible) != len(Prefijo)+6 {
		t.Errorf("el prefijo visible son los primeros caracteres: %q de %q", visible, claro)
	}
	otro, _, _, _ := Generar()
	if otro == claro {
		t.Error("dos tokens no pueden coincidir")
	}
}

func TestFormaValida(t *testing.T) {
	claro, _, _, _ := Generar()
	if !FormaValida(claro) {
		t.Fatal("uno nuestro debe valer")
	}
	for _, mal := range []string{"", "hl_", "hl_corto", claro + "x", "xx_" + claro[3:], claro[:len(claro)-1]} {
		if FormaValida(mal) {
			t.Errorf("%q no debe valer", mal)
		}
	}
}

func TestExtraerBearer(t *testing.T) {
	buenos := map[string]string{"Bearer abc": "abc", "bearer abc": "abc", "BEARER  abc ": "abc"}
	for entrada, esperado := range buenos {
		if got, ok := ExtraerBearer(entrada); !ok || got != esperado {
			t.Errorf("ExtraerBearer(%q) = %q,%v", entrada, got, ok)
		}
	}
	for _, mal := range []string{"", "Bearer", "Bearer ", "Basic abc", "abc"} {
		if _, ok := ExtraerBearer(mal); ok {
			t.Errorf("ExtraerBearer(%q) debía fallar", mal)
		}
	}
}

func TestValidaciones(t *testing.T) {
	if ValidarNombre("  ").OK || ValidarNombre(strings.Repeat("a", 61)).OK || !ValidarNombre(" Mi portátil ").OK {
		t.Error("nombre: vacío y largo fallan, uno normal vale")
	}
	if r := ValidarAlcance(""); !r.OK || r.Value != Lectura {
		t.Error("sin alcance es lectura: lo más prudente")
	}
	if r := ValidarAlcance("write"); !r.OK || !r.Value.PuedeEscribir() {
		t.Error("write puede escribir")
	}
	if ValidarAlcance("admin").OK || ValidarAlcance("Write").OK {
		t.Error("cualquier otro valor se rechaza")
	}
	if Lectura.PuedeEscribir() {
		t.Error("lectura no escribe")
	}
}

// ── Servicio con repositorio en memoria ────────────────────────────────────

type repoFalso struct {
	tokens   map[string]Token  // id → token
	huellas  map[string]string // huella → id
	usos     map[string]int
	revocado map[string]bool
	aprobado bool
}

func nuevo() *repoFalso {
	return &repoFalso{tokens: map[string]Token{}, huellas: map[string]string{}, usos: map[string]int{}, revocado: map[string]bool{}, aprobado: true}
}
func (r *repoFalso) Crear(_ context.Context, t Token, h string) error {
	r.tokens[t.ID], r.huellas[h] = t, t.ID
	return nil
}
func (r *repoFalso) Listar(_ context.Context, dueno string) ([]Token, error) {
	var out []Token
	for _, t := range r.tokens {
		if t.DuenoID == dueno {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *repoFalso) ContarActivos(_ context.Context, dueno string) (int, error) {
	n := 0
	for id, t := range r.tokens {
		if t.DuenoID == dueno && !r.revocado[id] {
			n++
		}
	}
	return n, nil
}
func (r *repoFalso) Revocar(_ context.Context, dueno, id string, _ time.Time) (bool, error) {
	if t, ok := r.tokens[id]; ok && t.DuenoID == dueno && !r.revocado[id] {
		r.revocado[id] = true
		return true, nil
	}
	return false, nil
}
func (r *repoFalso) BuscarPorHuella(_ context.Context, h string) (*Identidad, error) {
	id, ok := r.huellas[h]
	if !ok || r.revocado[id] || !r.aprobado {
		return nil, nil
	}
	t := r.tokens[id]
	return &Identidad{TokenID: id, UsuarioID: t.DuenoID, Alcance: t.Alcance}, nil
}
func (r *repoFalso) MarcarUso(_ context.Context, id string, _ time.Time) error {
	r.usos[id]++
	return nil
}

func TestServicio_ciclo(t *testing.T) {
	repo := nuevo()
	s := NewService(repo)
	ctx := context.Background()

	res, err := s.Crear(ctx, "ana", "Mi portátil", "write")
	if err != nil || !res.OK {
		t.Fatalf("crear: %v %+v", err, res)
	}
	if !strings.HasPrefix(res.Value.Claro, Prefijo) || res.Value.Token.Alcance != Escritura {
		t.Fatalf("token mal hecho: %+v", res.Value)
	}
	// Lo guardado no es el secreto.
	for h := range repo.huellas {
		if h == res.Value.Claro {
			t.Fatal("se guardó el secreto en claro")
		}
	}

	id, err := s.Autenticar(ctx, res.Value.Claro)
	if err != nil || id == nil || id.UsuarioID != "ana" || !id.Alcance.PuedeEscribir() {
		t.Fatalf("autenticar: %+v %v", id, err)
	}
	if repo.usos[res.Value.Token.ID] != 1 {
		t.Error("autenticar debe anotar el uso")
	}

	// Un token inventado, uno con mala forma y uno ajeno no entran.
	otro, _, _, _ := Generar()
	if id, _ := s.Autenticar(ctx, otro); id != nil {
		t.Error("un token que no existe no entra")
	}
	if id, _ := s.Autenticar(ctx, "hl_basura"); id != nil {
		t.Error("un token con mala forma no entra")
	}

	// Revocar: solo el dueño, y después ya no entra.
	if hecho, _ := s.Revocar(ctx, "luis", res.Value.Token.ID); hecho {
		t.Error("Luis no puede revocar el token de Ana")
	}
	if hecho, _ := s.Revocar(ctx, "ana", res.Value.Token.ID); !hecho {
		t.Error("Ana sí")
	}
	if id, _ := s.Autenticar(ctx, res.Value.Claro); id != nil {
		t.Error("un token revocado no entra")
	}
}

func TestServicio_cuentaNoAprobada(t *testing.T) {
	repo := nuevo()
	s := NewService(repo)
	res, _ := s.Crear(context.Background(), "ana", "x", "")
	repo.aprobado = false
	if id, _ := s.Autenticar(context.Background(), res.Value.Claro); id != nil {
		t.Error("si la cuenta deja de estar aprobada, sus tokens dejan de valer")
	}
}

func TestServicio_limites(t *testing.T) {
	s := NewService(nuevo())
	ctx := context.Background()
	for i := 0; i < MaxActivos; i++ {
		if r, _ := s.Crear(ctx, "ana", "t", ""); !r.OK {
			t.Fatalf("el token %d debía poder crearse: %s", i, r.Error)
		}
	}
	if r, _ := s.Crear(ctx, "ana", "uno más", ""); r.OK || !strings.Contains(r.Error, "10 tokens") {
		t.Errorf("el undécimo debe rechazarse: %+v", r)
	}
	if r, _ := s.Crear(ctx, "luis", "t", ""); !r.OK {
		t.Error("el límite es por persona")
	}
	if r, _ := s.Crear(ctx, "luis", "", ""); r.OK {
		t.Error("sin nombre no se crea")
	}
	if r, _ := s.Crear(ctx, "luis", "x", "root"); r.OK {
		t.Error("un permiso inventado no se acepta")
	}
}
