package github

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

func punt[T any](v T) *T { return &v }

func TestValidarUsuario(t *testing.T) {
	buenos := []string{"Romanxt66", "a", "con-guion", "A1-b2-c3", "  conespacios  "}
	for _, u := range buenos {
		if res := ValidarUsuario(u); !res.OK {
			t.Errorf("rechazó un usuario válido %q: %s", u, res.Error)
		}
	}

	malos := []string{
		"", "   ",
		"-empieza-con-guion",
		"acaba-con-guion-",
		"dos--guiones",
		"con espacio",
		"con_guionbajo",
		"con.punto",
		"abcdefghijabcdefghijabcdefghijabcdefghij", // 40 caracteres
	}
	for _, u := range malos {
		if ValidarUsuario(u).OK {
			t.Errorf("aceptó un usuario inválido: %q", u)
		}
	}

	// Justo 39 caracteres sí vale.
	justo := "abcdefghijabcdefghijabcdefghijabcdefghi"
	if len(justo) != 39 {
		t.Fatalf("la cadena de prueba tiene %d caracteres", len(justo))
	}
	if !ValidarUsuario(justo).OK {
		t.Error("rechazó un usuario de exactamente 39 caracteres")
	}
}

func TestNormalizarToken(t *testing.T) {
	if NormalizarToken("   ") != nil {
		t.Error("un token en blanco debería quedar nulo")
	}
	if NormalizarToken("") != nil {
		t.Error("un token vacío debería quedar nulo")
	}
	if got := NormalizarToken("  ghp_abc  "); got == nil || *got != "ghp_abc" {
		t.Errorf("no recortó el token: %v", got)
	}
}

// ── Ordenar y filtrar ──────────────────────────────────────────────────────

func reposDePrueba() []Repo {
	ayer := time.Now().Add(-24 * time.Hour)
	hace1mes := time.Now().Add(-30 * 24 * time.Hour)

	return []Repo{
		{Name: "zeta", Stars: 5, PushedAt: &hace1mes, Topics: []string{"go"}},
		{Name: "Alfa", Stars: 10, PushedAt: &ayer, Description: punt("Un panel")},
		{Name: "media", Stars: 10, Topics: []string{"react", "web"}},
	}
}

func nombres(repos []Repo) []string {
	var salida []string
	for _, r := range repos {
		salida = append(salida, r.Name)
	}
	return salida
}

func TestOrdenarPorReciente(t *testing.T) {
	got := nombres(Ordenar(reposDePrueba(), OrdenReciente))
	quiero := []string{"Alfa", "zeta", "media"}
	comparar(t, got, quiero)
}

// A igualdad de estrellas manda el más reciente: si no, el orden dependería
// del azar y la lista bailaría entre recargas.
func TestOrdenarPorEstrellasDesempataPorFecha(t *testing.T) {
	got := nombres(Ordenar(reposDePrueba(), OrdenEstrellas))
	quiero := []string{"Alfa", "media", "zeta"}
	comparar(t, got, quiero)
}

// Ordenar por nombre no debe distinguir mayúsculas: "Alfa" va antes que
// "media" aunque su código sea mayor.
func TestOrdenarPorNombreIgnoraMayusculas(t *testing.T) {
	got := nombres(Ordenar(reposDePrueba(), OrdenNombre))
	quiero := []string{"Alfa", "media", "zeta"}
	comparar(t, got, quiero)
}

func TestOrdenarNoTocaLaListaOriginal(t *testing.T) {
	original := reposDePrueba()
	antes := nombres(original)

	_ = Ordenar(original, OrdenNombre)

	comparar(t, nombres(original), antes)
}

func TestFiltrarPorTexto(t *testing.T) {
	casos := map[string][]string{
		"alfa":    {"Alfa"},
		"ALFA":    {"Alfa"},  // sin distinguir mayúsculas
		"panel":   {"Alfa"},  // busca en la descripción
		"react":   {"media"}, // y en los temas
		"":        {"zeta", "Alfa", "media"},
		"ninguno": nil,
	}

	for texto, quiero := range casos {
		got := nombres(Filtrar(reposDePrueba(), Filtro{
			Texto: texto, IncluirForks: true, IncluirArchivados: true,
		}))
		comparar(t, got, quiero)
	}
}

func TestFiltrarForksYArchivados(t *testing.T) {
	repos := []Repo{
		{Name: "propio"},
		{Name: "copia", IsFork: true},
		{Name: "viejo", IsArchived: true},
	}

	sinForks := nombres(Filtrar(repos, Filtro{IncluirForks: false, IncluirArchivados: true}))
	comparar(t, sinForks, []string{"propio", "viejo"})

	sinArchivados := nombres(Filtrar(repos, Filtro{IncluirForks: true, IncluirArchivados: false}))
	comparar(t, sinArchivados, []string{"propio", "copia"})

	soloPropio := nombres(Filtrar(repos, Filtro{}))
	comparar(t, soloPropio, []string{"propio"})
}

func comparar(t *testing.T, got, quiero []string) {
	t.Helper()
	if len(got) != len(quiero) {
		t.Fatalf("salieron %v, quería %v", got, quiero)
	}
	for i := range quiero {
		if got[i] != quiero[i] {
			t.Errorf("posición %d: %q, quería %q (lista %v)", i, got[i], quiero[i], got)
		}
	}
}

// ── El servicio ────────────────────────────────────────────────────────────

type clienteFalso struct {
	perfil Perfil
	repos  []Repo
	fallo  string
}

func (c *clienteFalso) Perfil(context.Context, Credenciales) kernel.Result[Perfil] {
	if c.fallo != "" {
		return kernel.Err[Perfil](c.fallo)
	}
	return kernel.Ok(c.perfil)
}

func (c *clienteFalso) Repos(context.Context, Credenciales) kernel.Result[[]Repo] {
	if c.fallo != "" {
		return kernel.Err[[]Repo](c.fallo)
	}
	return kernel.Ok(c.repos)
}

type repoFalso struct {
	activa   *Config
	guardada *Config
	fallo    error
}

func (r *repoFalso) GetActive(context.Context) (*Config, error) {
	return r.activa, r.fallo
}

func (r *repoFalso) Upsert(_ context.Context, u string, t *string) (*Config, error) {
	if r.fallo != nil {
		return nil, r.fallo
	}
	r.guardada = &Config{ID: "c1", Username: u, Token: t, Active: true}
	return r.guardada, nil
}

func (r *repoFalso) Remove(context.Context, string) error { return r.fallo }

func TestSinConfiguracionSeExplicaQueHayQueConfigurar(t *testing.T) {
	svc := NewService(&repoFalso{}, &clienteFalso{})

	res, err := svc.Resumen(context.Background())
	if err != nil {
		t.Fatalf("debía ser error de negocio: %v", err)
	}
	if res.OK {
		t.Fatal("dio un resumen sin configuración")
	}
	if res.Error == "" {
		t.Error("no explicó qué hacer")
	}
}

func TestResumenTraePerfilYRepos(t *testing.T) {
	repo := &repoFalso{activa: &Config{ID: "c1", Username: "yo"}}
	cliente := &clienteFalso{
		perfil: Perfil{Login: "yo"},
		repos:  reposDePrueba(),
	}

	res, err := NewService(repo, cliente).Resumen(context.Background())
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if res.Value.Perfil.Login != "yo" {
		t.Errorf("perfil = %+v", res.Value.Perfil)
	}
	if len(res.Value.Repos) != 3 {
		t.Errorf("trajo %d repos", len(res.Value.Repos))
	}
}

func TestUnUsuarioInvalidoNoSeGuarda(t *testing.T) {
	repo := &repoFalso{}

	res, err := NewService(repo, &clienteFalso{}).
		Guardar(context.Background(), "-no-vale-", "ghp_x")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK {
		t.Error("guardó un usuario inválido")
	}
	if repo.guardada != nil {
		t.Error("llegó a escribir en la base")
	}
}

func TestGuardarNormalizaElToken(t *testing.T) {
	repo := &repoFalso{}

	_, _ = NewService(repo, &clienteFalso{}).Guardar(context.Background(), "yo", "   ")
	if repo.guardada.Token != nil {
		t.Errorf("un token en blanco debería quedar nulo: %v", repo.guardada.Token)
	}
}

func TestFalloDeBaseSubeComoError(t *testing.T) {
	repo := &repoFalso{fallo: errors.New("sin conexión")}
	if _, err := NewService(repo, &clienteFalso{}).Resumen(context.Background()); err == nil {
		t.Error("se tragó un fallo de base")
	}
}

func TestExplicarDistingueLimiteDeFaltaDePermisos(t *testing.T) {
	limite := explicar(403, "0", nil)
	permisos := explicar(403, "42", []byte(`{"message":"Forbidden"}`))

	if limite == permisos {
		t.Error("un 403 por límite y uno por permisos son problemas distintos")
	}
	if explicar(401, "", nil) == "" {
		t.Error("401 sin mensaje")
	}
	if explicar(404, "", nil) == "" {
		t.Error("404 sin mensaje")
	}
}
