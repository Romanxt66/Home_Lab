package tokens

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Contra una PostgreSQL de verdad; se salta sin TOKENS_TEST_DB. Necesita una
// tabla usuarioslab(id, email, name, role, status) con «ana» aprobada y «pend»
// sin aprobar.
func TestPostgres_tokens(t *testing.T) {
	url := os.Getenv("TOKENS_TEST_DB")
	if url == "" {
		t.Skip("sin TOKENS_TEST_DB: se salta la prueba contra PostgreSQL")
	}
	db, err := sqlx.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := Migrar(ctx, db); err != nil || Migrar(ctx, db) != nil {
		t.Fatalf("migrar (dos veces): %v", err)
	}
	_, _ = db.Exec(`DELETE FROM hl_api_token`)

	s := NewService(NewPostgresRepo(db))
	res, err := s.Crear(ctx, "ana", "Prueba", "read")
	if err != nil || !res.OK {
		t.Fatalf("crear: %v %+v", err, res)
	}

	id, err := s.Autenticar(ctx, res.Value.Claro)
	if err != nil || id == nil || id.UsuarioID != "ana" || id.Alcance != Lectura || id.Nombre == "" {
		t.Fatalf("autenticar: %+v %v", id, err)
	}
	lista, _ := s.Listar(ctx, "ana")
	if len(lista) != 1 || lista[0].UsadoEn == nil || lista[0].Prefijo != res.Value.Token.Prefijo {
		t.Errorf("el uso debe quedar anotado: %+v", lista)
	}

	// El secreto no está en la base: ni en ninguna columna.
	var n int
	_ = db.Get(&n, `SELECT count(*) FROM hl_api_token WHERE token_hash = $1 OR nombre = $1 OR prefijo = $1`, res.Value.Claro)
	if n != 0 {
		t.Error("el secreto no debe estar guardado en claro")
	}

	// Cuenta sin aprobar: su token no entra.
	pend, _ := s.Crear(ctx, "pend", "x", "")
	if id, _ := s.Autenticar(ctx, pend.Value.Claro); id != nil {
		t.Error("una cuenta sin aprobar no entra con token")
	}

	// Un id raro no rompe nada y revocar es solo del dueño.
	if hecho, err := s.Revocar(ctx, "ana", "no-es-un-uuid"); hecho || err != nil {
		t.Errorf("id raro: %v %v", hecho, err)
	}
	if hecho, _ := s.Revocar(ctx, "luis", res.Value.Token.ID); hecho {
		t.Error("otra persona no revoca")
	}
	if hecho, _ := s.Revocar(ctx, "ana", res.Value.Token.ID); !hecho {
		t.Error("el dueño revoca")
	}
	if id, _ := s.Autenticar(ctx, res.Value.Claro); id != nil {
		t.Error("revocado, no entra")
	}
	if n, _ := NewPostgresRepo(db).ContarActivos(ctx, "ana"); n != 0 {
		t.Errorf("no quedan activos: %d", n)
	}
}
