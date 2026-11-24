package oauth

import (
	"context"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Contra una PostgreSQL de verdad; se salta sin OAUTH_TEST_DB. Necesita una tabla
// usuarioslab(id, email, name, role, status) con «ana» aprobada y «pend» sin aprobar.
// Vacía las tablas hl_oauth_*.
func baseDePrueba(t *testing.T) (*sqlx.DB, *Service) {
	t.Helper()
	dsn := os.Getenv("OAUTH_TEST_DB")
	if dsn == "" {
		t.Skip("sin OAUTH_TEST_DB: se salta la prueba contra PostgreSQL")
	}
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if err := Migrar(ctx, db); err != nil || Migrar(ctx, db) != nil {
		t.Fatalf("migrar (dos veces): %v", err)
	}
	for _, tabla := range []string{"hl_oauth_token", "hl_oauth_grant", "hl_oauth_code", "hl_oauth_client"} {
		_, _ = db.Exec(`DELETE FROM ` + tabla)
	}
	return db, NewService(NewPostgresRepo(db), []string{"claude.ai"})
}

func autorizarPG(t *testing.T, s *Service, uid string, permiso Alcance) (cliente, code string) {
	t.Helper()
	ctx := context.Background()
	c, e := s.Registrar(ctx, RegistroEntrada{Nombre: "Claude", RedirectURIs: []string{callback}})
	if e != nil {
		t.Fatal(e)
	}
	p := Peticion{TipoRespuesta: "code", ClienteID: c.ID, RedirectURI: callback, Challenge: challenge, Metodo: "S256", Estado: "s"}
	destino, e := s.Aprobar(ctx, uid, p, origen+"/mcp", true, permiso, origen)
	if e != nil {
		t.Fatal(e)
	}
	u, _ := url.Parse(destino)
	return c.ID, u.Query().Get("code")
}

func canje(cliente, code string) TokenEntrada {
	return TokenEntrada{GrantType: "authorization_code", Codigo: code, RedirectURI: callback, ClienteID: cliente, Verifier: verifier}
}

func TestPG_flujo(t *testing.T) {
	db, s := baseDePrueba(t)
	ctx := context.Background()
	cliente, code := autorizarPG(t, s, "ana", Escritura)

	tk, e := s.Canjear(ctx, canje(cliente, code), origen+"/mcp")
	if e != nil {
		t.Fatal(e)
	}
	id, err := s.Autenticar(ctx, tk.Acceso)
	if err != nil || id == nil || id.UsuarioID != "ana" || !id.Alcance.PuedeEscribir() || id.Nombre == "" {
		t.Fatalf("autenticar: %+v %v", id, err)
	}

	// Los secretos no están en la base: solo sus huellas.
	var n int
	_ = db.Get(&n, `SELECT count(*) FROM hl_oauth_token WHERE hash IN ($1,$2)`, tk.Acceso, tk.Refresco)
	if n != 0 {
		t.Error("los tokens no se guardan en claro")
	}
	_ = db.Get(&n, `SELECT count(*) FROM hl_oauth_token WHERE hash IN ($1,$2)`, Huella(tk.Acceso), Huella(tk.Refresco))
	if n != 2 {
		t.Errorf("sí sus huellas: %d", n)
	}
	_ = db.Get(&n, `SELECT count(*) FROM hl_oauth_code WHERE hash = $1`, code)
	if n != 0 {
		t.Error("tampoco el código en claro")
	}

	// Renovar rota los tokens.
	tk2, e := s.Canjear(ctx, TokenEntrada{GrantType: "refresh_token", Refresco: tk.Refresco, ClienteID: cliente}, "")
	if e != nil || tk2.Acceso == tk.Acceso {
		t.Fatalf("refrescar: %+v %v", tk2, e)
	}

	// Listado y revocación.
	lista, _ := s.Concesiones(ctx, "ana")
	if len(lista) != 1 || lista[0].ClienteNombre != "Claude" || lista[0].Alcance != Escritura || lista[0].UsadaEn == nil {
		t.Errorf("concesiones: %+v", lista)
	}
	if otra, _ := s.Concesiones(ctx, "luis"); len(otra) != 0 {
		t.Error("cada persona ve las suyas")
	}
	if hecho, _ := s.RevocarConcesion(ctx, "luis", lista[0].ID); hecho {
		t.Error("otra persona no revoca")
	}
	if hecho, _ := s.RevocarConcesion(ctx, "ana", "no-es-uuid"); hecho {
		t.Error("un id raro no rompe nada")
	}
	if hecho, _ := s.RevocarConcesion(ctx, "ana", lista[0].ID); !hecho {
		t.Fatal("el dueño revoca")
	}
	if id, _ := s.Autenticar(ctx, tk2.Acceso); id != nil {
		t.Error("revocado, el acceso deja de valer")
	}
	if lista, _ := s.Concesiones(ctx, "ana"); len(lista) != 0 {
		t.Error("una concesión revocada no se lista")
	}
}

// Dos canjes a la vez del mismo código: solo uno puede ganar.
func TestPG_codigoUnSoloUso(t *testing.T) {
	_, s := baseDePrueba(t)
	cliente, code := autorizarPG(t, s, "ana", Lectura)

	var wg sync.WaitGroup
	var mu sync.Mutex
	exitos := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Canjear(context.Background(), canje(cliente, code), origen+"/mcp"); e == nil {
				mu.Lock()
				exitos++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if exitos != 1 {
		t.Errorf("de 10 canjes simultáneos debía ganar exactamente 1 y ganaron %d", exitos)
	}
}

// Dos refrescos a la vez con el mismo token: uno rota y el otro se trata como reutilización.
func TestPG_refrescoSimultaneoYReutilizacion(t *testing.T) {
	_, s := baseDePrueba(t)
	ctx := context.Background()
	cliente, code := autorizarPG(t, s, "ana", Escritura)
	tk, _ := s.Canjear(ctx, canje(cliente, code), origen+"/mcp")

	var wg sync.WaitGroup
	var mu sync.Mutex
	var obtenidos []*Tokens
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if n, e := s.Canjear(ctx, TokenEntrada{GrantType: "refresh_token", Refresco: tk.Refresco, ClienteID: cliente}, ""); e == nil {
				mu.Lock()
				obtenidos = append(obtenidos, n)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(obtenidos) != 1 {
		t.Fatalf("de 6 refrescos simultáneos debía rotar exactamente 1 y rotaron %d", len(obtenidos))
	}
	// Los perdedores dispararon la detección de reutilización: la concesión entera quedó revocada.
	if id, _ := s.Autenticar(ctx, obtenidos[0].Acceso); id != nil {
		t.Error("tras la reutilización, ni lo emitido al ganador sigue valiendo")
	}
	if _, e := s.Canjear(ctx, TokenEntrada{GrantType: "refresh_token", Refresco: obtenidos[0].Refresco, ClienteID: cliente}, ""); e == nil {
		t.Error("ni su refresco")
	}
}

func TestPG_cuentaSinAprobarNoEntra(t *testing.T) {
	_, s := baseDePrueba(t)
	ctx := context.Background()
	cliente, code := autorizarPG(t, s, "pend", Lectura)
	tk, e := s.Canjear(ctx, canje(cliente, code), origen+"/mcp")
	if e != nil {
		t.Fatal(e)
	}
	if id, _ := s.Autenticar(ctx, tk.Acceso); id != nil {
		t.Error("una cuenta sin aprobar no entra aunque tenga token")
	}
	if _, e := s.Canjear(ctx, TokenEntrada{GrantType: "refresh_token", Refresco: tk.Refresco, ClienteID: cliente}, ""); e == nil {
		t.Error("ni puede renovarlo")
	}
}

func TestPG_caducidades(t *testing.T) {
	_, s := baseDePrueba(t)
	ctx := context.Background()
	ahora := time.Now().UTC()
	s.ahora = func() time.Time { return ahora }
	cliente, code := autorizarPG(t, s, "ana", Lectura)

	s.ahora = func() time.Time { return ahora.Add(6 * time.Minute) }
	if _, e := s.Canjear(ctx, canje(cliente, code), origen+"/mcp"); e == nil {
		t.Error("un código caduca a los 5 minutos")
	}

	s.ahora = func() time.Time { return ahora }
	cliente, code = autorizarPG(t, s, "ana", Lectura)
	tk, _ := s.Canjear(ctx, canje(cliente, code), origen+"/mcp")
	s.ahora = func() time.Time { return ahora.Add(61 * time.Minute) }
	if id, _ := s.Autenticar(ctx, tk.Acceso); id != nil {
		t.Error("el acceso caduca a la hora")
	}
	s.ahora = func() time.Time { return ahora.Add(31 * 24 * time.Hour) }
	if _, e := s.Canjear(ctx, TokenEntrada{GrantType: "refresh_token", Refresco: tk.Refresco, ClienteID: cliente}, ""); e == nil {
		t.Error("el refresco caduca a los 30 días")
	}
}

func TestPG_limpiezaDeClientesSinUso(t *testing.T) {
	db, s := baseDePrueba(t)
	ctx := context.Background()
	viejo := time.Now().UTC().Add(-8 * 24 * time.Hour)
	for _, id := range []string{"sin-uso", "con-uso"} {
		_ = NewPostgresRepo(db).CrearCliente(ctx, Cliente{ID: id, Nombre: id, RedirectURIs: []string{callback}, CreadoEn: viejo})
	}
	_, _ = db.Exec(`INSERT INTO hl_oauth_grant (id, user_id, client_id, alcance, created_at) VALUES (gen_random_uuid(), 'ana', 'con-uso', 'read', now())`)
	if _, e := s.Registrar(ctx, RegistroEntrada{Nombre: "nuevo", RedirectURIs: []string{callback}}); e != nil {
		t.Fatal(e)
	}
	var ids []string
	_ = db.Select(&ids, `SELECT id FROM hl_oauth_client WHERE id IN ('sin-uso','con-uso') ORDER BY id`)
	if len(ids) != 1 || ids[0] != "con-uso" {
		t.Errorf("se borra el que nadie usó y se conserva el que tiene una concesión: %v", ids)
	}
}
