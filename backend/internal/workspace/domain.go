// Package workspace es el espacio de trabajo: la biblioteca de skills de Claude
// y los documentos .md de instrucciones de cada persona, con su historial.
//
// Cada cosa es privada por defecto; su dueño puede compartirla para que el resto
// la vea y la descargue. Solo el dueño la edita. Una skill es texto que Claude
// obedece, así que nada ajeno llega a nadie sin que lo elija.
package workspace

import (
	"archive/zip"
	"bytes"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

// Tipo de elemento.
type Tipo string

const (
	Skill Tipo = "skill"
	Doc   Tipo = "doc"
)

// Visibilidad de un elemento.
type Visibilidad string

const (
	Privado    Visibilidad = "private"
	Compartido Visibilidad = "shared"
)

// Límites. Una skill con fuentes e imágenes pesa unos cientos de KB; esto deja
// holgura sin dejar que alguien llene la base de datos de golpe.
const (
	MaxArchivos        = 500
	MaxBytesArchivo    = 5 << 20  // un archivo
	MaxBytesTotal      = 12 << 20 // lo descomprimido de una skill
	MaxBytesZipSubido  = 12 << 20 // lo que se acepta subir
	MaxBytesDocumento  = 1 << 20  // un .md
	MaxLongitudCamino  = 200
	MaxLongitudMensaje = 200
	// MaxElementos por persona y tipo, y MaxVersiones por elemento. Un agente que
	// guarde en bucle (o una persona con un script) no debe poder llenar la base:
	// pasado el tope de versiones se descartan las más viejas.
	MaxElementos = 300
	MaxVersiones = 200
)

// Elemento es una skill o un documento. Su contenido vive en las versiones.
type Elemento struct {
	ID          string
	Tipo        Tipo
	DuenoID     string
	DuenoNombre string
	// Nombre es el identificador legible: el de la skill, o la ruta del documento.
	Nombre        string
	Descripcion   string
	Visibilidad   Visibilidad
	Version       int
	Archivos      int
	Bytes         int64
	CreadoEn      time.Time
	ActualizadoEn time.Time
}

// Archivo es un fichero dentro de una versión.
type Archivo struct {
	Ruta      string
	Contenido []byte
}

// Version es una foto del contenido en un momento.
type Version struct {
	Numero      int
	Mensaje     string
	AutorID     string
	AutorNombre string
	Descripcion string
	Archivos    int
	Bytes       int64
	CreadaEn    time.Time
}

// ── Permisos ───────────────────────────────────────────────────────────────

// PuedeVer: el dueño, y cualquiera si está compartido.
func (e Elemento) PuedeVer(uid string) bool {
	return e.DuenoID == uid || e.Visibilidad == Compartido
}

// PuedeEditar: solo el dueño. Nadie edita lo ajeno, ni siquiera un administrador:
// lo que Claude vaya a obedecer lo decide quien lo escribió.
func (e Elemento) PuedeEditar(uid string) bool { return e.DuenoID == uid }

// PuedeBorrar: el dueño, o un administrador (para retirar algo compartido que no debe estar).
func (e Elemento) PuedeBorrar(uid string, esAdmin bool) bool {
	return e.DuenoID == uid || esAdmin
}

// ── Nombres ────────────────────────────────────────────────────────────────

var reNombreSkill = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// ValidarNombreSkill: minúsculas, números, guiones; es el nombre de la carpeta
// que acabará en ~/.claude/skills.
func ValidarNombreSkill(n string) kernel.Result[string] {
	n = strings.TrimSpace(n)
	if !reNombreSkill.MatchString(n) {
		return kernel.Err[string]("El nombre de la skill solo puede llevar minúsculas, números, guiones y guiones bajos (máximo 64).")
	}
	return kernel.Ok(n)
}

// LimpiarRuta normaliza una ruta de archivo de un zip o de un documento y
// rechaza las que se salen de su carpeta, las absolutas y las raras.
func LimpiarRuta(r string) (string, bool) {
	r = strings.ReplaceAll(strings.TrimSpace(r), `\`, "/")
	if r == "" || len(r) > MaxLongitudCamino || strings.ContainsRune(r, 0) {
		return "", false
	}
	if strings.HasPrefix(r, "/") || strings.Contains(r, ":") {
		return "", false
	}
	limpia := path.Clean(r)
	if limpia == "." || limpia == ".." || strings.HasPrefix(limpia, "../") {
		return "", false
	}
	for _, parte := range strings.Split(limpia, "/") {
		if parte == "" || parte == ".." {
			return "", false
		}
	}
	return limpia, true
}

// ValidarRutaDocumento: una ruta con carpetas que acaba en .md.
func ValidarRutaDocumento(r string) kernel.Result[string] {
	limpia, ok := LimpiarRuta(r)
	if !ok {
		return kernel.Err[string]("La ruta del documento no es válida. Usa carpetas separadas por «/», por ejemplo proyectos/homelab/CLAUDE.md.")
	}
	if !strings.HasSuffix(strings.ToLower(limpia), ".md") {
		return kernel.Err[string]("El documento tiene que acabar en .md.")
	}
	return kernel.Ok(limpia)
}

// ValidarDocumento comprueba el texto de un .md.
func ValidarDocumento(contenido string) kernel.Result[string] {
	if len(contenido) > MaxBytesDocumento {
		return kernel.Err[string]("El documento es demasiado grande (máximo 1 MB).")
	}
	return kernel.Ok(contenido)
}

// ── Skills a partir de un zip ──────────────────────────────────────────────

// SkillLeida es el resultado de abrir un zip subido.
type SkillLeida struct {
	Nombre      string
	Descripcion string
	Archivos    []Archivo
	Bytes       int64
}

// LeerSkill abre un .skill/.zip, lo comprueba y lo normaliza. Se protege de lo
// que un zip hostil puede hacer: rutas que escapan de la carpeta (zip-slip),
// bombas de descompresión (se cuenta lo realmente leído, no lo que dice la
// cabecera) y un número desmesurado de archivos.
func LeerSkill(datos []byte) kernel.Result[SkillLeida] {
	if len(datos) == 0 {
		return kernel.Err[SkillLeida]("El archivo está vacío.")
	}
	if len(datos) > MaxBytesZipSubido {
		return kernel.Err[SkillLeida]("El archivo es demasiado grande (máximo 12 MB).")
	}

	zr, err := zip.NewReader(bytes.NewReader(datos), int64(len(datos)))
	if err != nil {
		return kernel.Err[SkillLeida]("No parece un .skill o un .zip válido.")
	}
	if len(zr.File) > MaxArchivos*2 {
		return kernel.Err[SkillLeida]("El zip tiene demasiados archivos.")
	}

	var archivos []Archivo
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Basura de macOS y de editores: no es parte de la skill.
		if strings.HasPrefix(f.Name, "__MACOSX/") || path.Base(f.Name) == ".DS_Store" {
			continue
		}
		// Un enlace simbólico dentro de un zip no tiene sitio en una skill.
		if f.Mode()&0o170000 == 0o120000 {
			return kernel.Err[SkillLeida]("El zip contiene enlaces simbólicos, que no se admiten.")
		}
		ruta, ok := LimpiarRuta(f.Name)
		if !ok {
			return kernel.Err[SkillLeida]("El zip contiene una ruta no permitida: «" + recortar(f.Name, 60) + "».")
		}
		if len(archivos) >= MaxArchivos {
			return kernel.Err[SkillLeida]("La skill tiene demasiados archivos (máximo 500).")
		}

		rc, err := f.Open()
		if err != nil {
			return kernel.Err[SkillLeida]("No se pudo leer «" + recortar(ruta, 60) + "» del zip.")
		}
		// +1 para distinguir «justo el máximo» de «se pasa».
		contenido, err := io.ReadAll(io.LimitReader(rc, MaxBytesArchivo+1))
		_ = rc.Close()
		if err != nil {
			return kernel.Err[SkillLeida]("El zip está dañado.")
		}
		if len(contenido) > MaxBytesArchivo {
			return kernel.Err[SkillLeida]("«" + recortar(ruta, 60) + "» pesa más de 5 MB.")
		}
		total += int64(len(contenido))
		if total > MaxBytesTotal {
			return kernel.Err[SkillLeida]("La skill descomprimida pesa más de 12 MB.")
		}
		archivos = append(archivos, Archivo{Ruta: ruta, Contenido: contenido})
	}

	archivos = quitarCarpetaRaiz(archivos)
	if len(archivos) == 0 {
		return kernel.Err[SkillLeida]("El zip no tiene archivos.")
	}

	var skillMD []byte
	for _, a := range archivos {
		if a.Ruta == "SKILL.md" {
			skillMD = a.Contenido
		}
	}
	if skillMD == nil {
		return kernel.Err[SkillLeida]("Falta el archivo SKILL.md en la raíz de la skill.")
	}

	nombre, descripcion := LeerCabecera(string(skillMD))
	if nombre == "" {
		return kernel.Err[SkillLeida]("El SKILL.md necesita una cabecera con «name:».")
	}
	v := ValidarNombreSkill(nombre)
	if !v.OK {
		return kernel.Err[SkillLeida](v.Error)
	}
	if strings.TrimSpace(descripcion) == "" {
		return kernel.Err[SkillLeida]("El SKILL.md necesita «description:» en su cabecera: es lo que Claude lee para decidir cuándo usarla.")
	}

	sort.Slice(archivos, func(i, j int) bool { return archivos[i].Ruta < archivos[j].Ruta })
	return kernel.Ok(SkillLeida{Nombre: v.Value, Descripcion: descripcion, Archivos: archivos, Bytes: total})
}

// quitarCarpetaRaiz: la mayoría de los .skill llevan una carpeta con el nombre
// de la skill y todo dentro. Se quita para que SKILL.md quede en la raíz.
func quitarCarpetaRaiz(archivos []Archivo) []Archivo {
	if len(archivos) == 0 {
		return archivos
	}
	for _, a := range archivos {
		if a.Ruta == "SKILL.md" {
			return archivos // ya está en la raíz
		}
	}
	raiz := strings.SplitN(archivos[0].Ruta, "/", 2)[0]
	for _, a := range archivos {
		if !strings.HasPrefix(a.Ruta, raiz+"/") {
			return archivos
		}
	}
	salida := make([]Archivo, 0, len(archivos))
	for _, a := range archivos {
		salida = append(salida, Archivo{Ruta: strings.TrimPrefix(a.Ruta, raiz+"/"), Contenido: a.Contenido})
	}
	return salida
}

// LeerCabecera saca name y description de la cabecera YAML de un SKILL.md.
// Es un YAML mínimo a propósito: dos claves de una línea (la descripción puede
// ir entre comillas), sin traer una librería para eso.
func LeerCabecera(md string) (nombre, descripcion string) {
	md = strings.TrimPrefix(md, "\xef\xbb\xbf")
	md = strings.ReplaceAll(md, "\r\n", "\n")
	if !strings.HasPrefix(md, "---\n") {
		return "", ""
	}
	resto := md[4:]
	fin := strings.Index(resto, "\n---")
	if fin < 0 {
		return "", ""
	}
	for _, linea := range strings.Split(resto[:fin], "\n") {
		clave, valor, ok := strings.Cut(linea, ":")
		if !ok {
			continue
		}
		valor = strings.TrimSpace(valor)
		valor = strings.Trim(valor, `"'`)
		switch strings.TrimSpace(clave) {
		case "name":
			nombre = valor
		case "description":
			descripcion = valor
		}
	}
	return nombre, descripcion
}

// EmpaquetarSkill vuelve a hacer el zip, con una carpeta con el nombre de la
// skill: descomprimido en ~/.claude/skills queda lista.
func EmpaquetarSkill(nombre string, archivos []Archivo) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, a := range archivos {
		w, err := zw.Create(nombre + "/" + a.Ruta)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(a.Contenido); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func utf8Valido(b []byte) bool { return utf8.Valid(b) }
