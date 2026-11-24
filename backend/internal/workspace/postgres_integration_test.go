package workspace

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Prueba contra una PostgreSQL de verdad. Se salta si no hay una: se activa con
//
//	WORKSPACE_TEST_DB="postgres://usuario@127.0.0.1:5432/base?sslmode=disable&search_path=lab"
//
// La base necesita una tabla usuarioslab(id, email, name) con las personas «ana»
// y «luis» (luis sin nombre, para comprobar que entonces sale su correo
// luis@x.com): de ahí saca el módulo los nombres.
func baseDePrueba(t *testing.T) *sqlx.DB {
	t.Helper()
	url := os.Getenv("WORKSPACE_TEST_DB")
	if url == "" {
		t.Skip("sin WORKSPACE_TEST_DB: se salta la prueba contra PostgreSQL")
	}
	db, err := sqlx.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrar(context.Background(), db); err != nil {
		t.Fatalf("migrar: %v", err)
	}
	// Dos veces seguidas: tiene que ser idempotente, porque corre en cada arranque.
	if err := Migrar(context.Background(), db); err != nil {
		t.Fatalf("migrar por segunda vez: %v", err)
	}
	_, _ = db.Exec(`DELETE FROM hl_workspace_item`)
	return db
}

func TestPostgres_ciclo(t *testing.T) {
	db := baseDePrueba(t)
	s := NewService(NewPostgresRepo(db))
	ctx := context.Background()
	ana := Quien{ID: "ana", Nombre: "Ana"}
	luis := Quien{ID: "luis", Nombre: "Luis"}
	// El nombre que debe salir es el que tenga la persona en la base.
	var nombreAna string
	if err := db.Get(&nombreAna, `SELECT COALESCE(NULLIF(name, ''), email) FROM usuarioslab WHERE id = 'ana'`); err != nil {
		t.Fatalf("la base de prueba necesita el usuario «ana» en usuarioslab: %v", err)
	}

	r1, err := s.SubirSkill(ctx, ana, zipDe(t, map[string]string{
		"mi-skill/SKILL.md": skillMD, "mi-skill/ref/a.md": "uno",
	}), "primera", Privado)
	if err != nil || !r1.OK {
		t.Fatalf("subir: %v %+v", err, r1)
	}
	if r1.Value.DuenoNombre != nombreAna || r1.Value.Archivos != 2 || r1.Value.Version != 1 {
		t.Errorf("elemento mal guardado: %+v", r1.Value)
	}

	// Nueva versión con otro contenido: la antigua se conserva.
	r2, _ := s.SubirSkill(ctx, ana, zipDe(t, map[string]string{"SKILL.md": skillMD, "ref/a.md": "dos"}), "segunda", Compartido)
	if !r2.OK || r2.Value.Version != 2 {
		t.Fatalf("segunda versión: %+v", r2)
	}
	viejo, err := s.Archivo(ctx, ana, r1.Value.ID, 1, "ref/a.md")
	if err != nil || string(viejo.Contenido) != "uno" {
		t.Errorf("la versión 1 debía seguir diciendo «uno»: %v %v", viejo, err)
	}
	actual, _ := s.Archivo(ctx, ana, r1.Value.ID, 0, "ref/a.md")
	if string(actual.Contenido) != "dos" {
		t.Errorf("la actual debía decir «dos»: %q", actual.Contenido)
	}

	// La visibilidad no cambia con una nueva versión (la fija quien la cambia).
	if lista, _ := s.Listar(ctx, luis, Skill); len(lista) != 0 {
		t.Errorf("sigue privada: Luis no la ve, pero ve %d", len(lista))
	}
	if err := s.CambiarVisibilidad(ctx, ana, r1.Value.ID, Compartido); err != nil {
		t.Fatal(err)
	}
	lista, _ := s.Listar(ctx, luis, Skill)
	if len(lista) != 1 {
		t.Fatalf("compartida: Luis debe verla: %d", len(lista))
	}

	// Una persona sin nombre usa su correo.
	d, _ := s.GuardarDoc(ctx, luis, "n/a.md", "# hola", "", Privado)
	if d.Value.DuenoNombre != "luis@x.com" {
		t.Errorf("sin nombre debía salir el correo: %q", d.Value.DuenoNombre)
	}

	vs, _ := s.Detalle(ctx, ana, r1.Value.ID)
	if len(vs.Versiones) != 2 || vs.Versiones[0].Numero != 2 || vs.Versiones[0].AutorNombre != nombreAna {
		t.Errorf("historial mal: %+v", vs.Versiones)
	}

	// Borrar arrastra versiones y archivos.
	if err := s.Borrar(ctx, ana, r1.Value.ID); err != nil {
		t.Fatal(err)
	}
	var restos int
	_ = db.Get(&restos, `SELECT (SELECT count(*) FROM hl_workspace_version WHERE item_id = $1) + (SELECT count(*) FROM hl_workspace_file WHERE item_id = $1)`, r1.Value.ID)
	if restos != 0 {
		t.Errorf("quedaron %d filas huérfanas", restos)
	}
}

// Dos guardados a la vez no pueden quedarse con el mismo número de versión.
func TestPostgres_versionesConcurrentes(t *testing.T) {
	db := baseDePrueba(t)
	s := NewService(NewPostgresRepo(db))
	ctx := context.Background()
	ana := Quien{ID: "ana", Nombre: "Ana"}
	r, _ := s.GuardarDoc(ctx, ana, "c.md", "inicio", "", Privado)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.GuardarDoc(ctx, ana, "c.md", "texto "+string(rune('a'+i)), "", Privado)
			if err != nil {
				t.Errorf("guardado concurrente %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	d, _ := s.Detalle(ctx, ana, r.Value.ID)
	if len(d.Versiones) != 9 {
		t.Errorf("debía haber 9 versiones (1 + 8), hay %d", len(d.Versiones))
	}
}

// Pasado el tope, las versiones más viejas se descartan (y sus archivos con ellas).
func TestPostgres_topeDeVersiones(t *testing.T) {
	db := baseDePrueba(t)
	s := NewService(NewPostgresRepo(db))
	ctx := context.Background()
	ana := Quien{ID: "ana", Nombre: "Ana"}

	var id string
	for i := 1; i <= MaxVersiones+5; i++ {
		r, err := s.GuardarDoc(ctx, ana, "tope.md", fmt.Sprintf("texto %d", i), "", Privado)
		if err != nil || !r.OK {
			t.Fatalf("guardado %d: %v %s", i, err, r.Error)
		}
		id = r.Value.ID
	}
	d, _ := s.Detalle(ctx, ana, id)
	if len(d.Versiones) != MaxVersiones {
		t.Fatalf("debían quedar %d versiones y hay %d", MaxVersiones, len(d.Versiones))
	}
	if d.Versiones[0].Numero != MaxVersiones+5 || d.Versiones[len(d.Versiones)-1].Numero != 6 {
		t.Errorf("deben quedar de la 6 a la %d: primera %d, última %d", MaxVersiones+5, d.Versiones[0].Numero, d.Versiones[len(d.Versiones)-1].Numero)
	}
	var huerfanos int
	_ = db.Get(&huerfanos, `SELECT count(*) FROM hl_workspace_file WHERE item_id = $1`, id)
	if huerfanos != MaxVersiones {
		t.Errorf("los archivos de las versiones descartadas deben irse con ellas: %d", huerfanos)
	}
	if a, _ := s.Archivo(ctx, ana, id, 0, "tope.md"); string(a.Contenido) != fmt.Sprintf("texto %d", MaxVersiones+5) {
		t.Errorf("la actual sigue intacta: %q", a.Contenido)
	}
}
