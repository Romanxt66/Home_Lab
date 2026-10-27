package deploygen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Romanxt66/Home_Lab/backend/internal/kernel"
)

const (
	apiGitHub    = "https://api.github.com"
	tiempoLimite = 15 * time.Second
)

// manifiestos son los archivos cuyo contenido se lee. Del resto basta con
// saber si están o no, que se deduce del listado del directorio.
var manifiestos = []string{
	"package.json",
	".nvmrc",
	"requirements.txt",
	"pyproject.toml",
	"composer.json",
	"go.mod",
	"Gemfile",
	"pom.xml",
	"build.gradle",
	"build.gradle.kts",
}

// RepoLeido es lo que se trajo del repositorio.
type RepoLeido struct {
	Owner    string
	Repo     string
	Branch   string
	Archivos Archivos
}

// reRepo acepta la URL completa, el "github.com/owner/repo", el "owner/repo"
// y la forma SSH "git@github.com:owner/repo".
//
// Los dos puntos quedan fuera del dueño a propósito: sin eso, de
// "git@github.com:Romanxt66/Lab" salía el dueño "git@github.com:Romanxt66",
// que es lo que hacía el Lab.
var reRepo = regexp.MustCompile(`(?:github\.com[/:])?([^/:\s]+)/([^/\s]+)$`)

// PartirRepo saca el dueño y el nombre de lo que haya escrito el usuario.
func PartirRepo(entrada string) (owner, repo string, ok bool) {
	limpio := strings.TrimSuffix(strings.TrimSpace(entrada), "/")
	limpio = strings.TrimSuffix(limpio, ".git")
	limpio = strings.TrimSuffix(limpio, "/")

	m := reRepo.FindStringSubmatch(limpio)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// LectorGitHub trae los manifiestos de un repositorio y, si hace falta,
// escribe un archivo en él.
type LectorGitHub struct {
	client *http.Client
}

func NewLectorGitHub() *LectorGitHub {
	return &LectorGitHub{client: &http.Client{Timeout: tiempoLimite}}
}

// Leer trae el listado del directorio y el contenido de los manifiestos.
func (l *LectorGitHub) Leer(ctx context.Context, repoURL, rama, subdir string, token *string) kernel.Result[RepoLeido] {
	owner, repo, ok := PartirRepo(repoURL)
	if !ok {
		return kernel.Err[RepoLeido](
			"Esa dirección de repositorio no vale. Usa owner/repo o la URL de GitHub.")
	}

	ref := strings.TrimSpace(rama)
	if ref == "" {
		var info struct {
			DefaultBranch string `json:"default_branch"`
		}
		res := l.leerJSON(ctx, apiGitHub+"/repos/"+owner+"/"+repo, token, &info)
		if !res.OK {
			return kernel.Err[RepoLeido](res.Error)
		}
		if res.Value == noEncontrado {
			return kernel.Err[RepoLeido]("GitHub no encuentra ese repositorio.")
		}
		ref = info.DefaultBranch
		if ref == "" {
			ref = "main"
		}
	}

	prefijo := strings.Trim(strings.TrimSpace(subdir), "/")
	ruta := ""
	if prefijo != "" {
		ruta = "/" + rutaCodificada(prefijo)
	}

	var listado []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	res := l.leerJSON(ctx,
		apiGitHub+"/repos/"+owner+"/"+repo+"/contents"+ruta+"?ref="+url.QueryEscape(ref),
		token, &listado)
	if !res.OK {
		return kernel.Err[RepoLeido](res.Error)
	}
	if res.Value == noEncontrado {
		if prefijo != "" {
			return kernel.Err[RepoLeido]("No existe el directorio «" + prefijo + "» en esa rama.")
		}
		return kernel.Err[RepoLeido]("No se pudo leer el contenido del repositorio.")
	}

	rutas := make([]string, 0, len(listado))
	presentes := make(map[string]bool, len(listado))
	for _, e := range listado {
		rutas = append(rutas, e.Name)
		presentes[strings.ToLower(e.Name)] = true
	}

	contenidos := map[string]string{}
	for _, manifiesto := range manifiestos {
		if !presentes[strings.ToLower(manifiesto)] {
			continue
		}
		destino := manifiesto
		if prefijo != "" {
			destino = prefijo + "/" + manifiesto
		}
		if texto, ok := l.leerArchivo(ctx, owner, repo, destino, ref, token); ok {
			contenidos[strings.ToLower(manifiesto)] = texto
		}
	}

	return kernel.Ok(RepoLeido{
		Owner: owner, Repo: repo, Branch: ref,
		Archivos: Archivos{Rutas: rutas, Contenidos: contenidos},
	})
}

func (l *LectorGitHub) leerArchivo(ctx context.Context, owner, repo, ruta, ref string, token *string) (string, bool) {
	var archivo struct {
		Content string `json:"content"`
	}
	res := l.leerJSON(ctx,
		apiGitHub+"/repos/"+owner+"/"+repo+"/contents/"+rutaCodificada(ruta)+"?ref="+url.QueryEscape(ref),
		token, &archivo)
	if !res.OK || res.Value == noEncontrado || archivo.Content == "" {
		return "", false
	}

	// GitHub devuelve el contenido en base64 con saltos de línea dentro.
	limpio := strings.NewReplacer("\n", "", "\r", "").Replace(archivo.Content)
	datos, err := base64.StdEncoding.DecodeString(limpio)
	if err != nil {
		return "", false
	}
	return string(datos), true
}

// Escribir crea o actualiza un archivo en el repositorio. Hace falta un token
// con permiso de escritura, y actualizar exige el sha de lo que ya hay.
func (l *LectorGitHub) Escribir(ctx context.Context, repoURL, rama, ruta, contenido, mensaje string, token *string) kernel.Result[string] {
	if token == nil {
		return kernel.Err[string](
			"Hace falta un token de GitHub con permiso de escritura. Conéctalo en la pantalla de GitHub.")
	}

	owner, repo, ok := PartirRepo(repoURL)
	if !ok {
		return kernel.Err[string]("Esa dirección de repositorio no vale.")
	}

	limpia := strings.TrimLeft(ruta, "/")
	destino := apiGitHub + "/repos/" + owner + "/" + repo + "/contents/" + rutaCodificada(limpia)

	// Si el archivo ya existe hace falta su sha para reemplazarlo.
	var actual struct {
		SHA string `json:"sha"`
	}
	res := l.leerJSON(ctx, destino+"?ref="+url.QueryEscape(rama), token, &actual)
	if !res.OK {
		return kernel.Err[string](res.Error)
	}

	cuerpo := map[string]string{
		"message": mensaje,
		"content": base64.StdEncoding.EncodeToString([]byte(contenido)),
		"branch":  rama,
	}
	if actual.SHA != "" {
		cuerpo["sha"] = actual.SHA
	}

	datos, _ := json.Marshal(cuerpo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, destino, bytes.NewReader(datos))
	if err != nil {
		return kernel.Err[string]("No se pudo preparar la escritura.")
	}
	ponerCabeceras(req, token)
	req.Header.Set("Content-Type", "application/json")

	respuesta, err := l.client.Do(req)
	if err != nil {
		return kernel.Err[string]("No se pudo conectar con GitHub.")
	}
	defer respuesta.Body.Close()

	if respuesta.StatusCode == http.StatusUnauthorized || respuesta.StatusCode == http.StatusForbidden {
		return kernel.Err[string](
			"GitHub rechazó la escritura: al token le falta permiso de escritura (scope repo, o Contents: write).")
	}
	if respuesta.StatusCode < 200 || respuesta.StatusCode >= 300 {
		detalle, _ := io.ReadAll(io.LimitReader(respuesta.Body, 1<<16))
		return kernel.Err[string](explicarGitHub(respuesta.StatusCode, detalle))
	}

	if actual.SHA != "" {
		return kernel.Ok(limpia + " actualizado en " + rama + ".")
	}
	return kernel.Ok(limpia + " creado en " + rama + ".")
}

// noEncontrado marca un 404, que aquí no es un error sino "no está".
const noEncontrado = "404"

// leerJSON hace un GET y vuelca el JSON. Devuelve noEncontrado como valor
// cuando GitHub responde 404, para distinguirlo de un fallo de verdad.
func (l *LectorGitHub) leerJSON(ctx context.Context, destino string, token *string, salida any) kernel.Result[string] {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, destino, nil)
	if err != nil {
		return kernel.Err[string]("No se pudo preparar la petición a GitHub.")
	}
	ponerCabeceras(req, token)

	res, err := l.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			return kernel.Err[string]("GitHub tardó demasiado en responder.")
		}
		return kernel.Err[string]("No se pudo conectar con GitHub.")
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return kernel.Ok(noEncontrado)
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return kernel.Err[string](
			"GitHub rechazó la petición. ¿Es un repositorio privado sin token, o se agotó el límite?")
	}

	cuerpo, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return kernel.Err[string](explicarGitHub(res.StatusCode, cuerpo))
	}
	if err := json.Unmarshal(cuerpo, salida); err != nil {
		return kernel.Err[string]("GitHub devolvió una respuesta que no se entiende.")
	}

	return kernel.Ok("")
}

func ponerCabeceras(req *http.Request, token *string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Home-Lab-Deploygen")
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+*token)
	}
}

// rutaCodificada escapa cada tramo por separado: escapar la ruta entera
// convertiría las barras en %2F y GitHub ya no vería los directorios.
func rutaCodificada(ruta string) string {
	tramos := strings.Split(ruta, "/")
	for i, t := range tramos {
		tramos[i] = url.PathEscape(t)
	}
	return strings.Join(tramos, "/")
}

func explicarGitHub(estado int, cuerpo []byte) string {
	var detalle struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(cuerpo, &detalle); err == nil && detalle.Message != "" {
		return "GitHub: " + detalle.Message
	}
	return fmt.Sprintf("GitHub respondió con un error (HTTP %d).", estado)
}
