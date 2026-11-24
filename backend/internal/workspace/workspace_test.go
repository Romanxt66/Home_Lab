package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// zipDe arma un zip en memoria con los archivos dados (ruta → contenido).
func zipDe(t *testing.T, archivos map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for ruta, contenido := range archivos {
		w, err := zw.Create(ruta)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(contenido))
	}
	_ = zw.Close()
	return buf.Bytes()
}

const skillMD = "---\nname: mi-skill\ndescription: \"Hace algo útil\"\n---\n# Mi skill\n"

func TestLeerSkill_conCarpetaRaiz(t *testing.T) {
	r := LeerSkill(zipDe(t, map[string]string{
		"mi-skill/SKILL.md":       skillMD,
		"mi-skill/scripts/run.sh": "echo hola",
		"__MACOSX/mi-skill/._x":   "basura",
		"mi-skill/.DS_Store":      "basura",
	}))
	if !r.OK {
		t.Fatalf("debería leerse: %s", r.Error)
	}
	if r.Value.Nombre != "mi-skill" || r.Value.Descripcion != "Hace algo útil" {
		t.Errorf("cabecera mal leída: %+v", r.Value)
	}
	rutas := []string{}
	for _, a := range r.Value.Archivos {
		rutas = append(rutas, a.Ruta)
	}
	if strings.Join(rutas, ",") != "SKILL.md,scripts/run.sh" {
		t.Errorf("la carpeta raíz debía quitarse y la basura ignorarse: %v", rutas)
	}
}

func TestLeerSkill_enLaRaiz(t *testing.T) {
	r := LeerSkill(zipDe(t, map[string]string{"SKILL.md": skillMD}))
	if !r.OK || len(r.Value.Archivos) != 1 {
		t.Fatalf("una skill con SKILL.md en la raíz debe valer: %+v", r)
	}
}

func TestLeerSkill_rechazos(t *testing.T) {
	casos := map[string]struct {
		datos []byte
		texto string
	}{
		"vacío":           {nil, "vacío"},
		"no es zip":       {[]byte("esto no es un zip"), "No parece"},
		"sin SKILL.md":    {zipDe(t, map[string]string{"a/leeme.txt": "x"}), "SKILL.md"},
		"sin name":        {zipDe(t, map[string]string{"SKILL.md": "---\ndescription: x\n---\n"}), "name"},
		"sin description": {zipDe(t, map[string]string{"SKILL.md": "---\nname: ok\n---\n"}), "description"},
		"nombre malo":     {zipDe(t, map[string]string{"SKILL.md": "---\nname: Mi Skill!\ndescription: x\n---\n"}), "nombre de la skill"},
		"zip-slip":        {zipDe(t, map[string]string{"SKILL.md": skillMD, "../../etc/passwd": "x"}), "ruta no permitida"},
		"ruta absoluta":   {zipDe(t, map[string]string{"SKILL.md": skillMD, "/etc/passwd": "x"}), "ruta no permitida"},
	}
	for nombre, c := range casos {
		r := LeerSkill(c.datos)
		if r.OK {
			t.Errorf("%s: debía rechazarse", nombre)
			continue
		}
		if !strings.Contains(strings.ToLower(r.Error), strings.ToLower(c.texto)) {
			t.Errorf("%s: el mensaje %q no menciona %q", nombre, r.Error, c.texto)
		}
	}
}

func TestLeerSkill_archivoDemasiadoGrande(t *testing.T) {
	grande := strings.Repeat("a", MaxBytesArchivo+10)
	r := LeerSkill(zipDe(t, map[string]string{"SKILL.md": skillMD, "enorme.txt": grande}))
	if r.OK || !strings.Contains(r.Error, "5 MB") {
		t.Fatalf("un archivo de más de 5 MB debía rechazarse: %+v", r)
	}
}

// Una bomba: muchos archivos que por separado caben pero juntos pasan del total.
func TestLeerSkill_totalDemasiadoGrande(t *testing.T) {
	mapa := map[string]string{"SKILL.md": skillMD}
	trozo := strings.Repeat("b", 4<<20)
	for _, n := range []string{"a", "b", "c", "d"} {
		mapa[n+".bin"] = trozo
	}
	r := LeerSkill(zipDe(t, mapa))
	if r.OK || !strings.Contains(r.Error, "12 MB") {
		t.Fatalf("pasar del total debía rechazarse: %+v", r)
	}
}

func TestLimpiarRuta(t *testing.T) {
	buenas := map[string]string{"a/b.md": "a/b.md", "a\\b.md": "a/b.md", "./x.md": "x.md", "a//b.md": "a/b.md"}
	for entrada, esperada := range buenas {
		if got, ok := LimpiarRuta(entrada); !ok || got != esperada {
			t.Errorf("LimpiarRuta(%q) = %q,%v; quería %q", entrada, got, ok, esperada)
		}
	}
	for _, mala := range []string{"", "..", "../x", "a/../../x", "/x", "C:/x", "a\x00b"} {
		if _, ok := LimpiarRuta(mala); ok {
			t.Errorf("LimpiarRuta(%q) debía rechazarse", mala)
		}
	}
}

func TestValidarRutaDocumento(t *testing.T) {
	if !ValidarRutaDocumento("proyectos/homelab/CLAUDE.md").OK {
		t.Error("una ruta con carpetas y .md debe valer")
	}
	for _, mala := range []string{"notas.txt", "../x.md", ""} {
		if ValidarRutaDocumento(mala).OK {
			t.Errorf("%q debía rechazarse", mala)
		}
	}
}

func TestEmpaquetarYVolverALeer(t *testing.T) {
	zipBytes, err := EmpaquetarSkill("mi-skill", []Archivo{{Ruta: "SKILL.md", Contenido: []byte(skillMD)}, {Ruta: "ref/a.md", Contenido: []byte("x")}})
	if err != nil {
		t.Fatal(err)
	}
	r := LeerSkill(zipBytes)
	if !r.OK || len(r.Value.Archivos) != 2 || r.Value.Nombre != "mi-skill" {
		t.Fatalf("el zip descargado debe poder volver a subirse tal cual: %+v", r)
	}
}

func TestPermisos(t *testing.T) {
	privado := Elemento{DuenoID: "ana", Visibilidad: Privado}
	compartido := Elemento{DuenoID: "ana", Visibilidad: Compartido}
	if !privado.PuedeVer("ana") || privado.PuedeVer("luis") {
		t.Error("lo privado solo lo ve su dueño")
	}
	if !compartido.PuedeVer("luis") {
		t.Error("lo compartido lo ve cualquiera")
	}
	if compartido.PuedeEditar("luis") {
		t.Error("nadie edita lo ajeno, aunque esté compartido")
	}
	if compartido.PuedeEditar("admin") {
		t.Error("ni un administrador edita lo ajeno")
	}
	if !compartido.PuedeBorrar("luis", true) || compartido.PuedeBorrar("luis", false) {
		t.Error("borrar: el dueño o un administrador")
	}
}

func TestLeerCabecera(t *testing.T) {
	n, d := LeerCabecera("\xef\xbb\xbf---\r\nname: x\r\ndescription: 'con: dos puntos'\r\n---\r\ncuerpo")
	if n != "x" || d != "con: dos puntos" {
		t.Errorf("cabecera con BOM, CRLF y comillas mal leída: %q %q", n, d)
	}
	if n, _ := LeerCabecera("sin cabecera"); n != "" {
		t.Error("sin cabecera no hay nombre")
	}
}

// ── Servicio, con un repositorio en memoria ────────────────────────────────

type repoFalso struct {
	elementos map[string]*Elemento
	archivos  map[string]map[int][]Archivo
	versiones map[string]map[int]Version
	seq       int
}

func nuevoRepo() *repoFalso {
	return &repoFalso{elementos: map[string]*Elemento{}, archivos: map[string]map[int][]Archivo{}, versiones: map[string]map[int]Version{}}
}

func (r *repoFalso) Listar(_ context.Context, uid string, tipo Tipo) ([]Elemento, error) {
	var out []Elemento
	for _, e := range r.elementos {
		if e.Tipo == tipo && e.PuedeVer(uid) {
			out = append(out, *e)
		}
	}
	return out, nil
}
func (r *repoFalso) Obtener(_ context.Context, id string) (*Elemento, error) {
	if e, ok := r.elementos[id]; ok {
		c := *e
		return &c, nil
	}
	return nil, nil
}
func (r *repoFalso) BuscarPorNombre(_ context.Context, dueno string, tipo Tipo, nombre string) (*Elemento, error) {
	for _, e := range r.elementos {
		if e.DuenoID == dueno && e.Tipo == tipo && e.Nombre == nombre {
			c := *e
			return &c, nil
		}
	}
	return nil, nil
}
func (r *repoFalso) Crear(_ context.Context, e Elemento, v Version, a []Archivo) (*Elemento, error) {
	r.seq++
	e.ID = string(rune('a' + r.seq))
	// El repositorio real trae el nombre de la persona con un join.
	e.Version, e.Descripcion, e.DuenoNombre = 1, v.Descripcion, e.DuenoID
	r.elementos[e.ID] = &e
	r.archivos[e.ID] = map[int][]Archivo{1: a}
	v.Numero = 1
	r.versiones[e.ID] = map[int]Version{1: v}
	c := e
	return &c, nil
}
func (r *repoFalso) NuevaVersion(_ context.Context, id string, v Version, a []Archivo) (*Elemento, error) {
	e := r.elementos[id]
	e.Version++
	e.Descripcion = v.Descripcion
	r.archivos[id][e.Version] = a
	v.Numero = e.Version
	r.versiones[id][e.Version] = v
	c := *e
	return &c, nil
}
func (r *repoFalso) CambiarVisibilidad(_ context.Context, id string, v Visibilidad) error {
	r.elementos[id].Visibilidad = v
	return nil
}
func (r *repoFalso) Versiones(_ context.Context, id string) ([]Version, error) {
	var out []Version
	for _, v := range r.versiones[id] {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Numero > out[j].Numero })
	return out, nil
}
func (r *repoFalso) Archivos(_ context.Context, id string, version int) ([]Archivo, error) {
	e, ok := r.elementos[id]
	if !ok {
		return nil, nil
	}
	if version == 0 {
		version = e.Version
	}
	return r.archivos[id][version], nil
}
func (r *repoFalso) Borrar(_ context.Context, id string) error {
	delete(r.elementos, id)
	return nil
}

var (
	ana   = Quien{ID: "ana", Nombre: "Ana"}
	luis  = Quien{ID: "luis", Nombre: "Luis"}
	admin = Quien{ID: "root", Nombre: "Admin", Admin: true}
)

func TestSubirSkill_creaYLuegoVersiona(t *testing.T) {
	s := NewService(nuevoRepo())
	ctx := context.Background()
	z := zipDe(t, map[string]string{"SKILL.md": skillMD})

	r1, err := s.SubirSkill(ctx, ana, z, "", Privado)
	if err != nil || !r1.OK || r1.Value.Version != 1 || r1.Value.Visibilidad != Privado {
		t.Fatalf("primera subida: %+v %v", r1, err)
	}
	r2, _ := s.SubirSkill(ctx, ana, z, "ajustes", Compartido)
	if !r2.OK || r2.Value.Version != 2 || r2.Value.ID != r1.Value.ID {
		t.Fatalf("subir otra vez la misma skill añade una versión, no un duplicado: %+v", r2.Value)
	}
	// Otra persona con el mismo nombre de skill tiene la suya.
	r3, _ := s.SubirSkill(ctx, luis, z, "", Privado)
	if !r3.OK || r3.Value.ID == r1.Value.ID || r3.Value.Version != 1 {
		t.Errorf("la skill de Luis es independiente de la de Ana: %+v", r3.Value)
	}
}

func TestSubirSkill_zipMalo(t *testing.T) {
	s := NewService(nuevoRepo())
	r, err := s.SubirSkill(context.Background(), ana, []byte("nada"), "", Privado)
	if err != nil || r.OK {
		t.Fatalf("un zip malo es un error de negocio, no técnico: %+v %v", r, err)
	}
}

func TestGuardarDoc_noCreaVersionSiNoCambia(t *testing.T) {
	s := NewService(nuevoRepo())
	ctx := context.Background()
	r1, _ := s.GuardarDoc(ctx, ana, "proyectos/x/CLAUDE.md", "# Reglas\nuna", "", Privado)
	r2, _ := s.GuardarDoc(ctx, ana, "proyectos/x/CLAUDE.md", "# Reglas\nuna", "", Privado)
	if !r1.OK || !r2.OK || r2.Value.Version != 1 {
		t.Fatalf("guardar el mismo texto no debe crear versión: %+v", r2.Value)
	}
	r3, _ := s.GuardarDoc(ctx, ana, "proyectos/x/CLAUDE.md", "# Reglas\ndos", "cambio", Privado)
	if r3.Value.Version != 2 || r3.Value.Descripcion != "Reglas" {
		t.Errorf("un texto distinto sí: %+v", r3.Value)
	}
}

func TestPrivacidad(t *testing.T) {
	s := NewService(nuevoRepo())
	ctx := context.Background()
	r, _ := s.GuardarDoc(ctx, ana, "privado.md", "secreto", "", Privado)
	id := r.Value.ID

	if _, err := s.Detalle(ctx, luis, id); err != ErrNoEncontrado {
		t.Errorf("lo privado de Ana no existe para Luis: %v", err)
	}
	if _, err := s.Descargar(ctx, luis, id, 0); err != ErrNoEncontrado {
		t.Errorf("tampoco se descarga: %v", err)
	}
	if lista, _ := s.Listar(ctx, luis, Doc); len(lista) != 0 {
		t.Error("el listado de Luis no incluye lo privado de Ana")
	}

	if err := s.CambiarVisibilidad(ctx, ana, id, Compartido); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Detalle(ctx, luis, id); err != nil {
		t.Errorf("compartido, Luis lo ve: %v", err)
	}
	if err := s.CambiarVisibilidad(ctx, luis, id, Privado); err != ErrSinPermiso {
		t.Errorf("Luis no cambia la visibilidad de Ana: %v", err)
	}
	if err := s.Borrar(ctx, luis, id); err != ErrSinPermiso {
		t.Errorf("Luis no borra lo de Ana: %v", err)
	}
	if err := s.Borrar(ctx, admin, id); err != nil {
		t.Errorf("un administrador sí puede retirarlo: %v", err)
	}
}

func TestDescargarTodo(t *testing.T) {
	s := NewService(nuevoRepo())
	ctx := context.Background()
	_, _ = s.SubirSkill(ctx, ana, zipDe(t, map[string]string{"SKILL.md": skillMD}), "", Privado)
	_, _ = s.GuardarDoc(ctx, ana, "notas/a.md", "# A", "", Privado)
	_, _ = s.SubirSkill(ctx, luis, zipDe(t, map[string]string{"SKILL.md": skillMD}), "", Compartido)

	d, err := s.DescargarTodo(ctx, ana, Todo)
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(d.Contenido), int64(len(d.Contenido)))
	nombres := map[string]bool{}
	for _, f := range zr.File {
		nombres[f.Name] = true
	}
	for _, esperado := range []string{"skills/mi-skill/SKILL.md", "docs/notas/a.md", "skills/compartido/luis/mi-skill/SKILL.md"} {
		if !nombres[esperado] {
			t.Errorf("falta %s en %v", esperado, nombres)
		}
	}

	if _, err := NewService(nuevoRepo()).DescargarTodo(ctx, ana, LoMio); err != ErrNoEncontrado {
		t.Errorf("sin nada que bajar: %v", err)
	}
}

func TestDescripcionDeDoc(t *testing.T) {
	if got := DescripcionDeDoc("\n\n## Reglas del repo\ntexto"); got != "Reglas del repo" {
		t.Errorf("quería el primer título, salió %q", got)
	}
}

func TestTopeDeElementos(t *testing.T) {
	s := NewService(nuevoRepo())
	ctx := context.Background()
	for i := 0; i < MaxElementos; i++ {
		r, err := s.GuardarDoc(ctx, ana, fmt.Sprintf("d/%d.md", i), "# x", "", Privado)
		if err != nil || !r.OK {
			t.Fatalf("el documento %d debía poder crearse: %v %s", i, err, r.Error)
		}
	}
	if r, _ := s.GuardarDoc(ctx, ana, "d/uno-mas.md", "# x", "", Privado); r.OK || !strings.Contains(r.Error, "300") {
		t.Errorf("el 301 debía rechazarse: %+v", r)
	}
	// Seguir editando uno existente sí se puede, y el tope es por persona.
	if r, _ := s.GuardarDoc(ctx, ana, "d/0.md", "# cambiado", "", Privado); !r.OK {
		t.Error("editar uno que ya existe no cuenta como crear")
	}
	if r, _ := s.GuardarDoc(ctx, luis, "d/0.md", "# x", "", Privado); !r.OK {
		t.Error("el tope es por persona")
	}
	// Lo ajeno compartido no cuenta contra el tope de quien lo ve.
	if r, _ := s.SubirSkill(ctx, ana, zipDe(t, map[string]string{"SKILL.md": skillMD}), "", Privado); !r.OK {
		t.Error("el tope es por tipo: las skills tienen el suyo")
	}
}
