package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// ErrNoEncontrado se devuelve tanto si no existe como si no se puede ver: así
// nadie averigua qué hay en el espacio privado de otra persona.
var ErrNoEncontrado = errors.New("workspace: no encontrado")

// ErrSinPermiso: existe y se ve, pero no es suyo para modificarlo.
var ErrSinPermiso = errors.New("workspace: sin permiso")

// Repo guarda elementos, versiones y archivos.
type Repo interface {
	// Listar devuelve lo que `uid` puede ver: lo suyo y lo compartido.
	Listar(ctx context.Context, uid string, tipo Tipo) ([]Elemento, error)
	Obtener(ctx context.Context, id string) (*Elemento, error)
	BuscarPorNombre(ctx context.Context, duenoID string, tipo Tipo, nombre string) (*Elemento, error)
	// Crear guarda el elemento con su primera versión.
	Crear(ctx context.Context, e Elemento, v Version, archivos []Archivo) (*Elemento, error)
	// NuevaVersion añade una versión y actualiza lo que el listado enseña.
	NuevaVersion(ctx context.Context, id string, v Version, archivos []Archivo) (*Elemento, error)
	CambiarVisibilidad(ctx context.Context, id string, v Visibilidad) error
	Versiones(ctx context.Context, id string) ([]Version, error)
	// Archivos devuelve los de una versión; 0 es la actual.
	Archivos(ctx context.Context, id string, version int) ([]Archivo, error)
	Borrar(ctx context.Context, id string) error
}

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service { return &Service{repo: repo} }

// Quien identifica a la persona que actúa.
type Quien struct {
	ID     string
	Nombre string
	Admin  bool
}

func (s *Service) Listar(ctx context.Context, q Quien, tipo Tipo) ([]Elemento, error) {
	return s.repo.Listar(ctx, q.ID, tipo)
}

// SubirSkill valida el zip y lo guarda: como skill nueva o como versión nueva
// si esa persona ya tiene una con el mismo nombre.
func (s *Service) SubirSkill(ctx context.Context, q Quien, datos []byte, mensaje string, vis Visibilidad) (kernel.Result[Elemento], error) {
	leida := LeerSkill(datos)
	if !leida.OK {
		return kernel.Err[Elemento](leida.Error), nil
	}
	sk := leida.Value
	mensaje = limpiarMensaje(mensaje, "Subida desde un zip")

	existente, err := s.repo.BuscarPorNombre(ctx, q.ID, Skill, sk.Nombre)
	if err != nil {
		return kernel.Result[Elemento]{}, err
	}
	v := Version{Mensaje: mensaje, AutorID: q.ID, Descripcion: sk.Descripcion, Archivos: len(sk.Archivos), Bytes: sk.Bytes}
	if existente != nil {
		e, err := s.repo.NuevaVersion(ctx, existente.ID, v, sk.Archivos)
		if err != nil {
			return kernel.Result[Elemento]{}, err
		}
		return kernel.Ok(*e), nil
	}
	if msg := s.comprobarTope(ctx, q, Skill); msg != "" {
		return kernel.Err[Elemento](msg), nil
	}
	e, err := s.repo.Crear(ctx, Elemento{
		Tipo: Skill, DuenoID: q.ID, Nombre: sk.Nombre, Descripcion: sk.Descripcion,
		Visibilidad: normalizarVisibilidad(vis),
	}, v, sk.Archivos)
	if err != nil {
		return kernel.Result[Elemento]{}, err
	}
	return kernel.Ok(*e), nil
}

// GuardarDoc crea el documento o, si la persona ya tiene uno en esa ruta, le
// añade una versión. Si el texto no cambió no se crea una versión vacía.
func (s *Service) GuardarDoc(ctx context.Context, q Quien, ruta, contenido, mensaje string, vis Visibilidad) (kernel.Result[Elemento], error) {
	r := ValidarRutaDocumento(ruta)
	if !r.OK {
		return kernel.Err[Elemento](r.Error), nil
	}
	c := ValidarDocumento(contenido)
	if !c.OK {
		return kernel.Err[Elemento](c.Error), nil
	}
	archivos := []Archivo{{Ruta: r.Value, Contenido: []byte(c.Value)}}
	v := Version{
		Mensaje: limpiarMensaje(mensaje, "Cambio"), AutorID: q.ID,
		Descripcion: DescripcionDeDoc(c.Value), Archivos: 1, Bytes: int64(len(c.Value)),
	}

	existente, err := s.repo.BuscarPorNombre(ctx, q.ID, Doc, r.Value)
	if err != nil {
		return kernel.Result[Elemento]{}, err
	}
	if existente == nil {
		if msg := s.comprobarTope(ctx, q, Doc); msg != "" {
			return kernel.Err[Elemento](msg), nil
		}
		e, err := s.repo.Crear(ctx, Elemento{
			Tipo: Doc, DuenoID: q.ID, Nombre: r.Value, Descripcion: v.Descripcion,
			Visibilidad: normalizarVisibilidad(vis),
		}, v, archivos)
		if err != nil {
			return kernel.Result[Elemento]{}, err
		}
		return kernel.Ok(*e), nil
	}

	actual, err := s.repo.Archivos(ctx, existente.ID, 0)
	if err != nil {
		return kernel.Result[Elemento]{}, err
	}
	if len(actual) == 1 && bytes.Equal(actual[0].Contenido, archivos[0].Contenido) {
		return kernel.Ok(*existente), nil
	}
	e, err := s.repo.NuevaVersion(ctx, existente.ID, v, archivos)
	if err != nil {
		return kernel.Result[Elemento]{}, err
	}
	return kernel.Ok(*e), nil
}

// Detalle es lo que enseña la pantalla de un elemento.
type Detalle struct {
	Elemento  Elemento
	Archivos  []ArchivoResumen
	Versiones []Version
}

type ArchivoResumen struct {
	Ruta  string
	Bytes int
	// Texto: se puede previsualizar.
	Texto bool
}

func (s *Service) Detalle(ctx context.Context, q Quien, id string) (*Detalle, error) {
	e, err := s.visible(ctx, q, id)
	if err != nil {
		return nil, err
	}
	archivos, err := s.repo.Archivos(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	versiones, err := s.repo.Versiones(ctx, id)
	if err != nil {
		return nil, err
	}
	res := make([]ArchivoResumen, 0, len(archivos))
	for _, a := range archivos {
		res = append(res, ArchivoResumen{Ruta: a.Ruta, Bytes: len(a.Contenido), Texto: EsTexto(a.Contenido)})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Ruta < res[j].Ruta })
	return &Detalle{Elemento: *e, Archivos: res, Versiones: versiones}, nil
}

// Archivo devuelve un archivo de una versión (0: la actual).
func (s *Service) Archivo(ctx context.Context, q Quien, id string, version int, ruta string) (*Archivo, error) {
	if _, err := s.visible(ctx, q, id); err != nil {
		return nil, err
	}
	archivos, err := s.repo.Archivos(ctx, id, version)
	if err != nil {
		return nil, err
	}
	for i := range archivos {
		if archivos[i].Ruta == ruta {
			return &archivos[i], nil
		}
	}
	return nil, ErrNoEncontrado
}

// Descarga es un archivo listo para bajar.
type Descarga struct {
	Nombre    string
	Tipo      string
	Contenido []byte
}

// Descargar: una skill como zip con su carpeta, un documento como .md.
func (s *Service) Descargar(ctx context.Context, q Quien, id string, version int) (*Descarga, error) {
	e, err := s.visible(ctx, q, id)
	if err != nil {
		return nil, err
	}
	archivos, err := s.repo.Archivos(ctx, id, version)
	if err != nil {
		return nil, err
	}
	if len(archivos) == 0 {
		return nil, ErrNoEncontrado
	}
	if e.Tipo == Doc {
		base := e.Nombre[strings.LastIndex(e.Nombre, "/")+1:]
		return &Descarga{Nombre: base, Tipo: "text/markdown; charset=utf-8", Contenido: archivos[0].Contenido}, nil
	}
	zip, err := EmpaquetarSkill(e.Nombre, archivos)
	if err != nil {
		return nil, err
	}
	return &Descarga{Nombre: e.Nombre + ".skill", Tipo: "application/zip", Contenido: zip}, nil
}

// Alcance de una descarga masiva.
type Alcance string

const (
	LoMio        Alcance = "mine"
	LoCompartido Alcance = "shared"
	Todo         Alcance = "all"
)

// DescargarTodo arma un zip con skills/<nombre>/… y docs/<ruta>.md, listo para
// copiar a ~/.claude (las skills a skills/, los documentos donde se quiera).
func (s *Service) DescargarTodo(ctx context.Context, q Quien, alcance Alcance) (*Descarga, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	n := 0
	for _, tipo := range []Tipo{Skill, Doc} {
		lista, err := s.repo.Listar(ctx, q.ID, tipo)
		if err != nil {
			return nil, err
		}
		for _, e := range lista {
			mia := e.DuenoID == q.ID
			if (alcance == LoMio && !mia) || (alcance == LoCompartido && mia) {
				continue
			}
			archivos, err := s.repo.Archivos(ctx, e.ID, 0)
			if err != nil {
				return nil, err
			}
			for _, a := range archivos {
				// Lo compartido por otras personas va en su propia carpeta, para
				// que dos «redactar» de dueños distintos no se pisen.
				prefijo := ""
				if !mia {
					prefijo = "compartido/" + carpetaSegura(e.DuenoNombre) + "/"
				}
				var ruta string
				if e.Tipo == Skill {
					ruta = "skills/" + prefijo + e.Nombre + "/" + a.Ruta
				} else {
					ruta = "docs/" + prefijo + a.Ruta
				}
				w, err := zw.Create(ruta)
				if err != nil {
					return nil, err
				}
				if _, err := w.Write(a.Contenido); err != nil {
					return nil, err
				}
				n++
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNoEncontrado
	}
	return &Descarga{Nombre: "espacio-de-trabajo.zip", Tipo: "application/zip", Contenido: buf.Bytes()}, nil
}

func (s *Service) CambiarVisibilidad(ctx context.Context, q Quien, id string, v Visibilidad) error {
	e, err := s.visible(ctx, q, id)
	if err != nil {
		return err
	}
	if !e.PuedeEditar(q.ID) {
		return ErrSinPermiso
	}
	return s.repo.CambiarVisibilidad(ctx, id, normalizarVisibilidad(v))
}

func (s *Service) Borrar(ctx context.Context, q Quien, id string) error {
	e, err := s.visible(ctx, q, id)
	if err != nil {
		return err
	}
	if !e.PuedeBorrar(q.ID, q.Admin) {
		return ErrSinPermiso
	}
	return s.repo.Borrar(ctx, id)
}

// visible trae el elemento si esa persona puede verlo; si no, «no existe».
func (s *Service) visible(ctx context.Context, q Quien, id string) (*Elemento, error) {
	e, err := s.repo.Obtener(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil || !e.PuedeVer(q.ID) {
		return nil, ErrNoEncontrado
	}
	return e, nil
}

func normalizarVisibilidad(v Visibilidad) Visibilidad {
	if v == Compartido {
		return Compartido
	}
	return Privado
}

func limpiarMensaje(m, porDefecto string) string {
	m = strings.TrimSpace(m)
	if m == "" {
		return porDefecto
	}
	return recortar(m, MaxLongitudMensaje)
}

// DescripcionDeDoc: el primer título o la primera línea con texto.
func DescripcionDeDoc(md string) string {
	for _, linea := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		l := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(linea), "#"))
		if l != "" && l != "---" {
			return recortar(l, 140)
		}
	}
	return ""
}

// EsTexto: sin bytes nulos y con UTF-8 válido, se puede enseñar como texto.
func EsTexto(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	return utf8Valido(b)
}

func carpetaSegura(nombre string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(nombre) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			sb.WriteRune('-')
		}
	}
	if sb.Len() == 0 {
		return "persona"
	}
	return sb.String()
}

// Buscar encuentra un elemento por su id o por su nombre entre lo que esa
// persona ve. Con compartido=false solo mira lo suyo; con true, también lo que
// otras personas comparten (y en un empate gana lo propio).
func (s *Service) Buscar(ctx context.Context, q Quien, tipo Tipo, ref string, compartido bool) (*Elemento, error) {
	lista, err := s.repo.Listar(ctx, q.ID, tipo)
	if err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	var ajeno *Elemento
	for i := range lista {
		e := lista[i]
		if e.ID != ref && e.Nombre != ref {
			continue
		}
		if e.DuenoID == q.ID {
			return &e, nil
		}
		if compartido && ajeno == nil {
			ajeno = &e
		}
	}
	if ajeno == nil {
		return nil, ErrNoEncontrado
	}
	return ajeno, nil
}

// comprobarTope devuelve un mensaje si la persona ya tiene el máximo de
// elementos de ese tipo; vacío si puede crear otro.
func (s *Service) comprobarTope(ctx context.Context, q Quien, tipo Tipo) string {
	lista, err := s.repo.Listar(ctx, q.ID, tipo)
	if err != nil {
		return "" // no se bloquea al usuario por no poder contar; el repositorio ya fallará si hay un problema real
	}
	propios := 0
	for _, e := range lista {
		if e.DuenoID == q.ID {
			propios++
		}
	}
	if propios >= MaxElementos {
		return "Has llegado al máximo de 300 elementos de este tipo. Elimina alguno que no uses antes de crear otro."
	}
	return ""
}
