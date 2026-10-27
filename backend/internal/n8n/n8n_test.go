package n8n

import (
	"context"
	"errors"
	"testing"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

func TestValidarBaseURL(t *testing.T) {
	casos := []struct {
		dado   string
		vale   bool
		quedar string
	}{
		{"https://n8n.midominio.com", true, "https://n8n.midominio.com"},
		{"  https://n8n.midominio.com/  ", true, "https://n8n.midominio.com"},
		{"https://n8n.midominio.com///", true, "https://n8n.midominio.com"},
		{"http://localhost:5678", true, "http://localhost:5678"},
		{"", false, ""},
		{"   ", false, ""},
		{"n8n.midominio.com", false, ""},
		{"ftp://n8n.midominio.com", false, ""},
	}

	for _, c := range casos {
		res := ValidarBaseURL(c.dado)
		if res.OK != c.vale {
			t.Errorf("ValidarBaseURL(%q).OK = %v, quería %v (%s)", c.dado, res.OK, c.vale, res.Error)
			continue
		}
		if c.vale && res.Value != c.quedar {
			t.Errorf("ValidarBaseURL(%q) = %q, quería %q", c.dado, res.Value, c.quedar)
		}
	}
}

func TestValidarAPIKey(t *testing.T) {
	if res := ValidarAPIKey("  abc  "); !res.OK || res.Value != "abc" {
		t.Errorf("no recortó la clave: %+v", res)
	}
	if ValidarAPIKey("   ").OK {
		t.Error("aceptó una clave vacía")
	}
}

// ── El servicio ────────────────────────────────────────────────────────────

type clienteFalso struct {
	pingFalla string
	workflows []Workflow
	pings     int
	activado  *bool
	ultimoID  string
}

func (c *clienteFalso) Ping(context.Context, Credenciales) kernel.Result[struct{}] {
	c.pings++
	if c.pingFalla != "" {
		return kernel.Err[struct{}](c.pingFalla)
	}
	return kernel.Ok(struct{}{})
}

func (c *clienteFalso) ListarWorkflows(context.Context, Credenciales) kernel.Result[[]Workflow] {
	return kernel.Ok(c.workflows)
}

func (c *clienteFalso) Activar(_ context.Context, _ Credenciales, id string, activo bool) kernel.Result[struct{}] {
	c.ultimoID = id
	c.activado = &activo
	return kernel.Ok(struct{}{})
}

type repoFalso struct {
	activa   *Config
	guardado *Config
	fallo    error
}

func (r *repoFalso) GetActive(context.Context) (*Config, error) {
	if r.fallo != nil {
		return nil, r.fallo
	}
	return r.activa, nil
}

func (r *repoFalso) Upsert(_ context.Context, baseURL, apiKey string) (*Config, error) {
	if r.fallo != nil {
		return nil, r.fallo
	}
	r.guardado = &Config{ID: "c1", BaseURL: baseURL, APIKey: apiKey, Active: true}
	return r.guardado, nil
}

func (r *repoFalso) Remove(context.Context, string) error { return r.fallo }

// Lo importante: no se guarda una credencial que no funciona. Si se guardara,
// el error aparecería más tarde y en otro sitio.
func TestNoSeGuardaUnaCredencialQueNoFunciona(t *testing.T) {
	cliente := &clienteFalso{pingFalla: "El API key no vale o no tiene permisos."}
	repo := &repoFalso{}

	res, err := NewService(cliente, repo).Conectar(context.Background(), "https://n8n.test", "mala")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if res.OK {
		t.Error("dio la conexión por buena")
	}
	if repo.guardado != nil {
		t.Error("guardó una credencial que no funciona")
	}
}

func TestConectarGuardaLoNormalizado(t *testing.T) {
	cliente := &clienteFalso{}
	repo := &repoFalso{}

	res, err := NewService(cliente, repo).Conectar(context.Background(), "  https://n8n.test/  ", "  clave  ")
	if err != nil || !res.OK {
		t.Fatalf("falló: err=%v res=%+v", err, res)
	}
	if repo.guardado.BaseURL != "https://n8n.test" {
		t.Errorf("guardó la URL sin normalizar: %q", repo.guardado.BaseURL)
	}
	if repo.guardado.APIKey != "clave" {
		t.Errorf("guardó la clave sin recortar: %q", repo.guardado.APIKey)
	}
}

// Una URL inválida no debe llegar a provocar una petición de red.
func TestURLInvalidaNoHacePing(t *testing.T) {
	cliente := &clienteFalso{}
	_, _ = NewService(cliente, &repoFalso{}).Conectar(context.Background(), "no-es-url", "clave")
	if cliente.pings != 0 {
		t.Errorf("hizo %d pings; no debía hacer ninguno", cliente.pings)
	}
}

func TestSinConexionSeExplicaQueHayQueConectar(t *testing.T) {
	svc := NewService(&clienteFalso{}, &repoFalso{activa: nil})

	res, err := svc.ListarWorkflows(context.Background())
	if err != nil {
		t.Fatalf("debía ser error de negocio: %v", err)
	}
	if res.OK {
		t.Fatal("listó flujos sin conexión configurada")
	}
	if res.Error == "" {
		t.Error("no explicó qué hacer")
	}
}

func TestActivarPasaElIDYElEstado(t *testing.T) {
	cliente := &clienteFalso{}
	repo := &repoFalso{activa: &Config{ID: "c1", BaseURL: "https://n8n.test", APIKey: "k"}}

	if _, err := NewService(cliente, repo).Activar(context.Background(), "wf-7", false); err != nil {
		t.Fatalf("falló: %v", err)
	}
	if cliente.ultimoID != "wf-7" {
		t.Errorf("mandó el id %q", cliente.ultimoID)
	}
	if cliente.activado == nil || *cliente.activado {
		t.Error("no pasó el estado desactivado")
	}
}

func TestFalloDeBaseSubeComoError(t *testing.T) {
	repo := &repoFalso{fallo: errors.New("sin conexión")}
	if _, err := NewService(&clienteFalso{}, repo).ListarWorkflows(context.Background()); err == nil {
		t.Error("se tragó un fallo de base")
	}
}

// ── La respuesta de n8n ────────────────────────────────────────────────────

// El id llega unas veces como texto y otras como número según la versión.
func TestElIDNumericoSeNormalizaATexto(t *testing.T) {
	salida := convertir([]workflowCrudo{
		{ID: "abc", Name: "Uno", Active: true},
		{ID: float64(42), Name: "Dos"},
		{ID: nil, Name: "   "},
	})

	if salida[0].ID != "abc" {
		t.Errorf("id de texto = %q", salida[0].ID)
	}
	if salida[1].ID != "42" {
		t.Errorf("id numérico = %q, quería \"42\"", salida[1].ID)
	}
	if salida[2].Name != "(sin nombre)" {
		t.Errorf("un flujo sin nombre debería decirlo: %q", salida[2].Name)
	}
}

func TestExplicarDaInstruccionesSegunElEstado(t *testing.T) {
	if msg := explicar(401, nil); msg == "" {
		t.Error("401 sin mensaje")
	}
	if msg := explicar(403, nil); msg != explicar(401, nil) {
		t.Error("401 y 403 son el mismo problema para quien lo lee")
	}
	if msg := explicar(500, []byte(`{"message":"se rompió algo"}`)); msg != "n8n: se rompió algo" {
		t.Errorf("no usó el mensaje de n8n: %q", msg)
	}
	if msg := explicar(500, []byte("no es json")); msg == "" {
		t.Error("sin json debería dar igual un mensaje")
	}
}
