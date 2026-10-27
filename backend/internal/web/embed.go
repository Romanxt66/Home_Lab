// Package web sirve la interfaz de React desde el propio binario.
//
// En producción no hay servidor de Node: los archivos que genera Vite se
// incrustan en el ejecutable con go:embed. Eso deja un único artefacto que
// desplegar, y de paso elimina el CORS, porque la API y la interfaz pasan a
// compartir origen.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist se rellena en el build con lo que produce `npm run build`. El archivo
// .gitkeep garantiza que el directorio exista aunque no se haya compilado el
// frontend todavía, porque go:embed falla si la carpeta no está.
//
//go:embed all:dist
var dist embed.FS

// Handler devuelve el servidor de archivos de la interfaz, o nil si el binario
// se compiló sin ella (desarrollo, donde Vite sirve en su propio puerto).
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}

	// Si no hay index.html es que no se incrustó nada.
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}

	return &spa{archivos: http.FS(sub), raiz: sub}
}

// spa sirve archivos estáticos y, para cualquier ruta que no exista, devuelve
// index.html. Es lo que necesita una aplicación de una sola página: /uptime no
// es un archivo, lo resuelve el enrutador de React en el navegador.
type spa struct {
	archivos http.FileSystem
	raiz     fs.FS
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	limpia := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if limpia == "." || limpia == "/" {
		limpia = "index.html"
	}

	if _, err := fs.Stat(s.raiz, limpia); err != nil {
		// No existe: es una ruta del enrutador, no un archivo.
		r = r.Clone(r.Context())
		r.URL.Path = "/"
		limpia = "index.html"
	}

	// Los recursos con huella en el nombre (app-CBFiwSH5.js) no cambian nunca,
	// así que se pueden cachear para siempre. index.html no, o el navegador se
	// quedaría con una versión vieja tras cada despliegue.
	if limpia != "index.html" && strings.HasPrefix(limpia, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	http.FileServer(s.archivos).ServeHTTP(w, r)
}
